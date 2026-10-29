package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These regressions cover build findings from the beta.3 review: interrupted
// multi-process publication, output containment through a symlinked parent,
// writable staged links/inodes, and quoted module/workspace paths with spaces.

func TestBuildPublicationRecoveryRollsBackInterruptedMultiProcessSet(t *testing.T) {
	outputDir := t.TempDir()
	writeBuildTestFile(t, filepath.Join(outputDir, "app"), "new app")
	writeBuildTestFile(t, filepath.Join(outputDir, buildBackupPrefix+"app-111111"), "previous app")
	writeBuildTestFile(t, filepath.Join(outputDir, buildBackupPrefix+"ssr-222222"), "previous ssr")
	writeBuildTestFile(t, filepath.Join(outputDir, buildTempPrefix+"ssr-333333"), "new ssr")
	writePublicationJournalForTest(t, outputDir, "publishing", []map[string]any{
		journalEntryForTest("app", buildTempPrefix+"app-444444", buildBackupPrefix+"app-111111", true),
		journalEntryForTest("ssr", buildTempPrefix+"ssr-333333", buildBackupPrefix+"ssr-222222", true),
	})

	if _, err := recoverInterruptedBuildPublication(outputDir); err != nil {
		t.Fatalf("recover: %v", err)
	}

	assertFileContains(t, filepath.Join(outputDir, "app"), "previous app")
	assertFileContains(t, filepath.Join(outputDir, "ssr"), "previous ssr")
	assertFileMissing(t, filepath.Join(outputDir, ".andurel-publish-journal.json"))
	assertFileMissing(t, filepath.Join(outputDir, buildTempPrefix+"ssr-333333"))
	assertFileMissing(t, filepath.Join(outputDir, buildBackupPrefix+"app-111111"))
	assertFileMissing(t, filepath.Join(outputDir, buildBackupPrefix+"ssr-222222"))
}

