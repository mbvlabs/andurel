package inertia

import (
	"strings"
	"testing"
	"time"
)

func TestParseSSRListenHealthRewritesUnspecifiedBind(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		listen    string
		host      string
		port      string
		healthURL string
	}{
		{
			name:      "ipv4 unspecified",
			listen:    "http://0.0.0.0:13714",
			host:      "0.0.0.0",
			port:      "13714",
			healthURL: "http://127.0.0.1:13714",
		},
		{
			name:      "ipv6 unspecified",
			listen:    "http://[::]:13714",
			host:      "::",
			port:      "13714",
			healthURL: "http://[::1]:13714",
		},
		{
			name:      "loopback ipv4",
			listen:    "http://127.0.0.1:13714",
			host:      "127.0.0.1",
			port:      "13714",
			healthURL: "http://127.0.0.1:13714",
		},
		{
			name:      "loopback ipv6",
			listen:    "http://[::1]:13714",
			host:      "::1",
			port:      "13714",
			healthURL: "http://[::1]:13714",
		},
		{
			name:      "localhost",
			listen:    "http://localhost:13714",
			host:      "localhost",
			port:      "13714",
			healthURL: "http://localhost:13714",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			listen, err := parseSSRListen(tc.listen)
			if err != nil {
				t.Fatalf("parseSSRListen(%q): %v", tc.listen, err)
			}
			if listen.host != tc.host || listen.port != tc.port || listen.healthURL != tc.healthURL {
				t.Fatalf("got %+v, want host=%q port=%q healthURL=%q", listen, tc.host, tc.port, tc.healthURL)
			}
		})
	}
}

func TestValidateSSRListenRejectsHTTPS(t *testing.T) {
	t.Parallel()

	if err := ValidateSSRListen("https://127.0.0.1:13714"); err == nil {
		t.Fatal("expected https listen URL to be rejected")
	}
}

func TestValidateSSRListenRejectsServiceHostname(t *testing.T) {
	t.Parallel()

	err := ValidateSSRListen("http://ssr-service:13714")
	if err == nil {
		t.Fatal("expected hostname listen URL to be rejected")
	}
	if !strings.Contains(err.Error(), "localhost") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateSSRClientAllowsServiceHostname(t *testing.T) {
	t.Parallel()

	if err := ValidateSSRClient("http://ssr-service:13714", 2*time.Second, 2<<20); err != nil {
		t.Fatalf("ValidateSSRClient: %v", err)
	}
	if err := ValidateSSRClient("https://ssr.example.com", 2*time.Second, 2<<20); err != nil {
		t.Fatalf("ValidateSSRClient https: %v", err)
	}
}

func TestNewSSRRuntimeAllowsUnspecifiedListenAndRejectsHostname(t *testing.T) {
	t.Parallel()

	runtime, err := NewSSRRuntime(
		"node",
		"assets/dist/ssr/ssr.js",
		"http://0.0.0.0:13714",
		10*time.Second,
		22,
		2*time.Second,
		nil,
	)
	if err != nil {
		t.Fatalf("NewSSRRuntime 0.0.0.0: %v", err)
	}
	if runtime.listenHost != "0.0.0.0" || runtime.renderer.baseURL.Hostname() != "127.0.0.1" {
		t.Fatalf(
			"bind host = %q health host = %q",
			runtime.listenHost,
			runtime.renderer.baseURL.Hostname(),
		)
	}

	runtime, err = NewSSRRuntime(
		"node",
		"assets/dist/ssr/ssr.js",
		"http://[::]:13714",
		10*time.Second,
		22,
		2*time.Second,
		nil,
	)
	if err != nil {
		t.Fatalf("NewSSRRuntime [::]: %v", err)
	}
	if runtime.listenHost != "::" || runtime.renderer.baseURL.Hostname() != "::1" {
		t.Fatalf(
			"ipv6 bind host = %q health host = %q",
			runtime.listenHost,
			runtime.renderer.baseURL.Hostname(),
		)
	}

	if _, err := NewSSRRuntime(
		"node",
		"assets/dist/ssr/ssr.js",
		"http://ssr-service:13714",
		10*time.Second,
		22,
		2*time.Second,
		nil,
	); err == nil {
		t.Fatal("expected hostname listen URL to be rejected")
	}
}

func TestNewHTTPRendererAllowsServiceHostname(t *testing.T) {
	t.Parallel()

	renderer, err := NewHTTPRenderer("http://ssr-service:13714", 2*time.Second, 2<<20)
	if err != nil {
		t.Fatalf("NewHTTPRenderer: %v", err)
	}
	if renderer.baseURL.Hostname() != "ssr-service" {
		t.Fatalf("hostname = %q", renderer.baseURL.Hostname())
	}
}
