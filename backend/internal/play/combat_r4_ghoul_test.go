package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The master rolls a restrained ghoul's claws at a wizard that has Mage Armor and used Shield.
// A restrained attacker rolls with disadvantage (SRD 5.1, Conditions), so the roll takes two
// d20: one typed face is refused as invalid_argument with the reason in its message (the screen
// shows it, web/src/app/core/combat/combat-errors.ts), and the pair is accepted.
func TestR4_AGhoulsClawsAtAWizardWithMageArmor(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.mustCastOut(t, a.ana, a.pens, mageArmorKey, slotOfLevel(1), false, []*charactersv1.Character{a.pens})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{1},
		players: map[string]int32{"Toren": 10, "Pensantus": 5, "Brisa": 1},
		setup:   true,
		at:      map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {6, 5}, "Brisa": {7, 5}},
	})
	a.h.roller.queue(1)
	res := a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
		r.CreatureKey = "monster:ghoul"
		r.Count = 1
		r.Hidden = new(false)
	})
	label := ""
	for _, c := range res.GetEncounter().GetCombatants() {
		if c.GetId() == res.GetCombatantIds()[0] {
			label = c.GetLabel()
		}
	}
	e = a.begin(t, a.get(t, a.master))
	// The goblin hits her first and she casts Shield; then the ghoul's turn comes.
	e = a.passTo(t, e, "Goblin")
	a.mustAttack(t, a.master, e, "Goblin", sword, "Pensantus", d20(19))
	if w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_SHIELD); w != nil {
		a.mustAnswer(t, a.ana, e, w.GetId(), useAnswer(1))
	}
	e = a.passTo(t, a.get(t, a.master), label)
	a.setConditions(t, e, label, "condition:restrained")

	_, err := a.attack(t, a.master, e, label, "basic:1", "Pensantus", d20(18))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || err == nil || !strings.Contains(err.Error(), "the roll takes 2 d20") {
		t.Fatalf("one typed d20 for a restrained ghoul = %v, want invalid_argument naming the 2 d20", err)
	}
	a.mustAttack(t, a.master, e, label, "basic:1", "Pensantus", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{18, 4} })
}
