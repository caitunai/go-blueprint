package db

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MaxConfigBytes             = 512 * 1024
	MaxConfigDescriptionBytes  = 512 * 1024
	maxConfigDepth             = 32
	maxConfigNodes             = 5000
	maxConfigCollection        = 1000
	maxConfigKeyLength         = 128
	maxConfigStringLength      = 64 * 1024
	maxConfigDescriptionLength = 2000
	maxConfigEnvironments      = 100
	maxEnvironmentDepth        = 16
)

var (
	ErrConfigEnvironmentNotFound = errors.New("config environment not found")
	ErrConfigInvalid             = errors.New("invalid configuration")
	ErrConfigEnvironmentInvalid  = errors.New("invalid config environment")
	ErrConfigEnvironmentConflict = errors.New("config environment conflicts with existing data")
	ErrConfigEnvironmentInUse    = errors.New("config environment is inherited by another environment")
	ErrConfigReleaseNotFound     = errors.New("published configuration not found")
	ErrConfigStorage             = errors.New("config storage operation failed")

	environmentSlugPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
)

// ConfigEnvironment stores only the overrides owned by an environment. The
// final configuration is resolved by merging each ancestor from root to leaf.
type ConfigEnvironment struct {
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	Name              string    `gorm:"size:100;not null" json:"name"`
	Slug              string    `gorm:"size:64;not null;uniqueIndex" json:"slug"`
	Description       string    `gorm:"size:500;not null;default:''" json:"description"`
	DraftConfig       string    `gorm:"type:mediumtext;not null" json:"-"`
	DraftDescriptions string    `gorm:"type:mediumtext;not null" json:"-"`
	ID                uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ParentID          uint      `gorm:"not null;default:0;index" json:"parent_id"`
	Version           uint64    `gorm:"-" json:"published_version"`
	HasDraft          bool      `gorm:"-" json:"has_draft"`
}

// ConfigRelease is an immutable, fully-resolved configuration snapshot.
type ConfigRelease struct {
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	BatchID       string    `gorm:"size:32;not null;index" json:"batch_id"`
	Config        string    `gorm:"type:mediumtext;not null" json:"-"`
	Descriptions  string    `gorm:"type:mediumtext;not null" json:"-"`
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	EnvironmentID uint      `gorm:"not null;uniqueIndex:idx_config_release_version" json:"environment_id"`
	Version       uint64    `gorm:"not null;uniqueIndex:idx_config_release_version" json:"version"`
}

type ConfigEnvironmentInput struct {
	Name        string
	Slug        string
	Description string
	ParentID    uint
}

type ResolvedConfig struct {
	Config       map[string]any      `json:"config"`
	Descriptions ConfigDescriptions  `json:"descriptions"`
	Chain        []ConfigEnvironment `json:"inheritance_chain"`
	Environment  ConfigEnvironment   `json:"environment"`
}

type PublishedConfig struct {
	PublishedAt   time.Time          `json:"published_at"`
	Config        map[string]any     `json:"config"`
	Descriptions  ConfigDescriptions `json:"descriptions"`
	BatchID       string             `json:"batch_id"`
	EnvironmentID uint               `json:"environment_id"`
	Version       uint64             `json:"version"`
}

type ConfigDescriptions map[string]string

type ConfigReleaseSummary struct {
	PublishedAt   time.Time `json:"published_at"`
	BatchID       string    `json:"batch_id"`
	EnvironmentID uint      `json:"environment_id"`
	Version       uint64    `json:"version"`
}

type ConfigPublishResult struct {
	BatchID  string            `json:"batch_id"`
	Releases []PublishedConfig `json:"releases"`
}

func ListConfigEnvironments(ctx context.Context) ([]ConfigEnvironment, error) {
	return listConfigEnvironments(ctx, DB())
}

