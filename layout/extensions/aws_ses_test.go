package extensions

import (
	"testing"
)

func TestAwsSesExtension_Name(t *testing.T) {
	ext := AwsSes{}
	expected := "aws-ses"

	if ext.Name() != expected {
		t.Errorf("Expected extension name %q, got %q", expected, ext.Name())
	}
}

func TestAwsSesExtension_Dependencies(t *testing.T) {
	ext := AwsSes{}
	deps := ext.Dependencies()

	if deps != nil {
		t.Errorf("Expected no dependencies, got %v", deps)
	}
}

func TestAwsSesExtension_Registration(t *testing.T) {
	ext := AwsSes{}

	err := Register(ext)
	if err != nil {
		t.Fatalf("Failed to register AWS SES extension: %v", err)
	}

	registered, ok := Get("aws-ses")
	if !ok {
		t.Fatal("AWS SES extension not found in registry")
	}

	if registered.Name() != ext.Name() {
		t.Errorf("Expected registered extension name %q, got %q", ext.Name(), registered.Name())
	}
}
