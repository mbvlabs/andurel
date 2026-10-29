package cli

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout"
)

func TestBuildOutputDirIsolatesSourceAndEmbedsAssets(t *testing.T) {
	requireLinuxBuildTest(t)
	root := filepath.Join(t.TempDir(), "project with space")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeIsolatedBuildProject(t, root, []string{"app"}, true)

	logPath := filepath.Join(t.TempDir(), "commands.log")
	installFakeBuildToolchain(t, logPath)

	outputDir := filepath.Join(root, "out dir")
	report, err := buildProject(root, buildOptions{
		Only:      []string{"app"},
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("buildProject: %v", err)
	}

	if !report.Isolated || !report.AssetsBuilt {
		t.Fatalf("report = %#v, want isolated with assets", report)
	}
	absoluteOutput, err := filepath.Abs(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if report.OutputDir != filepath.ToSlash(absoluteOutput) {
		t.Fatalf("output_dir = %q, want %q", report.OutputDir, absoluteOutput)
	}
	if len(report.Artifacts) != 1 {
		t.Fatalf("artifacts = %#v", report.Artifacts)
	}
	wantBinary := filepath.ToSlash(filepath.Join(absoluteOutput, "app"))
	if report.Artifacts[0].Path != wantBinary {
		t.Fatalf("artifact path = %q, want %q", report.Artifacts[0].Path, wantBinary)
	}

	assertFileMissing(t, filepath.Join(root, "assets", "dist", "marker.txt"))
	assertFileMissing(t, filepath.Join(root, "dist"))
	assertFileMissing(t, filepath.Join(root, "cmd", "app", "app_templ.go"))
	assertFileContains(t, filepath.Join(root, "assets", "dist", "seed.txt"), "seed")

	marker := runBuiltBinary(t, filepath.Join(absoluteOutput, "app"), t.TempDir())
	if !strings.Contains(marker, "andurel-build-") {
		t.Fatalf("built binary did not embed the staged Vite marker: %q", marker)
	}
}

func TestBuildOutputDirConcurrentBuildsUseDistinctRoots(t *testing.T) {
	requireLinuxBuildTest(t)
	root := t.TempDir()
	writeIsolatedBuildProject(t, root, []string{"app", "ssr"}, true)

	logPath := filepath.Join(t.TempDir(), "commands.log")
	installFakeBuildToolchain(t, logPath)

	outputDirs := []string{
		filepath.Join(root, "concurrent one"),
		filepath.Join(root, "concurrent two"),
	}
	reports := make([]buildReport, len(outputDirs))
	errs := make([]error, len(outputDirs))
	var wait sync.WaitGroup
	for index := range outputDirs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			reports[index], errs[index] = buildProject(root, buildOptions{
				Only:      []string{"app", "ssr"},
				OutputDir: outputDirs[index],
			})
		}(index)
	}
	wait.Wait()

	for index, err := range errs {
		if err != nil {
			t.Fatalf("concurrent build %d: %v", index, err)
		}
	}

	buildMarkers := make([][]string, len(outputDirs))
	for index, outputDir := range outputDirs {
		absolute, err := filepath.Abs(outputDir)
		if err != nil {
			t.Fatal(err)
		}
		for _, process := range []string{"app", "ssr"} {
			marker := runBuiltBinary(t, filepath.Join(absolute, process), t.TempDir())
			if !strings.Contains(marker, "andurel-build-") {
				t.Fatalf("build %d %s did not embed its staged assets: %q", index, process, marker)
			}
			buildMarkers[index] = append(buildMarkers[index], marker)
		}
	}
	for index := range buildMarkers {
		if buildMarkers[index][0] != buildMarkers[index][1] {
			t.Fatalf("processes in build %d did not share their stage: %q", index, buildMarkers[index])
		}
	}
	if buildMarkers[0][0] == buildMarkers[1][0] {
		t.Fatalf("concurrent builds shared a staging assets tree: %q", buildMarkers)
	}

	assertFileMissing(t, filepath.Join(root, "assets", "dist", "marker.txt"))
	assertFileMissing(t, filepath.Join(root, "dist"))
}

