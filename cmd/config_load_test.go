package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestMergePublishedConfigurationOverridesBusinessConfigAndProtectsBootstrap(t *testing.T) {
	t.Parallel()

	target := viper.New()
	target.SetConfigType("toml")
	if err := target.ReadConfig(strings.NewReader(`
[db]
host="bootstrap-db"
[configcrypt]
keyring="/external/keys.json"
[configcenter]
enabled=true
[configload]
source="database"
[redis]
host="bootstrap-redis"
`)); err != nil {
		t.Fatalf("ReadConfig() error = %v", err)
	}
	remote := map[string]any{
		"DB":           map[string]any{"host": "remote-db"},
		"configcrypt":  map[string]any{"keyring": "/remote/keys.json"},
		"configcenter": map[string]any{"enabled": false},
		"configload":   map[string]any{"source": "http"},
		"redis":        map[string]any{"host": "remote-redis", "database": json.Number("3")},
		"feature":      map[string]any{"Enabled": true},
	}

	if err := mergePublishedConfiguration(target, remote); err != nil {
		t.Fatalf("mergePublishedConfiguration() error = %v", err)
	}
	if target.GetString("db.host") != "bootstrap-db" ||
		target.GetString("configcrypt.keyring") != "/external/keys.json" ||
		!target.GetBool("configcenter.enabled") || target.GetString("configload.source") != "database" {
		t.Fatal("mergePublishedConfiguration() replaced protected bootstrap settings")
	}
	if target.GetString("redis.host") != "remote-redis" || target.GetInt("redis.database") != 3 ||
		!target.GetBool("feature.enabled") {
		t.Fatal("mergePublishedConfiguration() did not merge remote business settings")
	}
	if remote["feature"].(map[string]any)["Enabled"] != true {
		t.Fatal("mergePublishedConfiguration() mutated the source configuration")
	}
}
