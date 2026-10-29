package content

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeContentFile(t *testing.T, root, rel, body string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("create %s: %v", rel, err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func writeContentManifest(t *testing.T, root, body string) {
	t.Helper()
	writeContentFile(t, root, ManifestName, body)
}

// validContentFixture is the documented example shape: included docs, an
// excluded draft, a required file, inline and file-backed catalog entries, and
// one local link per allowed construct.
func validContentFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeContentManifest(t, root, `schemaVersion = 1
include = ["content/**/*.md"]
exclude = ["content/drafts/**"]
required = ["content/index.md"]
catalogFile = "content/catalog.toml"

[[catalog]]
slug = "getting-started"
file = "content/docs/getting-started.md"

[slugs]
pattern = "^[a-z0-9]+(?:-[a-z0-9]+)*$"

[links]
anchors = "check"
external = "ignore"
`)
	writeContentFile(t, root, "content/catalog.toml", `schemaVersion = 1

[[entry]]
slug = "install"
file = "content/docs/install.md"
`)
	writeContentFile(t, root, "content/index.md", `# Home

[Getting started](docs/getting-started.md)
[Install](docs/install.md#requirements)
[External](https://example.com/not-checked)
[Absolute](/site/path-not-checked)
[Asset](images/logo.png)
[Section](#home)
`)
	writeContentFile(t, root, "content/docs/getting-started.md", `# Getting started

[Back home](../index.md)
`)
	writeContentFile(t, root, "content/docs/install.md", `# Install

## Requirements

[Back home](../index.md)
`)
	writeContentFile(t, root, "content/drafts/draft.md", `# Draft

[Broken](missing-file.md) links in excluded documents are not checked.
`)
	writeContentFile(t, root, "content/images/logo.png", "png")
	return root
}

func TestLoadMissingManifestIsNotConfigured(t *testing.T) {
	cfg, diagnostics := Load(t.TempDir())
	if cfg != nil || len(diagnostics) != 0 {
		t.Fatalf("Load() = %#v, %#v; want not configured", cfg, diagnostics)
	}
}

func TestCheckValidFixturePasses(t *testing.T) {
	root := validContentFixture(t)
	cfg, diagnostics := Load(root)
	if len(diagnostics) != 0 {
		t.Fatalf("Load diagnostics = %#v", diagnostics)
	}
	if diagnostics := Check(root, cfg); len(diagnostics) != 0 {
		t.Fatalf("Check diagnostics = %#v, want none", diagnostics)
	}
}

func TestLoadRejectsUnsupportedConfig(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		want     string
	}{
		{
			name:     "schema version",
			manifest: "schemaVersion = 2\ninclude = [\"content/**/*.md\"]\n",
			want:     "only 1 is supported",
		},
		{
			name:     "unknown key",
			manifest: "schemaVersion = 1\ninclude = [\"content/**/*.md\"]\nunknownKey = true\n",
			want:     "unknown key",
		},
		{
			name:     "missing include",
			manifest: "schemaVersion = 1\n",
			want:     "include declares no Markdown globs",
		},
		{
			name:     "glob escapes",
			manifest: "schemaVersion = 1\ninclude = [\"../outside/**/*.md\"]\n",
			want:     "escapes the project root",
		},
		{
			name:     "bad anchor mode",
			manifest: "schemaVersion = 1\ninclude = [\"content/**/*.md\"]\n[links]\nanchors = \"strict\"\n",
			want:     "links.anchors",
		},
		{
			name:     "external checking",
			manifest: "schemaVersion = 1\ninclude = [\"content/**/*.md\"]\n[links]\nexternal = \"check\"\n",
			want:     "network link checking is not supported",
		},
		{
			name:     "checker without run",
			manifest: "schemaVersion = 1\ninclude = [\"content/**/*.md\"]\n[[checkers]]\nname = \"docs\"\n",
			want:     "has no run argv",
		},
		{
			name:     "checker dir escapes",
			manifest: "schemaVersion = 1\ninclude = [\"content/**/*.md\"]\n[[checkers]]\nname = \"docs\"\nrun = [\"go\", \"run\", \"./check\"]\ndir = \"../outside\"\n",
			want:     "escapes the project root",
		},
		{
			name:     "checker timeout bound",
			manifest: "schemaVersion = 1\ninclude = [\"content/**/*.md\"]\n[[checkers]]\nname = \"docs\"\nrun = [\"go\", \"run\", \"./check\"]\ntimeoutSeconds = 9999\n",
			want:     "timeoutSeconds must be between",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeContentManifest(t, root, test.manifest)
			cfg, diagnostics := Load(root)
			if cfg != nil {
				t.Fatalf("Load() returned a config for invalid input: %#v", cfg)
			}
			if len(diagnostics) == 0 || diagnostics[0].Rule != RuleConfig {
				t.Fatalf("diagnostics = %#v, want a content-config failure", diagnostics)
			}
			if !strings.Contains(diagnostics[0].Message, test.want) {
				t.Fatalf("message = %q, want %q", diagnostics[0].Message, test.want)
			}
		})
	}
}

