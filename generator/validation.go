package generator

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/jinzhu/inflection"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type InputValidator struct{}

func NewInputValidator() *InputValidator {
	return &InputValidator{}
}

func (v *InputValidator) ValidateResourceName(resourceName string) error {
	if resourceName == "" {
		return fmt.Errorf("resource name cannot be empty")
	}

	validIdentifier := regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*$`)
	if !validIdentifier.MatchString(resourceName) {
		return fmt.Errorf(
			"resource name '%s' must be a valid Go identifier starting with uppercase letter",
			resourceName,
		)
	}

	snake := naming.ToSnakeCase(resourceName)
	parts := strings.Split(snake, "_")
	if len(parts) == 0 {
		return fmt.Errorf("resource name '%s' could not be parsed", resourceName)
	}

	for idx, part := range parts {
		if part == "" {
			return fmt.Errorf("resource name '%s' contains an empty segment", resourceName)
		}

		singular := inflection.Singular(part)
		if idx == 0 {
			if singular != part {
				return fmt.Errorf(
					"resource name '%s' must start with a singular word; found '%s'",
					resourceName,
					part,
				)
			}
			continue
		}

		if idx < len(parts)-1 && singular != part {
			return fmt.Errorf(
				"resource name '%s' must use singular words before the final segment; found '%s'",
				resourceName,
				part,
			)
		}
	}

	return nil
}

func (v *InputValidator) ValidateTableName(tableName string) error {
	if tableName == "" {
		return fmt.Errorf("table name cannot be empty")
	}

	validSQLIdentifier := regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	if !validSQLIdentifier.MatchString(tableName) {
		return fmt.Errorf(
			"table name '%s' must be snake_case using lowercase letters, numbers, and underscores",
			tableName,
		)
	}

	if inflection.Plural(tableName) != tableName {
		return fmt.Errorf("table name '%s' must be plural snake_case", tableName)
	}

	reservedKeywords := []string{
		"select", "insert", "update", "delete", "drop", "create", "alter",
		"table", "index", "view", "database", "schema", "user", "group",
		"order", "by", "where", "from", "join", "union", "having",
	}

	lowerTableName := strings.ToLower(tableName)
	if slices.Contains(reservedKeywords, lowerTableName) {
		return fmt.Errorf("table name '%s' is a reserved SQL keyword", tableName)
	}

	if len(tableName) > 63 { // PostgreSQL limit
		return fmt.Errorf("table name '%s' is too long (max 63 characters)", tableName)
	}

	return nil
}

func (v *InputValidator) ValidateTableNameOverride(resourceName, tableNameOverride string) error {
	if tableNameOverride == "" {
		return fmt.Errorf("table name override cannot be empty")
	}

	conventionalTableName := naming.DeriveTableName(resourceName)

	validSQLIdentifier := regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	if !validSQLIdentifier.MatchString(tableNameOverride) {
		return fmt.Errorf(
			"table name '%s' must be snake_case using lowercase letters, numbers, and underscores",
			tableNameOverride,
		)
	}

	reservedKeywords := []string{
		"select", "insert", "update", "delete", "drop", "create", "alter",
		"table", "index", "view", "database", "schema", "user", "group",
		"order", "by", "where", "from", "join", "union", "having",
	}

	lowerTableName := strings.ToLower(tableNameOverride)
	if slices.Contains(reservedKeywords, lowerTableName) {
		return fmt.Errorf("table name '%s' is a reserved SQL keyword", tableNameOverride)
	}

	if len(tableNameOverride) > 63 {
		return fmt.Errorf("table name '%s' is too long (max 63 characters)", tableNameOverride)
	}

	if tableNameOverride != conventionalTableName {
		fmt.Printf("⚠️  Using custom table name '%s' instead of conventional '%s'\n", tableNameOverride, conventionalTableName)
		fmt.Printf("⚠️  Ensure migration creates the '%s' table\n", tableNameOverride)
	}

	if inflection.Plural(tableNameOverride) != tableNameOverride {
		fmt.Printf("⚠️  Table name '%s' does not appear to be plural. Convention suggests using plural names.\n", tableNameOverride)
	}

	return nil
}

func (v *InputValidator) ValidateFilePath(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	cleanPath := filepath.Clean(filePath)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("file path '%s' contains path traversal sequences", filePath)
	}

	if filepath.IsAbs(cleanPath) {
		return fmt.Errorf("file path '%s' must be relative to project root", filePath)
	}

	return nil
}

func (v *InputValidator) ValidateModulePath(modulePath string) error {
	if modulePath == "" {
		return fmt.Errorf("module path cannot be empty")
	}

	validModulePath := regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
	if !validModulePath.MatchString(modulePath) {
		return fmt.Errorf("module path '%s' contains invalid characters", modulePath)
	}

	return nil
}

func (v *InputValidator) ValidateAll(resourceName, tableName, modulePath string) error {
	if err := v.ValidateResourceName(resourceName); err != nil {
		return fmt.Errorf("resource name validation failed: %w", err)
	}

	if err := v.ValidateTableName(tableName); err != nil {
		return fmt.Errorf("table name validation failed: %w", err)
	}

	if err := v.ValidateModulePath(modulePath); err != nil {
		return fmt.Errorf("module path validation failed: %w", err)
	}

	return nil
}
