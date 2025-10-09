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

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "single word", input: "users", expected: "users"},
		{name: "two words", input: "admin_users", expected: "adminUsers"},
		{name: "three words", input: "product_categories", expected: "productCategories"},
		{name: "many words", input: "user_profile_settings", expected: "userProfileSettings"},
		{name: "empty string", input: "", expected: ""},
		{name: "already camelCase", input: "adminUsers", expected: "adminusers"},
		{name: "single char parts", input: "a_b_c", expected: "aBC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToCamelCase(tt.input)
			if got != tt.expected {
				t.Fatalf("ToCamelCase(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestToLowerCamelCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "single word", input: "User", expected: "user"},
		{name: "two words", input: "NewUser", expected: "newUser"},
		{name: "three words", input: "AdminUser", expected: "adminUser"},
		{name: "many words", input: "UserProfileSettings", expected: "userProfileSettings"},
		{name: "empty string", input: "", expected: ""},
		{name: "already camelCase", input: "user", expected: "user"},
		{name: "single char", input: "U", expected: "u"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToLowerCamelCase(tt.input)
			if got != tt.expected {
				t.Fatalf("ToLowerCamelCase(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
