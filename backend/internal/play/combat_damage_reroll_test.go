package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The damage dice a table feat lets the character roll again (#573, combat_damage_reroll.go): one use of the
// feat's resource covers one damage roll, every die that shows the limit or lower is rolled once more and the new
// face is used. These tests need the database (MEURPG_TEST_DATABASE_URL).

const rerollUses = "sorte_do_dano"

// rerollTable is a table where Toren, a Fighter, has a feat with two uses that rolls a damage die of 2 or lower again.
type rerollTable struct{ *armed }

func newRerollTable(t *testing.T, more ...*rulesv1.TableEffect) *rerollTable {
	t.Helper()
	r := &rerollTable{}
	r.armed = newArmedWith(t, func(a *armed) {
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableFeat{TableFeat: &rulesv1.TableFeat{
				NamePt: "Dado Sortudo", DescPt: []string{"Um talento da mesa."},
				Effects: append([]*rulesv1.TableEffect{
					{Type: "resource", Resource: rerollUses, Max: "2", Recharge: "short_rest"},
					{Type: "reroll", Rolls: []string{"damage"}, AtMost: 2, Resource: rerollUses, Cost: "1"},
				}, more...),
			}},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry() error = %v", err)
		}
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:fighter", 2, ""), abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil, nil)
		a.giveFeat(t, a.toren, res.Msg.GetEntry().GetKey())
	})
	r.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: r.capitao.GetId()}}, npcRolls: []int{3}, reveal: []string{"Capitão Goblin"},
		players: map[string]int32{"Toren": 18},
		at:      map[string][2]int32{"Toren": {3, 3}, "Capitão Goblin": {4, 3}},
	})
	return r
}

func (r *rerollTable) left(t *testing.T) int32 {
	t.Helper()
	left, _ := poolOf(r.vitals(t, r.toren), rerollUses)
	return left
}

// hit makes Toren hit the goblin and rolls the damage in the app with the dice queued, answering the roll.
func (r *rerollTable) hit(t *testing.T, e *playv1.Encounter, faces ...int) (*playv1.RollDamageResponse, *playv1.Encounter) {
	t.Helper()
	res := r.mustAttack(t, r.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(17))
	if res.GetPendingDamage() == nil {
		t.Fatalf("the attack = %v, want a hit", res.GetRoll())
	}
	r.h.roller.queue(faces...)
	return r.mustDamage(t, r.caio, e, res.GetPendingDamage().GetId(), inAppDamage), e
}

func TestDamageDieOfTwoOrLowerIsRolledAgainAndSpendsAUse(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t)
	if got := r.left(t); got != 2 {
		t.Fatalf("uses at the start = %d, want 2", got)
	}
	e := r.turnOf(t, "Toren")
	dmg, _ := r.hit(t, e, 1, 6) // the d8 shows 1, the new one 6
	roll := dmg.GetPendingDamage().GetRoll()
	if got := roll.GetFaces(); len(got) != 1 || got[0] != 6 {
		t.Errorf("faces = %v, want [6] (the new face is the one used)", got)
	}
	if got := r.left(t); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
}

func TestDamageRerollWithNoUsesLeftChangesNothing(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t)
	e := r.turnOf(t, "Toren")
	r.hit(t, e, 1, 6) // spends the first use
	e = r.turnOf(t, "Toren")
	r.hit(t, e, 2, 7) // the second
	if got := r.left(t); got != 0 {
		t.Fatalf("uses left = %d, want 0", got)
	}
	e = r.turnOf(t, "Toren")
	dmg, _ := r.hit(t, e, 1, 7) // no use left: the 1 stays
	if got := dmg.GetPendingDamage().GetRoll().GetFaces(); len(got) != 1 || got[0] != 1 {
		t.Errorf("faces = %v, want [1] (no use left)", got)
	}
	if got := r.left(t); got != 0 {
		t.Errorf("uses left = %d, want 0", got)
	}
}

func TestUndoOfTheDamageGivesTheRerollUseBack(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t)
	e := r.turnOf(t, "Toren")
	r.hit(t, e, 2, 5)
	if got := r.left(t); got != 1 {
		t.Fatalf("uses left after the reroll = %d, want 1", got)
	}
	r.undoLast(t, e) // the damage roll
	if got := r.left(t); got != 2 {
		t.Errorf("uses left after the undo = %d, want 2", got)
	}
}

func TestDamageDieAboveTheLimitIsNotRolledAgainAndSpendsNothing(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t)
	e := r.turnOf(t, "Toren")
	dmg, _ := r.hit(t, e, 3, 8) // 3 is above at_most 2
	if got := dmg.GetPendingDamage().GetRoll().GetFaces(); len(got) != 1 || got[0] != 3 {
		t.Errorf("faces = %v, want [3]", got)
	}
	if got := r.left(t); got != 2 {
		t.Errorf("uses left = %d, want 2", got)
	}
}

// A hit with parts (an extra the feat adds) rolls its dice the same way, keeps the face it replaced in the part's
// rerolls, and the undo gives the use back.
func TestDamageRerollOfAHitWithPartsKeepsTheReplacedFace(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t, &rulesv1.TableEffect{Type: "extra_damage", Applies: "any_target", Value: "3", TextPt: "Dano extra."})
	e := r.turnOf(t, "Toren")
	dmg, _ := r.hit(t, e, 2, 5)
	p := dmg.GetPendingDamage()
	if len(p.GetPartRolls()) == 0 {
		t.Fatalf("the hit has no parts, want the parts path: %v", p)
	}
	var weapon *playv1.DamagePartRoll
	for _, pr := range p.GetPartRolls() {
		if len(pr.GetFaces()) > 0 {
			weapon = pr
		}
	}
	if weapon == nil || weapon.GetFaces()[0] != 5 || len(weapon.GetRerolled()) != 1 || weapon.GetRerolled()[0].GetFrom() != 2 {
		t.Fatalf("the weapon part = %v, want the face 5 after a 2", weapon)
	}
	if got := r.left(t); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
	r.undoLast(t, e)
	if got := r.left(t); got != 2 {
		t.Errorf("uses left after the undo = %d, want 2", got)
	}
}
