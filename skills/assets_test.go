package skills

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestWalkAndurelSkillFiles(t *testing.T) {
	var paths []string
	files := make(map[string]string)

	err := WalkAndurelSkillFiles(func(path string, data []byte) error {
		paths = append(paths, path)
		files[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkAndurelSkillFiles failed: %v", err)
	}

	if len(paths) == 0 {
		t.Fatal("expected embedded skill files")
	}
	if got := files["SKILL.md"]; got != AndurelSkill {
		t.Fatalf("expected SKILL.md to match embedded string")
	}
	for _, path := range paths {
		if strings.HasPrefix(path, "andurel/") {
			t.Fatalf("expected trimmed relative path, got %q", path)
		}
	}
}

func TestReadAndurelSkillFile(t *testing.T) {
	data, err := ReadAndurelSkillFile("SKILL.md")
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	if string(data) != AndurelSkill {
		t.Fatal("SKILL.md does not match the embedded skill body")
	}

	if _, err := ReadAndurelSkillFile("references/does-not-exist.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected missing reference error, got %v", err)
	}
}

type unreadableSkillFS struct {
	fstest.MapFS
}

func (f unreadableSkillFS) ReadFile(name string) ([]byte, error) {
	if name == "andurel/unreadable.md" {
		return nil, fs.ErrPermission
	}
	return f.MapFS.ReadFile(name)
}

func TestWalkAndurelSkillFilesErrorPaths(t *testing.T) {
	t.Run("walk error", func(t *testing.T) {
		err := walkAndurelSkillFiles(fstest.MapFS{}, func(string, []byte) error { return nil })
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("expected missing root error, got %v", err)
		}
	})

	t.Run("read error", func(t *testing.T) {
		skillFS := unreadableSkillFS{MapFS: fstest.MapFS{
			"andurel/unreadable.md": &fstest.MapFile{Data: []byte("content")},
		}}
		err := walkAndurelSkillFiles(skillFS, func(string, []byte) error { return nil })
		if !errors.Is(err, fs.ErrPermission) {
			t.Fatalf("expected read permission error, got %v", err)
		}
	})
}

func TestWalkAndurelSkillFilesPropagatesCallbackError(t *testing.T) {
	expectedErr := errors.New("stop walking")

	err := WalkAndurelSkillFiles(func(path string, data []byte) error {
		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected callback error, got %v", err)
	}
}