func listConfigEnvironments(ctx context.Context, conn *gorm.DB) ([]ConfigEnvironment, error) {
	environments := make([]ConfigEnvironment, 0)
	if err := conn.WithContext(ctx).Order("name ASC, id ASC").Find(&environments).Error; err != nil {
		return nil, errors.Join(ErrConfigStorage, err)
	}

	latestVersions := conn.WithContext(ctx).
		Model(&ConfigRelease{}).
		Select("environment_id, MAX(version) AS version").
		Group("environment_id")
	latestReleases := make([]ConfigRelease, 0)
	if err := conn.WithContext(ctx).
		Where("(environment_id, version) IN (?)", latestVersions).
		Find(&latestReleases).Error; err != nil {
		return nil, errors.Join(ErrConfigStorage, err)
	}
	if err := applyConfigEnvironmentReleaseState(environments, latestReleases); err != nil {
		return nil, err
	}
	return environments, nil
}

func applyConfigEnvironmentReleaseState(environments []ConfigEnvironment, releases []ConfigRelease) error {
	latestByEnvironment := make(map[uint]ConfigRelease, len(releases))
	for _, release := range releases {
		latest, exists := latestByEnvironment[release.EnvironmentID]
		if !exists || release.Version > latest.Version {
			latestByEnvironment[release.EnvironmentID] = release
		}
	}
	for index := range environments {
		release, published := latestByEnvironment[environments[index].ID]
		if !published {
			environments[index].HasDraft = true
			continue
		}
		resolved, err := resolveConfigFromEnvironments(environments, environments[index].ID)
		if err != nil {
			return err
		}
		config, descriptions, err := encodeResolvedConfig(resolved)
		if err != nil {
			return err
		}
		environments[index].Version = release.Version
		environments[index].HasDraft = config != release.Config || descriptions != release.Descriptions
	}
	return nil
}

func CreateConfigEnvironment(ctx context.Context, input ConfigEnvironmentInput) (*ConfigEnvironment, error) {
	input = normalizeConfigEnvironmentInput(input)
	if err := validateConfigEnvironmentInput(input); err != nil {
		return nil, err
	}

	var created *ConfigEnvironment
	err := DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&ConfigEnvironment{}).Count(&count).Error; err != nil {
			return errors.Join(ErrConfigStorage, err)
		}
		if count >= maxConfigEnvironments {
			return errors.Join(ErrConfigEnvironmentInvalid, errors.New("environment limit reached"))
		}
		environments, err := lockedConfigEnvironments(ctx, tx)
		if err != nil {
			return err
		}
		if err := validateEnvironmentParent(environments, 0, input.ParentID); err != nil {
			return err
		}
		if environmentSlugExists(environments, input.Slug, 0) {
			return ErrConfigEnvironmentConflict
		}
		created = &ConfigEnvironment{
			Name:              input.Name,
			Slug:              input.Slug,
			Description:       input.Description,
			ParentID:          input.ParentID,
			DraftConfig:       "{}",
			DraftDescriptions: "{}",
		}
		if err := tx.Create(created).Error; err != nil {
			return errors.Join(ErrConfigStorage, err)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Join(ErrConfigStorage, err)
	}
	return created, nil
}

func UpdateConfigEnvironment(ctx context.Context, id uint, input ConfigEnvironmentInput) (*ConfigEnvironment, error) {
	input = normalizeConfigEnvironmentInput(input)
	if id == 0 {
		return nil, ErrConfigEnvironmentNotFound
	}
	if err := validateConfigEnvironmentInput(input); err != nil {
		return nil, err
	}

	updated := &ConfigEnvironment{}
	err := DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		environments, err := lockedConfigEnvironments(ctx, tx)
		if err != nil {
			return err
		}
		_, exists := configEnvironmentByID(environments, id)
		if !exists {
			return ErrConfigEnvironmentNotFound
		}
		if err := validateEnvironmentParent(environments, id, input.ParentID); err != nil {
			return err
		}
		if environmentSlugExists(environments, input.Slug, id) {
			return ErrConfigEnvironmentConflict
		}
		changes := map[string]any{
			"name":        input.Name,
			"slug":        input.Slug,
			"description": input.Description,
			"parent_id":   input.ParentID,
		}
		if err := tx.Model(&ConfigEnvironment{}).Where("id = ?", id).Updates(changes).Error; err != nil {
			return errors.Join(ErrConfigStorage, err)
		}
		if err := tx.First(updated, id).Error; err != nil {
			return errors.Join(ErrConfigStorage, err)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Join(ErrConfigStorage, err)
	}
	return updated, nil
}

