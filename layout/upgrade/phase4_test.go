package upgrade

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/layout"
)

func TestMigrationRegistrySelectsFrameworkAndSchemaIndependently(t *testing.T) {
	t.Parallel()

	frameworkOnly := selectMigrations(MigrationSelector{
		SourceFrameworkVersion: "v1.0.0-rc.2",
		TargetFrameworkVersion: "v1.0.0",
		SourceLockSchema:       1,
		TargetLockSchema:       1,
	})
	if names := migrationNames(frameworkOnly); !reflect.DeepEqual(names, []string{"rc2-project-corrections"}) {
		t.Fatalf("framework migrations = %v", names)
	}

	schemaOnly := selectMigrations(MigrationSelector{
		SourceFrameworkVersion: "v9.9.9",
		TargetFrameworkVersion: "v9.9.9",
		SourceLockSchema:       0,
		TargetLockSchema:       1,
	})
	if names := migrationNames(schemaOnly); !reflect.DeepEqual(names, []string{"lock-schema-legacy-to-1"}) {
		t.Fatalf("schema migrations = %v", names)
	}

	both := selectMigrations(MigrationSelector{
		SourceFrameworkVersion: "v1.0.0-rc.3",
		TargetFrameworkVersion: "v1.0.0",
		SourceLockSchema:       0,
		TargetLockSchema:       1,
	})
	if names := migrationNames(both); !reflect.DeepEqual(names, []string{"lock-schema-legacy-to-1", "rc3-project-corrections"}) {
		t.Fatalf("ordered migrations = %v", names)
	}
}

func TestUpgraderRemainsComparable(t *testing.T) {
	t.Parallel()

	requireComparable[Upgrader]()
}

func requireComparable[T comparable]() {}

func TestRCTagFixturesAreExactAndCoverVariants(t *testing.T) {
	t.Parallel()

	wantHashes := map[string]string{
		"cmd/app/main.go":  "cea4a830336e8fb924d50e31113c925c9da78d8a6f6ab01ec42b014a3b7e9061",
		"models/user.go":   "6f13b4a9e63e65d1449f6c8446b6796195647ec15a017e50fcac91aee334b18f",
		"router/router.go": "045f06ac6ac06a16b85ad9ecdc794da581ae5980dd4cf20bad82df50cfae7a14",
	}
	for _, release := range []string{"rc1", "rc2", "rc3"} {
		for path, want := range wantHashes {
			content := mustReadFixture(t, release, "pristine", path)
			if got := fmt.Sprintf("%x", sha256.Sum256(content)); got != want {
				t.Fatalf("%s %s hash = %s, want %s", release, path, got, want)
			}
		}
		lock := mustReadFixture(t, release, "pristine", "andurel.lock")
		if !bytes.Contains(lock, []byte(`"version": "v1.0.0-`+strings.ReplaceAll(release, "rc", "rc.")+`"`)) {
			t.Fatalf("%s lock does not identify its exact release", release)
		}
		variants := mustReadFixture(t, release, "variants.json")
		for _, required := range []string{"edited-lock", "edited-router", "edited-user", "edited-application", "unknown-target", "missing-target", "duplicate-target", "ambiguous-target"} {
			if !bytes.Contains(variants, []byte(required)) {
				t.Fatalf("%s variants missing %s", release, required)
			}
		}
	}
}

