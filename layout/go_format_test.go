package layout

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const golinesMaxLineLength = 100

func TestGeneratedScaffoldGoFilesGolinesCompatibleFormatting(t *testing.T) {
	root := t.TempDir()
	data := &TemplateData{ModuleName: "example.com/app"}
	data.SetBlueprint(initializeBlueprint("example.com/app"))
	if err := processTemplatedFiles(root, data); err != nil {
		t.Fatalf("process templates: %v", err)
	}

	assertScaffoldGoFormat(t, root)
}

func TestGeneratedInertiaScaffoldGoFilesGolinesCompatibleFormatting(t *testing.T) {
	root := t.TempDir()
	if err := Scaffold(root, "testapp", "postgresql", "test", nil, "react", ""); err != nil {
		t.Fatalf("scaffold inertia project: %v", err)
	}

	assertScaffoldGoFormat(t, root)
}

func assertScaffoldGoFormat(t *testing.T, root string) {
	t.Helper()
	assertScaffoldGoSpacing(t, root)

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_templ.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		body := string(content)
		assertGoMaxLineLength(t, body, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk generated scaffold: %v", err)
	}
}

func assertGoMaxLineLength(t *testing.T, content, name string) {
	t.Helper()

	for i, line := range strings.Split(content, "\n") {
		if len(line) <= golinesMaxLineLength || isAllowedLongLine(line) {
			continue
		}
		t.Errorf(
			"%s:%d: line length = %d, want <= %d",
			name,
			i+1,
			len(line),
			golinesMaxLineLength,
		)
	}
}

func isAllowedLongLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.Contains(trimmed, "WithExplicitBucketBoundaries") {
		return true
	}
	if strings.HasPrefix(trimmed, "//") {
		return true
	}
	return false
}
