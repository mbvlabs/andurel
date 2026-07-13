package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/layout"
)

func TestDoctorBuildReportSummaryHintsAndDetails(t *testing.T) {
	results := []checkResult{
		{name: "Go version", category: "environment", status: statusPass, message: "go1.26.4"},
		{name: "tool versions", category: "configuration", status: statusWarn, message: "1 missing", details: []string{"templ: missing"}},
		{name: "go vet", category: "code_quality", status: statusFail, message: "2 issues"},
	}

	report := buildDoctorReport("v1.0.0", "/repo", results)

	if report.Summary.Total != 3 || report.Summary.Passed != 1 || report.Summary.Warnings != 1 ||
		report.Summary.Failed != 1 || report.Summary.Status != "fail" {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if got, want := doctorSummaryMessage(report), "Doctor checks failed"; got != want {
		t.Fatalf("doctorSummaryMessage = %q, want %q", got, want)
	}
	if !report.Checks[2].Blocking {
		t.Fatalf("failed check should be blocking: %#v", report.Checks[2])
	}
	if !strings.Contains(report.Checks[1].Hint, "tool sync") {
		t.Fatalf("expected tool sync hint, got %#v", report.Checks[1])
	}
	if !reflect.DeepEqual(report.Checks[1].Details, []string{"templ: missing"}) {
		t.Fatalf("details were not copied: %#v", report.Checks[1].Details)
	}

	results[1].details[0] = "mutated"
	if report.Checks[1].Details[0] != "templ: missing" {
		t.Fatalf("doctorCheck details should be copied, got %#v", report.Checks[1].Details)
	}
}

func TestDoctorSummaryMessages(t *testing.T) {
	tests := map[string]string{
		"pass": "Doctor checks passed",
		"warn": "Doctor checks completed with warnings",
		"fail": "Doctor checks failed",
	}
	for status, want := range tests {
		report := doctorReport{Summary: doctorSummary{Status: status}}
		if got := doctorSummaryMessage(report); got != want {
			t.Fatalf("doctorSummaryMessage(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestDoctorVersionHelpers(t *testing.T) {
	if got, want := normalizeVersion(" v1.2.3 "), "1.2.3"; got != want {
		t.Fatalf("normalizeVersion = %q, want %q", got, want)
	}
	if !versionsMatch("v1.2.3", "1.2.3") {
		t.Fatalf("expected normalized versions to match")
	}
	if versionsMatch("", "1.2.3") || versionsMatch("1.2.3", "") {
		t.Fatalf("empty versions should not match")
	}

	output := "templ version v0.3.1\nUpdate available: v0.9.0\n"
	if got, want := extractVersion(output), "v0.3.1"; got != want {
		t.Fatalf("extractVersion = %q, want %q", got, want)
	}
	if got := extractVersion("Update available: v9.9.9\n"); got != "v9.9.9" {
		t.Fatalf("fallback extractVersion = %q", got)
	}
}

func TestTruncateDetails(t *testing.T) {
	details := []string{"one", "two", "three", "four", "five"}
	truncated := truncateDetails(details, 3)
	want := []string{"one", "two", "three", "... and 2 more (use --verbose to see all)"}
	if !reflect.DeepEqual(truncated, want) {
		t.Fatalf("truncateDetails = %#v, want %#v", truncated, want)
	}
	if !reflect.DeepEqual(truncateDetails(details[:2], 3), details[:2]) {
		t.Fatalf("short details should be unchanged")
	}
}

func TestDoctorLockAndVersionChecks(t *testing.T) {
	root := t.TempDir()

	missing := checkLockFile(root)
	if missing.status != statusFail || missing.message != "file not found" {
		t.Fatalf("missing lock check = %#v", missing)
	}

	if err := os.WriteFile(filepath.Join(root, "andurel.lock"), []byte("{"), 0o644); err != nil {
		t.Fatalf("write invalid lock: %v", err)
	}
	invalid := checkLockFile(root)
	if invalid.status != statusFail || !strings.Contains(invalid.message, "invalid format") {
		t.Fatalf("invalid lock check = %#v", invalid)
	}

	lock := layout.NewAndurelLock("v1.2.3")
	lock.Tools["templ"] = validTestTool("templ", "v0.3.1")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	valid := checkLockFile(root)
	if valid.status != statusPass || !strings.Contains(valid.message, "1 tools") {
		t.Fatalf("valid lock check = %#v", valid)
	}

	version := checkAndurelVersion(root, "1.2.3")
	if version.status != statusPass {
		t.Fatalf("matching version check = %#v", version)
	}
	mismatch := checkAndurelVersion(root, "1.2.4")
	if mismatch.status != statusWarn || !strings.Contains(mismatch.message, "current is 1.2.4") {
		t.Fatalf("mismatch version check = %#v", mismatch)
	}

	tools := checkToolVersions(root, false)
	if tools.status != statusWarn || !strings.Contains(tools.message, "1 missing") {
		t.Fatalf("missing tool check = %#v", tools)
	}
}

func TestDoctorProjectDetection(t *testing.T) {
	root := t.TempDir()
	originalFindGoModRoot := findGoModRoot
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	findGoModRoot = func() (string, error) {
		return "", os.ErrNotExist
	}
	missing := checkInAndurelProject()
	if missing.status != statusFail || !strings.Contains(missing.message, "go.mod not found") {
		t.Fatalf("missing project check = %#v", missing)
	}

	findGoModRoot = func() (string, error) {
		return root, nil
	}
	found := checkInAndurelProject()
	if found.status != statusPass || found.message != "found go.mod" {
		t.Fatalf("found project check = %#v", found)
	}

	if version := checkGoVersion(); version.status != statusPass || version.message != runtime.Version() {
		t.Fatalf("go version check = %#v", version)
	}
}

func TestRunDoctorStructuredPassAndFail(t *testing.T) {
	stubLatestAndurelVersion(t, "v1.2.3", nil)
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	writeExecutable(t, root, "bin/templ", "#!/bin/sh\nexit 0\n")
	lock := layout.NewAndurelLock("v1.2.3")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) {
		return root, nil
	}
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	var out bytes.Buffer
	cmd := newStructuredTestCommand(&out)
	if err := runDoctorStructured(cmd, "1.2.3", false); err != nil {
		t.Fatalf("runDoctorStructured pass: %v", err)
	}
	var envelope output.Envelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode doctor envelope: %v\n%s", err, out.String())
	}
	if envelope.Summary != "Doctor checks passed" {
		t.Fatalf("unexpected doctor summary: %q", envelope.Summary)
	}

	findGoModRoot = func() (string, error) {
		return "", os.ErrNotExist
	}
	if err := runDoctorStructured(cmd, "1.2.3", false); err == nil {
		t.Fatalf("expected structured doctor to fail outside project")
	}
}

func TestPrintResults(t *testing.T) {
	capture := captureProcessOutput(t, &os.Stdout)
	printResults([]checkResult{
		{name: "pass", status: statusPass, message: "ok", details: []string{"hidden"}},
		{name: "warn", status: statusWarn, message: "review", details: []string{"visible"}},
		{name: "fail", status: statusFail, message: "broken"},
		{name: "unknown", status: checkStatus(99)},
	}, true)
	out := capture()
	for _, want := range []string{"pass: ok", "warn: review", "visible", "fail: broken", "unknown"} {
		if !strings.Contains(out, want) {
			t.Fatalf("printResults output missing %q:\n%s", want, out)
		}
	}
}

func TestDoctorGoVetAndTidyChecks(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {}\n")

	vet := checkGoVet(root, false)
	if vet.status != statusPass {
		t.Fatalf("expected go vet pass, got %#v", vet)
	}

	tidy := checkGoModTidy(root, false)
	if tidy.status != statusPass {
		t.Fatalf("expected go mod tidy pass, got %#v", tidy)
	}

	writeTestFile(t, root, "bad.go", "package main\n\nimport \"fmt\"\n\nfunc bad() { fmt.Printf(\"%d\", \"x\") }\n")
	vet = checkGoVet(root, true)
	if vet.status != statusFail || !strings.Contains(vet.message, "issues found") || len(vet.details) == 0 {
		t.Fatalf("expected go vet failure with details, got %#v", vet)
	}

	missing := checkGoModTidy(filepath.Join(root, "missing"), false)
	if missing.status != statusFail || !strings.Contains(missing.message, "cannot read go.mod") {
		t.Fatalf("expected go mod tidy read failure, got %#v", missing)
	}
}

func TestDoctorTemplGenerateChecks(t *testing.T) {
	root := t.TempDir()

	warn := checkTemplGenerate(root, false)
	if warn.status != statusWarn || !strings.Contains(warn.message, "templ binary not found") {
		t.Fatalf("missing templ check = %#v", warn)
	}

	writeExecutable(t, root, "bin/templ", "#!/bin/sh\nexit 0\n")
	pass := checkTemplGenerate(root, false)
	if pass.status != statusPass {
		t.Fatalf("templ pass check = %#v", pass)
	}

	writeExecutable(t, root, "bin/templ", "#!/bin/sh\necho broken >&2\nexit 1\n")
	fail := checkTemplGenerate(root, true)
	if fail.status != statusFail || !strings.Contains(fail.details[0], "broken") {
		t.Fatalf("templ failure check = %#v", fail)
	}
}

func TestDoctorVersionCommandHelpers(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeExecutable(t, root, "bin/tool", "#!/bin/sh\necho tool version v1.2.3\n")

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) {
		return root, nil
	}
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	version, err := getToolVersion("bin/tool", &layout.VersionCheck{Args: []string{"--version"}}, "tool")
	if err != nil {
		t.Fatalf("getToolVersion: %v", err)
	}
	if version != "v1.2.3" {
		t.Fatalf("tool version = %q", version)
	}
	writeExecutable(t, root, "bin/custom", "#!/bin/sh\necho release-build-42\n")
	customVersion, err := getToolVersion(
		"bin/custom",
		&layout.VersionCheck{Args: []string{"--version"}, Regexp: `release-build-([0-9]+)`},
		"custom",
	)
	if err != nil {
		t.Fatalf("configured regexp: %v", err)
	}
	if customVersion != "42" {
		t.Fatalf("configured regexp version = %q", customVersion)
	}

	if _, err := versionFromCommand("bin/tool", nil, "tool"); err == nil {
		t.Fatalf("expected missing version check error")
	}
	writeExecutable(t, root, "bin/empty", "#!/bin/sh\nexit 0\n")
	if _, err := versionFromCommand("bin/empty", &layout.VersionCheck{Args: []string{"--version"}}, "empty"); err == nil {
		t.Fatalf("expected empty version output error")
	}
}

func TestRunWithTimeout(t *testing.T) {
	root := t.TempDir()
	writeExecutable(t, root, "bin/ok", "#!/bin/sh\necho ok\n")
	out, err := runWithTimeout(context.Background(), filepath.Join(root, "bin", "ok"))
	if err != nil {
		t.Fatalf("runWithTimeout ok: %v", err)
	}
	if !strings.Contains(string(out), "ok") {
		t.Fatalf("runWithTimeout output = %q", string(out))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	writeExecutable(t, root, "bin/slow", "#!/bin/sh\nsleep 2\n")
	if _, err := runWithTimeout(ctx, filepath.Join(root, "bin", "slow")); err == nil {
		t.Fatalf("expected canceled context error")
	}
}

func TestDoctorCollectReportAndCodeGenerationChecks(t *testing.T) {
	stubLatestAndurelVersion(t, "v1.2.3", nil)
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	writeExecutable(t, root, "bin/templ", "#!/bin/sh\nexit 0\n")
	lock := layout.NewAndurelLock("v1.2.3")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) {
		return root, nil
	}
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	report, err := collectDoctorReport("1.2.3", true)
	if err != nil {
		t.Fatalf("collectDoctorReport: %v", err)
	}
	if report.Root != root || report.Summary.Status != "pass" || report.Summary.Total == 0 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if projectUsesInertia(root) {
		t.Fatalf("project without inertia config should not use inertia")
	}
	if got := codeGenerationChecks(root, false); len(got) != 1 || got[0].name != "views generate" || got[0].status != statusPass {
		t.Fatalf("codeGenerationChecks = %#v", got)
	}

	lock.ScaffoldConfig = &layout.ScaffoldConfig{ProjectName: "app", Database: "postgresql", Inertia: "react"}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write inertia lock: %v", err)
	}
	if !projectUsesInertia(root) {
		t.Fatalf("project with react inertia config should use inertia")
	}
}

