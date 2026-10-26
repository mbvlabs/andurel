// Package auth provides password hashing and token primitives for Andurel
// applications.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 2
	argonMemory  = 19 * 1024
	argonThreads = 1
	argonKeyLen  = 32
	saltLen      = 16
)

// HashPassword hashes password with the current pepper using argon2id. The
// result is "base64hash:base64salt" with unpadded standard base64.
func HashPassword(password, pepper string) (string, error) {
	salt, err := generateSalt(saltLen)
	if err != nil {
		return "", err
	}

	hash := argon2.IDKey(
		[]byte(password+pepper),
		salt,
		argonTime,
		argonMemory,
		argonThreads,
		argonKeyLen,
	)

	encodedHash := fmt.Sprintf("%s:%s",
		base64.RawStdEncoding.EncodeToString(hash),
		base64.RawStdEncoding.EncodeToString(salt))

	return encodedHash, nil
}

// VerifyPassword reports whether password matches hash under pepper.
func VerifyPassword(hash, password, pepper string) (bool, error) {
	parts := strings.Split(hash, ":")
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid stored password format")
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return false, fmt.Errorf("failed to decode hash: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false, fmt.Errorf("failed to decode salt: %w", err)
	}

	newHash := argon2.IDKey(
		[]byte(password+pepper),
		salt,
		argonTime,
		argonMemory,
		argonThreads,
		uint32(len(expectedHash)),
	)

	return subtle.ConstantTimeCompare(newHash, expectedHash) == 1, nil
}

// VerifyPasswordAny checks the current pepper first, then any previous pepper.
// It reports needsRehash when a previous pepper matched, so the caller can
// store the hash with the current pepper.
func VerifyPasswordAny(
	hash, password, current string,
	previous ...string,
) (ok, needsRehash bool, err error) {
	ok, err = VerifyPassword(hash, password, current)
	if err != nil || ok {
		return ok, false, err
	}

	for _, pepper := range Peppers(current, previous) {
		ok, err = VerifyPassword(hash, password, pepper)
		if err != nil {
			return false, false, err
		}
		if ok {
			return true, true, nil
		}
	}

	return false, false, nil
}

// Peppers trims previous peppers and drops empty values and the current one.
func Peppers(current string, previous []string) []string {
	peppers := make([]string, 0, len(previous))
	for _, pepper := range previous {
		pepper = strings.TrimSpace(pepper)
		if pepper == "" || pepper == current {
			continue
		}
		peppers = append(peppers, pepper)
	}

	return peppers
}

func generateSalt(size int) ([]byte, error) {
	salt := make([]byte, size)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	return salt, nil
}
