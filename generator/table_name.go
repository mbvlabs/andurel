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

func ResolveTableName(modelsDir, resourceName string) string {
	tableName, _ := ResolveTableNameWithFlag(modelsDir, resourceName)
	return tableName
}

func ResolveTableNameWithFlag(modelsDir, resourceName string) (string, bool) {
	modelPath := BuildModelPath(modelsDir, resourceName)
	if overriddenTableName, found := ExtractTableNameOverride(modelPath, resourceName); found {
		return overriddenTableName, true
	}
	return naming.DeriveTableName(resourceName), false
}
