package routing

import (
	"strings"
	"testing"
)

func TestIsInertiaDefaultsFalse(t *testing.T) {
	route := NewSimpleRoute("/", "pages.home", "")
	if route.IsInertia() {
		t.Fatal("expected IsInertia to default to false")
	}

	uuidRoute := NewRouteWithUUIDID("/:id", "widgets.show", "/widgets")
	if uuidRoute.IsInertia() {
		t.Fatal("expected IsInertia to default to false for UUID routes")
	}
}

func TestInertiaRouteOption(t *testing.T) {
	route := NewSimpleRoute("/", "pages.home", "", InertiaRoute())
	if !route.IsInertia() {
		t.Fatal("expected InertiaRoute option to enable the flag")
	}

	paramsRoute := NewRouteWithParams[any]("/:slug", "posts.show", "/posts", InertiaRoute())
	if !paramsRoute.IsInertia() {
		t.Fatal("expected InertiaRoute option to enable the flag for params routes")
	}
}

func TestHostDefaultsToPrimary(t *testing.T) {
	route := NewSimpleRoute("/", "pages.home", "")
	if route.Host() != HostPrimary {
		t.Fatalf("expected HostPrimary, got %q", route.Host())
	}
}

func TestHostOption(t *testing.T) {
	const hostAdmin HostName = "admin"
	route := NewSimpleRoute("/widgets", "widgets.index", "", Host(hostAdmin))
	if route.Host() != hostAdmin {
		t.Fatalf("expected %q, got %q", hostAdmin, route.Host())
	}
}

func TestConfigureHostsRequiresPrimary(t *testing.T) {
	t.Cleanup(func() {
		hostMu.Lock()
		hostRegistry = nil
		hostMu.Unlock()
	})

	if err := ConfigureHosts(nil); err == nil {
		t.Fatal("expected error for empty registry")
	}
	if err := ConfigureHosts(map[HostName]HostSpec{
		"admin": {Hostname: "admin.example.com", Protocol: "https"},
	}); err == nil {
		t.Fatal("expected error when primary is missing")
	}
	if err := ConfigureHosts(map[HostName]HostSpec{
		HostPrimary: {Hostname: "", Protocol: "https"},
	}); err == nil {
		t.Fatal("expected error when primary hostname is empty")
	}
}

func TestFullURLUsesHostRegistry(t *testing.T) {
	const hostAdmin HostName = "admin"
	t.Cleanup(func() {
		hostMu.Lock()
		hostRegistry = nil
		hostMu.Unlock()
	})

	if err := ConfigureHosts(map[HostName]HostSpec{
		HostPrimary: {Hostname: "andurel.com", Protocol: "https"},
		hostAdmin:   {Hostname: "admin.andurel.com", Protocol: "https"},
	}); err != nil {
		t.Fatalf("configure hosts: %v", err)
	}

	primary := NewSimpleRoute("/", "pages.home", "")
	if got, want := primary.FullURL(), "https://andurel.com/"; got != want {
		t.Fatalf("primary FullURL = %q, want %q", got, want)
	}

	admin := NewSimpleRoute("", "widgets.index", "/widgets", Host(hostAdmin))
	if got, want := admin.FullURL(), "https://admin.andurel.com/widgets"; got != want {
		t.Fatalf("admin FullURL = %q, want %q", got, want)
	}
}

func TestHostBaseURLUnknownReturnsEmpty(t *testing.T) {
	t.Cleanup(func() {
		hostMu.Lock()
		hostRegistry = nil
		hostMu.Unlock()
	})

	if err := ConfigureHosts(map[HostName]HostSpec{
		HostPrimary: {Hostname: "andurel.com", Protocol: "https"},
	}); err != nil {
		t.Fatalf("configure hosts: %v", err)
	}

	if got := HostBaseURL("admin"); got != "" {
		t.Fatalf("unknown host BaseURL = %q, want empty", got)
	}
}

func TestConfigureHostsRejectsHostnameCollision(t *testing.T) {
	t.Cleanup(func() {
		hostMu.Lock()
		hostRegistry = nil
		hostMu.Unlock()
	})

	err := ConfigureHosts(map[HostName]HostSpec{
		HostPrimary: {Hostname: "andurel.com", Protocol: "https"},
		"admin":     {Hostname: "andurel.com", Protocol: "https"},
	})
	if err == nil {
		t.Fatal("expected hostname collision error")
	}
	if !strings.Contains(err.Error(), "andurel.com") {
		t.Fatalf("error should name the hostname, got %v", err)
	}
}

func TestConfigureHostsRejectsAliasCollision(t *testing.T) {
	t.Cleanup(func() {
		hostMu.Lock()
		hostRegistry = nil
		hostMu.Unlock()
	})

	err := ConfigureHosts(map[HostName]HostSpec{
		HostPrimary: {Hostname: "andurel.com", Aliases: []string{"www.andurel.com"}, Protocol: "https"},
		"admin":     {Hostname: "admin.andurel.com", Aliases: []string{"www.andurel.com"}, Protocol: "https"},
	})
	if err == nil {
		t.Fatal("expected alias collision error")
	}
}

func TestHostSpecOriginsIncludesAliases(t *testing.T) {
	spec := HostSpec{
		Hostname: "andurel.com",
		Aliases:  []string{"www.andurel.com", "andurel.com"},
		Protocol: "https",
	}
	origins := spec.Origins()
	if len(origins) != 2 {
		t.Fatalf("expected 2 unique origins, got %#v", origins)
	}
	if origins[0] != "https://andurel.com" || origins[1] != "https://www.andurel.com" {
		t.Fatalf("unexpected origins: %#v", origins)
	}
}
