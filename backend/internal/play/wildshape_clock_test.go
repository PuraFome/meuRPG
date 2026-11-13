package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The Wild Shape clock (#584): a form lasts half the druid's level in hours (SRD 5.1) on the
// lasting effects' clock, in a combat and out of one. Sálvia is a level 5 druid: 2 hours, 7200
// seconds, 1200 rounds. Tests that need the end of the time move the clock in the database
// rather than play 1200 rounds.

const (
	formSeconds = 7200
	formRounds  = 1200
	mastiffKey  = "monster:mastiff"
)

// formClocks are the form clock rows of the table: in a combat (the round they end in) and out
// of one (the seconds they have left); -1 for none.
func (s *shapers) formClocks(t *testing.T) (endsRound, secondsLeft int) {
	t.Helper()
	endsRound, secondsLeft = -1, -1
	for _, q := range []struct {
		sql string
		out *int
	}{
		{`SELECT ends_round FROM combatant_states WHERE kind = 'effect' AND source_key = 'effect:wild-shape'`, &endsRound},
		{`SELECT seconds_left FROM character_effects WHERE source_key = 'effect:wild-shape'`, &secondsLeft},
	} {
		rows, err := s.h.pool.Query(t.Context(), q.sql)
		if err != nil {
			t.Fatalf("read the form clocks: %v", err)
		}
		n := 0
		for rows.Next() {
			n++
			if err := rows.Scan(q.out); err != nil {
				t.Fatalf("read a form clock: %v", err)
			}
		}
		rows.Close()
		if n > 1 {
			t.Fatalf("%q = %d rows, want at most one clock per form", q.sql, n)
		}
	}
	return endsRound, secondsLeft
}

// setEndsRound moves the combat clock of the form to a round.
func (s *shapers) setEndsRound(t *testing.T, round int) {
	t.Helper()
	if _, err := s.h.pool.Exec(t.Context(), `UPDATE combatant_states SET ends_round = $1 WHERE kind = 'effect' AND source_key = 'effect:wild-shape'`, round); err != nil {
		t.Fatalf("move the form's clock: %v", err)
	}
}

// aRound ends turns until the next round begins (and Sálvia, first in the order, is up).
func (s *shapers) aRound(t *testing.T) *playv1.Encounter {
	t.Helper()
	e := s.get(t, s.master)
	round := e.GetRound()
	for range 12 {
		var err error
		if e, err = s.endTurn(t, s.master, e, true); err != nil {
			t.Fatalf("EndTurn() error = %v", err)
		}
		if e.GetRound() > round {
			return e
		}
	}
	t.Fatalf("the round did not pass")
	return e
}

// TestWildShapeClockOutsideACombat: the form out of a combat holds a clock of 2 hours (level
// 5), which only the master's game time moves; it reaches 0 and the form ends with the reason
// expired. Only the master and the owner read the clock (RN-20).
func TestWildShapeClockOutsideACombat(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	s.mustAssume(t, s.bia, s.bri, wolfKey)
	if round, secs := s.formClocks(t); round != -1 || secs != formSeconds {
		t.Fatalf("clocks = %d rounds, %d seconds, want %d seconds out of a combat", round, secs, formSeconds)
	}
	// RN-20: the master and the owner read the clock, the other players do not.
	if got := s.seenBy(t, s.bia, s.caio, s.bri.GetId()); got != "1/1/0" {
		t.Errorf("clock seen by master/owner/other = %s, want 1/1/0", got)
	}
	if got := s.seenBy(t, s.bia, s.ana, s.bri.GetId()); got != "1/1/0" {
		t.Errorf("clock seen by master/owner/third player = %s, want 1/1/0", got)
	}

	s.advanceGameTime(t, 3600)
	if w := s.vitals(t, s.bri).GetWildShape(); w.GetBeastKey() != wolfKey {
		t.Fatalf("form after an hour = %v, want the wolf still", w)
	}
	if _, secs := s.formClocks(t); secs != 3600 {
		t.Errorf("clock after an hour = %d seconds, want 3600", secs)
	}
	res := s.advanceGameTime(t, 3600)
	if res.GetEffectsEnded() != 1 {
		t.Errorf("effects ended = %d, want the clock", res.GetEffectsEnded())
	}
	if w := s.vitals(t, s.bri).GetWildShape(); w != nil {
		t.Fatalf("form after 2 hours = %v, want her own shape", w)
	}
	if round, secs := s.formClocks(t); round != -1 || secs != -1 {
		t.Errorf("clocks after the end = %d, %d, want none", round, secs)
	}
	if ended := s.lastPayload(t, eventWildShapeEnded); ended["reason"] != endedExpired || ended["beast"] != wolfKey {
		t.Errorf("wild_shape_ended = %v, want the wolf with the reason %q", ended, endedExpired)
	}
	// The hit points of her own shape were never touched.
	if got := s.vitals(t, s.bri).GetHitPointsCurrent(); got != 38 {
		t.Errorf("hit points = %d, want 38", got)
	}
}

// TestWildShapeClockPassesIntoACombat: the time the form has left goes with the druid into a
// combat (the same record, in rounds), and back out when it ends.
func TestWildShapeClockPassesIntoACombat(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	s.mustAssume(t, s.bia, s.bri, wolfKey)
	s.advanceGameTime(t, 60) // 10 rounds
	s.fight(t)
	if round, _ := s.formClocks(t); round != 1+(formRounds-10) {
		t.Fatalf("clock in the combat ends in round %d, want %d (the 7140 seconds left)", round, 1+formRounds-10)
	}
	if got := byLabel(t, s.get(t, s.master), "Sálvia").GetWildShapeBeastKey(); got != wolfKey {
		t.Errorf("Sálvia's form = %q, want the wolf", got)
	}
	if got := s.seenInCombat(t, s.bia, s.caio, "Sálvia"); got != "1/1/0" {
		t.Errorf("clock seen in the combat by master/owner/other = %s, want 1/1/0", got)
	}
}

// TestWildShapeClockInACombatExpires: the clock row ends at the start of the druid's turn that
// completes the duration; the form ends with the reason expired and the lasting effects'
// ended line.
func TestWildShapeClockInACombatExpires(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	s.fight(t)
	s.mustAssume(t, s.bia, s.bri, wolfKey)
	if round, _ := s.formClocks(t); round != 1+formRounds {
		t.Fatalf("clock ends in round %d, want %d (2 hours of 6-second rounds)", round, 1+formRounds)
	}
	s.correct(t, s.bri, func(r *playv1.AdjustCharacterVitalsRequest) { r.WildShapeHitPointsCurrent = new(int32(6)) })
	s.setEndsRound(t, 2)
	lines := s.eventCount(t, eventLastingEnded)

	// Round 1 ends with the wolf still on; the druid's turn of round 2 is when it runs out.
	e := s.aRound(t)
	if w := s.vitals(t, s.bri).GetWildShape(); w != nil {
		t.Fatalf("form in round 2 = %v, want her own shape", w)
	}
	got := byLabel(t, e, "Sálvia")
	if got.GetWildShapeBeastKey() != "" || got.GetSpeedFt() != 30 {
		t.Errorf("combatant = form %q, %d ft, want her own 30 ft", got.GetWildShapeBeastKey(), got.GetSpeedFt())
	}
	if ended := s.lastPayload(t, eventWildShapeEnded); ended["reason"] != endedExpired {
		t.Errorf("wild_shape_ended = %v, want the reason %q", ended, endedExpired)
	}
	if n := s.eventCount(t, eventLastingEnded); n != lines+1 {
		t.Errorf("lasting effect ended lines = %d, want %d (the notice the effects already give)", n, lines+1)
	}
	if round, _ := s.formClocks(t); round != -1 {
		t.Errorf("clock after the end = round %d, want none", round)
	}
	// The player of the log reads the reason and no number.
	if got := s.seenInCombat(t, s.bia, s.caio, "Sálvia"); got != "0/0/0" {
		t.Errorf("effects after the end = %s, want none", got)
	}

	// RN-20: everyone who sees her reads that the form ended and why (no number in it); the
	// effect's own line, with the time, is the master's and her player's alone.
	e = s.get(t, s.master)
	for name, tc := range map[string]struct {
		u    *user
		line bool
	}{"master": {s.master, true}, "owner": {s.bia, true}, "another player": {s.caio, false}, "a third player": {s.ana, false}} {
		log := s.log(t, tc.u, e)
		if got := lastShapeLine(t, shapeLines(log)); got.GetStarted() || got.GetEndReason() != playv1.WildShapeEndReason_WILD_SHAPE_END_REASON_EXPIRED || got.GetBeastKey() != wolfKey {
			t.Errorf("%s reads %v, want the wolf's end by expiry", name, got)
		}
		raw, err := protojson.Marshal(log)
		if err != nil {
			t.Fatalf("encode the log: %v", err)
		}
		if has := strings.Contains(string(raw), "effect:wild-shape"); has != tc.line {
			t.Errorf("%s: the log carries the form's clock line = %v, want %v", name, has, tc.line)
		}
	}

	// A turn change has no undo, the expiry with it (as for every effect whose time runs out): the
	// way back to the wolf is a new form (an action, a use), or the master giving the use back.
	if id := s.log(t, s.master, s.get(t, s.master)).GetUndoableEventId(); id != "" {
		t.Errorf("undoable event after the expiry = %q, want none (the turn change cannot be undone)", id)
	}
}

// TestWildShapeSwapIsLeaveThenAssume: the SRD has no free swap. In a beast form, another beast
// is refused; going back is a bonus action, and the new beast is an action and another use.
func TestWildShapeSwapIsLeaveThenAssume(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	s.fight(t)
	s.mustAssume(t, s.bia, s.bri, wolfKey)
	_, err := s.assume(t, s.bia, s.bri, mastiffKey)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ALREADY_IN_WILD_SHAPE)

	// The way back is the bonus action; the action of this turn is used, so the new beast waits.
	e := s.mustLeave(t, s.bia, s.bri).GetEncounter()
	if c := byLabel(t, e, "Sálvia"); !c.GetBonusActionUsed() || !c.GetActionUsed() {
		t.Errorf("economy after leaving = action %v, bonus %v, want both used (the action was the wolf's)", c.GetActionUsed(), c.GetBonusActionUsed())
	}
	if round, _ := s.formClocks(t); round != -1 {
		t.Errorf("clock after leaving = round %d, want none", round)
	}
	_, err = s.assume(t, s.bia, s.bri, mastiffKey)
	wantEncounterBlocked(t, err, blockedActionUsed)

	// Next turn: the mastiff costs the action and a second use, with a clock of its own.
	s.aRound(t)
	res := s.mustAssume(t, s.bia, s.bri, mastiffKey)
	if used(res.GetVitals(), wildShapeResource) != 2 || res.GetVitals().GetWildShape().GetBeastKey() != mastiffKey {
		t.Errorf("vitals = %v, want the mastiff and two uses spent", res.GetVitals())
	}
	if c := byLabel(t, res.GetEncounter(), "Sálvia"); !c.GetActionUsed() || c.GetBonusActionUsed() {
		t.Errorf("economy = action %v, bonus %v, want the action only", c.GetActionUsed(), c.GetBonusActionUsed())
	}
	if round, _ := s.formClocks(t); round != 2+formRounds {
		t.Errorf("clock ends in round %d, want %d", round, 2+formRounds)
	}
	// The third use does not exist (2 per short rest).
	s.mustLeave(t, s.bia, s.bri)
	s.aRound(t)
	_, err = s.assume(t, s.bia, s.bri, wolfKey)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_USES)
}

