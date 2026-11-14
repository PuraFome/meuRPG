package play

import (
	"testing"
	"time"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// heldReleaseFight is a fight where Aurora (Caio's wizard) readied Magic Missile on the goblin, the goblin's turn has come,
// the master fired it and Caio answered "Usar": the release waits for the Mago's Counterspell window.
func heldReleaseFight(t *testing.T) (*armed, *playv1.Encounter, *charactersv1.Character, *playv1.ReactionWindow) {
	t.Helper()
	var aurora *charactersv1.Character
	a := newArmedWith(t, func(a *armed) {
		aurora = a.caio.caster(t, a.campaignID, "Aurora", "class:wizard", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{magicMissileSpell}, []string{magicMissileSpell})
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{maceKey}, []string{sacredFlame}, nil,
			[]string{cureWounds})
	})
	mago := a.master.bossCaster(t, a.campaignID, "Mago Sombrio", 7, []string{fireBolt}, []string{counterspellSpell}, []string{counterspellSpell})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}, {CharacterId: mago}}, npcRolls: []int{1, 1}, reveal: []string{"Goblin", "Mago Sombrio"},
		players: map[string]int32{"Aurora": 20, "Brisa": 8},
		at:      map[string][2]int32{"Aurora": {5, 5}, "Mago Sombrio": {5, 8}, "Brisa": {4, 6}, "Goblin": {9, 5}},
	})
	e = a.onTurnOf(t, e, "Aurora")
	a.mustReady(t, a.caio, a.readySpell(t, e, "Aurora", magicMissileSpell, 1, "Goblin"))
	e = a.onTurnOf(t, e, "Goblin")
	w := a.mustFire(t, e, "Aurora")
	held, err := a.cast(t, a.caio, e, "Aurora", magicMissileSpell, nil, []*playv1.SpellTarget{darts(a, t, "Goblin", 3)}, noCastRoll)
	if err != nil || held.GetCast() != nil {
		t.Fatalf("the release = %v, %v; want it held for Counterspell", held, err)
	}
	return a, e, aurora, w
}

// The readier chose "Usar" and its release waits for a Counterspell window. A break of the held spell's concentration then
// (the master ends it, or a correction takes the readier to 0 hit points) must not turn the release into an ordinary
// off-turn cast, and no write may keep failing: after the window is answered the table moves on.
func TestABreakWhileTheReleaseIsHeldNeverMakesItAnOrdinaryCast(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		brk  func(t *testing.T, a *armed, e *playv1.Encounter, aurora *charactersv1.Character)
	}{
		{"the master ends its concentration", func(t *testing.T, a *armed, e *playv1.Encounter, _ *charactersv1.Character) {
			t.Helper()
			if _, err := a.conditions(t, a.master, e, "Aurora", nil, false, true); err != nil {
				t.Fatalf("SetCombatantConditions(end_concentration) error = %v", err)
			}
		}},
		{"a correction to 0 hit points", func(t *testing.T, a *armed, _ *playv1.Encounter, aurora *charactersv1.Character) {
			t.Helper()
			a.correct(t, aurora, hpIs(0))
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a, e, aurora, _ := heldReleaseFight(t)
			c.brk(t, a, e, aurora)
			hp := byLabel(t, a.get(t, a.master), "Goblin").GetHitPointsCurrent()
			cw := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
			if cw == nil {
				t.Fatal("the Mago lost his Counterspell window")
			}
			a.mustAnswer(t, a.master, e, cw.GetId(), passAnswer)
			if got := a.readiedStatus(t, "Aurora"); got != readiedBroken {
				t.Errorf("the row = %q, want broken", got)
			}
			if got := byLabel(t, a.get(t, a.master), "Goblin").GetHitPointsCurrent(); got != hp {
				t.Errorf("the goblin's hit points = %d, want %d: a lost spell was cast", got, hp)
			}
			var state string
			if err := a.h.pool.QueryRow(t.Context(), `SELECT state FROM reaction_holds WHERE kind = 'cast' AND encounter_id = $1`, e.GetId()).Scan(&state); err != nil || state != "dropped" {
				t.Errorf("the held release = %q, %v; want dropped", state, err)
			}
			for _, r := range a.log(t, a.master, e).GetRounds() {
				for _, l := range r.GetEntries() {
					if l.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_SPELL_CAST {
						t.Errorf("a cast line after a lost release: %v", l)
					}
				}
			}
			if _, err := a.endTurn(t, a.master, e, true); err != nil {
				t.Errorf("EndTurn after the release was answered: %v", err)
			}
		})
	}
}

// A readied attack waits for its roller's answer about a Bardic Inspiration die and the player never answers: the master
// answers the question after the player's first 10 s (AnswerHeldRoll), which ends the hold, uses the READIED window and
// frees the table. The fire's undo, refused while the roll was held, needs no one else.
func TestTheMasterEndsAHeldReadiedRollTheRollersPlayerNeverAnswers(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyThenPass(t, nil)
	a.giveCombatantDie(t, "Toren", e)
	w := a.mustFire(t, e, "Toren")
	res, err := a.attack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { d20(10)(r); r.CatchWindowId = w.GetId() })
	if err != nil || res.GetHeldRollOffer() == nil {
		t.Fatalf("the readied attack = %v, %v; want it held for the die", res, err)
	}
	hold := res.GetHeldRollOffer().GetHoldId()
	if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err == nil {
		t.Error("the fire's undo went through while its use waits for the die")
	}
	_, err = a.answerHeldRoll(t, a.master, hold, dieAnswer(false))
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_PLAYER_FIRST)
	a.h.clock.advance(11 * time.Second)
	if _, err := a.answerHeldRoll(t, a.master, hold, dieAnswer(false)); err != nil {
		t.Fatalf("the master's answer after the player's first chance: %v", err)
	}
	if got := a.readiedStatus(t, "Toren"); got != readiedFired || a.readiedOpen(t, "Toren") {
		t.Errorf("after the master's answer: rows %q, window open %v", got, a.readiedOpen(t, "Toren"))
	}
	if _, err := a.endTurn(t, a.master, e, true); err != nil {
		t.Errorf("EndTurn after the master's answer: %v", err)
	}
}
