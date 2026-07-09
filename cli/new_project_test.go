package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/cli/output"
)

func TestNewProjectRejectsExtraPositionalArguments(t *testing.T) {
	cmd := newProjectCommand("test")
	err := cmd.Args(cmd, []string{"app", "extra"})
	assertNewProjectCLIError(t, err, output.CodeUsage)
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
			assertNewProjectCLIError(t, err, output.CodeUsage)
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
		assertNewProjectCLIError(t, err, output.CodeUsage)
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
	assertNewProjectCLIError(t, err, output.CodeUnsafeAction)

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
	assertNewProjectCLIError(t, err, output.CodeUnsafeAction)

	hidden := filepath.Join(root, ".hidden")
	if err := os.Mkdir(hidden, 0o755); err != nil {
		t.Fatalf("create hidden directory: %v", err)
	}
	_, err = normalizeNewProjectDestination(hidden, ".")
	assertNewProjectCLIError(t, err, output.CodeUsage)
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
