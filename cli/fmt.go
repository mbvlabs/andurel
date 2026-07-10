package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newFmtCommand() *cobra.Command {
	var (
		checkMode bool
		skipTempl bool
		skipGo    bool
	)

	cmd := &cobra.Command{
		Use:     "fmt",
		Aliases: []string{"f"},
		Short:   "Format Go and Templ source files",
		Long: `Formats all source files in the project.

Runs gofmt (via go fmt), golines, and templ fmt to ensure consistent
code style across Go and Templ files.

Use --check to verify formatting without modifying files.`,
		Example: `  andurel fmt
  andurel fmt --check
  andurel fmt --skip-templ
  andurel fmt --skip-go`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return fmt.Errorf("not in an andurel project directory: %w", err)
			}
			return runFmtFunc(rootDir, checkMode, skipTempl, skipGo)
		},
	}

	cmd.Flags().BoolVar(&checkMode, "check", false, "Check formatting without modifying files")
	cmd.Flags().BoolVar(&skipTempl, "skip-templ", false, "Skip templ fmt")
	cmd.Flags().BoolVar(&skipGo, "skip-go", false, "Skip go fmt and golines")

	return cmd
}

func runFmt(rootDir string, checkMode, skipTempl, skipGo bool) error {
	hasIssues := false

	if !skipGo {
		if err := runGoFmtFunc(rootDir, checkMode); err != nil {
			hasIssues = true
			_, _ = fmt.Fprintf(os.Stderr, "go fmt: %v\n", err)
		}

		if err := runGolinesFunc(rootDir, checkMode); err != nil {
			hasIssues = true
			_, _ = fmt.Fprintf(os.Stderr, "golines: %v\n", err)
		}
	}

	if !skipTempl {
		if err := runTemplFmtFunc(rootDir, checkMode); err != nil {
			hasIssues = true
			_, _ = fmt.Fprintf(os.Stderr, "templ fmt: %v\n", err)
		}
	}

	if hasIssues {
		if checkMode {
			return fmt.Errorf("some files need formatting")
		}
		return fmt.Errorf("some formatters failed")
	}

	return nil
}

func runGoFmt(rootDir string, checkMode bool) error {
	files, err := collectGoFiles(rootDir)
	if err != nil {
		return fmt.Errorf("collecting Go files: %w", err)
	}
	if len(files) == 0 {
		return nil
	}

	args := []string{"-w"}
	if checkMode {
		args = []string{"-e", "-l"}
	}
	args = append(args, files...)

	cmd := exec.Command("gofmt", args...)
	cmd.Dir = rootDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}

	if checkMode && len(bytes.TrimSpace(out)) > 0 {
		fmt.Printf("Unformatted Go files:\n%s\n", string(out))
		return fmt.Errorf("unformatted Go files found")
	}

	return nil
}

func runGolines(rootDir string, checkMode bool) error {
	golinesPath, err := exec.LookPath("golines")
	if err != nil {
		return fmt.Errorf("golines not found in PATH: %w", err)
	}

	if !checkMode {
		cmd := exec.Command(golinesPath, "-w", "-m", "100", ".")
		cmd.Dir = rootDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	files, err := collectGoFiles(rootDir)
	if err != nil {
		return fmt.Errorf("collecting Go files: %w", err)
	}
	dirty := make([]string, 0)
	for _, file := range files {
		relPath, err := filepath.Rel(rootDir, file)
		if err != nil {
			return err
		}
		original, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		cmd := exec.Command(golinesPath, "-m", "100", relPath)
		cmd.Dir = rootDir
		formatted, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("golines %s: %w", filepath.ToSlash(relPath), err)
		}
		if !bytes.Equal(original, formatted) {
			dirty = append(dirty, filepath.ToSlash(relPath))
		}
	}
	if len(dirty) > 0 {
		return fmt.Errorf("golines would change: %s", strings.Join(dirty, ", "))
	}
	return nil
}

func collectGoFiles(rootDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != rootDir && skipGoPackageDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".go") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func skipGoPackageDir(name string) bool {
	return name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func runTemplFmt(rootDir string, checkMode bool) error {
	templBin := filepath.Join(rootDir, "bin", "templ")
	if _, err := os.Stat(templBin); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("templ binary not found at bin/templ, skipping templ fmt\nRun 'andurel tool sync' to download it")
			return nil
		}
		return err
	}

	if checkMode {
		return checkTemplFormatting(rootDir)
	}
	return runTemplFormatter(rootDir)
}

func checkTemplFormatting(rootDir string) error {
	var changed []string
	err := withDiagnosticProjectCopy(rootDir, func(tempRoot string) error {
		before, err := snapshotFilesForReport(tempRoot)
		if err != nil {
			return err
		}
		if err := runTemplFormatter(tempRoot); err != nil {
			return err
		}
		after, err := snapshotFilesForReport(tempRoot)
		if err != nil {
			return err
		}
		changed = changedSnapshotPaths(before, after)
		return nil
	})
	if err != nil {
		return err
	}
	if len(changed) > 0 {
		return fmt.Errorf("templ fmt would change: %s", strings.Join(changed, ", "))
	}
	return nil
}

func runTemplFormatter(rootDir string) error {
	templBin := filepath.Join(rootDir, "bin", "templ")
	dirs := []string{"views", "email"}
	for _, dir := range dirs {
		dirPath := filepath.Join(rootDir, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			continue
		}

		args := []string{"fmt", dir}
		cmd := exec.Command(templBin, args...)
		cmd.Dir = rootDir
		cmd.Stdout = io.Discard
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("templ fmt failed in %s: %w", dir, err)
		}
	}

	return nil
}
