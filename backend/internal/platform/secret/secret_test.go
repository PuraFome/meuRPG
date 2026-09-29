package secret

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestNewAndHash(t *testing.T) {
	t.Parallel()

	value, hash := New()
	if len(value) != 43 {
		t.Errorf("len(value) = %d, want 43 (32 bytes in unpadded base64url)", len(value))
	}
	if strings.ContainsAny(value, "+/=") {
		t.Errorf("value %q is not unpadded base64url", value)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(value)
	if want := sha256.Sum256(raw); !bytes.Equal(hash, want[:]) {
		t.Errorf("hash is not the SHA-256 of the raw bytes")
	}

	got, ok := Hash(value)
	if !ok || !bytes.Equal(got, hash) {
		t.Errorf("Hash(New()) = %x, %v; want %x, true", got, ok, hash)
	}

	other, _ := New()
	if other == value {
		t.Error("two calls to New returned the same secret")
	}
}

func TestHashRejectsWhatNewCannotProduce(t *testing.T) {
	t.Parallel()
	value, _ := New()
	for name, input := range map[string]string{
		"empty":     "",
		"too short": value[:42],
		"too long":  value + "A",
		"padded":    base64.URLEncoding.EncodeToString(make([]byte, Size)),
		// 0xfb bytes encode as "+/v7..." in standard base64.
		"standard base64": base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{0xfb}, Size)),
		"spaces":          " " + value,
		"not base64":      strings.Repeat("!", 43),
	} {
		if _, ok := Hash(input); ok {
			t.Errorf("%s: Hash(%q) = ok, want rejected", name, input)
		}
	}
}
