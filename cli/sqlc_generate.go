package cli

import (
	"fmt"
	"runtime"

	"github.com/mbvlabs/andurel/cli/output"
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
		return output.WrapError(
			output.CodeConfigError,
			fmt.Errorf("read andurel.lock: %w", err),
			output.ExitConfig,
			"Run this from an Andurel project with a valid andurel.lock file.",
		)
	}

	tool, ok := lock.Tools["sqlc"]
	if !ok {
		return output.NewError(
			output.CodeConfigError,
			"sqlc is not configured in andurel.lock",
			output.ExitConfig,
			"Restore the generated sqlc tool entry in andurel.lock, then run andurel tool sync.",
		)
	}

	if err := syncSingleToolFunc(rootDir, "sqlc", tool, runtime.GOOS, runtime.GOARCH); err != nil {
		return fmt.Errorf("sync sqlc: %w", err)
	}

	return cmds.RunSQLCGenerate(rootDir)
}
