package play

import (
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Characterization of Revivify outside a combat (revivify_requests.go) and of the master's
// switch (SetRevivifyBlocked), written before the function split: each refusal with its exact
// code and message, what each person reads, and the answer's replays. A "finding" comment marks a
// behavior that looks wrong and is kept on purpose. The helpers are in revivify_frozen_test.go.

const (
	noCharacterMsg  = "character not found"
	notRequestMsg   = "revivify request not found"
	noSessionMsg    = "the campaign has no open game session"
	notYoursMsg     = "only the character's player or the master may do this"
	notIdempotentOK = "idempotency_key must be a UUID"
)

func (a *armed) endRevivifySession(t *testing.T) {
	t.Helper()
	a.master.end(t, a.master.liveSession(t, a.campaignID).GetGameSession())
}

func (a *armed) previewOutsideRaw(t *testing.T, u *user, caster string) (*playv1.PreviewRevivifyResponse, error) {
	t.Helper()
	return a.previewRaw(t, u, &playv1.PreviewRevivifyRequest{CampaignId: a.campaignID, CasterCharacterId: caster})
}

func (a *armed) switchRaw(t *testing.T, target any, blocked bool) error {
	t.Helper()
	req := &playv1.SetRevivifyBlockedRequest{CampaignId: a.campaignID, Blocked: blocked}
	switch v := target.(type) {
	case playv1.SetRevivifyBlockedRequest_CombatantId:
		req.Target = &v
	case playv1.SetRevivifyBlockedRequest_CharacterId:
		req.Target = &v
	}
	_, err := a.master.revivify.SetRevivifyBlocked(t.Context(), connect.NewRequest(req))
	return err
}

func switchCombatant(id string) playv1.SetRevivifyBlockedRequest_CombatantId {
	return playv1.SetRevivifyBlockedRequest_CombatantId{CombatantId: id}
}

func switchCharacter(id string) playv1.SetRevivifyBlockedRequest_CharacterId {
	return playv1.SetRevivifyBlockedRequest_CharacterId{CharacterId: id}
}

// requestLine is a compact golden of the requests a person reads.
func requestLine(rs []*playv1.RevivifyRequest) string {
	var parts []string
	for _, r := range rs {
		parts = append(parts, fmt.Sprintf("%s>%s %s slot=%d/%v left=%d answered=%v", r.GetCasterName(), r.GetTargetName(),
			strings.TrimPrefix(r.GetStatus().String(), "REVIVIFY_REQUEST_STATUS_"), r.GetSlot().GetLevel(), r.GetSlot().GetPact(), r.GetSlotsLeft(), r.GetAnsweredAt() != nil))
	}
	return strings.Join(parts, "; ")
}

// The preview with no combat: who may ask, and what each person reads.
func TestRevivifyFrozen_OutsidePreview(t *testing.T) {
	t.Parallel()
	a := newVigil(t)
	for call, id := range map[string]string{"a bad id": "nope", "a character that is not there": newKey(), "an NPC": a.goblin.GetId()} {
		_, err := a.previewOutsideRaw(t, a.master, id)
		wantRefusal(t, "PreviewRevivify with "+call, err, connect.CodeNotFound, noCharacterMsg)
	}
	_, err := a.previewOutsideRaw(t, a.caio, a.pens.GetId())
	wantRefusal(t, "another player's character", err, connect.CodePermissionDenied, notYoursMsg)
	_, err = a.previewOutsideRaw(t, a.bia, a.bri.GetId())
	wantRefusal(t, "a caster with no Revivify", err, connect.CodeInvalidArgument, "spell_key is not one of the caster's spells")

	// Two dead characters, one of them marked by the master.
	if _, err := a.master.characters.MarkCharacterDead(t.Context(), connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: a.campaignID, CharacterId: a.bri.GetId()})); err != nil {
		t.Fatalf("MarkCharacterDead() error = %v", err)
	}
	if err := a.switchRaw(t, switchCharacter(a.bri.GetId()), true); err != nil {
		t.Fatalf("SetRevivifyBlocked() error = %v", err)
	}
	pv, err := a.previewOutsideRaw(t, a.ana, a.pens.GetId())
	wantRevivifyLine(t, "the caster's player", pv, err, "slot=3/2 time=false targets=[Toren rnil snil] unavailable=[]")
	pv, err = a.previewOutsideRaw(t, a.master, a.pens.GetId())
	wantRevivifyLine(t, "the master", pv, err, "slot=3/2 time=false targets=[Toren rnil snil] unavailable=[Brisa MASTER_BLOCKED rnil s0]")
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) {
		r.SpellSlotsUsed = []*playv1.SpellSlotsUsed{{Level: 3, Used: 2}}
	})
	pv, err = a.previewOutsideRaw(t, a.ana, a.pens.GetId())
	wantRevivifyLine(t, "the caster's player with no slot", pv, err, "slot=0/0 time=false targets=[Toren rnil snil] unavailable=[]")
}

