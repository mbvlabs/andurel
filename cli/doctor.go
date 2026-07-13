package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/layout"
	"github.com/spf13/cobra"
)

type checkResult struct {
	category string
	name     string
	status   checkStatus
	message  string
	details  []string
	hint     string
}

type checkStatus int

const (
	statusPass checkStatus = iota
	statusWarn
	statusFail
)

// String returns the status value used in doctor output.
func (s checkStatus) String() string {
	switch s {
	case statusPass:
		return "pass"
	case statusWarn:
		return "warn"
	case statusFail:
		return "fail"
	default:
		return "unknown"
	}
}

type doctorReport struct {
	Version string        `json:"version,omitempty"`
	Root    string        `json:"root,omitempty"`
	Checks  []doctorCheck `json:"checks"`
	Summary doctorSummary `json:"summary"`
}

type doctorCheck struct {
	Category string   `json:"category"`
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Message  string   `json:"message,omitempty"`
	Details  []string `json:"details,omitempty"`
	Hint     string   `json:"hint,omitempty"`
	Blocking bool     `json:"blocking"`
}

type doctorSummary struct {
	Total    int    `json:"total"`
	Passed   int    `json:"passed"`
	Warnings int    `json:"warnings"`
	Failed   int    `json:"failed"`
	Status   string `json:"status"`
}

func newDoctorCommand(currentVersion string) *cobra.Command {
	doctorCmd := &cobra.Command{
		Use:     "doctor",
		Aliases: []string{"doc"},
		Short:   "Run diagnostic checks on your Andurel project",
		Long: `Run comprehensive diagnostic checks to verify your Andurel project health.

This command will check:
  • Environment (Go version, latest stable Andurel release)
  • Configuration (andurel.lock)
  • Code quality (go vet, go mod tidy)
  • Code generation (templ)`,
		Example: `  andurel doctor
  andurel doctor --verbose`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			opts, err := output.ParseOptions(cmd)
			if err != nil {
				return err
			}
			if opts.Mode == output.ModeJSON || opts.Mode == output.ModeAgent {
				return runDoctorStructured(cmd, currentVersion, verbose)
			}
			return runDoctor(currentVersion, verbose)
		},
	}

	doctorCmd.Flags().Bool("verbose", false, "Emit verbose diagnostic output")

	return doctorCmd
}

func runDoctorStructured(cmd *cobra.Command, currentVersion string, verbose bool) error {
	report, err := collectDoctorReport(currentVersion, verbose)
	if err != nil {
		return err
	}
	if report.Summary.Status == "fail" {
		return output.NewError(output.CodeError, doctorSummaryMessage(report), output.ExitUsage, "Inspect the failed doctor checks and address the reported issues.")
	}
	if err := output.OK(cmd, report, doctorSummaryMessage(report)); err != nil {
		return err
	}
	return nil
}

func collectDoctorReport(currentVersion string, verbose bool) (doctorReport, error) {
	var results []checkResult

	results = append(results, categorizeResults("environment",
		checkGoVersion(),
		checkLatestAndurelRelease(currentVersion),
		checkInAndurelProject(),
	)...)

	rootDir, err := findGoModRoot()
	if err != nil {
		return buildDoctorReport(currentVersion, "", results), err
	}

	results = append(results, categorizeResults("configuration",
		checkLockFile(rootDir),
		checkAndurelVersion(rootDir, currentVersion),
		checkToolVersions(rootDir, verbose),
	)...)

	results = append(results, categorizeResults("code_quality",
		checkGoVet(rootDir, verbose),
		checkGoModTidy(rootDir, verbose),
	)...)

	results = append(results, categorizeResults("code_generation",
		codeGenerationChecks(rootDir, verbose)...,
	)...)

	return buildDoctorReport(currentVersion, rootDir, results), nil
}

func categorizeResults(category string, results ...checkResult) []checkResult {
	for i := range results {
		results[i].category = category
	}
	return results
}

func buildDoctorReport(currentVersion, rootDir string, results []checkResult) doctorReport {
	report := doctorReport{
		Version: currentVersion,
		Root:    rootDir,
		Checks:  make([]doctorCheck, 0, len(results)),
		Summary: doctorSummary{Total: len(results), Status: "pass"},
	}

	for _, result := range results {
		switch result.status {
		case statusPass:
			report.Summary.Passed++
		case statusWarn:
			report.Summary.Warnings++
		case statusFail:
			report.Summary.Failed++
		}

		report.Checks = append(report.Checks, doctorCheckFromResult(result))
	}

	if report.Summary.Failed > 0 {
		report.Summary.Status = "fail"
	} else if report.Summary.Warnings > 0 {
		report.Summary.Status = "warn"
	}

	return report
}

