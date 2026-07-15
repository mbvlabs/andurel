package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/layout"
)

func TestNewProjectRejectsExtraPositionalArguments(t *testing.T) {
	cmd := newProjectCommand("test")
	err := cmd.Args(cmd, []string{"app", "extra"})
	_ = assertNewProjectCLIError(t, err, output.CodeUsage)
}

func TestValidateNewProjectName(t *testing.T) {
	for _, name := range []string{
		"app",
		"App",
		"app-1",
		"app_name",
		"app.name",
		"9app",
		"App.Name_9-alpha",
	} {
		t.Run("valid_"+name, func(t *testing.T) {
			if err := validateNewProjectName(name); err != nil {
				t.Fatalf("validateNewProjectName(%q): %v", name, err)
			}
		})
	}

	for _, name := range []string{
		"",
		".",
		"..",
		".hidden",
		"-app",
		"_app",
		"app/name",
		`app\name`,
		"/absolute",
		"app name",
		" app",
		"app ",
		"café",
	} {
		t.Run("invalid_"+name, func(t *testing.T) {
			err := validateNewProjectName(name)
			_ = assertNewProjectCLIError(t, err, output.CodeUsage)
		})
	}
}

func TestNormalizeNewProjectDestinationBeforeCreation(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "SafeParent")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	symlink := filepath.Join(root, "parent-link")
	if err := os.Symlink(parent, symlink); err != nil {
		t.Fatalf("create parent symlink: %v", err)
	}

	destination, err := normalizeNewProjectDestination(symlink, "App-1")
	if err != nil {
		t.Fatalf("normalizeNewProjectDestination: %v", err)
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		t.Fatalf("resolve expected parent: %v", err)
	}
	if destination.projectName != "App-1" || destination.path != filepath.Join(resolvedParent, "App-1") || destination.currentDirectory {
		t.Fatalf("unexpected destination: %#v", destination)
	}
	if _, err := os.Stat(destination.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("validation created destination or returned an unexpected error: %v", err)
	}

	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("read parent: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("validation mutated parent directory: %#v", entries)
	}

	if _, err := normalizeNewProjectDestination(parent, "../escape"); err == nil {
		t.Fatal("expected traversal name to be rejected")
	} else {
		_ = assertNewProjectCLIError(t, err, output.CodeUsage)
	}
}

func TestNormalizeNewProjectDestinationRejectsExistingAndNonEmptyTargets(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "Parent")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	existing := filepath.Join(parent, "existing")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatalf("create existing destination: %v", err)
	}
	_, err := normalizeNewProjectDestination(parent, "existing")
	_ = assertNewProjectCLIError(t, err, output.CodeUnsafeAction)

	current := filepath.Join(root, "Current.App")
	if err := os.Mkdir(current, 0o755); err != nil {
		t.Fatalf("create current directory: %v", err)
	}
	destination, err := normalizeNewProjectDestination(current, ".")
	if err != nil {
		t.Fatalf("normalize empty current directory: %v", err)
	}
	if destination.projectName != "Current.App" || destination.path != current || !destination.currentDirectory {
		t.Fatalf("unexpected current-directory destination: %#v", destination)
	}

	if err := os.WriteFile(filepath.Join(current, "existing.txt"), []byte("user-owned"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	_, err = normalizeNewProjectDestination(current, ".")
	_ = assertNewProjectCLIError(t, err, output.CodeUnsafeAction)

	hidden := filepath.Join(root, ".hidden")
	if err := os.Mkdir(hidden, 0o755); err != nil {
		t.Fatalf("create hidden directory: %v", err)
	}
	_, err = normalizeNewProjectDestination(hidden, ".")
	_ = assertNewProjectCLIError(t, err, output.CodeUsage)
}

func TestScaffoldNewProjectPublishesOnlyCompleteProject(t *testing.T) {
	root := t.TempDir()
	destination := newProjectDestination{
		projectName: "app",
		path:        filepath.Join(root, "app"),
	}

	err := scaffoldNewProject(destination, func(target string) error {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(target, "partial.txt"), []byte("partial"), 0o644); err != nil {
			return err
		}
		return errors.New("injected scaffold failure")
	})
	if err == nil {
		t.Fatal("expected staged scaffold failure")
	}
	if _, err := os.Stat(destination.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed scaffold left a destination behind: %v", err)
	}
	assertNoNewProjectStagingDirectories(t, root)

	if err := scaffoldNewProject(destination, func(target string) error {
		return os.MkdirAll(filepath.Join(target, "complete"), 0o755)
	}); err != nil {
		t.Fatalf("publish complete scaffold: %v", err)
	}
	if info, err := os.Stat(filepath.Join(destination.path, "complete")); err != nil || !info.IsDir() {
		t.Fatalf("complete scaffold was not published: info=%v err=%v", info, err)
	}
	assertNoNewProjectStagingDirectories(t, root)
}

