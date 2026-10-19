package anduin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseTrustedProxies(t *testing.T) {
	t.Parallel()

	prefixes, err := ParseTrustedProxies([]string{" 10.0.0.0/8 ", "", "127.0.0.1", "::1", "192.168.1.7/16"})
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}
	want := []string{"10.0.0.0/8", "127.0.0.1/32", "::1/128", "192.168.0.0/16"}
	if len(prefixes) != len(want) {
		t.Fatalf("prefixes = %v, want %v", prefixes, want)
	}
	for i, prefix := range prefixes {
		if prefix.String() != want[i] {
			t.Errorf("prefix[%d] = %s, want %s", i, prefix, want[i])
		}
	}

	if _, err := ParseTrustedProxies([]string{"not-an-ip"}); err == nil {
		t.Fatal("ParseTrustedProxies accepted an invalid address")
	}
}

func TestClientIP(t *testing.T) {
	t.Parallel()

	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}

	cases := []struct {
		name       string
		remoteAddr string
		forwarded  []string
		middleware bool
		want       string
	}{
		{
			name:       "no middleware ignores forwarding headers",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"203.0.113.9"},
			want:       "10.0.0.1",
		},
		{
			name:       "untrusted peer ignores forwarding headers",
			remoteAddr: "198.51.100.4:1234",
			forwarded:  []string{"203.0.113.9"},
			middleware: true,
			want:       "198.51.100.4",
		},
		{
			name:       "trusted peer uses forwarded client",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"203.0.113.9"},
			middleware: true,
			want:       "203.0.113.9",
		},
		{
			name:       "spoofed left-most entries are skipped",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"1.2.3.4, 203.0.113.9, 10.0.0.2"},
			middleware: true,
			want:       "203.0.113.9",
		},
		{
			name:       "multiple headers are read right to left",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"1.2.3.4", "203.0.113.9"},
			middleware: true,
			want:       "203.0.113.9",
		},
		{
			name:       "all hops trusted yields left-most",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"10.0.0.3, 10.0.0.2"},
			middleware: true,
			want:       "10.0.0.3",
		},
		{
			name:       "invalid hop stops at last trusted address",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"203.0.113.9, garbage, 10.0.0.2"},
			middleware: true,
			want:       "10.0.0.2",
		},
		{
			name:       "trusted peer without header is the client",
			remoteAddr: "10.0.0.1:1234",
			middleware: true,
			want:       "10.0.0.1",
		},
		{
			name:       "ipv4-mapped peer is unmapped",
			remoteAddr: "[::ffff:198.51.100.4]:1234",
			middleware: true,
			want:       "198.51.100.4",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = tc.remoteAddr
			for _, value := range tc.forwarded {
				request.Header.Add("X-Forwarded-For", value)
			}

			var got string
			handler := func(ctx context.Context) error {
				got = ClientIP(ctx)
				return nil
			}
			if tc.middleware {
				handler = RealIP(trusted...)(handler)
			}
			if err := Serve(httptest.NewRecorder(), request, handler); err != nil {
				t.Fatalf("Serve: %v", err)
			}
			if got != tc.want {
				t.Fatalf("ClientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClientIPSurvivesRouteDispatch(t *testing.T) {
	t.Parallel()

	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}

	rt := NewRouter()
	rt.Use(RealIP(trusted...))
	rt.Get("/ip", func(ctx context.Context) error {
		return Text(ctx, http.StatusOK, ClientIP(ctx))
	})

	request := httptest.NewRequest(http.MethodGet, "/ip", nil)
	request.RemoteAddr = "10.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, request)

	if got := recorder.Body.String(); got != "203.0.113.9" {
		t.Fatalf("ClientIP in route = %q, want %q", got, "203.0.113.9")
	}
}