func DeleteConfigEnvironment(ctx context.Context, id uint) error {
	if id == 0 {
		return ErrConfigEnvironmentNotFound
	}
	err := DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		environments, err := lockedConfigEnvironments(ctx, tx)
		if err != nil {
			return err
		}
		if _, exists := configEnvironmentByID(environments, id); !exists {
			return ErrConfigEnvironmentNotFound
		}
		for _, environment := range environments {
			if environment.ParentID == id {
				return ErrConfigEnvironmentInUse
			}
		}
		if err := tx.Where("environment_id = ?", id).Delete(&ConfigRelease{}).Error; err != nil {
			return errors.Join(ErrConfigStorage, err)
		}
		if err := tx.Delete(&ConfigEnvironment{}, id).Error; err != nil {
			return errors.Join(ErrConfigStorage, err)
		}
		return nil
	})
	if err != nil {
		return errors.Join(ErrConfigStorage, err)
	}
	return nil
}

func SaveConfigDraft(ctx context.Context, environmentID uint, raw, rawDescriptions []byte) (*ResolvedConfig, error) {
	config, err := DecodeConfig(raw)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return nil, errors.Join(ErrConfigInvalid, err)
	}
	if len(encoded) > MaxConfigBytes {
		return nil, errors.Join(ErrConfigInvalid, errors.New("configuration is too large"))
	}
	descriptions, err := DecodeConfigDescriptions(rawDescriptions, config)
	if err != nil {
		return nil, err
	}
	encodedDescriptions, err := json.Marshal(descriptions)
	if err != nil {
		return nil, errors.Join(ErrConfigInvalid, err)
	}
	result := &ResolvedConfig{}
	err = DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var environment ConfigEnvironment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&environment, environmentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Join(ErrConfigEnvironmentNotFound, err)
			}
			return errors.Join(ErrConfigStorage, err)
		}
		changes := map[string]any{
			"draft_config":       string(encoded),
			"draft_descriptions": string(encodedDescriptions),
		}
		if err := tx.Model(&environment).Updates(changes).Error; err != nil {
			return errors.Join(ErrConfigStorage, err)
		}
		resolved, err := resolveConfig(ctx, tx, environmentID)
		if err != nil {
			return err
		}
		*result = *resolved
		return nil
	})
	if err != nil {
		return nil, errors.Join(ErrConfigStorage, err)
	}
	return result, nil
}

func GetConfigDraft(ctx context.Context, environmentID uint) (*ResolvedConfig, map[string]any, ConfigDescriptions, error) {
	resolved, err := resolveConfig(ctx, DB(), environmentID)
	if err != nil {
		return nil, nil, nil, err
	}
	draft, err := DecodeConfig([]byte(resolved.Environment.DraftConfig))
	if err != nil {
		return nil, nil, nil, errors.Join(ErrConfigStorage, err)
	}
	descriptions, err := DecodeConfigDescriptions([]byte(resolved.Environment.DraftDescriptions), draft)
	if err != nil {
		return nil, nil, nil, errors.Join(ErrConfigStorage, err)
	}
	return resolved, draft, descriptions, nil
}

func ResolveConfigDraft(ctx context.Context, environmentID uint) (*ResolvedConfig, error) {
	return resolveConfig(ctx, DB(), environmentID)
}

func resolveConfig(ctx context.Context, conn *gorm.DB, environmentID uint) (*ResolvedConfig, error) {
	environments := make([]ConfigEnvironment, 0)
	if err := conn.WithContext(ctx).Order("id ASC").Find(&environments).Error; err != nil {
		return nil, errors.Join(ErrConfigStorage, err)
	}
	return resolveConfigFromEnvironments(environments, environmentID)
}