func doctorCheckFromResult(result checkResult) doctorCheck {
	return doctorCheck{
		Category: result.category,
		Name:     result.name,
		Status:   result.status.String(),
		Message:  result.message,
		Details:  append([]string(nil), result.details...),
		Hint:     doctorHint(result),
		Blocking: result.status == statusFail,
	}
}

func doctorHint(result checkResult) string {
	if result.status == statusPass {
		return ""
	}
	if result.hint != "" {
		return result.hint
	}

	switch result.name {
	case "Andurel project":
		return "Run this from a directory containing an Andurel project's go.mod file."
	case "andurel.lock":
		return "Run from a generated Andurel project root with a valid andurel.lock file."
	case "Andurel version":
		return "Use the Andurel version recorded in andurel.lock or run andurel upgrade."
	case "tool versions":
		return "Run andurel tool sync to install or update framework tools."
	case "go vet":
		return "Run go vet ./... and fix the reported issues."
	case "go mod tidy":
		return "Run go mod tidy and commit the resulting go.mod or go.sum changes."
	case "views generate":
		return "Run andurel generate view and fix any template generation errors."
	case "routes.ts":
		return "Run andurel generate routes and commit the updated resources/js/routes.ts file."
	default:
		return ""
	}
}

func doctorSummaryMessage(report doctorReport) string {
	switch report.Summary.Status {
	case "fail":
		return "Doctor checks failed"
	case "warn":
		return "Doctor checks completed with warnings"
	default:
		return "Doctor checks passed"
	}
}

func runDoctor(currentVersion string, verbose bool) error {
	printDoctorBanner()
	fmt.Println("Running Andurel project diagnostics...")

	var results []checkResult
	hasErrors := false
	hasWarnings := false

	// Environment checks
	fmt.Println("\n=== Environment ===")
	environmentResults := []checkResult{
		checkGoVersion(),
		checkLatestAndurelRelease(currentVersion),
		checkInAndurelProject(),
	}
	results = append(results, environmentResults...)
	printResults(environmentResults, verbose)

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
		checkAndurelVersion(rootDir, currentVersion),
		checkToolVersions(rootDir, verbose),
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
	genResults := codeGenerationChecks(rootDir, verbose)
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
		message: versionOutput,
	}
}

