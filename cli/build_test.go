package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout"
)

func TestInertiaPackageManagerCommands(t *testing.T) {
	tests := []struct {
		name        string
		runtime     string
		wantInstall string
		wantBuild   string
	}{
		{
			name:        "default runtime uses pnpm",
			runtime:     "",
			wantInstall: "pnpm install --frozen-lockfile",
			wantBuild:   "pnpm run build",
		},
		{
			name:        "npm",
			runtime:     "npm",
			wantInstall: "npm ci",
			wantBuild:   "npm run build",
		},
		{
			name:        "pnpm",
			runtime:     "pnpm",
			wantInstall: "pnpm install --frozen-lockfile",
			wantBuild:   "pnpm run build",
		},
		{
			name:        "bun",
			runtime:     "bun",
			wantInstall: "bun install --frozen-lockfile",
			wantBuild:   "bun run build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			install, build, err := inertiaPackageManagerCommands(tt.runtime)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if install.String() != tt.wantInstall {
				t.Fatalf("install command = %q, want %q", install.String(), tt.wantInstall)
			}
			if build.String() != tt.wantBuild {
				t.Fatalf("build command = %q, want %q", build.String(), tt.wantBuild)
			}
		})
	}
}

func TestInertiaPackageManagerCommandsRejectsUnsupportedRuntime(t *testing.T) {
	for _, runtime := range []string{"deno", "yarn"} {
		if _, _, err := inertiaPackageManagerCommands(runtime); err == nil {
			t.Fatalf("expected unsupported runtime error for %q", runtime)
		}
	}
}

func TestExtractModuleName(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module github.com/acme/orders\n\ngo 1.27.1\n")

	name, err := extractModuleName(root)
	if err != nil {
		t.Fatalf("extractModuleName: %v", err)
	}
	if name != "orders" {
		t.Fatalf("module name = %q, want orders", name)
	}
}

func TestExtractModuleNameErrors(t *testing.T) {
	root := t.TempDir()
	if _, err := extractModuleName(root); err == nil {
		t.Fatalf("expected missing go.mod error")
	}

	writeTestFile(t, root, "go.mod", "go 1.27.1\n")
	if _, err := extractModuleName(
		root,
	); err == nil ||
		!strings.Contains(err.Error(), "module directive") {
		t.Fatalf("expected missing module directive error, got %v", err)
	}
}

func TestDetectGitVersion(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test User")
	writeTestFile(t, root, "go.mod", "module example.com/app\n")
	runGit(t, root, "add", "go.mod")
	runGit(t, root, "commit", "-m", "initial")
	runGit(t, root, "tag", "v1.2.3")

	version, err := detectGitVersion(root)
	if err != nil {
		t.Fatalf("detectGitVersion: %v", err)
	}
	if version != "v1.2.3" {
		t.Fatalf("git version = %q, want v1.2.3", version)
	}

	writeTestFile(t, root, "go.mod", "module example.com/app\n\n// dirty\n")
	version, err = detectGitVersion(root)
	if err != nil {
		t.Fatalf("detectGitVersion dirty: %v", err)
	}
	if !strings.Contains(version, "v1.2.3") || !strings.HasSuffix(version, "-dirty") {
		t.Fatalf("dirty git version = %q", version)
	}
}