func TestScaffoldNewProjectCurrentDirectoryFailureIsNonMutating(t *testing.T) {
	root := t.TempDir()
	destinationPath := filepath.Join(root, "CurrentApp")
	if err := os.Mkdir(destinationPath, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	destination := newProjectDestination{
		projectName:      "CurrentApp",
		path:             destinationPath,
		currentDirectory: true,
	}

	err := scaffoldNewProject(destination, func(target string) error {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(target, "partial.txt"), []byte("partial"), 0o644); err != nil {
			return err
		}
		return errors.New("injected scaffold failure")
	})
	if err == nil {
		t.Fatal("expected staged scaffold failure")
	}
	entries, err := os.ReadDir(destinationPath)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed current-directory scaffold left partial files: %#v", entries)
	}
	assertNoNewProjectStagingDirectories(t, root)

	wrapped := wrapNewProjectScaffoldError(err)
	cliErr := assertNewProjectCLIError(t, wrapped, output.CodeGenerationFailed)
	if !strings.Contains(cliErr.Hint, "No partial scaffold was retained") {
		t.Fatalf("generation error hint is not actionable: %q", cliErr.Hint)
	}

	if err := scaffoldNewProject(destination, func(target string) error {
		if err := os.MkdirAll(filepath.Join(target, ".git"), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(target, "go.mod"), []byte("module CurrentApp\n"), 0o644)
	}); err != nil {
		t.Fatalf("publish complete current-directory scaffold: %v", err)
	}
	for _, name := range []string{".git", "go.mod"} {
		if _, err := os.Stat(filepath.Join(destinationPath, name)); err != nil {
			t.Fatalf("published current-directory scaffold missing %s: %v", name, err)
		}
	}
	assertNoNewProjectStagingDirectories(t, root)
}

func TestNewProjectRejectsInvalidInertiaConfigurations(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	for _, test := range []struct {
		value string
		want  string
	}{
		{value: "angular", want: "invalid inertia adapter"},
		{value: "vue/deno", want: "invalid JavaScript runtime"},
	} {
		cmd := newProjectCommand("test")
		if err := cmd.Flags().Set("inertia", test.value); err != nil {
			t.Fatalf("set inertia flag: %v", err)
		}
		err := newProject(cmd, []string{"sample"}, "test", false, false)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("inertia %q error = %v, want %q", test.value, err, test.want)
		}
	}
}

func TestNewProjectAcceptsSvelteRuntimeSuffixes(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	for _, runtime := range []string{"npm", "pnpm", "bun", "yarn"} {
		projectName := "svelte-" + runtime
		cmd := newProjectCommand("test")
		if err := cmd.Flags().Set("inertia", "svelte/"+runtime); err != nil {
			t.Fatalf("set inertia flag: %v", err)
		}
		if err := newProject(cmd, []string{projectName}, "test", false, false); err != nil {
			t.Fatalf("create %s project: %v", runtime, err)
		}

		lock, err := layout.ReadLockFile(filepath.Join(root, projectName))
		if err != nil {
			t.Fatalf("read %s lock: %v", runtime, err)
		}
		if lock.ScaffoldConfig == nil || lock.ScaffoldConfig.Inertia != "svelte" || lock.ScaffoldConfig.JavaScriptRuntime != runtime {
			t.Fatalf("%s scaffold config = %#v", runtime, lock.ScaffoldConfig)
		}
	}
}

