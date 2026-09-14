package cli

import (
	"fmt"
	"runtime"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/cmds"
	"github.com/mbvlabs/andurel/pkg/storage"
)

func generateNarsilcIfNeeded(rootDir string) error {
	hasQueries, err := storage.HasQueryFiles(rootDir)
	if err != nil {
		return fmt.Errorf("check narsilc queries: %w", err)
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

	tool, ok := lock.Tools["narsilc"]
	if !ok {
		return output.NewError(
			output.CodeConfigError,
			"narsilc is not configured in andurel.lock",
			output.ExitConfig,
			"Restore the generated narsilc tool entry in andurel.lock, then run andurel tool sync.",
		)
	}

	if err := syncSingleToolFunc(rootDir, "narsilc", tool, runtime.GOOS, runtime.GOARCH); err != nil {
		return fmt.Errorf("sync narsilc: %w", err)
	}

	return cmds.RunNarsilcGenerate(rootDir)
}
