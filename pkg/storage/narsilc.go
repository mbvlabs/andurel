package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// QueriesDir holds hand-written SQL query files for narsilc.
	QueriesDir = "models/queries"
	// GeneratedQueriesDir is the narsilc output directory beneath models/internal.
	GeneratedQueriesDir = "models/internal/queries"
)

// HasQueryFiles reports whether models/queries contains narsilc query
// definitions. Files must include at least one active -- name: annotation.
func HasQueryFiles(projectRoot string) (bool, error) {
	queriesDir := filepath.Join(projectRoot, QueriesDir)
	entries, err := os.ReadDir(queriesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("storage: read narsilc queries directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(queriesDir, entry.Name()))
		if err != nil {
			return false, fmt.Errorf("storage: read query file %s: %w", entry.Name(), err)
		}
		if containsQueryAnnotation(string(content)) {
			return true, nil
		}
	}

	return false, nil
}

func containsQueryAnnotation(content string) bool {
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "-- name:") {
			return true
		}
	}
	return false
}
