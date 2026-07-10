package layout

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	layouttemplates "github.com/mbvlabs/andurel/layout/templates"
)

func TestGeneratedValidationBehavior(t *testing.T) {
	projectDir := t.TempDir()
	validationDir := filepath.Join(projectDir, "internal", "validation")
	if err := os.MkdirAll(validationDir, 0o755); err != nil {
		t.Fatalf("create validation directory: %v", err)
	}

	templateFiles := map[string]string{
		"framework_elements_validation_validation.tmpl": "validation.go",
		"framework_elements_validation_rules.tmpl":      "rules.go",
		"framework_elements_validation_helpers.tmpl":    "helpers.go",
	}
	for templateName, targetName := range templateFiles {
		renderValidationTemplate(t, templateName, filepath.Join(validationDir, targetName))
	}

	writeValidationFixtureFile(t, filepath.Join(projectDir, "go.mod"), "module validationfixture\n\ngo 1.26.5\n")
	writeValidationFixtureFile(
		t,
		filepath.Join(validationDir, "validation_test.go"),
		generatedValidationTests,
	)

	cmd := exec.Command("go", "test", "./internal/validation")
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOCACHE="+filepath.Join(projectDir, ".gocache"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated validation tests failed: %v\n%s", err, output)
	}
}

func renderValidationTemplate(t *testing.T, templateName, targetPath string) {
	t.Helper()

	content, err := layouttemplates.Files.ReadFile(templateName)
	if err != nil {
		t.Fatalf("read %s: %v", templateName, err)
	}

	tmpl, err := template.New(templateName).Parse(string(content))
	if err != nil {
		t.Fatalf("parse %s: %v", templateName, err)
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, struct{ FrameworkVersion string }{FrameworkVersion: "test"}); err != nil {
		t.Fatalf("render %s: %v", templateName, err)
	}

	writeValidationFixtureFile(t, targetPath, rendered.String())
}

func writeValidationFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