func resolveConfigFromEnvironments(environments []ConfigEnvironment, environmentID uint) (*ResolvedConfig, error) {
	byID := make(map[uint]ConfigEnvironment, len(environments))
	for _, environment := range environments {
		byID[environment.ID] = environment
	}
	target, exists := byID[environmentID]
	if !exists {
		return nil, ErrConfigEnvironmentNotFound
	}

	chain := make([]ConfigEnvironment, 0, maxEnvironmentDepth)
	visited := make(map[uint]struct{}, maxEnvironmentDepth)
	current := target
	for {
		if _, duplicate := visited[current.ID]; duplicate {
			return nil, errors.Join(ErrConfigEnvironmentInvalid, errors.New("environment inheritance cycle detected"))
		}
		visited[current.ID] = struct{}{}
		chain = append(chain, current)
		if len(chain) > maxEnvironmentDepth {
			return nil, errors.Join(ErrConfigEnvironmentInvalid, errors.New("environment inheritance is too deep"))
		}
		if current.ParentID == 0 {
			break
		}
		parent, found := byID[current.ParentID]
		if !found {
			return nil, errors.Join(ErrConfigEnvironmentInvalid, errors.New("parent environment does not exist"))
		}
		current = parent
	}
	slices.Reverse(chain)

	finalConfig := make(map[string]any)
	finalDescriptions := make(ConfigDescriptions)
	for _, environment := range chain {
		draft, err := DecodeConfig([]byte(environment.DraftConfig))
		if err != nil {
			return nil, errors.Join(ErrConfigStorage, err)
		}
		descriptions, err := DecodeConfigDescriptions([]byte(environment.DraftDescriptions), draft)
		if err != nil {
			return nil, errors.Join(ErrConfigStorage, err)
		}
		merged, mergedDescriptions := mergeConfigWithDescriptions(
			finalConfig,
			finalDescriptions,
			draft,
			descriptions,
			"",
		)
		finalConfig = merged.(map[string]any)
		finalDescriptions = mergedDescriptions
	}
	return &ResolvedConfig{
		Environment:  target,
		Chain:        chain,
		Config:       finalConfig,
		Descriptions: finalDescriptions,
	}, nil
}

func PublishConfigs(ctx context.Context, environmentIDs []uint) (*ConfigPublishResult, error) {
	ids := uniqueSortedIDs(environmentIDs)
	if len(ids) == 0 || len(ids) > maxConfigEnvironments {
		return nil, errors.Join(ErrConfigEnvironmentInvalid, errors.New("select between 1 and 100 environments"))
	}
	batchID, err := newConfigBatchID()
	if err != nil {
		return nil, errors.Join(ErrConfigStorage, err)
	}
	result := &ConfigPublishResult{BatchID: batchID, Releases: make([]PublishedConfig, 0, len(ids))}
	err = DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		environments, err := lockedConfigEnvironments(ctx, tx)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, exists := configEnvironmentByID(environments, id); !exists {
				return ErrConfigEnvironmentNotFound
			}
		}
		for _, id := range ids {
			resolved, err := resolveConfigFromEnvironments(environments, id)
			if err != nil {
				return err
			}
			encoded, encodedDescriptions, err := encodeResolvedConfig(resolved)
			if err != nil {
				return err
			}
			var latest uint64
			if err := tx.Model(&ConfigRelease{}).
				Where("environment_id = ?", id).
				Select("COALESCE(MAX(version), 0)").
				Scan(&latest).Error; err != nil {
				return errors.Join(ErrConfigStorage, err)
			}
			release := &ConfigRelease{
				EnvironmentID: id,
				BatchID:       batchID,
				Version:       latest + 1,
				Config:        encoded,
				Descriptions:  encodedDescriptions,
			}
			if err := tx.Create(release).Error; err != nil {
				return errors.Join(ErrConfigStorage, err)
			}
			result.Releases = append(result.Releases, PublishedConfig{
				EnvironmentID: id,
				BatchID:       batchID,
				Version:       release.Version,
				Config:        resolved.Config,
				Descriptions:  resolved.Descriptions,
				PublishedAt:   release.CreatedAt,
			})
		}
		return nil
	})
	if err != nil {
		return nil, errors.Join(ErrConfigStorage, err)
	}
	return result, nil
}

