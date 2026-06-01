package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func runTempl(args ...string) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	templBin := filepath.Join(rootDir, "bin", "templ")
	if _, err := os.Stat(templBin); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"templ binary not found at %s\nRun 'andurel tool sync' to download it",
				templBin,
			)
		}
		return err
	}

	cmd := exec.Command(templBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = rootDir

	return cmd.Run()
}
