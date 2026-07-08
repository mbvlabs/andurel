package cli

import (
	"os/exec"
	"strings"
	"testing"
)

func TestInertiaPackageManagerCommands(t *testing.T) {
	tests := []struct {
		name        string
		runtime     string
		wantInstall string
		wantBuild   string
	}{
		{
			name:        "default runtime uses npm",
			runtime:     "",
			wantInstall: "npm ci",
			wantBuild:   "npm run build",
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
		{
			name:        "yarn",
			runtime:     "yarn",
			wantInstall: "yarn install --frozen-lockfile",
			wantBuild:   "yarn build",
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
	if _, _, err := inertiaPackageManagerCommands("deno"); err == nil {
		t.Fatal("expected unsupported runtime error")
	}
}

func TestExtractModuleName(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module github.com/acme/orders\n\ngo 1.26\n")

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

	writeTestFile(t, root, "go.mod", "go 1.26\n")
	if _, err := extractModuleName(root); err == nil || !strings.Contains(err.Error(), "module directive") {
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

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}
