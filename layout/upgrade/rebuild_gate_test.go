package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout"
)

func TestExecuteRejectsPreRebuildUpgradeWithoutMutations(t *testing.T) {
	t.Parallel()

	root := newLegacyUpgradeProject(t, "v2.0.0-alpha.3")
	mustWriteTestFile(t, root, "go.mod", []byte(
		"module example.com/legacy\n\ngo 1.24.0\n\nrequire go.uber.org/fx v1.23.0\n",
	))
	for _, directory := range []string{"controllers", "router", "models"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshotUpgradeTree(t, root)

	upgrader, err := NewUpgrader(root, UpgradeOptions{TargetVersion: "v2.0.0-alpha.5"})
	if err != nil {
		t.Fatal(err)
	}
	report, err := upgrader.Execute()
	if err == nil {
		t.Fatal("expected a pre-rebuild upgrade to be refused")
	}

	var unsupported *UnsupportedUpgradeError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error = %T %v, want *UnsupportedUpgradeError", err, err)
	}
	if got := unsupported.Code(); got != CodeUnsupportedUpgrade {
		t.Fatalf("error code = %q, want %q", got, CodeUnsupportedUpgrade)
	}
	if !strings.Contains(unsupported.Hint(), "patterns show composition") {
		t.Fatalf("hint does not point at the patterns: %q", unsupported.Hint())
	}

	wantMarkers := []string{"controllers/", "router/", "models/", "go.uber.org/fx"}
	data := unsupported.Data()
	if data.From != "v2.0.0-alpha.3" || data.To != "v2.0.0-alpha.5" {
		t.Fatalf("error data versions = %#v", data)
	}
	if data.RebuildVersion != "v2.0.0-alpha.4" {
		t.Fatalf("rebuild version = %q", data.RebuildVersion)
	}
	if !slices.Equal(data.LegacyMarkers, wantMarkers) {
		t.Fatalf("legacy markers = %v, want %v", data.LegacyMarkers, wantMarkers)
	}

	// No notes, file changes, tool changes or git access happened: the gate
	// runs before all of them.
	if report == nil {
		t.Fatal("expected the failed report to be returned")
	}
	if len(report.ManualActions) != 0 {
		t.Fatalf("refused upgrade produced notes: %#v", report.ManualActions)
	}
	if report.FilesReplaced != 0 || report.FilesRemoved != 0 ||
		len(report.ReplacedFiles) != 0 || len(report.RemovedFiles) != 0 {
		t.Fatalf("refused upgrade planned file changes: %#v", report)
	}
	if report.ToolsAdded != 0 || report.ToolsRemoved != 0 || report.ToolsUpdated != 0 {
		t.Fatalf("refused upgrade planned tool changes: %#v", report)
	}
	assertSnapshotEqual(t, before, snapshotUpgradeTree(t, root))
}

func TestExecuteRejectsUpgradeToRebuildBoundary(t *testing.T) {
	t.Parallel()

	root := newLegacyUpgradeProject(t, "v2.0.0-alpha")
	before := snapshotUpgradeTree(t, root)

	upgrader, err := NewUpgrader(root, UpgradeOptions{TargetVersion: "v2.0.0-alpha.4"})
	if err != nil {
		t.Fatal(err)
	}
	report, err := upgrader.Execute()
	if err == nil {
		t.Fatal("expected an upgrade to the rebuild boundary to be refused")
	}

	var unsupported *UnsupportedUpgradeError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error = %T %v, want *UnsupportedUpgradeError", err, err)
	}
	if unsupported.FromVersion != "v2.0.0-alpha" ||
		unsupported.ToVersion != "v2.0.0-alpha.4" {
		t.Fatalf("unsupported error = %#v", unsupported)
	}
	if len(unsupported.Data().LegacyMarkers) != 0 {
		t.Fatalf(
			"absent legacy shape should still be gated by version: %#v",
			unsupported.Data(),
		)
	}
	if report == nil || len(report.ManualActions) != 0 {
		t.Fatalf("refused upgrade report = %#v", report)
	}
	assertSnapshotEqual(t, before, snapshotUpgradeTree(t, root))
}