func TestBuildAppRunsExpectedToolchain(t *testing.T) {
	resetCLITestSeams(t)

	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	writeTestFile(t, root, "cmd/app/main.go", "package main\n\nfunc main() {}\n")
	lock := layout.NewAndurelLock("test")
	lock.Tools["templ"] = validTestTool("templ", "v0.3.0")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName:       "app",
		Inertia:           "react",
		JavaScriptRuntime: "pnpm",
	}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	logPath := filepath.Join(root, "commands.log")
	fakePath := t.TempDir()
	writeExecutable(t, fakePath, "go", fakeGoCommandScript(logPath))
	writeExecutable(t, fakePath, "pnpm", fakeCommandScript(logPath, "pnpm"))
	t.Setenv("PATH", fakePath)

	var synced []string
	syncSingleToolFunc = func(projectRoot, name string, tool *layout.Tool, goos, goarch string) error {
		synced = append(synced, name+":"+tool.Version)
		writeExecutable(
			t,
			projectRoot,
			filepath.Join("bin", name),
			fakeCommandScript(logPath, name),
		)
		return nil
	}

	report, err := buildApp(root, "1.2.3", nil)
	if err != nil {
		t.Fatalf("buildApp: %v", err)
	}
	if len(report.Artifacts) != 1 || report.Artifacts[0].Process != "app" ||
		report.Artifacts[0].Path != "dist/app" || report.Artifacts[0].Version != "1.2.3" {
		t.Fatalf("build report artifacts = %#v", report.Artifacts)
	}

	if len(synced) != 2 || synced[0] != "templ:v0.3.0" ||
		!strings.HasPrefix(synced[1], "tailwindcli:v") {
		t.Fatalf("synced tools = %#v", synced)
	}
	log := readBuildTestFile(t, logPath)
	for _, want := range []string{
		"templ generate",
		"pnpm install --frozen-lockfile",
		"pnpm run build",
		"go mod download",
		"go build -v -ldflags -X main.appVersion=1.2.3 -X example.com/app/config.BuildVersion=1.2.3 -o dist/app ./cmd/app",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("command log missing %q:\n%s", want, log)
		}
	}
}

func TestBuildAppRunsNarsilcWhenQueriesExist(t *testing.T) {
	resetCLITestSeams(t)

	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	writeTestFile(t, root, "cmd/app/main.go", "package main\n\nfunc main() {}\n")
	writeTestFile(
		t,
		root,
		"domains/user/user.sql",
		"-- name: GetUser :one\nSELECT 1;\n",
	)
	lock := layout.NewAndurelLock("test")
	lock.Tools["templ"] = validTestTool("templ", "v0.3.0")
	lock.Tools["narsilc"] = validTestTool("narsilc", "v0.1.0")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName: "app",
	}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	logPath := filepath.Join(root, "commands.log")
	fakePath := t.TempDir()
	writeExecutable(t, fakePath, "go", fakeGoCommandScript(logPath))
	t.Setenv("PATH", fakePath)

	syncSingleToolFunc = func(projectRoot, name string, tool *layout.Tool, goos, goarch string) error {
		writeExecutable(
			t,
			projectRoot,
			filepath.Join("bin", name),
			fakeCommandScript(logPath, name),
		)
		return nil
	}

	if _, err := buildApp(root, "", nil); err != nil {
		t.Fatalf("buildApp: %v", err)
	}

	log := readBuildTestFile(t, logPath)
	for _, want := range []string{
		"templ generate",
		"narsilc generate",
		"go fmt ./domains/internal/queries/...",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("command log missing %q:\n%s", want, log)
		}
	}
}

func TestBuildAppReportsErrors(t *testing.T) {
	resetCLITestSeams(t)

	root := t.TempDir()
	if _, err := buildApp(
		root,
		"",
		nil,
	); err == nil ||
		!strings.Contains(err.Error(), "failed to read andurel.toml") {
		t.Fatalf("expected missing lock error, got %v", err)
	}

	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	writeTestFile(t, root, "cmd/app/main.go", "package main\n\nfunc main() {}\n")
	lock := layout.NewAndurelLock("test")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName:              "app",
		Inertia:                  "vue",
		JavaScriptPackageManager: "npm",
	}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	lockPath := filepath.Join(root, layout.ProjectTomlName)
	lockContent := readBuildTestFile(t, lockPath)
	replaced := strings.NewReplacer(
		`javascriptPackageManager = "npm"`, `javascriptPackageManager = "deno"`,
		`javascriptPackageManager = 'npm'`, `javascriptPackageManager = 'deno'`,
	).Replace(lockContent)
	if replaced == lockContent {
		t.Fatal("failed to rewrite javascriptPackageManager in andurel.toml")
	}
	writeTestFile(t, root, layout.ProjectTomlName, replaced)
	if _, err := buildApp(
		root,
		"",
		nil,
	); err == nil ||
		!strings.Contains(err.Error(), "scaffoldConfig.javascriptPackageManager is invalid") {
		t.Fatalf("expected invalid lock package manager error, got %v", err)
	}
}