func TestCheckCatalogInvariants(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		files    map[string]string
		wantRule string
		want     string
	}{
		{
			name:     "missing target",
			manifest: "schemaVersion = 1\ninclude = [\"content/**/*.md\"]\n[[catalog]]\nslug = \"missing\"\nfile = \"content/missing.md\"\n",
			wantRule: RuleCatalog,
			want:     "targets missing file",
		},
		{
			name:     "duplicate slug",
			manifest: "schemaVersion = 1\ninclude = [\"content/**/*.md\"]\n[[catalog]]\nslug = \"guide\"\nfile = \"content/a.md\"\n[[catalog]]\nslug = \"guide\"\nfile = \"content/b.md\"\n",
			files:    map[string]string{"content/a.md": "# A\n", "content/b.md": "# B\n"},
			wantRule: RuleSlug,
			want:     "duplicate catalog slug",
		},
		{
			name:     "invalid slug",
			manifest: "schemaVersion = 1\ninclude = [\"content/**/*.md\"]\n[[catalog]]\nslug = \"Docs Guide\"\nfile = \"content/a.md\"\n",
			files:    map[string]string{"content/a.md": "# A\n"},
			wantRule: RuleSlug,
			want:     "does not match",
		},
		{
			name:     "non markdown target",
			manifest: "schemaVersion = 1\ninclude = [\"content/**/*.md\"]\n[[catalog]]\nslug = \"guide\"\nfile = \"content/a.txt\"\n",
			files:    map[string]string{"content/a.txt": "text\n"},
			wantRule: RuleCatalog,
			want:     "not a Markdown file",
		},
		{
			name:     "excluded target",
			manifest: "schemaVersion = 1\ninclude = [\"content/**/*.md\"]\nexclude = [\"content/drafts/**\"]\n[[catalog]]\nslug = \"draft\"\nfile = \"content/drafts/draft.md\"\n",
			files:    map[string]string{"content/drafts/draft.md": "# Draft\n"},
			wantRule: RuleCatalog,
			want:     "targets excluded file",
		},
		{
			name:     "target outside include",
			manifest: "schemaVersion = 1\ninclude = [\"content/**/*.md\"]\n[[catalog]]\nslug = \"other\"\nfile = \"other/page.md\"\n",
			files:    map[string]string{"other/page.md": "# Page\n"},
			wantRule: RuleCatalog,
			want:     "outside include",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeContentManifest(t, root, test.manifest)
			for file, body := range test.files {
				writeContentFile(t, root, file, body)
			}
			cfg, diagnostics := Load(root)
			if len(diagnostics) != 0 {
				t.Fatalf("Load diagnostics = %#v", diagnostics)
			}
			diagnostics = Check(root, cfg)
			if len(diagnostics) != 1 {
				t.Fatalf("diagnostics = %#v, want one", diagnostics)
			}
			if diagnostics[0].Rule != test.wantRule || !strings.Contains(diagnostics[0].Message, test.want) {
				t.Fatalf("diagnostic = %#v, want rule %s containing %q", diagnostics[0], test.wantRule, test.want)
			}
		})
	}
}

