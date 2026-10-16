package play

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	maplink "github.com/PuraFome/meuRPG/backend/internal/maps/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

func trapSquare(col, row int) grid.Square { return grid.Square{Col: col, Row: row} }

func mapCreatureOf(id string) maplink.MapCreature { return maplink.MapCreature{ID: id} }

// Characterization of the firing of a trap (MR-035, RN-02, RN-10), written before FireTrap, the
// firing in and outside a combat, the settling of a trap damage and ListTrapDamages were split:
// every refusal with its code and message, the firings that traps_test.go does not reach and
// what each viewer reads of the damages that wait. Where a branch looks odd it is frozen and said
// so in a comment that starts with "Finding".

// trapShape is what a refusal says: the code, the message and the reason of its detail.
func trapShape(err error) string {
	if err == nil {
		return "<nil>"
	}
	out := connect.CodeOf(err).String() + "|" + err.Error()
	if ce, ok := errors.AsType[*connect.Error](err); ok {
		for _, d := range ce.Details() {
			v, derr := d.Value()
			if derr != nil {
				continue
			}
			switch b := v.(type) {
			case *playv1.GameSessionBlocked:
				out += "|" + b.GetReason().String()
			case *playv1.EncounterBlocked:
				out += "|" + b.GetReason().String()
			}
		}
	}
	return out
}

// wantTrapRefusal checks the exact code, message and reason of a refusal.
func wantTrapRefusal(t *testing.T, what string, err error, want string) {
	t.Helper()
	if got := trapShape(err); got != want {
		t.Errorf("%s = %q, want %q", what, got, want)
	}
}

var manualTrap = func(s *mapsv1.TrapSpec) { s.Trigger = rulesv1.TrapTrigger_TRAP_TRIGGER_MANUAL }

