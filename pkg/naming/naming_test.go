package naming

import "testing"

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "single word", input: "User", expected: "user"},
		{name: "multi word", input: "CompanyAccount", expected: "company_account"},
		{name: "acronym handling", input: "APIKey", expected: "api_key"},
		{name: "long phrase", input: "CompanyIntelligenceReport", expected: "company_intelligence_report"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToSnakeCase(tt.input)
			if got != tt.expected {
				t.Fatalf("ToSnakeCase(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDeriveTableName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "singular", input: "User", expected: "users"},
		{name: "already plural", input: "CompanyAccounts", expected: "company_accounts"},
		{name: "complex singular", input: "CompanyIntelligenceReport", expected: "company_intelligence_reports"},
		{name: "complex plural", input: "CompanyIntelligenceReports", expected: "company_intelligence_reports"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveTableName(tt.input)
			if got != tt.expected {
				t.Fatalf("DeriveTableName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
