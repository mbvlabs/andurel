package layout

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	layouttemplates "github.com/mbvlabs/andurel/v2/layout/templates"
)

const fakeEmailPreviews = `package email

type Transformer interface {
	ToHTML() (string, error)
	ToText() (string, error)
}

type Preview struct {
	Name    string
	Subject string
	Email   Transformer
}

type fixture struct {
	html string
	text string
}

func (f fixture) ToHTML() (string, error) { return f.html, nil }
func (f fixture) ToText() (string, error) { return f.text, nil }

func Previews() []Preview {
	return []Preview{
		{
			Name:    "verify_email",
			Subject: "Verify your email",
			Email: fixture{
				html: "<p>Hello & welcome, \"friend\".</p><style>@media (prefers-color-scheme: dark) { .email-bg { background: #000; } }</style>",
				text: "Hello & welcome, \"friend\".",
			},
		},
		{
			Name:    "reset_password",
			Subject: "Reset your password",
			Email: fixture{
				html: "<a href=\"https://example.test/reset\">Reset</a>",
				text: "Reset https://example.test/reset",
			},
		},
	}
}
`

const fakeBrokenEmailPreviews = `package email

import "errors"

type Transformer interface {
	ToHTML() (string, error)
	ToText() (string, error)
}

type Preview struct {
	Name    string
	Subject string
	Email   Transformer
}

type broken struct{}

func (broken) ToHTML() (string, error) { return "", errors.New("template exploded") }
func (broken) ToText() (string, error) { return "", errors.New("template exploded") }

func Previews() []Preview {
	return []Preview{{Name: "broken_email", Subject: "Broken", Email: broken{}}}
}
`

func TestEmailPreviewTemplatesAreRegistered(t *testing.T) {
	for templateName, want := range map[TmplTarget]TmplTargetPath{
		"cmd_emails_main.tmpl": "cmd/emails/main.go",
		"email_previews.tmpl":  "email/previews.go",
	} {
		if got := baseTemplateMappings[templateName]; got != want {
			t.Fatalf("%s maps to %q, want %q", templateName, got, want)
		}
	}
}

func TestEmailPreviewsTemplateListsFixtureEmails(t *testing.T) {
	assertTemplateContains(t, "email_previews.tmpl",
		"type Preview struct",
		"Email   pkgemail.Transformer",
		"// Add one entry per email",
		`Name:    "verify_email"`,
		`Subject: "Verify your email"`,
		`VerifyEmail{VerificationCode: "482917"}`,
		`Name:    "reset_password"`,
		`Subject: "Reset your password"`,
		`ResetPassword{ResetURL: "https://example.test/reset?token=abc123"}`,
	)
}

func TestGeneratedEmailPreviewsCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generated email previews check in short mode")
	}

	root := t.TempDir()
	data := &TemplateData{ModuleName: "example.com/app", ProjectName: "app"}
	if err := renderTemplate(
		root,
		"email_previews.tmpl",
		"email/previews.go",
		layouttemplates.Files,
		data,
	); err != nil {
		t.Fatalf("render email/previews template: %v", err)
	}

	writeGeneratedFile(t, root, "email/types.go", `package email

type VerifyEmail struct {
	VerificationCode string
}

func (VerifyEmail) ToHTML() (string, error) { return "", nil }
func (VerifyEmail) ToText() (string, error) { return "", nil }

type ResetPassword struct {
	ResetURL string
}

func (ResetPassword) ToHTML() (string, error) { return "", nil }
func (ResetPassword) ToText() (string, error) { return "", nil }
`)
	writeGeneratedFile(t, root, "pkgemail/go.mod", "module github.com/mbvlabs/andurel/pkg/email\n\ngo "+goVersion+"\n")
	writeGeneratedFile(t, root, "pkgemail/email.go", `package email

type Transformer interface {
	ToHTML() (string, error)
	ToText() (string, error)
}
`)
	writeGeneratedFile(t, root, "go.mod", `module example.com/app

go `+goVersion+`

require github.com/mbvlabs/andurel/pkg/email v0.0.0

replace github.com/mbvlabs/andurel/pkg/email => ./pkgemail
`)

	cmd := exec.Command("go", "vet", "./email")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go vet generated email previews: %v\n%s", err, output)
	}
}

