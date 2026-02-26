package factory

import (
	"testing"

	"github.com/mbvlabs/andurel/generator/models"
)

func TestFieldAnalyzer_StringDefaults(t *testing.T) {
	tests := []struct {
		fieldName string
		expected  string
	}{
		{"Email", "faker.Email()"},
		{"Name", "faker.Name()"},
		{"UserName", "faker.Name()"},
		{"PhoneNumber", "faker.Phonenumber()"},
		{"Description", "faker.Sentence()"},
		{"Title", "faker.Word()"},
		{"City", "faker.GetRealAddress().City"},
		{"Address", "faker.GetRealAddress().Address"},
		{"Country", "faker.GetRealAddress().Country"},
		{"RandomField", "faker.Word()"},
	}

	analyzer := NewFieldAnalyzer("postgres")
	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			got := analyzer.stringDefault(tt.fieldName)
			if got != tt.expected {
				t.Errorf("stringDefault(%s) = %s, want %s", tt.fieldName, got, tt.expected)
			}
		})
	}
}

func TestFieldAnalyzer_IntDefaults(t *testing.T) {
	analyzer := NewFieldAnalyzer("postgres")

	// intDefault now returns a generic random int call for all fields
	expected := "randomInt(1, 1000, 100)"
	got := analyzer.intDefault("anyField")
	if got != expected {
		t.Errorf("intDefault(anyField) = %s, want %s", got, expected)
	}
}

func TestFieldAnalyzer_AnalyzeField(t *testing.T) {
	analyzer := NewFieldAnalyzer("postgres")

	tests := []struct {
		name         string
		field        models.GeneratedField
		tableName    string
		expectedName string
		expectedType string
		expectedIsFK bool
		expectedIsID bool
	}{
		{
			name: "ID field",
			field: models.GeneratedField{
				Name: "ID",
				Type: "uuid.UUID",
			},
			tableName:    "products",
			expectedName: "ID",
			expectedType: "uuid.UUID",
			expectedIsFK: false,
			expectedIsID: true,
		},
		{
			name: "Foreign key field",
			field: models.GeneratedField{
				Name:         "CategoryID",
				Type:         "uuid.UUID",
				IsForeignKey: true,
			},
			tableName:    "products",
			expectedName: "CategoryID",
			expectedType: "uuid.UUID",
			expectedIsFK: true,
			expectedIsID: false,
		},
		{
			name: "Regular string field",
			field: models.GeneratedField{
				Name: "Name",
				Type: "string",
			},
			tableName:    "products",
			expectedName: "Name",
			expectedType: "string",
			expectedIsFK: false,
			expectedIsID: false,
		},
		{
			name: "Snake case table creates CamelCase option name",
			field: models.GeneratedField{
				Name: "TeamID",
				Type: "uuid.UUID",
			},
			tableName:    "team_memberships",
			expectedName: "TeamID",
			expectedType: "uuid.UUID",
			expectedIsFK: false,
			expectedIsID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.AnalyzeField(tt.field, tt.tableName)

			if result.Name != tt.expectedName {
				t.Errorf("Name = %s, want %s", result.Name, tt.expectedName)
			}
			if result.Type != tt.expectedType {
				t.Errorf("Type = %s, want %s", result.Type, tt.expectedType)
			}
			if result.IsFK != tt.expectedIsFK {
				t.Errorf("IsFK = %v, want %v", result.IsFK, tt.expectedIsFK)
			}
			if result.IsID != tt.expectedIsID {
				t.Errorf("IsID = %v, want %v", result.IsID, tt.expectedIsID)
			}
			if tt.tableName == "team_memberships" && result.OptionName != "WithTeamMembershipsTeamID" {
				t.Errorf("OptionName = %s, want %s", result.OptionName, "WithTeamMembershipsTeamID")
			}
		})
	}
}
