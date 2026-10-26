package anduin

import (
	"net/http"
	"slices"
	"testing"
)

func TestMergeOriginsTrimsDropsAndDeduplicates(t *testing.T) {
	got := MergeOrigins(
		[]string{" https://a.example ", "", "https://a.example"},
		[]string{"https://b.example", "   "},
	)
	want := []string{"https://a.example", "https://b.example"}

	if !slices.Equal(got, want) {
		t.Fatalf("MergeOrigins = %v, want %v", got, want)
	}
}

func TestDefaultCORSConfiguresCredentialedPolicy(t *testing.T) {
	cfg := DefaultCORS("https://a.example", "https://a.example")

	if !slices.Equal(cfg.AllowOrigins, []string{"https://a.example"}) {
		t.Fatalf("AllowOrigins = %v", cfg.AllowOrigins)
	}
	if !cfg.AllowCredentials {
		t.Fatal("AllowCredentials = false, want true")
	}
	if cfg.MaxAge != 300 {
		t.Fatalf("MaxAge = %d, want 300", cfg.MaxAge)
	}
	if !slices.Contains(cfg.AllowMethods, http.MethodOptions) {
		t.Fatalf("AllowMethods = %v, want OPTIONS", cfg.AllowMethods)
	}
	if !slices.Contains(cfg.AllowHeaders, "X-CSRF-Token") {
		t.Fatalf("AllowHeaders = %v, want X-CSRF-Token", cfg.AllowHeaders)
	}
}

func TestDefaultCSRFConfiguresStrategy(t *testing.T) {
	headerOnly := DefaultCSRF("header_only", true, " https://a.example ")
	if headerOnly.TokenLookup != "header:X-CSRF-Token" {
		t.Fatalf("TokenLookup = %q", headerOnly.TokenLookup)
	}
	if !slices.Equal(headerOnly.TrustedOrigins, []string{"https://a.example"}) {
		t.Fatalf("TrustedOrigins = %v", headerOnly.TrustedOrigins)
	}
	if headerOnly.CookieName != "_csrf" || headerOnly.CookiePath != "/" {
		t.Fatalf("cookie = %q %q", headerOnly.CookieName, headerOnly.CookiePath)
	}
	if !headerOnly.CookieSecure || !headerOnly.CookieHTTPOnly {
		t.Fatalf("cookie flags = secure:%t httpOnly:%t", headerOnly.CookieSecure, headerOnly.CookieHTTPOnly)
	}
	if headerOnly.CookieSameSite != http.SameSiteStrictMode {
		t.Fatalf("SameSite = %v", headerOnly.CookieSameSite)
	}

	legacy := DefaultCSRF("header_or_legacy_token", false)
	if legacy.TokenLookup != "header:X-CSRF-Token,form:_csrf" {
		t.Fatalf("legacy TokenLookup = %q", legacy.TokenLookup)
	}
	if legacy.CookieSecure {
		t.Fatal("CookieSecure = true, want false")
	}
}
