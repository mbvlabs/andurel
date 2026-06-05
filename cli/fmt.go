package cli

import (
	"bytes"
	"fmt"
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
		Short: "Format Go and Templ source files",
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
			return runFmt(rootDir, checkMode, skipTempl, skipGo)
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
		if err := runGoFmt(rootDir, checkMode); err != nil {
			hasIssues = true
			fmt.Fprintf(os.Stderr, "go fmt: %v\n", err)
		}

		if err := runGolines(rootDir, checkMode); err != nil {
			hasIssues = true
			fmt.Fprintf(os.Stderr, "golines: %v\n", err)
		}
	}

	if !skipTempl {
		if err := runTemplFmt(rootDir, checkMode); err != nil {
			hasIssues = true
			fmt.Fprintf(os.Stderr, "templ fmt: %v\n", err)
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
	args := []string{"fmt"}
	if checkMode {
		args = append(args, "-e", "-l")
	}
	args = append(args, "./...")

	cmd := exec.Command("go", args...)
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
		fmt.Println("golines not found in PATH, skipping")
		return nil
	}

	if !checkMode {
		cmd := exec.Command(golinesPath, "-w", "-m", "100", ".")
		cmd.Dir = rootDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	cmd := exec.Command(golinesPath, "-m", "100", ".")
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err != nil {
		return err
	}

	originalFiles, err := collectGoFiles(rootDir)
	if err != nil {
		return fmt.Errorf("collecting Go files: %w", err)
	}

	for _, f := range originalFiles {
		relPath, _ := filepath.Rel(rootDir, f)
		orig, _ := os.ReadFile(f)
		if !bytes.Contains(out, orig) {
			fmt.Printf("  %s\n", relPath)
		}
	}

	return nil
}

func collectGoFiles(rootDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".go") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
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

	dirs := []string{"views", "email"}
	for _, dir := range dirs {
		dirPath := filepath.Join(rootDir, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			continue
		}

		args := []string{"fmt", dir}
		cmd := exec.Command(templBin, args...)
		cmd.Dir = rootDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("templ fmt failed in %s: %w", dir, err)
		}
	}

	return nil
}
