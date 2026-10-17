package play

import (
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The characterization of MoveCombatant beyond the plain cave (see
// combat_move_helpers_test.go): doors, the state of the combat, the request, and what the other modules add
// to a move.

// doorMove is one move on the door cave: Toren on turn at (6, 7) with the door
// of the case at (8, 7).
type doorMove struct {
	name    string
	door    mapsv1.DoorState
	setup   func(t *testing.T, c *cave)
	as      string // "master" or Toren's player
	edit    []func(*playv1.MoveCombatantRequest)
	col     int32
	refused *refusal
	want    moveWant
	// opened is the door_opened payload the move wrote, "" when it opened nothing.
	opened string
}

func TestMoveCombatantFrozen_Doors(t *testing.T) {
	t.Parallel()
	closed, locked, barred := mapsv1.DoorState_DOOR_STATE_CLOSED, mapsv1.DoorState_DOOR_STATE_LOCKED, mapsv1.DoorState_DOOR_STATE_BARRED
	nextToTheDoor := standAt(map[string][2]int32{"Toren": {7, 7}})
	doorLockedRefusal := new(blockedBy("DOOR_LOCKED", "the door is locked"))
	for _, tc := range []doorMove{
		{
			name: "a player through a closed door", door: closed, col: 10,
			want: moveWant{col: 10, row: 7, usedDft: 200, lastDft: 200, event: `{"col": 10, "combatant_id": "<Toren>", "cost_dft": 200, "cost_ft": 20, "distance_dft": 200, "distance_ft": 20, "from": {"col": 6, "cover": "none", "placed": true, "row": 7}, "on_turn": true, "round": 1, "row": 7, "spent_dft": 200}`}, opened: `{"col": 8, "combatant_id": "<Toren>", "from": {"col": 6, "cover": "none", "placed": true, "row": 7}, "round": 1, "row": 7}`,
		},
		{
			name: "a player stopped by a locked door", door: locked, col: 10,
			want: moveWant{col: 7, row: 7, usedDft: 50, lastDft: 50, locked: true, event: `{"col": 7, "combatant_id": "<Toren>", "cost_dft": 50, "cost_ft": 5, "distance_dft": 50, "distance_ft": 5, "from": {"col": 6, "cover": "none", "placed": true, "row": 7}, "locked_door": true, "on_turn": true, "round": 1, "row": 7, "spent_dft": 50}`},
		},
		{name: "a player in front of a locked door", door: locked, setup: nextToTheDoor, col: 10, refused: doorLockedRefusal},
		{name: "a player at a barred door", door: barred, col: 10, refused: new(blockedBy("MOVE_BLOCKED", "a wall, a column or a squeeze is in the way"))},
		{
			name: "the master through a closed door", door: closed, as: "master", col: 10,
			want: moveWant{col: 10, row: 7, usedDft: 0, lastDft: 0, event: `{"col": 10, "combatant_id": "<Toren>", "distance_dft": 200, "distance_ft": 20, "from": {"col": 6, "cover": "none", "placed": true, "row": 7}, "on_turn": true, "round": 1, "row": 7}`}, opened: `{"col": 8, "combatant_id": "<Toren>", "from": {"col": 6, "cover": "none", "placed": true, "row": 7}, "round": 1, "row": 7}`,
		},
		{
			name: "the master stopped by a locked door", door: locked, as: "master", col: 10,
			want: moveWant{col: 7, row: 7, usedDft: 0, lastDft: 0, stopped: true, locked: true, event: `{"col": 7, "combatant_id": "<Toren>", "distance_dft": 50, "distance_ft": 5, "from": {"col": 6, "cover": "none", "placed": true, "row": 7}, "locked_door": true, "on_turn": true, "round": 1, "row": 7, "stopped_early": true}`},
		},
		{name: "the master in front of a locked door", door: locked, setup: nextToTheDoor, as: "master", col: 10, refused: doorLockedRefusal},
		{
			name: "the master stopped by a barred door", door: barred, as: "master", col: 10,
			want: moveWant{col: 7, row: 7, usedDft: 0, lastDft: 0, stopped: true, event: `{"col": 7, "combatant_id": "<Toren>", "distance_dft": 50, "distance_ft": 5, "from": {"col": 6, "cover": "none", "placed": true, "row": 7}, "on_turn": true, "round": 1, "row": 7, "stopped_early": true}`},
		},
		{
			name: "the master, forced, ignores a locked door", door: locked, as: "master", col: 10, edit: []func(*playv1.MoveCombatantRequest){asForced},
			want: moveWant{col: 10, row: 7, usedDft: 0, lastDft: 0, event: `{"col": 10, "combatant_id": "<Toren>", "distance_dft": 200, "distance_ft": 20, "from": {"col": 6, "cover": "none", "placed": true, "row": 7}, "on_turn": true, "round": 1, "row": 7}`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newDoorCave(t, tc.door)
			u := f.caio
			if tc.as == "master" {
				u = f.master
			}
			if tc.setup != nil {
				tc.setup(t, f.cave)
			}
			doorEvents := f.moveKindCount(t, "door_opened")
			f.frozenCheck(t, u, "Toren", tc.col, 7, tc.edit, tc.refused, tc.want)
			if got := f.moveKindCount(t, "door_opened") - doorEvents; (tc.opened == "") != (got == 0) || got > 1 {
				t.Fatalf("door_opened events written = %d, want one only when the door opened (%q)", got, tc.opened)
			}
			if tc.opened != "" {
				if got := f.lastMovePayload(t, "door_opened"); got != tc.opened {
					t.Errorf("the door_opened payload =\n  %s\nwant\n  %s", got, tc.opened)
				}
			}
		})
	}
}

// The state of the combat decides who may move at all.
func TestMoveCombatantFrozen_TheStateOfTheCombat(t *testing.T) {
	t.Parallel()
	t.Run("a combat that has not begun", func(t *testing.T) {
		t.Parallel()
		c := newCave(t)
		c.setupUnplaced(t)
		_, err := c.frozenMove(t, c.caio, "Toren", 7, 7)
		c.frozenWantRefusal(t, err, blockedBy("NOT_ACTIVE", "the combat is not running"))
		// The master sets the board up in this state: a move off turn is free.
		c.frozenCheck(t, c.master, "Toren", 7, 7, nil, nil, moveWant{col: 7, row: 7, usedDft: 0, lastDft: 0, event: `{"col": 7, "combatant_id": "<Toren>", "distance_dft": 50, "distance_ft": 5, "from": {"col": 6, "cover": "none", "placed": true, "row": 7}, "row": 7}`})
	})
	t.Run("a combat that has ended", func(t *testing.T) {
		t.Parallel()
		c := newCave(t)
		e := c.fight(t)
		if _, err := c.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: c.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
			t.Fatalf("EndEncounter() error = %v", err)
		}
		for _, u := range []*user{c.caio, c.master} {
			_, err := c.frozenMove(t, u, "Toren", 7, 7)
			c.frozenWantRefusal(t, err, blockedBy("ENCOUNTER_ENDED", "the combat has ended"))
		}
	})
	t.Run("a combat without a map", func(t *testing.T) {
		t.Parallel()
		a := newArmed(t)
		e := a.theatreThree(t)
		for _, u := range []*user{a.caio, a.master} {
			_, err := u.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
				CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Toren"), IdempotencyKey: newKey(), Col: 1, Row: 1,
			}))
			a.frozenWantRefusal(t, err, blockedBy("NEEDS_A_MAP", "this needs a map: a combat without one has no squares"))
		}
	})
	t.Run("a player at zero hit points", func(t *testing.T) {
		t.Parallel()
		c := newCave(t)
		c.fight(t)
		if _, err := c.master.play.AdjustCharacterVitals(t.Context(), connect.NewRequest(&playv1.AdjustCharacterVitalsRequest{
			CampaignId: c.campaignID, CharacterId: c.toren.GetId(), IdempotencyKey: newKey(), HitPointsCurrent: proto.Int32(0),
		})); err != nil {
			t.Fatalf("AdjustCharacterVitals() error = %v", err)
		}
		_, err := c.frozenMove(t, c.caio, "Toren", 7, 7)
		c.frozenWantRefusal(t, err, blockedBy("COMBATANT_DOWN", "the character is down"))
	})
	t.Run("an opportunity attack waits for its answer", func(t *testing.T) {
		t.Parallel()
		c := newCave(t)
		c.fight(t)
		standAt(map[string][2]int32{"Goblin 1": {7, 8}})(t, c)
		c.frozenCheck(t, c.caio, "Toren", 4, 6, nil, nil, moveWant{col: 4, row: 6, usedDft: 112, lastDft: 112, offered: true, event: `{"col": 4, "combatant_id": "<Toren>", "cost_dft": 112, "cost_ft": 11, "distance_dft": 112, "distance_ft": 11, "from": {"col": 6, "cover": "none", "placed": true, "row": 7}, "move_id": "<offers>", "on_turn": true, "round": 1, "row": 6, "spent_dft": 112}`})
		_, err := c.frozenMove(t, c.caio, "Toren", 3, 6)
		c.frozenWantRefusal(t, err, blockedBy("OPPORTUNITY_PENDING", "an opportunity attack waits for its answer"))
		// The master is never stopped by the wait.
		c.frozenCheck(t, c.master, "Toren", 3, 6, nil, nil, moveWant{col: 3, row: 6, usedDft: 112, lastDft: 112, event: `{"col": 3, "combatant_id": "<Toren>", "distance_dft": 50, "distance_ft": 5, "from": {"col": 4, "cover": "none", "last_dft": 112, "placed": true, "row": 6, "used_dft": 112}, "on_turn": true, "round": 1, "row": 6}`})
	})
}

