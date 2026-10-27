package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
)

const configInspectSource = `package config

import (
	"time"

	"github.com/mbvlabs/andurel/pkg/env"
)

const (
	DefaultEnvironment = "development"
	DefaultTimeout     = 120 * time.Second
	DefaultCount       = 512
	DefaultMaxBytes    = int64(2097152)
)

func New() {
	e := env.New()
	_ = e.String("ENVIRONMENT", DefaultEnvironment)
	_ = e.String("EMPTY", "")
	_ = e.RequiredString("SECRET")
	_ = e.Int("COUNT", DefaultCount)
	_ = e.Int64("MAX_BYTES", DefaultMaxBytes)
	_ = e.Duration("TIMEOUT", DefaultTimeout)
	_ = e.Bool("ON", true)
	_ = e.Strings("LIST", nil)
	_ = e.Int("FIELD", cfg.Max)
}
`

func TestCollectConfigKeys(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "config/app.go", configInspectSource)

	keys, err := collectConfigKeys(root)
	if err != nil {
		t.Fatalf("collect config keys: %v", err)
	}
	if len(keys) != 9 {
		t.Fatalf("config keys = %#v", keys)
	}

	byKey := map[string]configKeyInfo{}
	for _, key := range keys {
		byKey[key.Key] = key
	}

	for _, tc := range []struct {
		key        string
		defaultVal string
		hasDefault bool
		required   bool
		method     string
	}{
		{"ENVIRONMENT", "development", true, false, "String"},
		{"EMPTY", "", true, false, "String"},
		{"SECRET", "", false, true, "RequiredString"},
		{"COUNT", "512", true, false, "Int"},
		{"MAX_BYTES", "2097152", true, false, "Int64"},
		{"TIMEOUT", "2m0s", true, false, "Duration"},
		{"ON", "true", true, false, "Bool"},
		{"LIST", "", true, false, "Strings"},
	} {
		got, ok := byKey[tc.key]
		if !ok {
			t.Fatalf("missing key %s in %#v", tc.key, keys)
		}
		if got.Default != tc.defaultVal || got.HasDefault != tc.hasDefault ||
			got.Required != tc.required || got.Method != tc.method {
			t.Fatalf("%s = %#v, want default %q has %v required %v method %s",
				tc.key, got, tc.defaultVal, tc.hasDefault, tc.required, tc.method)
		}
		if got.SourceFile != "config/app.go" || got.Line == 0 {
			t.Fatalf("%s source = %s:%d", tc.key, got.SourceFile, got.Line)
		}
	}

	field := byKey["FIELD"]
	if field.HasDefault || field.DefaultExpression != "cfg.Max" {
		t.Fatalf("FIELD = %#v, want unresolved default expression cfg.Max", field)
	}
}

func TestConfigInspectionCommandJSON(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "config/app.go", configInspectSource)

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	var out bytes.Buffer
	cmd := newConfigInspectionCommand()
	output.RegisterPersistentFlags(cmd)
	cmd.SetOut(&out)
	_ = cmd.PersistentFlags().Set("json", "true")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect config: %v", err)
	}

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Keys []configKeyInfo `json:"keys"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, out.String())
	}
	if !envelope.OK || len(envelope.Data.Keys) != 9 {
		t.Fatalf("unexpected envelope:\n%s", out.String())
	}
	if !strings.Contains(out.String(), `"key": "ENVIRONMENT"`) {
		t.Fatalf("expected key in JSON:\n%s", out.String())
	}
}

func TestRenderConfigKeysHuman(t *testing.T) {
	var out bytes.Buffer
	err := renderConfigKeysHuman(&out, []configKeyInfo{
		{Key: "SECRET", Method: "RequiredString", Required: true, SourceFile: "config/app.go", Line: 3},
		{Key: "EMPTY", Method: "String", HasDefault: true, SourceFile: "config/app.go", Line: 4},
		{Key: "COUNT", Method: "Int", Default: "512", HasDefault: true, SourceFile: "config/app.go", Line: 5},
		{Key: "FIELD", Method: "Int", DefaultExpression: "cfg.Max", SourceFile: "config/app.go", Line: 6},
	})
	if err != nil {
		t.Fatalf("render config keys: %v", err)
	}
	for _, want := range []string{"SECRET", "required", "EMPTY", `""`, "COUNT", "512", "cfg.Max"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in:\n%s", want, out.String())
		}
	}
}
