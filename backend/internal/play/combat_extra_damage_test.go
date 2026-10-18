package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The extra_damage effect of the table's features on the attack card (a damage added to a weapon hit once
// per turn, when the target is what the effect asks, from a resource if it names one). The names are
// invented; these tests need no database.

const (
	giantFeature = "feature:golpe-de-gigante@mesa"
	giantUses    = "golpes_de_gigante"
)

func giantTrait(applies, resource string) link.ExtraDamage {
	return link.ExtraDamage{Key: giantFeature, NamePT: "Golpe de Gigante", Amount: 3, Applies: applies, Resource: resource, TextPT: "Só contra maiores."}
}

// giantFacts is a hit of a player's character of the given size on a target of the other size.
func giantFacts(attackerSize, targetSize string, extras ...link.ExtraDamage) hitFacts {
	return hitFacts{
		attacker: playdb.Combatant{ID: "a", Kind: kindPlayer, Size: attackerSize, TableOnceTurns: []byte(`{}`)},
		target:   playdb.Combatant{ID: "t", Size: targetSize},
		attack:   link.Attack{Key: "e", Name: "Espada", Weapon: true, Melee: true, DamageType: "damage-type:slashing"},
		sheet:    link.Sheet{Traits: link.Traits{ExtraDamages: extras}},
		usesLeft: map[string]int32{giantUses: 2},
		enc:      playdb.Encounter{Round: 2},
	}
}

func partByKey(parts []partRecord, key string) (partRecord, bool) {
	for _, p := range parts {
		if p.Key == key {
			return p, true
		}
	}
	return partRecord{}, false
}

func TestExtraDamageIsOfferedAfterAHitOnALargerTarget(t *testing.T) {
	t.Parallel()
	parts := giantFacts("medium", "large", giantTrait(rules.ExtraDamageLargerTarget, giantUses)).extraDamageParts()
	p, ok := partByKey(parts, giantFeature)
	if !ok || len(parts) != 1 {
		t.Fatalf("parts = %+v, want the extra damage alone", parts)
	}
	if p.Kind != partExtra || !p.Choosable || p.Selected || !p.Available || p.Flat != 3 || p.Count != 0 || p.Label != "Golpe de Gigante" ||
		p.DamageType != "damage-type:slashing" || p.Resource != giantUses || p.ResourceLeft != 2 || p.Rider != "Só contra maiores." || !p.Once || p.Table != "extra_damage" {
		t.Errorf("part = %+v, want a choosable flat 3 of the weapon's type, once a turn, spending %s", p, giantUses)
	}
}

func TestExtraDamageOnASameOrSmallerTargetIsOffTellingWhy(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"medium", "small", "tiny"} {
		parts := giantFacts("medium", target, giantTrait(rules.ExtraDamageLargerTarget, "")).extraDamageParts()
		p, ok := partByKey(parts, giantFeature)
		if !ok || p.Available || p.Reason == "" {
			t.Errorf("target %s: part = %+v, want it listed and off with a reason", target, p)
		}
	}
	// A small attacker against a medium target: larger, so it holds.
	parts := giantFacts("small", "medium", giantTrait(rules.ExtraDamageLargerTarget, "")).extraDamageParts()
	if p, _ := partByKey(parts, giantFeature); !p.Available {
		t.Errorf("a small attacker on a medium target: part = %+v, want it available", p)
	}
	// An unknown size is medium (the sheet says none).
	parts = giantFacts("", "large", giantTrait(rules.ExtraDamageLargerTarget, "")).extraDamageParts()
	if p, _ := partByKey(parts, giantFeature); !p.Available {
		t.Errorf("an attacker with no size on a large target: part = %+v, want it available", p)
	}
}