// RequestRevivify: the refusals in the order they are checked, who may ask, and what each person
// reads of the requests.
func TestRevivifyFrozen_RequestRefusalsAndWhoReadsThem(t *testing.T) {
	t.Parallel()
	a := newVigil(t)
	ask := func(u *user, edit func(*playv1.RequestRevivifyRequest)) error {
		t.Helper()
		var edits []func(*playv1.RequestRevivifyRequest)
		if edit != nil {
			edits = append(edits, edit)
		}
		_, err := a.ask(u, a.pens, a.toren, newKey(), edits...)
		return err
	}
	wantRefusal(t, "a bad key", ask(a.ana, func(r *playv1.RequestRevivifyRequest) { r.IdempotencyKey = "nope" }), connect.CodeInvalidArgument, notIdempotentOK)
	wantRefusal(t, "no diamonds", ask(a.ana, func(r *playv1.RequestRevivifyRequest) { r.MaterialConfirmed = false }), connect.CodeInvalidArgument,
		"material_confirmed is required: the spell consumes diamonds worth 300 gp")
	wantRefusal(t, "a bad target", ask(a.ana, func(r *playv1.RequestRevivifyRequest) { r.TargetCharacterId = "nope" }), connect.CodeNotFound, noCharacterMsg)
	// The key is read before the diamonds, and the diamonds before the target.
	wantRefusal(t, "a bad key and no diamonds", ask(a.ana, func(r *playv1.RequestRevivifyRequest) {
		r.IdempotencyKey, r.MaterialConfirmed, r.TargetCharacterId = "nope", false, "nope"
	}), connect.CodeInvalidArgument, notIdempotentOK)
	wantRefusal(t, "a caster that is not there", ask(a.ana, func(r *playv1.RequestRevivifyRequest) { r.CasterCharacterId = newKey() }), connect.CodeNotFound, noCharacterMsg)
	wantRefusal(t, "a bad caster", ask(a.ana, func(r *playv1.RequestRevivifyRequest) { r.CasterCharacterId = "nope" }), connect.CodeNotFound, noCharacterMsg)
	wantRefusal(t, "an NPC as the caster", ask(a.master, func(r *playv1.RequestRevivifyRequest) { r.CasterCharacterId = a.goblin.GetId() }), connect.CodeNotFound, noCharacterMsg)
	wantRefusal(t, "another player's caster", ask(a.caio, nil), connect.CodePermissionDenied, notYoursMsg)
	wantRefusal(t, "a caster with no Revivify", ask(a.bia, func(r *playv1.RequestRevivifyRequest) { r.CasterCharacterId = a.bri.GetId() }), connect.CodeInvalidArgument,
		"spell_key is not one of the caster's spells")
	slot := func(call string, s *playv1.SpellSlot, msg string) {
		t.Helper()
		wantRefusal(t, call, ask(a.ana, func(r *playv1.RequestRevivifyRequest) { r.Slot = s }), connect.CodeInvalidArgument, msg)
	}
	slot("no slot", nil, revivifyNoSlotMsg)
	slot("a 2nd level slot", slotOfLevel(2), revivifyNoSlotMsg)
	slot("a pact slot the caster does not have", &playv1.SpellSlot{Level: 3, Pact: true}, "slot is not one the caster has free")
	wantRefusal(t, "a character that is alive", ask(a.ana, func(r *playv1.RequestRevivifyRequest) { r.TargetCharacterId = a.bri.GetId() }), connect.CodeNotFound, noCharacterMsg)

	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) {
		r.SpellSlotsUsed = []*playv1.SpellSlotsUsed{{Level: 3, Used: 2}}
	})
	err := ask(a.ana, nil)
	wantRefusal(t, "no slot left", err, connect.CodeFailedPrecondition, "there is no free spell slot for this spell")
	wantBlockedBy(t, "no slot left", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_SLOT)
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) {
		r.SpellSlotsUsed = []*playv1.SpellSlotsUsed{{Level: 3, Used: 0}}
	})

	// The master asks for the player's caster: it is his request, and the player does not read it.
	mine, err := a.ask(a.master, a.pens, a.toren, newKey())
	if err != nil {
		t.Fatalf("the master's RequestRevivify error = %v", err)
	}
	if got, want := requestLine([]*playv1.RevivifyRequest{mine}), "Ilaria>Toren PENDING slot=3/false left=0 answered=false"; got != want {
		t.Errorf("the master's request = %s, want %s", got, want)
	}
	if got := a.requests(t, a.ana); len(got) != 0 {
		t.Errorf("the caster's player reads the master's request %v: a player reads the casts they made", got)
	}
	_, err = a.ask(a.ana, a.pens, a.toren, newKey())
	if err != nil {
		t.Fatalf("RequestRevivify() error = %v", err)
	}
	if got, want := requestLine(a.requests(t, a.ana)), "Ilaria>Toren PENDING slot=3/false left=0 answered=false"; got != want {
		t.Errorf("the caster's player reads %s, want %s", got, want)
	}
	if got := len(a.requests(t, a.master)); got != 2 {
		t.Errorf("the master reads %d requests, want 2", got)
	}

	// The master answers someone else's request only as the master.
	wantRefusal(t, "a player answering", func() error { _, e := a.answerRevivify(t, a.ana, mine.GetId(), false, newKey()); return e }(), connect.CodeNotFound, notRequestMsg)
}

