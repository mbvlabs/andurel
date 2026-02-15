package migrations

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func DiscoverMigrations(dirs []string) ([]Migration, error) {
	var migrations []Migration

	for _, dir := range dirs {
		dirMigrations, err := discoverMigrationsInDir(dir)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to discover migrations in %s: %w",
				dir,
				err,
			)
		}
		migrations = append(migrations, dirMigrations...)
	}

	sort.Slice(migrations, func(i, j int) bool {
		return filepath.Base(
			migrations[i].FilePath,
		) < filepath.Base(
			migrations[j].FilePath,
		)
	})

	return migrations, nil
}

func discoverMigrationsInDir(dir string) ([]Migration, error) {
	var migrations []Migration

	err := filepath.WalkDir(
		dir,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() || !strings.HasSuffix(path, ".sql") {
				return nil
			}

			if IsDownMigration(filepath.Base(path)) {
				return nil
			}

			migration, err := ParseMigration(path)
			if err != nil {
				return fmt.Errorf("failed to parse migration %s: %w", path, err)
			}

			migrations = append(migrations, *migration)
			return nil
		},
	)

	return migrations, err
}

func ParseMigration(filePath string) (*Migration, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migration file: %w", err)
	}

	filename := filepath.Base(filePath)
	sequence, name, err := parseFilename(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to parse filename %s: %w", filename, err)
	}

	format := detectMigrationFormat(string(content))
	upSQL := RemoveRollbackStatements(string(content), format)
	downSQL := extractDownSQL(string(content), format)

	migration := &Migration{
		FilePath:   filePath,
		Sequence:   sequence,
		Name:       name,
		Format:     format,
		UpSQL:      upSQL,
		DownSQL:    downSQL,
		Statements: parseStatements(upSQL),
	}

	return migration, nil
}

func RemoveRollbackStatements(content string, format MigrationFormat) string {
	switch format {
	case Goose:
		return extractUpSQLGoose(content)
	default:
		return content
	}
}

func IsDownMigration(filename string) bool {
	return strings.Contains(filename, ".down.") ||
		strings.HasSuffix(filename, ".down.sql")
}

func parseFilename(filename string) (sequence int, name string, err error) {
	re, err := regexp.Compile(`^(\d+)_(.+)\.sql$`)
	if err != nil {
		return 0, "", fmt.Errorf("failed to compile filename pattern: %w", err)
	}
	matches := re.FindStringSubmatch(filename)

	if len(matches) != 3 {
		return 0, "", fmt.Errorf(
			"invalid migration filename format: %s",
			filename,
		)
	}

	sequence, err = strconv.Atoi(matches[1])
	if err != nil {
		return 0, "", fmt.Errorf(
			"invalid sequence number in filename: %s",
			matches[1],
		)
	}

	name = matches[2]
	return sequence, name, nil
}

func detectMigrationFormat(content string) MigrationFormat {
	if strings.Contains(content, "-- +goose Up") ||
		strings.Contains(content, "-- +goose Down") {
		return Goose
	}

	return Goose
}

func extractUpSQLGoose(content string) string {
	lines := strings.Split(content, "\n")
	var upLines []string
	inUp := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- +goose Up") {
			inUp = true
			continue
		}
		if strings.HasPrefix(trimmed, "-- +goose Down") {
			break
		}
		if inUp && !strings.HasPrefix(trimmed, "-- +goose StatementBegin") &&
			!strings.HasPrefix(trimmed, "-- +goose StatementEnd") {
			if !strings.HasPrefix(trimmed, "SELECT ") ||
				!strings.Contains(trimmed, "SQL query") {
				upLines = append(upLines, line)
			}
		}
	}

	return strings.Join(upLines, "\n")
}

func extractDownSQL(content string, format MigrationFormat) string {
	switch format {
	case Goose:
		return extractDownSQLGoose(content)
	default:
		return ""
	}
}

func extractDownSQLGoose(content string) string {
	lines := strings.Split(content, "\n")
	var downLines []string
	inDown := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- +goose Down") {
			inDown = true
			continue
		}
		if inDown && !strings.HasPrefix(trimmed, "-- +goose StatementBegin") &&
			!strings.HasPrefix(trimmed, "-- +goose StatementEnd") {
			if !strings.HasPrefix(trimmed, "SELECT ") ||
				!strings.Contains(trimmed, "SQL query") {
				downLines = append(downLines, line)
			}
		}
	}

	return strings.Join(downLines, "\n")
}

func parseStatements(sql string) []string {
	var statements []string

	lines := strings.Split(sql, "\n")
	var currentStatement strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}

		currentStatement.WriteString(line)
		currentStatement.WriteString("\n")

		if strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSpace(currentStatement.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			currentStatement.Reset()
		}
	}

	if currentStatement.Len() > 0 {
		stmt := strings.TrimSpace(currentStatement.String())
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}

	return statements
}