func checkLatestAndurelRelease(currentVersion string) checkResult {
	current, ok := canonicalAndurelVersion(currentVersion)
	if !ok {
		return checkResult{
			name:    "Andurel release",
			status:  statusPass,
			message: "development build; update check skipped",
		}
	}

	latest, err := lookupLatestAndurelVersionFunc(context.Background())
	if err != nil {
		return checkResult{
			name:    "Andurel release",
			status:  statusWarn,
			message: "could not check the latest stable release",
			details: []string{err.Error()},
			hint:    "Check network connectivity and run andurel doctor again.",
		}
	}

	if newerAndurelVersion(current, latest) {
		return checkResult{
			name:    "Andurel release",
			status:  statusWarn,
			message: fmt.Sprintf("%s is available (current: %s)", latest, current),
			hint:    fmt.Sprintf("Run '%s', then run 'andurel upgrade'.", andurelInstallCommand(latest)),
		}
	}
	if current != latest {
		return checkResult{
			name:    "Andurel release",
			status:  statusPass,
			message: fmt.Sprintf("no newer stable release found (current: %s, latest stable: %s)", current, latest),
		}
	}

	return checkResult{
		name:    "Andurel release",
		status:  statusPass,
		message: fmt.Sprintf("latest stable release installed (%s)", current),
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
		name:    "Andurel project",
		status:  statusPass,
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

func checkAndurelVersion(rootDir, currentVersion string) checkResult {
	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		return checkResult{
			name:    "Andurel version",
			status:  statusWarn,
			message: "andurel.lock missing or invalid (skipping check)",
		}
	}

	if lock.Version == "" {
		return checkResult{
			name:    "Andurel version",
			status:  statusWarn,
			message: "andurel.lock has no framework version",
		}
	}

	if currentVersion == "" {
		return checkResult{
			name:    "Andurel version",
			status:  statusWarn,
			message: fmt.Sprintf("lock expects %s, current version unknown", lock.Version),
		}
	}

	if versionsMatch(lock.Version, currentVersion) {
		return checkResult{
			name:    "Andurel version",
			status:  statusPass,
			message: fmt.Sprintf("matches andurel.lock (%s)", lock.Version),
		}
	}

	return checkResult{
		name:    "Andurel version",
		status:  statusWarn,
		message: fmt.Sprintf("lock expects %s, current is %s", lock.Version, currentVersion),
	}
}

func checkToolVersions(rootDir string, verbose bool) checkResult {
	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		return checkResult{
			name:    "tool versions",
			status:  statusWarn,
			message: "andurel.lock missing or invalid (skipping check)",
		}
	}

	if len(lock.Tools) == 0 {
		return checkResult{
			name:    "tool versions",
			status:  statusPass,
			message: "no tools listed in andurel.lock",
		}
	}

	toolNames := make([]string, 0, len(lock.Tools))
	for name := range lock.Tools {
		toolNames = append(toolNames, name)
	}
	sort.Strings(toolNames)

	type versionResult struct {
		name        string
		expectedVer string
		actualVer   string
		missing     bool
		unknown     bool
		versionErr  error
	}

	results := make(chan versionResult, len(toolNames))

	// Check all tools in parallel
	for _, name := range toolNames {
		go func(name string) {
			tool := lock.Tools[name]
			fullBinPath := filepath.Join(rootDir, "bin", name)
			if _, err := os.Stat(fullBinPath); err != nil {
				results <- versionResult{name: name, expectedVer: tool.Version, missing: true}
				return
			}

			binPath := filepath.Join("bin", name)
			actualVersion, err := getToolVersion(binPath, tool.VersionCheck, name)
			if err != nil {
				results <- versionResult{name: name, expectedVer: tool.Version, unknown: true, versionErr: err}
				return
			}

			results <- versionResult{name: name, expectedVer: tool.Version, actualVer: actualVersion}
		}(name)
	}

	// Collect results
	resultMap := make(map[string]versionResult)
	for range toolNames {
		r := <-results
		resultMap[r.name] = r
	}

	var details []string
	mismatchCount := 0
	missingCount := 0
	unknownCount := 0

	// Process in sorted order for consistent output
	for _, name := range toolNames {
		r := resultMap[name]
		if r.missing {
			missingCount++
			details = append(details, fmt.Sprintf("%s: missing (expected %s)", name, r.expectedVer))
			continue
		}
		if r.unknown {
			unknownCount++
			msg := fmt.Sprintf("%s: could not determine version (expected %s)", name, r.expectedVer)
			if r.versionErr != nil {
				msg = fmt.Sprintf("%s: %v", name, r.versionErr)
			}
			details = append(details, msg)
			continue
		}
		if !versionsMatch(r.expectedVer, r.actualVer) {
			mismatchCount++
			details = append(
				details,
				fmt.Sprintf("%s: expected %s, found %s", name, r.expectedVer, r.actualVer),
			)
		}
	}

	if mismatchCount == 0 && missingCount == 0 && unknownCount == 0 {
		return checkResult{
			name:    "tool versions",
			status:  statusPass,
			message: "all tools match andurel.lock",
		}
	}

	message := fmt.Sprintf("%d mismatched, %d missing, %d unknown",
		mismatchCount, missingCount, unknownCount)
	if !verbose && len(details) > 0 {
		details = truncateDetails(details, 3)
	}

	return checkResult{
		name:    "tool versions",
		status:  statusWarn,
		message: message,
		details: details,
	}
}

func truncateDetails(details []string, max int) []string {
	if len(details) <= max {
		return details
	}
	remaining := len(details) - max
	truncated := append([]string{}, details[:max]...)
	truncated = append(
		truncated,
		fmt.Sprintf("... and %d more (use --verbose to see all)", remaining),
	)
	return truncated
}

func getToolVersion(binPath string, vc *layout.VersionCheck, toolName string) (string, error) {
	return versionFromCommand(binPath, vc, toolName)
}