func TestCheckRequiredPaths(t *testing.T) {
	root := t.TempDir()
	writeContentManifest(t, root, `schemaVersion = 1
include = ["content/**/*.md"]
required = ["content/index.md", "content/missing-dir"]
`)
	writeContentFile(t, root, "content/index.md", "# Home\n")
	cfg, diagnostics := Load(root)
	if len(diagnostics) != 0 {
		t.Fatalf("Load diagnostics = %#v", diagnostics)
	}
	diagnostics = Check(root, cfg)
	if len(diagnostics) != 1 || diagnostics[0].Rule != RuleRequired {
		t.Fatalf("diagnostics = %#v, want one content-required failure", diagnostics)
	}
	if !strings.Contains(diagnostics[0].Message, "content/missing-dir") {
		t.Fatalf("message = %q, want the missing required path", diagnostics[0].Message)
	}
}

func TestLoadMissingCatalogFileIsAConfigFailure(t *testing.T) {
	root := t.TempDir()
	writeContentManifest(t, root, `schemaVersion = 1
include = ["content/**/*.md"]
catalogFile = "content/catalog.toml"
`)
	writeContentFile(t, root, "content/index.md", "# Home\n")
	cfg, diagnostics := Load(root)
	if cfg != nil {
		t.Fatalf("Load() returned a config: %#v", cfg)
	}
	if len(diagnostics) != 1 || diagnostics[0].Rule != RuleConfig ||
		!strings.Contains(diagnostics[0].Message, "catalog") {
		t.Fatalf("diagnostics = %#v, want a content-config catalog failure", diagnostics)
	}
}

func TestLoadRejectsSymlinkedManifestEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture uses POSIX symlinks")
	}
	root := t.TempDir()
	outside := t.TempDir()
	writeContentFile(t, outside, ManifestName, "schemaVersion = 1\ninclude = [\"content/**/*.md\"]\n")
	if err := os.Symlink(filepath.Join(outside, ManifestName), filepath.Join(root, ManifestName)); err != nil {
		t.Fatalf("create manifest symlink: %v", err)
	}

	cfg, diagnostics := Load(root)
	if cfg != nil {
		t.Fatalf("Load() returned a config: %#v", cfg)
	}
	if len(diagnostics) != 1 || diagnostics[0].Rule != RuleConfig ||
		!strings.Contains(diagnostics[0].Message, "outside the project root") {
		t.Fatalf("diagnostics = %#v, want a content-config manifest escape", diagnostics)
	}
}

func TestLoadRejectsSymlinkedCatalogFileEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture uses POSIX symlinks")
	}
	root := t.TempDir()
	outside := t.TempDir()
	writeContentFile(t, outside, "catalog.toml", `schemaVersion = 1

[[entry]]
slug = "escaped"
file = "content/escaped.md"
`)
	writeContentManifest(t, root, `schemaVersion = 1
include = ["content/**/*.md"]
catalogFile = "content/catalog.toml"
`)
	writeContentFile(t, root, "content/index.md", "# Home\n")
	if err := os.MkdirAll(filepath.Join(root, "content"), 0o755); err != nil {
		t.Fatalf("create content directory: %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, "catalog.toml"), filepath.Join(root, "content", "catalog.toml")); err != nil {
		t.Fatalf("create catalog symlink: %v", err)
	}

	cfg, diagnostics := Load(root)
	if cfg != nil {
		t.Fatalf("Load() returned a config: %#v", cfg)
	}
	if len(diagnostics) != 1 || diagnostics[0].Rule != RuleConfig ||
		!strings.Contains(diagnostics[0].Message, "outside the project root") {
		t.Fatalf("diagnostics = %#v, want a content-config catalog escape", diagnostics)
	}
}

