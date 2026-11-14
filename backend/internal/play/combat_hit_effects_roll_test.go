package play

import (
	"regexp"
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The roll part of #577: the spell's words on its part of the damage (hitEffectNote), the part listed off when the
// weapon is not the effect's, and what the log carries when the concentration ends a hit effect that waits.

// The words of a hit effect are built from its modifier: the trigger when it says something, the save and the
// conditions with their length, and a short sentence when there is no save.
func TestHitEffectNote(t *testing.T) {
	t.Parallel()
	names := func(k string) string {
		return map[string]string{
			"condition:blinded": "Cego", "condition:frightened": "Amedrontado", "condition:prone": "Caído",
		}[k]
	}
	hit := func(trigger, save, duration string, conds ...string) rules.EffectModifier {
		return rules.EffectModifier{Trigger: trigger, Save: save, ConditionDuration: duration, OnFailConditions: conds}
	}
	cases := []struct {
		name string
		mod  rules.EffectModifier
		want string
	}{
		{
			"next melee hit with a save and a condition", hit(rules.HitTriggerNextMeleeHit, "con", "", "condition:blinded"),
			"O efeito acaba com este golpe. Depois do dano, o alvo faz um teste de resistência de Constituição; se falhar, fica Cego até o fim do próximo turno dele.",
		},
		{"next hit, dice alone", hit(rules.HitTriggerNextHit, "", ""), "O efeito acaba com este golpe."},
		{
			"each hit lasting as the spell does", hit(rules.HitTriggerEachHit, "wis", rules.ConditionDurationSpellEnd, "condition:frightened"),
			"O efeito vale em cada golpe enquanto durar. Depois do dano, o alvo faz um teste de resistência de Sabedoria; se falhar, fica Amedrontado enquanto a magia durar.",
		},
		{
			"marked target and two conditions", hit(rules.HitTriggerMarkedTarget, "con", "", "condition:blinded", "condition:prone"),
			"O efeito vale nos golpes contra o alvo marcado. Depois do dano, o alvo faz um teste de resistência de Constituição; se falhar, fica Cego e Caído até o fim do próximo turno dele.",
		},
		{"a save with no condition", hit(rules.HitTriggerNextHit, "wis", ""), "O efeito acaba com este golpe. Depois do dano, o alvo faz um teste de resistência de Sabedoria."},
		{"conditions with no save are never given", hit(rules.HitTriggerEachHit, "", "", "condition:blinded"), "O efeito vale em cada golpe enquanto durar."},
		{"an unreadable condition is dropped", hit(rules.HitTriggerNextHit, "con", "", "condition:unknown", "condition:blinded"), "O efeito acaba com este golpe. Depois do dano, o alvo faz um teste de resistência de Constituição; se falhar, fica Cego até o fim do próximo turno dele."},
		{"no readable condition leaves the save alone", hit(rules.HitTriggerNextHit, "con", "", "condition:unknown"), "O efeito acaba com este golpe. Depois do dano, o alvo faz um teste de resistência de Constituição."},
		{"an unreadable ability leaves the trigger alone", hit(rules.HitTriggerNextHit, "xyz", "", "condition:blinded"), "O efeito acaba com este golpe."},
		{
			"three conditions", hit(rules.HitTriggerNextHit, "con", "", "condition:blinded", "condition:prone", "condition:frightened"),
			"O efeito acaba com este golpe. Depois do dano, o alvo faz um teste de resistência de Constituição; se falhar, fica Cego, Caído e Amedrontado até o fim do próximo turno dele.",
		},
	}
	for _, tc := range cases {
		if got := hitEffectNote(tc.mod, names); got != tc.want {
			t.Errorf("%s: note = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// The part of a hit effect carries the note, and is listed off when a next_melee_hit meets a ranged weapon or a
// thrown one at range (it carries the reason and no note); a row that is spent or not the attacker's is no part at all.
func TestAHitEffectPartNoteAndTheOffPart(t *testing.T) {
	t.Parallel()
	mod := rules.EffectModifier{Kind: rules.ModifierWeaponHitDamage, Trigger: rules.HitTriggerNextMeleeHit, Die: 8, Dice: 2, Save: "con", OnFailConditions: []string{"condition:blinded"}}
	row := withMods(playdb.CombatantState{ID: "row-1", SourceKey: new("spell:x"), PlayerVisible: true, Audience: "all"}, []rules.EffectModifier{mod})
	spent := row
	spent.ID, spent.SpentAt = "row-2", &row.CreatedAt
	hf := func(attack func(*hitFacts)) hitFacts {
		h := hitFacts{
			attacker: playdb.Combatant{ID: "a"}, target: playdb.Combatant{ID: "t", Label: "Zumbi 1"}, attack: linkMeleeAttack(),
			states: map[string][]playdb.CombatantState{"a": {row}}, names: func(k string) string {
				return map[string]string{"ability:con": "Constituição", "condition:blinded": "Cego", "spell:x": "Clarão"}[k]
			},
		}
		attack(&h)
		return h
	}
	on := hf(func(*hitFacts) {}).hitEffectParts()
	if len(on) != 1 || !on[0].Available || on[0].Kind != partAuto || !strings.Contains(on[0].Note, "Depois do dano, o alvo faz um teste de resistência de Constituição") {
		t.Fatalf("the melee part = %+v, want an available automatic part with the spell's words", on)
	}
	off := hf(func(h *hitFacts) { h.attack = link.Attack{Weapon: true, RangeFt: 80} }).hitEffectParts()
	if len(off) != 1 {
		t.Fatalf("the ranged part = %+v, want it listed", off)
	}
	if p := off[0]; p.Available || p.Reason != offMeleeReason || p.Kind == partAuto || p.Choosable || p.Selected || p.Count != 2 || p.Sides != 8 || p.Effect != "row-1" {
		t.Errorf("the off part = %+v, want it unavailable with the reason, never automatic, choosable or selected, and the row's dice", p)
	}
	if offMeleeReason != "só num golpe corpo a corpo. Continua esperando." {
		t.Errorf("the reason = %q", offMeleeReason)
	}
	if none := hf(func(h *hitFacts) {
		h.attack = link.Attack{Weapon: true, RangeFt: 80}
		h.states = map[string][]playdb.CombatantState{"a": {spent}}
	}).hitEffectParts(); len(none) != 0 {
		t.Errorf("a spent row made the parts %+v", none)
	}
	if none := hf(func(h *hitFacts) { h.attack = link.Attack{Weapon: true, RangeFt: 80}; h.attacker.ID = "b" }).hitEffectParts(); len(none) != 0 {
		t.Errorf("another caster's row made the parts %+v", none)
	}
	each := mod
	each.Trigger = rules.HitTriggerMarkedTarget
	marked := withMods(row, []rules.EffectModifier{each})
	if none := hf(func(h *hitFacts) { h.states = map[string][]playdb.CombatantState{"a": {marked}} }).hitEffectParts(); len(none) != 0 {
		t.Errorf("an unmarked target made the parts %+v: only the trigger/weapon mismatch is listed off", none)
	}
}

// The others read no note and no number of a hit effect's part, and never its off part, hidden effect or public: the
// attacker's player and the master read the part and the off part whole. A hit whose only extra is an off row reads
// to the others as a hit with no parts.
func TestAHitPartNoteAndReasonAsOthersReadThem(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		hide   bool
		off    bool
		listed bool
	}{
		{"a public effect on", false, false, true},
		{"a hidden effect on", true, false, true},
		{"a public effect off", false, true, false},
		{"a hidden effect off", true, true, false},
	} {
		part := partRecord{
			Key: tableMark, Label: "Marca de Nanquim", Kind: partAuto, Count: 7, Sides: 12, Source: tableMark, Available: !tc.off, Table: tableHitEffect, Effect: "row-1", Hide: tc.hide,
			Note: "O efeito acaba com este golpe. Depois do dano, o alvo faz um teste de resistência de Sabedoria.",
		}
		if tc.off {
			part.Kind, part.Reason, part.Note = partExtra, offMeleeReason, ""
		}
		weapon := partRecord{Key: partWeapon, Label: "Arco", Kind: partWeapon, Count: 1, Sides: 6, Available: true, Plain: tc.off}
		p := playdb.PendingDamage{Parts: mustJSON([]partRecord{weapon, part})}
		whole := partsProtoFor(p, combat.CriticalDoubledDice, dtPT, true)
		if len(whole) != 2 || whole[1].GetNotePt() != part.Note || whole[1].GetReasonPt() != part.Reason {
			t.Errorf("%s: the owner reads %v, want the note and the reason whole", tc.name, whole)
		}
		others := partsProtoFor(p, combat.CriticalDoubledDice, dtPT, false)
		if tc.off {
			if len(others) != 0 {
				t.Errorf("%s: the others read %v, want a hit with no parts", tc.name, others)
			}
			continue
		}
		if len(others) != 2 {
			t.Fatalf("%s: the others read %d parts, want the weapon and the effect", tc.name, len(others))
		}
		o := others[1]
		if o.GetNotePt() != "" || o.GetDiceCount() != 0 || o.GetDiceSides() != 0 || o.GetReasonPt() != "" || (tc.hide && (o.GetLabelPt() != otherSourceTx || o.GetSourceKey() != "")) {
			t.Errorf("%s: the others read %v, want no note, no dice, no reason", tc.name, o)
		}
	}
}

// offRowReads are the stored pending damage of the ranged hit as a player who is not the attacker's reads it, before
// the roll and after it, and as the attacker's player reads it before.
type offRowReads struct {
	before, after string
	ownerBefore   []*playv1.DamagePart
	amount        int32
}

// readOffRow makes the ranged hit of Pensantus on Toren (a natural 20), with the spell cast first or not, and reads
// the stored pending damage as the others, with the same dice.
func readOffRow(t *testing.T, visibility rulesv1.TableEffectAudience, cast bool) offRowReads {
	t.Helper()
	a := newHitCasters(t, func(s *rulesv1.TableSpell) { s.Effect.Visibility = visibility }, hitMod(trigMelee, func(m *tableMod) { m.Die, m.Dice, m.MaxDice = 12, 7, 13 }))
	e := a.hitFight(t)
	if cast {
		a.castMark(t, e, 1)
	}
	hit := a.strike(t, e, shortbowKey, "Toren", d20(20))
	read := func() string {
		p, err := playdb.New(a.h.pool).GetPendingDamage(t.Context(), playdb.GetPendingDamageParams{EncounterID: e.GetId(), ID: hit.GetId()})
		if err != nil {
			t.Fatalf("GetPendingDamage() error = %v", err)
		}
		return jsonOf(&playv1.PendingDamage{Parts: partsProtoFor(p, combat.CriticalDoubledDice, dtPT, false), PartRolls: partRollsProto(p, false, false, dtPT)})
	}
	out := offRowReads{before: read(), ownerBefore: a.pendingOfPensantus(t, a.ana, e)[0].GetParts()}
	out.amount = a.rollDamage(t, hit, 3).GetAmount()
	out.after = read()
	return out
}

// The off part is read by the caster's player and the master alone: to every other player a hit whose only row is an
// off one is the same hit as one with no row (the parts and the part rolls, before and after the roll, and the amount).
func TestRN10_AnOffRowLooksLikeNoRowToOthers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		visibility rulesv1.TableEffectAudience
	}{
		{"a public effect", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC},
		{"an effect of its owner", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			withRow, noRow := readOffRow(t, tc.visibility, true), readOffRow(t, tc.visibility, false)
			if withRow.before != noRow.before || withRow.after != noRow.after || withRow.amount != noRow.amount {
				t.Errorf("the others read\n%s\n%s\n(%d), want the same as with no row:\n%s\n%s\n(%d)", withRow.before, withRow.after, withRow.amount, noRow.before, noRow.after, noRow.amount)
			}
			var off *playv1.DamagePart
			for _, d := range withRow.ownerBefore {
				if !d.GetAvailable() {
					off = d
				}
			}
			if off == nil || off.GetReasonPt() != offMeleeReason || off.GetDiceCount() != 7 {
				t.Errorf("the caster's player reads %v, want the off part with its reason", withRow.ownerBefore)
			}
		})
	}
}

// A next_melee_hit meets a ranged weapon: the part is listed off with its reason, never rolled, the total is the
// weapon's alone, the row waits through the damage roll and its undo, and the next melee hit still uses it.
func TestAHitSpellNextMeleeHitIsListedOffOnARangedWeapon(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigMelee, func(m *tableMod) { m.Dice = 2 }))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	ranged := a.strike(t, e, shortbowKey, "Capitão Goblin", d20(19))
	part := hitPart(ranged)
	if part == nil || part.GetAvailable() || part.GetAuto() || part.GetChoosable() || part.GetSelected() || part.GetReasonPt() != "só num golpe corpo a corpo. Continua esperando." || part.GetDiceCount() != 2 {
		t.Fatalf("the off part = %v, want it listed with the reason, not automatic, with its dice", part)
	}
	if !strings.Contains(a.readsOf(t, a.ana, e, "Pensantus"), "só num golpe corpo a corpo.") {
		t.Errorf("the caster's player does not read the reason")
	}
	dmg := a.rollDamage(t, ranged, 3)
	if hitRoll(dmg) != nil || dmg.GetAmount() < 3 || dmg.GetAmount() > 3+8 {
		t.Errorf("the damage = %d with the part's roll %v, want the weapon's alone", dmg.GetAmount(), hitRoll(dmg))
	}
	want := dmg.GetAmount()
	if all, spent := a.rows(t); all != 1 || spent != 0 {
		t.Fatalf("after the ranged hit: %d rows, %d spent, want the one, waiting", all, spent)
	}
	a.undoLast(t, e)
	if all, spent := a.rows(t); all != 1 || spent != 0 {
		t.Errorf("after the undo: %d rows, %d spent, want the one, waiting", all, spent)
	}
	if again := a.rollDamage(t, ranged, 3); again.GetAmount() != want {
		t.Errorf("the damage rolled again = %d, want %d: the off part adds nothing", again.GetAmount(), want)
	}
	e = a.nextTurn(t, e)
	melee := a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
	if on := wantHitPart(t, "the melee hit", melee, true); !on.GetAvailable() || on.GetReasonPt() != "" {
		t.Errorf("the melee part = %v, want it on", on)
	}
	a.rollDamage(t, melee, 3, 4, 5)
	if _, spent := a.rows(t); spent != 1 {
		t.Errorf("spent = %d after the melee hit, want 1", spent)
	}
}

