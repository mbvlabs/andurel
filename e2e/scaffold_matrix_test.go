package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mbvlabs/andurel/e2e/internal"
	"github.com/sebdah/goldie/v2"
)

type ScaffoldConfig struct {
	Name       string
	Database   string
	CSS        string
	DIMode     string
	Inertia    string
	Extensions []string
	Critical   bool
}

func getScaffoldConfigs() []ScaffoldConfig {
	return []ScaffoldConfig{
		{
			Name:     "postgresql-tailwind",
			Database: "postgresql",
			CSS:      "tailwind",
			Critical: true,
		},
		{
			Name:     "postgresql-vanilla",
			Database: "postgresql",
			CSS:      "vanilla",
			Critical: true,
		},
		{
			Name:       "postgresql-vanilla-css-components",
			Database:   "postgresql",
			CSS:        "vanilla",
			Extensions: []string{"css-components"},
			Critical:   true,
		},
		{
			Name:       "postgresql-tailwind-docker",
			Database:   "postgresql",
			CSS:        "tailwind",
			Extensions: []string{"docker"},
			Critical:   true,
		},
		{
			Name:       "postgresql-tailwind-aws-ses",
			Database:   "postgresql",
			CSS:        "tailwind",
			Extensions: []string{"aws-ses"},
			Critical:   true,
		},
		{
			Name:     "postgresql-tailwind-uberfx",
			Database: "postgresql",
			CSS:      "tailwind",
			DIMode:   "uberfx",
			Critical: true,
		},
		{
			Name:     "postgresql-tailwind-inertia-vue",
			Database: "postgresql",
			CSS:      "tailwind",
			Inertia:  "vue",
			Critical: true,
		},
		{
			Name:       "postgresql-vanilla-uberfx-all-extensions",
			Database:   "postgresql",
			CSS:        "vanilla",
			DIMode:     "uberfx",
			Extensions: []string{"docker", "aws-ses", "css-components"},
			Critical:   true,
		},
	}
}

func TestScaffoldGoldens(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E scaffold golden test in short mode")
	}

	binary := buildAndurelBinary(t)
	g := goldie.New(t, goldie.WithFixtureDir("testdata/golden/scaffolds"))

	configs := getScaffoldConfigs()

	for _, config := range configs {
		t.Run(config.Name, func(t *testing.T) {
			if isCriticalOnly() && !config.Critical {
				t.Skip("Skipping non-critical test in critical-only mode")
			}

			project := internal.NewProject(t, binary, getSharedBinDir())

			args := []string{
				"-c", config.CSS,
			}

			if config.DIMode != "" {
				args = append(args, "--di", config.DIMode)
			}

			if config.Inertia != "" {
				args = append(args, "--inertia", config.Inertia)
			}

			if len(config.Extensions) > 0 {
				for _, ext := range config.Extensions {
					args = append(args, "-e", ext)
				}
			}

			err := project.Scaffold(args...)
			internal.AssertCommandSucceeds(t, err, "scaffold")

			snapshot := scaffoldSnapshot(t, project)
			g.Assert(t, config.Name, []byte(snapshot))

			internal.AssertGoVetPasses(t, project)
		})
	}
}

func scaffoldSnapshot(t *testing.T, project *internal.Project) string {
	t.Helper()

	normalizer := newScaffoldNormalizer(t, project)

	var entries []snapshotEntry
	err := filepath.WalkDir(project.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(project.Dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if shouldSkipSnapshotPath(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		entry := snapshotEntry{
			Path: rel,
			Mode: info.Mode().
				Type().
				String() + info.Mode().Perm().String(),
		}

		switch {
		case d.IsDir():
			entry.Kind = "dir"
		case info.Mode().IsRegular():
			entry.Kind = "file"
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if isText(content) {
				entry.Content = normalizer.normalize(rel, content)
			} else {
				entry.Binary = true
				entry.Size = info.Size()
			}
		default:
			entry.Kind = info.Mode().Type().String()
		}

		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk scaffolded project: %v", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	var b strings.Builder
	for _, entry := range entries {
		switch entry.Kind {
		case "dir":
			fmt.Fprintf(&b, "dir  %s %s\n\n", entry.Mode, entry.Path)
		case "file":
			if entry.Binary {
				fmt.Fprintf(&b, "file %s %s <binary %d bytes>\n\n", entry.Mode, entry.Path, entry.Size)
				continue
			}
			fmt.Fprintf(&b, "file %s %s\n", entry.Mode, entry.Path)
			b.WriteString("```")
			b.WriteByte('\n')
			b.WriteString(entry.Content)
			if !strings.HasSuffix(entry.Content, "\n") {
				b.WriteByte('\n')
			}
			b.WriteString("```\n\n")
		default:
			fmt.Fprintf(&b, "%s %s %s\n\n", entry.Kind, entry.Mode, entry.Path)
		}
	}

	return b.String()
}

type snapshotEntry struct {
	Path    string
	Kind    string
	Mode    string
	Content string
	Binary  bool
	Size    int64
}

type scaffoldNormalizer struct {
	projectDir    string
	secretByValue map[string]string
}

func newScaffoldNormalizer(t *testing.T, project *internal.Project) *scaffoldNormalizer {
	t.Helper()

	normalizer := &scaffoldNormalizer{
		projectDir:    filepath.ToSlash(project.Dir),
		secretByValue: map[string]string{},
	}

	envPath := filepath.Join(project.Dir, ".env.example")
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read .env.example for normalization: %v", err)
	}

	for _, line := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || value == "" {
			continue
		}
		switch key {
		case "SESSION_KEY":
			normalizer.secretByValue[value] = "<SESSION_KEY>"
		case "SESSION_ENCRYPTION_KEY":
			normalizer.secretByValue[value] = "<SESSION_ENCRYPTION_KEY>"
		case "TOKEN_SIGNING_KEY":
			normalizer.secretByValue[value] = "<TOKEN_SIGNING_KEY>"
		case "PEPPER":
			normalizer.secretByValue[value] = "<PEPPER>"
		}
	}

	return normalizer
}

func (n *scaffoldNormalizer) normalize(rel string, content []byte) string {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	text = strings.ReplaceAll(text, filepath.FromSlash(n.projectDir), "<PROJECT_DIR>")
	text = strings.ReplaceAll(text, n.projectDir, "<PROJECT_DIR>")

	for value, placeholder := range n.secretByValue {
		text = strings.ReplaceAll(text, value, placeholder)
	}

	if rel == "andurel.lock" {
		text = normalizeLockFile(text)
	}

	return text
}

func normalizeLockFile(content string) string {
	var data any
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return content
	}

	normalizeAppliedAt(data)

	normalized, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return content
	}

	return string(normalized) + "\n"
}

func normalizeAppliedAt(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "appliedAt" {
				typed[key] = "<APPLIED_AT>"
				continue
			}
			normalizeAppliedAt(child)
		}
	case []any:
		for _, child := range typed {
			normalizeAppliedAt(child)
		}
	}
}

func shouldSkipSnapshotPath(rel string, d fs.DirEntry) bool {
	if rel == ".git" || strings.HasPrefix(rel, ".git/") {
		return true
	}
	if rel == "bin" || strings.HasPrefix(rel, "bin/") {
		return true
	}

	return false
}

func isText(content []byte) bool {
	if bytes.IndexByte(content, 0) >= 0 {
		return false
	}
	if len(content) == 0 {
		return true
	}
	return utf8.Valid(content)
}
