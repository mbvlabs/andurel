package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/layout/versions"
)

const testAndurelPackagesGoMod = `module example.com/app

go 1.26.0

require (
	github.com/mbvlabs/andurel/pkg/email v0.3.1
	github.com/mbvlabs/andurel/pkg/storage v0.7.0
	github.com/other/thing v1.0.0
)

require github.com/mbvlabs/andurel/pkg/server v0.3.2 // indirect

replace github.com/mbvlabs/andurel/pkg/email => ./vendor/email
`

func TestListAndurelPackages(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", testAndurelPackagesGoMod)

	packages, err := listAndurelPackages(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("list packages: %v", err)
	}
	if len(packages) != 2 {
		t.Fatalf("packages = %#v, want email and storage", packages)
	}
	if packages[0].Name != "email" || packages[0].Status != packageStatusReplaced ||
		packages[0].Replace != "./vendor/email" {
		t.Fatalf("email package = %#v", packages[0])
	}
	if packages[1].Name != "storage" || packages[1].Current != "v0.7.0" ||
		packages[1].Status != packageStatusCurrent {
		t.Fatalf("storage package = %#v", packages[1])
	}
}

func TestFilterAndurelPackages(t *testing.T) {
	packages := []packageVersion{
		{Name: "email", Path: versions.PkgPrefix + "email"},
		{Name: "storage", Path: versions.PkgPrefix + "storage"},
	}

	selected, err := filterAndurelPackages(packages, []string{"storage", versions.PkgPrefix + "email"})
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if len(selected) != 2 || selected[0].Name != "storage" || selected[1].Name != "email" {
		t.Fatalf("selected = %#v", selected)
	}

	_, err = filterAndurelPackages(packages, []string{"inertia"})
	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != output.CodeUsage {
		t.Fatalf("unknown package error = %v", err)
	}
}

func TestPackagesListJSON(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", testAndurelPackagesGoMod)
	findGoModRoot = func() (string, error) { return root, nil }
	stubLatestModuleVersions(t, map[string]string{
		versions.PkgPrefix + "storage": "v0.7.1",
	})

	var stdout bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"packages", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("packages --json: %v", err)
	}

	report := decodePackageReport(t, stdout.String())
	if report.Action != "list" || report.DryRun {
		t.Fatalf("report = %#v", report)
	}
	if len(report.Packages) != 2 {
		t.Fatalf("packages = %#v", report.Packages)
	}
	if report.Packages[0].Status != packageStatusReplaced {
		t.Fatalf("email status = %q", report.Packages[0].Status)
	}
	if report.Packages[1].Status != packageStatusOutdated ||
		report.Packages[1].Latest != "v0.7.1" {
		t.Fatalf("storage package = %#v", report.Packages[1])
	}
}

func TestPackagesUpdateDryRunDoesNotApply(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", testAndurelPackagesGoMod)
	findGoModRoot = func() (string, error) { return root, nil }
	stubLatestModuleVersions(t, map[string]string{
		versions.PkgPrefix + "storage": "v0.7.1",
	})
	applied := false
	applyAndurelPackageUpdatesFunc = func(context.Context, string, []string) error {
		applied = true
		return nil
	}

	var stdout bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"packages", "update", "--dry-run", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("packages update --dry-run: %v", err)
	}
	if applied {
		t.Fatal("dry-run must not apply updates")
	}

	report := decodePackageReport(t, stdout.String())
	if !report.DryRun || report.Action != "update" || len(report.CommandsRun) != 0 {
		t.Fatalf("report = %#v", report)
	}
	if report.Packages[1].Status != packageStatusOutdated {
		t.Fatalf("storage status = %q", report.Packages[1].Status)
	}
}

func TestPackagesUpdateAppliesOutdatedModules(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", testAndurelPackagesGoMod)
	findGoModRoot = func() (string, error) { return root, nil }
	stubLatestModuleVersions(t, map[string]string{
		versions.PkgPrefix + "storage": "v0.7.1",
	})
	var specs []string
	applyAndurelPackageUpdatesFunc = func(_ context.Context, dir string, got []string) error {
		if dir != root {
			t.Fatalf("dir = %q, want %q", dir, root)
		}
		specs = append([]string(nil), got...)
		return nil
	}

	var stdout bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"packages", "update", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("packages update: %v", err)
	}
	if len(specs) != 1 || specs[0] != versions.PkgPrefix+"storage@v0.7.1" {
		t.Fatalf("specs = %#v", specs)
	}

	report := decodePackageReport(t, stdout.String())
	if report.Packages[1].Status != packageStatusUpdated {
		t.Fatalf("storage status = %q", report.Packages[1].Status)
	}
	if len(report.CommandsRun) != 2 {
		t.Fatalf("commands = %#v", report.CommandsRun)
	}
}

func TestPackagesUpdateSelectedName(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeTestFile(
		t,
		root,
		"go.mod",
		`module example.com/app

go 1.26.0

require (
	github.com/mbvlabs/andurel/pkg/email v0.3.1
	github.com/mbvlabs/andurel/pkg/storage v0.7.0
)
`,
	)
	findGoModRoot = func() (string, error) { return root, nil }
	stubLatestModuleVersions(t, map[string]string{
		versions.PkgPrefix + "email":   "v0.3.2",
		versions.PkgPrefix + "storage": "v0.7.1",
	})
	var specs []string
	applyAndurelPackageUpdatesFunc = func(_ context.Context, _ string, got []string) error {
		specs = append([]string(nil), got...)
		return nil
	}

	var stdout bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"packages", "update", "email", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("packages update email: %v", err)
	}
	if len(specs) != 1 || specs[0] != versions.PkgPrefix+"email@v0.3.2" {
		t.Fatalf("specs = %#v", specs)
	}

	report := decodePackageReport(t, stdout.String())
	if len(report.Packages) != 1 || report.Packages[0].Name != "email" {
		t.Fatalf("report = %#v", report)
	}
}

func TestPackagesListMissingPackages(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeGoModule(t, root)
	findGoModRoot = func() (string, error) { return root, nil }

	cmd := NewRootCommand("test", "test-date")
	cmd.SetArgs([]string{"packages"})
	err := cmd.Execute()
	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != output.CodeConfigError {
		t.Fatalf("error = %v", err)
	}
}

func TestPackagesHumanList(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", testAndurelPackagesGoMod)
	findGoModRoot = func() (string, error) { return root, nil }
	stubLatestModuleVersions(t, map[string]string{
		versions.PkgPrefix + "storage": "v0.7.1",
	})

	var stdout bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"packages"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("packages: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "1 Andurel package has an update available") ||
		!strings.Contains(out, "storage") ||
		!strings.Contains(out, "v0.7.0 -> v0.7.1") ||
		!strings.Contains(out, "replaced") {
		t.Fatalf("human output:\n%s", out)
	}
}

func stubLatestModuleVersions(t *testing.T, versions map[string]string) {
	t.Helper()
	original := lookupLatestModuleVersionFunc
	lookupLatestModuleVersionFunc = func(_ context.Context, modulePath string) (string, error) {
		version, ok := versions[modulePath]
		if !ok {
			return "", errors.New("unexpected module " + modulePath)
		}
		return version, nil
	}
	t.Cleanup(func() {
		lookupLatestModuleVersionFunc = original
	})
}

func decodePackageReport(t *testing.T, stdout string) packageUpdateReport {
	t.Helper()
	var envelope struct {
		OK   bool                `json:"ok"`
		Data packageUpdateReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout)
	}
	if !envelope.OK {
		t.Fatalf("expected ok envelope: %s", stdout)
	}
	return envelope.Data
}