// Only the caster's player and the master read the off part and its reason: no other player reads the reason, the name or the dice, of a hidden effect or a public one.
func TestRN10_AnOffHitPartReachesOnlyItsCastersPlayerAndTheMaster(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, func(s *rulesv1.TableSpell) {
		s.Effect.Visibility = rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER
	},
		hitMod(trigMelee, func(m *tableMod) { m.Die, m.Dice, m.MaxDice = 12, 7, 13 }))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	hit := a.strike(t, e, shortbowKey, "Capitão Goblin", d20(19))
	canaries := []*regexp.Regexp{regexp.MustCompile(`corpo a corpo\. Continua esperando`), regexp.MustCompile(`"diceSides":\s*12`), regexp.MustCompile(`"diceCount":\s*7\b`)}
	for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
		if text := a.readsOf(t, u, e, "Pensantus"); !canaries[0].MatchString(text) || !canaries[1].MatchString(text) || !canaries[2].MatchString(text) {
			t.Errorf("%s does not read the reason and the dice before the roll: the canary is unreachable", name)
		}
	}
	others := func(when string) {
		for name, u := range map[string]*user{"Caio": a.caio, "Bia": a.bia} {
			text := a.readsOf(t, u, e, "Pensantus")
			for _, re := range canaries {
				if re.MatchString(text) {
					t.Errorf("%s reads %s of the off part %s", name, re, when)
				}
			}
		}
	}
	others("before the roll")
	a.rollDamage(t, hit, 3)
	others("after the roll")
}