func TestExtraDamageAgainstAnyTargetIgnoresSize(t *testing.T) {
	t.Parallel()
	p, ok := partByKey(giantFacts("large", "tiny", giantTrait(rules.ExtraDamageAnyTarget, "")).extraDamageParts(), giantFeature)
	if !ok || !p.Available || p.Resource != "" || p.ResourceLeft != 0 {
		t.Errorf("part = %+v, want it available, with no resource", p)
	}
}

func TestExtraDamageUsedThisTurnIsOffUntilTheNextTurn(t *testing.T) {
	t.Parallel()
	h := giantFacts("medium", "large", giantTrait(rules.ExtraDamageLargerTarget, ""))
	h.attacker.TableOnceTurns = []byte(`{"` + giantFeature + `":"` + turnKey(h.enc) + `"}`)
	if p, _ := partByKey(h.extraDamageParts(), giantFeature); p.Available || p.Reason == "" {
		t.Errorf("used this turn: part = %+v, want it off with a reason", p)
	}
	h.enc.Round = 3 // another turn
	if p, _ := partByKey(h.extraDamageParts(), giantFeature); !p.Available {
		t.Errorf("next turn: part = %+v, want it available again", p)
	}
}

func TestExtraDamageWithoutUsesLeftIsOff(t *testing.T) {
	t.Parallel()
	h := giantFacts("medium", "large", giantTrait(rules.ExtraDamageLargerTarget, giantUses))
	h.usesLeft = map[string]int32{giantUses: 0}
	if p, _ := partByKey(h.extraDamageParts(), giantFeature); p.Available || p.Reason != "Sem usos do recurso" {
		t.Errorf("part = %+v, want it off: no uses left", p)
	}
}

func TestExtraDamageIsForAPlayersWeaponAttackOnly(t *testing.T) {
	t.Parallel()
	npc := giantFacts("medium", "large", giantTrait(rules.ExtraDamageAnyTarget, ""))
	npc.attacker.Kind = "npc"
	if got := npc.extraDamageParts(); len(got) != 0 {
		t.Errorf("an NPC's hit offers %+v", got)
	}
	spell := giantFacts("medium", "large", giantTrait(rules.ExtraDamageAnyTarget, ""))
	spell.attack.Weapon = false
	if got := spell.extraDamageParts(); len(got) != 0 {
		t.Errorf("a spell's hit offers %+v", got)
	}
	if got := giantFacts("medium", "large").extraDamageParts(); len(got) != 0 {
		t.Errorf("a character with no extra damage offers %+v", got)
	}
}

// The extra is one more line of the damage of the hit, beside the weapon: buildParts lists it.
func TestTheHitPartsHoldTheExtraDamageBesideTheWeapon(t *testing.T) {
	t.Parallel()
	h := giantFacts("medium", "large", giantTrait(rules.ExtraDamageLargerTarget, ""))
	h.names = func(string) string { return "" }
	parts := h.buildParts()
	if len(parts) != 2 || parts[0].Key != partWeapon || parts[1].Key != giantFeature {
		t.Fatalf("parts = %+v, want the weapon then the extra damage", parts)
	}
}

// Marking an extra of the table twice is refused, and one per turn is remembered by feature.
func TestTableOnceMarksReadAndWrite(t *testing.T) {
	t.Parallel()
	marks := readTableOnce([]byte(`{"a":"2:x","b":"3:y"}`))
	if marks["a"] != "2:x" || marks["b"] != "3:y" || len(marks) != 2 {
		t.Errorf("marks = %v", marks)
	}
	if got := readTableOnce(nil); len(got) != 0 {
		t.Errorf("nil marks = %v", got)
	}
	if got := readTableOnce([]byte(`not json`)); len(got) != 0 {
		t.Errorf("garbage marks = %v", got)
	}
	marks["c"] = "4:z"
	if again := readTableOnce(writeTableOnce(marks)); again["c"] != "4:z" || len(again) != 3 {
		t.Errorf("round trip = %v", again)
	}
}

// ---- end to end (these need the database, MEURPG_TEST_DATABASE_URL) ----

