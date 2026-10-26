package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"math/big"
)

const codeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateCode returns n characters drawn uniformly from codeAlphabet.
// rand.Int avoids the bias a byte modulo the alphabet size would add.
func GenerateCode(n int) (string, error) {
	limit := big.NewInt(int64(len(codeAlphabet)))
	b := make([]byte, n)
	for i := range b {
		index, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		b[i] = codeAlphabet[index.Int64()]
	}

	return string(b), nil
}

// GenerateToken returns a URL-safe token backed by 15 random bytes.
func GenerateToken() (string, error) {
	b := make([]byte, 15)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// HashToken derives the stored hash for a plaintext token with HMAC-SHA256.
func HashToken(plain, secret string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(plain))

	return hex.EncodeToString(m.Sum(nil))
}

// VerifyToken compares a plaintext token against a stored hash in constant
// time.
func VerifyToken(plain, secret, hash string) bool {
	expected := HashToken(plain, secret)

	return hmac.Equal([]byte(expected), []byte(hash))
}
