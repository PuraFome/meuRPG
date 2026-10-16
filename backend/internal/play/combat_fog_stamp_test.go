package play

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// Characterization of combatTx.stamp, stampDoor and stampPlacedCast
// (combat_fog.go): who may have the line of an event written on a fog map. The
// combatants are the real ones of a table (smiteTable) and the sight is a fake
// that says which squares each player sees.

// stampSight is a maplink.CombatSight with a table of what each player sees.
type stampSight struct {
	users []string
	sees  map[string][]grid.Square
	doors map[string]*grid.DoorLayer // the doors each player knows of
	err   error                      // what KnownTerrain fails with
}

func (s *stampSight) Users() []string { return s.users }

func (s *stampSight) Sees(userID string, sq grid.Square) bool {
	return slices.Contains(s.sees[userID], sq)
}

func (s *stampSight) CanSee(grid.Square, vision.Senses, grid.Square) bool { return false }

func (s *stampSight) KnownTerrain(_ context.Context, _ pgx.Tx, userID string) (grid.Terrain, error) {
	if s.err != nil {
		return grid.Terrain{}, s.err
	}
	return grid.Terrain{Grid: stampGrid, Doors: s.doors[userID]}, nil
}

var stampGrid = grid.Grid{Columns: 24, Rows: 16}

// stampTable is the table of smiteTable with a combatTx on a fake sight.
type stampTable struct {
	*armed
	c    *combatTx
	sq   map[string]grid.Square // the square of each combatant, by label
	sees *stampSight
	cs   []playdb.Combatant
}

func newStampTable(t *testing.T) *stampTable {
	t.Helper()
	a, e := smiteTable(t)
	cs, err := a.h.svc.queries.ListCombatantsWithDismissed(t.Context(), e.GetId())
	if err != nil {
		t.Fatalf("ListCombatantsWithDismissed() error = %v", err)
	}
	st := &stampTable{armed: a, sq: map[string]grid.Square{}, cs: cs}
	for _, c := range cs {
		st.sq[c.Label] = squareOfCombatant(c)
	}
	st.sees = &stampSight{users: []string{a.caio.id, a.ana.id, a.bia.id}, sees: map[string][]grid.Square{}, doors: map[string]*grid.DoorLayer{}}
	st.c = &combatTx{q: a.h.svc.queries, enc: playdb.Encounter{ID: e.GetId()}, sight: &fogSight{sight: st.sees}}
	return st
}

func (st *stampTable) cid(t *testing.T, label string) string {
	t.Helper()
	for _, c := range st.cs {
		if c.Label == label {
			return c.ID
		}
	}
	t.Fatalf("no combatant %q in %v", label, st.cs)
	return ""
}

func (st *stampTable) stamp(t *testing.T, kind string, ev actionEvent) actionEvent {
	t.Helper()
	out, err := st.c.stamp(t.Context(), kind, ev)
	if err != nil {
		t.Fatalf("stamp(%s) error = %v", kind, err)
	}
	return out
}

func wantStamp(t *testing.T, what string, ev actionEvent, fogged bool, seenBy ...string) {
	t.Helper()
	if ev.Fogged != fogged || !slices.Equal(ev.SeenBy, seenBy) {
		t.Errorf("%s: fogged = %v, seen by %v; want %v, %v", what, ev.Fogged, ev.SeenBy, fogged, seenBy)
	}
}

func TestStamp_NoSightAndNoNPC(t *testing.T) {
	t.Parallel()
	st := newStampTable(t)
	ev := actionEvent{Actor: st.cid(t, "Toren"), Target: st.cid(t, "Brisa")}

	st.c.sight = nil
	if got := st.stamp(t, eventAttackRolled, actionEvent{Actor: st.cid(t, "Toren"), Target: st.cid(t, "Esqueleto")}); got.Fogged || got.SeenBy != nil {
		t.Errorf("with no fog = %+v, want it as it was", got)
	}
	st.c.sight = &fogSight{sight: st.sees}
	wantStamp(t, "two players", st.stamp(t, eventAttackRolled, ev), false)
	// An id the combat does not know is skipped.
	wantStamp(t, "unknown ids", st.stamp(t, eventAttackRolled, actionEvent{Actor: newKey(), Target: newKey()}), false)
	// A creature of the combat that is not an NPC is not a reason to fog.
	wantStamp(t, "no ids", st.stamp(t, eventTurnEnded, actionEvent{}), false)
}

