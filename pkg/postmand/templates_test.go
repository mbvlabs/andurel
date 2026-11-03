package postmand

import (
	"strings"
	"testing"
	"testing/fstest"
)

func testTemplatesFS() fstest.MapFS {
	return fstest.MapFS{
		"templates/verify_email.html.tmpl": &fstest.MapFile{
			Data: []byte(
				`<!--[if mso]><table><tr><td><![endif]-->` +
					`<a href="{{.URL | html}}">{{.Name | html}}</a>` +
					`<!--[if mso]></td></tr></table><![endif]-->`,
			),
		},
		"templates/verify_email.txt.tmpl": &fstest.MapFile{
			Data: []byte("Verify {{.Name}}: {{.URL}}"),
		},
		"templates/reset_password.html.tmpl": &fstest.MapFile{
			Data: []byte(`<p>{{.ResetURL | html}}</p>`),
		},
		"templates/reset_password.txt.tmpl": &fstest.MapFile{
			Data: []byte("Reset: {{.ResetURL}}"),
		},
	}
}

func TestTemplatesHTMLExplicitlyEscapesPlaceholders(t *testing.T) {
	templates := MustParseTemplates(testTemplatesFS(), "templates")

	html, err := templates.HTML("verify_email", struct {
		URL  string
		Name string
	}{
		URL:  `https://x.test/r?a=1&b=<2>"x`,
		Name: `A & B <admin>`,
	})
	if err != nil {
		t.Fatalf("HTML() = %v, want nil error", err)
	}

	for _, want := range []string{
		`href="https://x.test/r?a=1&amp;b=&lt;2&gt;&#34;x"`,
		`A &amp; B &lt;admin&gt;`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML() missing %q:\n%s", want, html)
		}
	}
	if strings.Contains(html, `a=1&b=`) {
		t.Errorf("HTML() did not escape the URL:\n%s", html)
	}
}

func TestTemplatesHTMLKeepsOutlookConditionalComments(t *testing.T) {
	templates := MustParseTemplates(testTemplatesFS(), "templates")

	html, err := templates.HTML("verify_email", map[string]any{
		"URL":  "https://example.test",
		"Name": "Person",
	})
	if err != nil {
		t.Fatalf("HTML() = %v, want nil error", err)
	}

	if !strings.Contains(html, "<!--[if mso]><table><tr><td><![endif]-->") {
		t.Fatalf("HTML() stripped the Outlook conditional comment:\n%s", html)
	}
	if !strings.Contains(html, "<!--[if mso]></td></tr></table><![endif]-->") {
		t.Fatalf("HTML() stripped the closing Outlook conditional comment:\n%s", html)
	}
}

func TestTemplatesTextDoesNotEscapePlaceholders(t *testing.T) {
	templates := MustParseTemplates(testTemplatesFS(), "templates")

	url := `https://x.test/r?a=1&b=<2>"x`
	text, err := templates.Text("verify_email", struct {
		URL  string
		Name string
	}{
		URL:  url,
		Name: "Person",
	})
	if err != nil {
		t.Fatalf("Text() = %v, want nil error", err)
	}

	if text != "Verify Person: "+url {
		t.Fatalf("Text() = %q, want the raw URL", text)
	}
	if strings.Contains(text, "&amp;") {
		t.Fatalf("Text() HTML-escaped the plain-text body:\n%s", text)
	}
}

func TestTemplatesUnknownNameListsAvailableTemplates(t *testing.T) {
	templates := MustParseTemplates(testTemplatesFS(), "templates")

	_, err := templates.HTML("missing", nil)
	if err == nil {
		t.Fatal("HTML() = nil error, want unknown template error")
	}
	for _, want := range []string{
		`unknown HTML template "missing"`,
		"available templates: reset_password, verify_email",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("HTML() error = %q, want %q", err, want)
		}
	}

	_, err = templates.Text("missing", nil)
	if err == nil {
		t.Fatal("Text() = nil error, want unknown template error")
	}
	if !strings.Contains(err.Error(), `unknown text template "missing"`) {
		t.Errorf("Text() error = %q, want unknown text template", err)
	}
}