const generatedValidationTests = `package validation

import (
	"database/sql"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestRuleManifestIsIndependentFromErrors(t *testing.T) {
	values := []string{"", "valid", "too long"}
	for _, value := range values {
		b := NewBuilder()
		b.MaxLen("title", value, 5)

		want := []Rule{{
			Code:    "max",
			Message: "must be at most 5 characters",
			Params:  map[string]any{"max": 5},
		}}
		if got := b.Rules()["title"]; !reflect.DeepEqual(got, want) {
			t.Fatalf("MaxLen(%q) rules = %#v, want %#v", value, got, want)
		}

		wantErrors := 0
		if value == "too long" {
			wantErrors = 1
		}
		if got := b.Errors().Len(); got != wantErrors {
			t.Fatalf("MaxLen(%q) errors = %d, want %d", value, got, wantErrors)
		}
	}
}

func TestComposedAndConditionalRuleRegistration(t *testing.T) {
	b := NewBuilder()
	b.RequiredWhen(false, "skipped", "")
	b.RequiredWhen(true, "conditional", "")
	b.RequiredURL("website", "")
	b.LenBetween("title", "", 2, 8)

	if _, exists := b.Rules()["skipped"]; exists {
		t.Fatal("RequiredWhen(false) registered a rule")
	}
	assertRuleCodes(t, b.Rules()["conditional"], "required")
	assertRuleCodes(t, b.Rules()["website"], "required", "url")
	assertRuleCodes(t, b.Rules()["title"], "min", "max")
}

func TestRuleMessagesParamsAndNormalization(t *testing.T) {
	b := NewBuilder()
	b.Required("", "", "custom required")
	b.OneOfWithMessage("status", "invalid", "custom membership", "draft", "published")
	b.AddRule("email", "email", "must be a valid email")

	unknownRules := b.Rules()[UnknownField]
	if len(unknownRules) != 1 || unknownRules[0].Message != "custom required" {
		t.Fatalf("unknown field rules = %#v", unknownRules)
	}
	if got := b.Errors()[0].Message; got != unknownRules[0].Message {
		t.Fatalf("error message = %q, rule message = %q", got, unknownRules[0].Message)
	}

	statusRule := b.Rules()["status"][0]
	if statusRule.Message != "custom membership" {
		t.Fatalf("OneOfWithMessage message = %q", statusRule.Message)
	}
	if got := statusRule.Params["allowed"]; !reflect.DeepEqual(got, []string{"draft", "published"}) {
		t.Fatalf("allowed = %#v", got)
	}
	assertRuleCodes(t, b.Rules()["email"], "email")
}

func TestRulesReturnsShallowSnapshots(t *testing.T) {
	allowed := []string{"draft", "published"}
	params := map[string]any{"allowed": allowed, "label": "original"}
	b := NewBuilder()
	b.AddRuleWithParams("status", "one_of", "invalid", params)

	params["label"] = "changed"
	first := b.Rules()
	if got := first["status"][0].Params["label"]; got != "original" {
		t.Fatalf("registration retained top-level params map: %v", got)
	}

	first["status"][0].Code = "changed"
	first["status"][0].Params["label"] = "snapshot changed"
	first["new"] = []Rule{{Code: "new"}}
	second := b.Rules()
	if got := second["status"][0].Code; got != "one_of" {
		t.Fatalf("snapshot changed stored rule code: %q", got)
	}
	if got := second["status"][0].Params["label"]; got != "original" {
		t.Fatalf("snapshot changed stored params: %v", got)
	}
	if _, exists := second["new"]; exists {
		t.Fatal("snapshot changed stored rules map")
	}

	allowed[0] = "review"
	if got := second["status"][0].Params["allowed"].([]string)[0]; got != "review" {
		t.Fatalf("nested params were deep-copied, got %q", got)
	}
}

func TestRecommendedLengthIsManifestOnly(t *testing.T) {
	b := NewBuilder()
	b.RecommendedLenBetween("title", 20, 60)
	if err := b.Err(); err != nil {
		t.Fatalf("RecommendedLenBetween returned error: %v", err)
	}

	rule := b.Rules()["title"][0]
	if rule.Code != "recommended_length" || rule.Message != "recommended length is between 20 and 60 characters" {
		t.Fatalf("recommended rule = %#v", rule)
	}
	if !reflect.DeepEqual(rule.Params, map[string]any{"min": 20, "max": 60}) {
		t.Fatalf("recommended params = %#v", rule.Params)
	}
}

func TestBuiltInRuleRegistration(t *testing.T) {
	b := NewBuilder()
	b.MinItems("items", nil, 1)
	b.MaxItems("items", nil, 3)
	b.NoBlankItems("items", nil)
	b.TimeBeforeOrEqual("start", time.Time{}, "", time.Time{})
	b.True("accepted", true)

	assertRuleCodes(t, b.Rules()["items"], "min_items", "max_items", "no_blank_items")
	assertRuleCodes(t, b.Rules()["start"], "before_or_equal")
	if got := b.Rules()["start"][0].Params["other"]; got != UnknownField {
		t.Fatalf("other field = %v", got)
	}
	assertRuleCodes(t, b.Rules()["accepted"], "true")
}

func TestIntegerValidation(t *testing.T) {
	type namedInt16 int16
	intValue := 2
	int32Value := int32(2)
	int64Value := int64(2)
	namedValue := namedInt16(2)

	values := []any{
		intValue,
		int32Value,
		int64Value,
		&intValue,
		&int32Value,
		&int64Value,
		namedValue,
		&namedValue,
		sql.NullInt32{Int32: 2, Valid: true},
		&sql.NullInt32{Int32: 2, Valid: true},
		sql.NullInt64{Int64: 2, Valid: true},
		&sql.NullInt64{Int64: 2, Valid: true},
	}

	for _, value := range values {
		b := NewBuilder()
		b.MinInt("count", value, 3)
		b.MaxInt("count", value, 1)
		if got := b.Errors().Len(); got != 2 {
			t.Fatalf("%T errors = %d, want 2", value, got)
		}
		assertRuleCodes(t, b.Rules()["count"], "min", "max")
		if _, ok := b.Rules()["count"][0].Params["min"].(int64); !ok {
			t.Fatalf("%T minimum param is not int64", value)
		}
	}

	boundary := NewBuilder()
	boundary.MinInt("count", int8(3), 3)
	boundary.MaxInt("count", int8(3), 3)
	if err := boundary.Err(); err != nil {
		t.Fatalf("exact integer boundary failed: %v", err)
	}
}

func TestIntegerValidationSkipsMissingAndUnsupportedValues(t *testing.T) {
	var intPtr *int
	var nullInt32Ptr *sql.NullInt32
	values := []any{
		nil,
		intPtr,
		nullInt32Ptr,
		sql.NullInt32{Int32: 5, Valid: false},
		sql.NullInt64{Int64: 5, Valid: false},
		uint(0),
		uint64(100),
		"2",
	}

	for _, value := range values {
		b := NewBuilder()
		b.MinInt("count", value, 3)
		b.MaxInt("count", value, 1)
		if err := b.Err(); err != nil {
			t.Fatalf("%T produced an integer error: %v", value, err)
		}
		assertRuleCodes(t, b.Rules()["count"], "min", "max")
	}
}

func TestNullInt32RequiredSemantics(t *testing.T) {
	invalid := NewBuilder()
	invalid.Required("count", sql.NullInt32{Int32: 5, Valid: false})
	if invalid.Errors().Len() != 1 {
		t.Fatal("invalid NullInt32 with payload was not missing")
	}

	validZero := NewBuilder()
	validZero.Required("count", sql.NullInt32{Int32: 0, Valid: true})
	if err := validZero.Err(); err != nil {
		t.Fatalf("valid zero NullInt32 was missing: %v", err)
	}
	validZero.MinInt("count", sql.NullInt32{Int32: 0, Valid: true}, 1)
	if validZero.Errors().Len() != 1 {
		t.Fatal("valid zero NullInt32 was not range-checked")
	}
}

func TestRulesJSONContract(t *testing.T) {
	b := NewBuilder()
	b.MaxLen("title", "", 60)
	encoded, err := json.Marshal(b.Rules())
	if err != nil {
		t.Fatalf("marshal rules: %v", err)
	}
	want := "{\"title\":[{\"code\":\"max\",\"message\":\"must be at most 60 characters\",\"params\":{\"max\":60}}]}"
	if string(encoded) != want {
		t.Fatalf("rules JSON = %s, want %s", encoded, want)
	}
}

func assertRuleCodes(t *testing.T, rules []Rule, want ...string) {
	t.Helper()
	got := make([]string, len(rules))
	for i, rule := range rules {
		got[i] = rule.Code
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rule codes = %v, want %v", got, want)
	}
}
`

