// Package apikey provides Argon2id hashing and verification for API keys.
package apikey

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	prefixLen = 16 // hex chars (8 bytes) stored plaintext for DB lookup

	argon2Time    uint32 = 3
	argon2Memory  uint32 = 64 * 1024 // 64 MB
	argon2Threads uint8  = 4
	argon2KeyLen  uint32 = 32
	argon2SaltLen        = 16
)

// Prefix returns the first 16 hex characters of key, used as the DB lookup index.
func Prefix(key string) string {
	if len(key) < prefixLen {
		return key
	}
	return key[:prefixLen]
}

// Hash derives an Argon2id hash of key and returns the encoded string (salt included).
func Hash(key string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	h := argon2.IDKey([]byte(key), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	encoded := base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(h)
	return encoded, nil
}

// Verify returns true if key matches the encoded Argon2id hash produced by Hash.
func Verify(key, encoded string) bool {
	parts := strings.SplitN(encoded, "$", 2)
	if len(parts) != 2 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil || len(salt) != argon2SaltLen {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(key), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return subtle.ConstantTimeCompare(got, expected) == 1
}