// fireRaw calls FireTrap as the master with a request the test edits.
func (r *trapRig) fireRaw(t *testing.T, p *mapsv1.MapPoint, edit func(*playv1.FireTrapRequest)) (*playv1.FireTrapResponse, error) {
	t.Helper()
	req := &playv1.FireTrapRequest{CampaignId: r.campaignID, MapId: r.mapID, PointId: p.GetId(), IdempotencyKey: newKey()}
	if edit != nil {
		edit(req)
	}
	res, err := r.master.play.FireTrap(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// lastFiringEvent is the id of the newest trap_triggered event of the session.
func (r *trapRig) lastFiringEvent(t *testing.T) string {
	t.Helper()
	var id string
	if err := r.h.pool.QueryRow(t.Context(), `SELECT id FROM session_events WHERE kind = 'trap_triggered' ORDER BY seq DESC LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("read the newest firing: %v", err)
	}
	return id
}

// TestTrapFiringRefusals: every request FireTrap refuses before it fires anything, with the
// exact answer, and what a firing outside a combat refuses after it read the map: a trap that is
// not armed, a creature that is not a token of the map, a firing that cannot be extended.
func TestTrapFiringRefusals(t *testing.T) { //nolint:tparallel // the subtests share one rig and go in order
	t.Parallel()
	r := newTrapRig(t)
	pitA := r.trap(t, "Poço A", 12, 7, pit("1d6"), manualTrap)
	pitB := r.trap(t, "Poço B", 14, 7, pit("1d6"), manualTrap)
	r.place(t, r.toren.GetId(), 12, 7)
	r.place(t, r.pens.GetId(), 12, 7)

	t.Run("the request", func(t *testing.T) {
		manyTargets := make([]string, maxTrapTargets+1)
		for i := range manyTargets {
			manyTargets[i] = uuid.New().String()
		}
		for _, tc := range []struct {
			name string
			edit func(*playv1.FireTrapRequest)
			want string
		}{
			{"a key that is no UUID", func(q *playv1.FireTrapRequest) { q.IdempotencyKey = "x" }, "invalid_argument|invalid_argument: idempotency_key must be a UUID"},
			{"a map that is no UUID", func(q *playv1.FireTrapRequest) { q.MapId = "x" }, "not_found|not_found: trap not found"},
			{"a point that is no UUID", func(q *playv1.FireTrapRequest) { q.PointId = "x" }, "not_found|not_found: trap not found"},
			{"too many targets", func(q *playv1.FireTrapRequest) { q.TargetIds = manyTargets }, "invalid_argument|invalid_argument: target_ids must have at most 40 ids"},
			{"a target that is no UUID", func(q *playv1.FireTrapRequest) { q.TargetIds = []string{r.toren.GetId(), "x"} }, "invalid_argument|invalid_argument: target_ids must be UUIDs"},
			{"an extension that is no UUID", func(q *playv1.FireTrapRequest) { q.ExtendFiringId = "x" }, "not_found|not_found: firing not found"},
			{"an extension with no one to add", func(q *playv1.FireTrapRequest) { q.ExtendFiringId = uuid.New().String() }, "invalid_argument|invalid_argument: extend_firing_id needs the target_ids to add"},
			{"a point of no trap", func(q *playv1.FireTrapRequest) { q.PointId = uuid.New().String() }, "not_found|not_found: trap not found"},
			{"a point of another map", func(q *playv1.FireTrapRequest) { q.MapId = uuid.New().String() }, "not_found|not_found: map not found"},
		} {
			_, err := r.fireRaw(t, pitA, tc.edit)
			wantTrapRefusal(t, "FireTrap("+tc.name+")", err, tc.want)
		}
		_, err := r.caio.play.FireTrap(t.Context(), connect.NewRequest(&playv1.FireTrapRequest{CampaignId: r.campaignID, MapId: r.mapID, PointId: pitA.GetId(), IdempotencyKey: newKey()}))
		wantTrapRefusal(t, "FireTrap(a player)", err, "permission_denied|permission_denied: only the campaign's master can do this")
		if n := r.eventCount(t, "trap_triggered"); n != 0 {
			t.Errorf("trap_triggered events after the refusals = %d, want none", n)
		}
	})

	t.Run("a server with the traps off", func(t *testing.T) {
		r.h.svc.SetTraps(nil)
		defer r.h.svc.SetTraps(r.msvc)
		_, err := r.fireRaw(t, pitA, nil)
		wantTrapRefusal(t, "FireTrap(no traps)", err, "unavailable|unavailable: traps are not available on this server")
		// Dropping a token or a creature is the maps module's call, and with no traps it does nothing.
		if err := r.h.svc.TokenDropped(t.Context(), r.campaignID, r.mapID, r.toren.GetId(), r.master.id, trapSquare(12, 7)); err != nil {
			t.Errorf("TokenDropped(no traps) error = %v, want none", err)
		}
		if err := r.h.svc.CreatureDropped(t.Context(), r.campaignID, r.mapID, mapCreatureOf("c-1"), r.master.id, trapSquare(12, 7)); err != nil {
			t.Errorf("CreatureDropped(no traps) error = %v, want none", err)
		}
	})

	t.Run("outside a combat", func(t *testing.T) {
		r.h.roller.queue(3, 4)
		first, err := r.fireRaw(t, pitA, func(q *playv1.FireTrapRequest) { q.TargetIds = []string{r.toren.GetId()} })
		if err != nil {
			t.Fatalf("FireTrap() error = %v", err)
		}
		host := first.GetFiring().GetId()
		if host == "" || host != r.lastFiringEvent(t) {
			t.Fatalf("the firing's id = %q, want the id of its event %q", host, r.lastFiringEvent(t))
		}
		_, err = r.fireRaw(t, pitA, func(q *playv1.FireTrapRequest) { q.TargetIds = []string{r.toren.GetId()} })
		wantTrapRefusal(t, "FireTrap(a trap that fired)", err, "failed_precondition|failed_precondition: the trap is not armed|ENCOUNTER_BLOCKED_REASON_TRAP_NOT_ARMED")
		_, err = r.fireRaw(t, pitB, func(q *playv1.FireTrapRequest) { q.TargetIds = []string{r.bri.GetId()} })
		wantTrapRefusal(t, "FireTrap(a character with no token)", err, "invalid_argument|invalid_argument: target_ids must be characters with a token on the trap's map")
		if got := r.stateOf(t, r.mapID, pitB.GetId()); got == mapsv1.TrapState_TRAP_STATE_TRIGGERED {
			t.Errorf("the second pit is %v after a refused firing, want it armed", got)
		}

		// An extension needs the firing of this trap, and it is made once: the extension itself is not one.
		_, err = r.fireRaw(t, pitB, func(q *playv1.FireTrapRequest) {
			q.TargetIds, q.ExtendFiringId = []string{r.pens.GetId()}, host
		})
		wantTrapRefusal(t, "FireTrap(extending the firing of another trap)", err, "not_found|not_found: firing not found")
		_, err = r.fireRaw(t, pitA, func(q *playv1.FireTrapRequest) {
			q.TargetIds, q.ExtendFiringId = []string{r.pens.GetId()}, r.writeEvent(t, eventDamageApplied, nil, actionEvent{})
		})
		wantTrapRefusal(t, "FireTrap(extending a firing of an event that is no firing)", err, "not_found|not_found: firing not found")
		extended, err := r.fireRaw(t, pitA, func(q *playv1.FireTrapRequest) {
			q.TargetIds, q.ExtendFiringId = []string{r.pens.GetId()}, host
		})
		if err != nil {
			t.Fatalf("FireTrap(extend) error = %v", err)
		}
		if got := extended.GetFiring(); got.GetId() != host || len(got.GetCaught()) != 1 || got.GetCaught()[0].GetTargetLabel() != "Pensantus" {
			t.Errorf("the extension = %v, want Pensantus alone under the firing %q", got, host)
		}
		_, err = r.fireRaw(t, pitA, func(q *playv1.FireTrapRequest) {
			q.TargetIds, q.ExtendFiringId = []string{r.toren.GetId()}, r.lastFiringEvent(t)
		})
		wantTrapRefusal(t, "FireTrap(extending an extension)", err, "invalid_argument|invalid_argument: that firing cannot be extended now")

		// The same firing, once a combat runs on the map, is not the combat's to extend.
		r.fight(t)
		_, err = r.fireRaw(t, pitA, func(q *playv1.FireTrapRequest) {
			q.TargetIds, q.ExtendFiringId = []string{r.id(t, "Toren")}, host
		})
		wantTrapRefusal(t, "FireTrap(extending a firing made outside the combat)", err, "invalid_argument|invalid_argument: that firing cannot be extended now")
		_, err = r.fireRaw(t, pitB, func(q *playv1.FireTrapRequest) { q.TargetIds = []string{uuid.New().String()} })
		wantTrapRefusal(t, "FireTrap(a combatant that is not there)", err, "not_found|not_found: combatant not found")
		// Dropped in the middle of a combat, a token or a creature fires nothing: the combat's moves are the combat's.
		if err := r.h.svc.TokenDropped(t.Context(), r.campaignID, r.mapID, r.toren.GetId(), r.master.id, trapSquare(14, 7)); err != nil {
			t.Errorf("TokenDropped(in a combat) error = %v, want none", err)
		}
		if got := r.stateOf(t, r.mapID, pitB.GetId()); got == mapsv1.TrapState_TRAP_STATE_TRIGGERED {
			t.Errorf("the second pit is %v after a token was dropped in a combat, want it armed", got)
		}
	})

	t.Run("a session that is not open", func(t *testing.T) {
		r.master.end(t, r.master.liveSession(t, r.campaignID).GetGameSession())
		_, err := r.fireRaw(t, pitB, nil)
		wantTrapRefusal(t, "FireTrap(no open session)", err, "failed_precondition|failed_precondition: the campaign has no open game session|GAME_SESSION_BLOCKED_REASON_NO_OPEN_SESSION")
		// A token dropped with no session open fires nothing and is no error: only a session keeps the vitals and the history.
		if err := r.h.svc.TokenDropped(t.Context(), r.campaignID, r.mapID, r.toren.GetId(), r.master.id, trapSquare(14, 7)); err != nil {
			t.Errorf("TokenDropped(no session) error = %v, want none", err)
		}
		if err := r.h.svc.CreatureDropped(t.Context(), r.campaignID, r.mapID, mapCreatureOf("c-1"), r.master.id, trapSquare(14, 7)); err != nil {
			t.Errorf("CreatureDropped(no session) error = %v, want none", err)
		}
	})
}

// writeEvent writes an event of the open session straight into its history, as the services that own
// the kind write it, with the idempotency key if there is one.
func (r *trapRig) writeEvent(t *testing.T, kind string, key *string, payload any) string {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	session := r.master.liveSession(t, r.campaignID).GetGameSession().GetId()
	seq, err := r.h.svc.queries.NextSessionEventSeq(t.Context(), session)
	if err != nil {
		t.Fatalf("NextSessionEventSeq() error = %v", err)
	}
	row, err := r.h.svc.queries.InsertSessionEvent(t.Context(), playdb.InsertSessionEventParams{GameSessionID: session, Seq: seq, Kind: kind, Payload: body, IdempotencyKey: key, CreatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("InsertSessionEvent() error = %v", err)
	}
	return row.ID
}

// TestTrapFiringThatLosesItsSession: the session ends between the moment the firing outside a
// combat read the tokens and the moment it takes the session's lock.
func TestTrapFiringThatLosesItsSession(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	calm := r.trap(t, "Calma", 12, 7, pit("1d6"), manualTrap)
	r.place(t, r.toren.GetId(), 12, 7)
	r.h.svc.afterTrapRead = func() {
		r.h.svc.afterTrapRead = nil
		r.master.end(t, r.master.liveSession(t, r.campaignID).GetGameSession())
	}
	_, err := r.fireRaw(t, calm, nil)
	r.h.svc.afterTrapRead = nil
	wantTrapRefusal(t, "FireTrap(a session that ended meanwhile)", err, "failed_precondition|failed_precondition: the campaign has no open game session|GAME_SESSION_BLOCKED_REASON_NO_OPEN_SESSION")
	if got := r.stateOf(t, r.mapID, calm.GetId()); got == mapsv1.TrapState_TRAP_STATE_TRIGGERED {
		t.Errorf("the trap is %v after the refused firing, want it armed", got)
	}
}

// TestTrapFiringOutsideACombatRollsEveryCreature: a trap with a
// save and conditions fired by hand with nobody named catches the characters and the creatures in
// its area. A character the master plays (an NPC) takes its damage as a line, a player's character
// leaves a damage that waits, and the creatures come last, in the order of their ids.
func TestTrapFiringOutsideACombatRollsEveryCreature(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	net := r.trap(t, "Rede", 12, 7, manualTrap, func(s *mapsv1.TrapSpec) {
		s.Effect = &rulesv1.TrapEffect{
			Conditions: []*rulesv1.TrapCondition{{ConditionKey: "condition:prone"}},
			Save: &rulesv1.TrapSaveEffect{
				Ability: rulesv1.Ability_ABILITY_DEXTERITY, Dc: 25, AppliesTo: rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_CAUGHT,
				OnFail: &rulesv1.TrapOnFail{Damage: []*rulesv1.TrapDamage{{Dice: "1d6", DamageTypeKey: "damage-type:bludgeoning"}}, Condition: &rulesv1.TrapCondition{ConditionKey: "condition:restrained"}},
				OnPass: rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_HALF,
			},
		}
	})
	r.place(t, r.pens.GetId(), 12, 7)
	r.place(t, r.goblins.GetId(), 12, 7)
	_, dropOne := r.wolfOf(t, r.mapID)
	_, dropTwo := r.wolfOf(t, r.mapID)
	dropOne(12, 7)
	dropTwo(12, 7)
	// Every save is a 1 against DC 25 and fails, and every 1d6 is a 3.
	r.h.roller.queue(1, 3, 1, 3, 1, 3, 1, 3)
	res, err := r.fireRaw(t, net, nil)
	if err != nil {
		t.Fatalf("FireTrap() error = %v", err)
	}
	caught := res.GetFiring().GetCaught()
	if len(caught) != 4 {
		t.Fatalf("caught = %v, want two characters and two creatures", caught)
	}
	var lines []string
	for i, c := range caught {
		if len(c.GetDamages()) != 1 || c.GetDamages()[0].GetAmount() != 3 || c.GetSave().GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_FAILED ||
			len(c.GetConditionKeys()) != 2 || c.GetConditionKeys()[0] != "condition:prone" || c.GetConditionKeys()[1] != "condition:restrained" {
			t.Errorf("caught[%d] = %v, want one damage of 3, a failed save, prone and restrained", i, c)
		}
		lines = append(lines, c.GetTargetLabel()+":"+c.GetDamages()[0].GetStatus().String())
	}
	// The characters come first, the creatures after them: a character's damage waits for the master
	// if it is a player's, and a creature's is a line only.
	if got := strings.Join(lines, ","); !strings.Contains(got, "Pensantus:PENDING_DAMAGE_STATUS_ROLLED") || strings.Count(got, "Nanquim:PENDING_DAMAGE_STATUS_APPLIED") != 2 {
		t.Errorf("the statuses = %s, want Pensantus's rolled and the creatures' applied", got)
	}
	if d := r.trapDamages(t); len(d) != 1 || d[0].GetCharacterName() != "Pensantus" || d[0].GetAmount() != 3 {
		t.Errorf("trap damages = %v, want 3 on Pensantus alone", d)
	}
	if caught[2].GetTargetId() >= caught[3].GetTargetId() {
		t.Errorf("the creatures %q and %q are not in the order of their ids", caught[2].GetTargetId(), caught[3].GetTargetId())
	}
}

// TestTrapFiringInACombatMergesConditions: a trap that gives a condition the combatant has
// already changes nothing of it (and an undo leaves it), and a trap with nobody in its area catches nobody.
func TestTrapFiringInACombatMergesConditions(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	cond := func(s *mapsv1.TrapSpec) {
		s.Effect = &rulesv1.TrapEffect{Conditions: []*rulesv1.TrapCondition{{ConditionKey: "condition:prone"}}}
	}
	one := r.trap(t, "Um", 20, 12, manualTrap, cond)
	two := r.trap(t, "Dois", 20, 13, manualTrap, cond)
	empty := r.trap(t, "Vazio", 20, 14, manualTrap, cond)
	r.fight(t)
	toren := r.id(t, "Toren")
	conditions := func() []string { return byLabel(t, r.get(t, r.master), "Toren").GetConditions() }
	for i, p := range []*mapsv1.MapPoint{one, two} {
		res, err := r.fireRaw(t, p, func(q *playv1.FireTrapRequest) { q.TargetIds = []string{toren} })
		if err != nil {
			t.Fatalf("FireTrap(%d) error = %v", i, err)
		}
		c := res.GetFiring().GetCaught()
		if len(c) != 1 || len(c[0].GetDamages()) != 0 || len(c[0].GetConditionKeys()) != 1 {
			t.Errorf("firing %d = %v, want Toren prone with no damage line", i, res.GetFiring())
		}
		if got := conditions(); len(got) != 1 || got[0] != "condition:prone" {
			t.Errorf("Toren's conditions after firing %d = %v, want prone once", i, got)
		}
	}
	// With nobody in its area, a firing with nobody named catches nobody.
	res, err := r.fireRaw(t, empty, nil)
	if err != nil || len(res.GetFiring().GetCaught()) != 0 {
		t.Errorf("FireTrap(nobody there) = %v, %v; want a firing that caught nobody", res, err)
	}
	// The second firing found Toren prone already: undoing it leaves him prone; undoing the first takes it away.
	r.undoLast(t) // the empty one
	r.undoLast(t)
	if got := conditions(); len(got) != 1 {
		t.Errorf("Toren's conditions after undoing the second firing = %v, want prone still", got)
	}
	r.undoLast(t)
	if got := conditions(); len(got) != 0 {
		t.Errorf("Toren's conditions after undoing the first firing = %v, want none", got)
	}
}

// settleTrapDmg settles a trap damage with a typed amount (nil for the rolled one).
func (r *trapRig) settleTrapDmg(t *testing.T, apply bool, key, id string, amount *int32) (*playv1.TrapDamage, error) {
	t.Helper()
	if apply {
		res, err := r.master.play.ApplyTrapDamage(t.Context(), connect.NewRequest(&playv1.ApplyTrapDamageRequest{CampaignId: r.campaignID, TrapDamageId: id, IdempotencyKey: key, Amount: amount}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetDamage(), nil
	}
	res, err := r.master.play.DiscardTrapDamage(t.Context(), connect.NewRequest(&playv1.DiscardTrapDamageRequest{CampaignId: r.campaignID, TrapDamageId: id, IdempotencyKey: key}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetDamage(), nil
}

// TestTrapDamageSettlingRefusals: the answers of Apply and DiscardTrapDamage that traps_test.go
// does not reach: a damage that is not there, an amount out of range, a key that stands for
// another change, the key of an event of the session, a discard and its retry.
func TestTrapDamageSettlingRefusals(t *testing.T) { //nolint:tparallel // the subtests share one rig and go in order
	t.Parallel()
	r := newTrapRig(t)
	pits := make([]*mapsv1.MapPoint, 4)
	for i := range pits {
		pits[i] = r.trap(t, "Poço", 12+i, 7, pit("1d6"), manualTrap)
		r.place(t, r.toren.GetId(), 12+i, 7)
	}
	// Four firings of one die: 4, 5, 6 and 2.
	var keyOfFiring string
	for i, face := range []int{4, 5, 6, 2} {
		r.h.roller.queue(face)
		if i == 0 {
			keyOfFiring = newKey()
			if _, err := r.fireWithKey(t, keyOfFiring, pits[i], r.toren.GetId()); err != nil {
				t.Fatalf("FireTrap(%d) error = %v", i, err)
			}
			continue
		}
		if _, err := r.fireByHand(t, pits[i], r.toren.GetId()); err != nil {
			t.Fatalf("FireTrap(%d) error = %v", i, err)
		}
	}
	damages := r.trapDamages(t)
	if len(damages) != 4 {
		t.Fatalf("trap damages = %d, want 4", len(damages))
	}
	ids := make([]string, len(damages))
	for i, d := range damages {
		ids[i] = d.GetId()
	}
	before := r.vitals(t, r.toren).GetHitPointsCurrent()

	t.Run("a damage that is not there", func(t *testing.T) {
		_, err := r.settleTrapDmg(t, true, newKey(), "x", nil)
		wantTrapRefusal(t, "ApplyTrapDamage(an id that is no UUID)", err, "not_found|not_found: trap damage not found")
		_, err = r.settleTrapDmg(t, false, newKey(), uuid.New().String(), nil)
		wantTrapRefusal(t, "DiscardTrapDamage(an id that is nobody's)", err, "not_found|not_found: trap damage not found")
		_, err = r.settleTrapDmg(t, false, "x", ids[0], nil)
		wantTrapRefusal(t, "DiscardTrapDamage(a key that is no UUID)", err, "invalid_argument|invalid_argument: idempotency_key must be a UUID")
	})

	t.Run("an amount out of range", func(t *testing.T) {
		for _, amount := range []int32{-1, maxHitPointChange + 1} {
			_, err := r.settleTrapDmg(t, true, newKey(), ids[0], proto.Int32(amount))
			wantTrapRefusal(t, "ApplyTrapDamage(an amount of "+strconv.Itoa(int(amount))+")", err, "invalid_argument|invalid_argument: amount must be 0 to 9999")
		}
		_, err := r.caio.play.ApplyTrapDamage(t.Context(), connect.NewRequest(&playv1.ApplyTrapDamageRequest{CampaignId: r.campaignID, TrapDamageId: ids[0], IdempotencyKey: newKey()}))
		wantTrapRefusal(t, "ApplyTrapDamage(a player)", err, "permission_denied|permission_denied: only the campaign's master can do this")
		_, err = r.caio.play.DiscardTrapDamage(t.Context(), connect.NewRequest(&playv1.DiscardTrapDamageRequest{CampaignId: r.campaignID, TrapDamageId: ids[0], IdempotencyKey: newKey()}))
		wantTrapRefusal(t, "DiscardTrapDamage(a player)", err, "permission_denied|permission_denied: only the campaign's master can do this")
		if got := r.vitals(t, r.toren).GetHitPointsCurrent(); got != before {
			t.Errorf("Toren's hit points = %d, want %d: nothing was settled", got, before)
		}
	})

	t.Run("a key the history already holds for another change", func(t *testing.T) {
		// Finding: the key of the firing of the first damage is an event of the session, so
		// settling a damage with it is refused as another change, not as a retry.
		_, err := r.settleTrapDmg(t, true, keyOfFiring, ids[0], nil)
		wantTrapRefusal(t, "ApplyTrapDamage(the key of a firing)", err, "invalid_argument|invalid_argument: idempotency_key was already used for another change")
	})

	t.Run("a discard and its retry", func(t *testing.T) {
		key := newKey()
		first, err := r.settleTrapDmg(t, false, key, ids[1], nil)
		if err != nil || first.GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_DISCARDED || first.GetAppliedAmount() != 0 {
			t.Fatalf("DiscardTrapDamage() = %v, %v; want it discarded", first, err)
		}
		again, err := r.settleTrapDmg(t, false, key, ids[1], nil)
		if err != nil || !proto.Equal(first, again) {
			t.Errorf("the retry = %v, %v; want the first answer %v", again, err, first)
		}
		_, err = r.settleTrapDmg(t, true, key, ids[1], nil)
		wantTrapRefusal(t, "ApplyTrapDamage(the key of a discard)", err, "invalid_argument|invalid_argument: idempotency_key was used for another change")
		_, err = r.settleTrapDmg(t, false, newKey(), ids[1], nil)
		wantTrapRefusal(t, "DiscardTrapDamage(a damage discarded already)", err, "failed_precondition|failed_precondition: the damage was applied or discarded already|ENCOUNTER_BLOCKED_REASON_DAMAGE_RESOLVED")
		_, err = r.settleTrapDmg(t, true, newKey(), ids[1], nil)
		wantTrapRefusal(t, "ApplyTrapDamage(a damage discarded already)", err, "failed_precondition|failed_precondition: the damage was applied or discarded already|ENCOUNTER_BLOCKED_REASON_DAMAGE_RESOLVED")
		if got := r.vitals(t, r.toren).GetHitPointsCurrent(); got != before {
			t.Errorf("Toren's hit points = %d, want %d: a discard takes nothing away", got, before)
		}
	})

	t.Run("the key of an event the damage's own row does not remember", func(t *testing.T) {
		// Finding: a damage_applied event of the session that names the damage and carries the key
		// makes the settling a retry even though the damage row never kept the key; the answer is
		// the damage as it is (still rolled) and nothing is applied. A key of another damage's event is refused.
		key, other := newKey(), newKey()
		r.writeEvent(t, eventDamageApplied, &key, actionEvent{Pending: ids[2]})
		r.writeEvent(t, eventDamageApplied, &other, actionEvent{Pending: ids[3]})
		got, err := r.settleTrapDmg(t, true, key, ids[2], nil)
		if err != nil || got.GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED {
			t.Fatalf("ApplyTrapDamage(the key of its event) = %v, %v; want the damage as it is, still rolled", got, err)
		}
		_, err = r.settleTrapDmg(t, true, other, ids[2], nil)
		wantTrapRefusal(t, "ApplyTrapDamage(the key of another damage's event)", err, "invalid_argument|invalid_argument: idempotency_key was already used for another change")
		if got := r.vitals(t, r.toren).GetHitPointsCurrent(); got != before {
			t.Errorf("Toren's hit points = %d, want %d: a retry applies nothing", got, before)
		}
	})
}

// updateTrapDamages writes the golden files of ListTrapDamages again.
var updateTrapDamages = flag.Bool("update-trap-damages", false, "write the golden file of ListTrapDamages again")

// TestListTrapDamagesGolden: the master reads the damage that waits in the open combat and the
// damage that waits from a combat that ended or a firing outside one, with the trap's and the
// character's names; a player reads nothing of it. The answer is a golden file with the ids
// and the times left out.
func TestListTrapDamagesGolden(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	outside := r.trap(t, "Fosso Fora", 12, 7, pit("1d6"), manualTrap)
	inside := r.trap(t, "Fosso Dentro", 14, 7, pit("1d6"), manualTrap)
	r.place(t, r.pens.GetId(), 12, 7)
	r.h.roller.queue(3)
	if _, err := r.fireByHand(t, outside, r.pens.GetId()); err != nil {
		t.Fatalf("FireTrap(outside) error = %v", err)
	}
	r.fight(t)
	r.h.roller.queue(5)
	if _, err := r.fireByHand(t, inside, r.id(t, "Toren")); err != nil {
		t.Fatalf("FireTrap(inside) error = %v", err)
	}
	_, err := r.caio.play.ListTrapDamages(t.Context(), connect.NewRequest(&playv1.ListTrapDamagesRequest{CampaignId: r.campaignID}))
	wantTrapRefusal(t, "ListTrapDamages(a player)", err, "permission_denied|permission_denied: only the campaign's master can do this")

	res, err := r.master.play.ListTrapDamages(t.Context(), connect.NewRequest(&playv1.ListTrapDamagesRequest{CampaignId: r.campaignID}))
	if err != nil {
		t.Fatalf("ListTrapDamages() error = %v", err)
	}
	raw, err := protojson.Marshal(res.Msg)
	if err != nil {
		t.Fatal(err)
	}
	for id, name := range map[string]string{
		r.id(t, "Toren"): "combatant-toren", r.toren.GetId(): "toren", r.pens.GetId(): "pens", outside.GetId(): "pit-outside", inside.GetId(): "pit-inside", r.get(t, r.master).GetId(): "encounter",
	} {
		raw = bytes.ReplaceAll(raw, []byte(id), []byte(name))
	}
	var tree map[string][]map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	for _, d := range tree["damages"] {
		d["id"] = "ID"
		for _, k := range []string{"createdAt", "sessionStartedAt"} {
			if _, ok := d[k]; ok {
				d[k] = "TIME"
			}
		}
	}
	got, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", "trap_damages_master.golden.json")
	if *updateTrapDamages {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the golden file: %v (write it with -update-trap-damages)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ListTrapDamages differs from %s:\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}
