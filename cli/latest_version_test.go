package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/cli/output"
)

func TestFetchLatestAndurelVersion(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       string
		wantErr    string
	}{
		{name: "stable release", statusCode: http.StatusOK, body: `{"Version":"v1.4.2"}`, want: "v1.4.2"},
		{name: "version without prefix", statusCode: http.StatusOK, body: `{"Version":"1.4.2"}`, want: "v1.4.2"},
		{name: "prerelease", statusCode: http.StatusOK, body: `{"Version":"v1.5.0-rc.1"}`, wantErr: "not a stable release"},
		{name: "invalid version", statusCode: http.StatusOK, body: `{"Version":"latest"}`, wantErr: "not valid semantic versioning"},
		{name: "server error", statusCode: http.StatusServiceUnavailable, body: "unavailable", wantErr: "503 Service Unavailable"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.Header.Get("Accept") != "application/json" {
					t.Errorf("Accept header = %q", request.Header.Get("Accept"))
				}
				response.WriteHeader(test.statusCode)
				_, _ = response.Write([]byte(test.body))
			}))
			defer server.Close()

			got, err := fetchLatestAndurelVersion(context.Background(), server.Client(), server.URL)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want error containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("fetch latest version: %v", err)
			}
			if got != test.want {
				t.Fatalf("version = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAndurelVersionHelpers(t *testing.T) {
	if got, ok := canonicalAndurelVersion(" 1.2.3 "); !ok || got != "v1.2.3" {
		t.Fatalf("canonical version = %q, %t", got, ok)
	}
	if got, ok := canonicalAndurelVersion("dev"); ok || got != "" {
		t.Fatalf("development version = %q, %t", got, ok)
	}
	if !newerAndurelVersion("v1.2.3", "v1.3.0") {
		t.Fatal("expected v1.3.0 to be newer than v1.2.3")
	}
	if newerAndurelVersion("v1.3.0", "v1.3.0") || newerAndurelVersion("v1.3.0", "v1.2.3") {
		t.Fatal("equal or older releases must not be reported as newer")
	}
	if got := andurelInstallCommand("v1.3.0"); got != "go install github.com/mbvlabs/andurel@v1.3.0" {
		t.Fatalf("install command = %q", got)
	}
}

func TestCheckLatestAndurelRelease(t *testing.T) {
	t.Run("new release", func(t *testing.T) {
		stubLatestAndurelVersion(t, "v1.3.0", nil)
		result := checkLatestAndurelRelease("v1.2.0")
		if result.status != statusWarn || !strings.Contains(result.message, "v1.3.0 is available") ||
			!strings.Contains(result.hint, "go install github.com/mbvlabs/andurel@v1.3.0") {
			t.Fatalf("release check = %#v", result)
		}
	})

	t.Run("current release", func(t *testing.T) {
		stubLatestAndurelVersion(t, "v1.3.0", nil)
		result := checkLatestAndurelRelease("v1.3.0")
		if result.status != statusPass || !strings.Contains(result.message, "latest stable release installed") {
			t.Fatalf("release check = %#v", result)
		}
	})

	t.Run("prerelease ahead of stable", func(t *testing.T) {
		stubLatestAndurelVersion(t, "v1.3.0", nil)
		result := checkLatestAndurelRelease("v1.4.0-rc.1")
		if result.status != statusPass || !strings.Contains(result.message, "no newer stable release found") {
			t.Fatalf("release check = %#v", result)
		}
	})

	t.Run("development build", func(t *testing.T) {
		called := false
		original := lookupLatestAndurelVersionFunc
		lookupLatestAndurelVersionFunc = func(context.Context) (string, error) {
			called = true
			return "v1.3.0", nil
		}
		t.Cleanup(func() { lookupLatestAndurelVersionFunc = original })

		result := checkLatestAndurelRelease("dev")
		if result.status != statusPass || called {
			t.Fatalf("development release check = %#v, called = %t", result, called)
		}
	})

	t.Run("lookup failure", func(t *testing.T) {
		stubLatestAndurelVersion(t, "", errors.New("network unavailable"))
		result := checkLatestAndurelRelease("v1.2.0")
		if result.status != statusWarn || !strings.Contains(result.message, "could not check") || len(result.details) != 1 {
			t.Fatalf("release check = %#v", result)
		}
	})
}

func TestRequireLatestAndurelRelease(t *testing.T) {
	t.Run("new release blocks upgrade", func(t *testing.T) {
		stubLatestAndurelVersion(t, "v1.3.0", nil)
		err := requireLatestAndurelRelease(context.Background(), "v1.2.0")
		var cliErr *output.CLIError
		if !errors.As(err, &cliErr) {
			t.Fatalf("error = %v, want CLIError", err)
		}
		if cliErr.Code != output.CodeUpdateRequired || cliErr.ExitCode != output.ExitDependency ||
			!strings.Contains(cliErr.Hint, "go install github.com/mbvlabs/andurel@v1.3.0") {
			t.Fatalf("CLI error = %#v", cliErr)
		}
	})

	t.Run("current release continues", func(t *testing.T) {
		stubLatestAndurelVersion(t, "v1.3.0", nil)
		if err := requireLatestAndurelRelease(context.Background(), "v1.3.0"); err != nil {
			t.Fatalf("current release: %v", err)
		}
	})

	t.Run("lookup failure continues", func(t *testing.T) {
		stubLatestAndurelVersion(t, "", errors.New("network unavailable"))
		if err := requireLatestAndurelRelease(context.Background(), "v1.2.0"); err != nil {
			t.Fatalf("lookup failure: %v", err)
		}
	})

	t.Run("development build continues without lookup", func(t *testing.T) {
		called := false
		original := lookupLatestAndurelVersionFunc
		lookupLatestAndurelVersionFunc = func(context.Context) (string, error) {
			called = true
			return "v1.3.0", nil
		}
		t.Cleanup(func() { lookupLatestAndurelVersionFunc = original })

		if err := requireLatestAndurelRelease(context.Background(), "dev"); err != nil || called {
			t.Fatalf("development release: error = %v, called = %t", err, called)
		}
	})
}
