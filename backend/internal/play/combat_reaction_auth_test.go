package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// TestReactionNamesAreTheOfficialOnes: the names the combat log and the prompts write are the
// Portuguese names of the content (names_pt.json), not a second spelling.
func TestReactionNamesAreTheOfficialOnes(t *testing.T) {
	t.Parallel()
	content, err := testRules()
	if err != nil {
		t.Fatalf("rules.LoadSRD() error = %v", err)
	}
	for kind, key := range reactionNameKeys {
		if got, want := reactionNames[kind], content.NamePT(key); got != want || want == "" {
			t.Errorf("%s is %q in the combat, %q in the content (%s)", kind, got, want, key)
		}
	}
}

// TestReactionWindowAnswersAreTheReactorsAndTheMasters: an NPC's window is the master's alone
// (a player, from any seat, and a stranger are refused the same way); a player's own window is
// that player's and the master's, and no other player's; and a concentration save is its
// owner's and the master's. A refusal reads the same for a window that does not exist (RN-10).
func TestReactionWindowAnswersAreTheReactorsAndTheMasters(t *testing.T) {
	t.Parallel()
	a, e, shield := shieldedHit(t)
	stranger := a.h.newUser("Eva")
	pass := func(u *user, id string) error {
		_, err := a.answerReaction(t, u, e, id, passAnswer)
		return err
	}
	fake := "00000000-0000-4000-8000-000000000001"
	for name, u := range map[string]*user{"Toren's player": a.caio, "Pensantus's player": a.ana, "Brisa's player": a.bia} {
		wantCode(t, name+" answering the Mago's window", pass(u, shield.GetId()), connect.CodePermissionDenied)
		wantCode(t, name+" answering a window that does not exist", pass(u, fake), connect.CodePermissionDenied)
	}
	if err := pass(stranger, shield.GetId()); err == nil {
		t.Error("a stranger answered a window")
	} else if c := connect.CodeOf(err); c != connect.CodeNotFound && c != connect.CodePermissionDenied {
		t.Errorf("a stranger's refusal = %v", err)
	}
	if _, err := a.resolveConcentration(t, stranger, e, fake, keepConcentration); err == nil {
		t.Error("a stranger resolved a concentration save")
	}
	if err := pass(a.master, shield.GetId()); err != nil {
		t.Errorf("the master answering the Mago's window error = %v", err)
	}

	// A player's own window: the owner and the master, not the other players.
	r, re := reactorFight(t, rogueFive)
	hit := r.mustAttack(t, r.master, re, "Goblin", sword, "Reator", d20(15))
	r.mustDamage(t, r.master, re, hit.GetPendingDamage().GetId(), typedDamage(6))
	w := r.windowOf(t, r.ana, playv1.ReactionKind_REACTION_KIND_UNCANNY_DODGE)
	if w == nil {
		t.Fatal("the rogue has no Uncanny Dodge window")
	}
	for name, u := range map[string]*user{"Toren's player": r.caio, "Brisa's player": r.bia} {
		_, err := r.answerReaction(t, u, re, w.GetId(), passAnswer)
		wantCode(t, name+" answering the rogue's window", err, connect.CodePermissionDenied)
		if len(r.windows(t, u)) != 0 {
			t.Errorf("%s reads the rogue's window", name)
		}
	}
	// The master answers for the player, and the window says so to its owner.
	if _, err := r.answerReaction(t, r.master, re, w.GetId(), useAnswer(0)); err != nil {
		t.Errorf("the master answering for the rogue error = %v", err)
	}
}
