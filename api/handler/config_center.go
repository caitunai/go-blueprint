package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/caitunai/go-blueprint/api/base"
	"github.com/caitunai/go-blueprint/db"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

const configCenterEntry = "src/config-center/main.js"

const (
	keyConfigEnvironment = "environment"
	keyInheritanceChain  = "inheritance_chain"
)

var errConfigInheritance = errors.New("resolve inherited configuration failed")

type configEnvironmentForm struct {
	Name        string `json:"name" binding:"required"`
	Slug        string `json:"slug" binding:"required"`
	Description string `json:"description"`
	ParentID    uint   `json:"parent_id"`
}

type configDraftForm struct {
	Config json.RawMessage `json:"config" binding:"required"`
}

type configPublishForm struct {
	EnvironmentIDs []uint `json:"environment_ids" binding:"required,min=1,max=100"`
}

func ConfigCenterPage(c *base.Context) {
	cssFiles, jsFiles := c.GetCSSJsFiles(configCenterEntry)
	assetMode := viper.GetString("ui.assetMode")
	if assetMode == "" {
		assetMode = "embedded"
	}
	c.View("config-center.index", gin.H{
		"title":       "配置中心",
		"use_vite":    assetMode == "vite",
		"vite_origin": strings.TrimRight(viper.GetString("ui.viteDevOrigin"), "/"),
		"entry":       configCenterEntry,
		"css_files":   cssFiles,
		"js_files":    jsFiles,
	})
}

func ConfigCenterControl(r *base.Router) {
	r.GET("/environments", listConfigEnvironments)
	r.POST("/environments", createConfigEnvironment)
	r.PUT("/environments/:id", updateConfigEnvironment)
	r.DELETE("/environments/:id", deleteConfigEnvironment)
	r.GET("/environments/:id/config", getConfigDraft)
	r.PUT("/environments/:id/config", saveConfigDraft)
	r.GET("/environments/:id/final", getFinalConfig)
	r.GET("/environments/:id/published", getPublishedConfig)
	r.POST("/publish", publishConfigs)
}

func listConfigEnvironments(c *base.Context) {
	environments, err := db.ListConfigEnvironments(c.Request.Context())
	if err != nil {
		respondConfigError(c, err)
		return
	}
	c.Success(gin.H{"environments": environments})
}

func createConfigEnvironment(c *base.Context) {
	form := &configEnvironmentForm{}
	if err := c.ShouldBindJSON(form); err != nil {
		c.ErrorForm("环境信息格式不正确", gin.H{KeyError: err.Error()})
		return
	}
	environment, err := db.CreateConfigEnvironment(c.Request.Context(), configEnvironmentInput(form))
	if err != nil {
		respondConfigError(c, err)
		return
	}
	log.Ctx(c.Request.Context()).Info().Uint("environment_id", environment.ID).Msg("config environment created")
	c.Success(gin.H{keyConfigEnvironment: environment})
}

func updateConfigEnvironment(c *base.Context) {
	id, ok := configEnvironmentID(c)
	if !ok {
		return
	}
	form := &configEnvironmentForm{}
	if err := c.ShouldBindJSON(form); err != nil {
		c.ErrorForm("环境信息格式不正确", gin.H{KeyError: err.Error()})
		return
	}
	environment, err := db.UpdateConfigEnvironment(c.Request.Context(), id, configEnvironmentInput(form))
	if err != nil {
		respondConfigError(c, err)
		return
	}
	log.Ctx(c.Request.Context()).Info().Uint("environment_id", id).Msg("config environment updated")
	c.Success(gin.H{keyConfigEnvironment: environment})
}

func deleteConfigEnvironment(c *base.Context) {
	id, ok := configEnvironmentID(c)
	if !ok {
		return
	}
	if err := db.DeleteConfigEnvironment(c.Request.Context(), id); err != nil {
		respondConfigError(c, err)
		return
	}
	log.Ctx(c.Request.Context()).Info().Uint("environment_id", id).Msg("config environment deleted")
	c.Success(gin.H{"id": id})
}

func getConfigDraft(c *base.Context) {
	id, ok := configEnvironmentID(c)
	if !ok {
		return
	}
	resolved, draft, err := db.GetConfigDraft(c.Request.Context(), id)
	if err != nil {
		respondConfigError(c, err)
		return
	}
	inherited, err := inheritedConfig(c, resolved.Environment.ParentID)
	if err != nil {
		respondConfigError(c, err)
		return
	}
	c.Success(gin.H{
		keyConfigEnvironment: resolved.Environment,
		keyInheritanceChain:  resolved.Chain,
		"draft":              draft,
		"inherited":          inherited,
		"final":              resolved.Config,
	})
}

