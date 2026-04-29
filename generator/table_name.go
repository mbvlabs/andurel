package generator

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/pkg/naming"
)

// ExtractTableNameOverride reads the actual table name from the bun.BaseModel tag
// in the generated entity struct. e.g.: bun.BaseModel `bun:"table:student_feedback"`
func ExtractTableNameOverride(modelPath string, resourceName string) (string, bool) {
	file, err := os.Open(modelPath)
	if err != nil {
		return "", false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "bun.BaseModel") {
			continue
		}
		start := strings.Index(line, `bun:"table:`)
		if start == -1 {
			break
		}
		rest := line[start+len(`bun:"table:`):]
		end := strings.IndexAny(rest, `",`)
		if end == -1 {
			break
		}
		return rest[:end], true
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
	if tableName, found := ExtractTableNameOverride(modelPath, resourceName); found {
		derived := naming.DeriveTableName(resourceName)
		return tableName, tableName != derived
	}
	return naming.DeriveTableName(resourceName), false
}
