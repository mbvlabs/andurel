package layout

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestGeneratedMailSupportsSMTPAndRejectsMailpitInProduction(t *testing.T) {
	assertTemplateContains(t, "config_email.tmpl",
		"SMTPDriver",
		"LogDriver",
		"DefaultSMTPPort",
		"DefaultSMTPTLS",
		`e.String("SMTP_HOST", "")`,
		`e.String("SMTP_PORT", DefaultSMTPPort)`,
		`e.String("SMTP_TLS", DefaultSMTPTLS)`,
	)

	assertTemplateContains(t, "runtime_runtime.tmpl",
		"func validateEmailConfig(cfg config.Mail, app config.App) error",
		"if app.IsProduction() {",
		"set EMAIL_PROVIDER=%s",
		"case config.SMTPDriver, config.LogDriver:",
		"email.NewLog(slog.Default())",
	)
}

func TestScaffoldEmailTemplatesRenderWithTextTemplate(t *testing.T) {
	root := renderScaffoldForEmailTest(t, "react")

	for _, name := range []string{"verify_email", "reset_password"} {
		for _, ext := range []string{"html", "txt"} {
			path := filepath.Join(root, "email", "templates", name+"."+ext+".tmpl")
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("scaffold did not write %s: %v", path, err)
			}
		}
	}

	verifySource, err := os.ReadFile(filepath.Join(root, "email", "verify_email.go"))
	if err != nil {
		t.Fatalf("read email/verify_email.go: %v", err)
	}
	for _, want := range []string{
		`renderHTML("verify_email", v)`,
		`renderText("verify_email", v)`,
		`var _ pkgemail.Transformer = (*VerifyEmail)(nil)`,
	} {
		if !strings.Contains(string(verifySource), want) {
			t.Errorf("email/verify_email.go missing %q", want)
		}
	}
	renderSource, err := os.ReadFile(filepath.Join(root, "email", "render.go"))
	if err != nil {
		t.Fatalf("read email/render.go: %v", err)
	}
	for _, want := range []string{
		`"text/template"`,
		`//go:embed templates/*.tmpl`,
	} {
		if !strings.Contains(string(renderSource), want) {
			t.Errorf("email/render.go missing %q", want)
		}
	}
	if strings.Contains(string(renderSource), `"html/template"`) {
		t.Error("email/render.go must use text/template so MSO comments survive")
	}

	verifyHTML := executeEmailTemplateForTest(
		t,
		filepath.Join(root, "email", "templates", "verify_email.html.tmpl"),
		struct{ VerificationCode string }{VerificationCode: "482917"},
	)
	if !strings.Contains(verifyHTML, "482917") {
		t.Fatal("verify_email.html.tmpl did not substitute the code")
	}
	verifyText := executeEmailTemplateForTest(
		t,
		filepath.Join(root, "email", "templates", "verify_email.txt.tmpl"),
		struct{ VerificationCode string }{VerificationCode: "482917"},
	)
	if !strings.Contains(verifyText, "482917") {
		t.Fatalf("verify_email.txt.tmpl did not substitute the code:\n%s", verifyText)
	}

	resetURL := `https://x.test/r?a=1&b=<2>"x`
	resetHTML := executeEmailTemplateForTest(
		t,
		filepath.Join(root, "email", "templates", "reset_password.html.tmpl"),
		struct{ ResetURL string }{ResetURL: resetURL},
	)
	for _, want := range []string{
		`href="https://x.test/r?a=1&amp;b=&lt;2&gt;&#34;x"`,
		"<!--[if mso]>",
		"@media (prefers-color-scheme: dark)",
	} {
		if !strings.Contains(resetHTML, want) {
			t.Errorf("reset_password.html.tmpl missing %q", want)
		}
	}
	if strings.Contains(resetHTML, `a=1&b=`) {
		t.Error("reset_password.html.tmpl did not HTML-escape the reset URL")
	}

	resetText := executeEmailTemplateForTest(
		t,
		filepath.Join(root, "email", "templates", "reset_password.txt.tmpl"),
		struct{ ResetURL string }{ResetURL: resetURL},
	)
	if !strings.Contains(resetText, resetURL) {
		t.Fatalf("reset_password.txt.tmpl did not keep the raw URL:\n%s", resetText)
	}
	if strings.Contains(resetText, "&amp;") {
		t.Error("reset_password.txt.tmpl HTML-escaped the plain-text URL")
	}

	for _, path := range emailTemplateFilesForTest(t, root) {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, forbidden := range []string{"padding-inline", "padding-block", "oklch", "var("} {
			if strings.Contains(string(content), forbidden) {
				t.Errorf("%s contains unsupported CSS %q", path, forbidden)
			}
		}
	}
}

func TestScaffoldWritesReactEmailSources(t *testing.T) {
	for _, adapter := range []string{"react", "vue", "svelte"} {
		t.Run(adapter, func(t *testing.T) {
			root := renderScaffoldForEmailTest(t, adapter)
			for _, path := range []string{
				"emails/theme.ts",
				"emails/_components/layout.tsx",
				"emails/_components/code-box.tsx",
				"emails/verify-email.tsx",
				"emails/reset-password.tsx",
				"emails/_export.tsx",
				"emails/_generated/props.ts",
				"emails/_generated/manifest.ts",
				"emails/tsconfig.json",
			} {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
					t.Fatalf("scaffold did not write %s: %v", path, err)
				}
			}
			packageJSON, err := os.ReadFile(filepath.Join(root, "package.json"))
			if err != nil {
				t.Fatalf("read package.json: %v", err)
			}
			for _, want := range []string{
				`"email:export": "tsx --tsconfig emails/tsconfig.json emails/_export.tsx"`,
				`"react-email": "6.11.6"`,
			} {
				if !strings.Contains(string(packageJSON), want) {
					t.Errorf("package.json missing %q:\n%s", want, packageJSON)
				}
			}
			for _, unwanted := range []string{`"email` + `:dev"`, `"@react-email` + `/ui"`} {
				if strings.Contains(string(packageJSON), unwanted) {
					t.Errorf("package.json should not contain %q:\n%s", unwanted, packageJSON)
				}
			}
		})
	}
}

func renderScaffoldForEmailTest(t *testing.T, adapter string) string {
	t.Helper()
	root := t.TempDir()
	data := &TemplateData{
		AppName:              "testapp",
		ProjectName:          "testapp",
		ModuleName:           "testapp",
		Database:             "postgresql",
		GoVersion:            goVersion,
		SessionKey:           "test-session-key",
		SessionEncryptionKey: "test-session-encryption-key",
		TokenSigningKey:      "test-token-signing-key",
		Pepper:               "test-pepper",
		FrameworkVersion:     "v2.0.0-alpha.5",
		Inertia:              adapter,
		JavaScriptSSRRuntime: "node",
		blueprint:            initializeBlueprint("testapp"),
	}
	if err := processTemplatedFiles(root, data); err != nil {
		t.Fatalf("render %s scaffold: %v", adapter, err)
	}
	return root
}

func executeEmailTemplateForTest(t *testing.T, path string, data any) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	tmpl, err := template.New(filepath.Base(path)).Parse(string(content))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("execute %s: %v", path, err)
	}
	return buf.String()
}

func emailTemplateFilesForTest(t *testing.T, root string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, "email", "templates", "*.tmpl"))
	if err != nil {
		t.Fatalf("glob email templates: %v", err)
	}
	if len(paths) != 4 {
		t.Fatalf("email templates = %d, want 4", len(paths))
	}
	return paths
}
