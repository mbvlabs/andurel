package cli

import (
	"fmt"
	"runtime"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/cmds"
	"github.com/mbvlabs/andurel/pkg/storage"
)

func generateSQLCIfNeeded(rootDir string) error {
	hasQueries, err := storage.HasSQLCQueryFiles(rootDir)
	if err != nil {
		return fmt.Errorf("check sqlc queries: %w", err)
	}
	if !hasQueries {
		return nil
	}

	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		return fmt.Errorf("read andurel.lock: %w", err)
	}

	tool, ok := lock.Tools["sqlc"]
	if !ok {
		return fmt.Errorf("sqlc is not configured in andurel.lock")
	}

	if err := syncSingleToolFunc(rootDir, "sqlc", tool, runtime.GOOS, runtime.GOARCH); err != nil {
		return fmt.Errorf("sync sqlc: %w", err)
	}

	return cmds.RunSQLCGenerate(rootDir)
}
