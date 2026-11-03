package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/mbvlabs/andurel/v2/layout/cmds"
	"github.com/mbvlabs/andurel/v2/layout/upgrade"
	"github.com/spf13/cobra"
)

type checkResult struct {
	category    string
	name        string
	status      checkStatus
	message     string
	details     []string
	hint        string
	missingTool string
	// changed lists the generated files that regeneration rewrote. verify uses
	// it to report one violation per file instead of one aggregate finding.
	changed []string
	// manualActions carries the static detector result for application-owned
	// manual upgrade actions.
	manualActions []upgrade.ActionCheck
}

type checkStatus int

const (
	statusPass checkStatus = iota
	statusWarn
	statusFail
	// statusSkip marks a check that could not run because its prerequisite is
	// missing, for example a Go toolchain or project-local tool. A skip is
	// never reported as a pass.
	statusSkip
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
	case statusSkip:
		return "skip"
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
	// ManualActions reports the static applied/not_applied/not_applicable
	// state of application-owned upgrade actions, keyed by their stable ID.
	ManualActions []upgrade.ActionCheck `json:"manual_actions,omitempty"`
}

type doctorSummary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Warnings int `json:"warnings"`
	Failed   int `json:"failed"`
	// Skipped counts checks that could not run; it is additive so existing
	// consumers keep working.
	Skipped int    `json:"skipped,omitempty"`
	Status  string `json:"status"`
}

func newDoctorCommand(currentVersion string) *cobra.Command {
	doctorCmd := &cobra.Command{
		Use:     "doctor",
		Aliases: []string{"doc"},
		Short:   "Run diagnostic checks on your Andurel project",
		Long: `Run comprehensive diagnostic checks to verify your Andurel project health.

This command will check:
  • Environment (Go version, latest Andurel release)
  • Configuration (andurel.lock, first-run setup, Inertia SSR readiness, narsilc generator/runtime alignment)
  • Manual actions (static applied / not applied / not applicable status for application-owned upgrade actions, by stable action ID)
  • Code quality (go vet, go mod tidy)
  • Code generation (templ, email templates, narsilc, and Inertia route helpers and payload types when configured)

A check that cannot run is reported as skipped, never as a pass.`,
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
			return runDoctor(cmd, currentVersion, verbose)
		},
	}

	doctorCmd.Flags().Bool("verbose", false, "Emit verbose diagnostic output")
	setAgentMetadata(
		doctorCmd,
		"diagnostics",
		"Read-only project checks include first-run setup, Inertia SSR readiness, the applied/not_applied/not_applicable state of application-owned upgrade actions by stable ID, templ, email templates, active narsilc queries, and Inertia route helper and payload type drift. Manual actions that are not applied warn; a check that could not run is reported as skipped. Use --json for actionable check hints.",
	)

	return doctorCmd
}

