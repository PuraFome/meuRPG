// Package secret makes the random tokens that go into links and cookies,
// such as invite links, and the hashes the database keeps instead of them.
//
// The rule behind it (ADR-0002, ADR-0009): a secret that grants access is
// stored only as its SHA-256, so someone who reads the database, or a
// backup, cannot use it. A 256-bit random value needs no salt or slow hash:
// nobody can guess it, so there is nothing to brute-force.
package secret

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// Size is the number of random bytes in a secret: 256 bits from
// crypto/rand, far beyond guessing.
const Size = 32

// New returns a random secret, base64url-encoded without padding (43
// characters, safe in a URL fragment or a cookie), and the SHA-256 of its
// raw bytes, which is all the database stores.
func New() (value string, hash []byte) {
	raw := make([]byte, Size)
	// Since Go 1.24, crypto/rand.Read never returns an error: if the OS
	// cannot supply randomness, the program crashes instead of going on with
	// weak secrets.
	_, _ = rand.Read(raw)
	sum := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(raw), sum[:]
}

// Hash returns the hash that New returned for value. It returns false for
// anything New could not have produced (wrong length, padding, characters
// outside base64url), so garbage from a client never reaches the database.
func Hash(value string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(raw) != Size {
		return nil, false
	}
	sum := sha256.Sum256(raw)
	return sum[:], true
}
