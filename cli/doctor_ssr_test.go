package cli

import (
	"testing"
)

func TestDoctorSSRListenHealthURLRewritesUnspecifiedBind(t *testing.T) {
	t.Parallel()

	got, err := doctorSSRListenHealthURL("http://0.0.0.0:13714")
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "http://127.0.0.1:13714" {
		t.Fatalf("ipv4 unspecified health = %q", got)
	}

	got, err = doctorSSRListenHealthURL("http://[::]:13714")
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "http://[::1]:13714" {
		t.Fatalf("ipv6 unspecified health = %q", got)
	}
}

func TestDoctorSSRListenHealthURLRejectsServiceHostname(t *testing.T) {
	t.Parallel()

	if _, err := doctorSSRListenHealthURL("http://ssr-service:13714"); err == nil {
		t.Fatal("expected hostname listen URL to be rejected")
	}
}

func TestDoctorSSRListenHealthURLAcceptsLoopback(t *testing.T) {
	t.Parallel()

	got, err := doctorSSRListenHealthURL("http://127.0.0.1:13714")
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "http://127.0.0.1:13714" {
		t.Fatalf("loopback health = %q", got)
	}
}