// A cast outside a combat is refused for a caster that is in one.
func TestRevivifyFrozen_RequestRefusedForACasterInACombat(t *testing.T) {
	t.Parallel()
	a, e := newMortuary(t)
	a.dies(t, e, "Toren", a.toren)
	_, err := a.ask(a.ana, a.pens, a.toren, newKey())
	wantRefusal(t, "a request while the caster is in a combat", err, connect.CodeInvalidArgument, "the caster is in a combat: cast Revivify inside it")
	_, err = a.ask(a.master, a.pens, a.toren, newKey())
	wantRefusal(t, "the master's request while the caster is in a combat", err, connect.CodeInvalidArgument, "the caster is in a combat: cast Revivify inside it")
	// A caster that is not in the combat may ask.
	if _, err := a.ask(a.bia, a.bri, a.toren, newKey()); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a fighter's request error = %v, want the spell refusal", err)
	}
}

// Without an open session nothing is asked, answered or listed; a request of an earlier session is
// not found in the next one.
func TestRevivifyFrozen_RequestsNeedTheOpenSession(t *testing.T) {
	t.Parallel()
	a := newVigil(t)
	req, err := a.ask(a.ana, a.pens, a.toren, newKey())
	if err != nil {
		t.Fatalf("RequestRevivify() error = %v", err)
	}
	a.endRevivifySession(t)
	_, err = a.ask(a.ana, a.pens, a.toren, newKey())
	wantRefusal(t, "a request with no session", err, connect.CodeFailedPrecondition, noSessionMsg)
	_, err = a.answerRevivify(t, a.master, req.GetId(), true, newKey())
	wantRefusal(t, "an answer with no session", err, connect.CodeFailedPrecondition, noSessionMsg)
	_, err = a.master.revivify.ListRevivifyRequests(t.Context(), connect.NewRequest(&playv1.ListRevivifyRequestsRequest{CampaignId: a.campaignID}))
	wantRefusal(t, "the list with no session", err, connect.CodeFailedPrecondition, noSessionMsg)

	a.master.start(t, a.campaignID)
	_, err = a.answerRevivify(t, a.master, req.GetId(), true, newKey())
	wantRefusal(t, "an answer to a request of the session before", err, connect.CodeNotFound, notRequestMsg)
	if got := a.requests(t, a.master); len(got) != 0 {
		t.Errorf("the new session lists %v, want none", got)
	}
}