// A hit effect that waits when the caster's concentration ends: the log says it with the lines it already has. The
// effect's ending line (change ENDED, reason "concentration", the effect's key and name, the caster as the actor)
// follows the effect's visibility; the caster's own line is the conditions' one that names the spell it stopped.
func TestAHitSpellEndedByTheConcentrationIsInTheLog(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		visibility rulesv1.TableEffectAudience
		othersRead bool
	}{
		{"a public effect", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC, true},
		{"an effect of its owner", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newHitCasters(t, func(s *rulesv1.TableSpell) { s.Effect.Visibility = tc.visibility }, hitMod(trigNext))
			e := a.hitFight(t)
			a.castMark(t, e, 1)
			a.dropConcentration(t, e)
			ended := func(u *user) *playv1.CombatLogEntry { return a.endedLine(t, u, e) }
			for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
				en := ended(u)
				if en == nil || en.GetEffect().GetReason() != "concentration" || en.GetEffect().GetSourceNamePt() != "Marca de Nanquim" || en.GetActorLabel() != "Pensantus" {
					t.Errorf("%s reads the ending line %v, want ENDED by the concentration, named, with Pensantus as the actor", name, en)
				}
			}
			if got := ended(a.caio) != nil; got != tc.othersRead {
				t.Errorf("another player reads the ending line = %v, want %v", got, tc.othersRead)
			}
			var own *playv1.CombatLogEntry
			for _, r := range a.log(t, a.ana, e).GetRounds() {
				for _, en := range r.GetEntries() {
					if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_CONDITIONS_CHANGED && en.GetConcentrationEndedKey() == tableMark {
						own = en
					}
				}
			}
			if own == nil {
				t.Errorf("the caster reads no conditions line that names %s", tableMark)
			}
		})
	}
}

// endedLine is the line of the log that says an effect ended by the concentration, as u reads it.
func (a *armed) endedLine(t *testing.T, u *user, e *playv1.Encounter) *playv1.CombatLogEntry {
	t.Helper()
	for _, r := range a.log(t, u, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT && en.GetEffect().GetChange() == playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_ENDED {
				return en
			}
		}
	}
	return nil
}
