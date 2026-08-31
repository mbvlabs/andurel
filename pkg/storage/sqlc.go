package storage

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// SQLCConfigFile is the project-root sqlc configuration filename.
	SQLCConfigFile = "sqlc.yaml"
	// SQLCQueriesDir holds hand-written SQL query files for sqlc.
	SQLCQueriesDir = "models/queries"
	// SQLCGeneratedDir is the sqlc output directory beneath models/internal.
	SQLCGeneratedDir = "models/internal/queries"
)

//go:embed sqlc.yaml
var sqlcConfig []byte

// SQLCConfig returns the canonical Andurel sqlc configuration.
func SQLCConfig() []byte {
	out := make([]byte, len(sqlcConfig))
	copy(out, sqlcConfig)
	return out
}

// WriteSQLCConfig writes the canonical sqlc.yaml into dir.
func WriteSQLCConfig(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("storage: sqlc config directory is required")
	}

	path := filepath.Join(dir, SQLCConfigFile)
	if err := os.WriteFile(path, sqlcConfig, 0o644); err != nil {
		return fmt.Errorf("storage: write sqlc config: %w", err)
	}

	return nil
}

// HasSQLCQueryFiles reports whether models/queries contains sqlc query
// definitions. Files must include at least one sqlc name annotation.
func HasSQLCQueryFiles(projectRoot string) (bool, error) {
	queriesDir := filepath.Join(projectRoot, SQLCQueriesDir)
	entries, err := os.ReadDir(queriesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("storage: read sqlc queries directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(queriesDir, entry.Name()))
		if err != nil {
			return false, fmt.Errorf("storage: read sqlc query file %s: %w", entry.Name(), err)
		}
		if containsSQLCQueryAnnotation(string(content)) {
			return true, nil
		}
	}

	return false, nil
}

func containsSQLCQueryAnnotation(content string) bool {
	return strings.Contains(content, "-- name:")
}
