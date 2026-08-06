package emailcompiler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCompileEndToEnd(t *testing.T) {
	root := t.TempDir()
	templateSource := `package email

templ message() {
	<html>
		<head><title>Test</title></head>
		<body class="bg-red unknown-class">
			<table class="w-full"><tr><td class="text-center max-sm:px-4 hover:text-blue" style="font-weight: 700">Hello</td></tr></table>
		</body>
	</html>
}
`
	writeTestFile(t, filepath.Join(root, "email", "message.templ"), templateSource, 0o644)
	writeTestFile(t, filepath.Join(root, "css", "email.css"), `@import "tailwindcss";
/* andurel:head:start */
body { margin: 0 !important; }
/* andurel:head:end */
`, 0o644)
	capturedInput := filepath.Join(root, "tailwind-input.css")
	tailwind := writeFakeTailwind(t, root, `.bg-red { background-color: #ff0000; }
.w-full { width: 600px; }
.text-center { text-align: center; }
.px-4 { padding-left: 1rem; padding-right: 1rem; }
.text-blue { color: #0000ff; }
`, capturedInput, "")
	target := filepath.Join(root, "email", "message_templ.go")
	writeTestFile(t, target, "stale", 0o600)

	err := Compile(context.Background(), Config{
		ProjectRoot:  root,
		EmailDir:     "email",
		CSSInputPath: "css/email.css",
		TailwindPath: tailwind,
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	generated := strings.ReplaceAll(readTestFile(t, target), `\"`, `"`)
	for _, expected := range []string{
		`style="background-color: #ff0000;"`,
		`class="unknown-class"`,
		`style="width: 600px;"`,
		`width="600"`,
		`class="andurel-email-max-sm-px-4 andurel-email-hover-text-blue"`,
		`style="text-align: center; font-weight: 700;"`,
		`align="center"`,
		`body { margin: 0 !important; }`,
		`@media screen and (max-width: 639px)`,
		`.andurel-email-max-sm-px-4 { padding-left: 16px !important; padding-right: 16px !important; }`,
		`.andurel-email-hover-text-blue:hover { color: #0000ff !important; }`,
	} {
		if !strings.Contains(generated, expected) {
			t.Errorf("generated output does not contain %q\n%s", expected, generated)
		}
	}
	if got := readTestFile(t, filepath.Join(root, "email", "message.templ")); got != templateSource {
		t.Errorf("authored template changed:\n%s", got)
	}
	input := readTestFile(t, capturedInput)
	if !strings.Contains(input, `@source inline("bg-red px-4 text-blue text-center unknown-class w-full");`) {
		t.Errorf("Tailwind input did not contain sorted unique candidates:\n%s", input)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("generated mode = %o, want 644", got)
	}
	assertNoCompilerTemporaryFiles(t, filepath.Join(root, "css"))
	assertNoCompilerTemporaryFiles(t, filepath.Join(root, "email"))
}

func TestCompileWithoutClasses(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "email", "plain.templ"), `package email
templ plain() { <p>Hello</p> }
`, 0o644)
	writeTestFile(t, filepath.Join(root, "css", "email.css"), "@import \"tailwindcss\";\n", 0o644)
	capturedInput := filepath.Join(root, "tailwind-input.css")
	tailwind := writeFakeTailwind(t, root, "", capturedInput, "")

	if err := Compile(context.Background(), Config{ProjectRoot: root, TailwindPath: tailwind}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if generated := strings.ReplaceAll(readTestFile(t, filepath.Join(root, "email", "plain_templ.go")), `\"`, `"`); !strings.Contains(generated, "<p>Hello</p>") {
		t.Errorf("generated output does not contain plain email markup:\n%s", generated)
	}
	if input := readTestFile(t, capturedInput); strings.Contains(input, "@source inline") {
		t.Errorf("class-free template unexpectedly added Tailwind candidates:\n%s", input)
	}
}

func TestResolveConfig(t *testing.T) {
	t.Run("requires project root", func(t *testing.T) {
		_, err := resolveConfig(Config{})
		if err == nil || !strings.Contains(err.Error(), "project root is required") {
			t.Fatalf("resolveConfig() error = %v", err)
		}
	})

	t.Run("resolves defaults and relative paths", func(t *testing.T) {
		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, "email", ".keep"), "", 0o644)
		writeTestFile(t, filepath.Join(root, "css", "email.css"), "", 0o644)
		writeTestFile(t, filepath.Join(root, "bin", "tailwindcli"), "", 0o755)

		got, err := resolveConfig(Config{ProjectRoot: root})
		if err != nil {
			t.Fatalf("resolveConfig() error = %v", err)
		}
		if got.ProjectRoot != root || got.EmailDir != filepath.Join(root, "email") ||
			got.CSSInputPath != filepath.Join(root, "css", "email.css") ||
			got.TailwindPath != filepath.Join(root, "bin", "tailwindcli") {
			t.Fatalf("resolveConfig() = %#v", got)
		}

		got, err = resolveConfig(Config{
			ProjectRoot:  root,
			EmailDir:     "email",
			CSSInputPath: "css/email.css",
			TailwindPath: "bin/tailwindcli",
		})
		if err != nil {
			t.Fatalf("resolveConfig(relative) error = %v", err)
		}
		if !filepath.IsAbs(got.EmailDir) || !filepath.IsAbs(got.CSSInputPath) || !filepath.IsAbs(got.TailwindPath) {
			t.Fatalf("relative paths were not made absolute: %#v", got)
		}
	})

	t.Run("reports each missing input", func(t *testing.T) {
		cases := []struct {
			name    string
			missing string
			want    string
		}{
			{name: "email directory", missing: "email", want: "email directory not found"},
			{name: "CSS input", missing: "css", want: "email CSS input not found"},
			{name: "Tailwind CLI", missing: "tailwind", want: "Tailwind CLI not found"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				root := t.TempDir()
				if tc.missing != "email" {
					writeTestFile(t, filepath.Join(root, "email", ".keep"), "", 0o644)
				}
				if tc.missing != "css" {
					writeTestFile(t, filepath.Join(root, "css", "email.css"), "", 0o644)
				}
				if tc.missing != "tailwind" {
					writeTestFile(t, filepath.Join(root, "bin", "tailwindcli"), "", 0o755)
				}
				_, err := resolveConfig(Config{ProjectRoot: root})
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("resolveConfig() error = %v, want containing %q", err, tc.want)
				}
			})
		}
	})
}

