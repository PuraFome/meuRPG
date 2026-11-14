package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// RN-10: the save of a hit effect on an NPC. The audience of the spell decides who reads the spell's name; it does not
// decide whether the attacker's player learns what the attacker's own hit did. That player reads the outcome of the
// NPC's save (saved or failed, and the condition it gave), never the NPC's die or total; the spell's name is theirs when
// they cast it or when every player may read it, and "Outra fonte" otherwise; the master reads all; another player
// reads nothing of an effect that is for its owner alone.

// onAnotherCharactersWeapon moves the row of the cast to the attacker, as a spell another character put on its weapon
// would leave it: the row is the attacker's and its source is still the caster. (No spell does it yet: the caster's
// own hits are what a spell adds to today.)
func (a *armed) onAnotherCharactersWeapon(t *testing.T, holder string) {
	t.Helper()
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE combatant_states SET combatant_id = $2 WHERE source_key = $1`, tableMark, a.id(t, holder)); err != nil {
		t.Fatalf("move the row to %s: %v", holder, err)
	}
}

// hitWithAnotherCharactersSpell is Toren's hit on the Capitão with Pensantus's spell on his weapon: the weapon, the die
// of the effect and the save (a 4: it fails).
func hitWithAnotherCharactersSpell(t *testing.T, audience rulesv1.TableEffectAudience) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newHitCasters(t, func(s *rulesv1.TableSpell) { s.Effect.Visibility = audience }, saveMod())
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	a.onAnotherCharactersWeapon(t, "Toren")
	e = a.passTo(t, e, "Toren")
	pending := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(19)).GetPendingDamage()
	a.h.roller.queue(1, 1, 4)
	a.mustDamage(t, a.caio, e, pending.GetId(), inAppDamage)
	return a, e
}

func saveMod() *tableMod {
	return hitMod(trigEach, func(m *tableMod) {
		m.Save, m.OnFailConditions, m.ConditionDuration = "wis", []string{frightenedKey}, spellEnd
	})
}

// wantAttackersSaveLine checks the one save line the attacker's player reads: the outcome, never the NPC's numbers,
// and the spell as the name says. The master's line is the canary: it carries the die (a 4) and the spell's name.
func wantAttackersSaveLine(t *testing.T, a *armed, attacker *user, e *playv1.Encounter, wantKey, wantName string) {
	t.Helper()
	master := a.hitRiderEntries(t, a.master, e)
	if len(master) != 1 || !master[0].GetHitRider().GetNumbers() || master[0].GetHitRider().GetD20() != 4 || master[0].GetKeyNamePt() != "Marca de Nanquim" {
		t.Fatalf("the master reads %v, want the line with the NPC's die and the spell's name: the canary is unreachable", master)
	}
	lines := a.hitRiderEntries(t, attacker, e)
	if len(lines) != 1 {
		t.Fatalf("the attacker's player reads %d save lines, want the outcome", len(lines))
	}
	r := lines[0].GetHitRider()
	if r.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED || r.GetConditionNamePt() == "" {
		t.Errorf("the attacker's player reads %v, want a failed save and the condition it gave", r)
	}
	if r.GetNumbers() || r.GetD20() != 0 || r.GetSaveTotal() != 0 || r.GetDc() != 0 || r.GetTreatedAs() != 0 {
		t.Errorf("the attacker's player reads the NPC's numbers: %v", r)
	}
	if lines[0].GetKey() != wantKey || lines[0].GetKeyNamePt() != wantName {
		t.Errorf("the attacker's player reads the spell %q %q, want %q %q", lines[0].GetKey(), lines[0].GetKeyNamePt(), wantKey, wantName)
	}
}

// (a) The attacker's player cast the spell, owner-only: it reads the spell's name and the outcome; the other player
// reads nothing of an effect that is for its owner.
func TestRN10_TheCasterOfAnOwnerOnlyHitEffectReadsItsNameOnTheSaveLine(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, func(s *rulesv1.TableSpell) {
		s.Effect.Visibility = rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER
	}, saveMod())
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 1, 1, 4) // the weapon, the die of the effect, the save (a 4: it fails)
	wantAttackersSaveLine(t, a, a.ana, e, tableMark, "Marca de Nanquim")
	for who, u := range map[string]*user{"Caio": a.caio, "Bia": a.bia} {
		if got := a.hitRiderEntries(t, u, e); len(got) != 0 {
			t.Errorf("%s reads %d save lines of an effect that is for its owner", who, len(got))
		}
	}
}

// (b) A spell another character put on the attacker's weapon, owner-only: the attacker's player did not cast it, so it
// reads "Outra fonte" and the outcome. The master's line carries the name; the caster's player reads no line of it.
func TestRN10_TheAttackerOfAnotherCharactersOwnerOnlyHitEffectReadsOutraFonteOnTheSaveLine(t *testing.T) {
	t.Parallel()
	a, e := hitWithAnotherCharactersSpell(t, rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER)
	wantAttackersSaveLine(t, a, a.caio, e, "", otherSourceTx)
	for who, u := range map[string]*user{"Ana": a.ana, "Bia": a.bia} {
		if got := a.hitRiderEntries(t, u, e); len(got) != 0 {
			t.Errorf("%s reads %d save lines of an effect that is for its owner", who, len(got))
		}
	}
}

// (c) A spell whose audience is all the players: the attacker's player reads its name, whoever cast it.
func TestRN10_TheAttackerOfAPublicHitEffectReadsItsNameOnTheSaveLine(t *testing.T) {
	t.Parallel()
	a, e := hitWithAnotherCharactersSpell(t, rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC)
	wantAttackersSaveLine(t, a, a.caio, e, tableMark, "Marca de Nanquim")
}