type giantTable struct {
	*armed
}

// newGiantTable makes a table subclass of the Fighter whose level 3 feature adds 3 damage to a weapon hit
// once per turn, against `applies`, spending a use of a pool of two, and Pensantus, Fighter 5, who has it.
func newGiantTable(t *testing.T, applies string) *giantTable {
	t.Helper()
	g := &giantTable{}
	g.armed = newArmedWith(t, func(a *armed) {
		feature := &rulesv1.TableFeature{
			NamePt: "Golpe de Gigante", DescPt: []string{"Dano extra."},
			Effects: []*rulesv1.TableEffect{
				{Type: "resource", Resource: giantUses, Max: "2", Recharge: "short_rest"},
				{Type: "extra_damage", Applies: applies, Value: "3", Resource: giantUses, TextPt: "Só contra maiores."},
			},
		}
		sub := &rulesv1.TableSubclass{
			NamePt: "Caçador de Gigantes", ClassKey: "class:fighter", DescPt: []string{"Golpes."},
			Levels: []*rulesv1.TableSubclassLevel{{Level: 3, Features: []*rulesv1.TableFeature{feature}}},
		}
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSubclass{TableSubclass: sub},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry() error = %v", err)
		}
		sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 14, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, RaceKey: "race:human",
			Background: &charactersv1.FullSheet_BackgroundKey{BackgroundKey: "background:acolyte"},
			Languages:  []string{"Anão", "Élfico", "Gnômico"},
			Classes: []*charactersv1.ClassLevel{{
				ClassKey: "class:fighter", Level: 5, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: res.Msg.GetEntry().GetKey()},
			}},
			SkillProficiencyKeys: []string{"skill:athletics", "skill:perception"},
			FeatureChoiceKeys:    []string{"feature:fighter-fighting-style-defense"},
			WeaponKeys:           []string{longsword},
			HitPoints:            &charactersv1.HitPoints{Method: charactersv1.HitPointsMethod_HIT_POINTS_METHOD_AVERAGE}, ExperiencePoints: 6500,
		}}}
		made, err := a.ana.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Pensantus", Sheet: sheet,
		}))
		if err != nil {
			t.Fatalf("CreateCharacter(Pensantus) error = %v", err)
		}
		a.pens = made.Msg.GetCharacter()
		if issues := a.pens.GetDerived().GetIssues(); len(issues) != 0 {
			t.Fatalf("the sheet has issues: %v", issues)
		}
		scores := &rulesv1.AbilityScores{Strength: 14, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, scores, []string{battleaxe}, nil)
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, scores, []string{rapier}, nil)
	})
	return g
}

func (g *giantTable) hit(t *testing.T, e *playv1.Encounter) *playv1.PendingDamage {
	t.Helper()
	res := g.mustAttack(t, g.ana, e, "Pensantus", longsword, "Capitão Goblin", d20(19))
	if res.GetPendingDamage() == nil {
		t.Fatalf("the attack = %v, want a hit", res.GetRoll())
	}
	return res.GetPendingDamage()
}

func (g *giantTable) usesLeft(t *testing.T) int32 {
	t.Helper()
	left, _ := poolOf(g.vitals(t, g.pens), giantUses)
	return left
}