func TestStamp_AnNPCInTheLineIsSeenByThoseWhoSawIt(t *testing.T) {
	t.Parallel()
	st := newStampTable(t)
	toren, brisa, pens := st.sees.users[0], st.sees.users[2], st.sees.users[1]
	st.sees.sees[toren] = []grid.Square{st.sq["Esqueleto"]}
	st.sees.sees[brisa] = []grid.Square{st.sq["Esqueleto"], st.sq["Bandido"]}
	st.sees.sees[pens] = []grid.Square{st.sq["Bandido"]}
	skel, band := st.cid(t, "Esqueleto"), st.cid(t, "Bandido")

	// One NPC: whoever sees its square.
	wantStamp(t, "one NPC", st.stamp(t, eventAttackRolled, actionEvent{Actor: st.cid(t, "Toren"), Target: skel}), true, toren, brisa)
	// Two NPCs: whoever sees both.
	wantStamp(t, "two NPCs", st.stamp(t, eventAttackRolled, actionEvent{Actor: skel, Target: band}), true, brisa)
	// A line that lists a spell's targets lists them all.
	wantStamp(t, "spell targets", st.stamp(t, eventSpellCast, actionEvent{Actor: st.cid(t, "Toren"), Hits: []castHit{{Target: skel}, {Target: band}, {Target: band}}}), true, brisa)
	// A hidden target of a cast is left out of the line.
	wantStamp(t, "hidden at cast", st.stamp(t, eventSpellCast, actionEvent{Actor: st.cid(t, "Toren"), Hits: []castHit{{Target: skel}, {Target: band, HiddenAtCast: true}}}), true, toren, brisa)
	// Damage that settled several.
	wantStamp(t, "settled", st.stamp(t, eventDamageRolled, actionEvent{Actor: st.cid(t, "Toren"), Settled: []damageHit{{Target: band}}}), true, pens, brisa)
	// Nobody sees it: fogged, seen by no one.
	st.sees.sees = map[string][]grid.Square{}
	wantStamp(t, "unseen", st.stamp(t, eventAttackRolled, actionEvent{Actor: skel}), true)
}

func TestStamp_AMoveIsSeenOnEitherSquareAndAnOpportunityAttackOnTheSquareItLeft(t *testing.T) {
	t.Parallel()
	st := newStampTable(t)
	toren, pens, brisa := st.sees.users[0], st.sees.users[1], st.sees.users[2]
	skel := st.cid(t, "Esqueleto")
	from := grid.Square{Col: 9, Row: 9}
	st.sees.sees[toren] = []grid.Square{st.sq["Esqueleto"]}
	st.sees.sees[pens] = []grid.Square{from}
	st.sees.sees[brisa] = []grid.Square{{Col: 1, Row: 1}}

	// The NPC moved: the square it stands on and the one it left.
	mv := actionEvent{Actor: skel, From: &moveState{Placed: true, Col: 9, Row: 9}}
	wantStamp(t, "move", st.stamp(t, eventCombatantMoved, mv), true, toren, pens)
	// A From that is not placed counts for nothing; so does a From on another kind.
	mv.From.Placed = false
	wantStamp(t, "unplaced from", st.stamp(t, eventCombatantMoved, mv), true, toren)
	mv.From.Placed = true
	wantStamp(t, "from on an attack", st.stamp(t, eventAttackRolled, mv), true, toren)
	// A move whose actor is not the NPC listed (the target) takes no From square.
	other := actionEvent{Actor: st.cid(t, "Toren"), Target: skel, From: &moveState{Placed: true, Col: 9, Row: 9}}
	wantStamp(t, "from of another", st.stamp(t, eventCombatantMoved, other), true, toren)

	// An opportunity attack at the mover: judged on the square it left (ev.Col, ev.Row).
	opp := actionEvent{Actor: st.cid(t, "Toren"), Target: skel, OfferID: "offer", Col: 1, Row: 1}
	wantStamp(t, "opportunity attack", st.stamp(t, eventAttackRolled, opp), true, brisa)
	// With no offer the square the NPC stands on counts.
	opp.OfferID = ""
	wantStamp(t, "attack without an offer", st.stamp(t, eventAttackRolled, opp), true, toren)
	// An offer against the actor, not the target, is not judged on that square.
	opp.OfferID, opp.Target = "offer", st.cid(t, "Toren")
	opp.Actor = skel
	wantStamp(t, "offer by the NPC", st.stamp(t, eventAttackRolled, opp), true, toren)
}

