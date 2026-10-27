package layout

import (
	"strings"
	"testing"
)

// TestThemeHasOneColorOwner keeps the color-token mappings in css/theme.css.
// css/base.css owns the Tailwind entry, fonts, and larger radii; a second @theme
// block with --color-* mappings silently overrides the first.
func TestThemeHasOneColorOwner(t *testing.T) {
	base := readLayoutTemplate(t, "css_base.tmpl")
	theme := readLayoutTemplate(t, "css_theme.tmpl")

	for _, want := range []string{
		"@apply font-sans;",
		"--font-mono:",
		"--radius-xl:",
	} {
		if !strings.Contains(base, want) {
			t.Errorf("css_base.tmpl missing %q", want)
		}
	}
	if strings.Contains(base, "--color-") {
		t.Error("css_base.tmpl must not map color tokens; css/theme.css owns them")
	}
	if strings.Contains(base, "@apply font-mono") {
		t.Error("css_base.tmpl must apply font-sans to html, not font-mono")
	}

	if count := strings.Count(theme, "@theme inline {"); count != 1 {
		t.Errorf("css_theme.tmpl has %d @theme inline blocks, want 1", count)
	}
	for _, want := range []string{
		"--color-background: var(--background);",
		"--color-destructive-foreground: var(--destructive-foreground);",
		"--color-sidebar-ring: var(--sidebar-ring);",
		"--color-chart-5: var(--chart-5);",
		"var(--background)",
		// css/email.css imports theme.css; email templates use these radii.
		"--radius-md:",
		"--radius-lg:",
	} {
		if !strings.Contains(theme, want) {
			t.Errorf("css_theme.tmpl missing %q", want)
		}
	}
}