func runDoctorStructured(cmd *cobra.Command, currentVersion string, verbose bool) error {
	report, err := collectDoctorReport(currentVersion, verbose)
	if err != nil {
		if cliErr, ok := err.(*output.CLIError); ok && cliErr.Data == nil {
			cliErr.Data = report
			return cliErr
		}
		return &output.CLIError{
			Code:     output.CodeError,
			Message:  doctorSummaryMessage(report),
			Hint:     "Inspect the failed doctor checks and address the reported issues.",
			ExitCode: output.ExitUsage,
			Cause:    err,
			Data:     report,
		}
	}
	if report.Summary.Status == "fail" {
		return &output.CLIError{
			Code:     output.CodeError,
			Message:  doctorSummaryMessage(report),
			Hint:     "Inspect the failed doctor checks and address the reported issues.",
			ExitCode: output.ExitUsage,
			Data:     report,
		}
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
		checkNarsilcRuntime(rootDir),
		checkProjectSetup(rootDir),
		checkManualActions(rootDir, currentVersion),
		checkInertiaSSRConfiguration(rootDir),
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
		case statusSkip:
			report.Summary.Skipped++
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
		Category:      result.category,
		Name:          result.name,
		Status:        result.status.String(),
		Message:       result.message,
		Details:       append([]string(nil), result.details...),
		Hint:          doctorHint(result),
		Blocking:      result.status == statusFail,
		ManualActions: append([]upgrade.ActionCheck(nil), result.manualActions...),
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
	case "narsilc runtime":
		return "Align the github.com/mbvlabs/narsilc version in go.mod with the narsilc tool, then run go mod tidy."
	case "Project setup":
		return "Run andurel tool sync and install the frontend dependencies from the project root."
	case "Manual actions":
		return "Apply the pending actions from 'andurel inspect upgrade-notes --from <old-version>', or confirm your own implementation is equivalent, then run doctor again."
	case "Inertia SSR":
		return "In development, Vite serves SSR. In production, build assets/dist/ssr/ssr.js and run cmd/ssr."
	case "go vet":
		return "Run go vet ./... and fix the reported issues."
	case "go mod tidy":
		return "Run go mod tidy and commit the resulting go.mod or go.sum changes."
	case "templ generate":
		return "Commit the generated *_templ.go output (andurel build regenerates it) and fix any template errors."
	case "emails export":
		return "Run andurel sync emails and commit the updated emails/_generated and email/templates files."
	case "narsilc generate":
		return "Run andurel sync queries and commit the updated domains/internal/queries output."
	case "routes.ts":
		return "Run andurel sync routes and commit the updated resources/js/routes.ts file."
	case "payloads.ts":
		return "Run andurel sync payloads and commit the updated resources/js/types/payloads.ts file."
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
		if report.Summary.Skipped > 0 {
			return "Doctor checks passed with skipped checks"
		}

		return "Doctor checks passed"
	}
}

func runDoctor(cmd *cobra.Command, currentVersion string, verbose bool) error {
	opts, _ := output.ParseOptions(cmd)
	quiet := opts.Quiet
	out := cmd.OutOrStdout()
	if !quiet {
		_, _ = fmt.Fprint(out, bannerAndurel)
		_, _ = fmt.Fprint(out, bannerDoctor)
		_, _ = fmt.Fprintln(out, "Running Andurel project diagnostics...")
	}

	var results []checkResult
	hasErrors := false
	hasWarnings := false

	// Environment checks
	if !quiet {
		_, _ = fmt.Fprintln(out, "\n=== Environment ===")
	}
	environmentResults := []checkResult{
		checkGoVersion(),
		checkLatestAndurelRelease(currentVersion),
		checkInAndurelProject(),
	}
	results = append(results, environmentResults...)
	printResults(out, environmentResults, verbose, quiet)

	// Get project root directory
	rootDir, err := findGoModRoot()
	if err != nil {
		// If we can't find the project root, we can't continue with remaining checks
		_, _ = fmt.Fprintf(out, "\n✗ Cannot continue: %v\n", err)
		return err
	}

	// Configuration checks
	if !quiet {
		_, _ = fmt.Fprintln(out, "\n=== Configuration ===")
	}
	configResults := []checkResult{
		checkLockFile(rootDir),
		checkAndurelVersion(rootDir, currentVersion),
		checkToolVersions(rootDir, verbose),
		checkNarsilcRuntime(rootDir),
		checkProjectSetup(rootDir),
		checkManualActions(rootDir, currentVersion),
		checkInertiaSSRConfiguration(rootDir),
	}
	results = append(results, configResults...)
	printResults(out, configResults, verbose, quiet)

	// Code quality checks
	if !quiet {
		_, _ = fmt.Fprintln(out, "\n=== Code Quality ===")
	}
	qualityResults := []checkResult{
		checkGoVet(rootDir, verbose),
		checkGoModTidy(rootDir, verbose),
	}
	results = append(results, qualityResults...)
	printResults(out, qualityResults, verbose, quiet)

	// Code generation checks
	if !quiet {
		_, _ = fmt.Fprintln(out, "\n=== Code Generation ===")
	}
	genResults := codeGenerationChecks(rootDir, verbose)
	results = append(results, genResults...)
	printResults(out, genResults, verbose, quiet)

	// Summary
	passCount := 0
	warnCount := 0
	failCount := 0
	skipCount := 0

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
		case statusSkip:
			skipCount++
		}
	}

	_, _ = fmt.Fprintln(out, "\n=== Summary ===")
	_, _ = fmt.Fprintf(out, "Total checks: %d\n", len(results))
	_, _ = fmt.Fprintf(out, "✓ Passed: %d\n", passCount)
	if warnCount > 0 {
		_, _ = fmt.Fprintf(out, "⚠ Warnings: %d\n", warnCount)
	}
	if failCount > 0 {
		_, _ = fmt.Fprintf(out, "✗ Failed: %d\n", failCount)
	}
	if skipCount > 0 {
		_, _ = fmt.Fprintf(out, "- Skipped: %d\n", skipCount)
	}

	if hasErrors {
		_, _ = fmt.Fprintln(out, "\n✗ Some checks failed. Please address the issues above.")
		return fmt.Errorf("doctor checks failed")
	} else if hasWarnings {
		if !quiet {
			_, _ = fmt.Fprintln(out, "\n⚠ All critical checks passed, but there are warnings to review.")
		}
	} else {
		if !quiet {
			_, _ = fmt.Fprintln(out, "\n✓ All checks passed! Your project is healthy.")
		}
	}

	return nil
}