func TestStamp_ASecretEventIsNoPlayersAndATrapFogsItsNPCs(t *testing.T) {
	t.Parallel()
	st := newStampTable(t)
	toren, pens := st.sees.users[0], st.sees.users[1]
	st.sees.sees[toren] = []grid.Square{st.sq["Esqueleto"]}
	st.sees.sees[pens] = []grid.Square{st.sq["Esqueleto"], st.sq["Bandido"]}
	skel := st.cid(t, "Esqueleto")

	wantStamp(t, "secret with an NPC", st.stamp(t, eventAttackRolled, actionEvent{Secret: true, Actor: skel}), true)
	wantStamp(t, "secret with none", st.stamp(t, eventAttackRolled, actionEvent{Secret: true, Actor: st.cid(t, "Toren")}), true)

	trap := &trapFireEvent{Caught: []trapCaughtEvent{{Target: skel}, {Target: st.cid(t, "Brisa")}, {Target: newKey()}, {Target: st.cid(t, "Bandido")}}}
	got := st.stamp(t, eventTrapTriggered, actionEvent{Trap: trap})
	wantStamp(t, "trap line", got, false) // no NPC in the line itself
	c := got.Trap.Caught
	if !c[0].Fogged || !slices.Equal(c[0].SeenBy, []string{toren, pens}) {
		t.Errorf("caught skeleton = %+v, want fogged, seen by Toren's and Pensantus's players", c[0])
	}
	if c[1].Fogged || c[2].Fogged {
		t.Errorf("caught player/unknown = %+v %+v, want them as they were", c[1], c[2])
	}
	if !c[3].Fogged || !slices.Equal(c[3].SeenBy, []string{pens}) {
		t.Errorf("caught bandit = %+v, want fogged, seen by Pensantus's player", c[3])
	}
	if trap.Caught[0].Fogged {
		t.Error("the trap event of the caller was changed: stamp must work on a copy")
	}
	// A trap and a secret together: the trap is stamped, the line is nobody's.
	both := st.stamp(t, eventTrapTriggered, actionEvent{Secret: true, Trap: trap})
	wantStamp(t, "secret trap", both, true)
	if !both.Trap.Caught[0].Fogged {
		t.Error("a secret trap line still fogs the caught NPC")
	}
}

func TestStamp_APlacedCastKeepsWhoSawEachTarget(t *testing.T) {
	t.Parallel()
	st := newStampTable(t)
	toren, pens, brisa := st.sees.users[0], st.sees.users[1], st.sees.users[2]
	skel, band := st.cid(t, "Esqueleto"), st.cid(t, "Bandido")
	st.sees.sees[pens] = []grid.Square{st.sq["Esqueleto"]}
	st.sees.sees[brisa] = []grid.Square{st.sq["Esqueleto"], st.sq["Bandido"]}
	caster := st.cid(t, "Toren") // Toren's player owns the line of their spell

	ev := st.stamp(t, eventSpellCast, actionEvent{Actor: caster, Placed: true, Hits: []castHit{
		{Target: skel}, {Target: band}, {Target: st.cid(t, "Brisa")}, {Target: skel, HiddenAtCast: true, SeenMask: 99}, {Target: newKey()},
	}})
	wantStamp(t, "placed cast", ev, true, pens, brisa, toren)
	if !slices.Equal(ev.CoverUsers, []string{pens, brisa}) {
		t.Errorf("CoverUsers = %v, want Pensantus's and Brisa's players in the order they were met", ev.CoverUsers)
	}
	masks := []uint64{ev.Hits[0].SeenMask, ev.Hits[1].SeenMask, ev.Hits[2].SeenMask, ev.Hits[3].SeenMask}
	if want := []uint64{0b11, 0b10, 0, 99}; !slices.Equal(masks, want) {
		t.Errorf("masks = %v, want %v", masks, want)
	}
	if !ev.Hits[0].Fogged || !ev.Hits[1].Fogged || ev.Hits[2].Fogged || ev.Hits[3].Fogged {
		t.Errorf("fogged hits = %v %v %v %v, want the NPCs only (not the player, not the hidden one)", ev.Hits[0].Fogged, ev.Hits[1].Fogged, ev.Hits[2].Fogged, ev.Hits[3].Fogged)
	}

	// A cast that hit no NPC is every player's line.
	wantStamp(t, "no NPC hit", st.stamp(t, eventSpellCast, actionEvent{Actor: caster, Placed: true, Hits: []castHit{{Target: st.cid(t, "Brisa")}}}), false)
	// A caster without a player (an NPC caster) adds nobody; a caster already in the list is not added twice.
	npc := st.stamp(t, eventSpellCast, actionEvent{Actor: skel, Placed: true, Hits: []castHit{{Target: band}}})
	wantStamp(t, "NPC caster", npc, true, brisa)
	st.sees.sees[toren] = []grid.Square{st.sq["Bandido"]}
	again := st.stamp(t, eventSpellCast, actionEvent{Actor: caster, Placed: true, Hits: []castHit{{Target: band}}})
	wantStamp(t, "caster saw it too", again, true, toren, brisa)
	// Nobody saw the NPC it hit: the caster's player has the line all the same, and the hit is fogged for them.
	st.sees.sees = map[string][]grid.Square{}
	none := st.stamp(t, eventSpellCast, actionEvent{Actor: caster, Placed: true, Hits: []castHit{{Target: band}}})
	wantStamp(t, "unseen target", none, true, toren)
	if !none.Hits[0].Fogged || none.Hits[0].SeenMask != 0 {
		t.Errorf("hit = %+v, want it fogged for everyone", none.Hits[0])
	}
}

