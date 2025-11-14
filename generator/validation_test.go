package generator

import (
	"bytes"
	"io"
	"os"
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
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := validator.ValidateTableNameOverride(tt.resourceName, tt.tableOverride)

			w.Close()
			os.Stdout = old

			var buf bytes.Buffer
			io.Copy(&buf, r)
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
