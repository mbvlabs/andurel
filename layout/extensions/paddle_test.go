package extensions

import (
	"testing"
)

func TestPaddleExtension_Name(t *testing.T) {
	ext := Paddle{}
	expected := "paddle"

	if ext.Name() != expected {
		t.Errorf("Expected extension name %q, got %q", expected, ext.Name())
	}
}

func TestPaddleExtension_Dependencies(t *testing.T) {
	ext := Paddle{}
	deps := ext.Dependencies()

	if len(deps) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(deps))
	}

	if deps[0] != "auth" {
		t.Errorf("Expected dependency 'auth', got %q", deps[0])
	}
}

func TestPaddleExtension_Registration(t *testing.T) {
	ext := Paddle{}

	err := Register(ext)
	if err != nil {
		t.Fatalf("Failed to register Paddle extension: %v", err)
	}

	registered, ok := Get("paddle")
	if !ok {
		t.Fatal("Paddle extension not found in registry")
	}

	if registered.Name() != ext.Name() {
		t.Errorf("Expected registered extension name %q, got %q", ext.Name(), registered.Name())
	}
}