func TestBuildAppBuildsEveryProcessExceptSeeds(t *testing.T) {
	resetCLITestSeams(t)

	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	for _, name := range []string{"app", "queue", "migrate", "seeds"} {
		writeTestFile(
			t,
			root,
			filepath.Join("cmd", name, "main.go"),
			"package main\n\nfunc main() {}\n",
		)
	}
	writeTestFile(t, root, "cmd/helper/helper.go", "package helper\n")
	lock := layout.NewAndurelLock("test")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{ProjectName: "app"}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	logPath := filepath.Join(root, "commands.log")
	fakePath := t.TempDir()
	writeExecutable(t, fakePath, "go", fakeGoCommandScript(logPath))
	t.Setenv("PATH", fakePath)

	syncSingleToolFunc = func(projectRoot, name string, tool *layout.Tool, goos, goarch string) error {
		return nil
	}

	report, err := buildApp(root, "1.2.3", nil)
	if err != nil {
		t.Fatalf("buildApp: %v", err)
	}

	var processes []string
	for _, artifact := range report.Artifacts {
		processes = append(processes, artifact.Process)
		if artifact.Path != filepath.ToSlash(filepath.Join("dist", artifact.Process)) {
			t.Fatalf("artifact path = %q", artifact.Path)
		}
		if artifact.Version != "1.2.3" {
			t.Fatalf("artifact version = %q", artifact.Version)
		}
	}
	want := []string{"app", "migrate", "queue"}
	if strings.Join(processes, ",") != strings.Join(want, ",") {
		t.Fatalf("built processes = %v, want %v", processes, want)
	}
	if report.Version != "1.2.3" || report.OutputDir != "dist" {
		t.Fatalf("build report = %#v", report)
	}

	log := readBuildTestFile(t, logPath)
	for _, process := range want {
		wantLine := "go build -v -ldflags -X main.appVersion=1.2.3 -X example.com/app/config.BuildVersion=1.2.3 -o dist/" +
			process + " ./cmd/" + process
		if !strings.Contains(log, wantLine) {
			t.Fatalf("command log missing %q:\n%s", wantLine, log)
		}
	}
	if strings.Contains(log, "./cmd/seeds") || strings.Contains(log, "./cmd/helper") {
		t.Fatalf("command log built an excluded process:\n%s", log)
	}
}

func TestBuildAppOnlyBuildsSelectedProcesses(t *testing.T) {
	resetCLITestSeams(t)

	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	for _, name := range []string{"app", "queue", "migrate"} {
		writeTestFile(
			t,
			root,
			filepath.Join("cmd", name, "main.go"),
			"package main\n\nfunc main() {}\n",
		)
	}
	lock := layout.NewAndurelLock("test")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{ProjectName: "app"}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	logPath := filepath.Join(root, "commands.log")
	fakePath := t.TempDir()
	writeExecutable(t, fakePath, "go", fakeGoCommandScript(logPath))
	t.Setenv("PATH", fakePath)
	syncSingleToolFunc = func(projectRoot, name string, tool *layout.Tool, goos, goarch string) error {
		return nil
	}

	report, err := buildApp(root, "", []string{"queue", "app", "queue"})
	if err != nil {
		t.Fatalf("buildApp: %v", err)
	}
	if len(report.Artifacts) != 2 || report.Artifacts[0].Process != "app" ||
		report.Artifacts[1].Process != "queue" {
		t.Fatalf("build report artifacts = %#v", report.Artifacts)
	}

	log := readBuildTestFile(t, logPath)
	if strings.Contains(log, "./cmd/migrate") {
		t.Fatalf("command log built migrate despite --only:\n%s", log)
	}
}

