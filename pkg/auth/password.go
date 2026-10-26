// Package auth provides password hashing and token primitives for Andurel
// applications.
//
// Password hashes use the argon2id PHC string format
// ($argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>). Legacy
// "base64hash:base64salt" hashes are still verified so applications can rehash
// them with the current parameters on the next successful login.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

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
// result is a PHC string with RawStd base64 salt and hash.
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

	return formatPHC(argonMemory, argonTime, argonThreads, salt, hash), nil
}

// dummyHash is computed once, on the first not-found login, so the dummy
// verification costs the same as a real one without paying for a fresh hash on
// every request.
var dummyHash = sync.OnceValue(func() string {
	hash, err := HashPassword("andurel-dummy-password", "andurel-dummy-pepper")
	if err != nil {
		return ""
	}

	return hash
})

// DummyVerify runs one full argon2id verification against a fixed package
// hash and discards the result. Call it on the unknown-user login path so both
// paths spend the same time verifying a password.
func DummyVerify(password, pepper string) {
	hash := dummyHash()
	if hash == "" {
		return
	}

	_, _ = VerifyPassword(hash, password, pepper)
}

// VerifyPassword reports whether password matches hash under pepper. It
// accepts PHC hashes and the legacy "base64hash:base64salt" format.
func VerifyPassword(hash, password, pepper string) (bool, error) {
	parsed, err := parsePasswordHash(hash)
	if err != nil {
		return false, err
	}

	return verifyParsed(parsed, password, pepper), nil
}

// VerifyPasswordAny checks the current pepper first, then any previous pepper.
// It reports needsRehash when the stored hash is legacy, uses different argon2
// parameters, or matched a previous pepper, so the caller can store the hash
// with the current format and pepper.
func VerifyPasswordAny(
	hash, password, current string,
	previous ...string,
) (ok, needsRehash bool, err error) {
	parsed, err := parsePasswordHash(hash)
	if err != nil {
		return false, false, err
	}

	rehash := parsed.needsRehash()

	if verifyParsed(parsed, password, current) {
		return true, rehash, nil
	}

	for _, pepper := range Peppers(current, previous) {
		if verifyParsed(parsed, password, pepper) {
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

type passwordHash struct {
	memory  uint32
	time    uint32
	threads uint8
	keyLen  uint32
	salt    []byte
	hash    []byte
	legacy  bool
}

// needsRehash reports whether the hash should be rewritten with HashPassword.
func (p passwordHash) needsRehash() bool {
	return p.legacy ||
		p.memory != argonMemory ||
		p.time != argonTime ||
		p.threads != argonThreads ||
		p.keyLen != argonKeyLen
}

func verifyParsed(parsed passwordHash, password, pepper string) bool {
	computed := argon2.IDKey(
		[]byte(password+pepper),
		parsed.salt,
		parsed.time,
		parsed.memory,
		parsed.threads,
		parsed.keyLen,
	)

	return subtle.ConstantTimeCompare(computed, parsed.hash) == 1
}

func formatPHC(
	memory uint32,
	time uint32,
	threads uint8,
	salt, hash []byte,
) string {
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		memory,
		time,
		threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
}

func parsePasswordHash(encoded string) (passwordHash, error) {
	if strings.HasPrefix(encoded, "$argon2id$") {
		return parsePHC(encoded)
	}

	return parseLegacy(encoded)
}

func parseLegacy(encoded string) (passwordHash, error) {
	parts := strings.Split(encoded, ":")
	if len(parts) != 2 {
		return passwordHash{}, fmt.Errorf("invalid stored password format")
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return passwordHash{}, fmt.Errorf("failed to decode hash: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return passwordHash{}, fmt.Errorf("failed to decode salt: %w", err)
	}
	if len(hash) == 0 || len(salt) == 0 {
		return passwordHash{}, fmt.Errorf("invalid stored password format")
	}

	return passwordHash{
		memory:  argonMemory,
		time:    argonTime,
		threads: argonThreads,
		keyLen:  uint32(len(hash)),
		salt:    salt,
		hash:    hash,
		legacy:  true,
	}, nil
}

func parsePHC(encoded string) (passwordHash, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return passwordHash{}, fmt.Errorf("invalid stored password format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return passwordHash{}, fmt.Errorf("invalid argon2 version: %w", err)
	}
	if version != argon2.Version {
		return passwordHash{}, fmt.Errorf("unsupported argon2 version %d", version)
	}

	var (
		memory     uint32
		iterations uint32
		threads    uint8
	)
	if _, err := fmt.Sscanf(
		parts[3],
		"m=%d,t=%d,p=%d",
		&memory,
		&iterations,
		&threads,
	); err != nil {
		return passwordHash{}, fmt.Errorf("invalid argon2 parameters: %w", err)
	}
	if memory == 0 || iterations == 0 || threads == 0 {
		return passwordHash{}, fmt.Errorf("invalid argon2 parameters")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return passwordHash{}, fmt.Errorf("failed to decode salt: %w", err)
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return passwordHash{}, fmt.Errorf("failed to decode hash: %w", err)
	}
	if len(salt) == 0 || len(hash) == 0 {
		return passwordHash{}, fmt.Errorf("invalid stored password format")
	}

	return passwordHash{
		memory:  memory,
		time:    iterations,
		threads: threads,
		keyLen:  uint32(len(hash)),
		salt:    salt,
		hash:    hash,
	}, nil
}

func generateSalt(size int) ([]byte, error) {
	salt := make([]byte, size)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	return salt, nil
}
