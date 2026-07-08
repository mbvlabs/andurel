package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultAgentConfig(t *testing.T) {
	cfg := defaultAgentConfig()

	if cfg.PreferredGeneratorMode != "safe" {
		t.Fatalf("expected safe generator mode, got %q", cfg.PreferredGeneratorMode)
	}
	if cfg.JavaScriptRuntime != "npm" {
		t.Fatalf("expected npm runtime, got %q", cfg.JavaScriptRuntime)
	}
	if cfg.OutputFormat != "human" {
		t.Fatalf("expected human output, got %q", cfg.OutputFormat)
	}
	if cfg.CommonDatabaseCommandOptions["migrate"] != "up" {
		t.Fatalf("expected default migrate option, got %#v", cfg.CommonDatabaseCommandOptions)
	}
	if cfg.Values == nil {
		t.Fatalf("expected initialized values map")
	}
}

func TestConfigPathSelectsProjectUserAndCacheScopes(t *testing.T) {
	rootDir := t.TempDir()
	configHome := t.TempDir()
	cacheHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) {
		return rootDir, nil
	}
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	tests := map[string]string{
		"project": filepath.Join(rootDir, ".andurel", "config.json"),
		"user":    filepath.Join(configHome, "andurel", "config.json"),
		"cache":   filepath.Join(cacheHome, "andurel", "config.json"),
	}
	for scope, expected := range tests {
		path, err := configPath(strings.ToUpper(scope))
		if err != nil {
			t.Fatalf("configPath(%q): %v", scope, err)
		}
		if path != expected {
			t.Fatalf("configPath(%q) = %q, want %q", scope, path, expected)
		}
	}

	if _, err := configPath("workspace"); err == nil {
		t.Fatalf("expected invalid scope error")
	}
}

func TestAgentConfigReadWriteRoundTripAndMissingOptional(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	cfg := agentConfig{
		PreferredGeneratorMode: "overwrite",
		JavaScriptRuntime:      "bun",
		DefaultNamespace:       "Admin",
		OutputFormat:           "json",
		CommonDatabaseCommandOptions: map[string]string{
			"seed": "status",
		},
		Values: map[string]string{
			"alpha": "one",
		},
	}

	if err := writeAgentConfig(path, cfg); err != nil {
		t.Fatalf("writeAgentConfig: %v", err)
	}

	read, err := readAgentConfig(path)
	if err != nil {
		t.Fatalf("readAgentConfig: %v", err)
	}
	if !reflect.DeepEqual(read, cfg) {
		t.Fatalf("round trip mismatch:\n got: %#v\nwant: %#v", read, cfg)
	}

	missing, err := readOptionalAgentConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("readOptionalAgentConfig missing: %v", err)
	}
	if missing.Values == nil || len(missing.Values) != 0 {
		t.Fatalf("expected empty initialized optional config, got %#v", missing)
	}
}

func TestReadAgentConfigInitializesValuesMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"output_format":"json"}`), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	cfg, err := readAgentConfig(path)
	if err != nil {
		t.Fatalf("readAgentConfig: %v", err)
	}
	if cfg.Values == nil {
		t.Fatalf("expected values map to be initialized")
	}
}

func TestMergeAgentConfigsPrecedenceAndCopiesMaps(t *testing.T) {
	user := agentConfig{
		PreferredGeneratorMode: "safe",
		OutputFormat:           "human",
		CommonDatabaseCommandOptions: map[string]string{
			"migrate": "up",
		},
		Values: map[string]string{
			"shared": "user",
			"user":   "kept",
		},
	}
	cache := agentConfig{
		JavaScriptRuntime: "pnpm",
		Values: map[string]string{
			"shared": "cache",
		},
	}
	project := agentConfig{
		OutputFormat: "agent",
		CommonDatabaseCommandOptions: map[string]string{
			"migrate": "status",
		},
		Values: map[string]string{
			"project": "wins",
		},
	}

	merged := mergeAgentConfigs(user, cache, project)
	if merged.PreferredGeneratorMode != "safe" || merged.JavaScriptRuntime != "pnpm" || merged.OutputFormat != "agent" {
		t.Fatalf("unexpected scalar merge: %#v", merged)
	}
	if merged.CommonDatabaseCommandOptions["migrate"] != "status" {
		t.Fatalf("expected project database option to win, got %#v", merged.CommonDatabaseCommandOptions)
	}
	if merged.Values["shared"] != "cache" || merged.Values["user"] != "kept" || merged.Values["project"] != "wins" {
		t.Fatalf("unexpected values merge: %#v", merged.Values)
	}

	project.Values["project"] = "mutated"
	project.CommonDatabaseCommandOptions["migrate"] = "down"
	if merged.Values["project"] != "wins" || merged.CommonDatabaseCommandOptions["migrate"] != "status" {
		t.Fatalf("merge should copy maps, got %#v / %#v", merged.Values, merged.CommonDatabaseCommandOptions)
	}
}

func TestSetUnsetAndSortedConfigKeys(t *testing.T) {
	cfg := agentConfig{}

	setConfigValue(&cfg, "preferred_generator_mode", "overwrite")
	setConfigValue(&cfg, "javascript_runtime", "yarn")
	setConfigValue(&cfg, "default_namespace", "Admin")
	setConfigValue(&cfg, "output_format", "markdown")
	setConfigValue(&cfg, "zeta", "last")
	setConfigValue(&cfg, "alpha", "first")

	if cfg.PreferredGeneratorMode != "overwrite" || cfg.JavaScriptRuntime != "yarn" ||
		cfg.DefaultNamespace != "Admin" || cfg.OutputFormat != "markdown" {
		t.Fatalf("known keys not set: %#v", cfg)
	}
	if cfg.Values["alpha"] != "first" || cfg.Values["zeta"] != "last" {
		t.Fatalf("custom values not set: %#v", cfg.Values)
	}

	unsetConfigValue(&cfg, "output_format")
	unsetConfigValue(&cfg, "zeta")
	if cfg.OutputFormat != "" {
		t.Fatalf("expected output format to be unset, got %q", cfg.OutputFormat)
	}
	if _, ok := cfg.Values["zeta"]; ok {
		t.Fatalf("expected custom key to be removed")
	}

	keys := sortedConfigKeys(cfg.Values)
	if !reflect.DeepEqual(keys, []string{"alpha"}) {
		t.Fatalf("sortedConfigKeys = %#v", keys)
	}
}
