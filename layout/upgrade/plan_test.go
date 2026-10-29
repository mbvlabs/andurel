package upgrade

import (
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/mbvlabs/andurel/v2/layout/versions"
)

func TestCloneReportPlansManifestFilesWithoutCountingThem(t *testing.T) {
	t.Parallel()

	plan := &upgradePlan{files: []plannedFile{
		{path: "go.mod", after: []byte("module x\n")},
		{path: "internal/old.go", remove: true},
		{path: layout.ProjectTomlName, after: []byte("version = \"v2\"\n"), isLock: true},
	}}
	report := plan.cloneReport()

	if report.FilesReplaced != 1 || report.FilesRemoved != 1 {
		t.Fatalf("framework counts = %d replaced, %d removed", report.FilesReplaced, report.FilesRemoved)
	}
	if len(report.PlannedChanges) != 3 {
		t.Fatalf("planned changes = %#v, want all three files", report.PlannedChanges)
	}
	manifest := report.PlannedChanges[2]
	if manifest.Path != layout.ProjectTomlName || string(manifest.Content) != "version = \"v2\"\n" {
		t.Fatalf("manifest change = %#v", manifest)
	}
	if !report.PlannedChanges[1].Remove {
		t.Fatalf("removal not planned: %#v", report.PlannedChanges[1])
	}
}

func TestShouldUpgradePackage(t *testing.T) {
	t.Parallel()

	if shouldUpgradePackage(versions.Storage, versions.Storage) {
		t.Fatal("equal versions must not upgrade")
	}
	if !shouldUpgradePackage("v0.6.0", versions.Storage) {
		t.Fatal("older versions must upgrade")
	}
	if shouldUpgradePackage("v9.0.0", versions.Storage) {
		t.Fatal("newer versions must not downgrade")
	}
}

func TestAddVerifiedPackageDependenciesPinsRequiredModules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteTestFile(t, root, "go.mod", []byte(`module testapp

go 1.27.1

require (
	github.com/mbvlabs/andurel/pkg/email v0.9.0
	github.com/mbvlabs/andurel/pkg/storage v0.6.0
)
`))

	plan := &upgradePlan{}
	upgrader := &Upgrader{projectRoot: root}
	if err := upgrader.addVerifiedPackageDependencies(plan, &layout.ScaffoldConfig{
		Inertia: "vue",
	}); err != nil {
		t.Fatal(err)
	}
	if len(plan.files) != 1 || plan.files[0].path != "go.mod" {
		t.Fatalf("planned files = %#v", plan.files)
	}

	goMod := string(plan.files[0].after)
	if !strings.Contains(goMod, "github.com/mbvlabs/andurel/pkg/storage "+versions.Storage) {
		t.Fatalf("storage was not pinned:\n%s", goMod)
	}
	if !strings.Contains(goMod, "github.com/mbvlabs/andurel/pkg/email v0.9.0") {
		t.Fatalf("newer email pin was downgraded:\n%s", goMod)
	}
	if !strings.Contains(goMod, "github.com/mbvlabs/andurel/pkg/inertia "+versions.Inertia) {
		t.Fatalf("missing Inertia dependency was not added:\n%s", goMod)
	}
	if !strings.Contains(goMod, "github.com/mbvlabs/andurel/pkg/telemetry "+versions.Telemetry) {
		t.Fatalf("missing telemetry dependency was not added:\n%s", goMod)
	}
	if strings.Contains(goMod, "github.com/mbvlabs/andurel/pkg/routing") {
		t.Fatalf("unrequired package was added:\n%s", goMod)
	}
}

func TestAddVerifiedPackageDependenciesSkipsNonInertiaProjects(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteTestFile(t, root, "go.mod", []byte(`module testapp

go 1.27.1

require github.com/mbvlabs/andurel/pkg/storage v0.6.0
`))

	plan := &upgradePlan{}
	upgrader := &Upgrader{projectRoot: root}
	if err := upgrader.addVerifiedPackageDependencies(plan, &layout.ScaffoldConfig{}); err != nil {
		t.Fatal(err)
	}
	if len(plan.files) != 1 {
		t.Fatalf("planned files = %#v", plan.files)
	}
	goMod := string(plan.files[0].after)
	if strings.Contains(goMod, "github.com/mbvlabs/andurel/pkg/inertia") {
		t.Fatalf("Inertia was added to a non-Inertia project:\n%s", goMod)
	}
	if !strings.Contains(goMod, "github.com/mbvlabs/andurel/pkg/storage "+versions.Storage) {
		t.Fatalf("storage was not pinned:\n%s", goMod)
	}
	if !strings.Contains(goMod, "github.com/mbvlabs/andurel/pkg/telemetry "+versions.Telemetry) {
		t.Fatalf("missing telemetry dependency was not added:\n%s", goMod)
	}
}

func TestAddVerifiedPackageDependenciesNoopsWhenCurrent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteTestFile(t, root, "go.mod", []byte(
		"module testapp\n\ngo 1.27.1\n\nrequire (\n\tgithub.com/mbvlabs/andurel/pkg/storage "+
			versions.Storage+"\n\tgithub.com/mbvlabs/andurel/pkg/telemetry "+
			versions.Telemetry+"\n)\n",
	))

	plan := &upgradePlan{}
	upgrader := &Upgrader{projectRoot: root}
	if err := upgrader.addVerifiedPackageDependencies(plan, &layout.ScaffoldConfig{}); err != nil {
		t.Fatal(err)
	}
	if len(plan.files) != 0 {
		t.Fatalf("expected no go.mod rewrite, planned %#v", plan.files)
	}
}

func TestAddVerifiedPackageDependenciesPinsNarsilc(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteTestFile(t, root, "go.mod", []byte(`module testapp

go 1.27.1

require github.com/mbvlabs/narsilc v0.4.5
`))

	plan := &upgradePlan{}
	upgrader := &Upgrader{projectRoot: root}
	if err := upgrader.addVerifiedPackageDependencies(plan, &layout.ScaffoldConfig{}); err != nil {
		t.Fatal(err)
	}
	if len(plan.files) != 1 || plan.files[0].path != "go.mod" {
		t.Fatalf("planned files = %#v", plan.files)
	}

	goMod := string(plan.files[0].after)
	if !strings.Contains(goMod, "github.com/mbvlabs/narsilc "+versions.NarsilcModule) {
		t.Fatalf("narsilc was not pinned:\n%s", goMod)
	}
}

func TestAddVerifiedPackageDependenciesDoesNotDowngradeNarsilc(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteTestFile(t, root, "go.mod", []byte(
		"module testapp\n\ngo 1.27.1\n\nrequire (\n\tgithub.com/mbvlabs/andurel/pkg/telemetry "+
			versions.Telemetry+"\n\tgithub.com/mbvlabs/narsilc v0.99.0\n)\n",
	))

	plan := &upgradePlan{}
	upgrader := &Upgrader{projectRoot: root}
	if err := upgrader.addVerifiedPackageDependencies(plan, &layout.ScaffoldConfig{}); err != nil {
		t.Fatal(err)
	}
	if len(plan.files) != 0 {
		t.Fatalf("expected no go.mod rewrite, planned %#v", plan.files)
	}
}
