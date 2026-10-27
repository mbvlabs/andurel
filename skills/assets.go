// Package skills exposes the embedded Andurel skill documentation and assets.
package skills

import (
	"embed"
	"io/fs"
	"strings"
)

// AndurelSkillFS contains the embedded Andurel skill files.
//
//go:embed andurel
var AndurelSkillFS embed.FS

// AndurelSkill contains the embedded Andurel skill instructions.
//
//go:embed andurel/SKILL.md
var AndurelSkill string

// WalkAndurelSkillFiles walks andurel skill files.
func WalkAndurelSkillFiles(fn func(path string, data []byte) error) error {
	return walkAndurelSkillFiles(AndurelSkillFS, fn)
}

// ReadAndurelSkillFile reads one embedded skill file by its path relative to
// the skill root, for example "references/http-and-routing.md".
func ReadAndurelSkillFile(path string) ([]byte, error) {
	return fs.ReadFile(AndurelSkillFS, "andurel/"+strings.TrimPrefix(path, "/"))
}

func walkAndurelSkillFiles(skillFS fs.FS, fn func(path string, data []byte) error) error {
	return fs.WalkDir(skillFS, "andurel", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		data, err := fs.ReadFile(skillFS, path)
		if err != nil {
			return err
		}
		return fn(strings.TrimPrefix(path, "andurel/"), data)
	})
}
