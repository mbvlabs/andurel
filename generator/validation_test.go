package generator

import "testing"

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