// ConfirmRevivifyTime: the refusals in order, the answers and their replays.
func TestRevivifyFrozen_ConfirmRefusalsAndReplays(t *testing.T) {
	t.Parallel()
	a := newVigil(t)
	req, err := a.ask(a.ana, a.pens, a.toren, newKey())
	if err != nil {
		t.Fatalf("RequestRevivify() error = %v", err)
	}
	answer := func(u *user, id string, within bool, key string) error {
		t.Helper()
		_, err := a.answerRevivify(t, u, id, within, key)
		return err
	}
	// A player is told the request is not there before anything else is read, even for a bad key.
	wantRefusal(t, "a player with a bad key", answer(a.ana, req.GetId(), true, "nope"), connect.CodeNotFound, notRequestMsg)
	wantRefusal(t, "a bad id", answer(a.master, "nope", true, newKey()), connect.CodeNotFound, notRequestMsg)
	wantRefusal(t, "a bad id with a bad key", answer(a.master, "nope", true, "nope"), connect.CodeNotFound, notRequestMsg)
	wantRefusal(t, "a bad key", answer(a.master, req.GetId(), true, "nope"), connect.CodeInvalidArgument, notIdempotentOK)
	wantRefusal(t, "a request that is not there", answer(a.master, newKey(), true, newKey()), connect.CodeNotFound, notRequestMsg)

	// Not enough slots when the master says yes: nothing is spent and the request still waits.
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) {
		r.SpellSlotsUsed = []*playv1.SpellSlotsUsed{{Level: 3, Used: 2}}
	})
	err = answer(a.master, req.GetId(), true, newKey())
	if err == nil {
		t.Fatalf("ConfirmRevivifyTime with no slot left succeeded")
	}
	t.Logf("finding: the master's yes with no slot left answers %v", err)
	if got, want := requestLine(a.requests(t, a.master)), "Ilaria>Toren PENDING slot=3/false left=0 answered=false"; got != want {
		t.Errorf("the request after the refusal = %s, want %s", got, want)
	}
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) {
		r.SpellSlotsUsed = []*playv1.SpellSlotsUsed{{Level: 3, Used: 0}}
	})

	// "Já passou": denied, and the same key again reads the same answer; the same key with the
	// other answer is not the same request.
	k := newKey()
	denied, err := a.answerRevivify(t, a.master, req.GetId(), false, k)
	if err != nil {
		t.Fatalf("ConfirmRevivifyTime(false) error = %v", err)
	}
	if got, want := requestLine([]*playv1.RevivifyRequest{denied}), "Ilaria>Toren DENIED slot=3/false left=0 answered=true"; got != want {
		t.Errorf("the denied request = %s, want %s", got, want)
	}
	err = answer(a.master, req.GetId(), true, k)
	t.Logf("the same key with the other answer answers %v", err)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("the same key with the other answer error = %v, want invalid_argument", err)
	}
	wantRefusal(t, "a new key after the answer", answer(a.master, req.GetId(), false, newKey()), connect.CodeFailedPrecondition, "the request was answered already")

	// "Faz menos de 1 minuto": confirmed with the slots left; the replay reads the same request.
	second, err := a.ask(a.ana, a.pens, a.toren, newKey())
	if err != nil {
		t.Fatalf("second RequestRevivify() error = %v", err)
	}
	k2 := newKey()
	done, err := a.answerRevivify(t, a.master, second.GetId(), true, k2)
	if err != nil {
		t.Fatalf("ConfirmRevivifyTime(true) error = %v", err)
	}
	if got, want := requestLine([]*playv1.RevivifyRequest{done}), "Ilaria>Toren CONFIRMED slot=3/false left=1 answered=true"; got != want {
		t.Errorf("the confirmed request = %s, want %s", got, want)
	}
	again, err := a.answerRevivify(t, a.master, second.GetId(), true, k2)
	if err != nil {
		t.Fatalf("the replay of ConfirmRevivifyTime error = %v", err)
	}
	// Finding: the replay of a confirmation reads slots_left 0, where the first answer read the
	// slots that were left (the count is worked out from the spend, which a replay does not make).
	if got, want := requestLine([]*playv1.RevivifyRequest{again}), "Ilaria>Toren CONFIRMED slot=3/false left=0 answered=true"; got != want {
		t.Errorf("the replayed confirmation = %s, want %s", got, want)
	}
	if v := a.vitals(t, a.pens); usedSlots(v, 3) != 1 {
		t.Errorf("3rd level slots used after the replay = %d, want 1", usedSlots(v, 3))
	}
}

