package router

import (
	"net/http"
	"reflect"
	"testing"
)

func TestApplicationSessionStoreOptions(t *testing.T) {
	tests := []struct {
		name   string
		secure bool
	}{
		{name: "development", secure: false},
		{name: "production", secure: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, err := newApplicationSessionStore([]byte("auth-key"), []byte("encryption-key"), 604800, test.secure)
			if err != nil {
				t.Fatalf("newApplicationSessionStore returned an error: %v", err)
			}

			options := store.Options
			if options.Path != "/" || options.MaxAge != 604800 || !options.HttpOnly || options.Secure != test.secure || options.SameSite != http.SameSiteLaxMode {
				t.Fatalf("unexpected session options: %+v", options)
			}
		})
	}
}

func TestApplicationSessionStoreRejectsInvalidLifetime(t *testing.T) {
	if _, err := newApplicationSessionStore([]byte("auth-key"), []byte("encryption-key"), 0, false); err == nil {
		t.Fatal("expected an invalid session lifetime error")
	}
}

func TestCORSDefaultsToApplicationOrigin(t *testing.T) {
	config, err := newCORSConfig("https://app.example.com", nil)
	if err != nil {
		t.Fatalf("newCORSConfig returned an error: %v", err)
	}

	want := []string{"https://app.example.com"}
	if !reflect.DeepEqual(config.AllowOrigins, want) {
		t.Fatalf("AllowOrigins = %v, want %v", config.AllowOrigins, want)
	}
	if !config.AllowCredentials {
		t.Fatal("expected credentialed CORS")
	}
}

func TestCORSRequiresExplicitAdditionalOrigins(t *testing.T) {
	config, err := newCORSConfig(
		"https://app.example.com",
		[]string{" https://admin.example.com ", ""},
	)
	if err != nil {
		t.Fatalf("newCORSConfig returned an error: %v", err)
	}

	want := []string{"https://app.example.com", "https://admin.example.com"}
	if !reflect.DeepEqual(config.AllowOrigins, want) {
		t.Fatalf("AllowOrigins = %v, want %v", config.AllowOrigins, want)
	}
}

func TestCredentialedCORSRejectsWildcards(t *testing.T) {
	tests := []struct {
		name              string
		applicationOrigin string
		additionalOrigins []string
	}{
		{name: "wildcard application origin", applicationOrigin: "*"},
		{name: "wildcard additional origin", applicationOrigin: "https://app.example.com", additionalOrigins: []string{"https://*"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := newCORSConfig(test.applicationOrigin, test.additionalOrigins); err == nil {
				t.Fatalf("expected wildcard CORS configuration to be rejected")
			}
		})
	}
}
