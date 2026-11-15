package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A readied spell that needs the map (a point, a direction, a wall) is released with CastSpell by the readier's player
// or the master while the readier's READIED window is open (#857). These tests are the server side of that path.

// readiedInTheCave readies the spell at 1st level on Pensantus's turn in the area cave, spends every slot he has left,
// and gives the turn on: the row waits and its window is not fired yet.
func (c *cave) readiedInTheCave(t *testing.T, spell string) *playv1.Encounter {
	t.Helper()
	e := c.areaFight(t)
	c.mustReady(t, c.ana, c.readySpell(t, e, "Pensantus", spell, 1, ""))
	c.execSQL(t, `UPDATE character_vitals SET spell_slots_used = '{9,9,9}' WHERE character_id = $1`, c.pens.GetId())
	return c.mustEndTurn(t, c.master, e)
}

// While the READIED window is open the readier previews the held spell's area at the held slot, with no free slot of that
// level left and no slot in the request; with no open window the preview is refused as before.
func TestAReadierPreviewsTheAreaOfItsHeldSpellWhileTheWindowIsOpen(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, spell string
		origin      *playv1.SpellOrigin
		dir         *playv1.SpellDirection
	}{
		{"a point", sleepSpell, at(20, 7), nil},
		{"a direction", burningHands, nil, toward(1, 0)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newAreaCave(t)
			e := c.readiedInTheCave(t, tt.spell)
			_, err := c.preview(t, c.ana, "Pensantus", tt.spell, nil, tt.origin, tt.dir)
			wantCode(t, "a preview while the row waits and no window is open", err, connect.CodeInvalidArgument)
			w := c.mustFire(t, e, "Pensantus")
			for who, u := range map[string]*user{"the readier's player": c.ana, "the master": c.master} {
				res, err := c.preview(t, u, "Pensantus", tt.spell, nil, tt.origin, tt.dir)
				if err != nil || len(res.GetSquares()) == 0 {
					t.Errorf("%s: preview = %v, %v; want the area of the held spell", who, res, err)
				}
			}
			if _, err := c.preview(t, c.ana, "Pensantus", tt.spell, slotOfLevel(3), tt.origin, tt.dir); err != nil {
				t.Errorf("a request slot is not read for the held spell: error = %v", err)
			}
			if _, err := c.preview(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil); err == nil {
				t.Error("another spell, with no free slot, was previewed through the window")
			}
			c.mustAnswer(t, c.ana, e, w.GetId(), passAnswer)
			_, err = c.preview(t, c.ana, "Pensantus", tt.spell, nil, tt.origin, tt.dir)
			wantCode(t, "a preview once the window is closed", err, connect.CodeInvalidArgument)
		})
	}
}

// A held spell's row carries the slot level it was held with, to the readier's player and the master; a held cantrip
// carries 0, and another player reads no row.
func TestTheReadiedRowCarriesTheSlotLevelOfTheHeldSpell(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFight(t, 1)
	a.mustReady(t, a.ana, a.readySpell(t, e, "Pensantus", magicMissileSpell, 2, ""))
	for who, u := range map[string]*user{"the readier's player": a.ana, "the master": a.master} {
		if got := byLabel(t, a.get(t, u), "Pensantus").GetReadied().GetSlotLevel(); got != 2 {
			t.Errorf("%s reads slot_level %d, want 2", who, got)
		}
	}
	if got := byLabel(t, a.get(t, a.caio), "Pensantus").GetReadied(); got != nil {
		t.Errorf("another player reads the row %v", got)
	}
	e = a.passTo(t, e, "Brisa")
	a.mustReady(t, a.bia, a.readySpell(t, e, "Brisa", sacredFlame, 0, "Goblin"))
	if row := byLabel(t, a.get(t, a.bia), "Brisa").GetReadied(); row.GetKind() != playv1.ReadiedKind_READIED_KIND_SPELL || row.GetSlotLevel() != 0 {
		t.Errorf("a held cantrip's row = %v, want a spell with slot_level 0", row)
	}
}