func TestLoadRejectsCatalogFileBelowSymlinkedAncestor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture uses POSIX symlinks")
	}
	root := t.TempDir()
	outside := t.TempDir()
	writeContentManifest(t, root, `schemaVersion = 1
include = ["content/**/*.md"]
catalogFile = "linked/new/catalog.toml"
`)
	writeContentFile(t, root, "content/index.md", "# Home\n")
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatalf("create escaping symlink: %v", err)
	}

	cfg, diagnostics := Load(root)
	if cfg != nil {
		t.Fatalf("Load() returned a config: %#v", cfg)
	}
	if len(diagnostics) != 1 || diagnostics[0].Rule != RuleConfig ||
		!strings.Contains(diagnostics[0].Message, "outside the project root") {
		t.Fatalf("diagnostics = %#v, want a content-config catalog escape", diagnostics)
	}
}

func TestCheckLocalLinksAndAnchors(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		want      string
		anchors   string
		extraFile map[string]string
	}{
		{
			name: "missing local link",
			body: "[Missing](missing.md)\n",
			want: "resolves to missing file",
		},
		{
			name: "missing image",
			body: "![Logo](images/missing.png)\n",
			want: "resolves to missing file",
		},
		{
			name: "escapes project",
			body: "[Outside](../../outside.md)\n",
			want: "escapes the project root",
		},
		{
			name: "directory without index",
			body: "[Docs](docs/)\n",
			want: "without index.md or README.md",
		},
		{
			name:    "missing anchor when enabled",
			body:    "[Bad](#missing-heading)\n",
			anchors: "check",
			want:    "anchor \"#missing-heading\" does not exist",
		},
		{
			name:    "present anchor when enabled",
			body:    "[Good](#home)\n",
			anchors: "check",
		},
		{
			name:      "broken target anchor",
			body:      "[Bad](other.md#nope)\n",
			anchors:   "check",
			extraFile: map[string]string{"content/other.md": "# Other\n"},
			want:      "anchor \"#nope\" does not exist in content/other.md",
		},
		{
			name: "fenced and inline code ignored",
			body: "```md\n[Missing](missing.md)\n```\n\nUse `[Missing](missing.md)` in prose.\n",
		},
		{
			name:      "percent encoded target",
			body:      "[Space](my%20file.md)\n",
			extraFile: map[string]string{"content/my file.md": "# Space\n"},
		},
		{
			name: "query stripped",
			body: "[Query](index.md?view=1)\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			anchors := test.anchors
			if anchors == "" {
				anchors = "ignore"
			}
			writeContentManifest(t, root, `schemaVersion = 1
include = ["content/**/*.md"]
[links]
anchors = "`+anchors+`"
`)
			writeContentFile(t, root, "content/index.md", "# Home\n\n"+test.body)
			for file, body := range test.extraFile {
				writeContentFile(t, root, file, body)
			}
			cfg, diagnostics := Load(root)
			if len(diagnostics) != 0 {
				t.Fatalf("Load diagnostics = %#v", diagnostics)
			}
			diagnostics = Check(root, cfg)
			if test.want == "" {
				if len(diagnostics) != 0 {
					t.Fatalf("diagnostics = %#v, want none", diagnostics)
				}
				return
			}
			if len(diagnostics) != 1 || diagnostics[0].Rule != RuleLink {
				t.Fatalf("diagnostics = %#v, want one content-link failure", diagnostics)
			}
			if !strings.Contains(diagnostics[0].Message, test.want) {
				t.Fatalf("message = %q, want %q", diagnostics[0].Message, test.want)
			}
			if diagnostics[0].Line == 0 || diagnostics[0].File != "content/index.md" {
				t.Fatalf("diagnostic = %#v, want content/index.md with a line", diagnostics[0])
			}
		})
	}
}

func TestCheckExclusionsSkipBrokenLinks(t *testing.T) {
	root := validContentFixture(t)
	writeContentFile(t, root, "content/drafts/more.md", "[Broken](missing.md)\n")
	cfg, diagnostics := Load(root)
	if len(diagnostics) != 0 {
		t.Fatalf("Load diagnostics = %#v", diagnostics)
	}
	if diagnostics := Check(root, cfg); len(diagnostics) != 0 {
		t.Fatalf("excluded document should not be checked: %#v", diagnostics)
	}
}

