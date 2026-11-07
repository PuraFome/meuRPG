package play

import (
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Both sides of a contest add the roll bonuses of their lasting effects that apply to ability checks, through the
// reader the other checks use. A bonus to saves never enters a contest.

func rollBonusOf(n int32, kind rulesv1.TableEffectRollKind) *tableMod {
	return &tableMod{
		Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ROLL_BONUS, Value: n, AppliesTo: rollKinds(kind),
	}
}

var (
	checkBonus3 = rollBonusOf(3, rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_CHECK)
	saveBonus3  = rollBonusOf(3, rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_SAVE)
)

// contestRolls is the roll of the marked side of a contest, as its player and as the master read it.
type contestRolls struct{ player, master *playv1.CheckRoll }

// markedContest marks Toren (who starts a grapple on the Hobgoblin) or Brisa (whom the Hobgoblin grapples) with Pensantus's
// spell, hides the effect when asked, and plays the contest.
func markedContest(t *testing.T, side string, hide bool, mod *tableMod) contestRolls {
	t.Helper()
	a := newMarkCasters(t, mod)
	c := a.arena(t, arenaPlan{toren: 10, brisa: 11, pens: 20})
	target := map[string]string{"initiator": "Toren", "defender": "Brisa"}[side]
	a.mustCast(t, a.ana, c.e, "Pensantus", tableMark, slotOfLevel(1), a.at(t, target), noCastRoll)
	if hide {
		hideEffect(t, a, c.refresh(t), target)
	}
	var out contestRolls
	if side == "initiator" {
		c.advance(t, "Toren")
		c.h.roller.queue(10)
		started := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), purposeGrappleP
		})
		out.player = started.GetContest().GetInitiatorRoll()
		out.master = c.state(t, a.master).GetContests()[0].GetInitiatorRoll()
		return out
	}
	c.advance(t, c.hob)
	c.h.roller.queue(17)
	started := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Brisa"), purposeGrappleP
	})
	answered := c.mustRespond(t, a.bia, started.GetContest().GetId(), answerSkillAthletics, 8)
	out.player = answered.GetContest().GetDefenderRoll()
	out.master = c.state(t, a.master).GetContests()[0].GetDefenderRoll()
	return out
}

func noteTexts(r *playv1.CheckRoll) string {
	var out []string
	for _, n := range r.GetNotes() {
		out = append(out, n.GetLabelPt())
	}
	return strings.Join(out, " | ")
}

func TestAContestAddsTheCheckBonusOfAnEffect(t *testing.T) {
	t.Parallel()
	for _, side := range []string{"initiator", "defender"} {
		t.Run(side, func(t *testing.T) {
			t.Parallel()
			base := markedContest(t, side, false, saveBonus3)
			with := markedContest(t, side, false, checkBonus3)
			if got, want := with.player.GetModifier(), base.player.GetModifier()+3; got != want {
				t.Errorf("modifier = %d, want %d: the base and the effect's +3", got, want)
			}
			if with.player.GetTotal() != with.player.GetFaces()[0]+with.player.GetModifier() {
				t.Errorf("roll = %v, want the total to be the d20 and the modifier", with.player)
			}
			if txt := noteTexts(with.player); !strings.Contains(txt, "Marca de Nanquim +3") {
				t.Errorf("notes = %q, want the effect and its bonus named", txt)
			}
		})
	}
}

// A bonus to saving throws is no bonus to a contest: the modifier and the notes are as with no effect at all.
func TestAContestIgnoresTheSaveBonusOfAnEffect(t *testing.T) {
	t.Parallel()
	for _, side := range []string{"initiator", "defender"} {
		t.Run(side, func(t *testing.T) {
			t.Parallel()
			saves := markedContest(t, side, false, saveBonus3)
			attacks := markedContest(t, side, false, rollBonusOf(3, rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_ATTACK))
			if saves.player.GetModifier() != attacks.player.GetModifier() {
				t.Errorf("modifier with a save bonus = %d, with an attack bonus = %d, want the same base", saves.player.GetModifier(), attacks.player.GetModifier())
			}
			if txt := noteTexts(saves.player); strings.Contains(txt, "Marca") || strings.Contains(txt, "+3") {
				t.Errorf("notes = %q, want no effect note: a save bonus does not count", txt)
			}
			if saves.player.GetMode() != playv1.RollModeKind_ROLL_MODE_KIND_NORMAL {
				t.Errorf("mode = %v, want normal: a bonus says nothing of advantage", saves.player.GetMode())
			}
		})
	}
}

// RN-10: what the master hides adds its bonus, and the player reads "Outra fonte" while the master reads the effect.
func TestAContestNamesAHiddenEffectToTheMasterAlone(t *testing.T) {
	t.Parallel()
	for _, side := range []string{"initiator", "defender"} {
		t.Run(side, func(t *testing.T) {
			t.Parallel()
			r := markedContest(t, side, true, checkBonus3)
			if txt := noteTexts(r.player); !strings.Contains(txt, "Outra fonte +3") || strings.Contains(txt, "Marca") {
				t.Errorf("the player reads notes %q, want %q and no name", txt, "Outra fonte +3")
			}
			if txt := noteTexts(r.master); !strings.Contains(txt, "Marca de Nanquim +3") {
				t.Errorf("the master reads notes %q, want the effect's name", txt)
			}
			if r.player.GetModifier() != r.master.GetModifier() {
				t.Errorf("the player reads modifier %d and the master %d, want the same number", r.player.GetModifier(), r.master.GetModifier())
			}
		})
	}
}

// The answer a player is offered says what it would roll before the roll: the same bonus, with the same names.
func TestTheAnswerOptionsOfAContestCountTheCheckBonus(t *testing.T) {
	t.Parallel()
	a := newMarkCasters(t, checkBonus3)
	c := a.arena(t, arenaPlan{toren: 10, brisa: 11, pens: 20})
	a.mustCast(t, a.ana, c.e, "Pensantus", tableMark, slotOfLevel(1), a.at(t, "Brisa"), noCastRoll)
	c.advance(t, c.hob)
	c.h.roller.queue(17)
	started := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Brisa"), purposeGrappleP
	})
	opts := c.answerContestOf(t, started.GetContest().GetId()).GetAnswerOptions()
	if len(opts) == 0 {
		t.Fatal("Brisa has no answer options")
	}
	for _, o := range opts {
		var texts []string
		for _, n := range o.GetNotes() {
			texts = append(texts, n.GetLabelPt())
		}
		if got := strings.Join(texts, " | "); !strings.Contains(got, "Marca de Nanquim +3") {
			t.Errorf("option %v has notes %q, want the effect's name and bonus", o.GetSkill(), got)
		}
	}
}