func TestParseTemplatesRequiresMatchingPairs(t *testing.T) {
	tests := []struct {
		name  string
		files fstest.MapFS
		wants []string
	}{
		{
			name: "HTML without text",
			files: fstest.MapFS{
				"templates/welcome_email.html.tmpl": &fstest.MapFile{
					Data: []byte("<p>Welcome</p>"),
				},
				"templates/reset_password.html.tmpl": &fstest.MapFile{
					Data: []byte("<p>Reset</p>"),
				},
				"templates/reset_password.txt.tmpl": &fstest.MapFile{
					Data: []byte("Reset"),
				},
			},
			wants: []string{
				"welcome_email has HTML but no text template (welcome_email.txt.tmpl)",
			},
		},
		{
			name: "text without HTML",
			files: fstest.MapFS{
				"templates/welcome_email.html.tmpl": &fstest.MapFile{
					Data: []byte("<p>Welcome</p>"),
				},
				"templates/welcome_email.txt.tmpl": &fstest.MapFile{
					Data: []byte("Welcome"),
				},
				"templates/reset_password.txt.tmpl": &fstest.MapFile{
					Data: []byte("Reset"),
				},
			},
			wants: []string{
				"reset_password has text but no HTML template (reset_password.html.tmpl)",
			},
		},
		{
			name: "fully mismatched",
			files: fstest.MapFS{
				"templates/welcome_email.html.tmpl": &fstest.MapFile{
					Data: []byte("<p>Welcome</p>"),
				},
				"templates/reset_password.txt.tmpl": &fstest.MapFile{
					Data: []byte("Reset"),
				},
			},
			wants: []string{
				"welcome_email has HTML but no text template (welcome_email.txt.tmpl)",
				"reset_password has text but no HTML template (reset_password.html.tmpl)",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseTemplates(test.files, "templates")
			if err == nil {
				t.Fatal("ParseTemplates() = nil error, want unmatched-pair error")
			}
			if !strings.Contains(err.Error(), "template pairs mismatch") {
				t.Fatalf("ParseTemplates() error = %q, want a template-pair error", err)
			}
			for _, want := range test.wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("ParseTemplates() error = %q, want %q", err, want)
				}
			}
		})
	}
}

func TestTemplatesMissingKeyIsAnError(t *testing.T) {
	templates := MustParseTemplates(fstest.MapFS{
		"templates/missing_key.html.tmpl": &fstest.MapFile{
			Data: []byte(`{{.Missing}}`),
		},
		"templates/missing_key.txt.tmpl": &fstest.MapFile{
			Data: []byte(`{{.Missing}}`),
		},
	}, "templates")

	for name, render := range map[string]func() (string, error){
		"HTML": func() (string, error) {
			return templates.HTML("missing_key", map[string]string{})
		},
		"Text": func() (string, error) {
			return templates.Text("missing_key", map[string]string{})
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := render()
			if err == nil {
				t.Fatal("render = nil error, want missing key error")
			}
			if !strings.Contains(err.Error(), "Missing") {
				t.Fatalf("render error = %q, want the missing key name", err)
			}
		})
	}
}

func TestParseTemplatesEmptyDirectoryIsAnError(t *testing.T) {
	_, err := ParseTemplates(fstest.MapFS{}, "templates")
	if err == nil {
		t.Fatal("ParseTemplates() = nil error, want empty directory error")
	}
	if !strings.Contains(err.Error(), "no HTML templates") {
		t.Fatalf("ParseTemplates() error = %q, want a missing-template error", err)
	}
}

func TestMustParseTemplatesPanicsOnError(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustParseTemplates() did not panic")
		}
	}()

	MustParseTemplates(fstest.MapFS{}, "templates")
}