func TestBuildOutputDirPreservesCommittedEmbedInputsWithoutFrontend(t *testing.T) {
	requireLinuxBuildTest(t)
	root := t.TempDir()
	writeIsolatedBuildProject(t, root, []string{"app"}, false)

	logPath := filepath.Join(t.TempDir(), "commands.log")
	installFakeBuildToolchain(t, logPath)

	outputDir := filepath.Join(t.TempDir(), "committed-assets")
	report, err := buildProject(root, buildOptions{
		Only:      []string{"app"},
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("buildProject: %v", err)
	}
	if report.AssetsBuilt {
		t.Fatal("non-Inertia build reported Vite assets")
	}
	output := runBuiltBinary(t, filepath.Join(outputDir, "app"), t.TempDir())
	if output != "seed" {
		t.Fatalf("binary did not embed the committed assets/dist input: %q", output)
	}
}

func TestBuildOutputDirFailurePreservesPreviousArtifact(t *testing.T) {
	requireLinuxBuildTest(t)
	root := t.TempDir()
	writeIsolatedBuildProject(t, root, []string{"app", "queue"}, false)

	logPath := filepath.Join(t.TempDir(), "commands.log")
	installFakeBuildToolchain(t, logPath)
	outputDir := filepath.Join(t.TempDir(), "published")

	if _, err := buildProject(root, buildOptions{
		Only:      []string{"app"},
		OutputDir: outputDir,
	}); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	appPath := filepath.Join(outputDir, "app")
	before := hashFileForBuildTest(t, appPath)

	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	fakePath := t.TempDir()
	writeExecutable(t, fakePath, "go", fmt.Sprintf(`#!/bin/sh
case "$*" in
  *"./cmd/queue"*) echo "forced queue failure" >&2; exit 1 ;;
esac
exec %q "$@"
`, realGo))
	t.Setenv("PATH", fakePath+string(os.PathListSeparator)+os.Getenv("PATH"))

	if _, err := buildProject(root, buildOptions{
		Only:      []string{"app", "queue"},
		OutputDir: outputDir,
	}); err == nil {
		t.Fatal("expected the queue build to fail")
	}

	after := hashFileForBuildTest(t, appPath)
	if before != after {
		t.Fatalf("failed build replaced the previous app artifact: %x != %x", before, after)
	}
	assertFileMissing(t, filepath.Join(outputDir, "queue"))
}

func TestBuildOutputDirRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.1\n")

	for _, raw := range []string{".", "assets/out", "bin/out", "cmd/out", filepath.Dir(root)} {
		if _, err := resolveBuildOutputDir(root, raw); err == nil {
			t.Fatalf("expected --output-dir %q to be rejected", raw)
		}
	}

	for _, raw := range []string{"dist", "out dir"} {
		if _, err := resolveBuildOutputDir(root, raw); err != nil {
			t.Fatalf("expected --output-dir %q to be accepted: %v", raw, err)
		}
	}

	writeTestFile(t, root, "not-a-directory", "file")
	if _, err := resolveBuildOutputDir(root, "not-a-directory"); err == nil {
		t.Fatal("expected an existing file output path to be rejected")
	}
}

func TestBuildOnlyMigrateSkipsFrontendAssets(t *testing.T) {
	requireLinuxBuildTest(t)
	root := t.TempDir()
	writeIsolatedBuildProject(t, root, []string{"migrate"}, true)

	logPath := filepath.Join(t.TempDir(), "commands.log")
	installFakeBuildToolchain(t, logPath)

	report, err := buildProject(root, buildOptions{
		Only:      []string{"migrate"},
		OutputDir: filepath.Join(t.TempDir(), "migrate-only"),
	})
	if err != nil {
		t.Fatalf("buildProject: %v", err)
	}
	if report.AssetsBuilt {
		t.Fatal("migrate-only build reported frontend assets")
	}
	if data, err := os.ReadFile(logPath); err == nil && strings.Contains(string(data), "pnpm") {
		t.Fatalf("migrate-only build ran the frontend package manager:\n%s", data)
	}
	assertFileMissing(t, filepath.Join(root, "assets", "dist", "marker.txt"))
}