// A call that is not well formed.
func TestMoveCombatantFrozen_TheRequest(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.fight(t)
	stranger := c.h.newUser("Zé")
	for _, tc := range []struct {
		name string
		as   *user
		edit func(*playv1.MoveCombatantRequest)
		want refusal
	}{
		{"a key that is not a UUID", c.caio, func(r *playv1.MoveCombatantRequest) { r.IdempotencyKey = "not-a-key" }, refuse(connect.CodeInvalidArgument, "idempotency_key must be a UUID")},
		{"an encounter that is not a UUID", c.caio, func(r *playv1.MoveCombatantRequest) { r.EncounterId = "x" }, refuse(connect.CodeNotFound, "encounter not found")},
		{"a combatant that is not a UUID", c.caio, func(r *playv1.MoveCombatantRequest) { r.CombatantId = "x" }, refuse(connect.CodeNotFound, "combatant not found")},
		{"a combatant of no combat", c.caio, func(r *playv1.MoveCombatantRequest) { r.CombatantId = newKey() }, refuse(connect.CodeNotFound, "combatant not found")},
		{"a stranger to the campaign", stranger, func(*playv1.MoveCombatantRequest) {}, refuse(connect.CodeNotFound, "campaign not found")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := c.frozenMove(t, tc.as, "Toren", 7, 7, tc.edit)
			c.frozenWantRefusal(t, err, tc.want)
		})
	}
}