func printResults(out io.Writer, results []checkResult, verbose bool, quiet bool) {
	for _, r := range results {
		if quiet && r.status == statusPass {
			continue
		}
		var symbol string
		switch r.status {
		case statusPass:
			symbol = "✓"
		case statusWarn:
			symbol = "⚠"
		case statusFail:
			symbol = "✗"
		case statusSkip:
			symbol = "-"
		}

		_, _ = fmt.Fprintf(out, "%s %s", symbol, r.name)
		if r.message != "" {
			_, _ = fmt.Fprintf(out, ": %s", r.message)
		}
		_, _ = fmt.Fprintln(out)

		// Details explain why a check did not pass, so they are printed without
		// --verbose. Passing checks keep their details behind the flag.
		if len(r.details) > 0 && (verbose || r.status != statusPass) {
			for _, detail := range r.details {
				_, _ = fmt.Fprintf(out, "  %s\n", detail)
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
			message: "could not check the latest release",
			details: []string{err.Error()},
			hint:    "Check network connectivity and run andurel doctor again.",
		}
	}

	if newerAndurelVersion(current, latest) {
		return checkResult{
			name:    "Andurel release",
			status:  statusWarn,
			message: fmt.Sprintf("%s is available (current: %s)", latest, current),
			hint: fmt.Sprintf(
				"Run '%s', then run 'andurel upgrade'.",
				andurelInstallCommand(latest),
			),
		}
	}
	if current != latest {
		return checkResult{
			name:   "Andurel release",
			status: statusPass,
			message: fmt.Sprintf(
				"no newer release found (current: %s, latest published: %s)",
				current,
				latest,
			),
		}
	}

	return checkResult{
		name:    "Andurel release",
		status:  statusPass,
		message: fmt.Sprintf("latest release installed (%s)", current),
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
	tomlPath := filepath.Join(rootDir, layout.ProjectTomlName)
	if _, err := os.Stat(tomlPath); err != nil {
		return checkResult{
			name:    layout.ProjectTomlName,
			status:  statusFail,
			message: "file not found",
		}
	}

	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		return checkResult{
			name:    layout.ProjectTomlName,
			status:  statusFail,
			message: fmt.Sprintf("invalid format: %v", err),
		}
	}

	return checkResult{
		name:    layout.ProjectTomlName,
		status:  statusPass,
		message: fmt.Sprintf("valid (version: %s, %d tools)", lock.Version, len(lock.Tools)),
	}
}

func checkManualActions(rootDir, currentVersion string) checkResult {
	lock, err := layout.ReadLockFile(rootDir)
	if err != nil || lock.Version == "" {
		return checkResult{
			name:    "Manual actions",
			status:  statusSkip,
			message: "andurel.lock has no framework version (skipping manual action checks)",
		}
	}
	inertia := lock.ScaffoldConfig != nil &&
		layout.IsSupportedInertiaAdapter(lock.ScaffoldConfig.Inertia)
	checks := upgrade.CheckManualActions(rootDir, lock.Version, currentVersion, inertia)
	if len(checks) == 0 {
		return checkResult{
			name:    "Manual actions",
			status:  statusPass,
			message: "no manual actions apply to version " + lock.Version,
		}
	}

	applied, notApplied, notApplicable := 0, 0, 0
	details := make([]string, 0, len(checks))
	for _, check := range checks {
		switch check.Status {
		case upgrade.ActionApplied:
			applied++
		case upgrade.ActionNotApplied:
			notApplied++
		case upgrade.ActionNotApplicable:
			notApplicable++
		}
		detail := fmt.Sprintf("%s: %s", check.ID, check.Status)
		if check.Detail != "" {
			detail += " (" + check.Detail + ")"
		}
		details = append(details, detail)
	}

	status := statusPass
	if notApplied > 0 {
		status = statusWarn
	}
	return checkResult{
		name:          "Manual actions",
		status:        status,
		message:       fmt.Sprintf("%d applied, %d not applied, %d not applicable", applied, notApplied, notApplicable),
		details:       details,
		manualActions: checks,
	}
}

func checkInertiaSSRConfiguration(rootDir string) checkResult {
	lock, err := layout.ReadLockFile(rootDir)
	if err != nil || lock.ScaffoldConfig == nil ||
		!layout.IsSupportedInertiaAdapter(lock.ScaffoldConfig.Inertia) {
		return checkResult{name: "Inertia SSR", status: statusPass, message: "not configured"}
	}

	values := readDoctorEnv(filepath.Join(rootDir, ".env.example"))
	maps.Copy(values, readDoctorEnv(filepath.Join(rootDir, ".env")))

	if _, err := os.Stat(filepath.Join(rootDir, "cmd", "ssr", "main.go")); err != nil {
		return checkResult{
			name:    "Inertia SSR",
			status:  statusFail,
			message: "cmd/ssr/main.go missing (Inertia SSR entrypoint)",
		}
	}

	rawURL := strings.TrimSpace(values["INERTIA_SSR_URL"])
	if rawURL == "" {
		rawURL = "http://127.0.0.1:13714"
	}
	if parsedURL, err := url.Parse(rawURL); err != nil || parsedURL.Host == "" ||
		(parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return checkResult{
			name:    "Inertia SSR",
			status:  statusFail,
			message: "SSR renderer URL is invalid",
		}
	}

	rawListen := strings.TrimSpace(values["INERTIA_SSR_LISTEN"])
	if rawListen == "" {
		rawListen = "http://127.0.0.1:13714"
	}
	healthURL, err := doctorSSRListenHealthURL(rawListen)
	if err != nil {
		return checkResult{
			name:    "Inertia SSR",
			status:  statusFail,
			message: err.Error(),
		}
	}

	executable := strings.TrimSpace(values["INERTIA_SSR_RUNTIME"])
	if executable == "" {
		executable = "node"
	}
	if _, err := exec.LookPath(executable); err != nil {
		return checkResult{
			name:    "Inertia SSR",
			status:  statusFail,
			message: fmt.Sprintf("runtime %q not found", executable),
		}
	}

	if doctorUsesDevelopmentSSR(values) {
		return checkResult{
			name:    "Inertia SSR",
			status:  statusPass,
			message: "development SSR is served by Vite; production bundle not required",
		}
	}

	bundle := strings.TrimSpace(values["INERTIA_SSR_BUNDLE"])
	if bundle == "" {
		bundle = "assets/dist/ssr/ssr.js"
	}
	if _, err := os.Stat(filepath.Join(rootDir, filepath.FromSlash(bundle))); err != nil {
		return checkResult{
			name:    "Inertia SSR",
			status:  statusFail,
			message: fmt.Sprintf("SSR bundle %s is required in production (cmd/ssr needs it)", bundle),
			hint:    "Run pnpm build (or andurel build) and ship assets/dist/ssr/ssr.js with cmd/ssr.",
		}
	}

	if err := checkSSRHealth(healthURL); err != nil {
		return checkResult{
			name:   "Inertia SSR",
			status: statusWarn,
			message: fmt.Sprintf(
				"cmd/ssr renderer unreachable at %s: %v",
				healthURL.Redacted(),
				err,
			),
		}
	}
	return checkResult{
		name:    "Inertia SSR",
		status:  statusPass,
		message: fmt.Sprintf("cmd/ssr ready; renderer healthy at %s", healthURL.Redacted()),
	}
}

func doctorUsesDevelopmentSSR(values map[string]string) bool {
	environment := strings.TrimSpace(os.Getenv("ENVIRONMENT"))
	if environment == "" {
		environment = strings.TrimSpace(values["ENVIRONMENT"])
	}
	if environment == "" {
		environment = "development"
	}
	return environment == "development"
}

// doctorSSRListenHealthURL rewrites unspecified bind hosts to the same-family
// loopback so doctor can probe Node on this machine. Keep in sync with
// pkg/inertia parseSSRListen.
func doctorSSRListenHealthURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.Scheme != "http" {
		return nil, fmt.Errorf("SSR listen URL is invalid")
	}

	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		return nil, fmt.Errorf("SSR listen URL must include a port")
	}
	if !strings.EqualFold(host, "localhost") && net.ParseIP(host) == nil {
		return nil, fmt.Errorf(
			"SSR listen URL must bind an IP address or localhost, not a service hostname",
		)
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		if ip.To4() != nil {
			host = "127.0.0.1"
		} else {
			host = "::1"
		}
	}

	return url.Parse("http://" + net.JoinHostPort(host, port))
}

