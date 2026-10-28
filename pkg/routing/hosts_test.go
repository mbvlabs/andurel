package routing

import "testing"

func TestHostsResolve(t *testing.T) {
	t.Parallel()

	const (
		hostAdmin HostName = "admin"
		hostAPI   HostName = "api"
		hostIPv6  HostName = "ipv6"
	)

	hosts, err := NewHosts(map[HostName]HostSpec{
		HostPrimary: {Hostname: "andurel.com", Protocol: "https"},
		hostAdmin:   {Hostname: "admin.andurel.com", Aliases: []string{"manage.andurel.com"}},
		hostAPI:     {Hostname: "api.andurel.com:8080"},
		hostIPv6:    {Hostname: "[::1]"},
	})
	if err != nil {
		t.Fatalf("new hosts: %v", err)
	}

	cases := []struct {
		label     string
		authority string
		want      HostName
		wantOK    bool
	}{
		{"exact hostname", "andurel.com", HostPrimary, true},
		{"exact hostname with port", "andurel.com:443", HostPrimary, true},
		{"bare hostname with default http port", "andurel.com:80", HostPrimary, true},
		{"bare hostname with arbitrary port", "andurel.com:8080", HostPrimary, true},
		{"exact admin hostname", "admin.andurel.com", hostAdmin, true},
		{"alias", "manage.andurel.com", hostAdmin, true},
		{"alias with port", "manage.andurel.com:8443", hostAdmin, true},
		{"configured host with port", "api.andurel.com:8080", hostAPI, true},
		{"configured host without its port", "api.andurel.com", "", false},
		{"uppercase hostname", "ADMIN.ANDUREL.COM", hostAdmin, true},
		{"mixed case hostname with port", "Admin.Andurel.Com:443", hostAdmin, true},
		{"surrounding whitespace", "  andurel.com  ", HostPrimary, true},
		{"ipv6 literal with port", "[::1]:8080", hostIPv6, true},
		{"ipv6 literal", "[::1]", hostIPv6, true},
		{"unknown hostname", "unconfigured.example", "", false},
		{"empty authority", "", "", false},
		{"whitespace authority", "   ", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			t.Parallel()

			name, ok := hosts.Resolve(tc.authority)
			if ok != tc.wantOK || name != tc.want {
				t.Fatalf(
					"Resolve(%q) = (%q, %v), want (%q, %v)",
					tc.authority,
					name,
					ok,
					tc.want,
					tc.wantOK,
				)
			}
		})
	}
}

func TestNilHostsResolveReturnsUnknown(t *testing.T) {
	t.Parallel()

	var hosts *Hosts
	name, ok := hosts.Resolve("andurel.com")
	if ok || name != "" {
		t.Fatalf("nil Hosts.Resolve = (%q, %v), want (\"\", false)", name, ok)
	}
}

func TestNewHostsRejectsCaseInsensitiveHostnameCollision(t *testing.T) {
	t.Parallel()

	_, err := NewHosts(map[HostName]HostSpec{
		HostPrimary: {Hostname: "andurel.com", Protocol: "https"},
		"admin":     {Hostname: "Andurel.Com", Protocol: "https"},
	})
	if err == nil {
		t.Fatal("expected case-insensitive hostname collision error")
	}
}

func TestNewHostsRejectsCaseInsensitiveAliasCollision(t *testing.T) {
	t.Parallel()

	_, err := NewHosts(map[HostName]HostSpec{
		HostPrimary: {
			Hostname: "andurel.com",
			Aliases:  []string{"www.andurel.com"},
			Protocol: "https",
		},
		"admin": {
			Hostname: "admin.andurel.com",
			Aliases:  []string{"WWW.Andurel.Com"},
			Protocol: "https",
		},
	})
	if err == nil {
		t.Fatal("expected case-insensitive alias collision error")
	}
}
