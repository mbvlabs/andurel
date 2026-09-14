package generator

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/internal/naming"
)

// ExtractTableNameOverride reads the table name from the andurel:table
// comment on a generated model file.
func ExtractTableNameOverride(modelPath string, resourceName string) (string, bool) {
	content, err := os.ReadFile(modelPath)
	if err != nil {
		return "", false
	}
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if after, ok := strings.CutPrefix(line, "// andurel:table "); ok {
			table := strings.TrimSpace(after)
			if table != "" {
				return table, true
			}
		}
	}

	return "", false
}

// BuildModelPath performs build model path.
func BuildModelPath(modelsDir, resourceName string) string {
	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3)
	modelFileName.WriteString(naming.ToSnakeCase(resourceName))
	modelFileName.WriteString(".go")
	return filepath.Join(modelsDir, modelFileName.String())
}

// ResolveTableName resolves table name.
func ResolveTableName(modelsDir, resourceName string) string {
	tableName, _ := ResolveTableNameWithFlag(modelsDir, resourceName)
	return tableName
}

// ResolveTableNameWithFlag resolves table name with flag.
func ResolveTableNameWithFlag(modelsDir, resourceName string) (string, bool) {
	modelPath := BuildModelPath(modelsDir, resourceName)
	if tableName, found := ExtractTableNameOverride(modelPath, resourceName); found {
		derived := naming.DeriveTableName(resourceName)
		return tableName, tableName != derived
	}
	return naming.DeriveTableName(resourceName), false
}
