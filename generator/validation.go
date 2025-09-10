package generator

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
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

	return nil
}

func (v *InputValidator) ValidateTableName(tableName string) error {
	if tableName == "" {
		return fmt.Errorf("table name cannot be empty")
	}

	validSQLIdentifier := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	if !validSQLIdentifier.MatchString(tableName) {
		return fmt.Errorf(
			"table name '%s' must be a valid SQL identifier (letters, numbers, underscore only)",
			tableName,
		)
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