// runWithTimeout runs a command with a timeout, killing the entire process group
// if the timeout is exceeded. This ensures child processes that inherit stdout/stderr
// pipes are also killed, preventing CombinedOutput-style hangs.
func runWithTimeout(ctx context.Context, path string, args ...string) ([]byte, error) {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdin = nil

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	type result struct {
		output []byte
		err    error
	}
	done := make(chan result, 1)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, io.LimitReader(io.MultiReader(stdout, stderr), 1<<20))
		err := cmd.Wait()
		done <- result{buf.Bytes(), err}
	}()

	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil, ctx.Err()
	case r := <-done:
		return r.output, r.err
	}
}

func versionFromCommand(binPath string, vc *layout.VersionCheck, toolName string) (string, error) {
	if vc == nil || len(vc.Args) == 0 {
		return "", fmt.Errorf(
			"%s: missing versionCheck in andurel.lock (e.g. \"versionCheck\": {\"args\": [\"version\", \"--flag\"]})",
			toolName,
		)
	}

	rootDir, err := findGoModRoot()
	if err != nil {
		return "", fmt.Errorf("could not find project root: %w", err)
	}

	fullPath := filepath.Join(rootDir, binPath)
	return versionFromExecutable(fullPath, vc, toolName)
}

func versionFromExecutable(fullPath string, vc *layout.VersionCheck, toolName string) (string, error) {
	if vc == nil || len(vc.Args) == 0 {
		return "", fmt.Errorf(
			"%s: missing versionCheck in andurel.lock (e.g. \"versionCheck\": {\"args\": [\"version\", \"--flag\"]})",
			toolName,
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	output, err := runWithTimeout(ctx, fullPath, vc.Args...)
	cancel()
	if err != nil {
		return "", fmt.Errorf("version command failed for %s: %w", toolName, err)
	}

	version, err := extractVersionWithCheck(string(output), vc.Regexp)
	if err != nil {
		return "", fmt.Errorf("invalid version expression for %s: %w", toolName, err)
	}
	if version == "" {
		return "", fmt.Errorf("no version output for %s", toolName)
	}
	return version, nil
}

func extractVersionWithCheck(output, expression string) (string, error) {
	if expression == "" {
		return extractVersion(output), nil
	}
	pattern, err := regexp.Compile(expression)
	if err != nil {
		return "", err
	}
	matches := pattern.FindStringSubmatch(output)
	if len(matches) == 0 {
		return "", nil
	}
	for _, match := range matches[1:] {
		if match != "" {
			return match, nil
		}
	}
	return matches[0], nil
}

func extractVersion(output string) string {
	versionPattern, err := regexp.Compile(`v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?`)
	if err != nil {
		return ""
	}

	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "update available") || strings.Contains(lower, "new version available") {
			continue
		}
		if version := versionPattern.FindString(line); version != "" {
			return version
		}
	}

	return versionPattern.FindString(output)
}

