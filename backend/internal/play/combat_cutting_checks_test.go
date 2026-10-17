package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Cutting Words on an ability check and on an NPC's spell attack roll (SRD, College of Lore):
// the Bardic Inspiration die comes off a check of a creature of the other side that the bard
// sees within 60 feet, and off its spell attack roll, as it comes off a weapon attack. The
// check waits in the same window, with the same die, use and reaction; the "todos" setting asks
// for the check, the default one asks for the spell attack only.

// cuttingArena is the contest arena (Toren, the Hobgoblin and the Goblin on the map) where
// Brisa is a bard of the College of Lore with the given setting.
func cuttingArena(t *testing.T, ask charactersv1.CuttingWordsAsk) *cx {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, scores16(), []string{battleaxe}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: abilities(10, 14, 12, 10, 16), RaceKey: "race:human", Classes: []*charactersv1.ClassLevel{classLevel("class:bard", 3, "subclass:lore")},
			WeaponKeys: []string{rapier}, CuttingWordsAsk: ask,
		}}}
		res, err := a.bia.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Brisa", Sheet: sheet,
		}))
		if err != nil {
			t.Fatalf("CreateCharacter(Brisa) error = %v", err)
		}
		a.bri = res.Msg.GetCharacter()
	})
	return a.arena(t, arenaPlan{})
}

// cutWindow is the bard's open Cutting Words window, or nil.
func (c *cx) cutWindow(t *testing.T) *playv1.ReactionWindow {
	t.Helper()
	return c.windowOf(t, c.bia, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS)
}

// contestsOf are the contests the master reads.
func (c *cx) contestsOf(t *testing.T) []*playv1.ContestView {
	t.Helper()
	return c.state(t, c.master).GetContests()
}

