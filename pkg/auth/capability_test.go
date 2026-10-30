package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestNewCapabilityToken(t *testing.T) {
	const (
		prefix       = "dp_"
		entropyBytes = 32
	)

	plain, displayPrefix, hash, err := NewCapabilityToken(prefix, entropyBytes)
	if err != nil {
		t.Fatalf("NewCapabilityToken: %v", err)
	}

	if !strings.HasPrefix(plain, prefix) {
		t.Fatalf("token = %q, want prefix %q", plain, prefix)
	}

	encoded := strings.TrimPrefix(plain, prefix)
	if strings.ContainsAny(encoded, "=+/") {
		t.Fatalf("encoded entropy = %q, want unpadded base64url", encoded)
	}

	entropy, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode token entropy: %v", err)
	}
	if len(entropy) != entropyBytes {
		t.Fatalf("entropy length = %d, want %d", len(entropy), entropyBytes)
	}

	if displayPrefix != encoded[:CapabilityTokenDisplayLength] {
		t.Fatalf("display prefix = %q, want %q", displayPrefix, encoded[:CapabilityTokenDisplayLength])
	}

	wantHash := sha256.Sum256([]byte(plain))
	if !bytes.Equal(hash, wantHash[:]) {
		t.Fatalf("hash = %x, want %x", hash, wantHash)
	}
}

func TestNewCapabilityTokenReturnsDistinctTokens(t *testing.T) {
	seen := make(map[string]bool)

	for range 32 {
		plain, _, _, err := NewCapabilityToken("dp_", 32)
		if err != nil {
			t.Fatalf("NewCapabilityToken: %v", err)
		}
		if seen[plain] {
			t.Fatalf("NewCapabilityToken returned duplicate token %q", plain)
		}
		seen[plain] = true
	}
}

func TestNewCapabilityTokenRejectsShortEntropy(t *testing.T) {
	for _, entropyBytes := range []int{-1, 0, 15} {
		plain, displayPrefix, hash, err := NewCapabilityToken("dp_", entropyBytes)
		if err == nil {
			t.Fatalf("NewCapabilityToken(%d) = nil error, want rejection", entropyBytes)
		}
		if plain != "" || displayPrefix != "" || hash != nil {
			t.Fatalf(
				"NewCapabilityToken(%d) = %q, %q, %x, want empty results",
				entropyBytes,
				plain,
				displayPrefix,
				hash,
			)
		}
	}

	if _, _, _, err := NewCapabilityToken("dp_", 16); err != nil {
		t.Fatalf("NewCapabilityToken(16): %v", err)
	}
}

func TestHashCapabilityToken(t *testing.T) {
	const plain = "dp_example"

	got := HashCapabilityToken(plain)
	want := sha256.Sum256([]byte(plain))
	if !bytes.Equal(got, want[:]) {
		t.Fatalf("HashCapabilityToken = %x, want %x", got, want)
	}
}

func TestVerifyCapabilityToken(t *testing.T) {
	const plain = "dp_example"

	hash := HashCapabilityToken(plain)

	if !VerifyCapabilityToken(plain, hash) {
		t.Fatal("VerifyCapabilityToken rejected the matching hash")
	}
	if VerifyCapabilityToken("dp_other", hash) {
		t.Fatal("VerifyCapabilityToken accepted a different token")
	}
	if VerifyCapabilityToken(plain, hash[:len(hash)-1]) {
		t.Fatal("VerifyCapabilityToken accepted a truncated hash")
	}
	if VerifyCapabilityToken(plain, nil) {
		t.Fatal("VerifyCapabilityToken accepted a nil hash")
	}
}

func TestNewCapabilityTokenRejectsUnsafePrefixes(t *testing.T) {
	for _, prefix := range []string{
		"dp/",
		"dp token",
		"dp\t",
		"dp+",
		"dp=",
		"dp_ø",
	} {
		t.Run(prefix, func(t *testing.T) {
			plain, displayPrefix, hash, err := NewCapabilityToken(prefix, 16)
			if err == nil {
				t.Fatalf("NewCapabilityToken(%q) = nil error, want rejection", prefix)
			}
			if plain != "" || displayPrefix != "" || hash != nil {
				t.Fatalf(
					"NewCapabilityToken(%q) = %q, %q, %x, want empty results",
					prefix,
					plain,
					displayPrefix,
					hash,
				)
			}
		})
	}
}

func TestNewCapabilityTokenAcceptsURLSafePrefixes(t *testing.T) {
	for _, prefix := range []string{"", "dp_", "DP-123", "a", "dp--"} {
		t.Run(prefix, func(t *testing.T) {
			plain, _, _, err := NewCapabilityToken(prefix, 16)
			if err != nil {
				t.Fatalf("NewCapabilityToken(%q): %v", prefix, err)
			}
			if strings.ContainsAny(plain, " \t\r\n/+=") {
				t.Fatalf("token %q is not URL-safe", plain)
			}
			if !strings.HasPrefix(plain, prefix) {
				t.Fatalf("token = %q, want prefix %q", plain, prefix)
			}
		})
	}
}