func TestBuildOutputDirRewritesLocalModuleReplacements(t *testing.T) {
	requireLinuxBuildTest(t)
	base := filepath.Join(t.TempDir(), "base with space")
	root := filepath.Join(base, "app")
	dep := filepath.Join(base, "dep")
	writeTestFile(t, dep, "go.mod", "module example.com/dep\n\ngo 1.27.3\n")
	writeTestFile(t, dep, "dep.go", "package dep\n\nfunc Value() string { return \"from-local-replace\" }\n")
	writeIsolatedBuildProject(t, root, []string{"app"}, false)
	writeTestFile(t, root, "go.mod", `module example.com/app

go 1.27.3

require example.com/dep v0.0.0

replace example.com/dep => ../dep
`)
	writeTestFile(t, root, "cmd/app/main.go", `package main

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
	outputDir := filepath.Join(t.TempDir(), "replace-build")
	if _, err := buildProject(root, buildOptions{
		Only:      []string{"app"},
		OutputDir: outputDir,
	}); err != nil {
		t.Fatalf("buildProject: %v", err)
	}
	output := runBuiltBinary(t, filepath.Join(outputDir, "app"), t.TempDir())
	if output != "from-local-replace" {
		t.Fatalf("built binary output = %q", output)
	}
}

func TestBuildOutputDirRewritesGoWorkUses(t *testing.T) {
	requireLinuxBuildTest(t)
	base := t.TempDir()
	root := filepath.Join(base, "app")
	dep := filepath.Join(base, "dep")
	writeTestFile(t, dep, "go.mod", "module example.com/dep\n\ngo 1.27.3\n")
	writeTestFile(t, dep, "dep.go", "package dep\n\nfunc Value() string { return \"from-go-work\" }\n")
	writeIsolatedBuildProject(t, root, []string{"app"}, false)
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.3\n\nrequire example.com/dep v0.0.0\n")
	writeTestFile(t, root, "go.work", "go 1.27.3\n\nuse (\n\t.\n\t../dep\n)\n")
	writeTestFile(t, root, "cmd/app/main.go", `package main

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
	outputDir := filepath.Join(t.TempDir(), "gowork-build")
	if _, err := buildProject(root, buildOptions{
		Only:      []string{"app"},
		OutputDir: outputDir,
	}); err != nil {
		t.Fatalf("buildProject: %v", err)
	}
	output := runBuiltBinary(t, filepath.Join(outputDir, "app"), t.TempDir())
	if output != "from-go-work" {
		t.Fatalf("built binary output = %q", output)
	}
}

func writeIsolatedBuildProject(t *testing.T, root string, processes []string, inertia bool) {
	t.Helper()
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.3\n")
	writeTestFile(t, root, "assets/assets.go", `package assets

import "embed"

//go:embed *
var Files embed.FS
`)
	writeTestFile(t, root, "assets/dist/seed.txt", "seed\n")

	for _, process := range processes {
		mainBody := "package main\n\nfunc main() {}\n"
		if process == "app" || process == "ssr" {
			mainBody = `package main

import (
	"fmt"

	"example.com/app/assets"
)

func main() {
	data, err := assets.Files.ReadFile("dist/marker.txt")
	if err != nil {
		fmt.Print("seed")
		return
	}
	fmt.Print(string(data))
}
`
		}
		writeTestFile(t, root, filepath.Join("cmd", process, "main.go"), mainBody)
	}

	lock := layout.NewAndurelLock("test")
	lock.Tools["templ"] = validTestTool("templ", "v0.3.0")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{ProjectName: "app"}
	if inertia {
		lock.ScaffoldConfig.Inertia = "react"
		lock.ScaffoldConfig.JavaScriptPackageManager = "pnpm"
	}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
}

func installFakeBuildToolchain(t *testing.T, logPath string) {
	t.Helper()
	resetCLITestSeams(t)
	syncSingleToolFunc = func(projectRoot, name string, tool *layout.Tool, goos, goarch string) error {
		writeExecutable(t, projectRoot, filepath.Join("bin", name), fakeCommandScript(logPath, name))
		return nil
	}

	fakePath := t.TempDir()
	writeExecutable(t, fakePath, "pnpm", fmt.Sprintf(`#!/bin/sh
printf 'pnpm %%s\n' "$*" >> %q
mkdir -p assets/dist
printf '%%s' "$(pwd)" > assets/dist/marker.txt
`, logPath))
	t.Setenv("PATH", fakePath+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func runBuiltBinary(t *testing.T, path, dir string) string {
	t.Helper()
	cmd := exec.Command(path)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run built binary %s: %v\n%s", path, err, output)
	}
	return string(output)
}

func hashFileForBuildTest(t *testing.T, path string) [32]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return sha256.Sum256(data)
}

func assertFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s not to exist", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s does not contain %q:\n%s", path, want, data)
	}
}

