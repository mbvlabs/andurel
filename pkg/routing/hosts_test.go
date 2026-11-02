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

func TestNewHostsValidatesHostnameSyntax(t *testing.T) {
	t.Parallel()

	const hostAdmin HostName = "admin"
	cases := []struct {
		label    string
		hostname string
		wantErr  string
	}{
		{"bare host", "admin.localhost", ""},
		{"host and port", "admin.localhost:8080", ""},
		{"single label", "localhost", ""},
		{"underscore and digits", "admin_2.example.com", ""},
		{"ipv4", "127.0.0.1:8080", ""},
		{"ipv6", "[::1]", ""},
		{"ipv6 and port", "[::1]:8080", ""},
		{"surrounding whitespace is trimmed", "  admin.localhost  ", ""},
		{
			"whitespace and path", "bad host/x",
			`routing: host "admin" (HOST_ADMIN): hostname "bad host/x" must be host or host:port: it contains whitespace`,
		},
		{
			"scheme", "http://admin.localhost",
			`routing: host "admin" (HOST_ADMIN): hostname "http://admin.localhost" must be host or host:port: remove the scheme`,
		},
		{
			"path", "admin.localhost/x",
			`routing: host "admin" (HOST_ADMIN): hostname "admin.localhost/x" must be host or host:port: remove the path`,
		},
		{
			"query", "admin.localhost?x=1",
			`routing: host "admin" (HOST_ADMIN): hostname "admin.localhost?x=1" must be host or host:port: remove the path`,
		},
		{
			"user info", "me@admin.localhost",
			`routing: host "admin" (HOST_ADMIN): hostname "me@admin.localhost" must be host or host:port: remove the user info`,
		},
		{
			"empty host part", ":8080",
			`routing: host "admin" (HOST_ADMIN): hostname ":8080" must be host or host:port: the host part is empty`,
		},
		{
			"empty port", "admin.localhost:",
			`routing: host "admin" (HOST_ADMIN): hostname "admin.localhost:" must be host or host:port: the port is empty`,
		},
		{
			"port not a number", "admin.localhost:http",
			`routing: host "admin" (HOST_ADMIN): hostname "admin.localhost:http" must be host or host:port: port "http" is not a number from 1 to 65535`,
		},
		{
			"port out of range", "admin.localhost:70000",
			`routing: host "admin" (HOST_ADMIN): hostname "admin.localhost:70000" must be host or host:port: port "70000" is not a number from 1 to 65535`,
		},
		{
			"unbracketed ipv6", "::1",
			`routing: host "admin" (HOST_ADMIN): hostname "::1" must be host or host:port: put an IPv6 address in brackets`,
		},
		{
			"bracketed non-ipv6", "[admin]",
			`routing: host "admin" (HOST_ADMIN): hostname "[admin]" must be host or host:port: "admin" in brackets is not an IPv6 address`,
		},
		{
			"unclosed bracket", "[::1",
			`routing: host "admin" (HOST_ADMIN): hostname "[::1" must be host or host:port: the IPv6 address has no closing bracket`,
		},
		{
			"bad character", "admin*.localhost",
			`routing: host "admin" (HOST_ADMIN): hostname "admin*.localhost" must be host or host:port: it contains '*'`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			t.Parallel()

			hosts, err := NewHosts(map[HostName]HostSpec{
				HostPrimary: {Hostname: "localhost:8080"},
				hostAdmin:   {Hostname: tc.hostname},
			})
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("NewHosts(%q) error = %v, want nil", tc.hostname, err)
				}
				if _, ok := hosts.Lookup(hostAdmin); !ok {
					t.Fatalf("NewHosts(%q) dropped the admin host", tc.hostname)
				}
				return
			}
			if err == nil {
				t.Fatalf("NewHosts(%q) error = nil, want %q", tc.hostname, tc.wantErr)
			}
			if err.Error() != tc.wantErr {
				t.Fatalf("NewHosts(%q) error =\n  %s\nwant\n  %s", tc.hostname, err, tc.wantErr)
			}
		})
	}
}

func TestNewHostsValidatesPrimaryAndAliases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		label   string
		specs   map[HostName]HostSpec
		wantErr string
	}{
		{
			"primary with scheme",
			map[HostName]HostSpec{HostPrimary: {Hostname: "https://example.com"}},
			`routing: host "primary" (HOST_PRIMARY): hostname "https://example.com" must be host or host:port: remove the scheme`,
		},
		{
			"alias with path",
			map[HostName]HostSpec{
				HostPrimary: {Hostname: "example.com", Aliases: []string{"www.example.com/home"}},
			},
			`routing: host "primary": alias "www.example.com/home" must be host or host:port: remove the path`,
		},
		{
			"env key for a name with a dash",
			map[HostName]HostSpec{
				HostPrimary:   {Hostname: "example.com"},
				"back-office": {Hostname: "bad host"},
			},
			`routing: host "back-office" (HOST_BACK_OFFICE): hostname "bad host" must be host or host:port: it contains whitespace`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			t.Parallel()

			_, err := NewHosts(tc.specs)
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("NewHosts error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}