func TestExecuteAllowsSameMajorUpgradeAtRebuildVersion(t *testing.T) {
	t.Parallel()

	root := newUpgradeFixtureProject(t)
	lock, err := layout.ReadLockFile(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Version = goNativeRebuildVersion
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatal(err)
	}
	commitUpgradeTree(t, root, "move fixture to the rebuild version")

	upgrader, err := NewUpgrader(
		root,
		UpgradeOptions{DryRun: true, TargetVersion: "v2.0.0-alpha.5"},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := upgrader.Execute()
	if err != nil {
		t.Fatalf("alpha.4 to alpha.5 upgrade = %v", err)
	}
	if !report.Success {
		t.Fatalf("alpha.4 to alpha.5 report = %#v", report)
	}
	if report.FromVersion != goNativeRebuildVersion ||
		report.ToVersion != "v2.0.0-alpha.5" {
		t.Fatalf("alpha.4 to alpha.5 report versions = %#v", report)
	}
}

func TestValidateGoNativeRebuildVersionOrdering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{name: "alpha to rebuild boundary", from: "v2.0.0-alpha", to: "v2.0.0-alpha.4", want: true},
		{name: "alpha.3 to boundary", from: "v2.0.0-alpha.3", to: "v2.0.0-alpha.4", want: true},
		{name: "alpha.3 to alpha.5", from: "v2.0.0-alpha.3", to: "v2.0.0-alpha.5", want: true},
		{name: "alpha.10 is past alpha.4", from: "v2.0.0-alpha.10", to: "v2.0.0-alpha.11"},
		{name: "alpha.4 to alpha.5", from: "v2.0.0-alpha.4", to: "v2.0.0-alpha.5"},
		{name: "same rebuild version", from: "v2.0.0-alpha.4", to: "v2.0.0-alpha.4"},
		{name: "alpha.3 to alpha.3", from: "v2.0.0-alpha.3", to: "v2.0.0-alpha.3"},
		{name: "post rebuild to stable", from: "v2.0.0-alpha.5", to: "v2.0.0"},
		{name: "development build", from: "dev", to: "v2.0.0-alpha.5"},
		{name: "non-semver target", from: "v2.0.0-alpha.3", to: "dev"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateGoNativeRebuild(t.TempDir(), test.from, test.to)
			if got := err != nil; got != test.want {
				t.Fatalf("validateGoNativeRebuild(%q, %q) = %v, want refused %t",
					test.from, test.to, err, test.want)
			}
		})
	}
}

func TestDetectLegacyMarkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		goMod   string
		dirs    []string
		markers []string
	}{
		{
			name:    "no legacy shape",
			goMod:   "module example.com/app\n\ngo 1.24.0\n",
			markers: []string{},
		},
		{
			name:    "directories only",
			goMod:   "module example.com/app\n\ngo 1.24.0\n",
			dirs:    []string{"controllers", "router", "models"},
			markers: []string{"controllers/", "router/", "models/"},
		},
		{
			name:    "fx requirement only",
			goMod:   "module example.com/app\n\ngo 1.24.0\n\nrequire go.uber.org/fx v1.23.0\n",
			markers: []string{"go.uber.org/fx"},
		},
		{
			name: "all markers",
			goMod: "module example.com/app\n\ngo 1.24.0\n\nrequire (\n" +
				"\tgo.uber.org/fx v1.23.0\n)\n",
			dirs:    []string{"controllers", "router", "models"},
			markers: []string{"controllers/", "router/", "models/", "go.uber.org/fx"},
		},
		{
			name:    "unparseable go.mod is informational only",
			goMod:   "module\n",
			dirs:    []string{"controllers"},
			markers: []string{"controllers/"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			mustWriteTestFile(t, root, "go.mod", []byte(test.goMod))
			for _, directory := range test.dirs {
				if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			if got := detectLegacyMarkers(root); !slices.Equal(got, test.markers) {
				t.Fatalf("markers = %v, want %v", got, test.markers)
			}
		})
	}
}

func newLegacyUpgradeProject(t *testing.T, version string) string {
	t.Helper()

	root := t.TempDir()
	mustWriteTestFile(t, root, "go.mod", []byte("module example.com/legacy\n\ngo 1.24.0\n"))
	lock := layout.NewAndurelLock(version)
	lock.ScaffoldConfig = &layout.ScaffoldConfig{ProjectName: "legacy"}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatal(err)
	}
	return root
}
