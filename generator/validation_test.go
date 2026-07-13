package generator

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestValidateTableName(t *testing.T) {
	validator := NewInputValidator()

	valid := []string{
		"users",
		"company_accounts",
		"company_intelligence_reports",
	}

	for _, tableName := range valid {
		if err := validator.ValidateTableName(tableName); err != nil {
			t.Fatalf("ValidateTableName(%q) returned error: %v", tableName, err)
		}
	}

	invalid := map[string]string{
		"Users":           "must reject uppercase table names",
		"companyAccounts": "must reject camelCase table names",
		"company_account": "must reject singular snake case table names",
		"comp:accounts":   "must reject invalid characters",
	}

	for tableName, msg := range invalid {
		if err := validator.ValidateTableName(tableName); err == nil {
			t.Fatalf("ValidateTableName(%q) succeeded but should fail: %s", tableName, msg)
		}
	}
}

func TestValidateResourceName(t *testing.T) {
	validator := NewInputValidator()

	valid := []string{
		"User",
		"CompanyAccount",
		"CompanyAccounts",
		"CompanyIntelligenceReport",
		"CompanyIntelligenceReports",
		"StatusReport",
		"AnalysisReport",
	}

	for _, resource := range valid {
		if err := validator.ValidateResourceName(resource); err != nil {
			t.Fatalf("ValidateResourceName(%q) returned error: %v", resource, err)
		}
	}

	invalid := map[string]string{
		"Users":           "single-word plurals must be rejected",
		"UsersConfuser":   "first word must be singular",
		"CompaniesReport": "intermediate segment must be singular",
	}

	for resource, reason := range invalid {
		if err := validator.ValidateResourceName(resource); err == nil {
			t.Fatalf("ValidateResourceName(%q) succeeded but should fail: %s", resource, reason)
		}
	}
}

func TestValidateTableNameOverride(t *testing.T) {
	validator := NewInputValidator()

	tests := []struct {
		name          string
		resourceName  string
		tableOverride string
		wantError     bool
		wantWarning   bool
	}{
		{
			name:          "valid override with different table name",
			resourceName:  "User",
			tableOverride: "accounts",
			wantError:     false,
			wantWarning:   true,
		},
		{
			name:          "valid override matching convention",
			resourceName:  "User",
			tableOverride: "users",
			wantError:     false,
			wantWarning:   false,
		},
		{
			name:          "valid override with underscores",
			resourceName:  "CompanyAccount",
			tableOverride: "legacy_accounts",
			wantError:     false,
			wantWarning:   true,
		},
		{
			name:          "invalid override - uppercase",
			resourceName:  "User",
			tableOverride: "Accounts",
			wantError:     true,
			wantWarning:   false,
		},
		{
			name:          "invalid override - camelCase",
			resourceName:  "User",
			tableOverride: "userAccounts",
			wantError:     true,
			wantWarning:   false,
		},
		{
			name:          "invalid override - empty",
			resourceName:  "User",
			tableOverride: "",
			wantError:     true,
			wantWarning:   false,
		},
		{
			name:          "invalid override - reserved keyword",
			resourceName:  "User",
			tableOverride: "select",
			wantError:     true,
			wantWarning:   false,
		},
		{
			name:          "invalid override - too long",
			resourceName:  "User",
			tableOverride: "this_is_a_very_long_table_name_that_exceeds_the_postgresql_limit_of_63_characters_which_will_cause_an_error",
			wantError:     true,
			wantWarning:   false,
		},
		{
			name:          "valid singular override with warning",
			resourceName:  "User",
			tableOverride: "account",
			wantError:     false,
			wantWarning:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := os.Stdout
			r, w, pipeErr := os.Pipe()
			if pipeErr != nil {
				t.Fatalf("create output pipe: %v", pipeErr)
			}
			os.Stdout = w

			err := validator.ValidateTableNameOverride(tt.resourceName, tt.tableOverride)

			if closeErr := w.Close(); closeErr != nil {
				t.Fatalf("close output writer: %v", closeErr)
			}
			os.Stdout = old

			var buf bytes.Buffer
			if _, copyErr := io.Copy(&buf, r); copyErr != nil {
				t.Fatalf("read warning output: %v", copyErr)
			}
			if closeErr := r.Close(); closeErr != nil {
				t.Fatalf("close output reader: %v", closeErr)
			}
			output := buf.String()

			if tt.wantError && err == nil {
				t.Errorf("ValidateTableNameOverride() expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("ValidateTableNameOverride() unexpected error: %v", err)
			}

			if tt.wantWarning && output == "" {
				t.Errorf("ValidateTableNameOverride() expected warning output but got none")
			}
			if !tt.wantWarning && !tt.wantError && output != "" {
				t.Errorf("ValidateTableNameOverride() unexpected warning output: %s", output)
			}
		})
	}
}

func TestInputValidatorRemainingValidationBranches(t *testing.T) {
	validator := NewInputValidator()
	for _, resource := range []string{"", "user", "User-Account", "9User"} {
		if err := validator.ValidateResourceName(resource); err == nil {
			t.Fatalf("resource %q should be rejected", resource)
		}
	}

	for _, table := range []string{"", "account", "select", strings.Repeat("a", 64)} {
		if err := validator.ValidateTableName(table); err == nil {
			t.Fatalf("table %q should be rejected", table)
		}
	}

	for _, path := range []string{"", "../secret", "safe/../../secret", "/absolute/path"} {
		if err := validator.ValidateFilePath(path); err == nil {
			t.Fatalf("file path %q should be rejected", path)
		}
	}
	for _, path := range []string{"models/user.go", "router/routes.go", "file.go"} {
		if err := validator.ValidateFilePath(path); err != nil {
			t.Fatalf("file path %q should be accepted: %v", path, err)
		}
	}

	for _, module := range []string{"", "example.com/my module", "example.com/app@v2"} {
		if err := validator.ValidateModulePath(module); err == nil {
			t.Fatalf("module %q should be rejected", module)
		}
	}
	for _, module := range []string{"example.com/app", "github.com/org/project/v2", "local_module"} {
		if err := validator.ValidateModulePath(module); err != nil {
			t.Fatalf("module %q should be accepted: %v", module, err)
		}
	}

	checks := []struct {
		resource string
		table    string
		module   string
		want     string
	}{
		{resource: "Users", table: "users", module: "example.com/app", want: "resource name validation failed"},
		{resource: "User", table: "user", module: "example.com/app", want: "table name validation failed"},
		{resource: "User", table: "users", module: "bad module", want: "module path validation failed"},
	}
	for _, check := range checks {
		err := validator.ValidateAll(check.resource, check.table, check.module)
		if err == nil || !strings.Contains(err.Error(), check.want) {
			t.Fatalf("ValidateAll(%q, %q, %q) = %v, want %q", check.resource, check.table, check.module, err, check.want)
		}
	}
	if err := validator.ValidateAll("User", "users", "example.com/app"); err != nil {
		t.Fatalf("valid inputs failed: %v", err)
	}
}

func TestTableOverrideWarningsAreDeduplicatedAndNilSafe(t *testing.T) {
	validator := &InputValidator{}
	if !validator.shouldWarnTableOverride("User", "legacy_users") {
		t.Fatal("first override should warn")
	}
	if validator.shouldWarnTableOverride("user", "legacy_users") {
		t.Fatal("case-insensitive duplicate override should not warn")
	}
	if !(*InputValidator)(nil).shouldWarnTableOverride("User", "legacy_users") {
		t.Fatal("nil validator should conservatively request a warning")
	}
}