func encodeResolvedConfig(resolved *ResolvedConfig) (string, string, error) {
	encoded, err := json.Marshal(resolved.Config)
	if err != nil {
		return "", "", errors.Join(ErrConfigInvalid, err)
	}
	encodedDescriptions, err := json.Marshal(resolved.Descriptions)
	if err != nil {
		return "", "", errors.Join(ErrConfigInvalid, err)
	}
	return string(encoded), string(encodedDescriptions), nil
}

func LatestPublishedConfig(ctx context.Context, environmentID uint) (*PublishedConfig, error) {
	var release ConfigRelease
	err := DB().WithContext(ctx).
		Where("environment_id = ?", environmentID).
		Order("version DESC").
		First(&release).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.Join(ErrConfigReleaseNotFound, err)
		}
		return nil, errors.Join(ErrConfigStorage, err)
	}
	return publishedConfigFromRelease(release)
}

func PublishedConfigVersion(ctx context.Context, environmentID uint, version uint64) (*PublishedConfig, error) {
	if environmentID == 0 || version == 0 {
		return nil, ErrConfigReleaseNotFound
	}
	var release ConfigRelease
	err := DB().WithContext(ctx).
		Where("environment_id = ? AND version = ?", environmentID, version).
		First(&release).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.Join(ErrConfigReleaseNotFound, err)
		}
		return nil, errors.Join(ErrConfigStorage, err)
	}
	return publishedConfigFromRelease(release)
}

func ListConfigReleases(ctx context.Context, environmentID uint) ([]ConfigReleaseSummary, error) {
	if environmentID == 0 {
		return nil, ErrConfigEnvironmentNotFound
	}
	var count int64
	if err := DB().WithContext(ctx).Model(&ConfigEnvironment{}).Where("id = ?", environmentID).Count(&count).Error; err != nil {
		return nil, errors.Join(ErrConfigStorage, err)
	}
	if count == 0 {
		return nil, ErrConfigEnvironmentNotFound
	}
	releases := make([]ConfigReleaseSummary, 0)
	if err := DB().WithContext(ctx).
		Model(&ConfigRelease{}).
		Select("environment_id, batch_id, version, created_at AS published_at").
		Where("environment_id = ?", environmentID).
		Order("version DESC").
		Scan(&releases).Error; err != nil {
		return nil, errors.Join(ErrConfigStorage, err)
	}
	return releases, nil
}

func ListAllConfigReleases(ctx context.Context) ([]ConfigReleaseSummary, error) {
	releases := make([]ConfigReleaseSummary, 0)
	if err := DB().WithContext(ctx).
		Model(&ConfigRelease{}).
		Select("environment_id, batch_id, version, created_at AS published_at").
		Order("environment_id ASC, version DESC").
		Scan(&releases).Error; err != nil {
		return nil, errors.Join(ErrConfigStorage, err)
	}
	return releases, nil
}

func publishedConfigFromRelease(release ConfigRelease) (*PublishedConfig, error) {
	config, err := DecodeConfig([]byte(release.Config))
	if err != nil {
		return nil, errors.Join(ErrConfigStorage, err)
	}
	descriptions, err := DecodeConfigDescriptions([]byte(release.Descriptions), config)
	if err != nil {
		return nil, errors.Join(ErrConfigStorage, err)
	}
	return &PublishedConfig{
		EnvironmentID: release.EnvironmentID,
		BatchID:       release.BatchID,
		Version:       release.Version,
		Config:        config,
		Descriptions:  descriptions,
		PublishedAt:   release.CreatedAt,
	}, nil
}