// TestCuttingWordsTakesTheDieOffAnNPCsContestRoll: the Hobgoblin grapples Toren; the bard sees
// it, so the roll waits for her (nothing is rolled or opened before), and the die she rolls
// comes off the initiator's total.
func TestCuttingWordsTakesTheDieOffAnNPCsContestRoll(t *testing.T) {
	t.Parallel()
	c := cuttingArena(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	c.advance(t, c.hob)
	held, err := c.grapple(t, c.master, c.hob, "Toren", 12)
	if err != nil {
		t.Fatalf("StartContest() error = %v", err)
	}
	if held.GetContest() != nil || len(c.contestsOf(t)) != 0 {
		t.Fatalf("the contest exists while the bard decides: %v", held.GetContest())
	}
	w := c.cutWindow(t)
	if w == nil || w.GetCuttingWords().GetRollKind() != playv1.ReactionRollKind_REACTION_ROLL_KIND_TEST {
		t.Fatalf("the bard's window = %v, want a Cutting Words on the check", w)
	}
	res := c.mustAnswer(t, c.bia, c.e, w.GetId(), typedAnswer(4))
	if cw := res.GetResult().GetCuttingWords(); !cw.GetEffective() || cw.GetRollKind() != playv1.ReactionRollKind_REACTION_ROLL_KIND_TEST || cw.GetDie().GetTotal() != 4 {
		t.Fatalf("the result = %v, want a die 4 taken off the check", res.GetResult())
	}
	cs := c.contestsOf(t)
	if len(cs) != 1 {
		t.Fatalf("contests after the answer = %d, want 1", len(cs))
	}
	roll := cs[0].GetInitiatorRoll()
	if want := roll.GetFaces()[0] + roll.GetModifier() - 4; roll.GetFaces()[0] != 12 || roll.GetTotal() != want {
		t.Errorf("the initiator's roll = %v, want the d20 12 and a total 4 lower than the sum", roll)
	}
	if !byLabel(t, res.GetEncounter(), "Brisa").GetReactionUsed() {
		t.Error("the bard's reaction is not spent")
	}
	// The master reads the numbers of the cut; the bard's player reads that the check went down, and no
	// total of the creature (RN-10, RN-20).
	if !c.logSays(t, c.master, c.e, "do teste de") || !c.logSays(t, c.master, c.e, "13 → 9") {
		t.Error("the master's log does not say the check went from 13 to 9")
	}
	if !c.logSays(t, c.bia, c.e, "Palavras de Interrupção") || c.logSays(t, c.bia, c.e, "13") || c.logSays(t, c.bia, c.e, " 9") {
		t.Error("the bard's log hides the cut or tells a total of the creature")
	}
}

// TestCuttingWordsTakesTheDieOffAnNPCsAnswerToAContest: Toren grapples the Hobgoblin; the
// master rolls its answer, which waits for the bard.
func TestCuttingWordsTakesTheDieOffAnNPCsAnswerToAContest(t *testing.T) {
	t.Parallel()
	c := cuttingArena(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	if _, err := c.grapple(t, c.caio, "Toren", c.hob, 10); err != nil {
		t.Fatalf("StartContest(Toren) error = %v", err)
	}
	if w := c.cutWindow(t); w != nil {
		t.Fatalf("a window opened for the bard's ally's roll: %v", w)
	}
	id := c.contestsOf(t)[0].GetId()
	c.h.roller.queue(15)
	if _, err := c.master.contests.RespondContest(t.Context(), connect.NewRequest(&playv1.RespondContestRequest{
		CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), ContestId: id, Skill: playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, Roll: rollIn(),
	})); err != nil {
		t.Fatalf("RespondContest() error = %v", err)
	}
	if v := c.contestsOf(t)[0]; v.GetDefenderRoll() != nil {
		t.Fatalf("the defender's roll exists while the bard decides: %v", v.GetDefenderRoll())
	}
	w := c.cutWindow(t)
	if w == nil {
		t.Fatal("no Cutting Words window for the defender's roll")
	}
	c.mustAnswer(t, c.bia, c.e, w.GetId(), typedAnswer(5))
	roll := c.contestsOf(t)[0].GetDefenderRoll()
	if want := roll.GetFaces()[0] + roll.GetModifier() - 5; roll.GetFaces()[0] != 15 || roll.GetTotal() != want {
		t.Errorf("the defender's roll = %v, want the d20 15 and a total 5 lower than the sum", roll)
	}
}

// TestCuttingWordsTakesTheDieOffAnNPCsHide: a Stealth check of the Goblin waits for the bard.
func TestCuttingWordsTakesTheDieOffAnNPCsHide(t *testing.T) {
	t.Parallel()
	c := cuttingArena(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	c.advance(t, "Goblin")
	if _, err := c.hide(t, c.master, "Goblin", actionHide, 14); err != nil {
		t.Fatalf("Hide() error = %v", err)
	}
	if n := len(c.state(t, c.master).GetHideAttempts()); n != 0 {
		t.Fatalf("hide attempts while the bard decides = %d, want 0", n)
	}
	w := c.cutWindow(t)
	if w == nil {
		t.Fatal("no Cutting Words window for the Stealth check")
	}
	c.mustAnswer(t, c.bia, c.e, w.GetId(), typedAnswer(3))
	at := c.state(t, c.master).GetHideAttempts()
	if len(at) != 1 {
		t.Fatalf("hide attempts = %d, want 1", len(at))
	}
	if roll := at[0].GetRoll(); roll.GetFaces()[0] != 14 || roll.GetTotal() != roll.GetFaces()[0]+roll.GetModifier()-3 {
		t.Errorf("the Stealth roll = %v, want the d20 14 and a total 3 lower than the sum", roll)
	}
}

// TestCuttingWordsAsksForAChecksOnlyInTheTodosSetting: the default setting asks for attacks,
// so a check goes through; so does "nunca".
func TestCuttingWordsAsksForAChecksOnlyInTheTodosSetting(t *testing.T) {
	t.Parallel()
	for name, ask := range map[string]charactersv1.CuttingWordsAsk{
		"attacks": charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ONLY_ATTACKS, "never": charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_NEVER,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := cuttingArena(t, ask)
			c.advance(t, c.hob)
			held, err := c.grapple(t, c.master, c.hob, "Toren", 12)
			if err != nil {
				t.Fatalf("StartContest() error = %v", err)
			}
			if held.GetContest() == nil || c.cutWindow(t) != nil {
				t.Errorf("the contest = %v, window = %v; want the contest rolled and no window", held.GetContest(), c.cutWindow(t))
			}
		})
	}
}

