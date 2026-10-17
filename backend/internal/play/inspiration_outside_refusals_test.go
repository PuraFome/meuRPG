package play

import (
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// These tests freeze what the Bardic Inspiration given out of a combat refuses, what each viewer
// reads of the dice held, and how the die moves onto a combatant and back (the inspiration_outside.go
// file). They need the database (MEURPG_TEST_DATABASE_URL).

// giveOutsideReq calls GiveBardicInspirationOutside as u with the request the test made.
func giveOutsideReq(t *testing.T, u *user, req *playv1.GiveBardicInspirationOutsideRequest) error {
	t.Helper()
	_, err := u.resource.GiveBardicInspirationOutside(t.Context(), connect.NewRequest(req))
	return err
}

// wantOutsideRefusal checks a failed precondition of the die given out of a combat: the reason
// and the exact message.
func wantOutsideRefusal(t *testing.T, call string, err error, reason playv1.ResourceBlockedReason, message string) {
	t.Helper()
	wantResourceBlocked(t, err, reason)
	if got := err.Error(); !strings.HasSuffix(got, ": "+message) {
		t.Errorf("%s message = %q, want it to end with %q", call, got, message)
	}
}

// wantOutsideCode checks the code and the exact message of a plain refusal.
func wantOutsideCode(t *testing.T, call string, err error, code connect.Code, message string) {
	t.Helper()
	wantCode(t, call, err, code)
	if got := err.Error(); got != fmt.Sprintf("%s: %s", code, message) {
		t.Errorf("%s message = %q, want %q", call, got, fmt.Sprintf("%s: %s", code, message))
	}
}

// Giving the die out of a combat: every refusal, in the order the checks run, with the code, the
// reason and the message.
func TestBardicInspirationOutsideRefusals(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	good := func() *playv1.GiveBardicInspirationOutsideRequest {
		return &playv1.GiveBardicInspirationOutsideRequest{CampaignId: a.campaignID, TargetCharacterId: a.toren.GetId(), IdempotencyKey: newKey()}
	}

	// Who may call: a member, with a valid key and a valid target.
	outsider := a.h.newUser("Intruso")
	wantOutsideCode(t, "an outsider", giveOutsideReq(t, outsider, good()), connect.CodeNotFound, "campaign not found")
	bad := good()
	bad.IdempotencyKey = ""
	wantOutsideCode(t, "no key", giveOutsideReq(t, a.bia, bad), connect.CodeInvalidArgument, "idempotency_key must be a UUID")
	bad = good()
	bad.TargetCharacterId = "not-a-uuid"
	wantOutsideCode(t, "a target that is not an id", giveOutsideReq(t, a.bia, bad), connect.CodeNotFound, "character not found")

	// The master has no character, so no feature.
	wantOutsideRefusal(t, "the master", giveOutsideReq(t, a.master, good()),
		playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NOT_AVAILABLE, "the character does not have this feature")
	// A target that is not a player of the table is not found: an NPC, a reserved character, an id that is not there.
	for name, target := range map[string]string{
		"an NPC":     a.goblin.GetId(),
		"a stranger": "00000000-0000-4000-8000-000000000001",
	} {
		req := good()
		req.TargetCharacterId = target
		wantOutsideCode(t, name, giveOutsideReq(t, a.bia, req), connect.CodeNotFound, "character not found")
	}
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE characters SET reserved = true, player_user_id = NULL, claimed_at = NULL WHERE id = $1`, a.pens.GetId()); err != nil {
		t.Fatalf("reserve a character: %v", err)
	}
	req := good()
	req.TargetCharacterId = a.pens.GetId()
	wantOutsideCode(t, "a reserved character", giveOutsideReq(t, a.bia, req), connect.CodeNotFound, "character not found")
	// The bard itself, and a die already held.
	req = good()
	req.TargetCharacterId = a.bri.GetId()
	wantOutsideRefusal(t, "the bard itself", giveOutsideReq(t, a.bia, req),
		playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_TARGET_REFUSED, "the creature cannot get a die")

	// The same request again by someone who did not make it.
	first := good()
	if err := giveOutsideReq(t, a.bia, first); err != nil {
		t.Fatalf("give error = %v", err)
	}
	wantOutsideCode(t, "the key of another's change", giveOutsideReq(t, a.master, first), connect.CodeInvalidArgument,
		"idempotency_key was already used for another change")
	// The retry by the bard changes nothing and reads the same.
	if err := giveOutsideReq(t, a.bia, first); err != nil {
		t.Fatalf("retry error = %v", err)
	}
	if got := a.usesLeft(t); got != 2 {
		t.Errorf("uses left = %d, want 2", got)
	}
}

// A combat in progress takes the die in it instead; the table without an open session has nothing to give in.
func TestBardicInspirationOutsideNeedsNoCombatAndAnOpenSession(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.resourceFight(t)
	err := giveOutsideReq(t, a.bia, &playv1.GiveBardicInspirationOutsideRequest{CampaignId: a.campaignID, TargetCharacterId: a.toren.GetId(), IdempotencyKey: newKey()})
	wantOutsideRefusal(t, "a combat running", err,
		playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_COMBAT_OPEN, "a combat is running: give the die in it")
	if got := a.outsideState(t, a.master); !got.GetCombatOpen() {
		t.Errorf("the master reads no combat open: %v", got)
	}
	if got := a.outsideState(t, a.caio); !got.GetCombatOpen() {
		t.Errorf("a player reads no combat open: %v", got)
	}
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	if got := a.outsideState(t, a.caio); got.GetCombatOpen() {
		t.Errorf("a player reads a combat open after it ended: %v", got)
	}
	a.endSession(t)
	err = giveOutsideReq(t, a.bia, &playv1.GiveBardicInspirationOutsideRequest{CampaignId: a.campaignID, TargetCharacterId: a.toren.GetId(), IdempotencyKey: newKey()})
	wantCode(t, "no open session", err, connect.CodeFailedPrecondition)
	if got := err.Error(); got != "failed_precondition: the campaign has no open game session" {
		t.Errorf("no open session message = %q", got)
	}
}

// outsideReads is what a viewer reads of the dice held out of a combat, with the names and not the ids.
func outsideReads(r *playv1.GetOutsideInspirationResponse) string {
	var b strings.Builder
	fmt.Fprintf(&b, "combat_open=%v bard=%v sides=%d uses=%d/%d", r.GetCombatOpen(), r.GetIsBard(), r.GetSides(), r.GetUsesLeft(), r.GetUsesMax())
	die := func(d *playv1.OutsideInspirationDie) string {
		if d == nil {
			return "-"
		}
		return fmt.Sprintf("%s:d%d from %q %ds", d.GetCharacterName(), d.GetSides(), d.GetFromName(), d.GetSecondsLeft())
	}
	fmt.Fprintf(&b, "\nmine=%s", die(r.GetMine()))
	for _, d := range r.GetHeld() {
		fmt.Fprintf(&b, "\nheld %s", die(d))
	}
	for _, o := range r.GetOffers() {
		fmt.Fprintf(&b, "\noffer d%d", o.GetSides())
	}
	for range r.GetLuckyOffers() {
		b.WriteString("\nlucky offer")
	}
	for _, tg := range r.GetTargets() {
		fmt.Fprintf(&b, "\ntarget %s refusal=%q", tg.GetName(), tg.GetDisabledReasonPt())
	}
	return b.String()
}

// What each viewer reads: the master every die, the holder its die and the question that waits for
// it, the bard its uses and who it may give to, a player with no die nothing, an outsider is refused.
func TestGetOutsideInspirationGoldenPerViewer(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	if _, err := a.giveOutside(t, a.bia, a.toren.GetId()); err != nil {
		t.Fatalf("give error = %v", err)
	}
	point, actions := a.h.newScene(a.mapID, "Ponte", true, 3, sceneSpec{key: "skill:perception", dc: new(int32(15))})
	a.openScene(t, point)
	if _, err := a.rollSceneApp(t, a.caio, actions[0], 7); err != nil {
		t.Fatalf("RollSceneCheck() error = %v", err)
	}
	for who, want := range map[string]string{
		"master": "combat_open=false bard=false sides=0 uses=0/0\nmine=-\nheld Tavo:d8 from \"Orla\" 600s",
		"holder": "combat_open=false bard=false sides=0 uses=0/0\nmine=Tavo:d8 from \"Orla\" 600s\noffer d8",
		"bard":   "combat_open=false bard=true sides=8 uses=2/3\nmine=-\ntarget Tavo refusal=\"Já tem um dado\"\ntarget Nael refusal=\"\"",
		// Nael's sorcerer has no Bardic Inspiration and holds no die: the read is the empty one.
		"other": "combat_open=false bard=false sides=0 uses=0/0\nmine=-",
	} {
		u := map[string]*user{"master": a.master, "holder": a.caio, "bard": a.bia, "other": a.ana}[who]
		got := outsideReads(a.outsideState(t, u))
		if got != want {
			t.Errorf("%s reads\n%s\nwant\n%s", who, got, want)
		}
	}
	outsider := a.h.newUser("Intruso")
	_, err := outsider.resource.GetOutsideInspiration(t.Context(), connect.NewRequest(&playv1.GetOutsideInspirationRequest{CampaignId: a.campaignID}))
	wantOutsideCode(t, "an outsider reading", err, connect.CodeNotFound, "campaign not found")
}

// The die goes onto the combatant with no giver when the stored one has none; a die with less than a
// round left does not come in and is gone; a player's combatant that leaves in the setup (the
// only time it may) loses its die, and the one that stays keeps its own.
func TestBardicInspirationOutsideIntoACombatWithoutItsGiverOrWithLessThanARound(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	for _, target := range []string{a.toren.GetId(), a.pens.GetId()} {
		if _, err := a.giveOutside(t, a.bia, target); err != nil {
			t.Fatalf("give error = %v", err)
		}
	}
	// Tavo's die has no giver; Orla's has 5 seconds left: less than a round.
	for _, q := range []struct{ sql, id string }{
		{`UPDATE character_inspiration SET source_character_id = NULL WHERE character_id = $1`, a.toren.GetId()},
		{`INSERT INTO character_inspiration (character_id, campaign_id, sides, seconds_left, created_at)
			SELECT id, campaign_id, 6, 5, now() FROM characters WHERE id = $1`, a.bri.GetId()},
	} {
		if _, err := a.h.pool.Exec(t.Context(), q.sql, q.id); err != nil {
			t.Fatalf("prepare the dice: %v", err)
		}
	}
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3}, setup: true,
	})
	tavo := byLabel(t, a.get(t, a.master), "Tavo").GetInspirationDie()
	if tavo == nil || tavo.GetSides() != 8 || tavo.GetFromLabel() != "" || tavo.GetExpiresAtRound() != 101 {
		t.Errorf("Tavo's die = %v, want a d8 with no giver that ends in round 101", tavo)
	}
	if nael := byLabel(t, a.get(t, a.master), "Nael").GetInspirationDie(); nael == nil || nael.GetFromLabel() != "Orla" {
		t.Errorf("Nael's die = %v, want the d8 from Orla", nael)
	}
	if orla := byLabel(t, a.get(t, a.master), "Orla").GetInspirationDie(); orla != nil {
		t.Errorf("Orla's die = %v, want none: it had less than a round", orla)
	}
	if n := a.dieRows(t, a.bri.GetId()); n != 0 {
		t.Errorf("%d dice left on Orla, want 0: the one that ran out is gone", n)
	}
	// A combatant that leaves in the setup loses its die; the one that stays keeps its own.
	if _, err := a.master.combat.RemoveCombatant(t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Tavo"), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("RemoveCombatant() error = %v", err)
	}
	// Finding: the player's combatant leaves in the setup by DeleteCombatant alone (combat.go RemoveCombatant), so the
	// die it holds is lost: it does not go back to the character (dieToCharacters only runs for the ones that
	// dropCombatants takes out, which no die holds).
	if n := a.dieRows(t, a.toren.GetId()); n != 0 {
		t.Errorf("%d dice back on Tavo, want 0: the die goes with the combatant", n)
	}
	if n := a.dieRows(t, a.pens.GetId()); n != 0 {
		t.Errorf("%d dice back on Nael, who is still in the combat, want 0", n)
	}
	if nael := byLabel(t, a.get(t, a.master), "Nael").GetInspirationDie(); nael == nil {
		t.Error("Nael lost her die when Tavo left")
	}
}