func TestUpgradeCleanAndDirtyWorktreeBehavior(t *testing.T) {
	cleanRoot := newRCFixtureProject(t, "rc3")
	clean, err := NewUpgrader(cleanRoot, UpgradeOptions{DryRun: true, TargetVersion: "v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	report, err := clean.Execute()
	if err != nil || report.DirtyWorktree {
		t.Fatalf("clean dry run: report=%#v err=%v", report, err)
	}

	dirtyRoot := newRCFixtureProject(t, "rc3")
	mustWriteTestFile(t, dirtyRoot, "user-edit.txt", []byte("dirty\n"))
	dry, err := NewUpgrader(dirtyRoot, UpgradeOptions{DryRun: true, TargetVersion: "v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	report, err = dry.Execute()
	if err != nil || !report.DirtyWorktree {
		t.Fatalf("dirty dry run: report=%#v err=%v", report, err)
	}

	real, err := NewUpgrader(dirtyRoot, UpgradeOptions{TargetVersion: "v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	before := snapshotUpgradeTree(t, dirtyRoot)
	if _, err := real.Execute(); err == nil || !strings.Contains(err.Error(), "worktree is dirty") {
		t.Fatalf("dirty real upgrade error = %v", err)
	}
	assertSnapshotEqual(t, before, snapshotUpgradeTree(t, dirtyRoot))
}

func TestDryRunIsDeterministicAndByteReadOnlyOnSameInstance(t *testing.T) {
	root := newRCFixtureProject(t, "rc1")
	mustWriteTestFile(t, root, "dirty.txt", []byte("preserve me\n"))
	before := snapshotUpgradeTree(t, root)
	statusBefore := gitOutput(t, root, "status", "--porcelain=v1")
	upgrader, err := NewUpgrader(root, UpgradeOptions{DryRun: true, TargetVersion: "v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}

	first, err := upgrader.Execute()
	if err != nil {
		t.Fatal(err)
	}
	second, err := upgrader.Execute()
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("repeated dry runs differ\nfirst: %s\nsecond: %s", firstJSON, secondJSON)
	}
	if !sort.StringsAreSorted(first.ReplacedFiles) || !sort.StringsAreSorted(first.RemovedFiles) {
		t.Fatalf("report paths are not deterministic: %#v", first)
	}
	if len(first.Diffs) == 0 || len(first.LockMigrations) == 0 || len(first.FrameworkMigrations) == 0 {
		t.Fatalf("dry-run report is incomplete: %#v", first)
	}
	assertSnapshotEqual(t, before, snapshotUpgradeTree(t, root))
	if statusAfter := gitOutput(t, root, "status", "--porcelain=v1"); statusAfter != statusBefore {
		t.Fatalf("dry run changed Git state\nbefore: %q\nafter: %q", statusBefore, statusAfter)
	}
}

func TestPristineRCFixturesUpgradeAtomicallyAndIdempotently(t *testing.T) {
	for _, release := range []string{"rc1", "rc2", "rc3"} {
		t.Run(release, func(t *testing.T) {
			root := newRCFixtureProject(t, release)
			upgrader, err := NewUpgrader(root, UpgradeOptions{TargetVersion: "v1.0.0"})
			if err != nil {
				t.Fatal(err)
			}
			report, err := upgrader.Execute()
			if err != nil || !report.Success {
				t.Fatalf("upgrade: report=%#v err=%v", report, err)
			}
			assertRCOutcome(t, root)
			commitUpgradeTree(t, root, "upgraded")
			before := snapshotUpgradeTree(t, root)
			second, err := upgrader.Execute()
			if err != nil || !second.Success {
				t.Fatalf("idempotent upgrade: report=%#v err=%v", second, err)
			}
			if second.FilesReplaced != 0 || second.FilesRemoved != 0 || len(second.Diffs) != 0 {
				t.Fatalf("second upgrade was not a no-op: %#v", second)
			}
			assertSnapshotEqual(t, before, snapshotUpgradeTree(t, root))
		})
	}
}

func TestRecognizedIndependentEditsArePreserved(t *testing.T) {
	root := newRCFixtureProject(t, "rc2")
	appendTestFile(t, root, "router/router.go", "\nconst UserRouterCustomization = true\n")
	appendTestFile(t, root, "models/user.go", "\nconst UserModelCustomization = true\n")
	appendTestFile(t, root, "cmd/app/main.go", "\nconst UserApplicationCustomization = true\n")
	addCustomToolToLegacyLock(t, root)
	commitUpgradeTree(t, root, "intentional edits")

	upgrader, err := NewUpgrader(root, UpgradeOptions{TargetVersion: "v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := upgrader.Execute(); err != nil {
		t.Fatal(err)
	}
	for path, marker := range map[string]string{
		"router/router.go": "UserRouterCustomization",
		"models/user.go":   "UserModelCustomization",
		"cmd/app/main.go":  "UserApplicationCustomization",
	} {
		if !bytes.Contains(mustReadProjectFile(t, root, path), []byte(marker)) {
			t.Fatalf("%s lost independent edit %s", path, marker)
		}
	}
	lock, err := layoutReadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lock.Tools["user-tool"]; !ok {
		t.Fatal("custom lock tool was not preserved")
	}
}

func TestUnknownMissingDuplicatedAndAmbiguousStructuresConflictWithoutWrites(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{name: "unknown", mutate: func(t *testing.T, root string) {
			replaceTestFile(t, root, "models/user.go", "db.NewUpdate().", "db.NewUpdate().\n\t\tComment(\"user edit in targeted node\").", 1)
		}},
		{name: "missing", mutate: func(t *testing.T, root string) {
			replaceTestFile(t, root, "models/user.go", "func (u user) Destroy(", "func (u user) DestroyMissing(", 1)
		}},
		{name: "duplicated", mutate: func(t *testing.T, root string) {
			duplicateFunction(t, root, "models/user.go", "Destroy")
		}},
		{name: "ambiguous", mutate: func(t *testing.T, root string) {
			replaceTestFile(t, root, "router/router.go", "func SetupGlobalMiddleware(", "func SetupGlobalMiddlewareUnknown(", 1)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newRCFixtureProject(t, "rc3")
			test.mutate(t, root)
			commitUpgradeTree(t, root, test.name)
			before := snapshotUpgradeTree(t, root)
			upgrader, err := NewUpgrader(root, UpgradeOptions{TargetVersion: "v1.0.0"})
			if err != nil {
				t.Fatal(err)
			}
			report, err := upgrader.Execute()
			if err == nil || report == nil || len(report.Conflicts) == 0 {
				t.Fatalf("expected conflict, report=%#v err=%v", report, err)
			}
			assertSnapshotEqual(t, before, snapshotUpgradeTree(t, root))
		})
	}
}

func TestInjectedFailuresRollBackByteIdentically(t *testing.T) {
	operations := []string{
		"stage", "validation", "backup", "write", "sync", "close", "rename",
		"directory-sync", "post-write-validation", "cleanup",
	}
	for _, operation := range operations {
		t.Run(operation, func(t *testing.T) {
			root := newRCFixtureProject(t, "rc3")
			before := snapshotUpgradeTree(t, root)
			upgrader, err := NewUpgrader(root, UpgradeOptions{TargetVersion: "v1.0.0"})
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			upgrader.transaction = &transactionRuntime{failureInjector: func(got, _ string) error {
				if got != operation {
					return nil
				}
				seen++
				if (operation == "write" || operation == "rename" || operation == "directory-sync") && seen == 1 {
					return nil
				}
				return errors.New("injected " + operation)
			}}
			if _, err := upgrader.Execute(); err == nil {
				t.Fatalf("expected injected %s failure", operation)
			}
			assertSnapshotEqual(t, before, snapshotUpgradeTree(t, root))
		})
	}
}

func TestLockIsReplacedLast(t *testing.T) {
	root := newRCFixtureProject(t, "rc3")
	upgrader, err := NewUpgrader(root, UpgradeOptions{TargetVersion: "v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	var writes []string
	upgrader.transaction = &transactionRuntime{failureInjector: func(operation, path string) error {
		if operation == "write" {
			writes = append(writes, path)
		}
		return nil
	}}
	if _, err := upgrader.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(writes) == 0 || writes[len(writes)-1] != "andurel.lock" {
		t.Fatalf("write order = %v, lock must be last", writes)
	}
}

func migrationNames(migrations []Migration) []string {
	names := make([]string, len(migrations))
	for index, migration := range migrations {
		names[index] = migration.Name
	}
	return names
}

func newRCFixtureProject(t *testing.T, release string) string {
	t.Helper()
	root := t.TempDir()
	fixtureRoot := filepath.Join("testdata", release, "pristine")
	err := filepath.WalkDir(fixtureRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(fixtureRoot, path)
		if err != nil {
			return err
		}
		relative = strings.TrimSuffix(relative, ".golden")
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		mustWriteTestFile(t, root, relative, content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	mustWriteTestFile(t, root, "go.mod", []byte("module testapp\n\ngo 1.24.0\n"))
	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.email", "upgrade-test@example.com")
	gitRun(t, root, "config", "user.name", "Upgrade Test")
	commitUpgradeTree(t, root, "fixture")
	return root
}

func commitUpgradeTree(t *testing.T, root, message string) {
	t.Helper()
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-m", message)
}

func gitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func mustReadFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	path := filepath.Join(append([]string{"testdata"}, parts...)...)
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		content, err = os.ReadFile(path + ".golden")
	}
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func mustWriteTestFile(t *testing.T, root, path string, content []byte) {
	t.Helper()
	fullPath := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustReadProjectFile(t *testing.T, root, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func appendTestFile(t *testing.T, root, path, suffix string) {
	t.Helper()
	content := mustReadProjectFile(t, root, path)
	mustWriteTestFile(t, root, path, append(content, suffix...))
}

func replaceTestFile(t *testing.T, root, path, old, replacement string, count int) {
	t.Helper()
	content := mustReadProjectFile(t, root, path)
	updated := bytes.Replace(content, []byte(old), []byte(replacement), count)
	if bytes.Equal(content, updated) {
		t.Fatalf("%s does not contain %q", path, old)
	}
	mustWriteTestFile(t, root, path, updated)
}

func duplicateFunction(t *testing.T, root, path, name string) {
	t.Helper()
	content := mustReadProjectFile(t, root, path)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var function *ast.FuncDecl
	for _, declaration := range file.Decls {
		candidate, ok := declaration.(*ast.FuncDecl)
		if ok && candidate.Name.Name == name {
			function = candidate
			break
		}
	}
	if function == nil {
		t.Fatalf("missing function %s", name)
	}
	start := fset.Position(function.Pos()).Offset
	end := fset.Position(function.End()).Offset
	appendTestFile(t, root, path, "\n"+string(content[start:end])+"\n")
}

func addCustomToolToLegacyLock(t *testing.T, root string) {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(mustReadProjectFile(t, root, "andurel.lock"), &object); err != nil {
		t.Fatal(err)
	}
	tools := object["tools"].(map[string]any)
	tools["user-tool"] = map[string]any{
		"version":      "v9.9.9",
		"path":         "cmd/user-tool/main.go",
		"versionCheck": map[string]any{"args": []any{"--version"}},
	}
	content, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mustWriteTestFile(t, root, "andurel.lock", append(content, '\n'))
}

func snapshotUpgradeTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if relative == ".git" || strings.HasPrefix(entry.Name(), ".andurel-upgrade-") {
				return filepath.SkipDir
			}
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = content
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertSnapshotEqual(t *testing.T, want, got map[string][]byte) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		wantPaths := make([]string, 0, len(want))
		gotPaths := make([]string, 0, len(got))
		for path := range want {
			wantPaths = append(wantPaths, path)
		}
		for path := range got {
			gotPaths = append(gotPaths, path)
		}
		sort.Strings(wantPaths)
		sort.Strings(gotPaths)
		t.Fatalf("tree changed\nwant paths: %v\ngot paths: %v", wantPaths, gotPaths)
	}
}

func assertRCOutcome(t *testing.T, root string) {
	t.Helper()
	for path, marker := range map[string]string{
		"models/user.go":   `.NewDelete()`,
		"router/router.go": `func newCORSConfig(`,
		"cmd/app/main.go":  `func stopAndWait(`,
	} {
		if !bytes.Contains(mustReadProjectFile(t, root, path), []byte(marker)) {
			t.Fatalf("%s missing migrated marker %q", path, marker)
		}
	}
	lock, err := layoutReadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if lock.SchemaVersion != 1 || lock.Version != "v1.0.0" {
		t.Fatalf("lock versions = schema %d framework %s", lock.SchemaVersion, lock.Version)
	}
	if lock.Tools["shadowfax"].Version != "v0.8.4" {
		t.Fatalf("shadowfax version = %s", lock.Tools["shadowfax"].Version)
	}
}

func layoutReadLock(root string) (*layout.AndurelLock, error) {
	return layout.ReadLockFile(root)
}

func TestMigrationRegistryOrderIsStable(t *testing.T) {
	orders := make([]int, len(migrationRegistry))
	for index, migration := range migrationRegistry {
		orders[index] = migration.Order
	}
	if !slices.IsSorted(orders) {
		t.Fatalf("migration registry order = %v", orders)
	}
}