func versionsMatch(expected, actual string) bool {
	if expected == "" || actual == "" {
		return false
	}

	expectedNorm := normalizeVersion(expected)
	actualNorm := normalizeVersion(actual)
	return expectedNorm == actualNorm
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	return version
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
			previewCount := min(issueCount, 3)
			for i := range previewCount {
				if i < len(lines) && strings.TrimSpace(lines[i]) != "" {
					details = append(details, lines[i])
				}
			}
			if issueCount > previewCount {
				details = append(
					details,
					fmt.Sprintf(
						"... and %d more (use --verbose to see all)",
						issueCount-previewCount,
					),
				)
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

	goSumOrig, err := readOptionalFile(goSumPath)
	if err != nil {
		return checkResult{
			name:    "go mod tidy",
			status:  statusFail,
			message: fmt.Sprintf("cannot read go.sum: %v", err),
		}
	}

	var goModNew, goSumNew []byte
	err = withDiagnosticProjectCopy(rootDir, func(tempRoot string) error {
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = tempRoot
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("go mod tidy failed: %w: %s", err, strings.TrimSpace(stderr.String()))
		}

		var err error
		goModNew, err = os.ReadFile(filepath.Join(tempRoot, "go.mod"))
		if err != nil {
			return err
		}
		goSumNew, err = readOptionalFile(filepath.Join(tempRoot, "go.sum"))
		return err
	})
	if err != nil {
		return checkResult{
			name:    "go mod tidy",
			status:  statusFail,
			message: "temporary tidy diagnostic failed",
			details: []string{err.Error()},
		}
	}

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

func readOptionalFile(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return content, err
}

func checkTemplGenerate(rootDir string, verbose bool) checkResult {
	templPath := filepath.Join(rootDir, "bin", "templ")
	if _, err := os.Stat(templPath); err != nil {
		return checkResult{
			name:    "views generate",
			status:  statusWarn,
			message: "templ binary not found (skipping check)",
		}
	}

	var changed []string
	err := withDiagnosticProjectCopy(rootDir, func(tempRoot string) error {
		before, err := snapshotFilesForReport(tempRoot)
		if err != nil {
			return err
		}
		cmd := exec.Command(filepath.Join(tempRoot, "bin", "templ"), "generate")
		cmd.Dir = tempRoot
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("templ generate failed: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		after, err := snapshotFilesForReport(tempRoot)
		if err != nil {
			return err
		}
		changed = changedSnapshotPaths(before, after)
		return nil
	})
	if err != nil {
		return checkResult{
			name:    "views generate",
			status:  statusFail,
			message: "temporary generation diagnostic failed",
			details: []string{err.Error()},
		}
	}
	if len(changed) > 0 {
		details := []string{"Run 'andurel generate view' and commit the generated output."}
		if verbose {
			details = append(details, changed...)
		}
		return checkResult{
			name:    "views generate",
			status:  statusFail,
			message: "generated templates are out of date",
			details: details,
		}
	}

	return checkResult{
		name:    "views generate",
		status:  statusPass,
		message: "templates generated successfully",
	}
}

func changedSnapshotPaths(before, after fileSnapshot) []string {
	changed := make([]string, 0)
	for path, state := range after {
		if previous, ok := before[path]; !ok || previous.Hash != state.Hash || previous.Mode != state.Mode {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed
}

func codeGenerationChecks(rootDir string, verbose bool) []checkResult {
	results := []checkResult{
		checkTemplGenerate(rootDir, verbose),
	}
	if projectUsesInertia(rootDir) {
		results = append(results, checkRoutesTSGenerate(rootDir, verbose))
	}
	return results
}

func projectUsesInertia(rootDir string) bool {
	lock, err := layout.ReadLockFile(rootDir)
	return err == nil &&
		lock.ScaffoldConfig != nil &&
		layout.IsSupportedInertiaAdapter(lock.ScaffoldConfig.Inertia)
}

func checkRoutesTSGenerate(rootDir string, verbose bool) checkResult {
	var expected []byte
	var helperCount int
	var skippedCount int
	err := withDiagnosticProjectCopy(rootDir, func(tempRoot string) error {
		manifest, err := collectRouteManifest(tempRoot)
		if err != nil {
			return err
		}
		report, err := generateRoutesJSFile(tempRoot, manifest)
		if err != nil {
			return err
		}
		helperCount = report.GeneratedHelpers
		skippedCount = report.SkippedCount
		expected, err = os.ReadFile(filepath.Join(tempRoot, generatedRoutesJSPath))
		return err
	})
	if err != nil {
		return checkResult{
			name:    "routes.ts",
			status:  statusFail,
			message: "temporary route generation diagnostic failed",
			details: []string{err.Error()},
		}
	}

	target := filepath.Join(rootDir, generatedRoutesJSPath)
	actual, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return checkResult{
				name:    "routes.ts",
				status:  statusFail,
				message: "resources/js/routes.ts is missing",
				details: []string{"Run 'andurel generate routes' to create it."},
			}
		}
		return checkResult{
			name:    "routes.ts",
			status:  statusFail,
			message: "could not read resources/js/routes.ts",
			details: []string{err.Error()},
		}
	}

	if !bytes.Equal(actual, expected) {
		details := []string{"Run 'andurel generate routes' to update resources/js/routes.ts."}
		if verbose {
			details = append(details, fmt.Sprintf("expected %d bytes, found %d bytes", len(expected), len(actual)))
			if skippedCount > 0 {
				details = append(details, fmt.Sprintf("%d route manifest entries were skipped", skippedCount))
			}
		}
		return checkResult{
			name:    "routes.ts",
			status:  statusFail,
			message: "resources/js/routes.ts is out of date",
			details: details,
		}
	}

	return checkResult{
		name:    "routes.ts",
		status:  statusPass,
		message: fmt.Sprintf("matches route manifest (%d helpers)", helperCount),
	}
}
