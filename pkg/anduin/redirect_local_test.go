package anduin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSafeLocalPath(t *testing.T) {
	cases := []struct {
		name   string
		target string
		want   bool
	}{
		{"root", "/", true},
		{"simple path", "/ok", true},
		{"nested path with query", "/users/42?tab=profile#top", true},
		{"path with encoded slash", "/a%2Fb", true},
		{"surrounding spaces", "  /ok  ", true},
		{"surrounding newline", "\n/ok\r\n", true},

		{"empty", "", false},
		{"relative path", "ok", false},
		{"dot relative", "./ok", false},
		{"scheme-relative", "//evil.example", false},
		{"scheme-relative with spaces", " //evil.example", false},
		{"absolute https", "https://evil.example", false},
		{"absolute http", "http://evil.example/", false},
		{"javascript scheme", "javascript:alert(1)", false},
		{"scheme after slash", "/\\/evil.example", false},
		{"tab splits slashes", "/\t/evil.example", false},
		{"backslash", "/\\evil.example", false},
		{"backslash pair", "\\\\evil.example", false},
		{"newline splits slashes", "/\n/evil.example", false},
		{"carriage return splits slashes", "/\r/evil.example", false},
		{"control character inside path", "/o\x00k", false},
		{"trailing control character trimmed like a browser", "/ok\x00", true},
		{"invalid escape", "/%zz", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SafeLocalPath(tc.target); got != tc.want {
				t.Fatalf("SafeLocalPath(%q) = %v, want %v", tc.target, got, tc.want)
			}
		})
	}
}

func TestRedirectLocalFollowsSafeTarget(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx := ReplaceResponseWriter(context.Background(), recorder)

	if err := RedirectLocal(ctx, "/dashboard?x=1", "/", http.StatusSeeOther); err != nil {
		t.Fatalf("RedirectLocal: %v", err)
	}
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", recorder.Code)
	}
	if got := recorder.Header().Get("Location"); got != "/dashboard?x=1" {
		t.Fatalf("Location = %q, want %q", got, "/dashboard?x=1")
	}
}

func TestRedirectLocalUsesFallbackForUnsafeTarget(t *testing.T) {
	for _, target := range []string{"", "//evil.example", "https://evil.example", "/\t/evil.example", "/\\evil.example"} {
		recorder := httptest.NewRecorder()
		ctx := ReplaceResponseWriter(context.Background(), recorder)

		if err := RedirectLocal(ctx, target, "/", http.StatusSeeOther); err != nil {
			t.Fatalf("RedirectLocal(%q): %v", target, err)
		}
		if got := recorder.Header().Get("Location"); got != "/" {
			t.Fatalf("RedirectLocal(%q) Location = %q, want fallback %q", target, got, "/")
		}
	}
}

func TestRedirectLocalRejectsNonRedirectStatus(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx := ReplaceResponseWriter(context.Background(), recorder)

	err := RedirectLocal(ctx, "/ok", "/", http.StatusOK)
	if err == nil {
		t.Fatal("RedirectLocal error = nil, want a status error")
	}
	if got := StatusCode(err); got != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", got)
	}
}

// browserNormalise is an independent, test-only version of the browser rules
// that the fuzz test checks SafeLocalPath against.
func browserNormalise(s string) string {
	s = strings.TrimFunc(s, func(r rune) bool { return r <= ' ' })
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\t', '\n', '\r':
		case '\\':
			b.WriteByte('/')
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func FuzzSafeLocalPath(f *testing.F) {
	for _, seed := range []string{
		"/\t/evil.example",
		"/\\evil.example",
		"//evil.example",
		"https://evil.example",
		"/ok",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, target string) {
		if !SafeLocalPath(target) {
			return
		}
		parsed, err := url.Parse(browserNormalise(target))
		if err != nil {
			t.Fatalf("SafeLocalPath(%q) is true but the browser form does not parse: %v", target, err)
		}
		if parsed.Host != "" || parsed.Scheme != "" {
			t.Fatalf("SafeLocalPath(%q) is true but the browser form has scheme %q and host %q",
				target, parsed.Scheme, parsed.Host)
		}
	})
}