func TestBuildAppRejectsUnknownOnlyProcess(t *testing.T) {
	resetCLITestSeams(t)

	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	writeTestFile(t, root, "cmd/app/main.go", "package main\n\nfunc main() {}\n")
	if err := layout.NewAndurelLock("test").WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	_, err := buildApp(root, "", []string{"worker"})
	if err == nil || !strings.Contains(err.Error(), "unknown process") {
		t.Fatalf("expected unknown process error, got %v", err)
	}
}

func TestBuildAppReportsMissingProcesses(t *testing.T) {
	resetCLITestSeams(t)

	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	if err := layout.NewAndurelLock("test").WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	_, err := buildApp(root, "", nil)
	if err == nil || !strings.Contains(err.Error(), "no buildable processes") {
		t.Fatalf("expected no processes error, got %v", err)
	}
}

func TestWriteBuildReportListsPathSizeAndVersion(t *testing.T) {
	var out bytes.Buffer
	err := writeBuildReport(&out, buildReport{
		Version:   "1.2.3",
		OutputDir: "dist",
		Artifacts: []buildArtifact{
			{Process: "app", Path: "dist/app", Size: 2 * 1024 * 1024, Version: "1.2.3"},
			{Process: "queue", Path: "dist/queue", Size: 512, Version: "1.2.3"},
		},
	})
	if err != nil {
		t.Fatalf("writeBuildReport: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"Built 2 processes into dist (version 1.2.3)",
		"dist/app",
		"2.0 MB",
		"dist/queue",
		"512 B",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("build report missing %q:\n%s", want, got)
		}
	}
}

func TestBuildCommandJSONReportsArtifactManifest(t *testing.T) {
	resetCLITestSeams(t)

	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	for _, name := range []string{"app", "queue"} {
		writeTestFile(
			t,
			root,
			filepath.Join("cmd", name, "main.go"),
			"package main\n\nfunc main() {}\n",
		)
	}
	lock := layout.NewAndurelLock("test")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{ProjectName: "app"}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	logPath := filepath.Join(root, "commands.log")
	fakePath := t.TempDir()
	writeExecutable(t, fakePath, "go", fakeGoCommandScript(logPath))
	t.Setenv("PATH", fakePath)
	syncSingleToolFunc = func(projectRoot, name string, tool *layout.Tool, goos, goarch string) error {
		return nil
	}

	defaultFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findGoModRoot = defaultFindGoModRoot })

	cmd := newBuildCommand()
	output.RegisterPersistentFlags(cmd)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--json", "--only", "app,queue", "--version", "1.2.3"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("build --json: %v", err)
	}

	var envelope struct {
		OK          bool        `json:"ok"`
		Data        buildReport `json:"data"`
		Summary     string      `json:"summary"`
		Breadcrumbs []struct {
			Command string `json:"cmd"`
		} `json:"breadcrumbs"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode JSON envelope: %v\n%s", err, out.String())
	}
	if !envelope.OK || len(envelope.Data.Artifacts) != 2 {
		t.Fatalf("build JSON envelope = %#v", envelope)
	}
	if envelope.Data.Version != "1.2.3" || envelope.Data.OutputDir != "dist" {
		t.Fatalf("build report = %#v", envelope.Data)
	}
	if envelope.Data.Artifacts[0].Process != "app" ||
		envelope.Data.Artifacts[1].Process != "queue" {
		t.Fatalf("build artifacts = %#v", envelope.Data.Artifacts)
	}
	if len(envelope.Breadcrumbs) == 0 ||
		envelope.Breadcrumbs[0].Command != "andurel patterns show deploy" {
		t.Fatalf("build breadcrumbs = %#v", envelope.Breadcrumbs)
	}
	if strings.Contains(out.String(), "Building app...") {
		t.Fatalf("JSON output leaked human build progress:\n%s", out.String())
	}
}

func TestBuildCommandOutputDirJSONManifest(t *testing.T) {
	resetCLITestSeams(t)

	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	writeTestFile(t, root, "cmd/app/main.go", "package main\n\nfunc main() {}\n")
	lock := layout.NewAndurelLock("test")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{ProjectName: "app"}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	logPath := filepath.Join(root, "commands.log")
	fakePath := t.TempDir()
	writeExecutable(t, fakePath, "go", fakeGoCommandScript(logPath))
	t.Setenv("PATH", fakePath)
	syncSingleToolFunc = func(projectRoot, name string, tool *layout.Tool, goos, goarch string) error {
		return nil
	}

	defaultFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findGoModRoot = defaultFindGoModRoot })

	outputDir := filepath.Join(root, "json out")
	cmd := newBuildCommand()
	output.RegisterPersistentFlags(cmd)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--json", "--only", "app", "--output-dir", outputDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("build --json --output-dir: %v", err)
	}

	var envelope struct {
		OK   bool        `json:"ok"`
		Data buildReport `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode JSON envelope: %v\n%s", err, out.String())
	}
	absoluteOutput, err := filepath.Abs(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if !envelope.OK || !envelope.Data.Isolated ||
		envelope.Data.OutputDir != filepath.ToSlash(absoluteOutput) {
		t.Fatalf("build report = %#v", envelope.Data)
	}
	if len(envelope.Data.Artifacts) != 1 ||
		envelope.Data.Artifacts[0].Path != filepath.ToSlash(filepath.Join(absoluteOutput, "app")) {
		t.Fatalf("artifacts = %#v", envelope.Data.Artifacts)
	}
}

