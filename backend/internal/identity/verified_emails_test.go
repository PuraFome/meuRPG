package identity

import (
	"slices"
	"testing"
)

// campaigns reads the verified e-mails for the allow-list of who may create
// campaigns (RN-30): an unverified e-mail is never stored, so it never shows.
func TestPostgresStoreVerifiedEmails(t *testing.T) {
	t.Parallel()
	store := NewPostgresStore(testPool(t))
	ctx := t.Context()

	verified, err := store.UpsertUser(ctx, ExternalIdentity{Issuer: "https://idp.example", Subject: "ve-1", Email: "Ana@Example.com"})
	if err != nil {
		t.Fatalf("UpsertUser() error = %v", err)
	}
	unverified, err := store.UpsertUser(ctx, ExternalIdentity{Issuer: "https://idp.example", Subject: "ve-2"})
	if err != nil {
		t.Fatalf("UpsertUser() error = %v", err)
	}

	if got, err := store.VerifiedEmails(ctx, verified); err != nil || !slices.Equal(got, []string{"Ana@Example.com"}) {
		t.Errorf("VerifiedEmails(verified) = %v, %v", got, err)
	}
	if got, err := store.VerifiedEmails(ctx, unverified); err != nil || len(got) != 0 {
		t.Errorf("VerifiedEmails(unverified) = %v, %v; want none", got, err)
	}
	// The next sign-in refreshes it: an e-mail that is no longer verified goes away.
	if _, err := store.UpsertUser(ctx, ExternalIdentity{Issuer: "https://idp.example", Subject: "ve-1"}); err != nil {
		t.Fatalf("UpsertUser() error = %v", err)
	}
	if got, err := store.VerifiedEmails(ctx, verified); err != nil || len(got) != 0 {
		t.Errorf("VerifiedEmails after the e-mail stopped being verified = %v, %v; want none", got, err)
	}
}
