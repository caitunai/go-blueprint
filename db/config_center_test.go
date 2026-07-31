package db

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

const (
	testBase    = "base"
	testFlags   = "flags"
	testPort    = "port"
	testService = "service"
)

func TestDecodeConfigPreservesSupportedTypes(t *testing.T) {
	t.Parallel()

	config, err := DecodeConfig([]byte(`{
		"object":{"enabled":true},
		"array":["a",2,false],
		"string":"value",
		"bool":true,
		"int":42,
		"float":3.14
	}`))
	if err != nil {
		t.Fatalf("DecodeConfig() error = %v", err)
	}
	if got := config["int"]; got != json.Number("42") {
		t.Fatalf("int = %#v, want json.Number(42)", got)
	}
	if got := config["float"]; got != json.Number("3.14") {
		t.Fatalf("float = %#v, want json.Number(3.14)", got)
	}
}

func TestDecodeConfigRejectsUnsupportedValues(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"null":           `{"value":null}`,
		"non-object":     `[1,2,3]`,
		"empty key":      `{" ":true}`,
		"multiple roots": `{} {}`,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeConfig([]byte(raw))
			if !errors.Is(err, ErrConfigInvalid) {
				t.Fatalf("DecodeConfig() error = %v, want ErrConfigInvalid", err)
			}
		})
	}
}

func TestDecodeConfigRejectsExcessiveDepth(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat(`{"nested":`, maxConfigDepth+1) + `true` + strings.Repeat("}", maxConfigDepth+1)
	_, err := DecodeConfig([]byte(raw))
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("DecodeConfig() error = %v, want ErrConfigInvalid", err)
	}
}

func TestMergeConfigRecursivelyMergesObjectsAndReplacesArrays(t *testing.T) {
	t.Parallel()

	base := map[string]any{
		testService: map[string]any{"host": testBase, testPort: json.Number("80")},
		testFlags:   []any{testBase},
		"keep":      true,
	}
	override := map[string]any{
		testService: map[string]any{testPort: json.Number("443")},
		testFlags:   []any{"child", "replacement"},
	}
	want := map[string]any{
		testService: map[string]any{"host": testBase, testPort: json.Number("443")},
		testFlags:   []any{"child", "replacement"},
		"keep":      true,
	}
	got := mergeConfig(base, override)
	gotJSON, gotErr := json.Marshal(got)
	wantJSON, wantErr := json.Marshal(want)
	if gotErr != nil || wantErr != nil || string(gotJSON) != string(wantJSON) {
		t.Fatalf("mergeConfig() = %#v, want %#v", got, want)
	}
	got.(map[string]any)["service"].(map[string]any)["host"] = "changed"
	if base["service"].(map[string]any)["host"] != testBase {
		t.Fatal("mergeConfig() mutated the base configuration")
	}
}

func TestResolveConfigFromEnvironmentsBuildsRootToLeafChain(t *testing.T) {
	t.Parallel()

	environments := []ConfigEnvironment{
		{ID: 3, ParentID: 2, Name: "prod", DraftConfig: `{"service":{"host":"prod"}}`},
		{ID: 1, Name: testBase, DraftConfig: `{"service":{"host":"base","port":80},"enabled":true}`},
		{ID: 2, ParentID: 1, Name: "region", DraftConfig: `{"service":{"port":8080}}`},
	}
	resolved, err := resolveConfigFromEnvironments(environments, 3)
	if err != nil {
		t.Fatalf("resolveConfigFromEnvironments() error = %v", err)
	}
	if got := []uint{resolved.Chain[0].ID, resolved.Chain[1].ID, resolved.Chain[2].ID}; !slices.Equal(got, []uint{1, 2, 3}) {
		t.Fatalf("chain IDs = %v, want [1 2 3]", got)
	}
	service := resolved.Config[testService].(map[string]any)
	if service["host"] != "prod" || service[testPort] != json.Number("8080") {
		t.Fatalf("resolved service = %#v", service)
	}
}

func TestValidateEnvironmentParentRejectsCycle(t *testing.T) {
	t.Parallel()

	environments := []ConfigEnvironment{
		{ID: 1, ParentID: 0},
		{ID: 2, ParentID: 1},
		{ID: 3, ParentID: 2},
	}
	err := validateEnvironmentParent(environments, 1, 3)
	if !errors.Is(err, ErrConfigEnvironmentInvalid) {
		t.Fatalf("validateEnvironmentParent() error = %v, want ErrConfigEnvironmentInvalid", err)
	}
}

func TestUniqueSortedIDs(t *testing.T) {
	t.Parallel()

	got := uniqueSortedIDs([]uint{4, 0, 2, 4, 1})
	want := []uint{1, 2, 4}
	if !slices.Equal(got, want) {
		t.Fatalf("uniqueSortedIDs() = %v, want %v", got, want)
	}
}