func TestSelectBuildProcesses(t *testing.T) {
	processes := []buildProcess{{Name: "app"}, {Name: "migrate"}, {Name: "queue"}}

	selected, err := selectBuildProcesses(processes, nil)
	if err != nil {
		t.Fatalf("selectBuildProcesses: %v", err)
	}
	if len(selected) != len(processes) {
		t.Fatalf("selected = %#v", selected)
	}

	selected, err = selectBuildProcesses(processes, []string{"queue", "app"})
	if err != nil {
		t.Fatalf("selectBuildProcesses: %v", err)
	}
	if len(selected) != 2 || selected[0].Name != "app" || selected[1].Name != "queue" {
		t.Fatalf("selected = %#v", selected)
	}

	if _, err := selectBuildProcesses(processes, []string{"worker"}); err == nil ||
		!strings.Contains(err.Error(), "available: app, migrate, queue") {
		t.Fatalf("expected unknown process error, got %v", err)
	}
	if _, err := selectBuildProcesses(processes, []string{""}); err == nil ||
		!strings.Contains(err.Error(), "selected no processes") {
		t.Fatalf("expected empty selection error, got %v", err)
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func fakeCommandScript(logPath, name string) string {
	return fmt.Sprintf("#!/bin/sh\nprintf '%%s %%s\\n' %q \"$*\" >> %q\n", name, logPath)
}

// fakeGoCommandScript logs the Go command and, for `go build`, creates the
// requested -o output so buildApp can report its size. buildApp creates the
// output directory first, so the script needs no external commands.
func fakeGoCommandScript(logPath string) string {
	return fmt.Sprintf(`#!/bin/sh
printf 'go %%s\n' "$*" >> %q
if [ "$1" = "build" ]; then
	previous=""
	for argument in "$@"; do
		if [ "$previous" = "-o" ]; then
			: >"$argument"
		fi
		previous="$argument"
	done
fi
`, logPath)
}

func readBuildTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