func checkSSRHealth(baseURL *url.URL) error {
	endpoint := *baseURL
	endpoint.Path = strings.TrimSuffix(endpoint.Path, "/") + "/health"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	return nil
}

func readDoctorEnv(path string) map[string]string {
	values := make(map[string]string)
	content, err := os.ReadFile(path)
	if err != nil {
		return values
	}
	for line := range strings.SplitSeq(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return values
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

func checkNarsilcRuntime(rootDir string) checkResult {
	const name = "narsilc runtime"

	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		return checkResult{
			name:    name,
			status:  statusSkip,
			message: "andurel.lock missing or invalid (skipping narsilc runtime check)",
		}
	}
	tool, configured := lock.Tools["narsilc"]
	if !configured || tool == nil {
		return checkResult{
			name:    name,
			status:  statusPass,
			message: "narsilc is not configured in andurel.lock",
		}
	}

	state := inspectNarsilcRuntime(rootDir, tool.Version)
	switch state.Status {
	case narsilcRuntimeAligned:
		return checkResult{
			name:    name,
			status:  statusPass,
			message: fmt.Sprintf("go.mod matches narsilc tool %s", state.ToolVersion),
		}
	case narsilcRuntimeMissing:
		return checkResult{
			name:   name,
			status: statusPass,
			message: fmt.Sprintf(
				"narsilc tool %s is configured; go.mod has no narsilc runtime requirement",
				state.ToolVersion,
			),
		}
	case narsilcRuntimeMismatch:
		details := []string{state.Detail, "Run:"}
		details = append(details, state.remediation()...)
		return checkResult{
			name:   name,
			status: statusWarn,
			message: fmt.Sprintf(
				"narsilc tool %s, go.mod requires %s",
				state.ToolVersion,
				state.RequiredVersion,
			),
			details: details,
			hint: fmt.Sprintf(
				"Run '%s', then 'go mod tidy' to align the runtime with the narsilc tool.",
				state.remediation()[0],
			),
		}
	case narsilcRuntimeReplaced:
		return checkResult{
			name:   name,
			status: statusWarn,
			message: fmt.Sprintf(
				"narsilc runtime is replaced; alignment with narsilc tool %s could not be verified",
				state.ToolVersion,
			),
			details: []string{state.Detail},
			hint:    "Review the github.com/mbvlabs/narsilc replace directive in go.mod and confirm it matches the narsilc tool version.",
		}
	default:
		return checkResult{
			name:    name,
			status:  statusWarn,
			message: "narsilc runtime alignment could not be verified",
			details: []string{state.Detail},
			hint:    "Inspect go.mod and confirm the github.com/mbvlabs/narsilc requirement is readable and has a valid module version.",
		}
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

func versionFromExecutable(
	fullPath string,
	vc *layout.VersionCheck,
	toolName string,
) (string, error) {
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
		if strings.Contains(lower, "update available") ||
			strings.Contains(lower, "new version available") {
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
	output, err := runGoVet(rootDir)
	if err != nil {
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
			name:        "templ generate",
			status:      statusWarn,
			message:     "templ binary not found (skipping check)",
			missingTool: "templ",
		}
	}

	var changed []string
	err := withDiagnosticProjectCopyTools(rootDir, []string{"templ"}, func(tempRoot string) error {
		before, err := snapshotFilesForReport(tempRoot)
		if err != nil {
			return err
		}
		cmd := exec.Command(filepath.Join(tempRoot, "bin", "templ"), templGenerateArgs...)
		cmd.Dir = tempRoot
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf(
				"templ generate failed: %w: %s",
				err,
				strings.TrimSpace(stderr.String()),
			)
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
			name:    "templ generate",
			status:  statusFail,
			message: "temporary generation diagnostic failed",
			details: []string{err.Error()},
		}
	}
	if len(changed) > 0 {
		details := []string{"Commit the generated *_templ.go output; andurel build regenerates it."}
		if verbose {
			details = append(details, changed...)
		}
		return checkResult{
			name:    "templ generate",
			status:  statusFail,
			message: "generated templates are out of date",
			details: details,
			changed: changed,
		}
	}

	return checkResult{
		name:    "templ generate",
		status:  statusPass,
		message: "templates generated successfully",
	}
}

// checkEmailsExport compares emails/_generated props/manifest and the committed
// email/templates to a fresh generation and export. The export needs installed
// frontend dependencies, so a project without node_modules reports a skip with
// the install command instead of a failure.
func checkEmailsExport(rootDir string, verbose bool) checkResult {
	name := "emails export"
	if !hasReactEmails(rootDir) {
		return checkResult{
			name:    name,
			status:  statusPass,
			message: "no React Email templates configured",
		}
	}
	if info, err := os.Stat(filepath.Join(rootDir, "email")); err != nil || !info.IsDir() {
		return checkResult{
			name:    name,
			status:  statusPass,
			message: "no React Email templates configured",
		}
	}

	packageManager := frontendPackageManager(rootDir)
	var changed []string
	exportSkipped := false
	skipReason := ""

	err := withDiagnosticProjectCopyTools(rootDir, nil, func(tempRoot string) error {
		if _, err := generateEmailsFiles(tempRoot); err != nil {
			return fmt.Errorf("email props generation failed: %w", err)
		}
		changed = append(changed, compareEmailsGeneratedFiles(rootDir, tempRoot)...)
		if len(changed) > 0 {
			return nil
		}

		nodeModules := filepath.Join(rootDir, "node_modules")
		if info, err := os.Stat(nodeModules); err != nil || !info.IsDir() {
			exportSkipped = true
			skipReason = "node_modules is missing"
			return nil
		}
		if _, err := exec.LookPath(packageManager); err != nil {
			exportSkipped = true
			skipReason = packageManager + " is not installed"
			return nil
		}

		if err := os.Symlink(nodeModules, filepath.Join(tempRoot, "node_modules")); err != nil {
			return fmt.Errorf("link frontend dependencies for export: %w", err)
		}
		var stdout, stderr bytes.Buffer
		if err := runEmailsExport(
			context.Background(),
			tempRoot,
			&stdout,
			&stderr,
		); err != nil {
			return fmt.Errorf(
				"email export failed: %w: %s",
				err,
				strings.TrimSpace(stderr.String()),
			)
		}
		templatesChanged, err := compareEmailTemplateFiles(rootDir, tempRoot)
		if err != nil {
			return err
		}
		changed = append(changed, templatesChanged...)
		return nil
	})
	if err != nil {
		return checkResult{
			name:    name,
			status:  statusFail,
			message: "temporary email export diagnostic failed",
			details: []string{err.Error()},
		}
	}
	if len(changed) > 0 {
		details := []string{
			"Run 'andurel sync emails' and commit the updated emails/_generated and email/templates files.",
		}
		if verbose {
			details = append(details, changed...)
		}
		return checkResult{
			name:    name,
			status:  statusFail,
			message: "generated email output is out of date",
			details: details,
			changed: changed,
		}
	}
	if exportSkipped {
		fix := "Run '" + packageManager + " install' to check email/templates drift."
		return checkResult{
			name:        name,
			status:      statusSkip,
			message:     skipReason + " (skipping export check)",
			details:     []string{fix},
			hint:        fix,
			missingTool: "node_modules",
		}
	}

	return checkResult{
		name:    name,
		status:  statusPass,
		message: "email props and templates match emails/",
	}
}

func compareEmailsGeneratedFiles(rootDir, tempRoot string) []string {
	changed := make([]string, 0, 2)
	for _, path := range []string{generatedEmailsPropsTSPath, generatedEmailsManifestPath} {
		expected, err := os.ReadFile(filepath.Join(tempRoot, filepath.FromSlash(path)))
		if err != nil {
			changed = append(changed, path)
			continue
		}
		actual, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(path)))
		if err != nil || !bytes.Equal(actual, expected) {
			changed = append(changed, path)
		}
	}
	return changed
}

func compareEmailTemplateFiles(rootDir, tempRoot string) ([]string, error) {
	expected, err := snapshotOptionalDir(filepath.Join(tempRoot, "email", "templates"))
	if err != nil {
		return nil, err
	}
	actual, err := snapshotOptionalDir(filepath.Join(rootDir, "email", "templates"))
	if err != nil {
		return nil, err
	}
	paths := changedSnapshotPaths(actual, expected)
	for i := range paths {
		paths[i] = "email/templates/" + paths[i]
	}
	return paths, nil
}

func snapshotOptionalDir(dir string) (fileSnapshot, error) {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return fileSnapshot{}, nil
		}
		return nil, err
	}
	return snapshotFilesForReport(dir)
}

