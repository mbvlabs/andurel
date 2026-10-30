package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
)

// CapabilityTokenDisplayLength is the number of leading characters of the
// encoded entropy, after the prefix, that NewCapabilityToken returns as
// displayPrefix. Store that prefix, never the full plaintext, for later display
// in lists and audit logs.
const CapabilityTokenDisplayLength = 8

// NewCapabilityToken returns a capability token made of prefix followed by
// entropyBytes random bytes encoded with unpadded base64url. It also returns
// the first CapabilityTokenDisplayLength characters of the encoded entropy for
// display, and the SHA-256 hash of the full plaintext for storage and lookup.
// prefix may contain only ASCII letters, digits, '-' and '_', so the complete
// token stays URL-safe. entropyBytes must be at least 16 so the token has at
// least 128 bits of entropy.
func NewCapabilityToken(
	prefix string,
	entropyBytes int,
) (plain string, displayPrefix string, hash []byte, err error) {
	if err := validateCapabilityTokenPrefix(prefix); err != nil {
		return "", "", nil, err
	}
	if entropyBytes < 16 {
		return "", "", nil, fmt.Errorf(
			"auth: capability token entropy must be at least 16 bytes, got %d",
			entropyBytes,
		)
	}

	entropy := make([]byte, entropyBytes)
	if _, err := rand.Read(entropy); err != nil {
		return "", "", nil, fmt.Errorf("auth: generate capability token: %w", err)
	}

	encoded := base64.RawURLEncoding.EncodeToString(entropy)
	plain = prefix + encoded
	displayPrefix = encoded[:CapabilityTokenDisplayLength]

	return plain, displayPrefix, HashCapabilityToken(plain), nil
}

func validateCapabilityTokenPrefix(prefix string) error {
	for index := 0; index < len(prefix); index++ {
		char := prefix[index]
		if char >= 'a' && char <= 'z' ||
			char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' ||
			char == '-' || char == '_' {
			continue
		}

		return fmt.Errorf(
			"auth: capability token prefix %q contains invalid character %q; use ASCII letters, digits, '-' and '_'",
			prefix,
			char,
		)
	}

	return nil
}

// HashCapabilityToken returns the unkeyed SHA-256 hash stored for a capability
// token. Store this hash, not the plaintext, and look tokens up by it.
func HashCapabilityToken(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))

	return sum[:]
}

// VerifyCapabilityToken reports whether hash is the SHA-256 of plain. It
// compares in constant time, so it is safe for capability-token lookup.
func VerifyCapabilityToken(plain string, hash []byte) bool {
	expected := sha256.Sum256([]byte(plain))

	return subtle.ConstantTimeCompare(expected[:], hash) == 1
}
