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
	tests := []struct {
		fieldName string
		expected  string
	}{
		{"Price", "faker.RandomInt(100, 10000)"},
		{"Amount", "faker.RandomInt(100, 10000)"},
		{"Count", "faker.RandomInt(1, 100)"},
		{"Quantity", "faker.RandomInt(1, 100)"},
		{"Age", "faker.RandomInt(18, 80)"},
		{"RandomNumber", "faker.RandomInt(1, 1000)"},
	}

	analyzer := NewFieldAnalyzer("postgres")
	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			got := analyzer.intDefault(tt.fieldName)
			if got != tt.expected {
				t.Errorf("intDefault(%s) = %s, want %s", tt.fieldName, got, tt.expected)
			}
		})
	}
}

func TestFieldAnalyzer_AnalyzeField(t *testing.T) {
	analyzer := NewFieldAnalyzer("postgres")

	tests := []struct {
		name          string
		field         models.GeneratedField
		tableName     string
		expectedName  string
		expectedType  string
		expectedIsFK  bool
		expectedIsID  bool
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
				Name: "CategoryID",
				Type: "uuid.UUID",
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
		})
	}
}