func TestParseEmailTemplatesAndCollectClassUses(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "email", "z.templ"), `package email
templ z() { <div class="p-4 text-red p-4"></div> }
`, 0o644)
	writeTestFile(t, filepath.Join(root, "email", "nested", "a.templ"), `package email
templ a() { <span CLASS="font-bold"></span> }
`, 0o644)
	writeTestFile(t, filepath.Join(root, "email", "ignored.txt"), "not a template", 0o644)

	files, err := parseEmailTemplates(root, filepath.Join(root, "email"))
	if err != nil {
		t.Fatalf("parseEmailTemplates() error = %v", err)
	}
	gotFiles := []string{files[0].relative, files[1].relative}
	wantFiles := []string{"email/nested/a.templ", "email/z.templ"}
	if !slices.Equal(gotFiles, wantFiles) {
		t.Fatalf("relative files = %v, want %v", gotFiles, wantFiles)
	}
	uses, err := collectClassUses(files)
	if err != nil {
		t.Fatalf("collectClassUses() error = %v", err)
	}
	gotClasses := make([]string, 0, len(uses))
	for _, use := range uses {
		gotClasses = append(gotClasses, use.name)
		if use.file == "" || use.line == 0 || use.col == 0 {
			t.Errorf("class use lacks source location: %#v", use)
		}
	}
	if want := []string{"font-bold", "p-4", "text-red", "p-4"}; !slices.Equal(gotClasses, want) {
		t.Errorf("classes = %v, want %v", gotClasses, want)
	}
}

func TestCompileRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		css         string
		tailwindCSS string
		want        string
	}{
		{
			name:     "no templates",
			template: "",
			css:      "",
			want:     "no email templates found",
		},
		{
			name:     "invalid template",
			template: "package email\ntempl broken( {",
			css:      "",
			want:     "parse ",
		},
		{
			name: "dynamic class",
			template: `package email
templ message(className string) { <div class={ className }></div> }
`,
			css:  "",
			want: "dynamic class attributes are not supported",
		},
		{
			name: "conditional class",
			template: `package email
templ message(enabled bool) {
	<div
		if enabled {
			class="p-4"
		} else {
			class="p-8"
		}
	></div>
}
`,
			css:  "",
			want: "conditional class attributes are not supported",
		},
		{
			name: "invalid Tailwind class",
			template: `package email
templ message() { <div class="text-[red"></div> }
`,
			css:  "",
			want: `invalid Tailwind class "text-[red"`,
		},
		{
			name: "malformed retained CSS markers",
			template: `package email
templ message() { <html><head></head><body></body></html> }
`,
			css:  headCSSStartMarker,
			want: "email head CSS markers must be present once and in order",
		},
		{
			name: "invalid compiled CSS",
			template: `package email
templ message() { <div class="p-4"></div> }
`,
			css:         "",
			tailwindCSS: ".p-4 { padding: 1rem;",
			want:        "parse Tailwind email output",
		},
		{
			name: "retained CSS without head",
			template: `package email
templ message() { <div class="p-4"></div> }
`,
			css:         headCSSStartMarker + "\np { color: red; }\n" + headCSSEndMarker,
			tailwindCSS: ".p-4 { padding: 1rem; }",
			want:        "retained email styles require a <head> element",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.template != "" {
				writeTestFile(t, filepath.Join(root, "email", "message.templ"), tc.template, 0o644)
			} else if err := os.MkdirAll(filepath.Join(root, "email"), 0o755); err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(root, "css", "email.css"), tc.css, 0o644)
			tailwind := writeFakeTailwind(t, root, tc.tailwindCSS, "", "")
			err := Compile(context.Background(), Config{ProjectRoot: root, TailwindPath: tailwind})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Compile() error = %v, want containing %q", err, tc.want)
			}
			if _, statErr := os.Stat(filepath.Join(root, "email", "message_templ.go")); !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("generated output exists after failed compilation: %v", statErr)
			}
		})
	}
}

func TestCompileReportsTailwindFailure(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "email", "message.templ"), `package email
templ message() { <div class="p-4"></div> }
`, 0o644)
	writeTestFile(t, filepath.Join(root, "css", "email.css"), "", 0o644)
	tailwind := writeFakeTailwind(t, root, "", "", "tailwind exploded")

	err := Compile(context.Background(), Config{ProjectRoot: root, TailwindPath: tailwind})
	if err == nil || !strings.Contains(err.Error(), "compile email Tailwind CSS") || !strings.Contains(err.Error(), "tailwind exploded") {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestExtractHeadCSS(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		err   bool
	}{
		{name: "absent", input: "p { color: red; }"},
		{name: "extracts and trims", input: "before\n" + headCSSStartMarker + "\n p { color: red; } \n" + headCSSEndMarker + "\nafter", want: "p { color: red; }"},
		{name: "missing start", input: headCSSEndMarker, err: true},
		{name: "missing end", input: headCSSStartMarker, err: true},
		{name: "wrong order", input: headCSSEndMarker + headCSSStartMarker, err: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractHeadCSS(tc.input)
			if (err != nil) != tc.err {
				t.Fatalf("extractHeadCSS() error = %v, want error %v", err, tc.err)
			}
			if got != tc.want {
				t.Errorf("extractHeadCSS() = %q, want %q", got, tc.want)
			}
		})
	}
}

func writeFakeTailwind(t *testing.T, root, outputCSS, capturedInput, failure string) string {
	t.Helper()
	script := `#!/bin/sh
set -eu
input=""
output=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-i) input="$2"; shift 2 ;;
		-o) output="$2"; shift 2 ;;
		*) shift ;;
	esac
done
`
	if capturedInput != "" {
		script += "cp \"$input\" " + shellQuote(capturedInput) + "\n"
	}
	if failure != "" {
		script += "printf '%s\\n' " + shellQuote(failure) + " >&2\nexit 1\n"
	} else {
		script += "printf '%s' " + shellQuote(outputCSS) + " > \"$output\"\n"
	}
	path := filepath.Join(root, "fake-tailwind")
	writeTestFile(t, path, script, 0o755)
	return path
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func writeTestFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func assertNoCompilerTemporaryFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".andurel-email-") {
			t.Errorf("temporary compiler file was not removed: %s", filepath.Join(dir, entry.Name()))
		}
	}
}
