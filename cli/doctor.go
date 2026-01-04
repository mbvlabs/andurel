package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mbvlabs/andurel/layout"
	"github.com/spf13/cobra"
)

type checkResult struct {
	name    string
	status  checkStatus
	message string
	details []string
}

type checkStatus int

const (
	statusPass checkStatus = iota
	statusWarn
	statusFail
)

func newDoctorCommand() *cobra.Command {
	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run diagnostic checks on your Andurel project",
		Long: `Run comprehensive diagnostic checks to verify your Andurel project health.

This command will check:
  • Environment (Go version)
  • Configuration (andurel.lock)
  • Code quality (go vet, go mod tidy)
  • Code generation (templ, sqlc)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			return runDoctor(verbose)
		},
	}

	doctorCmd.Flags().Bool("verbose", false, "Show detailed output from all checks")

	return doctorCmd
}

func runDoctor(verbose bool) error {
	fmt.Println("Running Andurel project diagnostics...")

	var results []checkResult
	hasErrors := false
	hasWarnings := false

	// Environment checks
	fmt.Println("\n=== Environment ===")
	results = append(results, checkGoVersion())
	results = append(results, checkInAndurelProject())
	printResults(results[len(results)-2:], verbose)

	// Get project root directory
	rootDir, err := findGoModRoot()
	if err != nil {
		// If we can't find the project root, we can't continue with remaining checks
		fmt.Printf("\n✗ Cannot continue: %v\n", err)
		return err
	}

	// Configuration checks
	fmt.Println("\n=== Configuration ===")
	configResults := []checkResult{
		checkLockFile(rootDir),
	}
	results = append(results, configResults...)
	printResults(configResults, verbose)

	// Code quality checks
	fmt.Println("\n=== Code Quality ===")
	qualityResults := []checkResult{
		checkGoVet(rootDir, verbose),
		checkGoModTidy(rootDir, verbose),
	}
	results = append(results, qualityResults...)
	printResults(qualityResults, verbose)

	// Code generation checks
	fmt.Println("\n=== Code Generation ===")
	genResults := []checkResult{
		checkTemplGenerate(rootDir, verbose),
		checkSqlcGenerate(rootDir, verbose),
	}
	results = append(results, genResults...)
	printResults(genResults, verbose)

	// Summary
	passCount := 0
	warnCount := 0
	failCount := 0

	for _, r := range results {
		switch r.status {
		case statusPass:
			passCount++
		case statusWarn:
			warnCount++
			hasWarnings = true
		case statusFail:
			failCount++
			hasErrors = true
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total checks: %d\n", len(results))
	fmt.Printf("✓ Passed: %d\n", passCount)
	if warnCount > 0 {
		fmt.Printf("⚠ Warnings: %d\n", warnCount)
	}
	if failCount > 0 {
		fmt.Printf("✗ Failed: %d\n", failCount)
	}

	if hasErrors {
		fmt.Printf("\n✗ Some checks failed. Please address the issues above.\n")
		return fmt.Errorf("doctor checks failed")
	} else if hasWarnings {
		fmt.Printf("\n⚠ All critical checks passed, but there are warnings to review.\n")
	} else {
		fmt.Printf("\n✓ All checks passed! Your project is healthy.\n")
	}

	return nil
}

func printResults(results []checkResult, verbose bool) {
	for _, r := range results {
		var symbol string
		switch r.status {
		case statusPass:
			symbol = "✓"
		case statusWarn:
			symbol = "⚠"
		case statusFail:
			symbol = "✗"
		}

		fmt.Printf("%s %s", symbol, r.name)
		if r.message != "" {
			fmt.Printf(": %s", r.message)
		}
		fmt.Println()

		if verbose && len(r.details) > 0 {
			for _, detail := range r.details {
				fmt.Printf("  %s\n", detail)
			}
		}
	}
}

func checkGoVersion() checkResult {
	versionOutput := runtime.Version()

	return checkResult{
		name:    "Go version",
		status:  statusPass,
		message: fmt.Sprintf("%s", versionOutput),
	}
}

func checkInAndurelProject() checkResult {
	_, err := findGoModRoot()
	if err != nil {
		return checkResult{
			name:    "Andurel project",
			status:  statusFail,
			message: "not in an Andurel project (go.mod not found)",
		}
	}

	return checkResult{
		name:   "Andurel project",
		status: statusPass,
		message: "found go.mod",
	}
}

func checkLockFile(rootDir string) checkResult {
	lockPath := filepath.Join(rootDir, "andurel.lock")
	if _, err := os.Stat(lockPath); err != nil {
		return checkResult{
			name:    "andurel.lock",
			status:  statusFail,
			message: "file not found",
		}
	}

	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		return checkResult{
			name:    "andurel.lock",
			status:  statusFail,
			message: fmt.Sprintf("invalid format: %v", err),
		}
	}

	return checkResult{
		name:    "andurel.lock",
		status:  statusPass,
		message: fmt.Sprintf("valid (version: %s, %d tools)", lock.Version, len(lock.Tools)),
	}
}

func checkGoVet(rootDir string, verbose bool) checkResult {
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = rootDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		output := stderr.String()
		if output == "" {
			output = stdout.String()
		}

		lines := strings.Split(strings.TrimSpace(output), "\n")
		issueCount := 0
		var details []string

		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				issueCount++
				if verbose {
					details = append(details, line)
				}
			}
		}

		message := fmt.Sprintf("%d issues found", issueCount)
		if !verbose && issueCount > 0 {
			// Show first 3 issues in non-verbose mode
			previewCount := 3
			if issueCount < previewCount {
				previewCount = issueCount
			}
			for i := 0; i < previewCount; i++ {
				if i < len(lines) && strings.TrimSpace(lines[i]) != "" {
					details = append(details, lines[i])
				}
			}
			if issueCount > previewCount {
				details = append(details, fmt.Sprintf("... and %d more (use --verbose to see all)", issueCount-previewCount))
			}
		}

		return checkResult{
			name:    "go vet",
			status:  statusFail,
			message: message,
			details: details,
		}
	}

	return checkResult{
		name:    "go vet",
		status:  statusPass,
		message: "no issues found",
	}
}

func checkGoModTidy(rootDir string, verbose bool) checkResult {
	goModPath := filepath.Join(rootDir, "go.mod")
	goSumPath := filepath.Join(rootDir, "go.sum")

	// Read original content
	goModOrig, err := os.ReadFile(goModPath)
	if err != nil {
		return checkResult{
			name:    "go mod tidy",
			status:  statusFail,
			message: fmt.Sprintf("cannot read go.mod: %v", err),
		}
	}

	goSumOrig, _ := os.ReadFile(goSumPath) // go.sum might not exist, that's ok

	// Ensure we restore original files when function exits
	defer func() {
		os.WriteFile(goModPath, goModOrig, 0644)
		if goSumOrig != nil {
			os.WriteFile(goSumPath, goSumOrig, 0644)
		}
	}()

	// Run go mod tidy
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = rootDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return checkResult{
			name:    "go mod tidy",
			status:  statusFail,
			message: fmt.Sprintf("failed to run: %v", err),
			details: []string{stderr.String()},
		}
	}

	// Read new content after tidy
	goModNew, _ := os.ReadFile(goModPath)
	goSumNew, _ := os.ReadFile(goSumPath)

	// Compare
	goModChanged := !bytes.Equal(goModOrig, goModNew)
	goSumChanged := !bytes.Equal(goSumOrig, goSumNew)

	if goModChanged || goSumChanged {
		return checkResult{
			name:    "go mod tidy",
			status:  statusWarn,
			message: "go.mod or go.sum needs tidying",
			details: []string{"Run 'go mod tidy' to update"},
		}
	}

	return checkResult{
		name:    "go mod tidy",
		status:  statusPass,
		message: "dependencies are tidy",
	}
}

func checkTemplGenerate(rootDir string, verbose bool) checkResult {
	templPath := filepath.Join(rootDir, "bin", "templ")
	if _, err := os.Stat(templPath); err != nil {
		return checkResult{
			name:    "templ generate",
			status:  statusWarn,
			message: "templ binary not found (skipping check)",
		}
	}

	cmd := exec.Command(templPath, "generate")
	cmd.Dir = rootDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return checkResult{
			name:    "templ generate",
			status:  statusFail,
			message: "compilation failed",
			details: []string{stderr.String()},
		}
	}

	return checkResult{
		name:    "templ generate",
		status:  statusPass,
		message: "templates compile successfully",
	}
}

func checkSqlcGenerate(rootDir string, verbose bool) checkResult {
	sqlcPath := filepath.Join(rootDir, "bin", "sqlc")
	if _, err := os.Stat(sqlcPath); err != nil {
		return checkResult{
			name:    "sqlc compile",
			status:  statusWarn,
			message: "sqlc binary not found (skipping check)",
		}
	}

	// Check if database/sqlc.yaml exists
	sqlcConfigPath := filepath.Join(rootDir, "database", "sqlc.yaml")
	if _, err := os.Stat(sqlcConfigPath); err != nil {
		return checkResult{
			name:    "sqlc compile",
			status:  statusWarn,
			message: "database/sqlc.yaml not found (skipping check)",
		}
	}

	cmd := exec.Command(sqlcPath, "compile", "-f", "sqlc.yaml")
	cmd.Dir = filepath.Join(rootDir, "database")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		output := stderr.String()
		if output == "" {
			output = stdout.String()
		}

		lines := strings.Split(strings.TrimSpace(output), "\n")
		var details []string
		issueCount := len(lines)

		if verbose {
			details = lines
		} else {
			// Show first 3 issues in non-verbose mode
			previewCount := 3
			if issueCount < previewCount {
				previewCount = issueCount
			}
			for i := 0; i < previewCount; i++ {
				if i < len(lines) && strings.TrimSpace(lines[i]) != "" {
					details = append(details, lines[i])
				}
			}
			if issueCount > previewCount {
				details = append(details, fmt.Sprintf("... and %d more (use --verbose to see all)", issueCount-previewCount))
			}
		}

		return checkResult{
			name:    "sqlc compile",
			status:  statusFail,
			message: fmt.Sprintf("%d issues found", issueCount),
			details: details,
		}
	}

	return checkResult{
		name:    "sqlc compile",
		status:  statusPass,
		message: "SQL queries compile successfully",
	}
}
