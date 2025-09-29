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