func TestCheckDeclaredCheckers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("checker fixtures use POSIX shell scripts")
	}
	t.Run("diagnostic output and cwd", func(t *testing.T) {
		root := t.TempDir()
		writeContentManifest(t, root, `schemaVersion = 1
include = ["content/**/*.md"]

[[checkers]]
name = "docs-check"
run = ["sh", "-c", "test -f marker.txt || exit 2; printf 'docs/page.md:7: broken catalog entry\n'; exit 1"]
dir = "content"
`)
		writeContentFile(t, root, "content/marker.txt", "present\n")
		cfg, diagnostics := Load(root)
		if len(diagnostics) != 0 {
			t.Fatalf("Load diagnostics = %#v", diagnostics)
		}
		diagnostics = Check(root, cfg)
		if len(diagnostics) != 1 || diagnostics[0].Rule != RuleChecker {
			t.Fatalf("diagnostics = %#v, want one content-checker failure", diagnostics)
		}
		if diagnostics[0].File != "docs/page.md" || diagnostics[0].Line != 7 ||
			!strings.Contains(diagnostics[0].Message, "broken catalog entry") {
			t.Fatalf("checker diagnostic = %#v, want docs/page.md:7", diagnostics[0])
		}
	})
	t.Run("passing checker ignores output", func(t *testing.T) {
		root := t.TempDir()
		writeContentManifest(t, root, `schemaVersion = 1
include = ["content/**/*.md"]

[[checkers]]
name = "docs-check"
run = ["sh", "-c", "printf 'informational only\n'; exit 0"]
`)
		cfg, diagnostics := Load(root)
		if len(diagnostics) != 0 {
			t.Fatalf("Load diagnostics = %#v", diagnostics)
		}
		if diagnostics := Check(root, cfg); len(diagnostics) != 0 {
			t.Fatalf("exit 0 must pass: %#v", diagnostics)
		}
	})
	t.Run("missing command fails", func(t *testing.T) {
		root := t.TempDir()
		writeContentManifest(t, root, `schemaVersion = 1
include = ["content/**/*.md"]

[[checkers]]
name = "missing-checker"
run = ["definitely-not-a-real-command-andurel"]
`)
		cfg, diagnostics := Load(root)
		if len(diagnostics) != 0 {
			t.Fatalf("Load diagnostics = %#v", diagnostics)
		}
		diagnostics = Check(root, cfg)
		if len(diagnostics) != 1 || diagnostics[0].Rule != RuleChecker {
			t.Fatalf("diagnostics = %#v, want a content-checker failure", diagnostics)
		}
	})
	t.Run("timeout fails", func(t *testing.T) {
		root := t.TempDir()
		writeContentManifest(t, root, `schemaVersion = 1
include = ["content/**/*.md"]

[[checkers]]
name = "slow-checker"
run = ["sh", "-c", "exec sleep 5"]
timeoutSeconds = 1
`)
		cfg, diagnostics := Load(root)
		if len(diagnostics) != 0 {
			t.Fatalf("Load diagnostics = %#v", diagnostics)
		}
		diagnostics = Check(root, cfg)
		if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "timed out") {
			t.Fatalf("diagnostics = %#v, want a timeout failure", diagnostics)
		}
	})
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"content/**/*.md", "content/index.md", true},
		{"content/**/*.md", "content/docs/deep/page.md", true},
		{"content/**/*.md", "content/index.mdx", false},
		{"content/*.md", "content/docs/page.md", false},
		{"content/*.md", "content/page.md", true},
		{"**/*.md", "page.md", true},
		{"content/**", "content/anything/deep.txt", true},
	}
	for _, test := range tests {
		if got := matchGlob(test.pattern, test.name); got != test.want {
			t.Fatalf("matchGlob(%q, %q) = %v, want %v", test.pattern, test.name, got, test.want)
		}
	}
}