func DecodeConfig(raw []byte) (map[string]any, error) {
	if len(raw) == 0 || len(raw) > MaxConfigBytes {
		return nil, errors.Join(ErrConfigInvalid, errors.New("configuration size is invalid"))
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var config map[string]any
	if err := decoder.Decode(&config); err != nil {
		return nil, errors.Join(ErrConfigInvalid, err)
	}
	if config == nil {
		return nil, errors.Join(ErrConfigInvalid, errors.New("configuration root must be an object"))
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("configuration contains multiple JSON values")
		}
		return nil, errors.Join(ErrConfigInvalid, err)
	}
	nodes := 0
	if err := validateConfigValue(config, 1, &nodes); err != nil {
		return nil, err
	}
	return config, nil
}

func DecodeConfigDescriptions(raw []byte, config map[string]any) (ConfigDescriptions, error) {
	if len(raw) == 0 {
		return make(ConfigDescriptions), nil
	}
	if len(raw) > MaxConfigDescriptionBytes {
		return nil, errors.Join(ErrConfigInvalid, errors.New("configuration descriptions are too large"))
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var descriptions ConfigDescriptions
	if err := decoder.Decode(&descriptions); err != nil {
		return nil, errors.Join(ErrConfigInvalid, err)
	}
	if descriptions == nil {
		descriptions = make(ConfigDescriptions)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("configuration descriptions contain multiple JSON values")
		}
		return nil, errors.Join(ErrConfigInvalid, err)
	}
	if len(descriptions) > maxConfigNodes {
		return nil, errors.Join(ErrConfigInvalid, errors.New("configuration contains too many descriptions"))
	}
	normalized := make(ConfigDescriptions, len(descriptions))
	for pointer, description := range descriptions {
		description = strings.TrimSpace(description)
		if description == "" {
			continue
		}
		if len(description) > maxConfigDescriptionLength {
			return nil, errors.Join(ErrConfigInvalid, errors.New("configuration description is too long"))
		}
		path, err := parseJSONPointer(pointer)
		if err != nil || !configPathExists(config, path) {
			return nil, errors.Join(ErrConfigInvalid, errors.New("configuration description path is invalid"))
		}
		normalized[pointer] = description
	}
	return normalized, nil
}

func validateConfigValue(value any, depth int, nodes *int) error {
	(*nodes)++
	if depth > maxConfigDepth {
		return errors.Join(ErrConfigInvalid, errors.New("configuration nesting is too deep"))
	}
	if *nodes > maxConfigNodes {
		return errors.Join(ErrConfigInvalid, errors.New("configuration contains too many values"))
	}
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) > maxConfigCollection {
			return errors.Join(ErrConfigInvalid, errors.New("configuration object contains too many fields"))
		}
		for key, child := range typed {
			if strings.TrimSpace(key) == "" || len(key) > maxConfigKeyLength {
				return errors.Join(ErrConfigInvalid, errors.New("configuration key is invalid"))
			}
			if err := validateConfigValue(child, depth+1, nodes); err != nil {
				return err
			}
		}
	case []any:
		if len(typed) > maxConfigCollection {
			return errors.Join(ErrConfigInvalid, errors.New("configuration array contains too many items"))
		}
		for _, child := range typed {
			if err := validateConfigValue(child, depth+1, nodes); err != nil {
				return err
			}
		}
	case string:
		if len(typed) > maxConfigStringLength {
			return errors.Join(ErrConfigInvalid, errors.New("configuration string is too long"))
		}
	case bool, json.Number:
		return nil
	default:
		return errors.Join(ErrConfigInvalid, errors.New("configuration contains an unsupported value type"))
	}
	return nil
}

