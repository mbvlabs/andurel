package generator

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/pkg/naming"
)

func ExtractTableNameOverride(modelPath string, resourceName string) (string, bool) {
	file, err := os.Open(modelPath)
	if err != nil {
		return "", false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	marker := fmt.Sprintf("// %s_MODEL_TABLE_NAME:", strings.ToUpper(resourceName))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, marker) {
			tableName := strings.TrimSpace(strings.TrimPrefix(line, marker))
			if tableName != "" {
				return tableName, true
			}
		}

		if strings.HasPrefix(line, "package ") {
			continue
		}

		if line != "" && !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "import") {
			break
		}
	}

	return "", false
}

func BuildModelPath(modelsDir, resourceName string) string {
	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3)
	modelFileName.WriteString(naming.ToSnakeCase(resourceName))
	modelFileName.WriteString(".go")
	return filepath.Join(modelsDir, modelFileName.String())
}

func ResolveTableName(modelsDir, queriesDir, resourceName string) string {
	tableName, _ := ResolveTableNameWithFlag(modelsDir, queriesDir, resourceName)
	return tableName
}

func ResolveTableNameWithFlag(modelsDir, queriesDir, resourceName string) (string, bool) {
	modelPath := BuildModelPath(modelsDir, resourceName)
	if overriddenTableName, found := ExtractTableNameOverride(modelPath, resourceName); found {
		derived := naming.DeriveTableName(resourceName)
		return overriddenTableName, overriddenTableName != derived
	}

	if tableName, found := resolveTableNameFromQueries(queriesDir, resourceName); found {
		derived := naming.DeriveTableName(resourceName)
		return tableName, tableName != derived
	}
	return naming.DeriveTableName(resourceName), false
}

func resolveTableNameFromQueries(queriesDir, resourceName string) (string, bool) {
	if queriesDir == "" {
		return "", false
	}

	entries, err := os.ReadDir(queriesDir)
	if err != nil {
		return "", false
	}

	needle := fmt.Sprintf("-- name: Query%sByID", resourceName)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		path := filepath.Join(queriesDir, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		found := false
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), needle) {
				found = true
				break
			}
		}
		file.Close()

		if found {
			return strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), true
		}
	}

	return "", false
}