// What the other modules add to a move: a drag, a surprise, a trap.

// A grappler drags the creature it holds, at half speed.
func TestMoveCombatantFrozen_ADrag(t *testing.T) {
	t.Parallel()
	t.Run("the creature is left behind and the walk costs twice", func(t *testing.T) {
		t.Parallel()
		a := newArmed(t)
		c := dragFight(t, a, arenaPlan{})
		c.frozenCheck(t, a.caio, "Toren", 3, 0, nil, nil, moveWant{col: 3, row: 0, usedDft: 300, lastDft: 150, event: `{"col": 3, "combatant_id": "<Toren>", "cost_dft": 150, "cost_ft": 15, "distance_dft": 150, "distance_ft": 15, "dragged_from": {"col": 4, "cover": "none", "placed": true, "row": 3}, "dragged_id": "<Hobgoblin>", "dragging": true, "from": {"col": 3, "cover": "none", "placed": true, "row": 3}, "on_turn": true, "round": 2, "spent_dft": 300}`})
		if got := c.square(t, c.hob); got != [2]int32{3, 1} {
			t.Errorf("the dragged creature stands on %v, want (3, 1)", got)
		}
		if got := c.moveRowOf(t, c.hob); got.UsedDft != 0 || got.LastDft != 0 {
			t.Errorf("the dragged creature spent %v, want nothing", got)
		}
	})
	t.Run("a forced move drags nobody", func(t *testing.T) {
		t.Parallel()
		a := newArmed(t)
		c := dragFight(t, a, arenaPlan{})
		c.frozenCheck(t, a.master, "Toren", 3, 0, []func(*playv1.MoveCombatantRequest){asForced}, nil, moveWant{col: 3, row: 0, usedDft: 0, lastDft: 0, event: `{"col": 3, "combatant_id": "<Toren>", "distance_dft": 150, "distance_ft": 15, "from": {"col": 3, "cover": "none", "placed": true, "row": 3}, "on_turn": true, "round": 2}`})
		if got := c.square(t, c.hob); got != [2]int32{4, 3} {
			t.Errorf("the creature stands on %v after a forced move, want where it was, (4, 3)", got)
		}
	})
	t.Run("a long jump drags nobody", func(t *testing.T) {
		t.Parallel()
		a := newArmed(t)
		c := dragFight(t, a, arenaPlan{})
		c.frozenCheck(t, a.caio, "Toren", 3, 2, []func(*playv1.MoveCombatantRequest){jumpTo(playv1.JumpKind_JUMP_KIND_LONG)}, nil, moveWant{col: 3, row: 2, usedDft: 50, lastDft: 0, event: `{"col": 3, "combatant_id": "<Toren>", "cost_dft": 50, "cost_ft": 5, "distance_dft": 50, "distance_ft": 5, "from": {"col": 3, "cover": "none", "placed": true, "row": 3}, "jump": "long", "on_turn": true, "round": 2, "row": 2}`})
		if got := c.square(t, c.hob); got != [2]int32{4, 3} {
			t.Errorf("the creature stands on %v after a jump, want where it was, (4, 3)", got)
		}
	})
	t.Run("no room to leave the creature", func(t *testing.T) {
		t.Parallel()
		a := newArmed(t)
		c := a.arena(t, arenaPlan{})
		c.grappleWon(t)
		c.advance(t, c.hob)
		c.advance(t, "Toren")
		if _, err := c.moveTo(t, a.master, "Pensantus", 3, 2); err != nil {
			t.Fatalf("placing Pensantus: %v", err)
		}
		before, events := c.moveRowOf(t, "Toren"), c.events(t)
		_, err := c.frozenMove(t, a.caio, "Toren", 3, 1)
		c.frozenWantRefusal(t, err, contestBlockedBy("NO_ROOM_TO_DRAG", "there is no room to leave the creature"))
		if c.moveRowOf(t, "Toren") != before || c.events(t) != events {
			t.Error("a refused drag changed the grappler or wrote an event")
		}
	})
}