func requireLinuxBuildTest(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("andurel build cross-compiles GOOS=linux; this test runs the artifact")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not installed")
	}
}

func TestRecoverInterruptedBuildPublicationRestoresPreviousBinary(t *testing.T) {
	outputDir := t.TempDir()
	writeBuildTestFile(t, filepath.Join(outputDir, ".andurel-backup-app-123456"), "previous app")
	writeBuildTestFile(t, filepath.Join(outputDir, ".andurel-app-987654"), "partial app")

	recovered, err := recoverInterruptedBuildPublication(outputDir)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if len(recovered) != 2 {
		t.Fatalf("recovered = %#v, want a restored publication and a removed temp", recovered)
	}
	assertFileContains(t, filepath.Join(outputDir, "app"), "previous app")
	assertFileMissing(t, filepath.Join(outputDir, ".andurel-backup-app-123456"))
	assertFileMissing(t, filepath.Join(outputDir, ".andurel-app-987654"))
}

func TestRecoverInterruptedBuildPublicationKeepsPublishedBinary(t *testing.T) {
	outputDir := t.TempDir()
	writeBuildTestFile(t, filepath.Join(outputDir, "app"), "new app")
	writeBuildTestFile(t, filepath.Join(outputDir, ".andurel-backup-app-123456"), "previous app")

	recovered, err := recoverInterruptedBuildPublication(outputDir)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if len(recovered) != 1 {
		t.Fatalf("recovered = %#v, want one stale backup removed", recovered)
	}
	assertFileContains(t, filepath.Join(outputDir, "app"), "new app")
	assertFileMissing(t, filepath.Join(outputDir, ".andurel-backup-app-123456"))
}

func TestRecoverInterruptedBuildPublicationIgnoresForeignNames(t *testing.T) {
	outputDir := t.TempDir()
	unsafeBackup := filepath.Join(outputDir, ".andurel-backup-..-123456")
	writeBuildTestFile(t, unsafeBackup, "not ours")
	writeBuildTestFile(t, filepath.Join(outputDir, ".andurel-backup-app-notdigits"), "not ours")

	recovered, err := recoverInterruptedBuildPublication(outputDir)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if len(recovered) != 0 {
		t.Fatalf("recovered = %#v, want foreign names untouched", recovered)
	}
	assertFileContains(t, unsafeBackup, "not ours")
}

func TestPublishBuildArtifactsRepairsInterruptedPublication(t *testing.T) {
	workRoot := t.TempDir()
	outputDir := t.TempDir()
	staged := filepath.Join(workRoot, "dist", "app")
	writeBuildTestFile(t, staged, "new app")
	writeBuildTestFile(t, filepath.Join(outputDir, ".andurel-backup-app-123456"), "previous app")

	report := &buildReport{}
	if err := publishBuildArtifacts(workRoot, outputDir, []buildProcess{{Name: "app"}}, report); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(report.Recovered) != 1 {
		t.Fatalf("report.Recovered = %#v, want the interrupted publication", report.Recovered)
	}
	assertFileContains(t, filepath.Join(outputDir, "app"), "new app")
	assertFileMissing(t, filepath.Join(outputDir, ".andurel-backup-app-123456"))
}

func writeBuildTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