func saveConfigDraft(c *base.Context) {
	id, ok := configEnvironmentID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, db.MaxConfigBytes+1024)
	form := &configDraftForm{}
	if err := c.ShouldBindJSON(form); err != nil {
		c.ErrorForm("配置内容格式不正确", gin.H{KeyError: err.Error()})
		return
	}
	resolved, err := db.SaveConfigDraft(c.Request.Context(), id, form.Config)
	if err != nil {
		respondConfigError(c, err)
		return
	}
	log.Ctx(c.Request.Context()).Info().Uint("environment_id", id).Msg("config draft saved")
	inherited, err := inheritedConfig(c, resolved.Environment.ParentID)
	if err != nil {
		respondConfigError(c, err)
		return
	}
	c.Success(gin.H{
		keyConfigEnvironment: resolved.Environment,
		keyInheritanceChain:  resolved.Chain,
		"inherited":          inherited,
		"final":              resolved.Config,
	})
}

func getFinalConfig(c *base.Context) {
	id, ok := configEnvironmentID(c)
	if !ok {
		return
	}
	resolved, err := db.ResolveConfigDraft(c.Request.Context(), id)
	if err != nil {
		respondConfigError(c, err)
		return
	}
	c.Success(gin.H{
		keyConfigEnvironment: resolved.Environment,
		keyInheritanceChain:  resolved.Chain,
		"config":             resolved.Config,
	})
}

func getPublishedConfig(c *base.Context) {
	id, ok := configEnvironmentID(c)
	if !ok {
		return
	}
	published, err := db.LatestPublishedConfig(c.Request.Context(), id)
	if err != nil {
		respondConfigError(c, err)
		return
	}
	c.Success(gin.H{"release": published})
}

func publishConfigs(c *base.Context) {
	form := &configPublishForm{}
	if err := c.ShouldBindJSON(form); err != nil {
		c.ErrorForm("请选择需要发布的环境", gin.H{KeyError: err.Error()})
		return
	}
	result, err := db.PublishConfigs(c.Request.Context(), form.EnvironmentIDs)
	if err != nil {
		respondConfigError(c, err)
		return
	}
	log.Ctx(c.Request.Context()).Info().
		Str("batch_id", result.BatchID).
		Int("environment_count", len(result.Releases)).
		Msg("config environments published")
	c.Success(gin.H{"publication": result})
}

func configEnvironmentID(c *base.Context) (uint, bool) {
	parsed, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize)
	if err != nil || parsed == 0 {
		c.ErrorForm("环境 ID 不正确", gin.H{})
		return 0, false
	}
	return uint(parsed), true
}

func configEnvironmentInput(form *configEnvironmentForm) db.ConfigEnvironmentInput {
	return db.ConfigEnvironmentInput{
		Name:        form.Name,
		Slug:        form.Slug,
		Description: form.Description,
		ParentID:    form.ParentID,
	}
}

func inheritedConfig(c *base.Context, parentID uint) (map[string]any, error) {
	if parentID == 0 {
		return make(map[string]any), nil
	}
	resolved, err := db.ResolveConfigDraft(c.Request.Context(), parentID)
	if err != nil {
		return nil, errors.Join(errConfigInheritance, err)
	}
	return resolved.Config, nil
}

func respondConfigError(c *base.Context, err error) {
	switch {
	case errors.Is(err, db.ErrConfigEnvironmentNotFound):
		c.NotFound("配置环境不存在", gin.H{})
	case errors.Is(err, db.ErrConfigReleaseNotFound):
		c.NotFound("该环境尚未发布", gin.H{})
	case errors.Is(err, db.ErrConfigEnvironmentConflict):
		c.Conflict("环境标识已存在", gin.H{})
	case errors.Is(err, db.ErrConfigEnvironmentInUse):
		c.Conflict("请先调整子环境的继承关系", gin.H{})
	case errors.Is(err, db.ErrConfigInvalid):
		c.ErrorForm("配置内容不合法，请检查字段类型和结构", gin.H{"reason": err.Error()})
	case errors.Is(err, db.ErrConfigEnvironmentInvalid):
		c.ErrorForm("环境信息或继承关系不合法", gin.H{"reason": err.Error()})
	default:
		log.Ctx(c.Request.Context()).Error().Err(err).Msg("config center request failed")
		c.ErrorMessage("配置系统暂时不可用")
	}
}