func TestBuildPublicationRecoveryHandlesEveryInterruptionBoundary(t *testing.T) {
	t.Run("journal written before backups", func(t *testing.T) {
		outputDir := t.TempDir()
		writeBuildTestFile(t, filepath.Join(outputDir, "app"), "previous app")
		writeBuildTestFile(t, filepath.Join(outputDir, "ssr"), "previous ssr")
		writeBuildTestFile(t, filepath.Join(outputDir, buildTempPrefix+"app-111111"), "new app")
		writeBuildTestFile(t, filepath.Join(outputDir, buildTempPrefix+"ssr-222222"), "new ssr")
		writePublicationJournalForTest(t, outputDir, "publishing", []map[string]any{
			journalEntryForTest("app", buildTempPrefix+"app-111111", buildBackupPrefix+"app-333333", true),
			journalEntryForTest("ssr", buildTempPrefix+"ssr-222222", buildBackupPrefix+"ssr-444444", true),
		})

		if _, err := recoverInterruptedBuildPublication(outputDir); err != nil {
			t.Fatalf("recover: %v", err)
		}
		assertFileContains(t, filepath.Join(outputDir, "app"), "previous app")
		assertFileContains(t, filepath.Join(outputDir, "ssr"), "previous ssr")
		assertFileMissing(t, filepath.Join(outputDir, buildTempPrefix+"app-111111"))
		assertFileMissing(t, filepath.Join(outputDir, buildTempPrefix+"ssr-222222"))
	})

	t.Run("first final moved to backup", func(t *testing.T) {
		outputDir := t.TempDir()
		writeBuildTestFile(t, filepath.Join(outputDir, "ssr"), "previous ssr")
		writeBuildTestFile(t, filepath.Join(outputDir, buildBackupPrefix+"app-111111"), "previous app")
		writeBuildTestFile(t, filepath.Join(outputDir, buildTempPrefix+"app-222222"), "new app")
		writeBuildTestFile(t, filepath.Join(outputDir, buildTempPrefix+"ssr-333333"), "new ssr")
		writePublicationJournalForTest(t, outputDir, "publishing", []map[string]any{
			journalEntryForTest("app", buildTempPrefix+"app-222222", buildBackupPrefix+"app-111111", true),
			journalEntryForTest("ssr", buildTempPrefix+"ssr-333333", buildBackupPrefix+"ssr-444444", true),
		})

		if _, err := recoverInterruptedBuildPublication(outputDir); err != nil {
			t.Fatalf("recover: %v", err)
		}
		assertFileContains(t, filepath.Join(outputDir, "app"), "previous app")
		assertFileContains(t, filepath.Join(outputDir, "ssr"), "previous ssr")
	})

	t.Run("all finals moved to backups", func(t *testing.T) {
		outputDir := t.TempDir()
		writeBuildTestFile(t, filepath.Join(outputDir, buildBackupPrefix+"app-111111"), "previous app")
		writeBuildTestFile(t, filepath.Join(outputDir, buildBackupPrefix+"ssr-222222"), "previous ssr")
		writeBuildTestFile(t, filepath.Join(outputDir, buildTempPrefix+"app-333333"), "new app")
		writeBuildTestFile(t, filepath.Join(outputDir, buildTempPrefix+"ssr-444444"), "new ssr")
		writePublicationJournalForTest(t, outputDir, "publishing", []map[string]any{
			journalEntryForTest("app", buildTempPrefix+"app-333333", buildBackupPrefix+"app-111111", true),
			journalEntryForTest("ssr", buildTempPrefix+"ssr-444444", buildBackupPrefix+"ssr-222222", true),
		})

		if _, err := recoverInterruptedBuildPublication(outputDir); err != nil {
			t.Fatalf("recover: %v", err)
		}
		assertFileContains(t, filepath.Join(outputDir, "app"), "previous app")
		assertFileContains(t, filepath.Join(outputDir, "ssr"), "previous ssr")
	})

	t.Run("first replacement published", func(t *testing.T) {
		outputDir := t.TempDir()
		writeBuildTestFile(t, filepath.Join(outputDir, "app"), "new app")
		writeBuildTestFile(t, filepath.Join(outputDir, buildBackupPrefix+"app-111111"), "previous app")
		writeBuildTestFile(t, filepath.Join(outputDir, buildBackupPrefix+"ssr-222222"), "previous ssr")
		writeBuildTestFile(t, filepath.Join(outputDir, buildTempPrefix+"ssr-333333"), "new ssr")
		writePublicationJournalForTest(t, outputDir, "publishing", []map[string]any{
			journalEntryForTest("app", buildTempPrefix+"app-444444", buildBackupPrefix+"app-111111", true),
			journalEntryForTest("ssr", buildTempPrefix+"ssr-333333", buildBackupPrefix+"ssr-222222", true),
		})

		if _, err := recoverInterruptedBuildPublication(outputDir); err != nil {
			t.Fatalf("recover: %v", err)
		}
		assertFileContains(t, filepath.Join(outputDir, "app"), "previous app")
		assertFileContains(t, filepath.Join(outputDir, "ssr"), "previous ssr")
	})

	t.Run("all replacements published before commit marker", func(t *testing.T) {
		outputDir := t.TempDir()
		writeBuildTestFile(t, filepath.Join(outputDir, "app"), "new app")
		writeBuildTestFile(t, filepath.Join(outputDir, "ssr"), "new ssr")
		writeBuildTestFile(t, filepath.Join(outputDir, buildBackupPrefix+"app-111111"), "previous app")
		writeBuildTestFile(t, filepath.Join(outputDir, buildBackupPrefix+"ssr-222222"), "previous ssr")
		writePublicationJournalForTest(t, outputDir, "publishing", []map[string]any{
			journalEntryForTest("app", buildTempPrefix+"app-333333", buildBackupPrefix+"app-111111", true),
			journalEntryForTest("ssr", buildTempPrefix+"ssr-444444", buildBackupPrefix+"ssr-222222", true),
		})

		if _, err := recoverInterruptedBuildPublication(outputDir); err != nil {
			t.Fatalf("recover: %v", err)
		}
		assertFileContains(t, filepath.Join(outputDir, "app"), "previous app")
		assertFileContains(t, filepath.Join(outputDir, "ssr"), "previous ssr")
	})

	t.Run("committed journal keeps new set", func(t *testing.T) {
		outputDir := t.TempDir()
		writeBuildTestFile(t, filepath.Join(outputDir, "app"), "new app")
		writeBuildTestFile(t, filepath.Join(outputDir, "ssr"), "new ssr")
		writeBuildTestFile(t, filepath.Join(outputDir, buildBackupPrefix+"app-111111"), "previous app")
		writeBuildTestFile(t, filepath.Join(outputDir, buildBackupPrefix+"ssr-222222"), "previous ssr")
		writePublicationJournalForTest(t, outputDir, "committed", []map[string]any{
			journalEntryForTest("app", buildTempPrefix+"app-333333", buildBackupPrefix+"app-111111", true),
			journalEntryForTest("ssr", buildTempPrefix+"ssr-444444", buildBackupPrefix+"ssr-222222", true),
		})

		if _, err := recoverInterruptedBuildPublication(outputDir); err != nil {
			t.Fatalf("recover: %v", err)
		}
		assertFileContains(t, filepath.Join(outputDir, "app"), "new app")
		assertFileContains(t, filepath.Join(outputDir, "ssr"), "new ssr")
		assertFileMissing(t, filepath.Join(outputDir, buildBackupPrefix+"app-111111"))
		assertFileMissing(t, filepath.Join(outputDir, buildBackupPrefix+"ssr-222222"))
	})

	t.Run("first build interrupted after one rename", func(t *testing.T) {
		outputDir := t.TempDir()
		writeBuildTestFile(t, filepath.Join(outputDir, "app"), "new app")
		writeBuildTestFile(t, filepath.Join(outputDir, buildTempPrefix+"ssr-111111"), "new ssr")
		writePublicationJournalForTest(t, outputDir, "publishing", []map[string]any{
			journalEntryForTest("app", buildTempPrefix+"app-222222", "", false),
			journalEntryForTest("ssr", buildTempPrefix+"ssr-111111", "", false),
		})

		if _, err := recoverInterruptedBuildPublication(outputDir); err != nil {
			t.Fatalf("recover: %v", err)
		}
		assertFileMissing(t, filepath.Join(outputDir, "app"))
		assertFileMissing(t, filepath.Join(outputDir, "ssr"))
		assertFileMissing(t, filepath.Join(outputDir, buildTempPrefix+"ssr-111111"))
	})
}