func checkNarsilcGenerate(rootDir string, verbose bool) checkResult {
	hasQueries, err := cmds.HasQueryFiles(rootDir)
	if err != nil {
		return checkResult{
			name:    "narsilc generate",
			status:  statusFail,
			message: "failed to inspect narsilc query files",
			details: []string{err.Error()},
		}
	}
	if !hasQueries {
		return checkResult{
			name:    "narsilc generate",
			status:  statusPass,
			message: "no narsilc queries configured",
		}
	}

	narsilcPath := filepath.Join(rootDir, "bin", "narsilc")
	if _, err := os.Stat(narsilcPath); err != nil {
		return checkResult{
			name:        "narsilc generate",
			status:      statusWarn,
			message:     "narsilc binary not found (skipping check)",
			missingTool: "narsilc",
		}
	}

	var changed []string
	var generateErr error
	err = withDiagnosticProjectCopyTools(rootDir, []string{"narsilc"}, func(tempRoot string) error {
		before, err := snapshotFilesForReport(tempRoot)
		if err != nil {
			return err
		}
		cmd := exec.Command(filepath.Join(tempRoot, "bin", "narsilc"), "generate")
		cmd.Dir = tempRoot
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			// narsilc checks the queries against migrations/, so a failed run is
			// reported as a query problem with narsilc's own output attached,
			// not as a failed diagnostic copy.
			generateErr = fmt.Errorf(
				"narsilc generate failed: %w: %s",
				err,
				strings.TrimSpace(stderr.String()),
			)
			return nil
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
			name:    "narsilc generate",
			status:  statusFail,
			message: "temporary narsilc generation diagnostic failed",
			details: []string{err.Error()},
		}
	}
	if generateErr != nil {
		return checkResult{
			name:    "narsilc generate",
			status:  statusFail,
			message: querySchemaMessage,
			details: []string{generateErr.Error()},
			hint:    querySchemaHint,
		}
	}
	if len(changed) > 0 {
		details := []string{"Run 'andurel sync queries' and commit the generated output."}
		if verbose {
			details = append(details, changed...)
		}
		return checkResult{
			name:    "narsilc generate",
			status:  statusFail,
			message: "generated narsilc code is out of date",
			details: details,
			changed: changed,
		}
	}

	return checkResult{
		name:    "narsilc generate",
		status:  statusPass,
		message: "narsilc code generated successfully",
	}
}