func TestNewProjectReportDryRunAndExistingTarget(t *testing.T) {
	root := t.TempDir()
	report, err := newProjectReport("sample", filepath.Join(root, "unused"), true, true, func(target string) error {
		if !strings.Contains(target, "andurel-new-dry-run-") {
			t.Fatalf("dry run target is not temporary: %s", target)
		}
		if err := os.MkdirAll(filepath.Join(target, "nested"), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(target, "nested", "file.txt"), []byte("created\n"), 0o644)
	})
	if err != nil {
		t.Fatalf("build dry-run report: %v", err)
	}
	if !report.DryRun || report.Action != "new project" || report.Resource != "sample" || len(report.FilesCreated) == 0 {
		t.Fatalf("unexpected dry-run report: %#v", report)
	}

	target := filepath.Join(root, "published")
	report, err = newProjectReport("sample", target, false, false, func(got string) error {
		if got != target {
			t.Fatalf("target = %q, want %q", got, target)
		}
		if err := os.MkdirAll(got, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(got, "go.mod"), []byte("module sample\n"), 0o644)
	})
	if err != nil {
		t.Fatalf("build published report: %v", err)
	}
	if report.DryRun || len(report.FilesCreated) != 1 || report.FilesCreated[0] != "go.mod" {
		t.Fatalf("unexpected published report: %#v", report)
	}

	injected := errors.New("injected report failure")
	if _, err := newProjectReport("sample", target, false, false, func(string) error { return injected }); !errors.Is(err, injected) {
		t.Fatalf("expected scaffold error, got %v", err)
	}
}

func TestScaffoldNewProjectDetectsPublishRacesAndInvalidSources(t *testing.T) {
	root := t.TempDir()
	destination := newProjectDestination{projectName: "sample", path: filepath.Join(root, "sample")}
	err := scaffoldNewProject(destination, func(target string) error {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
		return os.Mkdir(destination.path, 0o755)
	})
	if err == nil || !strings.Contains(err.Error(), "now exists") {
		t.Fatalf("expected destination race error, got %v", err)
	}
	assertNoNewProjectStagingDirectories(t, root)

	staged := filepath.Join(root, "staged")
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(staged, 0o755); err != nil {
		t.Fatalf("create staged directory: %v", err)
	}
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatalf("create current directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(current, "user.txt"), []byte("owned"), 0o644); err != nil {
		t.Fatalf("write current file: %v", err)
	}
	if err := publishStagedProjectContents(staged, current); err == nil || !strings.Contains(err.Error(), "no longer empty") {
		t.Fatalf("expected non-empty destination error, got %v", err)
	}
	if err := publishStagedProjectContents(filepath.Join(root, "missing"), t.TempDir()); err == nil || !strings.Contains(err.Error(), "read staged scaffold") {
		t.Fatalf("expected missing staged scaffold error, got %v", err)
	}
}

func TestNewProjectErrorWrappingPreservesTypedErrors(t *testing.T) {
	typed := output.NewError(output.CodeUnsafeAction, "unsafe", output.ExitUnsafe, "retry elsewhere")
	if got := wrapNewProjectScaffoldError(typed); got != typed {
		t.Fatalf("typed error was replaced: %v", got)
	}

	pathErr := newProjectPathError("inspect", os.ErrPermission)
	cliErr := assertNewProjectCLIError(t, pathErr, output.CodeGenerationFailed)
	if !errors.Is(cliErr, os.ErrPermission) {
		t.Fatalf("wrapped path error lost cause: %v", cliErr)
	}
}

func assertNoNewProjectStagingDirectories(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read staging parent: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".andurel-new-") {
			t.Fatalf("staging directory was not removed: %s", entry.Name())
		}
	}
}

func assertNewProjectCLIError(t *testing.T, err error, code string) *output.CLIError {
	t.Helper()
	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("error = %T %[1]v, want *output.CLIError", err)
	}
	if cliErr.Code != code {
		t.Fatalf("error code = %q, want %q", cliErr.Code, code)
	}
	if cliErr.Hint == "" {
		t.Fatal("typed error is missing an actionable hint")
	}
	return cliErr
}
