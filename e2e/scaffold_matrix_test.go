package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
	cssFrameworks := []string{"tailwind"}
	diModes := []string{"uberfx"}
	inertiaModesByCSS := map[string][]string{
		"tailwind": {"", "vue"},
	}
	extensionSets := extensionPowerSet([]string{"docker", "aws-ses", "css-components"})

	var configs []ScaffoldConfig
	for _, css := range cssFrameworks {
		for _, diMode := range diModes {
			for _, inertia := range inertiaModesByCSS[css] {
				for _, extensions := range extensionSets {
					configs = append(configs, ScaffoldConfig{
						Name:       scaffoldConfigName("postgresql", css, diMode, inertia, extensions),
						Database:   "postgresql",
						CSS:        css,
						DIMode:     diMode,
						Inertia:    inertia,
						Extensions: extensions,
						Critical:   isCriticalScaffoldConfig("postgresql", css, diMode, inertia, extensions),
					})
				}
			}
		}
	}

	return configs
}

func isCriticalScaffoldConfig(database, css, diMode, inertia string, extensions []string) bool {
	criticalConfigs := map[string]bool{
		"postgresql-tailwind":                               true,
		"postgresql-tailwind-inertia-vue":                   true,
		"postgresql-tailwind-docker-aws-ses-css-components": true,
	}

	return criticalConfigs[scaffoldConfigName(database, css, diMode, inertia, extensions)]
}

func extensionPowerSet(extensions []string) [][]string {
	sets := make([][]string, 0, 1<<len(extensions))
	for mask := 0; mask < 1<<len(extensions); mask++ {
		var set []string
		for i, extension := range extensions {
			if mask&(1<<i) != 0 {
				set = append(set, extension)
			}
		}
		sets = append(sets, set)
	}

	return sets
}

func scaffoldConfigName(database, css, diMode, inertia string, extensions []string) string {
	parts := []string{database, css}

	if diMode != "" && diMode != "uberfx" {
		parts = append(parts, diMode)
	}

	if inertia != "" {
		parts = append(parts, "inertia", inertia)
	}

	parts = append(parts, extensions...)

	return strings.Join(parts, "-")
}

func TestScaffoldCriticalConfigs(t *testing.T) {
	expected := []string{
		"postgresql-tailwind",
		"postgresql-tailwind-docker-aws-ses-css-components",
		"postgresql-tailwind-inertia-vue",
	}

	var actual []string
	for _, config := range getScaffoldConfigs() {
		if config.Critical {
			actual = append(actual, config.Name)
		}
	}
	sort.Strings(actual)

	if strings.Join(actual, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("critical scaffold configs changed\nexpected:\n%s\nactual:\n%s", strings.Join(expected, "\n"), strings.Join(actual, "\n"))
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

			var args []string

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

var generatedVersionPattern = regexp.MustCompile(`Code generated by andurel [^;]+; DO NOT EDIT\.`)

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
	text = generatedVersionPattern.ReplaceAllString(text, "Code generated by andurel <ANDUREL_VERSION>; DO NOT EDIT.")

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

	normalizeLockVersion(data)
	normalizeAppliedAt(data)

	normalized, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return content
	}

	return string(normalized) + "\n"
}

func normalizeLockVersion(value any) {
	data, ok := value.(map[string]any)
	if !ok {
		return
	}
	if _, ok := data["version"]; ok {
		data["version"] = "<ANDUREL_VERSION>"
	}
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
