package identity

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestTokenAuthStyle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		methods []string
		want    oauth2.AuthStyle
		wantErr bool
	}{
		{methods: nil, want: oauth2.AuthStyleInHeader}, // the spec's default
		{methods: []string{"client_secret_basic"}, want: oauth2.AuthStyleInHeader},
		{methods: []string{"client_secret_post", "client_secret_basic"}, want: oauth2.AuthStyleInHeader},
		{methods: []string{"private_key_jwt", "client_secret_post"}, want: oauth2.AuthStyleInParams},
		{methods: []string{"private_key_jwt", "tls_client_auth"}, wantErr: true},
	}
	for _, tt := range tests {
		got, err := tokenAuthStyle(tt.methods)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("tokenAuthStyle(%v) = %v, %v; want %v, error %v", tt.methods, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestFlexibleBool(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]bool{
		`true`: true, `"true"`: true,
		`false`: false, `"false"`: false, `"TRUE"`: false, `1`: false, `null`: false, `"yes"`: false,
	} {
		var b flexibleBool
		if err := json.Unmarshal([]byte(raw), &b); err != nil {
			t.Errorf("unmarshal %s: %v", raw, err)
			continue
		}
		if bool(b) != want {
			t.Errorf("flexibleBool(%s) = %v, want %v", raw, b, want)
		}
	}
}

func TestNumericDate(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]time.Time{
		`1790000000`:     time.Unix(1790000000, 0),
		`1790000000.75`:  time.Unix(1790000000, 0),
		``:               {},
		`null`:           {},
		`"1790000000"`:   {}, // a string is not a NumericDate
		`-5`:             {},
		`0`:              {},
		`1e20`:           {}, // past year 9999
		`{"not":"time"}`: {},
	} {
		if got := numericDate(json.RawMessage(raw)); !got.Equal(want) {
			t.Errorf("numericDate(%s) = %v, want %v", raw, got, want)
		}
	}
}

func TestSecrets(t *testing.T) {
	t.Parallel()

	value, hash := newSecret()
	if len(value) != 43 || len(hash) != 32 {
		t.Fatalf("newSecret() = %d chars, %d-byte hash; want 43 and 32", len(value), len(hash))
	}
	if got, ok := hashSecret(value); !ok || string(got) != string(hash) {
		t.Errorf("hashSecret(newSecret()) does not give back the stored hash")
	}
	if other, _ := newSecret(); other == value {
		t.Error("newSecret() returned the same value twice")
	}

	for _, bad := range []string{
		"",
		"short",
		value + "A", // 33 bytes
		value[:42],  // truncated
		strings.ReplaceAll(value, value[:1], "+"), // not base64url
		base64.RawURLEncoding.EncodeToString(make([]byte, 31)),
		base64.URLEncoding.EncodeToString(make([]byte, 32)), // padded
	} {
		if _, ok := hashSecret(bad); ok {
			t.Errorf("hashSecret(%q) accepted a value newSecret cannot produce", bad)
		}
	}
}
