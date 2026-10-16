package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Characterization of the summoning, familiar sight and Wild Shape handlers
// (creature_cast.go, familiarsight.go, wildshape.go) before they were split into
// phases: each refusal with its exact code and message, and the answers a retry
// gets. They need the database (MEURPG_TEST_DATABASE_URL).

const (
	summonsNoSuchID  = "5b0c7f0e-4a52-4f2b-9d0e-6c1d5f3a9b11"
	summonsNotAKey   = "not-a-key"
	summonsBadKeyMsg = "idempotency_key must be a UUID"
)

// wantSummonsRefusal fails unless err is a connect error with this code and exactly
// this message.
func wantSummonsRefusal(t *testing.T, call string, err error, code connect.Code, msg string) {
	t.Helper()
	wantCode(t, call, err, code)
	var ce *connect.Error
	if !asConnectError(err, &ce) || ce.Message() != msg {
		t.Errorf("%s message = %q, want %q", call, err.Error(), msg)
	}
}

// wantSummonsBlocked fails unless err is the failed_precondition with this reason
// and exactly this message.
func wantSummonsBlocked(t *testing.T, call string, err error, reason playv1.EncounterBlockedReason, msg string) {
	t.Helper()
	wantBlockedBy(t, call, err, reason)
	var ce *connect.Error
	if !asConnectError(err, &ce) || ce.Message() != msg {
		t.Errorf("%s message = %q, want %q", call, err.Error(), msg)
	}
}

func asConnectError(err error, target **connect.Error) bool {
	ce, ok := err.(*connect.Error) //nolint:errorlint // the client returns the connect error itself
	if ok {
		*target = ce
	}
	return ok
}

// summonRequest is a valid ritual CastSummon of Pensantus's familiar, for a test to
// break one field of.
func (a *armed) summonRequest(key string) *playv1.CastSummonRequest {
	return &playv1.CastSummonRequest{
		CampaignId: a.campaignID, CharacterId: a.pens.GetId(), SpellKey: findFamiliar, Ritual: true, IdempotencyKey: key,
		Summon: &playv1.SummonChoice{CreatureKeys: []string{"monster:owl"}, Names: []string{"Nanquim"}},
	}
}

