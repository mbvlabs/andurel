package auth

import (
	"slices"
	"strings"
	"testing"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	const (
		password = "correct horse battery staple"
		pepper   = "current-pepper"
	)

	hash, err := HashPassword(password, pepper)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.Contains(hash, ":") {
		t.Fatalf("HashPassword = %q, want base64hash:base64salt", hash)
	}

	ok, err := VerifyPassword(hash, password, pepper)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Fatal("VerifyPassword rejected the matching password")
	}

	ok, err = VerifyPassword(hash, "wrong password", pepper)
	if err != nil {
		t.Fatalf("VerifyPassword wrong password: %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword accepted a wrong password")
	}

	ok, err = VerifyPassword(hash, password, "other-pepper")
	if err != nil {
		t.Fatalf("VerifyPassword other pepper: %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword accepted a different pepper")
	}
}

func TestVerifyPasswordAcceptsExistingHash(t *testing.T) {
	// Produced by the original scaffold with the same argon2id parameters and
	// "base64hash:base64salt" format.
	const (
		hash     = "tm6xMFPJrPeFRka9pWT8TVMWYfhpWaj7mJTZc7cuarQ:MDEyMzQ1Njc4OWFiY2RlZg"
		password = "correct horse battery staple"
		pepper   = "pepper"
	)

	ok, err := VerifyPassword(hash, password, pepper)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Fatal("VerifyPassword rejected a hash from the original scaffold")
	}
}

func TestVerifyPasswordRejectsInvalidFormat(t *testing.T) {
	for _, hash := range []string{"", "missing-separator", "a:b:c", "not-base64:salt"} {
		if _, err := VerifyPassword(hash, "password", "pepper"); err == nil {
			t.Fatalf("VerifyPassword(%q) error = nil, want an invalid format error", hash)
		}
	}
}

func TestVerifyPasswordAny(t *testing.T) {
	const (
		password       = "correct horse battery staple"
		currentPepper  = "current-pepper"
		previousPepper = "previous-pepper"
	)

	tests := []struct {
		name             string
		hashPepper       string
		providedPassword string
		previousPeppers  []string
		wantValid        bool
		wantRehash       bool
	}{
		{
			name:             "current pepper",
			hashPepper:       currentPepper,
			providedPassword: password,
			previousPeppers:  []string{previousPepper},
			wantValid:        true,
		},
		{
			name:             "previous pepper requires rehash",
			hashPepper:       previousPepper,
			providedPassword: password,
			previousPeppers:  []string{"older-pepper", previousPepper},
			wantValid:        true,
			wantRehash:       true,
		},
		{
			name:             "invalid after all peppers",
			hashPepper:       "unknown-pepper",
			providedPassword: "wrong password",
			previousPeppers:  []string{"older-pepper", previousPepper},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hash, err := HashPassword(password, test.hashPepper)
			if err != nil {
				t.Fatalf("HashPassword: %v", err)
			}

			valid, needsRehash, err := VerifyPasswordAny(
				hash,
				test.providedPassword,
				currentPepper,
				test.previousPeppers...,
			)
			if err != nil {
				t.Fatalf("VerifyPasswordAny: %v", err)
			}
			if valid != test.wantValid || needsRehash != test.wantRehash {
				t.Fatalf(
					"result = valid %t, rehash %t; want valid %t, rehash %t",
					valid,
					needsRehash,
					test.wantValid,
					test.wantRehash,
				)
			}
		})
	}
}

func TestPeppersTrimsAndDropsCurrentAndEmpty(t *testing.T) {
	got := Peppers("current", []string{
		"",
		"  current  ",
		" previous ",
		"   ",
		"older",
		"previous",
	})
	want := []string{"previous", "older", "previous"}

	if !slices.Equal(got, want) {
		t.Fatalf("Peppers() = %#v, want %#v", got, want)
	}
}

func TestGenerateCode(t *testing.T) {
	code, err := GenerateCode(6)
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("GenerateCode length = %d, want 6", len(code))
	}
	for _, char := range code {
		if !strings.ContainsRune(codeAlphabet, char) {
			t.Fatalf("GenerateCode returned %q outside the alphabet", code)
		}
	}
}

func TestGenerateToken(t *testing.T) {
	first, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	second, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if len(first) != 24 {
		t.Fatalf("GenerateToken length = %d, want 24", len(first))
	}
	if first == second {
		t.Fatal("GenerateToken returned the same token twice")
	}
}

func TestTokenHashRoundTrip(t *testing.T) {
	const secret = "signing-secret"

	hash := HashToken("plain-token", secret)
	if !VerifyToken("plain-token", secret, hash) {
		t.Fatal("VerifyToken rejected the matching token")
	}
	if VerifyToken("other-token", secret, hash) {
		t.Fatal("VerifyToken accepted a different token")
	}
	if VerifyToken("plain-token", "other-secret", hash) {
		t.Fatal("VerifyToken accepted a different secret")
	}
	if VerifyToken("plain-token", secret, "") {
		t.Fatal("VerifyToken accepted an empty hash")
	}
}
