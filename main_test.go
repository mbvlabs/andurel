package main

import "testing"

func TestGetVersionUsesExplicitVersionAndFallback(t *testing.T) {
	original := version
	t.Cleanup(func() {
		version = original
	})

	version = "v9.9.9"
	if got := getVersion(); got != "v9.9.9" {
		t.Fatalf("getVersion explicit = %q", got)
	}

	version = ""
	if got := getVersion(); got == "" {
		t.Fatalf("getVersion fallback should not be empty")
	}
}