func changedSnapshotPaths(before, after fileSnapshot) []string {
	changed := make([]string, 0)
	for path, state := range after {
		if previous, ok := before[path]; !ok || previous.Hash != state.Hash ||
			previous.Mode != state.Mode {
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
		checkEmailsExport(rootDir, verbose),
		checkNarsilcGenerate(rootDir, verbose),
	}
	if projectUsesInertia(rootDir) {
		results = append(results, checkRoutesTSGenerate(rootDir, verbose))
		results = append(results, checkPayloadsTSGenerate(rootDir, verbose))
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
	err := withDiagnosticProjectCopyTools(rootDir, nil, func(tempRoot string) error {
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
				details: []string{"Run 'andurel sync routes' to create it."},
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
		details := []string{"Run 'andurel sync routes' to update resources/js/routes.ts."}
		if verbose {
			details = append(
				details,
				fmt.Sprintf("expected %d bytes, found %d bytes", len(expected), len(actual)),
			)
			if skippedCount > 0 {
				details = append(
					details,
					fmt.Sprintf("%d route manifest entries were skipped", skippedCount),
				)
			}
		}
		return checkResult{
			name:    "routes.ts",
			status:  statusFail,
			message: "resources/js/routes.ts is out of date",
			details: details,
			changed: []string{generatedRoutesJSPath},
		}
	}

	return checkResult{
		name:    "routes.ts",
		status:  statusPass,
		message: fmt.Sprintf("matches route manifest (%d helpers)", helperCount),
	}
}

func checkPayloadsTSGenerate(rootDir string, verbose bool) checkResult {
	var expected []byte
	var typeCount int
	err := withDiagnosticProjectCopyTools(rootDir, nil, func(tempRoot string) error {
		report, err := generatePayloadsFile(tempRoot)
		if err != nil {
			return err
		}
		typeCount = report.TypeCount
		expected, err = os.ReadFile(filepath.Join(tempRoot, generatedPayloadsTSPath))
		return err
	})
	if err != nil {
		return checkResult{
			name:    "payloads.ts",
			status:  statusFail,
			message: "temporary payload generation diagnostic failed",
			details: []string{err.Error()},
		}
	}

	target := filepath.Join(rootDir, generatedPayloadsTSPath)
	actual, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return checkResult{
				name:    "payloads.ts",
				status:  statusFail,
				message: "resources/js/types/payloads.ts is missing",
				details: []string{"Run 'andurel sync payloads' to create it."},
			}
		}
		return checkResult{
			name:    "payloads.ts",
			status:  statusFail,
			message: "could not read resources/js/types/payloads.ts",
			details: []string{err.Error()},
		}
	}

	if !bytes.Equal(actual, expected) {
		details := []string{"Run 'andurel sync payloads' to update resources/js/types/payloads.ts."}
		if verbose {
			details = append(
				details,
				fmt.Sprintf("expected %d bytes, found %d bytes", len(expected), len(actual)),
			)
		}
		return checkResult{
			name:    "payloads.ts",
			status:  statusFail,
			message: "resources/js/types/payloads.ts is out of date",
			details: details,
			changed: []string{generatedPayloadsTSPath},
		}
	}

	return checkResult{
		name:    "payloads.ts",
		status:  statusPass,
		message: fmt.Sprintf("matches payloads/ (%d types)", typeCount),
	}
}
