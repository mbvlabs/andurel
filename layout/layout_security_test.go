package layout

import (
	"errors"
	"strings"
	"testing"
)

type failingRandomReader struct {
	err error
}

func (r failingRandomReader) Read([]byte) (int, error) {
	return 0, r.err
}

func TestGenerateRandomHex(t *testing.T) {
	got, err := generateRandomHex(strings.NewReader("abcd"), 4)
	if err != nil {
		t.Fatalf("generateRandomHex returned an error: %v", err)
	}
	if got != "61626364" {
		t.Fatalf("generateRandomHex = %q, want %q", got, "61626364")
	}
}

func TestGenerateScaffoldSecretsReturnsRandomSourceFailure(t *testing.T) {
	randomErr := errors.New("random source failed")

	_, err := generateScaffoldSecrets(failingRandomReader{err: randomErr})
	if !errors.Is(err, randomErr) {
		t.Fatalf("generateScaffoldSecrets error = %v, want wrapped random source failure", err)
	}
}