// A surprised creature does not move on its first turn.
func TestMoveCombatantFrozen_Surprised(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{setup: true})
	c.mustSetSurprised(t, "Toren", true)
	c.e = a.begin(t, c.e)
	_, err := c.frozenMove(t, a.caio, "Toren", 4, 3)
	c.frozenWantRefusal(t, err, contestBlockedBy("SURPRISED", "the combatant is surprised"))
	// The master, who moves a token off the rules, is not held by it.
	c.frozenCheck(t, a.master, "Toren", 4, 2, nil, nil, moveWant{col: 4, row: 2, usedDft: 0, lastDft: 0, event: `{"col": 4, "combatant_id": "<Toren>", "distance_dft": 71, "distance_ft": 7, "from": {"col": 3, "cover": "none", "placed": true, "row": 3}, "on_turn": true, "round": 1, "row": 2}`})
}

// An armed trap on the way stops the move, and fires where the move ends.
func TestMoveCombatantFrozen_Traps(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		trap [2]int
		as   string // "master" or Toren's player
		col  int32
		edit []func(*playv1.MoveCombatantRequest)
		want moveWant
	}{
		{
			name: "a walk stops on the first square of the area", trap: [2]int{9, 7}, col: 12,
			want: moveWant{col: 9, row: 7, usedDft: 150, lastDft: 150, stopped: true, event: `{"col": 9, "combatant_id": "<Toren>", "cost_dft": 150, "cost_ft": 15, "distance_dft": 150, "distance_ft": 15, "from": {"col": 6, "cover": "none", "placed": true, "row": 7}, "on_turn": true, "round": 1, "row": 7, "spent_dft": 150, "stopped_early": true}`},
		},
		{
			name: "a walk that ends in the area is not cut short", trap: [2]int{9, 7}, col: 9,
			want: moveWant{col: 9, row: 7, usedDft: 150, lastDft: 150, event: `{"col": 9, "combatant_id": "<Toren>", "cost_dft": 150, "cost_ft": 15, "distance_dft": 150, "distance_ft": 15, "from": {"col": 6, "cover": "none", "placed": true, "row": 7}, "on_turn": true, "round": 1, "row": 7, "spent_dft": 150}`},
		},
		{
			name: "a long jump fires where it lands", trap: [2]int{7, 7}, col: 7, edit: []func(*playv1.MoveCombatantRequest){jumpTo(playv1.JumpKind_JUMP_KIND_LONG)},
			want: moveWant{col: 7, row: 7, usedDft: 50, lastDft: 0, event: `{"col": 7, "combatant_id": "<Toren>", "cost_dft": 50, "cost_ft": 5, "distance_dft": 50, "distance_ft": 5, "from": {"col": 6, "cover": "none", "placed": true, "row": 7}, "jump": "long", "on_turn": true, "round": 1, "row": 7}`},
		},
		{
			name: "the master's move fires where it ends", trap: [2]int{9, 7}, as: "master", col: 9,
			want: moveWant{col: 9, row: 7, usedDft: 0, lastDft: 0, event: `{"col": 9, "combatant_id": "<Toren>", "distance_dft": 150, "distance_ft": 15, "from": {"col": 6, "cover": "none", "placed": true, "row": 7}, "on_turn": true, "round": 1, "row": 7}`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newTrapRig(t)
			r.trap(t, "Fosso Dourado", tc.trap[0], tc.trap[1], pit("1d6"))
			r.fight(t)
			r.h.roller.queue(4)
			u := r.caio
			if tc.as == "master" {
				u = r.master
			}
			fired := r.moveKindCount(t, "trap_triggered")
			r.frozenCheck(t, u, "Toren", tc.col, 7, tc.edit, nil, tc.want)
			if got := r.moveKindCount(t, "trap_triggered") - fired; got != 1 {
				t.Errorf("trap_triggered events written = %d, want 1", got)
			}
		})
	}
}
