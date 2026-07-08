package skills

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed andurel
var AndurelSkillFS embed.FS

//go:embed andurel/SKILL.md
var AndurelSkill string

// WalkAndurelSkillFiles walks andurel skill files.
func WalkAndurelSkillFiles(fn func(path string, data []byte) error) error {
	return fs.WalkDir(AndurelSkillFS, "andurel", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		data, err := AndurelSkillFS.ReadFile(path)
		if err != nil {
			return err
		}
		return fn(strings.TrimPrefix(path, "andurel/"), data)
	})
}