func TestGeneratedValidationTemplatesExposePublicContract(t *testing.T) {
	validation := readGeneratedApplicationTemplate(t, "framework_elements_validation_validation.tmpl")
	rules := readGeneratedApplicationTemplate(t, "framework_elements_validation_rules.tmpl")
	helpers := readGeneratedApplicationTemplate(t, "framework_elements_validation_helpers.tmpl")

	for _, want := range []string{
		"type Rule struct",
		"type Rules map[string][]Rule",
		"func (b *ValidationBuilder) AddRule(",
		"func (b *ValidationBuilder) AddRuleWithParams(",
		"func (b *ValidationBuilder) Rules() Rules",
	} {
		if !strings.Contains(validation, want) {
			t.Errorf("validation template missing %q", want)
		}
	}

	for _, want := range []string{
		"func (b *ValidationBuilder) RecommendedLenBetween(",
		"func (b *ValidationBuilder) MinInt(",
		"func (b *ValidationBuilder) MaxInt(",
	} {
		if !strings.Contains(rules, want) {
			t.Errorf("rules template missing %q", want)
		}
	}

	for _, want := range []string{
		"case *sql.NullInt32:",
		"case sql.NullInt32:",
		"func intValue(",
	} {
		if !strings.Contains(helpers, want) {
			t.Errorf("helpers template missing %q", want)
		}
	}
}