// The master's switch: its input, who it reaches and what it carries over.
func TestRevivifyFrozen_SwitchInACombat(t *testing.T) {
	t.Parallel()
	a, e := newMortuary(t)
	a.dies(t, e, "Toren", a.toren)
	if _, err := a.adjustHP(t, e, "Goblin", damageHP(99)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	blockedOf := func(label string) bool {
		t.Helper()
		var b bool
		if err := a.h.pool.QueryRow(t.Context(), `SELECT revivify_blocked FROM combatants WHERE id = $1`, a.id(t, label)).Scan(&b); err != nil {
			t.Fatalf("read the switch of %s: %v", label, err)
		}
		return b
	}
	characterBlocked := func(id string) bool {
		t.Helper()
		var b bool
		if err := a.h.pool.QueryRow(t.Context(), `SELECT revivify_blocked FROM characters WHERE id = $1`, id).Scan(&b); err != nil {
			t.Fatalf("read the switch of the character: %v", err)
		}
		return b
	}

	// The input.
	wantRefusal(t, "no target", a.switchRaw(t, nil, true), connect.CodeInvalidArgument, "set combatant_id or character_id")
	wantRefusal(t, "a bad combatant id", a.switchRaw(t, switchCombatant("nope"), true), connect.CodeNotFound, "combatant not found")
	wantRefusal(t, "a bad character id", a.switchRaw(t, switchCharacter("nope"), true), connect.CodeNotFound, noCharacterMsg)
	wantRefusal(t, "a combatant that is not there", a.switchRaw(t, switchCombatant(newKey()), true), connect.CodeNotFound, "combatant not found")
	wantRefusal(t, "a creature that is alive", a.switchRaw(t, switchCombatant(a.id(t, "Brisa")), true), connect.CodeFailedPrecondition, "the creature is not dead")
	err := a.switchRaw(t, switchCharacter(a.bri.GetId()), true)
	t.Logf("a living character answers %v", err)
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("the switch on a living character error = %v, want failed_precondition", err)
	}
	if blockedOf("Brisa") || characterBlocked(a.bri.GetId()) {
		t.Errorf("a refused switch changed something")
	}
	for who, u := range map[string]*user{"a player": a.ana, "Toren's player": a.caio} {
		_, err := u.revivify.SetRevivifyBlocked(t.Context(), connect.NewRequest(&playv1.SetRevivifyBlockedRequest{
			CampaignId: a.campaignID, Target: &playv1.SetRevivifyBlockedRequest_CombatantId{CombatantId: a.id(t, "Toren")}, Blocked: true,
		}))
		wantCode(t, who+"'s SetRevivifyBlocked", err, connect.CodePermissionDenied)
	}

	// By combatant: a player's character carries it to the character (the cast outside a combat
	// reads that one); an NPC has no character.
	if err := a.switchRaw(t, switchCombatant(a.id(t, "Toren")), true); err != nil {
		t.Fatalf("SetRevivifyBlocked(Toren) error = %v", err)
	}
	if !blockedOf("Toren") || !characterBlocked(a.toren.GetId()) {
		t.Errorf("the switch by combatant: combatant %v, character %v; want both", blockedOf("Toren"), characterBlocked(a.toren.GetId()))
	}
	if err := a.switchRaw(t, switchCombatant(a.id(t, "Goblin")), true); err != nil {
		t.Fatalf("SetRevivifyBlocked(Goblin) error = %v", err)
	}
	if !blockedOf("Goblin") {
		t.Errorf("the goblin's switch is off, want on")
	}

	// By character: the combatant of the open combat that has it dead follows.
	if err := a.switchRaw(t, switchCharacter(a.toren.GetId()), false); err != nil {
		t.Fatalf("SetRevivifyBlocked(character) error = %v", err)
	}
	if blockedOf("Toren") || characterBlocked(a.toren.GetId()) {
		t.Errorf("the switch by character: combatant %v, character %v; want both off", blockedOf("Toren"), characterBlocked(a.toren.GetId()))
	}
	// A dead character that is not in the combat: only the character changes.
	nuvem := a.caio.hero(t, a.campaignID, "Nuvem", "class:fighter", "race:human", 1,
		&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
	if _, err := a.master.characters.MarkCharacterDead(t.Context(), connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: a.campaignID, CharacterId: nuvem.GetId()})); err != nil {
		t.Fatalf("MarkCharacterDead() error = %v", err)
	}
	if err := a.switchRaw(t, switchCharacter(nuvem.GetId()), true); err != nil {
		t.Fatalf("SetRevivifyBlocked(Nuvem) error = %v", err)
	}
	if !characterBlocked(nuvem.GetId()) {
		t.Errorf("Nuvem's switch is off, want on")
	}

	// The combat ended: no combatant is found, and a character is still switched.
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	wantRefusal(t, "a combatant of a combat that ended", a.switchRaw(t, switchCombatant(a.id(t, "Toren")), true), connect.CodeNotFound, "combatant not found")
	if err := a.switchRaw(t, switchCharacter(a.toren.GetId()), true); err != nil {
		t.Fatalf("SetRevivifyBlocked(character) after the end error = %v", err)
	}
	if !characterBlocked(a.toren.GetId()) {
		t.Errorf("Toren's switch is off after the end, want on")
	}
	// No open session: a combatant names nothing, a character is still switched.
	a.endRevivifySession(t)
	wantRefusal(t, "a combatant with no session", a.switchRaw(t, switchCombatant(newKey()), true), connect.CodeFailedPrecondition, noSessionMsg)
	if err := a.switchRaw(t, switchCharacter(a.toren.GetId()), false); err != nil {
		t.Fatalf("SetRevivifyBlocked(character) with no session error = %v", err)
	}
	if characterBlocked(a.toren.GetId()) {
		t.Errorf("Toren's switch is on with no session, want off")
	}
}
