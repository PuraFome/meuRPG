package play

import (
	"regexp"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// RN-10: another player never reads a hidden effect's name, spell or row id, and never a 0 that is not a real roll. A
// hidden source reads "Outra fonte" with its amount; a typed damage-type group that holds a hidden part is ONE row for
// them (the visible parts' names and "Outra fonte", the damage type and the group's sum), since the server knows only
// the group's sum. The canaries are the effect's own sum (23, two d12 that came up 11 and 12) and the group's sum (27);
// the master and the attacker's player read the parts whole, so a needle nobody reaches would fail.

var (
	hiddenEffectSum = regexp.MustCompile(`"sum":\s*23\b`)
	typedGroupSum   = regexp.MustCompile(`"sum":\s*27\b`)
)

// effectRowID is the id of the spell's effect row: what a hidden part carries for a key.
func (a *armed) effectRowID(t *testing.T) string {
	t.Helper()
	var id string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT id FROM combatant_states WHERE source_key = $1`, tableMark).Scan(&id); err != nil {
		t.Fatalf("the effect row: %v", err)
	}
	return id
}

// leaksOfEffect are the words of a hidden effect that another player must not read: the spell's name and key and the
// flag that says a spell made the part.
func leaksOfEffect(text string) []string {
	var out []string
	for _, needle := range []string{"Marca de Nanquim", tableMark, `"hitEffect": true`, `"hit_effect": true`} {
		if strings.Contains(text, needle) {
			out = append(out, needle)
		}
	}
	return out
}

func newRowsFight(t *testing.T, audience rulesv1.TableEffectAudience) (*armed, *playv1.Encounter, *playv1.PendingDamage) {
	t.Helper()
	a := newHitCasters(t, func(s *rulesv1.TableSpell) { s.Effect.Visibility = audience },
		hitMod(trigEach, func(m *tableMod) { m.Die, m.Dice, m.MaxDice = 12, 2, 13 }))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	hit := a.strike(t, e, battleaxe, "Goblin", d20(19))
	return a, e, hit
}

// readStored is the stored pending damage as a viewer reads it: the parts and the part rolls, as the owner (and the master)
// or as the other players. The log names the spell of the cast itself, which is public, so it is not read here.
func (a *armed) readStored(t *testing.T, e *playv1.Encounter, hit *playv1.PendingDamage, u *user, others bool) (text string, rolls []*playv1.DamagePartRoll) {
	t.Helper()
	p, err := playdb.New(a.h.pool).GetPendingDamage(t.Context(), playdb.GetPendingDamageParams{EncounterID: e.GetId(), ID: hit.GetId()})
	if err != nil {
		t.Fatalf("GetPendingDamage() error = %v", err)
	}
	view := &playv1.PendingDamage{Parts: partsProtoFor(p, combat.CriticalDoubledDice, dtPT, !others), PartRolls: partRollsProto(p, !others && u == a.master, !others, dtPT)}
	return jsonOf(view), view.GetPartRolls()
}

func TestRN10_AHiddenEffectPartReadsAsOtherSourceForOthersInTheRolledDice(t *testing.T) {
	t.Parallel()
	a, e, hit := newRowsFight(t, rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER)
	a.rollDamage(t, hit, 3, 11, 12) // the weapon, the two dice of the effect
	row := a.effectRowID(t)
	for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
		text, _ := a.readStored(t, e, hit, u, false)
		if !hiddenEffectSum.MatchString(text) || !strings.Contains(text, "Marca de Nanquim") {
			t.Errorf("%s does not read the effect's row and sum: the canary is unreachable", name)
		}
	}
	for name, u := range map[string]*user{"Caio": a.caio, "Bia": a.bia} {
		text, rolls := a.readStored(t, e, hit, u, true)
		if words := leaksOfEffect(text); len(words) > 0 {
			t.Errorf("%s reads %q of the hidden effect", name, words)
		}
		if len(rolls) != 2 || rolls[1].GetLabelPt() != otherSourceTx || rolls[1].GetSum() != 23 || rolls[1].GetPartKey() != row {
			t.Errorf("%s reads the rolls %v, want the weapon and an Outra fonte row of 23", name, rolls)
		}
	}
}

func TestRN10_AGroupThatHoldsAHiddenPartIsOneRowForOthers(t *testing.T) {
	t.Parallel()
	a, e, hit := newRowsFight(t, rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER)
	a.mustDamage(t, a.ana, e, hit.GetId(), extrasMarked(nil, &playv1.TypedPart{DamageType: "damage-type:slashing", Sum: 27}))
	for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
		text, rolls := a.readStored(t, e, hit, u, false)
		if !typedGroupSum.MatchString(text) || !strings.Contains(text, "Marca de Nanquim") || len(rolls) != 2 {
			t.Errorf("%s reads %d rolls and %s: the canary is unreachable", name, len(rolls), text)
		}
	}
	for name, u := range map[string]*user{"Caio": a.caio, "Bia": a.bia} {
		text, rolls := a.readStored(t, e, hit, u, true)
		if words := leaksOfEffect(text); len(words) > 0 {
			t.Errorf("%s reads %q of the hidden effect", name, words)
		}
		if len(rolls) != 1 || rolls[0].GetSum() != 27 || rolls[0].GetDamageTypeKey() != "damage-type:slashing" ||
			!strings.Contains(rolls[0].GetLabelPt(), "Machado de batalha") || !strings.Contains(rolls[0].GetLabelPt(), otherSourceTx) {
			t.Errorf("%s reads the rolls %v, want one row of the group: the weapon, Outra fonte and the sum 27", name, rolls)
		}
		for _, r := range rolls {
			if r.GetSum() == 0 && r.GetDiceCount() > 0 && len(r.GetFaces()) == 0 {
				t.Errorf("%s reads %s with dice and a sum of 0", name, r.GetPartKey())
			}
		}
	}
}

// A public effect is read as before: its row, its name and its amount.
func TestRN10_APublicEffectPartKeepsItsRowForOthers(t *testing.T) {
	t.Parallel()
	a, e, hit := newRowsFight(t, rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC)
	a.rollDamage(t, hit, 3, 11, 12)
	for name, u := range map[string]*user{"Caio": a.caio, "Bia": a.bia} {
		text, _ := a.readStored(t, e, hit, u, true)
		if !hiddenEffectSum.MatchString(text) || !strings.Contains(text, "Marca de Nanquim") {
			t.Errorf("%s does not read the public effect's row and amount", name)
		}
		if strings.Contains(text, `"diceSides": 12`) {
			t.Errorf("%s reads the dice of the effect: only its amount is theirs", name)
		}
	}
}