func TestGeneratedEmailsCommandRendersGallery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generated cmd/emails check in short mode")
	}

	root := t.TempDir()
	data := &TemplateData{ModuleName: "example.com/app", ProjectName: "app"}
	if err := renderTemplate(
		root,
		"cmd_emails_main.tmpl",
		"cmd/emails/main.go",
		layouttemplates.Files,
		data,
	); err != nil {
		t.Fatalf("render cmd/emails template: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module example.com/app\n\ngo "+goVersion+"\n"),
		0o644,
	); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	writePreviewFixtures(t, root, fakeEmailPreviews)

	stdout, stderr, err := runGeneratedEmails(t, root, "-out", "tmp/emails", "-json")
	if err != nil {
		t.Fatalf("go run ./cmd/emails: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	var report struct {
		Emails []struct {
			Name    string `json:"name"`
			Subject string `json:"subject"`
			HTML    string `json:"html"`
			Text    string `json:"text"`
			Bytes   int    `json:"bytes"`
		} `json:"emails"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode email report: %v\nstdout:\n%s", err, stdout)
	}
	if len(report.Emails) != 2 {
		t.Fatalf("rendered emails = %#v", report.Emails)
	}
	if report.Emails[0].Name != "verify_email" ||
		report.Emails[0].Subject != "Verify your email" {
		t.Fatalf("first email = %#v", report.Emails[0])
	}
	if report.Emails[0].HTML != filepath.Join("tmp", "emails", "verify_email.html") ||
		report.Emails[0].Text != filepath.Join("tmp", "emails", "verify_email.txt") {
		t.Fatalf("first email paths = %#v", report.Emails[0])
	}
	if report.Emails[0].Bytes == 0 {
		t.Fatalf("first email bytes = 0")
	}
	for _, path := range []string{report.Emails[0].HTML, report.Emails[0].Text} {
		content, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if len(content) == 0 {
			t.Fatalf("%s is empty", path)
		}
	}

	index, err := os.ReadFile(filepath.Join(root, "tmp", "emails", "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	indexHTML := string(index)
	for _, want := range []string{
		"verify_email",
		"Verify your email",
		"Desktop 600",
		"Mobile 375",
		"Dark 600",
		`class="preview dark"`,
		`<pre class="text-body">Hello &amp; welcome, &#34;friend&#34;.</pre>`,
		`srcdoc="&lt;p&gt;Hello &amp; welcome, &#34;friend&#34;.&lt;/p&gt;`,
		`href="verify_email.html"`,
		`href="verify_email.txt"`,
		"@media all",
	} {
		if !strings.Contains(indexHTML, want) {
			t.Fatalf("index.html missing %q:\n%s", want, indexHTML)
		}
	}
	if strings.Contains(indexHTML, `srcdoc="<p>`) {
		t.Fatalf("index.html did not escape srcdoc:\n%s", indexHTML)
	}
	if got := strings.Count(indexHTML, "@media (prefers-color-scheme: dark)"); got != 2 {
		t.Fatalf(
			"index.html kept %d dark media queries, want only the desktop and mobile panes:\n%s",
			got,
			indexHTML,
		)
	}
	paneOrder := []string{"Desktop 600", "Mobile 375", "Dark 600", "Text"}
	last := -1
	for _, pane := range paneOrder {
		at := strings.Index(indexHTML, pane)
		if at <= last {
			t.Fatalf("pane %q is out of order in index.html:\n%s", pane, indexHTML)
		}
		last = at
	}
	if strings.Contains(indexHTML, "version.js") {
		t.Fatalf("non-watch index.html must stay static:\n%s", indexHTML)
	}
	if _, err := os.Stat(filepath.Join(root, "tmp", "emails", "version.js")); !os.IsNotExist(err) {
		t.Fatalf("non-watch render wrote version.js: %v", err)
	}

	watchOut := filepath.Join("tmp", "emails-watch")
	if _, watchStderr, err := runGeneratedEmails(t, root, "-out", watchOut, "-watch", "-json"); err != nil {
		t.Fatalf("go run ./cmd/emails -watch: %v\nstderr:\n%s", err, watchStderr)
	}
	firstVersion, err := os.ReadFile(filepath.Join(root, watchOut, "version.js"))
	if err != nil {
		t.Fatalf("read watch version.js: %v", err)
	}
	watchIndex, err := os.ReadFile(filepath.Join(root, watchOut, "index.html"))
	if err != nil {
		t.Fatalf("read watch index.html: %v", err)
	}
	for _, want := range []string{
		`<script src="version.js"></script>`,
		"version.js?t=",
		"@media all",
		"Dark 600",
	} {
		if !strings.Contains(string(watchIndex), want) {
			t.Fatalf("watch index.html missing %q:\n%s", want, watchIndex)
		}
	}
	if _, watchStderr, err := runGeneratedEmails(t, root, "-out", watchOut, "-watch", "-json"); err != nil {
		t.Fatalf("second go run ./cmd/emails -watch: %v\nstderr:\n%s", err, watchStderr)
	}
	secondVersion, err := os.ReadFile(filepath.Join(root, watchOut, "version.js"))
	if err != nil {
		t.Fatalf("read second watch version.js: %v", err)
	}
	if string(firstVersion) == string(secondVersion) {
		t.Fatalf("version.js did not change between watch renders: %s", firstVersion)
	}

	// A later one-shot render in the same directory must drop watch-only
	// files and generate a fully static gallery again.
	if _, stderr, err := runGeneratedEmails(t, root, "-out", watchOut, "-json"); err != nil {
		t.Fatalf("go run ./cmd/emails after watch: %v\nstderr:\n%s", err, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, watchOut, "version.js")); !os.IsNotExist(err) {
		t.Fatalf("one-shot render left version.js behind: %v", err)
	}
	plainIndex, err := os.ReadFile(filepath.Join(root, watchOut, "index.html"))
	if err != nil {
		t.Fatalf("read one-shot index.html: %v", err)
	}
	if strings.Contains(string(plainIndex), "version.js") {
		t.Fatalf("one-shot index.html still polls version.js:\n%s", plainIndex)
	}

	onlyStdout, onlyStderr, err := runGeneratedEmails(t, root, "-out", "tmp/emails", "-only", "verify_email", "-json")
	if err != nil {
		t.Fatalf("go run ./cmd/emails -only: %v\nstdout:\n%s\nstderr:\n%s", err, onlyStdout, onlyStderr)
	}
	var onlyReport struct {
		Emails []struct {
			Name string `json:"name"`
		} `json:"emails"`
	}
	if err := json.Unmarshal([]byte(onlyStdout), &onlyReport); err != nil {
		t.Fatalf("decode only report: %v\nstdout:\n%s", err, onlyStdout)
	}
	if len(onlyReport.Emails) != 1 || onlyReport.Emails[0].Name != "verify_email" {
		t.Fatalf("only report = %#v", onlyReport)
	}

	_, unknownStderr, err := runGeneratedEmails(t, root, "-out", "tmp/emails", "-only", "missing_email")
	if err == nil {
		t.Fatal("expected unknown email to fail")
	}
	for _, want := range []string{"unknown email", "verify_email", "reset_password"} {
		if !strings.Contains(unknownStderr, want) {
			t.Fatalf("unknown email stderr missing %q:\n%s", want, unknownStderr)
		}
	}

	writePreviewFixtures(t, root, fakeBrokenEmailPreviews)
	_, brokenStderr, err := runGeneratedEmails(t, root, "-out", "tmp/emails")
	if err == nil {
		t.Fatal("expected broken email render to fail")
	}
	for _, want := range []string{"render email broken_email", "template exploded"} {
		if !strings.Contains(brokenStderr, want) {
			t.Fatalf("broken email stderr missing %q:\n%s", want, brokenStderr)
		}
	}
}

func writePreviewFixtures(t *testing.T, root, content string) {
	t.Helper()
	writeGeneratedFile(t, root, "email/previews.go", content)
}

func writeGeneratedFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func runGeneratedEmails(t *testing.T, root string, args ...string) (string, string, error) {
	t.Helper()

	cmdArgs := append([]string{"run", "./cmd/emails"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
