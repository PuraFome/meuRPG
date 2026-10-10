package play

import (
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The combat without a grid, the "teatro da mente" (MR-025, RN-25, ADR-0017;
// Etapa 10, slice 10.5b). These tests need the database
// (MEURPG_TEST_DATABASE_URL). The fixtures are the ones of the combat tests on a
// map (newArmed, newCasters, newSummoners), started without a map: nobody has a
// square, and what the server would have checked about places is the master's word.

var (
	blockedTheatreOnly     = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_THEATRE_ONLY
	blockedNeedsAMap       = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NEEDS_A_MAP
	blockedTheatreHasNoMap = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_THEATRE_HAS_NO_MAP
	blockedNoOpportunity   = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_OPPORTUNITY
	blockedReactionUsed    = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_REACTION_USED
	blockedTooFar          = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TOO_FAR
)

// theatreThree is threeAndAGoblin without a map: Toren first, then Pensantus, then
// the goblin (revealed) and Brisa.
func (a *armed) theatreThree(t *testing.T) *playv1.Encounter {
	t.Helper()
	return a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		theatre:  true,
	})
}

// theatreCasters is castersFight without a map: Pensantus first, then Toren, Brisa,
// the goblins and the Capitão (revealed).
func (a *armed) theatreCasters(t *testing.T, goblins int32) *playv1.Encounter {
	t.Helper()
	npcs := []*playv1.Participant{{CharacterId: a.goblin.GetId(), Count: goblins}, {CharacterId: a.capitao.GetId()}}
	rolls := make([]int, 0, int(goblins)+1)
	for range goblins + 1 {
		rolls = append(rolls, 1)
	}
	reveal := []string{"Capitão Goblin"}
	if goblins == 1 {
		reveal = append(reveal, "Goblin")
	} else {
		for i := range goblins {
			reveal = append(reveal, fmt.Sprintf("Goblin %d", i+1))
		}
	}
	return a.start(t, plan{
		npcs: npcs, npcRolls: rolls, reveal: reveal, theatre: true,
		players: map[string]int32{"Pensantus": 20, "Toren": 15, "Brisa": 10},
	})
}