func mergeConfig(base, override any) any {
	baseObject, baseIsObject := base.(map[string]any)
	overrideObject, overrideIsObject := override.(map[string]any)
	if !baseIsObject || !overrideIsObject {
		return cloneConfigValue(override)
	}
	merged := make(map[string]any, len(baseObject)+len(overrideObject))
	for key, value := range baseObject {
		merged[key] = cloneConfigValue(value)
	}
	for key, value := range overrideObject {
		if inherited, exists := merged[key]; exists {
			merged[key] = mergeConfig(inherited, value)
		} else {
			merged[key] = cloneConfigValue(value)
		}
	}
	return merged
}

func mergeConfigWithDescriptions(
	base any,
	baseDescriptions ConfigDescriptions,
	override any,
	overrideDescriptions ConfigDescriptions,
	pointer string,
) (any, ConfigDescriptions) {
	mergedDescriptions := cloneConfigDescriptions(baseDescriptions)
	baseObject, baseIsObject := base.(map[string]any)
	overrideObject, overrideIsObject := override.(map[string]any)
	if !baseIsObject || !overrideIsObject {
		removeDescriptionSubtree(mergedDescriptions, pointer)
		copyDescriptionSubtree(mergedDescriptions, overrideDescriptions, pointer)
		return cloneConfigValue(override), mergedDescriptions
	}

	merged := make(map[string]any, len(baseObject)+len(overrideObject))
	for key, value := range baseObject {
		merged[key] = cloneConfigValue(value)
	}
	if description, exists := overrideDescriptions[pointer]; exists && pointer != "" {
		mergedDescriptions[pointer] = description
	}
	for key, value := range overrideObject {
		childPointer := appendJSONPointer(pointer, key)
		if inherited, exists := merged[key]; exists {
			merged[key], mergedDescriptions = mergeConfigWithDescriptions(
				inherited,
				mergedDescriptions,
				value,
				overrideDescriptions,
				childPointer,
			)
			continue
		}
		merged[key] = cloneConfigValue(value)
		copyDescriptionSubtree(mergedDescriptions, overrideDescriptions, childPointer)
	}
	return merged, mergedDescriptions
}

func parseJSONPointer(pointer string) ([]string, error) {
	if pointer == "" || !strings.HasPrefix(pointer, "/") {
		return nil, errors.New("JSON pointer must identify a configuration item")
	}
	encodedSegments := strings.Split(pointer[1:], "/")
	segments := make([]string, len(encodedSegments))
	for index, encoded := range encodedSegments {
		var builder strings.Builder
		for position := 0; position < len(encoded); position++ {
			if encoded[position] != '~' {
				builder.WriteByte(encoded[position])
				continue
			}
			if position+1 >= len(encoded) || (encoded[position+1] != '0' && encoded[position+1] != '1') {
				return nil, errors.New("JSON pointer contains an invalid escape")
			}
			position++
			if encoded[position] == '0' {
				builder.WriteByte('~')
			} else {
				builder.WriteByte('/')
			}
		}
		segments[index] = builder.String()
	}
	return segments, nil
}

func configPathExists(config map[string]any, path []string) bool {
	var current any = config
	for _, segment := range path {
		switch typed := current.(type) {
		case map[string]any:
			var exists bool
			current, exists = typed[segment]
			if !exists {
				return false
			}
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(typed) || strconv.Itoa(index) != segment {
				return false
			}
			current = typed[index]
		default:
			return false
		}
	}
	return true
}

func appendJSONPointer(pointer, segment string) string {
	escaped := strings.ReplaceAll(strings.ReplaceAll(segment, "~", "~0"), "/", "~1")
	return pointer + "/" + escaped
}

func cloneConfigDescriptions(descriptions ConfigDescriptions) ConfigDescriptions {
	cloned := make(ConfigDescriptions, len(descriptions))
	for pointer, description := range descriptions {
		cloned[pointer] = description
	}
	return cloned
}

func removeDescriptionSubtree(descriptions ConfigDescriptions, pointer string) {
	if pointer == "" {
		clear(descriptions)
		return
	}
	for candidate := range descriptions {
		if candidate == pointer || strings.HasPrefix(candidate, pointer+"/") {
			delete(descriptions, candidate)
		}
	}
}