func TestBuildOutputDirRejectsNonexistentChildBelowSymlinkedSourceDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("assets", filepath.Join(root, "out-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("out-link", filepath.Join(root, "chained-link")); err != nil {
		t.Fatal(err)
	}

	for _, raw := range []string{
		"out-link/new-output",
		"out-link/new-output/nested",
		"chained-link/new-output",
	} {
		if _, err := resolveBuildOutputDir(root, raw); err == nil {
			t.Fatalf("expected --output-dir %q to be rejected through the symlinked source parent", raw)
		}
	}

	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(root, "external-link")); err != nil {
		t.Fatal(err)
	}
	output, err := resolveBuildOutputDir(root, "external-link/new-output")
	if err != nil {
		t.Fatalf("external symlink output should be accepted: %v", err)
	}
	if output.AbsPath != filepath.Join(external, "new-output") {
		t.Fatalf("resolved output = %q, want %q", output.AbsPath, filepath.Join(external, "new-output"))
	}
}

func TestBuildStageDoesNotShareDependencyInodesOrSourceSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixtures require a Unix host")
	}
	root := t.TempDir()
	writeBuildTestFile(t, filepath.Join(root, "node_modules", ".pnpm", "pkg@1", "node_modules", "pkg", "index.js"), "dependency source\n")
	if err := os.Symlink(
		filepath.Join(".pnpm", "pkg@1", "node_modules", "pkg"),
		filepath.Join(root, "node_modules", "pkg"),
	); err != nil {
		t.Fatal(err)
	}
	writeBuildTestFile(t, filepath.Join(root, "assets", "seed.txt"), "asset source\n")
	if err := os.MkdirAll(filepath.Join(root, "resources"), 0o755); err != nil {
		t.Fatal(err)
	}
	sourceTarget := filepath.Join(root, "assets", "seed.txt")
	if err := os.Symlink(sourceTarget, filepath.Join(root, "resources", "linked.txt")); err != nil {
		t.Fatal(err)
	}
	dependencyHash := hashFileForBuildTest(t, filepath.Join(root, "node_modules", "pkg", "index.js"))
	assetHash := hashFileForBuildTest(t, sourceTarget)

	stageRoot, err := stageBuildSource(root, filepath.Join(t.TempDir(), "output"))
	if err != nil {
		t.Fatalf("stage source: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(stageRoot) })

	if data, err := os.ReadFile(filepath.Join(stageRoot, "node_modules", "pkg", "index.js")); err != nil {
		t.Fatalf("read staged dependency through pnpm symlink: %v", err)
	} else if string(data) != "dependency source\n" {
		t.Fatalf("staged dependency = %q", data)
	}

	writeBuildTestFile(t, filepath.Join(stageRoot, "node_modules", "pkg", "index.js"), "plugin mutation\n")
	writeBuildTestFile(t, filepath.Join(stageRoot, "resources", "linked.txt"), "plugin mutation\n")

	if data, err := os.ReadFile(filepath.Join(root, "node_modules", "pkg", "index.js")); err != nil {
		t.Fatalf("read source dependency: %v", err)
	} else if string(data) != "dependency source\n" {
		t.Fatalf("staged plugin write mutated the source dependency: %q", data)
	}
	if after := hashFileForBuildTest(t, filepath.Join(root, "node_modules", "pkg", "index.js")); after != dependencyHash {
		t.Fatalf("staged write changed the source dependency hash: %x != %x", after, dependencyHash)
	}
	if data, err := os.ReadFile(sourceTarget); err != nil {
		t.Fatalf("read source asset: %v", err)
	} else if string(data) != "asset source\n" {
		t.Fatalf("staged plugin write mutated the source asset: %q", data)
	}
	if after := hashFileForBuildTest(t, sourceTarget); after != assetHash {
		t.Fatalf("staged write changed the source asset hash: %x != %x", after, assetHash)
	}

	stagedLinkTarget, err := filepath.EvalSymlinks(filepath.Join(stageRoot, "resources", "linked.txt"))
	if err != nil {
		t.Fatalf("resolve staged source symlink: %v", err)
	}
	if rel, ok := relativeWithin(stageRoot, stagedLinkTarget); !ok || rel == "" {
		t.Fatalf("staged source symlink %q resolves outside the stage (%q)", stagedLinkTarget, rel)
	}
}

func TestBuildStageRejectsSymlinksEscapingTheProject(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixtures require a Unix host")
	}

	t.Run("source tree", func(t *testing.T) {
		root := t.TempDir()
		external := t.TempDir()
		writeBuildTestFile(t, filepath.Join(external, "shared.txt"), "external\n")
		if err := os.MkdirAll(filepath.Join(root, "resources"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(external, filepath.Join(root, "resources", "external")); err != nil {
			t.Fatal(err)
		}

		if _, err := stageBuildSource(root, filepath.Join(t.TempDir(), "output")); err == nil {
			t.Fatal("expected an external source symlink to be rejected")
		} else if !strings.Contains(err.Error(), "outside the project") {
			t.Fatalf("error = %v, want an actionable outside-the-project message", err)
		}
	})

	t.Run("dependency tree", func(t *testing.T) {
		root := t.TempDir()
		external := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(external, filepath.Join(root, "node_modules", "external-package")); err != nil {
			t.Fatal(err)
		}

		if _, err := stageBuildSource(root, filepath.Join(t.TempDir(), "output")); err == nil {
			t.Fatal("expected an external dependency symlink to be rejected")
		} else if !strings.Contains(err.Error(), "outside the project") {
			t.Fatalf("error = %v, want an actionable outside-the-project message", err)
		}
	})
}

func TestBuildOutputDirPluginWritesStayInStage(t *testing.T) {
	requireLinuxBuildTest(t)
	root := t.TempDir()
	writeIsolatedBuildProject(t, root, []string{"app"}, true)
	writeBuildTestFile(t, filepath.Join(root, "node_modules", "dep", "index.js"), "dependency source\n")
	writeBuildTestFile(t, filepath.Join(root, "assets", "source.txt"), "asset source\n")
	dependencyPath := filepath.Join(root, "node_modules", "dep", "index.js")
	assetPath := filepath.Join(root, "assets", "source.txt")
	dependencyHash := hashFileForBuildTest(t, dependencyPath)
	assetHash := hashFileForBuildTest(t, assetPath)
	if err := os.MkdirAll(filepath.Join(root, "resources"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "assets", "source.txt"), filepath.Join(root, "resources", "linked.txt")); err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(t.TempDir(), "commands.log")
	installFakeBuildToolchain(t, logPath)
	fakePath := t.TempDir()
	writeExecutable(t, fakePath, "pnpm", `#!/bin/sh
mkdir -p assets/dist
printf '%s' "$(pwd)" > assets/dist/marker.txt
printf 'plugin mutation\n' > node_modules/dep/index.js
printf 'plugin mutation\n' > resources/linked.txt
`)
	t.Setenv("PATH", fakePath+string(os.PathListSeparator)+os.Getenv("PATH"))

	outputDir := filepath.Join(t.TempDir(), "plugin-build")
	if _, err := buildProject(root, buildOptions{
		Only:      []string{"app"},
		OutputDir: outputDir,
	}); err != nil {
		t.Fatalf("buildProject: %v", err)
	}

	if data, err := os.ReadFile(dependencyPath); err != nil {
		t.Fatalf("read source dependency: %v", err)
	} else if string(data) != "dependency source\n" {
		t.Fatalf("plugin mutated the source dependency through a shared inode: %q", data)
	}
	if after := hashFileForBuildTest(t, dependencyPath); after != dependencyHash {
		t.Fatalf("plugin changed the source dependency hash: %x != %x", after, dependencyHash)
	}
	if data, err := os.ReadFile(assetPath); err != nil {
		t.Fatalf("read source asset: %v", err)
	} else if string(data) != "asset source\n" {
		t.Fatalf("plugin mutated the source asset through a symlink: %q", data)
	}
	if after := hashFileForBuildTest(t, assetPath); after != assetHash {
		t.Fatalf("plugin changed the source asset hash: %x != %x", after, assetHash)
	}
	runBuiltBinary(t, filepath.Join(outputDir, "app"), t.TempDir())
}

func TestBuildOutputDirRewritesQuotedLocalReplacementWithSpaces(t *testing.T) {
	requireLinuxBuildTest(t)
	base := filepath.Join(t.TempDir(), "base with space")
	root := filepath.Join(base, "app")
	dep := filepath.Join(base, "local module")
	writeBuildTestFile(t, filepath.Join(dep, "go.mod"), "module example.com/dep\n\ngo 1.27.3\n")
	writeBuildTestFile(t, filepath.Join(dep, "dep.go"), "package dep\n\nfunc Value() string { return \"from-quoted-replace\" }\n")
	writeIsolatedBuildProject(t, root, []string{"app"}, false)
	writeBuildTestFile(t, filepath.Join(root, "go.mod"), `module example.com/app

go 1.27.3

require example.com/dep v0.0.0

replace example.com/dep => "../local module"
`)
	writeBuildTestFile(t, filepath.Join(root, "cmd", "app", "main.go"), `package main

import (
	"fmt"

	"example.com/app/assets"
	"example.com/dep"
)

func main() {
	_, _ = assets.Files.ReadFile("dist/seed.txt")
	fmt.Print(dep.Value())
}
`)

	logPath := filepath.Join(t.TempDir(), "commands.log")
	installFakeBuildToolchain(t, logPath)
	outputDir := filepath.Join(t.TempDir(), "quoted-replace-build")
	if _, err := buildProject(root, buildOptions{
		Only:      []string{"app"},
		OutputDir: outputDir,
	}); err != nil {
		t.Fatalf("buildProject: %v", err)
	}
	if output := runBuiltBinary(t, filepath.Join(outputDir, "app"), t.TempDir()); output != "from-quoted-replace" {
		t.Fatalf("built binary output = %q", output)
	}
}

func TestBuildOutputDirRewritesQuotedGoWorkUseWithSpaces(t *testing.T) {
	requireLinuxBuildTest(t)
	base := filepath.Join(t.TempDir(), "workspace base")
	root := filepath.Join(base, "app")
	dep := filepath.Join(base, "local module")
	writeBuildTestFile(t, filepath.Join(dep, "go.mod"), "module example.com/dep\n\ngo 1.27.3\n")
	writeBuildTestFile(t, filepath.Join(dep, "dep.go"), "package dep\n\nfunc Value() string { return \"from-quoted-go-work\" }\n")
	writeIsolatedBuildProject(t, root, []string{"app"}, false)
	writeBuildTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.27.3\n\nrequire example.com/dep v0.0.0\n")
	writeBuildTestFile(t, filepath.Join(root, "go.work"), "go 1.27.3\n\nuse (\n\t.\n\t\"../local module\"\n)\n")
	writeBuildTestFile(t, filepath.Join(root, "cmd", "app", "main.go"), `package main

import (
	"fmt"

	"example.com/app/assets"
	"example.com/dep"
)

func main() {
	_, _ = assets.Files.ReadFile("dist/seed.txt")
	fmt.Print(dep.Value())
}
`)

	logPath := filepath.Join(t.TempDir(), "commands.log")
	installFakeBuildToolchain(t, logPath)
	outputDir := filepath.Join(t.TempDir(), "quoted-gowork-build")
	if _, err := buildProject(root, buildOptions{
		Only:      []string{"app"},
		OutputDir: outputDir,
	}); err != nil {
		t.Fatalf("buildProject: %v", err)
	}
	if output := runBuiltBinary(t, filepath.Join(outputDir, "app"), t.TempDir()); output != "from-quoted-go-work" {
		t.Fatalf("built binary output = %q", output)
	}
}

func TestPublishBuildArtifactsPublishesCompleteSetAndRemovesJournal(t *testing.T) {
	workRoot := t.TempDir()
	outputDir := t.TempDir()
	for _, process := range []string{"app", "ssr"} {
		writeBuildTestFile(t, filepath.Join(workRoot, "dist", process), "new "+process)
		writeBuildTestFile(t, filepath.Join(outputDir, process), "previous "+process)
	}

	report := &buildReport{Artifacts: []buildArtifact{
		{Process: "app"},
		{Process: "ssr"},
	}}
	if err := publishBuildArtifacts(workRoot, outputDir, []buildProcess{{Name: "app"}, {Name: "ssr"}}, report); err != nil {
		t.Fatalf("publish: %v", err)
	}

	assertFileContains(t, filepath.Join(outputDir, "app"), "new app")
	assertFileContains(t, filepath.Join(outputDir, "ssr"), "new ssr")
	assertFileMissing(t, filepath.Join(outputDir, ".andurel-publish-journal.json"))
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".andurel") {
			t.Fatalf("publication left temporary metadata behind: %s", entry.Name())
		}
	}
	if len(report.Artifacts) != 2 || report.Artifacts[0].Size == 0 || report.Artifacts[1].Size == 0 {
		t.Fatalf("manifest did not report both artifacts: %#v", report.Artifacts)
	}
}

func writePublicationJournalForTest(t *testing.T, outputDir, phase string, entries []map[string]any) {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"phase":   phase,
		"entries": entries,
	})
	if err != nil {
		t.Fatalf("encode journal: %v", err)
	}
	writeBuildTestFile(t, filepath.Join(outputDir, ".andurel-publish-journal.json"), string(data))
}

func journalEntryForTest(process, temp, backup string, hadPrevious bool) map[string]any {
	return map[string]any{
		"process":      process,
		"final":        process,
		"temp":         temp,
		"backup":       backup,
		"had_previous": hadPrevious,
	}
}
