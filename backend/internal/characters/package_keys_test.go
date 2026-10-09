package characters

import "testing"

// Every kind the table can create is a key a campaign package can carry: the feats
// were once refused on import because the pattern listed the kinds by hand.
func TestContentKeyShapeAcceptsEveryTableKind(t *testing.T) {
	t.Parallel()
	for _, tk := range tableKinds {
		key := tk.prefix + ":vigia-do-farol@mesa"
		if !contentKeyShape.MatchString(key) {
			t.Errorf("contentKeyShape refuses %q", key)
		}
	}
	for _, bad := range []string{"feat:vigia-do-farol", "monster:vigia@mesa", "feat:Vigia@mesa", "feat:@mesa"} {
		if contentKeyShape.MatchString(bad) {
			t.Errorf("contentKeyShape accepts %q", bad)
		}
	}
}