// TestWildShapeStayInTheForm: the same beast again, before the time ends, spends the uses and
// no action, keeps the beast's hit points and starts the clock over (SRD: "unless you expend
// another use"). The undo gives back the use and the time that was left.
func TestWildShapeStayInTheForm(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	s.fight(t)
	s.mustAssume(t, s.bia, s.bri, wolfKey)
	s.correct(t, s.bri, func(r *playv1.AdjustCharacterVitalsRequest) { r.WildShapeHitPointsCurrent = new(int32(6)) })
	s.setEndsRound(t, 5)
	s.aRound(t)

	before := byLabel(t, s.get(t, s.master), "Sálvia")
	if before.GetActionUsed() {
		t.Fatal("the new round left the action used")
	}
	res := s.mustAssume(t, s.bia, s.bri, wolfKey)
	w := res.GetVitals().GetWildShape()
	if w.GetBeastKey() != wolfKey || w.GetHitPointsCurrent() != 6 || used(res.GetVitals(), wildShapeResource) != 2 {
		t.Errorf("vitals after the stay = %v, want the same wolf at 6 and two uses spent", res.GetVitals())
	}
	if c := byLabel(t, res.GetEncounter(), "Sálvia"); c.GetActionUsed() || c.GetSpeedFt() != 40 {
		t.Errorf("combatant after the stay = action used %v, %d ft, want no action spent and the wolf's 40", c.GetActionUsed(), c.GetSpeedFt())
	}
	if round, _ := s.formClocks(t); round != 2+formRounds {
		t.Errorf("clock ends in round %d, want %d (the whole duration from round 2)", round, 2+formRounds)
	}
	// A third time has no use to spend; the form and the clock stay as they were.
	_, err := s.assume(t, s.bia, s.bri, wolfKey)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_USES)

	s.undoLast(t, s.get(t, s.master))
	v := s.vitals(t, s.bri)
	if used(v, wildShapeResource) != 1 || v.GetWildShape().GetHitPointsCurrent() != 6 {
		t.Errorf("vitals after the undo = %v, want one use and the wolf at 6", v)
	}
	if round, _ := s.formClocks(t); round != 5 {
		t.Errorf("clock after the undo = round %d, want the 5 it had", round)
	}
}