// spend calls SpendMovement as u, in whole feet, by label.
func (a *armed) spend(t *testing.T, u *user, e *playv1.Encounter, label string, ft int32, key ...string) (*playv1.SpendMovementResponse, error) {
	t.Helper()
	k := newKey()
	if len(key) > 0 {
		k = key[0]
	}
	res, err := u.combat.SpendMovement(t.Context(), connect.NewRequest(&playv1.SpendMovementRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, label), IdempotencyKey: k, DistanceFt: ft,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) mustSpend(t *testing.T, u *user, e *playv1.Encounter, label string, ft int32) *playv1.SpendMovementResponse {
	t.Helper()
	res, err := a.spend(t, u, e, label, ft)
	if err != nil {
		t.Fatalf("SpendMovement(%s, %d ft) error = %v", label, ft, err)
	}
	return res
}

// offer calls OfferOpportunity as u: mover left the reach of reactor.
func (a *armed) offer(t *testing.T, u *user, e *playv1.Encounter, mover, reactor string, key ...string) (*playv1.OfferOpportunityResponse, error) {
	t.Helper()
	k := newKey()
	if len(key) > 0 {
		k = key[0]
	}
	res, err := u.combat.OfferOpportunity(t.Context(), connect.NewRequest(&playv1.OfferOpportunityRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: k, MoverId: a.id(t, mover), ReactorId: a.id(t, reactor),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) mustOffer(t *testing.T, e *playv1.Encounter, mover, reactor string) *playv1.OfferOpportunityResponse {
	t.Helper()
	res, err := a.offer(t, a.master, e, mover, reactor)
	if err != nil {
		t.Fatalf("OfferOpportunity(%s leaves %s's reach) error = %v", mover, reactor, err)
	}
	return res
}

// withdraw calls WithdrawOpportunity as u.
func (a *armed) withdraw(t *testing.T, u *user, e *playv1.Encounter, offerID string) error {
	t.Helper()
	_, err := u.combat.WithdrawOpportunity(t.Context(), connect.NewRequest(&playv1.WithdrawOpportunityRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), OpportunityOfferId: offerID, IdempotencyKey: newKey(),
	}))
	return err
}

// theatreCombatant is the combatant as u reads the combat.
func (a *armed) theatreCombatant(t *testing.T, u *user, label string) *playv1.Combatant {
	t.Helper()
	return byLabel(t, a.get(t, u), label)
}

// wantNoSquares fails when the combat has a map, a grid or any combatant with a
// square.
func wantNoSquares(t *testing.T, who string, e *playv1.Encounter) {
	t.Helper()
	if e.GetMode() != playv1.EncounterMode_ENCOUNTER_MODE_THEATRE || e.GetMapId() != "" || e.GetMapPointId() != "" || e.GetGridColumns() != 0 || e.GetGridRows() != 0 {
		t.Errorf("%s: combat = mode %v, map %q, point %q, grid %d x %d; want THEATRE with no map and no grid", who, e.GetMode(), e.GetMapId(), e.GetMapPointId(), e.GetGridColumns(), e.GetGridRows())
	}
	for _, c := range e.GetCombatants() {
		if c.GetPlaced() || c.GetCol() != 0 || c.GetRow() != 0 {
			t.Errorf("%s: %s has a square (%v, %d, %d), want none", who, c.GetLabel(), c.GetPlaced(), c.GetCol(), c.GetRow())
		}
	}
}

// setCombatWithoutMap saves the table's rules with "combate com mapa" off, as the master.
func (a *armed) setCombatWithoutMap(t *testing.T, without bool) {
	t.Helper()
	got, err := a.master.campaigns.GetTableRules(t.Context(), connect.NewRequest(&campaignsv1.GetTableRulesRequest{CampaignId: a.campaignID}))
	if err != nil {
		t.Fatalf("GetTableRules() error = %v", err)
	}
	rules := got.Msg.GetRules()
	rules.CombatStartsWithMap = !without
	if _, err := a.master.campaigns.SetTableRules(t.Context(), connect.NewRequest(&campaignsv1.SetTableRulesRequest{CampaignId: a.campaignID, Rules: rules})); err != nil {
		t.Fatalf("SetTableRules() error = %v", err)
	}
}

// startPlain starts a combat with the party and one goblin, as the master, with the
// mode asked (UNSPECIFIED: the table's rule).
func (a *armed) startPlain(t *testing.T, mode playv1.EncounterMode, point string) (*playv1.StartEncounterResponse, error) {
	t.Helper()
	res, err := a.master.combat.StartEncounter(t.Context(), connect.NewRequest(&playv1.StartEncounterRequest{
		CampaignId: a.campaignID, IdempotencyKey: newKey(), Name: "Emboscada", MapPointId: point, Mode: mode,
		Participants: []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// TestRN25_ACombatStartsInOneModeForGood: the mode is the request's, or, when it
// says none, the table's rule "combate com mapa" read when the combat starts; a
// combat on a map is what a client that does not know the field has always got. A
// combat without a map needs no map and takes none, and leaves the session's
// current map alone; no call changes the mode afterwards.
func TestRN25_ACombatStartsInOneModeForGood(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	end := func(e *playv1.Encounter) {
		t.Helper()
		if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
			t.Fatalf("EndEncounter() error = %v", err)
		}
	}

	// A client that never heard of the field, and the table's default ("com mapa"): a
	// combat on the map, as it has always been.
	res, err := a.startPlain(t, playv1.EncounterMode_ENCOUNTER_MODE_UNSPECIFIED, "")
	if err != nil {
		t.Fatalf("StartEncounter(no mode) error = %v", err)
	}
	if e := res.GetEncounter(); e.GetMode() != playv1.EncounterMode_ENCOUNTER_MODE_GRID || e.GetMapId() != a.mapID || e.GetGridColumns() != gridColumns {
		t.Errorf("a combat started without a mode = %v on map %q (%d columns); want GRID on the current map", e.GetMode(), e.GetMapId(), e.GetGridColumns())
	}
	end(res.GetEncounter())

	// The table's rule says "sem mapa": the same request now starts a theatre combat.
	a.setCombatWithoutMap(t, true)
	res, err = a.startPlain(t, playv1.EncounterMode_ENCOUNTER_MODE_UNSPECIFIED, "")
	if err != nil {
		t.Fatalf("StartEncounter(no mode, the table starts without a map) error = %v", err)
	}
	wantNoSquares(t, "the default from the table's rule", res.GetEncounter())
	end(res.GetEncounter())

	// An explicit mode wins over the rule, both ways.
	res, err = a.startPlain(t, playv1.EncounterMode_ENCOUNTER_MODE_GRID, "")
	if err != nil {
		t.Fatalf("StartEncounter(GRID) error = %v", err)
	}
	if e := res.GetEncounter(); e.GetMode() != playv1.EncounterMode_ENCOUNTER_MODE_GRID || e.GetMapId() != a.mapID {
		t.Errorf("GRID asked = %v on %q, want GRID on the current map", e.GetMode(), e.GetMapId())
	}
	end(res.GetEncounter())
	a.setCombatWithoutMap(t, false)
	res, err = a.startPlain(t, playv1.EncounterMode_ENCOUNTER_MODE_THEATRE, "")
	if err != nil {
		t.Fatalf("StartEncounter(THEATRE) error = %v", err)
	}
	wantNoSquares(t, "THEATRE asked", res.GetEncounter())
	enc := res.GetEncounter()

	// A map point (and a mode that is not one) are refused; the combat is still the
	// session's one, and the session's current map did not move.
	end(enc)
	point := a.h.newMap(a.campaignID, 10)
	var pointID string
	if err := a.h.pool.QueryRow(t.Context(),
		`INSERT INTO map_points (map_id, kind, name, x_bp, y_bp, target_map_id, created_at, updated_at)
		 VALUES ($1, 'battle', 'Emboscada', 100, 100, $2, now(), now()) RETURNING id`, a.mapID, point).Scan(&pointID); err != nil {
		t.Fatalf("insert point: %v", err)
	}
	_, err = a.startPlain(t, playv1.EncounterMode_ENCOUNTER_MODE_THEATRE, pointID)
	wantBlockedBy(t, "a theatre combat from a battle point", err, blockedTheatreHasNoMap)
	_, err = a.startPlain(t, playv1.EncounterMode(99), "")
	wantCode(t, "StartEncounter(mode 99)", err, connect.CodeInvalidArgument)
	if live := a.master.liveSession(t, a.campaignID); live.GetCurrentMapId() != a.mapID {
		t.Errorf("current map = %q after the refusals, want %q", live.GetCurrentMapId(), a.mapID)
	}

	// A battle point leads to a map, so it implies a combat on one: with no mode, the
	// table's rule "sem mapa" does not turn it into a combat without a map.
	a.setCombatWithoutMap(t, true)
	res, err = a.startPlain(t, playv1.EncounterMode_ENCOUNTER_MODE_UNSPECIFIED, pointID)
	if err != nil {
		t.Fatalf("StartEncounter(a battle point, no mode, the table starts without a map) error = %v", err)
	}
	if e := res.GetEncounter(); e.GetMode() != playv1.EncounterMode_ENCOUNTER_MODE_GRID || e.GetMapId() != point {
		t.Errorf("a combat from a battle point with no mode = %v on %q, want GRID on the point's map %q", e.GetMode(), e.GetMapId(), point)
	}
	end(res.GetEncounter())
	a.setCombatWithoutMap(t, false)

	// A theatre combat needs no current map at all.
	if _, err := a.master.play.SetCurrentMap(t.Context(), connect.NewRequest(&playv1.SetCurrentMapRequest{CampaignId: a.campaignID, MapId: ""})); err != nil {
		t.Fatalf("SetCurrentMap(none) error = %v", err)
	}
	if _, err := a.startPlain(t, playv1.EncounterMode_ENCOUNTER_MODE_GRID, ""); err == nil {
		t.Fatal("a combat on a map started with no current map")
	} else {
		wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_CURRENT_MAP)
	}
	res, err = a.startPlain(t, playv1.EncounterMode_ENCOUNTER_MODE_THEATRE, "")
	if err != nil {
		t.Fatalf("StartEncounter(THEATRE, no current map) error = %v", err)
	}
	e := res.GetEncounter()
	wantNoSquares(t, "no current map", e)

	// The mode is for the whole combat: the setup, the beginning, a reinforcement and the
	// end read the same. (No call changes it, and the table keeps the CHECK.)
	a.h.roller.queue(5)
	for _, label := range []string{"Pensantus", "Toren", "Brisa"} {
		if _, err := a.master.combat.SubmitInitiative(t.Context(), connect.NewRequest(&playv1.SubmitInitiativeRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: byLabel(t, e, label).GetId(), IdempotencyKey: newKey(),
			Roll: &playv1.SubmitInitiativeRequest_D20Face{D20Face: 10},
		})); err != nil {
			t.Fatalf("SubmitInitiative(%s) error = %v", label, err)
		}
	}
	begun, err := a.master.combat.BeginCombat(t.Context(), connect.NewRequest(&playv1.BeginCombatRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()}))
	if err != nil {
		t.Fatalf("BeginCombat() error = %v", err)
	}
	wantNoSquares(t, "after BeginCombat", begun.Msg.GetEncounter())
	added, err := a.master.combat.AddCombatants(t.Context(), connect.NewRequest(&playv1.AddCombatantsRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), Participants: []*playv1.Participant{{CharacterId: a.capitao.GetId()}},
	}))
	if err != nil {
		t.Fatalf("AddCombatants() error = %v", err)
	}
	wantNoSquares(t, "after a reinforcement", added.Msg.GetEncounter())
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	last := a.get(t, a.caio)
	if last.GetStatus() != playv1.EncounterStatus_ENCOUNTER_STATUS_ENDED {
		t.Errorf("status = %v, want ENDED", last.GetStatus())
	}
	wantNoSquares(t, "after EndEncounter, as a player", last)

	// The database says the same: three combats on a grid, two without, with the grid CHECK.
	var theatre, grid int
	if err := a.h.pool.QueryRow(t.Context(),
		`SELECT count(*) FILTER (WHERE mode = 'theatre'), count(*) FILTER (WHERE mode = 'grid') FROM encounters`).Scan(&theatre, &grid); err != nil {
		t.Fatalf("count the modes: %v", err)
	}
	if theatre != 3 || grid != 3 {
		t.Errorf("encounters by mode = %d theatre, %d grid; want 3 and 3", theatre, grid)
	}
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE encounters SET grid_columns = 20 WHERE mode = 'theatre'`); err == nil {
		t.Error("the table accepted a grid on a combat without one")
	}
}

// TestRN25_ReadsAndTheStreamCarryTheMode: the combat read by every viewer, and the
// hint of the stream, say the mode, so a screen draws a list or a map without asking.
func TestRN25_ReadsAndTheStreamCarryTheMode(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	master, player := a.master.watch(t, a.campaignID), a.caio.watch(t, a.campaignID)
	master.ready(t)
	player.ready(t)
	e := a.theatreThree(t)
	for who, u := range map[string]*user{"the master": a.master, "a player": a.caio} {
		if got := a.get(t, u); got.GetMode() != playv1.EncounterMode_ENCOUNTER_MODE_THEATRE {
			t.Errorf("GetEncounter as %s: mode = %v, want THEATRE", who, got.GetMode())
		}
	}
	for who, w := range map[string]*watcher{"the master": master, "a player": player} {
		var sawHint bool
		for range 4 {
			ev := w.nextChange(t)
			if ec := ev.GetEncounterChanged(); ec != nil && ec.GetEncounterId() == e.GetId() {
				sawHint = true
				if ec.GetMode() != playv1.EncounterMode_ENCOUNTER_MODE_THEATRE {
					t.Errorf("encounter_changed for %s: mode = %v, want THEATRE", who, ec.GetMode())
				}
			}
			if sawHint {
				break
			}
		}
		if !sawHint {
			t.Errorf("%s got no encounter_changed", who)
		}
	}
}

// TestRN25_SpendMovementByNumber: the player spends movement by number on their own
// turn, never more than the turn has (the speed, twice after the Dash action), the
// answer says what is left, the master spends for anyone the same way and the undo
// puts it back. Nothing has a square.
func TestRN25_SpendMovementByNumber(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.theatreThree(t) // Toren first: 30 ft
	torenLeft := func(u *user) int32 { return a.theatreCombatant(t, u, "Toren").GetMovementLeftDft() }
	if got := torenLeft(a.caio); got != 300 {
		t.Fatalf("Toren's movement = %d tenths of a foot, want 300 (30 ft)", got)
	}

	// Spends, and the answer says what is left ("Restam 3,0 m" is 100).
	key := newKey()
	first, err := a.spend(t, a.caio, e, "Toren", 20, key)
	if err != nil {
		t.Fatalf("SpendMovement(20 ft) error = %v", err)
	}
	c := byLabel(t, first.GetEncounter(), "Toren")
	if first.GetMovementLeftDft() != 100 || c.GetMovementLeftDft() != 100 || c.GetMovementUsedDft() != 200 || c.GetMovementUsedFt() != 20 {
		t.Errorf("after 20 ft: left %d (combatant %d), used %d tenths and %d ft; want 100, 100, 200 and 20", first.GetMovementLeftDft(), c.GetMovementLeftDft(), c.GetMovementUsedDft(), c.GetMovementUsedFt())
	}
	wantNoSquares(t, "after SpendMovement", first.GetEncounter())
	// A retry under the same key spends nothing twice.
	before := a.events(t)
	again, err := a.spend(t, a.caio, e, "Toren", 20, key)
	if err != nil || again.GetMovementLeftDft() != 100 || a.events(t) != before {
		t.Errorf("the retry = left %d, %d new events, error %v; want 100, none and no error", again.GetMovementLeftDft(), a.events(t)-before, err)
	}

	// Never more than the turn has: 15 ft is 5 ft too many, and nothing is spent.
	_, err = a.spend(t, a.caio, e, "Toren", 15)
	b := wantBlockedBy(t, "SpendMovement past the speed", err, blockedTooFar)
	if b.GetMissingFt() != 5 || b.GetMissingDft() != 50 {
		t.Errorf("TOO_FAR missing = %d ft, %d tenths; want 5 and 50", b.GetMissingFt(), b.GetMissingDft())
	}
	if got := torenLeft(a.caio); got != 100 {
		t.Errorf("a refused spend left %d, want 100", got)
	}
	for _, ft := range []int32{0, -5, 601} {
		_, err = a.spend(t, a.caio, e, "Toren", ft)
		wantCode(t, fmt.Sprintf("SpendMovement(%d ft)", ft), err, connect.CodeInvalidArgument)
	}

	// The master's log has the line, and the master can undo it.
	log := a.log(t, a.master, e)
	var line *playv1.CombatLogEntry
	for _, r := range log.GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_MOVED {
				line = en
			}
		}
	}
	if line == nil || line.GetActorLabel() != "Toren" || line.GetDistanceFt() != 20 || line.GetDistanceDft() != 200 || !line.GetUndoable() {
		t.Fatalf("the log's line = %v, want Toren's move of 20 ft, undoable", line)
	}
	a.undoLast(t, e)
	if got := torenLeft(a.caio); got != 300 {
		t.Errorf("after the undo Toren has %d, want 300", got)
	}

	// The Dash action doubles the turn's movement; the limit follows it.
	if _, err := a.action(t, a.caio, e, "Toren", "standard:dash"); err != nil {
		t.Fatalf("Dash error = %v", err)
	}
	if got := torenLeft(a.caio); got != 600 {
		t.Errorf("after the Dash Toren has %d, want 600", got)
	}
	a.mustSpend(t, a.caio, e, "Toren", 55)
	_, err = a.spend(t, a.caio, e, "Toren", 10)
	wantBlockedBy(t, "SpendMovement past the dashed speed", err, blockedTooFar)
	if res := a.mustSpend(t, a.caio, e, "Toren", 5); res.GetMovementLeftDft() != 0 {
		t.Errorf("after spending it all: left %d, want 0", res.GetMovementLeftDft())
	}

	// Out of turn, a player cannot; nobody else's combatant is theirs; a hidden NPC is
	// not found for a player (RN-10).
	_, err = a.spend(t, a.ana, e, "Pensantus", 5)
	wantBlockedBy(t, "SpendMovement out of turn", err, blockedNotYourTurn)
	_, err = a.spend(t, a.caio, e, "Pensantus", 5)
	wantCode(t, "SpendMovement for another player's character", err, connect.CodePermissionDenied)
	// The master spends for an NPC the same way, out of its turn too, held to its speed.
	a.mustSpend(t, a.master, e, "Goblin", 30)
	_, err = a.spend(t, a.master, e, "Goblin", 5)
	wantBlockedBy(t, "the master's SpendMovement past an NPC's speed", err, blockedTooFar)
	// And for a player's character that is on turn.
	if a.theatreCombatant(t, a.master, "Toren").GetMovementLeftDft() != 0 {
		t.Errorf("the master reads Toren's movement left as %d, want 0", a.theatreCombatant(t, a.master, "Toren").GetMovementLeftDft())
	}
	_, err = a.spend(t, a.master, e, "Toren", 5)
	wantBlockedBy(t, "the master's SpendMovement past a player's speed", err, blockedTooFar)

	// A new turn gives the movement back.
	a.mustEndTurn(t, a.caio, e) // Pensantus
	if got := torenLeft(a.caio); got != 0 {
		t.Errorf("Toren outside his turn: left %d, still what he spent", got)
	}
	a.passTo(t, e, "Toren")
	if got := torenLeft(a.caio); got != 300 {
		t.Errorf("Toren's next turn starts with %d, want 300", got)
	}

	// The hidden Capitão is never found by a player.
	secret := a.start2(t)
	_, err = a.spend(t, a.caio, secret, "Capitão Goblin", 5)
	wantCode(t, "a player's SpendMovement for a hidden NPC", err, connect.CodeNotFound)
}

// start2 ends the combat and starts another one without a map, with a hidden Capitão.
func (a *armed) start2(t *testing.T) *playv1.Encounter {
	t.Helper()
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: a.get(t, a.master).GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	return a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.capitao.GetId()}},
		npcRolls: []int{1},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 5},
		theatre:  true,
	})
}

// TestRN25_WhatNeedsAMapIsRefused: a combat without a map has no squares, so a move,
// a jump, a placement, a trap and the familiar's eyes answer a typed refusal; the
// read of where a combatant can go is valid and empty; and a combat on a map keeps
// SpendMovement and OfferOpportunity out.
func TestRN25_WhatNeedsAMapIsRefused(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.theatreThree(t)
	toren := a.id(t, "Toren")

	// A move, a jump and a placement: the master's and the player's.
	for who, u := range map[string]*user{"the master": a.master, "the player": a.caio} {
		for name, edit := range map[string]func(*playv1.MoveCombatantRequest){
			"a move":  func(r *playv1.MoveCombatantRequest) { r.Col, r.Row = 3, 3 },
			"a jump":  func(r *playv1.MoveCombatantRequest) { r.Col, r.Row, r.Jump = 3, 3, playv1.JumpKind_JUMP_KIND_LONG },
			"a high":  func(r *playv1.MoveCombatantRequest) { r.Jump, r.JumpHeightDft = playv1.JumpKind_JUMP_KIND_HIGH, 20 },
			"forced":  func(r *playv1.MoveCombatantRequest) { r.Col, r.Row, r.Forced = 5, 5, who == "the master" },
			"a plain": func(r *playv1.MoveCombatantRequest) { r.Col, r.Row = 0, 0 },
		} {
			req := &playv1.MoveCombatantRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: toren, IdempotencyKey: newKey()}
			edit(req)
			_, err := u.combat.MoveCombatant(t.Context(), connect.NewRequest(req))
			wantBlockedBy(t, who+"'s "+name+" without a map", err, blockedNeedsAMap)
		}
	}
	wantNoSquares(t, "after the refused moves", a.get(t, a.master))

	// Where a combatant can go: valid and empty, with what is left to spend.
	for who, u := range map[string]*user{"the master": a.master, "the player": a.caio} {
		res, err := u.combat.GetMoveOptions(t.Context(), connect.NewRequest(&playv1.GetMoveOptionsRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: toren}))
		if err != nil {
			t.Fatalf("GetMoveOptions as %s error = %v", who, err)
		}
		if len(res.Msg.GetReachable()) != 0 || len(res.Msg.GetRefused()) != 0 || res.Msg.GetMovementLeftDft() != 300 {
			t.Errorf("GetMoveOptions as %s = %v, want no squares and 300 tenths left", who, res.Msg)
		}
	}
	// Out of turn is still out of turn for a player.
	_, err := a.master.combat.GetMoveOptions(t.Context(), connect.NewRequest(&playv1.GetMoveOptionsRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Pensantus")}))
	if err != nil {
		t.Errorf("GetMoveOptions as the master for Pensantus error = %v", err)
	}
	_, err = a.ana.combat.GetMoveOptions(t.Context(), connect.NewRequest(&playv1.GetMoveOptionsRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Pensantus")}))
	wantBlockedBy(t, "GetMoveOptions out of turn", err, blockedNotYourTurn)

	// A combat on a map keeps these two out (the server finds the opportunity
	// attacks there, and a move is a move on the map).
	g := newArmed(t)
	ge := g.threeAndAGoblin(t)
	_, err = g.spend(t, g.caio, ge, "Toren", 5)
	wantBlockedBy(t, "SpendMovement on a map", err, blockedTheatreOnly)
	_, err = g.offer(t, g.master, ge, "Toren", "Goblin")
	wantBlockedBy(t, "OfferOpportunity on a map", err, blockedTheatreOnly)
	if got := g.get(t, g.master).GetMode(); got != playv1.EncounterMode_ENCOUNTER_MODE_GRID {
		t.Errorf("a combat started the way it always was has mode %v, want GRID", got)
	}
}

// TestRN25_OpportunityAttacksAreOfferedByTheMaster: the master says who left whose
// reach; the reactor's player gets the prompt (nobody else does), attacks with the
// reaction (one a round) or declines; the master answers for an NPC, takes an offer
// back, and the server refuses what cannot be offered.
func TestRN25_OpportunityAttacksAreOfferedByTheMaster(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.theatreThree(t)

	// The goblin's turn: Toren first, then Pensantus, then the goblin.
	a.passTo(t, e, "Goblin")
	key := newKey()
	made, err := a.offer(t, a.master, e, "Goblin", "Toren", key)
	if err != nil {
		t.Fatalf("OfferOpportunity() error = %v", err)
	}
	offerID := made.GetOpportunityOfferId()
	if offerID == "" {
		t.Fatal("the answer has no offer id")
	}
	if replay, err := a.offer(t, a.master, e, "Goblin", "Toren", key); err != nil || replay.GetOpportunityOfferId() != offerID {
		t.Errorf("the replay = %v, %v; want the same offer", replay.GetOpportunityOfferId(), err)
	}
	offers := func(u *user) []*playv1.OpportunityOffer { return a.get(t, u).GetOpportunityOffers() }
	if got := offers(a.caio); len(got) != 1 || got[0].GetId() != offerID || !got[0].GetForYou() || !got[0].GetByHand() || got[0].GetMoverLabel() != "Goblin" ||
		len(got[0].GetAttacks()) == 0 || got[0].GetAttacks()[0].GetKey() != battleaxe {
		t.Errorf("Toren's player reads %v, want the offer for him, by hand, with his axe", got)
	}
	if got := offers(a.master); len(got) != 1 || !got[0].GetByHand() || got[0].GetReactorLabel() != "Toren" {
		t.Errorf("the master reads %v, want the one offer, by hand", got)
	}
	for who, u := range map[string]*user{"Pensantus's player": a.ana, "Brisa's player": a.bia} {
		if got := offers(u); len(got) != 0 {
			t.Errorf("%s reads %v, want no offer: it is not theirs", who, got)
		}
	}
	// The same offer twice is refused; so is one a player makes.
	_, err = a.offer(t, a.master, e, "Goblin", "Toren")
	wantBlockedBy(t, "the same offer twice", err, blockedNoOpportunity)
	_, err = a.offer(t, a.caio, e, "Goblin", "Toren")
	wantCode(t, "a player's OfferOpportunity", err, connect.CodePermissionDenied)

	// Toren attacks with his reaction, with no reach check: nobody has a square.
	hit, err := a.attackWithOffer(t, a.caio, e, "Toren", battleaxe, "Goblin", offerID, d20(15))
	if err != nil {
		t.Fatalf("the opportunity attack error = %v", err)
	}
	if !byLabel(t, hit.GetEncounter(), "Toren").GetReactionUsed() {
		t.Error("the opportunity attack did not spend Toren's reaction")
	}
	if got := offers(a.master); len(got) != 0 {
		t.Errorf("the offer waits after the attack: %v", got)
	}
	a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), typedDamage(1))
	// One reaction a round: Toren has none left to be offered another.
	_, err = a.offer(t, a.master, e, "Goblin", "Toren")
	wantBlockedBy(t, "a second offer in the same round", err, blockedReactionUsed)

	// A reactor with no weapon still has the unarmed strike, a melee attack: Pensantus
	// can be offered one, and declines; and the goblin is no reactor to itself or to
	// its own side.
	offerP := a.mustOffer(t, e, "Goblin", "Pensantus").GetOpportunityOfferId()
	if _, err := a.ana.combat.DeclineOpportunity(t.Context(), connect.NewRequest(&playv1.DeclineOpportunityRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), OpportunityOfferId: offerP, IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("Pensantus declining the offer, error = %v", err)
	}
	_, err = a.offer(t, a.master, e, "Goblin", "Goblin")
	wantCode(t, "a combatant leaving its own reach", err, connect.CodeInvalidArgument)
	// The mover is the one on turn.
	_, err = a.offer(t, a.master, e, "Toren", "Goblin")
	wantBlockedBy(t, "an offer for a mover that is not on turn", err, blockedNotYourTurn)

	// Brisa declines: she keeps her reaction. Another offer is taken back by the master
	// (the undo puts it back), and one the master skips.
	offerB := a.mustOffer(t, e, "Goblin", "Brisa").GetOpportunityOfferId()
	if _, err := a.bia.combat.DeclineOpportunity(t.Context(), connect.NewRequest(&playv1.DeclineOpportunityRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), OpportunityOfferId: offerB, IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("DeclineOpportunity() error = %v", err)
	}
	if byLabel(t, a.get(t, a.master), "Brisa").GetReactionUsed() || len(offers(a.master)) != 0 {
		t.Error("declining spent Brisa's reaction or left the offer")
	}
	// The offer is a line of the log (the mover as the actor, the reactor as the
	// target) for the master and the two players in it, and nobody else. The decline
	// belongs to it: it is the entry the master undoes.
	offerLine := func(u *user) *playv1.CombatLogEntry {
		t.Helper()
		var found *playv1.CombatLogEntry
		for _, r := range a.log(t, u, e).GetRounds() {
			for _, en := range r.GetEntries() {
				if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_OPPORTUNITY_OFFERED && en.GetTargetLabel() == "Brisa" {
					found = en
				}
			}
		}
		return found
	}
	line := offerLine(a.master)
	if line == nil || line.GetActorLabel() != "Goblin" || !line.GetUndoable() {
		t.Fatalf("the master's line of the offer = %v, want Goblin leaving Brisa's reach, undoable after the decline", line)
	}
	if offerLine(a.bia) == nil || offerLine(a.ana) != nil || offerLine(a.caio) != nil {
		t.Errorf("who gets the offer's line: Brisa's player %v, Pensantus's %v, Toren's %v; want only the first", offerLine(a.bia) != nil, offerLine(a.ana) != nil, offerLine(a.caio) != nil)
	}
	if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err != nil {
		t.Fatalf("the undo of the decline, through the log, error = %v", err)
	}
	if got := offers(a.bia); len(got) != 1 || got[0].GetId() != offerB {
		t.Errorf("after the undo Brisa's player reads %v, want the offer waiting again", got)
	}
	if _, err := a.bia.combat.DeclineOpportunity(t.Context(), connect.NewRequest(&playv1.DeclineOpportunityRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), OpportunityOfferId: offerB, IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("DeclineOpportunity() again error = %v", err)
	}
	offerC := a.mustOffer(t, e, "Goblin", "Brisa").GetOpportunityOfferId()
	if err := a.withdraw(t, a.bia, e, offerC); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a player's WithdrawOpportunity: %v, want permission_denied", err)
	}
	if err := a.withdraw(t, a.master, e, offerC); err != nil {
		t.Fatalf("WithdrawOpportunity() error = %v", err)
	}
	if got := offers(a.bia); len(got) != 0 || byLabel(t, a.get(t, a.master), "Brisa").GetReactionUsed() {
		t.Errorf("after the withdrawal Brisa's player reads %v (reaction used: %v), want nothing", got, byLabel(t, a.get(t, a.master), "Brisa").GetReactionUsed())
	}
	if err := a.withdraw(t, a.master, e, offerC); connect.CodeOf(err) != connect.CodeAborted {
		t.Errorf("withdrawing it twice: %v, want aborted", err)
	}
	a.undoLast(t, e) // the withdrawal: the offer waits again
	if got := offers(a.bia); len(got) != 1 || got[0].GetId() != offerC {
		t.Errorf("after the undo of the withdrawal Brisa's player reads %v, want the offer back", got)
	}
	if _, err := a.master.combat.SkipOpportunity(t.Context(), connect.NewRequest(&playv1.SkipOpportunityRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), OpportunityOfferId: offerC, IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("SkipOpportunity() error = %v", err)
	}

	// The offer is no part of what an undo takes back: it closes the chain.
	offerD := a.mustOffer(t, e, "Goblin", "Brisa").GetOpportunityOfferId()
	if id := a.log(t, a.master, e).GetUndoableEventId(); id != "" {
		t.Errorf("the log's undoable event after an offer made by hand = %q, want none", id)
	}
	if err := a.withdraw(t, a.master, e, offerD); err != nil {
		t.Fatalf("WithdrawOpportunity() error = %v", err)
	}

	// A mover that is hidden, or took the Disengage action, cannot be hit.
	if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Goblin"), IdempotencyKey: newKey(), Hidden: true,
	})); err != nil {
		t.Fatalf("SetCombatantHidden() error = %v", err)
	}
	_, err = a.offer(t, a.master, e, "Goblin", "Brisa")
	wantBlockedBy(t, "an offer for a hidden mover", err, blockedNoOpportunity)

	// A player mover: the master offers the NPC's attack, and the mover's turn waits.
	if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Goblin"), IdempotencyKey: newKey(), Hidden: false,
	})); err != nil {
		t.Fatalf("SetCombatantHidden() error = %v", err)
	}
	a.passTo(t, e, "Toren")
	waiting := a.mustOffer(t, e, "Toren", "Goblin").GetOpportunityOfferId()
	_, err = a.spend(t, a.caio, e, "Toren", 5)
	wantBlockedBy(t, "SpendMovement while an opportunity attack waits", err, reasonOpportunityPending)
	if got := offers(a.caio); len(got) != 1 || got[0].GetForYou() {
		t.Errorf("the mover's player reads %v, want the offer on him, not for him to answer", got)
	}
	if got := offers(a.ana); len(got) != 0 {
		t.Errorf("another player reads %v, want nothing", got)
	}
	if _, err := a.master.combat.SkipOpportunity(t.Context(), connect.NewRequest(&playv1.SkipOpportunityRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), OpportunityOfferId: waiting, IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("SkipOpportunity() error = %v", err)
	}
	a.mustSpend(t, a.caio, e, "Toren", 5)
	// The Disengage action: a combatant that took it provokes nothing.
	if _, err := a.action(t, a.caio, e, "Toren", "standard:disengage"); err != nil {
		t.Fatalf("Disengage error = %v", err)
	}
	_, err = a.offer(t, a.master, e, "Toren", "Goblin")
	wantBlockedBy(t, "an offer for a mover that took the Disengage action", err, blockedNoOpportunity)
}

// attackWithOffer calls RollAttack as u, answering an opportunity offer.
func (a *armed) attackWithOffer(t *testing.T, u *user, e *playv1.Encounter, attacker, key, target, offerID string, roll func(*playv1.RollAttackRequest)) (*playv1.RollAttackResponse, error) {
	t.Helper()
	req := &playv1.RollAttackRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), AttackerId: a.id(t, attacker), AttackKey: key, TargetId: a.id(t, target),
		IdempotencyKey: newKey(), OpportunityOfferId: offerID,
	}
	roll(req)
	res, err := u.combat.RollAttack(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// TestRN25_AttacksAndCoverWithoutReach: an attack reaches whoever the rules allow
// whatever the weapon: no square, no distance and nobody too far; the cover is the
// one the master marks, in its four degrees; and a hidden NPC is never in what a
// player reads (RN-10).
func TestRN25_AttacksAndCoverWithoutReach(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}, {CharacterId: a.capitao.GetId()}},
		npcRolls: []int{3, 2},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"}, // the Capitão stays hidden
		theatre:  true,
	})
	capitao := a.id(t, "Capitão Goblin")

	// The list of targets: every combatant a player may see and attack, with no
	// distance and nobody too far, melee or not; the hidden Capitão is not in it.
	opts := a.mustOptions(t, a.caio, e, "Toren")
	if len(opts.GetAttackTargets()) == 0 {
		t.Fatal("Toren has no attack targets")
	}
	for _, at := range opts.GetAttackTargets() {
		var labels []string
		for _, tg := range at.GetTargets() {
			labels = append(labels, tg.GetLabel())
			if tg.DistanceFt != nil || tg.GetTooFar() {
				t.Errorf("target %s of %s = distance %v, too far %v; want none and no", tg.GetLabel(), at.GetAttackKey(), tg.DistanceFt, tg.GetTooFar())
			}
		}
		if got := strings.Join(labels, ","); got != "Pensantus,Goblin,Brisa" && got != "Goblin,Pensantus,Brisa" && got != "Pensantus,Brisa,Goblin" && got != "Brisa,Pensantus,Goblin" && got != "Goblin,Brisa,Pensantus" && got != "Brisa,Goblin,Pensantus" {
			t.Errorf("the targets of %s = %v, want the party and the goblin, and not the hidden Capitão", at.GetAttackKey(), labels)
		}
	}
	// A melee axe at "anyone": the attack is not held to a reach, and not to a square.
	hit := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", d20(15))
	if hit.GetPendingDamage() == nil {
		t.Fatal("the hit opened no damage")
	}
	a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), typedDamage(3))
	// RN-10: what a player reads of the combat never has the hidden Capitão.
	for who, u := range map[string]*user{"Toren's player": a.caio, "Pensantus's player": a.ana} {
		for what, js := range map[string]string{
			"the combat":  protojson.Format(a.get(t, u)),
			"the options": protojson.Format(a.mustOptions(t, u, e, ownLabel(who))),
			"the log":     protojson.Format(a.log(t, u, e)),
		} {
			if strings.Contains(js, capitao) || strings.Contains(js, "Capitão") {
				t.Errorf("%s reads %s with the hidden Capitão in it: %s", who, what, js)
			}
		}
	}
	// The attack on a hidden NPC is not found for a player (the label is the master's).
	if _, err := a.caio.combat.RollAttack(t.Context(), connect.NewRequest(&playv1.RollAttackRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), AttackerId: a.id(t, "Toren"), AttackKey: battleaxe, TargetId: capitao,
		IdempotencyKey: newKey(), Roll: &playv1.RollAttackRequest_D20Face{D20Face: 15},
	})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a player's attack on a hidden NPC: %v, want not_found", err)
	}

	// Cover: the master's four degrees, with the bonus on the roll and the word to the
	// attacker; total cover takes the target out of the list and refuses the attack.
	cover := func(label string, degree playv1.CoverDegree) {
		t.Helper()
		if _, err := a.master.combat.SetCombatantCover(t.Context(), connect.NewRequest(&playv1.SetCombatantCoverRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, label), IdempotencyKey: newKey(), Cover: degree,
		})); err != nil {
			t.Fatalf("SetCombatantCover(%s, %v) error = %v", label, degree, err)
		}
	}
	a.mustEndTurn(t, a.caio, e)
	a.passTo(t, e, "Brisa")
	for _, tc := range []struct {
		degree playv1.CoverDegree
		bonus  int32
	}{
		{playv1.CoverDegree_COVER_DEGREE_NONE, 0}, {playv1.CoverDegree_COVER_DEGREE_HALF, 2}, {playv1.CoverDegree_COVER_DEGREE_THREE_QUARTERS, 5},
	} {
		cover("Goblin", tc.degree)
		for _, at := range a.mustOptions(t, a.bia, e, "Brisa").GetAttackTargets() {
			for _, tg := range at.GetTargets() {
				if tg.GetLabel() == "Goblin" && tg.GetCover() != tc.degree {
					t.Errorf("the Goblin's cover for the attacker = %v, want %v", tg.GetCover(), tc.degree)
				}
			}
		}
	}
	cover("Goblin", playv1.CoverDegree_COVER_DEGREE_HALF)
	res := a.mustAttack(t, a.bia, e, "Brisa", rapier, "Goblin", d20(10))
	if r := res.GetRoll(); r.GetCover() != playv1.CoverDegree_COVER_DEGREE_HALF || r.GetCoverSource() != playv1.CoverSource_COVER_SOURCE_MARK {
		t.Errorf("the attack's cover = %v from %v, want half, marked by the master", r.GetCover(), r.GetCoverSource())
	}
	cover("Goblin", playv1.CoverDegree_COVER_DEGREE_TOTAL)
	for _, at := range a.mustOptions(t, a.bia, e, "Brisa").GetAttackTargets() {
		for _, tg := range at.GetTargets() {
			if tg.GetLabel() == "Goblin" && !tg.GetUntargetable() {
				t.Errorf("the Goblin under total cover is targetable for %s", at.GetAttackKey())
			}
		}
	}
	a.passTo(t, e, "Toren") // (the master passes the turns: Brisa's hit waits for its damage)
	_, err := a.attack(t, a.caio, e, "Toren", battleaxe, "Goblin", d20(15))
	wantBlockedBy(t, "an attack on a target under total cover", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_COVER_TOTAL)
}

func ownLabel(who string) string {
	if strings.HasPrefix(who, "Toren") {
		return "Toren"
	}
	return "Pensantus"
}

// TestRN25_SpellsAndActionsWithoutReach: a spell with one target, with several, an
// area spell, a touch spell, the Help, Dash, Dodge and Disengage actions all work
// without squares: no range is checked, the lists carry no distance, and an area
// spell is "any number of targets".
func TestRN25_SpellsAndActionsWithoutReach(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.theatreCasters(t, 2) // Pensantus, Toren, Brisa, Goblin 1 and 2, the Capitão

	// Pensantus: the lists of spell targets have everyone, with no distance and nobody
	// out of range, whatever the spell's range.
	opts := a.mustOptions(t, a.ana, e, "Pensantus")
	for _, key := range []string{magicMissileSpell, sleepSpell, burningHands} {
		st := spellTargetsOf(opts, key)
		if st == nil || len(st.GetTargets()) < 5 {
			t.Fatalf("the targets of %s = %v, want everyone", key, st)
		}
		for _, tg := range st.GetTargets() {
			if tg.DistanceFt != nil && tg.GetCombatantId() != a.id(t, "Pensantus") {
				t.Errorf("%s: %s has a distance %d, want none", key, tg.GetLabel(), tg.GetDistanceFt())
			}
			if tg.GetTooFar() {
				t.Errorf("%s: %s is too far, want nobody", key, tg.GetLabel())
			}
		}
	}
	if st := spellTargetsOf(opts, sleepSpell); st.GetMaxTargets() != 0 {
		t.Errorf("Sono takes at most %d targets, want any number (an area)", st.GetMaxTargets())
	}

	// A cantrip with one target (a spell attack), a leveled spell with one target.
	hit := a.mustAttack(t, a.ana, e, "Pensantus", fireBolt, "Capitão Goblin", d20(19))
	if hit.GetPendingDamage() == nil {
		t.Fatal("Raio de Fogo opened no damage")
	}
	a.passTo(t, e, "Toren")
	// Dash: the speed doubles, and Toren spends all of it.
	if _, err := a.action(t, a.caio, e, "Toren", "standard:dash"); err != nil {
		t.Fatalf("Dash error = %v", err)
	}
	a.mustSpend(t, a.caio, e, "Toren", 60)
	if got := a.theatreCombatant(t, a.caio, "Toren").GetMovementLeftDft(); got != 0 {
		t.Errorf("Toren has %d left after dashing and spending 60 ft, want 0", got)
	}
	// Brisa: Cure Wounds touches Toren, with nobody next to anyone.
	a.passTo(t, e, "Brisa")
	a.correct(t, a.toren, hpIs(5))
	a.mustCast(t, a.bia, e, "Brisa", cureWounds, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)

	// Round 2: Pensantus's darts in two targets; Toren dodges; Brisa disengages.
	a.passTo(t, e, "Pensantus")
	cast := a.mustCast(t, a.ana, e, "Pensantus", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Goblin 1", 2), darts(a, t, "Goblin 2", 1)}, noCastRoll)
	if len(cast.GetCast().GetTargets()) != 2 {
		t.Errorf("Mísseis Mágicos hit %d targets, want 2", len(cast.GetCast().GetTargets()))
	}
	a.passTo(t, e, "Toren")
	if _, err := a.action(t, a.caio, e, "Toren", "standard:dodge"); err != nil {
		t.Fatalf("Dodge error = %v", err)
	}
	a.passTo(t, e, "Brisa")
	if _, err := a.action(t, a.bia, e, "Brisa", "standard:disengage"); err != nil {
		t.Fatalf("Disengage error = %v", err)
	}

	// Round 3: an area spell on every enemy (three targets); Toren helps; Brisa's cantrip.
	a.passTo(t, e, "Pensantus")
	a.h.roller.queue(5, 5, 5)
	area := a.mustCast(t, a.ana, e, "Pensantus", burningHands, slotOfLevel(1), a.at(t, "Goblin 1", "Goblin 2", "Capitão Goblin"), noCastRoll)
	if len(area.GetCast().GetTargets()) != 3 {
		t.Errorf("Mãos Flamejantes hit %d targets, want 3", len(area.GetCast().GetTargets()))
	}
	a.passTo(t, e, "Toren")
	if _, err := a.caio.contests.Help(t.Context(), connect.NewRequest(&playv1.HelpRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Toren"), IdempotencyKey: newKey(),
		Kind: playv1.HelpKind_HELP_KIND_CHECK, AllyId: a.id(t, "Brisa"), TaskKey: "skill:perception",
	})); err != nil {
		t.Fatalf("Help error = %v", err)
	}
	a.passTo(t, e, "Brisa")
	a.h.roller.queue(12)
	a.mustCast(t, a.bia, e, "Brisa", sacredFlame, nil, a.at(t, "Goblin 1"), noCastRoll)
	wantNoSquares(t, "after the whole round", a.get(t, a.master))
}

// TestRN25_SummonedCreaturesAndWildShapeWithoutASquare: a summoning spell cast in a
// combat without a map brings its creatures into the order, without a square; a
// creature takes its turn and attacks with no reach; the Wild Shape works the same;
// and the familiar's eyes are refused.
func TestRN25_SummonedCreaturesAndWildShapeWithoutASquare(t *testing.T) {
	t.Parallel()
	a := newSummoners(t)
	a.mustCastSummon(t, a.ana, a.pens, findFamiliar, nil, 0, []string{"monster:owl"}, "Nanquim") // the ritual, before the fight
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{2},
		players:  map[string]int32{"Pensantus": 20, "Sálvia": 15, "Toren": 12},
		reveal:   []string{"Goblin"},
		theatre:  true,
		setup:    true,
	})
	if _, err := a.submitFor(t, a.ana, e, "Nanquim", 8); err != nil {
		t.Fatalf("SubmitInitiative(Nanquim) error = %v", err)
	}
	begun, err := a.master.combat.BeginCombat(t.Context(), connect.NewRequest(&playv1.BeginCombatRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()}))
	if err != nil {
		t.Fatalf("BeginCombat() error = %v", err)
	}
	e = begun.Msg.GetEncounter()
	a.toSalvia(t, e)
	// A summon cast in the combat: two dire wolves, in the order, no squares.
	cast, err := a.summonWolves(t, a.bia, e, 14)
	if err != nil {
		t.Fatalf("Conjurar Animais in a combat without a map error = %v", err)
	}
	wolves := cast.GetSummonedCombatantIds()
	if len(wolves) != 2 {
		t.Fatalf("the cast summoned %v, want two wolves", wolves)
	}
	got := a.get(t, a.master)
	wantNoSquares(t, "after the summon", got)
	for _, id := range wolves {
		found := false
		for _, c := range got.GetCombatants() {
			if c.GetId() == id {
				found = true
				if c.GetKind() != playv1.CombatantKind_COMBATANT_KIND_CREATURE {
					t.Errorf("%s is %v, want a creature", c.GetLabel(), c.GetKind())
				}
			}
		}
		if !found {
			t.Errorf("the wolf %s is not in the order", id)
		}
	}
	// The wolves' turn: the creature's player attacks with it, with no reach, and the
	// creature spends movement by number too.
	wolf := byLabel(t, got, "Lobo atroz 1")
	a.passTo(t, e, "Lobo atroz 1")
	a.mustSpend(t, a.bia, e, "Lobo atroz 1", 20)
	if cur := a.get(t, a.master).GetCurrentCombatantId(); cur != wolf.GetId() {
		t.Fatalf("the current combatant = %s, want the wolf", cur)
	}
	atk := a.mustAttack(t, a.bia, e, "Lobo atroz 1", direWolfBite, "Goblin", d20(12))
	if atk.GetRoll() == nil {
		t.Error("the wolf's attack gave no result")
	}
	// The master spends for the NPC on its turn too, and the goblin ends where it was
	// (no square, nothing to read).
	a.passTo(t, e, "Goblin")
	a.mustSpend(t, a.master, e, "Goblin", 25)

	// The Wild Shape in the combat (Sálvia is a druid of the 5th level).
	a.passTo(t, e, "Sálvia")
	if _, err := a.assume(t, a.bia, a.bri, wolfKey); err != nil {
		t.Fatalf("AssumeWildShape in a combat without a map error = %v", err)
	}
	a.mustSpend(t, a.bia, e, "Sálvia", 20)
	if _, err := a.leave(t, a.bia, a.bri); err != nil {
		t.Fatalf("LeaveWildShape error = %v", err)
	}
	wantNoSquares(t, "after the Wild Shape", a.get(t, a.master))

	// The familiar's eyes: refused, and nothing was spent.
	a.passTo(t, e, "Pensantus")
	_, _, err = a.sightCall(t, a.ana, a.pens, true)
	wantSightBlocked(t, err, playv1.FamiliarSightBlockedReason_FAMILIAR_SIGHT_BLOCKED_REASON_NO_MAP)
	if used := byLabel(t, a.get(t, a.master), "Pensantus").GetActionUsed(); used {
		t.Error("the refused sight spent Pensantus's action")
	}
}

// TestRN25_DeathSavesUndoAndTheEnd: a character at 0 hit points makes its death save
// on its turn, the undo takes back an attack and a spent movement in a row, and the
// end of the combat gives the XP of the NPCs that fell, as on a map.
func TestRN25_DeathSavesUndoAndTheEnd(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.theatreCasters(t, 1)

	// Pensantus attacks; Toren is dropped and rolls his death save on his turn.
	a.correct(t, a.toren, hpIs(0))
	a.mustEndTurn(t, a.ana, e) // Toren's turn: a death save is due
	if !byLabel(t, a.get(t, a.caio), "Toren").GetDeathSaveDue() {
		t.Fatal("no death save due at Toren's turn")
	}
	a.h.roller.queue(14)
	res, err := a.deathSave(t, a.caio, e, "Toren", rollApp)
	if err != nil || res.GetDeathSave().GetOutcome() != playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_SUCCESS {
		t.Fatalf("the death save = %v, %v; want a success", res.GetDeathSave(), err)
	}
	// Brisa heals him, and the master undoes the heal and then (still the last action) nothing more.
	a.mustEndTurn(t, a.caio, e)
	a.mustCast(t, a.bia, e, "Brisa", cureWounds, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)
	a.undoLast(t, e)
	if got := a.vitals(t, a.toren).GetHitPointsCurrent(); got != 0 {
		t.Errorf("Toren's hit points after the undo of the cast = %d, want 0", got)
	}

	// The undo of an attack: Pensantus's bolt hits and the undo takes the roll away.
	a.passTo(t, e, "Pensantus")
	a.mustAttack(t, a.ana, e, "Pensantus", fireBolt, "Goblin", d20(19))
	a.undoLast(t, e)
	if c := byLabel(t, a.get(t, a.master), "Pensantus"); c.GetActionUsed() {
		t.Error("the undo of an attack left the action spent")
	}

	// The end: the NPC that fell counts, and the master reads its XP.
	if _, err := a.adjustHP(t, e, "Goblin", damageHP(100)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	var xp int64
	if err := a.h.pool.QueryRow(t.Context(), `SELECT coalesce(sum(xp_value), 0) FROM combatants WHERE encounter_id = $1 AND defeated`, e.GetId()).Scan(&xp); err != nil {
		t.Fatalf("read the XP: %v", err)
	}
	enc, err := a.h.svc.CampaignEncounter(t.Context(), nil, a.campaignID, e.GetId())
	if err != nil {
		t.Fatalf("CampaignEncounter() error = %v", err)
	}
	if !enc.Ended || int64(enc.XP) != xp {
		t.Errorf("the ended combat = %+v, want ended with the defeated NPCs' XP (%d)", enc, xp)
	}
}

// TestRN25_AGridOfferIsNotWithdrawn: "Retirar a oferta" is for the offers the master
// made by hand; the one a move made on a map is answered, or skipped.
func TestRN25_AGridOfferIsNotWithdrawn(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	e := c.oppFight(t)
	c.leaveGoblin(t)
	offers := c.offersOf(t, c.master)
	if len(offers) == 0 {
		t.Fatal("leaving the goblin's reach offered nothing")
	}
	err := c.withdraw(t, c.master, e, offers[0].GetId())
	wantBlockedBy(t, "WithdrawOpportunity of a move's offer", err, blockedTheatreOnly)
	if got := c.offersOf(t, c.master); len(got) != len(offers) {
		t.Errorf("the refused withdrawal left %d offers, want %d", len(got), len(offers))
	}
}

// TestRN25_ConditionsThatLeaveNoSpeed: a grappled or restrained creature has speed
// 0 and a paralyzed, petrified, stunned or unconscious one cannot move (SRD 5.1), on
// a map and without one: the movement left is 0 and a move or a spend is refused.
func TestRN25_ConditionsThatLeaveNoSpeed(t *testing.T) {
	t.Parallel()
	noSpeed := []string{"condition:grappled", "condition:restrained", "condition:paralyzed", "condition:petrified", "condition:stunned", "condition:unconscious"}

	// Without a map.
	a := newArmed(t)
	e := a.theatreThree(t)
	for _, key := range noSpeed {
		if _, err := a.conditions(t, a.master, e, "Toren", []string{key}, true, false); err != nil {
			t.Fatalf("SetCombatantConditions(%s) error = %v", key, err)
		}
		if got := a.theatreCombatant(t, a.caio, "Toren").GetMovementLeftDft(); got != 0 {
			t.Errorf("%s: Toren's movement left = %d, want 0", key, got)
		}
		_, err := a.spend(t, a.caio, e, "Toren", 5)
		wantBlockedBy(t, key+": SpendMovement", err, blockedTooFar)
		res, err := a.caio.combat.GetMoveOptions(t.Context(), connect.NewRequest(&playv1.GetMoveOptionsRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Toren")}))
		if err != nil || res.Msg.GetMovementLeftDft() != 0 {
			t.Errorf("%s: GetMoveOptions = %v, %v; want 0 left", key, res.Msg, err)
		}
	}
	// An incapacitated creature takes no action at all (SRD 5.1, Conditions); a grappled one
	// has no speed and may still act.
	if _, err := a.action(t, a.caio, e, "Toren", "standard:dash"); err == nil {
		t.Errorf("an unconscious Toren dashed")
	}
	if _, err := a.conditions(t, a.master, e, "Toren", []string{"condition:grappled"}, true, false); err != nil {
		t.Fatalf("SetCombatantConditions(grappled) error = %v", err)
	}
	if _, err := a.action(t, a.caio, e, "Toren", "standard:dash"); err != nil {
		t.Fatalf("Dash error = %v", err)
	}
	if got := a.theatreCombatant(t, a.caio, "Toren").GetMovementLeftDft(); got != 0 {
		t.Errorf("a grappled Toren that dashes has %d left, want 0 (twice nothing)", got)
	}
	if _, err := a.conditions(t, a.master, e, "Toren", nil, true, false); err != nil {
		t.Fatalf("clearing the conditions error = %v", err)
	}
	if got := a.theatreCombatant(t, a.caio, "Toren").GetMovementLeftDft(); got != 600 {
		t.Errorf("after the conditions went, Toren has %d left, want 600 (30 ft, dashed)", got)
	}
	a.mustSpend(t, a.caio, e, "Toren", 10)

	// On a map.
	g := newArmed(t)
	ge := g.threeAndAGoblin(t) // Toren at (3, 3), first
	move := func() (*playv1.MoveCombatantResponse, error) {
		res, err := g.caio.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
			CampaignId: g.campaignID, EncounterId: ge.GetId(), CombatantId: g.id(t, "Toren"), IdempotencyKey: newKey(), Col: 3, Row: 4,
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	for _, key := range noSpeed {
		if _, err := g.conditions(t, g.master, ge, "Toren", []string{key}, true, false); err != nil {
			t.Fatalf("SetCombatantConditions(%s) error = %v", key, err)
		}
		_, err := move()
		wantBlockedBy(t, key+": MoveCombatant on a map", err, blockedTooFar)
		res, err := g.caio.combat.GetMoveOptions(t.Context(), connect.NewRequest(&playv1.GetMoveOptionsRequest{CampaignId: g.campaignID, EncounterId: ge.GetId(), CombatantId: g.id(t, "Toren")}))
		if err != nil || res.Msg.GetMovementLeftDft() != 0 || len(res.Msg.GetReachable()) != 0 {
			t.Errorf("%s: GetMoveOptions = %v, %v; want 0 left and nowhere to go", key, res.Msg, err)
		}
	}
	if _, err := g.conditions(t, g.master, ge, "Toren", nil, true, false); err != nil {
		t.Fatalf("clearing the conditions error = %v", err)
	}
	if _, err := move(); err != nil {
		t.Errorf("MoveCombatant after the conditions went error = %v", err)
	}
}

// TestRN25_ATrapDoesNotExistInACombatWithoutAMap: while a combat without a map is
// on the table, a trap is not fired by hand, not searched for and not triggered by
// a token or a creature dropped on it, whichever map is current; a missing trap is
// still not found, and a retry of what was done before the combat gets its stored
// answer.
func TestRN25_ATrapDoesNotExistInACombatWithoutAMap(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	first := r.trap(t, "Fosso", 12, 7, func(s *mapsv1.TrapSpec) { s.Trigger = rulesv1.TrapTrigger_TRAP_TRIGGER_MANUAL })
	second := r.trap(t, "Rede", 14, 7)
	hidden := r.trap(t, "Fosso Oculto", 4, 4, func(s *mapsv1.TrapSpec) { s.FindDc = 10 })
	r.place(t, r.toren.GetId(), 3, 3)

	// Before the combat: a firing by hand and a search, each with a key of its own.
	fireKey, searchKey := newKey(), newKey()
	fire := func(point string, key string) (*playv1.FireTrapResponse, error) {
		res, err := r.master.play.FireTrap(t.Context(), connect.NewRequest(&playv1.FireTrapRequest{CampaignId: r.campaignID, MapId: r.mapID, PointId: point, IdempotencyKey: key}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	search := func(key string) (*playv1.SearchForTrapsResponse, error) {
		res, err := r.caio.play.SearchForTraps(t.Context(), connect.NewRequest(&playv1.SearchForTrapsRequest{
			CampaignId: r.campaignID, IdempotencyKey: key, Skill: playv1.TrapSearchSkill_TRAP_SEARCH_SKILL_INVESTIGATION,
			Roll: &playv1.SearchForTrapsRequest_D20Face{D20Face: 12},
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	if _, err := fire(first.GetId(), fireKey); err != nil {
		t.Fatalf("FireTrap() before the combat error = %v", err)
	}
	found, err := search(searchKey)
	if err != nil || len(found.GetFoundPointIds()) != 1 || found.GetFoundPointIds()[0] != hidden.GetId() {
		t.Fatalf("SearchForTraps() before the combat = %v, %v; want the hidden pit", found, err)
	}
	fired := r.eventCount(t, eventTrapTriggered)

	e := r.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: r.capitao.GetId()}},
		npcRolls: []int{1},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 5},
		reveal:   []string{"Capitão Goblin"},
		theatre:  true,
	})
	wantNoSquares(t, "the combat", e)

	_, err = fire(second.GetId(), newKey())
	wantBlockedBy(t, "FireTrap during a combat without a map", err, blockedNeedsAMap)
	_, err = fire(newKey(), newKey())
	wantCode(t, "FireTrap of a trap that is not there", err, connect.CodeNotFound)
	_, err = search(newKey())
	wantBlockedBy(t, "SearchForTraps during a combat without a map", err, blockedNeedsAMap)
	// The retries of what was done before the combat began are answered from the history.
	if res, err := fire(first.GetId(), fireKey); err != nil || res.GetFiring() == nil {
		t.Errorf("the retry of the firing = %v, %v; want the stored answer", res, err)
	}
	if res, err := search(searchKey); err != nil || len(res.GetFoundPointIds()) != 1 {
		t.Errorf("the retry of the search = %v, %v; want the stored answer", res, err)
	}

	// A token dropped on the armed trap, and a creature's: nothing fires.
	r.place(t, r.pens.GetId(), 14, 7)
	r.place(t, r.goblins.GetId(), 14, 7)
	if got := r.eventCount(t, eventTrapTriggered); got != fired {
		t.Errorf("trap_triggered events = %d, want %d: a token dropped fired a trap in a combat without a map", got, fired)
	}
	if n := len(r.trapDamages(t)); n != 0 {
		t.Errorf("trap damages = %d, want none", n)
	}
	if p := r.point(t, r.master, second.GetId()); p.GetTrap().GetState() != mapsv1.TrapState_TRAP_STATE_ARMED {
		t.Errorf("the second trap is %v, want it armed still", p.GetTrap().GetState())
	}
}

// TestRN10_AHiddenNPCIsNeverNamedInATheatreCombat: the master spends a hidden NPC's
// movement, attacks with it and offers it as the reactor on a player's mover; what
// the players read (the combat, the options, the log) and what their streams carry
// never names it.
func TestRN10_AHiddenNPCIsNeverNamedInATheatreCombat(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.capitao.GetId()}},
		npcRolls: []int{1},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 5},
		theatre:  true, // the Capitão stays hidden
	})
	capitao := a.id(t, "Capitão Goblin")
	streams := map[string]*watcher{"Toren's player": a.caio.watch(t, a.campaignID), "Pensantus's player": a.ana.watch(t, a.campaignID)}
	for _, w := range streams {
		w.ready(t)
	}

	a.mustSpend(t, a.master, e, "Capitão Goblin", 20)
	a.passTo(t, e, "Capitão Goblin")
	if _, err := a.attack(t, a.master, e, "Capitão Goblin", sword, "Toren", d20(15)); err != nil {
		t.Fatalf("the hidden Capitão's attack error = %v", err)
	}
	a.passTo(t, e, "Toren")
	offer := a.mustOffer(t, e, "Toren", "Capitão Goblin") // Toren leaves the hidden Capitão's reach
	if got := a.get(t, a.caio).GetOpportunityOffers(); len(got) != 1 || got[0].GetReactorId() != "" || got[0].GetReactorLabel() != "" || got[0].GetForYou() {
		t.Errorf("Toren's player reads the offer %v, want it with the reactor unnamed and not for him to answer", got)
	}
	if _, err := a.master.combat.SkipOpportunity(t.Context(), connect.NewRequest(&playv1.SkipOpportunityRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), OpportunityOfferId: offer.GetOpportunityOfferId(), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("SkipOpportunity() error = %v", err)
	}

	a.master.markCurrentMap(t, a.campaignID, a.mapID, streams["Toren's player"], streams["Pensantus's player"]) // a visible event ends what the streams are read for
	for who, u := range map[string]*user{"Toren's player": a.caio, "Pensantus's player": a.ana} {
		for what, js := range map[string]string{
			"the combat":  protojson.Format(a.get(t, u)),
			"the options": protojson.Format(a.mustOptions(t, u, e, ownLabel(who))),
			"the log":     protojson.Format(a.log(t, u, e)),
		} {
			if strings.Contains(js, capitao) || strings.Contains(js, "Capitão") {
				t.Errorf("%s reads %s with the hidden Capitão in it", who, what)
			}
		}
	}
	for who, w := range streams {
		for _, ev := range w.beforeMarker(t) {
			if js := protojson.Format(ev); strings.Contains(js, capitao) || strings.Contains(js, "Capitão") {
				t.Errorf("%s's stream names the hidden Capitão: %s", who, js)
			}
		}
	}
}