func TestRunDoctorHumanPassWarnAndProjectFailure(t *testing.T) {
	stubLatestAndurelVersion(t, "v1.2.3", nil)
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	lock := layout.NewAndurelLock("v1.2.3")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	fakePath := t.TempDir()
	writeExecutable(t, fakePath, "go", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", fakePath)

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	writeExecutable(t, root, "bin/templ", "#!/bin/sh\nexit 0\n")
	capture := captureProcessOutput(t, &os.Stdout)
	if err := runDoctor("1.2.3", true); err != nil {
		t.Fatalf("runDoctor pass: %v", err)
	}
	if out := capture(); !strings.Contains(out, "All checks passed") {
		t.Fatalf("expected pass summary, got:\n%s", out)
	}

	if err := os.Remove(filepath.Join(root, "bin", "templ")); err != nil {
		t.Fatalf("remove templ: %v", err)
	}
	capture = captureProcessOutput(t, &os.Stdout)
	if err := runDoctor("1.2.3", false); err != nil {
		t.Fatalf("runDoctor warn: %v", err)
	}
	if out := capture(); !strings.Contains(out, "warnings to review") {
		t.Fatalf("expected warning summary, got:\n%s", out)
	}

	findGoModRoot = func() (string, error) { return "", os.ErrNotExist }
	capture = captureProcessOutput(t, &os.Stdout)
	if err := runDoctor("1.2.3", false); err == nil {
		t.Fatalf("expected project failure")
	}
	if out := capture(); !strings.Contains(out, "Cannot continue") {
		t.Fatalf("expected cannot continue output, got:\n%s", out)
	}
}

func TestDoctorToolVersionMismatchesAndUnknowns(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeExecutable(t, root, "bin/templ", "#!/bin/sh\necho templ v0.1.0\n")
	writeExecutable(t, root, "bin/goose", "#!/bin/sh\necho no version here\n")
	lock := layout.NewAndurelLock("v1.2.3")
	lock.Tools["templ"] = &layout.Tool{Version: "v9.9.9", Path: "bin/templ", VersionCheck: &layout.VersionCheck{Args: []string{"--version"}}}
	lock.Tools["goose"] = &layout.Tool{Version: "v1.0.0", Path: "bin/goose", VersionCheck: &layout.VersionCheck{Args: []string{"--version"}}}
	lock.Tools["mailpit"] = &layout.Tool{Version: "v1.0.0", Path: "bin/mailpit", VersionCheck: &layout.VersionCheck{Args: []string{"--version"}}}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	result := checkToolVersions(root, true)
	if result.status != statusWarn || !strings.Contains(result.message, "1 mismatched, 1 missing, 1 unknown") {
		t.Fatalf("unexpected tool version result: %#v", result)
	}
	if len(result.details) != 3 {
		t.Fatalf("expected verbose details for all issues, got %#v", result.details)
	}
}

func writeGoModule(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26.0\n")
}

func writeExecutable(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", rel, err)
	}
}
