package skills

import (
	"errors"
	"strings"
	"testing"
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

func TestWalkAndurelSkillFilesPropagatesCallbackError(t *testing.T) {
	expectedErr := errors.New("stop walking")

	err := WalkAndurelSkillFiles(func(path string, data []byte) error {
		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected callback error, got %v", err)
	}
}