// TestWildShapeClockEndsWithTheForm: every way out of the form takes the clock away: leaving,
// the beast falling (the damage carried over), the master's hand.
func TestWildShapeClockEndsWithTheForm(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	for name, end := range map[string]func(){
		"leaving": func() { s.mustLeave(t, s.bia, s.bri) },
		"master's hand": func() {
			s.correct(t, s.bri, func(r *playv1.AdjustCharacterVitalsRequest) { r.WildShapeHitPointsCurrent = new(int32(0)) })
		},
	} {
		s.mustAssume(t, s.bia, s.bri, wolfKey)
		if _, secs := s.formClocks(t); secs != formSeconds {
			t.Fatalf("%s: clock = %d, want %d", name, secs, formSeconds)
		}
		end()
		if round, secs := s.formClocks(t); round != -1 || secs != -1 {
			t.Errorf("after %s the clocks = %d, %d, want none", name, round, secs)
		}
		if _, err := s.master.play.AdjustCharacterVitals(t.Context(), connect.NewRequest(&playv1.AdjustCharacterVitalsRequest{
			CampaignId: s.campaignID, CharacterId: s.bri.GetId(), IdempotencyKey: newKey(), ExpectedRevision: s.master.revisionOf(t, s.campaignID, s.bri.GetId()), ResourcesUsed: []*playv1.ResourceUsed{{Key: wildShapeResource, Used: 0}},
		})); err != nil {
			t.Fatalf("giving the uses back: %v", err)
		}
	}
}
