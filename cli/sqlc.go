package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func runSQLC(args ...string) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	sqlcBin := filepath.Join(rootDir, "bin", "sqlc")
	if _, err := os.Stat(sqlcBin); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"sqlc binary not found at %s\nRun 'andurel tool sync' to download it",
				sqlcBin,
			)
		}
		return err
	}

	cmd := exec.Command(sqlcBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = rootDir

	return cmd.Run()
}