// TestCuttingWordsOpensNoWindowForAHiddenNPCsCheck (RN-10): a window would tell the bard's
// player that a creature she does not see makes a check.
func TestCuttingWordsOpensNoWindowForAHiddenNPCsCheck(t *testing.T) {
	t.Parallel()
	c := cuttingArena(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	c.advance(t, c.hob)
	if _, err := c.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: c.campaignID, EncounterId: c.e.GetId(), CombatantId: c.id(t, c.hob), IdempotencyKey: newKey(), Hidden: true,
	})); err != nil {
		t.Fatalf("SetCombatantHidden() error = %v", err)
	}
	held, err := c.grapple(t, c.master, c.hob, "Toren", 12)
	if err != nil {
		t.Fatalf("StartContest() error = %v", err)
	}
	if held.GetContest() == nil || c.cutWindow(t) != nil {
		t.Errorf("the contest = %v, window = %v; want the contest rolled and no window for the bard", held.GetContest(), c.cutWindow(t))
	}
}

// cuttingBossFight is the fight of counterFight with Brisa as a bard of the College of Lore
// in place of the cleric (the default setting).
func cuttingBossFight(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5, scores16(), []string{battleaxe}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: abilities(10, 14, 12, 10, 16), RaceKey: "race:human", Classes: []*charactersv1.ClassLevel{classLevel("class:bard", 3, "subclass:lore")},
			WeaponKeys: []string{rapier},
		}}}
		res, err := a.bia.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Brisa", Sheet: sheet,
		}))
		if err != nil {
			t.Fatalf("CreateCharacter(Brisa) error = %v", err)
		}
		a.bri = res.Msg.GetCharacter()
	})
	id := a.master.bossCaster(t, a.campaignID, "Mago Sombrio", 7, []string{fireBolt}, []string{"spell:acid-arrow"}, []string{"spell:acid-arrow"})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: id}}, npcRolls: []int{20}, reveal: []string{"Mago Sombrio"},
		at:      map[string][2]int32{"Mago Sombrio": {2, 5}, "Pensantus": {5, 5}, "Toren": {6, 5}, "Brisa": {5, 6}},
		players: map[string]int32{"Pensantus": 10, "Toren": 9, "Brisa": 8},
	})
	return a, e
}

// TestCuttingWordsTakesTheDieOffAnNPCsSpellAttackRoll: the Mago Sombrio casts Acid Arrow (a ranged spell attack, cast from a slot) at
// Toren. The cast waits for the bard (the default setting asks for attack rolls); the die comes
// off the spell attack's bonus, and the cast then lands with what the bard's answer made of it.
func TestCuttingWordsTakesTheDieOffAnNPCsSpellAttackRoll(t *testing.T) {
	t.Parallel()
	a, e := cuttingBossFight(t)
	held, err := a.cast(t, a.master, e, "Mago Sombrio", "spell:acid-arrow", slotOfLevel(2), a.at(t, "Toren"), d20Face(10))
	if err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	if held.GetCast() != nil {
		t.Fatalf("the cast answers %v while the bard decides", held.GetCast())
	}
	w := a.windowOf(t, a.bia, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS)
	if w == nil || w.GetCuttingWords().GetRollKind() != playv1.ReactionRollKind_REACTION_ROLL_KIND_ATTACK {
		t.Fatalf("the bard's window = %v, want a Cutting Words on the attack roll", w)
	}
	res := a.mustAnswer(t, a.bia, e, w.GetId(), typedAnswer(5))
	// 10 + 7 = 17 reaches Toren's armor class; with the die 5 off it (12) it does not.
	if cw := res.GetResult().GetCuttingWords(); !cw.GetEffective() || cw.GetDie().GetTotal() != 5 || cw.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS {
		t.Fatalf("the result = %v, want a die 5 taken off the spell attack and a miss", res.GetResult())
	}
	if !a.logSays(t, a.master, e, "Palavras de Interrupção") {
		t.Error("the master's log has no Cutting Words line")
	}
}
