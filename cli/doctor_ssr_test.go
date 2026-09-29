package cli

import (
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout"
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

func TestCheckInertiaSSRSkipsProductionBundleInDevelopment(t *testing.T) {
	root := t.TempDir()
	writeInertiaSSRDoctorProject(t, root, "ENVIRONMENT=development\n")
	withFakeNodeOnPath(t)

	got := checkInertiaSSRConfiguration(root)
	if got.status != statusPass {
		t.Fatalf("development SSR = %#v", got)
	}
	if !strings.Contains(got.message, "Vite") {
		t.Fatalf("development SSR message = %q", got.message)
	}
}

func TestCheckInertiaSSRDefaultsToDevelopmentWhenEnvironmentUnset(t *testing.T) {
	root := t.TempDir()
	writeInertiaSSRDoctorProject(t, root, "")
	withFakeNodeOnPath(t)

	got := checkInertiaSSRConfiguration(root)
	if got.status != statusPass {
		t.Fatalf("default environment SSR = %#v", got)
	}
}

func TestCheckInertiaSSRWarnsWhenProductionBundleMissing(t *testing.T) {
	root := t.TempDir()
	writeInertiaSSRDoctorProject(t, root, "ENVIRONMENT=production\n")
	withFakeNodeOnPath(t)

	got := checkInertiaSSRConfiguration(root)
	if got.status != statusWarn || !strings.Contains(got.message, "not built") {
		t.Fatalf("production missing bundle = %#v", got)
	}
}

func withFakeNodeOnPath(t *testing.T) {
	t.Helper()
	fakePath := t.TempDir()
	writeExecutable(t, fakePath, "node", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", fakePath)
}

func writeInertiaSSRDoctorProject(t *testing.T, root, env string) {
	t.Helper()
	lock := layout.NewAndurelLock("v1")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName: "app",
		Inertia:     "react",
	}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	writeTestFile(t, root, "cmd/ssr/main.go", "package main\n")
	if env != "" {
		writeTestFile(t, root, ".env", env)
	}
}