func copyDescriptionSubtree(target, source ConfigDescriptions, pointer string) {
	for candidate, description := range source {
		if pointer == "" || candidate == pointer || strings.HasPrefix(candidate, pointer+"/") {
			target[candidate] = description
		}
	}
}

func cloneConfigValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, child := range typed {
			cloned[key] = cloneConfigValue(child)
		}
		return cloned
	case []any:
		cloned := make([]any, len(typed))
		for index, child := range typed {
			cloned[index] = cloneConfigValue(child)
		}
		return cloned
	default:
		return typed
	}
}

func normalizeConfigEnvironmentInput(input ConfigEnvironmentInput) ConfigEnvironmentInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = strings.ToLower(strings.TrimSpace(input.Slug))
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func validateConfigEnvironmentInput(input ConfigEnvironmentInput) error {
	if input.Name == "" || len(input.Name) > 100 {
		return errors.Join(ErrConfigEnvironmentInvalid, errors.New("environment name must contain 1 to 100 characters"))
	}
	if !environmentSlugPattern.MatchString(input.Slug) {
		return errors.Join(ErrConfigEnvironmentInvalid, errors.New("environment slug must start with a letter and contain only lowercase letters, numbers, hyphens, or underscores"))
	}
	if len(input.Description) > 500 {
		return errors.Join(ErrConfigEnvironmentInvalid, errors.New("environment description is too long"))
	}
	return nil
}

func lockedConfigEnvironments(ctx context.Context, tx *gorm.DB) ([]ConfigEnvironment, error) {
	environments := make([]ConfigEnvironment, 0)
	if err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Order("id ASC").
		Find(&environments).Error; err != nil {
		return nil, errors.Join(ErrConfigStorage, err)
	}
	return environments, nil
}

func validateEnvironmentParent(environments []ConfigEnvironment, environmentID, parentID uint) error {
	if parentID == 0 {
		return nil
	}
	if parentID == environmentID {
		return errors.Join(ErrConfigEnvironmentInvalid, errors.New("an environment cannot inherit from itself"))
	}
	byID := make(map[uint]ConfigEnvironment, len(environments))
	for _, environment := range environments {
		byID[environment.ID] = environment
	}
	parent, exists := byID[parentID]
	if !exists {
		return errors.Join(ErrConfigEnvironmentInvalid, errors.New("parent environment does not exist"))
	}
	visited := map[uint]struct{}{environmentID: {}}
	for depth := 1; ; depth++ {
		if depth > maxEnvironmentDepth {
			return errors.Join(ErrConfigEnvironmentInvalid, errors.New("environment inheritance is too deep"))
		}
		if _, duplicate := visited[parent.ID]; duplicate {
			return errors.Join(ErrConfigEnvironmentInvalid, errors.New("environment inheritance cycle detected"))
		}
		visited[parent.ID] = struct{}{}
		if parent.ParentID == 0 {
			return nil
		}
		parent, exists = byID[parent.ParentID]
		if !exists {
			return errors.Join(ErrConfigEnvironmentInvalid, errors.New("parent environment does not exist"))
		}
	}
}

func configEnvironmentByID(environments []ConfigEnvironment, id uint) (ConfigEnvironment, bool) {
	for _, environment := range environments {
		if environment.ID == id {
			return environment, true
		}
	}
	return ConfigEnvironment{}, false
}

func environmentSlugExists(environments []ConfigEnvironment, slug string, exceptID uint) bool {
	for _, environment := range environments {
		if environment.ID != exceptID && environment.Slug == slug {
			return true
		}
	}
	return false
}

func uniqueSortedIDs(ids []uint) []uint {
	unique := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id > 0 {
			unique[id] = struct{}{}
		}
	}
	result := make([]uint, 0, len(unique))
	for id := range unique {
		result = append(result, id)
	}
	slices.Sort(result)
	return result
}

func newConfigBatchID() (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", errors.Join(ErrConfigStorage, err)
	}
	return hex.EncodeToString(random), nil
}