func (a *armed) callSummon(u *user, req *playv1.CastSummonRequest) (*playv1.CastSummonResponse, error) {
	res, err := u.play.CastSummon(a.h.t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// TestSummonsCastSummonRefusesEachBadRequest: the order the fields are checked in
// (caller, key, character, spell_key, summon, ritual against slot) and the exact
// message of each; none of them writes.
func TestSummonsCastSummonRefusesEachBadRequest(t *testing.T) {
	t.Parallel()
	a := newSummoners(t)
	invalid := connect.CodeInvalidArgument
	cases := []struct {
		name string
		edit func(r *playv1.CastSummonRequest)
		code connect.Code
		msg  string
	}{
		{"key", func(r *playv1.CastSummonRequest) { r.IdempotencyKey = summonsNotAKey }, invalid, summonsBadKeyMsg},
		{"character", func(r *playv1.CastSummonRequest) { r.CharacterId = summonsNotAKey }, connect.CodeNotFound, "character not found"},
		{"empty spell", func(r *playv1.CastSummonRequest) { r.SpellKey = "" }, invalid, "spell_key must name a summoning spell"},
		{"long spell", func(r *playv1.CastSummonRequest) { r.SpellKey = strings.Repeat("s", 101) }, invalid, "spell_key must name a summoning spell"},
		{"no choice", func(r *playv1.CastSummonRequest) { r.Summon = nil }, invalid, "summon must say what the spell brings"},
		{"ritual with a slot", func(r *playv1.CastSummonRequest) { r.Slot = slotOfLevel(1) }, invalid, "a ritual is cast with no slot, and any other casting needs one"},
		{"slot cast without a slot", func(r *playv1.CastSummonRequest) { r.Ritual = false }, invalid, "a ritual is cast with no slot, and any other casting needs one"},
		{"unknown character", func(r *playv1.CastSummonRequest) { r.CharacterId = summonsNoSuchID }, connect.CodeNotFound, "character not found"},
		{"an NPC", func(r *playv1.CastSummonRequest) { r.CharacterId = a.goblin.GetId() }, connect.CodeNotFound, "character not found"},
		{"not the caster's spell", func(r *playv1.CastSummonRequest) {
			r.Ritual, r.Slot, r.SpellKey = false, slotOfLevel(3), "spell:fireball"
		}, invalid, "spell_key is not one of the caster's spells"},
	}
	for _, tc := range cases {
		req := a.summonRequest(newKey())
		tc.edit(req)
		_, err := a.callSummon(a.ana, req)
		wantSummonsRefusal(t, "CastSummon("+tc.name+")", err, tc.code, tc.msg)
	}
	// A stranger to the campaign is turned away before the request is read.
	stranger := a.h.newUser("Intruso")
	req := a.summonRequest(summonsNotAKey)
	_, err := a.callSummon(stranger, req)
	if connect.CodeOf(err) == invalid || err == nil {
		t.Errorf("CastSummon(stranger) error = %v, want a refusal of the caller, not of the request", err)
	}
	// The master may cast for the character; another player may not.
	_, err = a.callSummon(a.caio, a.summonRequest(newKey()))
	wantSummonsRefusal(t, "CastSummon(another player)", err, connect.CodePermissionDenied, "only the character's player or the master may cast for it")
	if n := a.eventCount(t, "creature_summoned"); n != 0 {
		t.Errorf("creature_summoned events = %d after refusals, want 0", n)
	}
	if res, err := a.callSummon(a.master, a.summonRequest(newKey())); err != nil || len(res.GetCreatureIds()) != 1 {
		t.Errorf("CastSummon(master) = %v, %v, want the familiar", res, err)
	}
}

// TestSummonsCastSummonRetryAnswersTheSame: the same key is the same request, and
// answers with the creatures of the first casting, never a second casting; the same
// key for another request is refused.
func TestSummonsCastSummonRetryAnswersTheSame(t *testing.T) {
	t.Parallel()
	a := newSummoners(t)
	key := newKey()
	first, err := a.callSummon(a.ana, a.summonRequest(key))
	if err != nil {
		t.Fatalf("CastSummon() error = %v", err)
	}
	again, err := a.callSummon(a.ana, a.summonRequest(key))
	if err != nil {
		t.Fatalf("CastSummon() retry error = %v", err)
	}
	if len(first.GetCreatureIds()) != 1 || again.GetCreatureIds()[0] != first.GetCreatureIds()[0] || len(again.GetDismissedCreatureIds()) != 0 {
		t.Errorf("retry = %v, want %v", again, first)
	}
	if n := a.eventCount(t, "creature_summoned"); n != 1 {
		t.Errorf("creature_summoned events = %d, want 1", n)
	}
	other := a.summonRequest(key)
	other.Summon.Names = []string{"Outro"}
	_, err = a.callSummon(a.ana, other)
	if err == nil {
		t.Errorf("CastSummon() with the key of another request succeeded, want a refusal")
	}
}

// TestSummonsCastSummonInACombatAndInABeastForm: a character in a combat casts there,
// and a druid in a beast form casts nothing; the replaced creatures are dismissed with
// the reason of what replaced them.
func TestSummonsCastSummonInACombatAndInABeastForm(t *testing.T) {
	t.Parallel()
	a := newSummoners(t)

	// A druid in a beast form casts no spell.
	a.mustAssume(t, a.bia, a.bri, wolfKey)
	_, err := a.castSummon(t, a.bia, a.bri, conjureAnimals, slotOfLevel(3), 1, []string{direWolf, direWolf})
	wantSummonsBlocked(t, "CastSummon(in a beast form)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WILD_SHAPE_NO_SPELLS, "a druid in a beast form cannot cast spells")
	a.mustLeave(t, a.bia, a.bri)

	// Outside a combat a concentration spell cast again replaces the creatures of the first.
	a.mustCastSummon(t, a.bia, a.bri, conjureAnimals, slotOfLevel(3), 1, []string{direWolf, direWolf})
	res := a.mustCastSummon(t, a.bia, a.bri, conjureAnimals, slotOfLevel(3), 1, []string{direWolf, direWolf})
	if len(res.GetCreatureIds()) != 2 || len(res.GetDismissedCreatureIds()) != 2 || res.GetVitals() == nil {
		t.Fatalf("second Conjurar Animais = %v, want 2 new, 2 dismissed and the vitals", res)
	}
	if got := a.lastPayload(t, "creature_dismissed")["reason"]; got != "concentration" {
		t.Errorf("dismissal reason = %v, want concentration", got)
	}
	a.mustCastSummon(t, a.ana, a.pens, findFamiliar, nil, 0, []string{"monster:owl"}, "Nanquim")
	a.mustCastSummon(t, a.ana, a.pens, findFamiliar, nil, 0, []string{"monster:raven"}, "Corvo")
	if got := a.lastPayload(t, "creature_dismissed")["reason"]; got != "replaced" {
		t.Errorf("dismissal reason = %v, want replaced", got)
	}

	// In a combat the cast is CastSpell's (a table of its own: the creatures above
	// would need an initiative roll to start it).
	a = newSummoners(t)
	a.summonersFight(t)
	_, err = a.castSummon(t, a.ana, a.pens, animateDead, slotOfLevel(3), 0, []string{"monster:skeleton"})
	wantSummonsBlocked(t, "CastSummon(in a combat)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_SUMMON_IN_COMBAT, "the character is in a combat: cast the spell there")
}

// endConcentrationCall calls EndConcentration as u with the raw fields.
func (a *armed) endConcentrationCall(u *user, campaign, enc, who, key string) error {
	_, err := u.combat.EndConcentration(a.h.t.Context(), connect.NewRequest(&playv1.EndConcentrationRequest{
		CampaignId: campaign, EncounterId: enc, CombatantId: who, IdempotencyKey: key,
	}))
	return err
}

// TestSummonsEndConcentrationRefusals: each bad field with its message, a combatant
// that is not in the combat, a combat that has ended, and a retry.
func TestSummonsEndConcentrationRefusals(t *testing.T) {
	t.Parallel()
	a := newSummoners(t)
	e := a.toSalvia(t, a.summonersFight(t))
	if _, err := a.summonWolves(t, a.bia, e, 11); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	salvia := a.id(t, "Sálvia")
	notFound := connect.CodeNotFound
	wantSummonsRefusal(t, "bad key", a.endConcentrationCall(a.bia, a.campaignID, e.GetId(), salvia, summonsNotAKey), connect.CodeInvalidArgument, summonsBadKeyMsg)
	wantSummonsRefusal(t, "bad encounter", a.endConcentrationCall(a.bia, a.campaignID, summonsNotAKey, salvia, newKey()), notFound, "encounter not found")
	wantSummonsRefusal(t, "bad combatant", a.endConcentrationCall(a.bia, a.campaignID, e.GetId(), summonsNotAKey, newKey()), notFound, "combatant not found")
	wantSummonsRefusal(t, "unknown combatant", a.endConcentrationCall(a.bia, a.campaignID, e.GetId(), summonsNoSuchID, newKey()), notFound, "combatant not found")
	if err := a.endConcentrationCall(a.h.newUser("Intruso"), a.campaignID, e.GetId(), salvia, newKey()); err == nil || connect.CodeOf(err) == connect.CodeInvalidArgument {
		t.Errorf("EndConcentration(stranger) error = %v, want the caller refused", err)
	}
	wantSummonsRefusal(t, "another player", a.endConcentrationCall(a.caio, a.campaignID, e.GetId(), salvia, newKey()), connect.CodePermissionDenied, "only the combatant's player or the master may do this")

	key := newKey()
	if err := a.endConcentrationCall(a.bia, a.campaignID, e.GetId(), salvia, key); err != nil {
		t.Fatalf("EndConcentration() error = %v", err)
	}
	if err := a.endConcentrationCall(a.bia, a.campaignID, e.GetId(), salvia, key); err != nil {
		t.Errorf("EndConcentration() retry error = %v", err)
	}
	if n := a.eventCount(t, "creature_dismissed"); n != 2 {
		t.Errorf("creature_dismissed events = %d, want 2 (one for each wolf, once)", n)
	}
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	err := a.endConcentrationCall(a.bia, a.campaignID, e.GetId(), salvia, newKey())
	wantBlockedBy(t, "EndConcentration(after the end)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ENCOUNTER_ENDED)
}

// TestSummonsEndConcentrationOfHuntersMarkClearsTheMark: ending the concentration on
// Hunter's Mark takes the mark off its target.
func TestSummonsEndConcentrationOfHuntersMarkClearsTheMark(t *testing.T) {
	t.Parallel()
	a := newRangers(t)
	e := a.castersFight(t, 1)
	a.passTo(t, e, "Brisa")
	a.mustCast(t, a.bia, e, "Brisa", huntersMark, slotOfLevel(1), a.at(t, "Goblin"), noCastRoll)
	if huntersMarkOn(t, a.get(t, a.master), "Goblin") == nil {
		t.Fatalf("setup: the Goblin has no mark")
	}
	if err := a.endConcentrationCall(a.bia, a.campaignID, e.GetId(), a.id(t, "Brisa"), newKey()); err != nil {
		t.Fatalf("EndConcentration() error = %v", err)
	}
	out := a.get(t, a.master)
	if huntersMarkOn(t, out, "Goblin") != nil || byLabel(t, out, "Brisa").GetConcentrationSpell() != "" {
		t.Errorf("after ending: mark %v, concentration %q, want none", huntersMarkOn(t, out, "Goblin"), byLabel(t, out, "Brisa").GetConcentrationSpell())
	}
}

// sightRaw calls Start or StopFamiliarSight as u with the raw fields.
func (a *armed) sightRaw(u *user, start bool, character, key string) error {
	if start {
		_, err := u.play.StartFamiliarSight(a.h.t.Context(), connect.NewRequest(&playv1.StartFamiliarSightRequest{CampaignId: a.campaignID, CharacterId: character, IdempotencyKey: key}))
		return err
	}
	_, err := u.play.StopFamiliarSight(a.h.t.Context(), connect.NewRequest(&playv1.StopFamiliarSightRequest{CampaignId: a.campaignID, CharacterId: character, IdempotencyKey: key}))
	return err
}

// TestSummonsFamiliarSightRefusesEachBadRequest: caller, key, character; and a sight
// with no map under the session, a map with no grid, and a familiar with no square in
// a combat are all "not on the map".
func TestSummonsFamiliarSightRefusesEachBadRequest(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	a := s.armed
	owl := s.nanquim(t)
	x, y := centerBP(0, 0)
	a.h.placeToken(a.mapID, s.pens.GetId(), x, y)
	a.placeCreatureToken(t, owl, 3, 0)
	for _, start := range []bool{true, false} {
		wantSummonsRefusal(t, "bad key", a.sightRaw(s.ana, start, s.pens.GetId(), summonsNotAKey), connect.CodeInvalidArgument, summonsBadKeyMsg)
		wantSummonsRefusal(t, "bad character", a.sightRaw(s.ana, start, summonsNotAKey, newKey()), connect.CodeNotFound, "character not found")
		wantSummonsRefusal(t, "an NPC", a.sightRaw(s.ana, start, s.goblin.GetId(), newKey()), connect.CodeNotFound, "character not found")
		wantSummonsRefusal(t, "another player", a.sightRaw(s.caio, start, s.pens.GetId(), newKey()), connect.CodePermissionDenied, "only the character's player or the master may do this")
		if err := a.sightRaw(a.h.newUser("Intruso"), start, s.pens.GetId(), newKey()); err == nil || connect.CodeOf(err) == connect.CodeInvalidArgument {
			t.Errorf("sight (start %v) by a stranger error = %v, want the caller refused", start, err)
		}
	}

	// No map under the session.
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE game_sessions SET current_map_id = NULL WHERE campaign_id = $1 AND ended_at IS NULL`, a.campaignID); err != nil {
		t.Fatal(err)
	}
	_, _, err := a.sightCall(t, s.ana, s.pens, true)
	wantSightBlocked(t, err, playv1.FamiliarSightBlockedReason_FAMILIAR_SIGHT_BLOCKED_REASON_NOT_ON_MAP)
	// A map with no grid has no distances.
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE game_sessions SET current_map_id = $2 WHERE campaign_id = $1 AND ended_at IS NULL`, a.campaignID, a.mapID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE maps SET grid_columns = NULL WHERE id = $1`, a.mapID); err != nil {
		t.Fatal(err)
	}
	_, _, err = a.sightCall(t, s.ana, s.pens, true)
	wantSightBlocked(t, err, playv1.FamiliarSightBlockedReason_FAMILIAR_SIGHT_BLOCKED_REASON_NOT_ON_MAP)
}

// TestSummonsFamiliarSightInACombatNeedsASquareForTheFamiliar: in a combat the squares
// are the combatants', and a familiar with none is not on the map.
func TestSummonsFamiliarSightInACombatNeedsASquareForTheFamiliar(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	s.nanquim(t)
	s.pensantusFirst(t, nil)
	if _, err := s.h.pool.Exec(t.Context(), `UPDATE combatants SET grid_col = NULL, grid_row = NULL WHERE label = 'Nanquim'`); err != nil {
		t.Fatal(err)
	}
	_, _, err := s.sightCall(t, s.ana, s.pens, true)
	wantSightBlocked(t, err, playv1.FamiliarSightBlockedReason_FAMILIAR_SIGHT_BLOCKED_REASON_NOT_ON_MAP)
}

// shapeRaw calls AssumeWildShape or LeaveWildShape as u with the raw fields.
func (a *armed) shapeRaw(u *user, assume bool, character, beast, key string) error {
	if assume {
		_, err := u.play.AssumeWildShape(a.h.t.Context(), connect.NewRequest(&playv1.AssumeWildShapeRequest{CampaignId: a.campaignID, CharacterId: character, BeastKey: beast, IdempotencyKey: key}))
		return err
	}
	_, err := u.play.LeaveWildShape(a.h.t.Context(), connect.NewRequest(&playv1.LeaveWildShapeRequest{CampaignId: a.campaignID, CharacterId: character, IdempotencyKey: key}))
	return err
}

// TestSummonsWildShapeRefusesEachBadRequest: caller, beast_key, key, character, who
// may act, the beasts the druid may not take and a form that is on already; a retry
// of the same key answers the same and spends one use only.
func TestSummonsWildShapeRefusesEachBadRequest(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	a := s.armed
	salvia := s.bri.GetId()
	for _, assume := range []bool{true, false} {
		wantSummonsRefusal(t, "bad key", a.shapeRaw(s.bia, assume, salvia, wolfKey, summonsNotAKey), connect.CodeInvalidArgument, summonsBadKeyMsg)
		wantSummonsRefusal(t, "bad character", a.shapeRaw(s.bia, assume, summonsNotAKey, wolfKey, newKey()), connect.CodeNotFound, "character not found")
		wantSummonsRefusal(t, "an NPC", a.shapeRaw(s.bia, assume, s.goblin.GetId(), wolfKey, newKey()), connect.CodeNotFound, "character not found")
		wantSummonsRefusal(t, "another player", a.shapeRaw(s.caio, assume, salvia, wolfKey, newKey()), connect.CodePermissionDenied, "only the character's player or the master may do this")
		if err := a.shapeRaw(a.h.newUser("Intruso"), assume, salvia, wolfKey, newKey()); err == nil || connect.CodeOf(err) == connect.CodeInvalidArgument {
			t.Errorf("shape (assume %v) by a stranger error = %v, want the caller refused", assume, err)
		}
	}
	wantSummonsRefusal(t, "empty beast", a.shapeRaw(s.bia, true, salvia, "", newKey()), connect.CodeInvalidArgument, "beast_key must name an SRD beast")
	wantSummonsRefusal(t, "long beast", a.shapeRaw(s.bia, true, salvia, strings.Repeat("b", 101), newKey()), connect.CodeInvalidArgument, "beast_key must name an SRD beast")
	wantSummonsRefusal(t, "leave in its own shape", a.shapeRaw(s.bia, false, salvia, "", newKey()), connect.CodeFailedPrecondition, "the character is in its own shape")

	err := a.shapeRaw(s.bia, true, salvia, direWolf, newKey())
	wantSummonsBlocked(t, "Assume(too strong)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WILD_SHAPE_BEAST_NOT_ALLOWED, "the beast is not one this character's Wild Shape allows")
	key := newKey()
	if err := a.shapeRaw(s.bia, true, salvia, wolfKey, key); err != nil {
		t.Fatalf("AssumeWildShape() error = %v", err)
	}
	if err := a.shapeRaw(s.bia, true, salvia, wolfKey, key); err != nil {
		t.Errorf("AssumeWildShape() retry error = %v", err)
	}
	if got := used(a.vitals(t, s.bri), wildShapeResource); got != 1 {
		t.Errorf("Wild Shape uses spent = %d after a retry, want 1", got)
	}
	err = a.shapeRaw(s.bia, true, salvia, wolfKey, newKey())
	wantSummonsBlocked(t, "Assume(again)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ALREADY_IN_WILD_SHAPE, "the character is in a beast form already")
}
