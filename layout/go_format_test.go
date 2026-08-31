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

	assertScaffoldGoFormat(t, root, "example.com/app")
}

func TestGeneratedInertiaScaffoldGoFilesGolinesCompatibleFormatting(t *testing.T) {
	root := t.TempDir()
	if err := Scaffold(root, "testapp", "postgresql", "test", nil, "react", ""); err != nil {
		t.Fatalf("scaffold inertia project: %v", err)
	}

	assertScaffoldGoFormat(t, root, "testapp")
}

func assertScaffoldGoFormat(t *testing.T, root, moduleName string) {
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
		assertGoImportOrder(t, body, moduleName, rel)
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

func assertGoImportOrder(t *testing.T, content, moduleName, name string) {
	t.Helper()

	block := extractImportBlock(content)
	if block == "" {
		return
	}

	groups := splitImportGroups(block)
	if len(groups) == 0 {
		return
	}

	wantKinds := []importKind{importStdlib}
	if containsImportKind(groups, importLocal, moduleName) {
		wantKinds = append(wantKinds, importLocal)
	}
	if containsImportKind(groups, importThirdParty, moduleName) {
		wantKinds = append(wantKinds, importThirdParty)
	}

	if len(groups) != len(wantKinds) {
		t.Errorf("%s: import groups = %d, want %d", name, len(groups), len(wantKinds))
	}

	for i, group := range groups {
		if i >= len(wantKinds) {
			break
		}
		for _, path := range group {
			if got := importGroupKind(path, moduleName); got != wantKinds[i] {
				t.Errorf("%s: import %q is in wrong group, got %v want %v", name, path, got, wantKinds[i])
			}
		}
	}
}

func extractImportBlock(content string) string {
	const prefix = "import ("
	start := strings.Index(content, prefix)
	if start == -1 {
		return ""
	}
	start += len(prefix)
	end := strings.Index(content[start:], "\n)")
	if end == -1 {
		return ""
	}
	return content[start : start+end]
}

func splitImportGroups(block string) [][]string {
	var groups [][]string
	var current []string

	for line := range strings.SplitSeq(block, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(current) > 0 {
				groups = append(groups, current)
				current = nil
			}
			continue
		}
		if path := parseImportPath(trimmed); path != "" {
			current = append(current, path)
		}
	}

	if len(current) > 0 {
		groups = append(groups, current)
	}

	return groups
}

func parseImportPath(line string) string {
	start := strings.Index(line, `"`)
	if start == -1 {
		return ""
	}
	end := strings.Index(line[start+1:], `"`)
	if end == -1 {
		return ""
	}
	return line[start+1 : start+1+end]
}

type importKind int

const (
	importStdlib importKind = iota
	importLocal
	importThirdParty
)

func containsImportKind(groups [][]string, kind importKind, moduleName string) bool {
	for _, group := range groups {
		for _, path := range group {
			if importGroupKind(path, moduleName) == kind {
				return true
			}
		}
	}
	return false
}

func importGroupKind(path, moduleName string) importKind {
	if path == moduleName || strings.HasPrefix(path, moduleName+"/") {
		return importLocal
	}
	if !strings.Contains(path, ".") {
		return importStdlib
	}
	return importThirdParty
}