// Marking the extra adds its fixed damage once, spends a use, and the same turn offers it off.
func TestExtraDamageIsAddedOncePerTurnAndSpendsAUse(t *testing.T) {
	t.Parallel()
	g := newGiantTable(t, rules.ExtraDamageAnyTarget)
	e := g.passTo(t, g.castersFight(t, 1), "Pensantus")
	first := g.hit(t, e)
	p := partOf(first.GetParts(), giantFeatureKey(t, g))
	if p == nil || !p.GetChoosable() || !p.GetAvailable() || p.GetFlat() != 3 || p.GetDiceCount() != 0 || p.GetManeuver() || p.GetResourceLeft() != 2 {
		t.Fatalf("the offered extra = %v, want a choosable flat 3 with 2 uses, not a maneuver", p)
	}
	g.h.roller.queue(5) // the weapon die alone: the extra rolls nothing
	dmg := g.mustDamage(t, g.ana, e, first.GetId(), marking(p.GetKey())).GetPendingDamage()
	var extra *playv1.DamagePartRoll
	for _, r := range dmg.GetPartRolls() {
		if r.GetPartKey() == p.GetKey() {
			extra = r
		}
	}
	if extra == nil || !extra.GetCounted() || extra.GetFlat() != 3 {
		t.Fatalf("the part rolls = %v, want the extra counted as a flat 3", dmg.GetPartRolls())
	}
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
	second := g.hit(t, e) // the second attack of Extra Attack, the same turn
	if q := partOf(second.GetParts(), p.GetKey()); q == nil || q.GetAvailable() || q.GetReasonPt() == "" {
		t.Fatalf("the extra the same turn = %v, want it listed and off with a reason", q)
	}
	if _, err := g.damage(t, g.ana, e, second.GetId(), marking(p.GetKey())); err == nil {
		t.Error("the extra twice in one turn is accepted")
	}
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left after the refusal = %d, want 1", got)
	}
}

// The master's undo gives the use and the turn's mark back.
func TestExtraDamageUndoGivesTheUseAndTheMarkBack(t *testing.T) {
	t.Parallel()
	g := newGiantTable(t, rules.ExtraDamageAnyTarget)
	e := g.passTo(t, g.castersFight(t, 1), "Pensantus")
	first := g.hit(t, e)
	key := giantFeatureKey(t, g)
	g.h.roller.queue(5)
	g.mustDamage(t, g.ana, e, first.GetId(), marking(key))
	if got := g.usesLeft(t); got != 1 {
		t.Fatalf("uses left = %d, want 1", got)
	}
	g.undoLast(t, e) // the damage roll
	if got := g.usesLeft(t); got != 2 {
		t.Errorf("uses left after the undo = %d, want 2", got)
	}
	again := partOf(first.GetParts(), key)
	if again == nil || !again.GetAvailable() {
		t.Fatalf("the extra after the undo = %v, want it available again", again)
	}
	g.h.roller.queue(5)
	if _, err := g.damage(t, g.ana, e, first.GetId(), marking(key)); err != nil {
		t.Errorf("rolling the damage again with the extra: %v", err)
	}
}

// Against a creature that is not larger the extra is listed off, and refused if marked.
func TestExtraDamageOnALargerTargetNeedsALargerTarget(t *testing.T) {
	t.Parallel()
	g := newGiantTable(t, rules.ExtraDamageLargerTarget)
	e := g.passTo(t, g.castersFight(t, 1), "Pensantus")
	pending := g.hit(t, e) // the Capitão Goblin is not larger than a human
	key := giantFeatureKey(t, g)
	if p := partOf(pending.GetParts(), key); p == nil || p.GetAvailable() || p.GetReasonPt() == "" {
		t.Fatalf("the extra on a creature that is not larger = %v, want it listed and off with a reason", p)
	}
	if _, err := g.damage(t, g.ana, e, pending.GetId(), marking(key)); err == nil {
		t.Error("the extra on a creature that is not larger is accepted")
	}
	if got := g.usesLeft(t); got != 2 {
		t.Errorf("uses left = %d, want 2", got)
	}
}

// giantFeatureKey is the key the table gave the feature: the extra's part is named by it.
func giantFeatureKey(t *testing.T, g *giantTable) string {
	t.Helper()
	for _, f := range g.pens.GetDerived().GetFeatures() {
		if f.GetName() == "Golpe de Gigante" {
			return f.GetKey()
		}
	}
	t.Fatalf("Pensantus has no Golpe de Gigante among %v", g.pens.GetDerived().GetFeatures())
	return ""
}
