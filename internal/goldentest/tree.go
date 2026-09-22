package goldentest

import (
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FullScaffoldDenylist is the nightly `andurel new` capture denylist.
// Paths that float, contain secrets, or are local toolchain artifacts are
// omitted. models/internal/queries and *_templ.go are captured.
var FullScaffoldDenylist = []string{
	".git/",
	"bin/",
	".env",
	".env.example",
	"go.sum",
	"node_modules/",
}

// ShouldSkipFullScaffoldPath reports whether rel matches denylist entries.
// Directory entries such as ".git/" also match the directory itself.
func ShouldSkipFullScaffoldPath(rel string, isDir bool, denylist []string) bool {
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" {
		return false
	}
	for _, raw := range denylist {
		if denylistMatch(rel, isDir, raw) {
			return true
		}
	}
	return false
}

func denylistMatch(rel string, isDir bool, rule string) bool {
	rule = filepath.ToSlash(strings.TrimSpace(rule))
	if rule == "" {
		return false
	}
	if before, ok := strings.CutSuffix(rule, "/"); ok {
		dir := before
		return rel == dir || strings.HasPrefix(rel, dir+"/")
	}
	return rel == rule
}

// AssertTree goldie-asserts every non-denylisted file under projectDir against
// testdata/golden/<goldenSubdir>/<rel>. Orphan .golden files (no project
// counterpart, or a denylisted path) fail the test. When -update is set,
// orphans are removed instead of failing so blessing can drop stale snapshots.
func AssertTree(t testing.TB, projectDir, goldenSubdir string, denylist []string) {
	t.Helper()
	if denylist == nil {
		denylist = FullScaffoldDenylist
	}

	g := NewGoldie(t)
	seen := map[string]struct{}{}

	err := filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(projectDir, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if ShouldSkipFullScaffoldPath(relSlash, d.IsDir(), denylist) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		seen[relSlash] = struct{}{}
		goldenName := filepath.ToSlash(filepath.Join(goldenSubdir, relSlash))
		AssertFile(t, g, goldenName, projectDir, relSlash)
		return nil
	})
	if err != nil {
		t.Fatalf("walk project %s: %v", projectDir, err)
	}

	goldenRoot := filepath.Join(GoldenDir(), filepath.FromSlash(goldenSubdir))
	if _, err := os.Stat(goldenRoot); err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("stat golden dir %s: %v", goldenRoot, err)
	}

	var orphans []string
	err = filepath.WalkDir(goldenRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".golden" {
			return nil
		}
		rel, err := filepath.Rel(goldenRoot, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		projectRel := strings.TrimSuffix(relSlash, ".golden")
		if _, ok := seen[projectRel]; ok {
			return nil
		}
		orphans = append(orphans, relSlash)
		if updatingGoldens() {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk golden dir %s: %v", goldenRoot, err)
	}
	if len(orphans) == 0 {
		return
	}
	if updatingGoldens() {
		t.Logf("removed %d orphan golden file(s) under %s", len(orphans), goldenSubdir)
		return
	}
	t.Fatalf(
		"orphan golden files under %s (no project counterpart): %s",
		goldenSubdir,
		strings.Join(orphans, ", "),
	)
}

func updatingGoldens() bool {
	f := flag.Lookup("update")
	if f == nil {
		return false
	}
	return f.Value.String() == "true"
}