func TestStamp_APlacedCastKeepsAtMostMaxCoverUsers(t *testing.T) {
	t.Parallel()
	st := newStampTable(t)
	band := st.cid(t, "Bandido")
	st.sees.users = nil
	for i := range maxCoverUsers + 3 {
		u := fmt.Sprintf("player-%02d", i)
		st.sees.users = append(st.sees.users, u)
		st.sees.sees[u] = []grid.Square{st.sq["Bandido"]}
	}
	ev := st.stamp(t, eventSpellCast, actionEvent{Actor: st.cid(t, "Esqueleto"), Placed: true, Hits: []castHit{{Target: band}}})
	if len(ev.CoverUsers) != maxCoverUsers || len(ev.SeenBy) != maxCoverUsers+3 {
		t.Fatalf("cover users %d, seen by %d; want %d and %d", len(ev.CoverUsers), len(ev.SeenBy), maxCoverUsers, maxCoverUsers+3)
	}
	if ev.Hits[0].SeenMask != ^uint64(0) {
		t.Errorf("mask = %b, want the %d bits of the users kept", ev.Hits[0].SeenMask, maxCoverUsers)
	}
}

func TestStamp_ADoorIsSeenByTheOnesWhoKnowItAndTheMover(t *testing.T) {
	t.Parallel()
	st := newStampTable(t)
	toren, pens, brisa := st.sees.users[0], st.sees.users[1], st.sees.users[2]
	door := grid.Square{Col: 5, Row: 6}
	known := grid.NewDoorLayer(stampGrid)
	known.Set(door.Col, door.Row, grid.DoorClosed)
	st.sees.doors[pens] = known
	st.sees.doors[brisa] = grid.NewDoorLayer(stampGrid) // knows no door there
	skel := st.cid(t, "Esqueleto")

	// A player's character opens it: the player and whoever knows the door.
	ev := actionEvent{Actor: st.cid(t, "Toren"), Col: int32(door.Col), Row: int32(door.Row)}
	wantStamp(t, "player mover", st.stamp(t, eventDoorOpened, ev), true, toren, pens)
	// An unknown actor: only those who know the door.
	wantStamp(t, "unknown mover", st.stamp(t, eventDoorOpened, actionEvent{Actor: newKey(), Col: ev.Col, Row: ev.Row}), true, pens)
	// An NPC opens it: the ones who know the door must also have seen the NPC, on either square.
	npc := actionEvent{Actor: skel, Col: ev.Col, Row: ev.Row}
	wantStamp(t, "NPC unseen", st.stamp(t, eventDoorOpened, npc), true)
	st.sees.sees[pens] = []grid.Square{st.sq["Esqueleto"]}
	wantStamp(t, "NPC seen where it stands", st.stamp(t, eventDoorOpened, npc), true, pens)
	st.sees.sees[pens] = []grid.Square{{Col: 9, Row: 9}}
	npc.From = &moveState{Placed: true, Col: 9, Row: 9}
	wantStamp(t, "NPC seen where it came from", st.stamp(t, eventDoorOpened, npc), true, pens)
	npc.From.Placed = false
	wantStamp(t, "NPC from not placed", st.stamp(t, eventDoorOpened, npc), true)
	// An NPC mover that a player sees but who does not know the door: not theirs.
	st.sees.sees[brisa] = []grid.Square{st.sq["Esqueleto"]}
	wantStamp(t, "NPC seen, door unknown", st.stamp(t, eventDoorOpened, npc), true)

	// The terrain of a player cannot be read: the error comes out.
	st.sees.err = errors.New("boom")
	if _, err := st.c.stamp(t.Context(), eventDoorOpened, ev); err == nil || !errors.Is(err, st.sees.err) {
		t.Errorf("stamp with a failing terrain read: error = %v, want it wrapped", err)
	}
}
