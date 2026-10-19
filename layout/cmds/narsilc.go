package cmds

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	// QueriesDir is the root under which hand-written narsilc query files live.
	// Each domain colocates its SQL beside the entity it describes, e.g.
	// domains/user.sql, so narsilc scans this tree recursively.
	QueriesDir = "domains"
	// GeneratedQueriesDir is the narsilc output directory. Only domains may
	// import it.
	GeneratedQueriesDir = "domains/internal/queries"
)

// HasQueryFiles reports whether the domains tree contains narsilc query
// definitions. Files must include at least one active -- name: annotation
// followed by a SQL statement. The generated output directory is ignored.
func HasQueryFiles(projectRoot string) (bool, error) {
	queriesRoot := filepath.Join(projectRoot, QueriesDir)
	if _, err := os.Stat(queriesRoot); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat narsilc queries root: %w", err)
	}

	generatedRoot := filepath.Join(projectRoot, GeneratedQueriesDir)
	found := false
	walkErr := filepath.WalkDir(queriesRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path == generatedRoot {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".sql") {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read query file %s: %w", path, readErr)
		}
		if containsQueryAnnotation(string(content)) {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	if walkErr != nil {
		return false, fmt.Errorf("scan narsilc queries: %w", walkErr)
	}

	return found, nil
}

func containsQueryAnnotation(content string) bool {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "-- name:") {
			continue
		}
		for _, next := range lines[i+1:] {
			trimmed := strings.TrimSpace(next)
			if trimmed == "" || strings.HasPrefix(trimmed, "--") {
				continue
			}
			return true
		}
	}
	return false
}
