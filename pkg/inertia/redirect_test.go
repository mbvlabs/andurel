package inertia

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRedirectCrossDomainInertiaUsesLocation(t *testing.T) {
	t.Parallel()

	renderer, err := newTestRenderer(WithRoot(testRoot(new(RootData))))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "https://admin.example.com/sign-out", nil)
	request.Header.Set(HeaderInertia, "true")
	recorder := httptest.NewRecorder()
	if err := renderer.Redirect(testContext(request, recorder), "https://example.com/", http.StatusSeeOther); err != nil {
		t.Fatalf("Redirect: %v", err)
	}

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
	if got := recorder.Header().Get(HeaderLocation); got != "https://example.com/" {
		t.Fatalf("%s = %q, want %q", HeaderLocation, got, "https://example.com/")
	}
	if got := recorder.Header().Get("Location"); got != "" {
		t.Fatalf("Location = %q, want empty for cross-domain Inertia redirect", got)
	}
}

func TestRedirectCrossDomainHostComparisonIsCaseInsensitivePortAware(t *testing.T) {
	t.Parallel()

	renderer, err := newTestRenderer(WithRoot(testRoot(new(RootData))))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	tests := []struct {
		name     string
		host     string
		location string
		want     int
	}{
		{name: "same host different case", host: "Admin.Example.com", location: "https://admin.example.com/users", want: http.StatusSeeOther},
		{name: "same host and port", host: "localhost:3000", location: "http://localhost:3000/users", want: http.StatusSeeOther},
		{name: "different port", host: "localhost:3000", location: "http://localhost:4000/users", want: http.StatusConflict},
		{name: "different host", host: "admin.example.com", location: "https://example.com/", want: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodPost, "/sign-out", nil)
			request.Host = test.host
			request.Header.Set(HeaderInertia, "true")
			recorder := httptest.NewRecorder()
			if err := renderer.Redirect(testContext(request, recorder), test.location, http.StatusSeeOther); err != nil {
				t.Fatalf("Redirect: %v", err)
			}
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d", recorder.Code, test.want)
			}
		})
	}
}

func TestRedirectInertiaSameDomainAbsoluteURLIsNormalRedirect(t *testing.T) {
	t.Parallel()

	renderer, err := newTestRenderer(WithRoot(testRoot(new(RootData))))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "https://example.com/sign-out", nil)
	request.Header.Set(HeaderInertia, "true")
	recorder := httptest.NewRecorder()
	if err := renderer.Redirect(testContext(request, recorder), "https://example.com/login", http.StatusSeeOther); err != nil {
		t.Fatalf("Redirect: %v", err)
	}

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if got := recorder.Header().Get("Location"); got != "https://example.com/login" {
		t.Fatalf("Location = %q, want %q", got, "https://example.com/login")
	}
	if got := recorder.Header().Get(HeaderLocation); got != "" {
		t.Fatalf("%s = %q, want empty for same-domain redirect", HeaderLocation, got)
	}
}

func TestRedirectInertiaRelativePathIsNormalRedirect(t *testing.T) {
	t.Parallel()

	renderer, err := newTestRenderer(WithRoot(testRoot(new(RootData))))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "https://example.com/sign-out", nil)
	request.Header.Set(HeaderInertia, "true")
	recorder := httptest.NewRecorder()
	if err := renderer.Redirect(testContext(request, recorder), "/login", http.StatusSeeOther); err != nil {
		t.Fatalf("Redirect: %v", err)
	}

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if got := recorder.Header().Get("Location"); got != "/login" {
		t.Fatalf("Location = %q, want %q", got, "/login")
	}
}

func TestRedirectNonInertiaCrossDomainIsNormalRedirect(t *testing.T) {
	t.Parallel()

	renderer, err := newTestRenderer(WithRoot(testRoot(new(RootData))))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "https://admin.example.com/sign-out", nil)
	recorder := httptest.NewRecorder()
	if err := renderer.Redirect(testContext(request, recorder), "https://example.com/", http.StatusSeeOther); err != nil {
		t.Fatalf("Redirect: %v", err)
	}

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if got := recorder.Header().Get("Location"); got != "https://example.com/" {
		t.Fatalf("Location = %q, want %q", got, "https://example.com/")
	}
	if got := recorder.Header().Get(HeaderLocation); got != "" {
		t.Fatalf("%s = %q, want empty for non-Inertia redirect", HeaderLocation, got)
	}
}
