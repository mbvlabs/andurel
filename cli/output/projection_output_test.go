package output

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestJQWritesDirectJSONValuesFromCommandData(t *testing.T) {
	data := map[string]any{
		"array":   []any{"one", float64(2)},
		"boolean": true,
		"null":    nil,
		"number":  42,
		"object":  map[string]any{"name": "post"},
		"string":  "post",
	}

	for _, name := range []string{"array", "boolean", "null", "number", "object", "string"} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := projectionTestCommand(t, &out, "jq", "."+name)
			if err := OK(cmd, data, "ignored summary"); err != nil {
				t.Fatalf("OK: %v", err)
			}
			goldenPath := filepath.Join("testdata", "projection_output", name+".golden.json")
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read %s: %v", goldenPath, err)
			}
			if !bytes.Equal(out.Bytes(), want) {
				t.Fatalf("direct projection mismatch\ngot:\n%s\nwant:\n%s", out.Bytes(), want)
			}
		})
	}
}

func TestJQRejectsInvalidExpressionsAndMissingSelections(t *testing.T) {
	for _, expression := range []string{"data.name", ".name | length", ".missing", ".name.value"} {
		t.Run(expression, func(t *testing.T) {
			var out bytes.Buffer
			cmd := projectionTestCommand(t, &out, "jq", expression)
			err := OK(cmd, map[string]any{"name": "post"}, "ignored")
			if err == nil {
				t.Fatal("expected projection error")
			}
			var cliErr *CLIError
			if !errors.As(err, &cliErr) || cliErr.Code != CodeUsage {
				t.Fatalf("expected usage CLIError, got %T %v", err, err)
			}
		})
	}
}

func TestIDsOnlyAndCountWriteRawValues(t *testing.T) {
	data := []map[string]any{{"name": "alpha"}, {"name": "beta"}}
	for _, test := range []struct {
		name string
		flag string
		want string
	}{
		{name: "identifiers", flag: "ids-only", want: "alpha\nbeta\n"},
		{name: "count", flag: "count", want: "2\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := projectionTestCommand(t, &out, test.flag, "true")
			if err := OK(cmd, data, "ignored"); err != nil {
				t.Fatalf("OK: %v", err)
			}
			if out.String() != test.want {
				t.Fatalf("output = %q, want %q", out.String(), test.want)
			}
		})
	}
}

func TestProjectionFlagsAreMutuallyExclusive(t *testing.T) {
	combinations := [][]string{
		{"jq", "ids-only"},
		{"jq", "count"},
		{"ids-only", "count"},
		{"jq", "ids-only", "count"},
	}
	for _, combination := range combinations {
		cmd := &cobra.Command{Use: "andurel"}
		RegisterPersistentFlags(cmd)
		for _, flag := range combination {
			value := "true"
			if flag == "jq" {
				value = ".name"
			}
			if err := cmd.PersistentFlags().Set(flag, value); err != nil {
				t.Fatalf("set --%s: %v", flag, err)
			}
		}
		if _, err := ParseOptions(cmd); err == nil {
			t.Fatalf("expected conflict for %v", combination)
		}
	}
}

func TestProjectionCollectionsIdentifiersAndEncodingErrors(t *testing.T) {
	for _, field := range []string{"routes", "items", "results", "names", "extensions", "tools"} {
		items, err := projectionItems(map[string]any{field: []any{"one", "two"}})
		if err != nil || len(items) != 2 {
			t.Fatalf("projectionItems(%s) = %#v, %v", field, items, err)
		}
	}
	for _, value := range []any{"scalar", 42, map[string]any{"other": []any{"one"}}} {
		if _, err := projectionItems(value); err == nil {
			t.Fatalf("expected non-collection %#v to fail", value)
		}
	}

	identifiers := []struct {
		value any
		want  string
		ok    bool
	}{
		{value: "direct", want: "direct", ok: true},
		{value: float64(42.5), want: "42.5", ok: true},
		{value: map[string]any{"id": "id-value"}, want: "id-value", ok: true},
		{value: map[string]any{"name": "name-value"}, want: "name-value", ok: true},
		{value: map[string]any{"variable": "variable-value"}, want: "variable-value", ok: true},
		{value: map[string]any{"path": "/path"}, want: "/path", ok: true},
		{value: map[string]any{"id": ""}, ok: false},
		{value: true, ok: false},
	}
	for _, test := range identifiers {
		got, ok := projectionIdentifier(test.value)
		if got != test.want || ok != test.ok {
			t.Fatalf("projectionIdentifier(%#v) = %q, %v; want %q, %v", test.value, got, ok, test.want, test.ok)
		}
	}

	unsupported := make(chan int)
	if _, err := normalizeJSONValue(unsupported); err == nil {
		t.Fatal("expected unsupported JSON value to fail")
	}
	if _, err := applyJQ(unsupported, ".value"); err == nil {
		t.Fatal("expected jq normalization failure")
	}
	if _, err := projectionItems(unsupported); err == nil {
		t.Fatal("expected collection normalization failure")
	}
	if err := writeIDs(&bytes.Buffer{}, []any{true}); err == nil {
		t.Fatal("expected missing identifier error")
	}
}

func projectionTestCommand(t *testing.T, out *bytes.Buffer, flag, value string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "andurel"}
	RegisterPersistentFlags(cmd)
	cmd.SetOut(out)
	if err := cmd.PersistentFlags().Set(flag, value); err != nil {
		t.Fatalf("set --%s: %v", flag, err)
	}
	return cmd
}
