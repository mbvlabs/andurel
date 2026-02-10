package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreatedFileTracker_CleanupCreatedFiles(t *testing.T) {
	rootDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(rootDir, "go.mod"), []byte("module test"), 0o644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(rootDir, "existing"), 0o755); err != nil {
		t.Fatalf("failed to create existing dir: %v", err)
	}
	existingFile := filepath.Join(rootDir, "existing", "keep.txt")
	if err := os.WriteFile(existingFile, []byte("keep"), 0o644); err != nil {
		t.Fatalf("failed to write existing file: %v", err)
	}

	existingFiles, err := snapshotFiles(rootDir)
	if err != nil {
		t.Fatalf("failed to snapshot files: %v", err)
	}

	tracker := &createdFileTracker{
		rootDir:       rootDir,
		existingFiles: existingFiles,
	}

	newFileOne := filepath.Join(rootDir, "controllers", "users.go")
	if err := os.MkdirAll(filepath.Dir(newFileOne), 0o755); err != nil {
		t.Fatalf("failed to create controllers dir: %v", err)
	}
	if err := os.WriteFile(newFileOne, []byte("new"), 0o644); err != nil {
		t.Fatalf("failed to write new file one: %v", err)
	}

	newFileTwo := filepath.Join(rootDir, "views", "users_resource.templ")
	if err := os.MkdirAll(filepath.Dir(newFileTwo), 0o755); err != nil {
		t.Fatalf("failed to create views dir: %v", err)
	}
	if err := os.WriteFile(newFileTwo, []byte("new"), 0o644); err != nil {
		t.Fatalf("failed to write new file two: %v", err)
	}

	removed, cleanupFailures, err := tracker.cleanupCreatedFiles()
	if err != nil {
		t.Fatalf("cleanupCreatedFiles failed: %v", err)
	}

	if len(cleanupFailures) != 0 {
		t.Fatalf("expected no cleanup failures, got: %v", cleanupFailures)
	}

	if len(removed) != 2 {
		t.Fatalf("expected 2 removed files, got %d (%v)", len(removed), removed)
	}

	if _, statErr := os.Stat(existingFile); statErr != nil {
		t.Fatalf("existing file should remain, got error: %v", statErr)
	}

	if _, statErr := os.Stat(newFileOne); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("newFileOne should be removed, got: %v", statErr)
	}
	if _, statErr := os.Stat(newFileTwo); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("newFileTwo should be removed, got: %v", statErr)
	}
}

func TestFormatGenerateFailure_WithCleanupDetails(t *testing.T) {
	runErr := errors.New("failed to generate model: sqlc failed")
	formattedErr := formatGenerateFailure(
		runErr,
		[]string{"controllers/users.go", "views/users_resource.templ"},
		[]string{"router/routes/users.go (permission denied)"},
		nil,
	)

	msg := formattedErr.Error()
	expectedParts := []string{
		"failed to generate model: sqlc failed",
		"Generation failed and automatic cleanup ran.",
		"Removed 2 created file(s):",
		"controllers/users.go",
		"Could not remove 1 file(s):",
		"Please remove these files manually.",
	}

	for _, part := range expectedParts {
		if !strings.Contains(msg, part) {
			t.Fatalf("expected formatted error to contain %q, got: %s", part, msg)
		}
	}
}
