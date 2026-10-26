package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The spell_modifier effect of the table's features on a cast: the options a feature offers for the spells it fits
// (swap the damage type, add a fixed damage, add dice for a higher slot). The names are invented ("a table
// subclass whose feature adds 2 cold damage to an evocation spell, 2 uses per long rest").

const (
	emberFeature = "Brasa Contida"
	emberUses    = "brasas"
	fireType     = "damage-type:fire"
	coldType     = "damage-type:cold"
)

func emberResource() *rulesv1.TableEffect {
	return &rulesv1.TableEffect{Type: "resource", Resource: emberUses, Max: "2", Recharge: "long_rest"}
}

// ---- pure: the facts of a spell, what applies to it, and what the options do to its damage ----

func burningFacts() link.Spell {
	return link.Spell{
		Key: burningHands, Level: 1, School: "school:evocation",
		Damages: []link.Dice{{Count: 3, Sides: 6, DamageType: fireType}},
	}
}

func TestSpellOptionsFitTheSpellsTheyNameAndOnlyWhenTheyDealDamage(t *testing.T) {
	t.Parallel()
	sp := burningFacts()
	f := factsOfSpell(sp)
	for _, tc := range []struct {
		appliesTo string
		fits      bool
	}{
		{rules.SpellAppliesDamageSpell, true},
		{rules.SpellAppliesCantrip, false},
		{"school:evocation", true},
		{"school:necromancy", false},
		{fireType, true},
		{coldType, false},
	} {
		if got := f.fits(link.SpellModifier{AppliesTo: tc.appliesTo}); got != tc.fits {
			t.Errorf("applies_to %q: fits = %v, want %v", tc.appliesTo, got, tc.fits)
		}
	}
	cantrip := factsOfSpell(link.Spell{Level: 0, Damages: []link.Dice{{Count: 1, Sides: 10, DamageType: fireType}}})
	if !cantrip.fits(link.SpellModifier{AppliesTo: rules.SpellAppliesCantrip}) {
		t.Error("a cantrip that deals damage does not fit cantrip")
	}
	if factsOfSpell(link.Spell{Level: 1, School: "school:evocation"}).fits(link.SpellModifier{AppliesTo: "school:evocation"}) {
		t.Error("a spell with no damage fits an option")
	}
}

func TestATypeSwapOffersTheOtherTypesOfItsList(t *testing.T) {
	t.Parallel()
	swap := link.SpellModifier{Change: rules.SpellChangeDamageTypeSwap, From: []string{fireType, coldType, "damage-type:lightning"}}
	got := factsOfSpell(burningFacts()).swapTypes(swap)
	if len(got) != 2 || got[0] != coldType || got[1] != "damage-type:lightning" {
		t.Errorf("swap types = %v, want cold and lightning", got)
	}
	if got := factsOfSpell(link.Spell{Damages: []link.Dice{{DamageType: "damage-type:acid"}}}).swapTypes(swap); got != nil {
		t.Errorf("a spell of a type outside the list offers %v, want none", got)
	}
}

func TestOptionsChangeTheDamageOfTheSpell(t *testing.T) {
	t.Parallel()
	swap := link.SpellModifier{Key: "swap", AppliesTo: "school:evocation", Change: rules.SpellChangeDamageTypeSwap, From: []string{fireType, coldType}}
	bonus := link.SpellModifier{Key: "bonus", AppliesTo: "damage_spell", Change: rules.SpellChangeBonusDamage, Bonus: 2}
	cold := link.SpellModifier{Key: "cold", AppliesTo: fireType, Change: rules.SpellChangeBonusDamage, Bonus: 3, BonusType: coldType, Resource: emberUses}
	dice := link.SpellModifier{Key: "dice", AppliesTo: "school:evocation", Change: rules.SpellChangeExtraDice, ExtraDice: 1}
	mods := []link.SpellModifier{swap, bonus, cold, dice}
	damages := func(sp link.Spell) []link.Dice { return sp.Damages }
	tests := []struct {
		name  string
		ticks []tableTick
		slot  int
		want  []link.Dice
	}{
		{"only the always-on bonus", nil, 1, []link.Dice{{Count: 3, Sides: 6, Bonus: 2, DamageType: fireType}}},
		{"the swap turns the type, the bonus follows it", []tableTick{{mod: swap, pick: coldType}}, 1, []link.Dice{{Count: 3, Sides: 6, Bonus: 2, DamageType: coldType}}},
		{
			"a ticked bonus of another type is a part of its own",
			[]tableTick{{mod: cold}},
			1,
			[]link.Dice{{Count: 3, Sides: 6, Bonus: 2, DamageType: fireType}, {Bonus: 3, DamageType: coldType}},
		},
		{"a higher slot adds the dice, the lower one does not", nil, 3, []link.Dice{{Count: 3 + 2, Sides: 6, Bonus: 2, DamageType: fireType}}},
	}
	for _, tc := range tests {
		got := damages(applyTableOptions(burningFacts(), mods, tc.ticks, tc.slot))
		if len(got) != len(tc.want) {
			t.Errorf("%s: damages = %+v, want %+v", tc.name, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: damages = %+v, want %+v", tc.name, got, tc.want)
			}
		}
	}
	// The spell the options were applied to is left as it was.
	original := burningFacts()
	applyTableOptions(original, mods, []tableTick{{mod: swap, pick: coldType}}, 3)
	if original.Damages[0].DamageType != fireType || original.Damages[0].Bonus != 0 || original.Damages[0].Count != 3 {
		t.Errorf("the options changed the spell's own damages: %+v", original.Damages)
	}
}

func TestAnExtraDiceOptionOfADamageTypeGoesToThatTypesDice(t *testing.T) {
	t.Parallel()
	sp := link.Spell{Level: 1, Damages: []link.Dice{{Count: 2, Sides: 6, DamageType: coldType}, {Count: 1, Sides: 4, DamageType: fireType}}}
	m := link.SpellModifier{AppliesTo: fireType, Change: rules.SpellChangeExtraDice, ExtraDice: 2}
	got := applyTableOptions(sp, []link.SpellModifier{m}, nil, 3).Damages
	if got[0].Count != 2 || got[1].Count != 1+2*2 {
		t.Errorf("damages = %+v, want the extra dice on the fire part only", got)
	}
	if got := applyTableOptions(link.Spell{Level: 0, Damages: sp.Damages}, []link.SpellModifier{m}, nil, 0).Damages; got[1].Count != 1 {
		t.Errorf("a cantrip got extra dice: %+v", got)
	}
}

func TestTickedOptionsAreCheckedAgainstTheSpell(t *testing.T) {
	t.Parallel()
	swap := link.SpellModifier{Key: "swap", AppliesTo: "damage_spell", Change: rules.SpellChangeDamageTypeSwap, From: []string{fireType, coldType}}
	paid := link.SpellModifier{Key: "paid", AppliesTo: "damage_spell", Change: rules.SpellChangeBonusDamage, Bonus: 1, Resource: emberUses}
	free := link.SpellModifier{Key: "free", AppliesTo: "damage_spell", Change: rules.SpellChangeBonusDamage, Bonus: 1}
	cantrips := link.SpellModifier{Key: "cantrips", AppliesTo: "cantrip", Change: rules.SpellChangeBonusDamage, Bonus: 1, Resource: emberUses}
	mods := []link.SpellModifier{swap, paid, free, cantrips}
	pick := func(key, typ string) *playv1.SpellOptionChoice {
		return &playv1.SpellOptionChoice{Key: key, DamageTypeKey: typ}
	}
	for _, tc := range []struct {
		name   string
		chosen []*playv1.SpellOptionChoice
		ok     bool
	}{
		{"nothing", nil, true},
		{"a swap to another type of its list", []*playv1.SpellOptionChoice{pick("swap", coldType)}, true},
		{"a swap to the type the spell already has", []*playv1.SpellOptionChoice{pick("swap", fireType)}, false},
		{"a swap with no type", []*playv1.SpellOptionChoice{pick("swap", "")}, false},
		{"a swap to a type outside its list", []*playv1.SpellOptionChoice{pick("swap", "damage-type:acid")}, false},
		{"a paid bonus", []*playv1.SpellOptionChoice{pick("paid", "")}, true},
		{"a paid bonus with a type", []*playv1.SpellOptionChoice{pick("paid", coldType)}, false},
		{"an always-on option", []*playv1.SpellOptionChoice{pick("free", "")}, false},
		{"an option the spell does not fit", []*playv1.SpellOptionChoice{pick("cantrips", "")}, false},
		{"an option nobody offers", []*playv1.SpellOptionChoice{pick("ghost", "")}, false},
		{"an option twice", []*playv1.SpellOptionChoice{pick("paid", ""), pick("paid", "")}, false},
	} {
		_, err := tickedOptions(tc.chosen, mods, burningFacts())
		if (err == nil) != tc.ok || (err != nil && connect.CodeOf(err) != connect.CodeInvalidArgument) {
			t.Errorf("%s: error = %v, want ok %v (invalid_argument otherwise)", tc.name, err, tc.ok)
		}
	}
}

func TestTheCastSheetListsTheOptionsThatFitWithTheirUses(t *testing.T) {
	t.Parallel()
	mods := []link.SpellModifier{
		{Key: "swap", NamePT: "Troca", AppliesTo: "damage_spell", Change: rules.SpellChangeDamageTypeSwap, From: []string{fireType, coldType}},
		{Key: "paid", NamePT: "Paga", AppliesTo: "school:evocation", Change: rules.SpellChangeBonusDamage, Bonus: 1, Resource: emberUses},
		{Key: "free", NamePT: "Livre", AppliesTo: "damage_spell", Change: rules.SpellChangeExtraDice, ExtraDice: 1},
		{Key: "cantrips", NamePT: "Truques", AppliesTo: "cantrip", Change: rules.SpellChangeBonusDamage, Bonus: 1},
		{Key: "lonely", NamePT: "Sozinha", AppliesTo: "damage_spell", Change: rules.SpellChangeDamageTypeSwap, From: []string{fireType, "damage-type:acid"}},
	}
	vitals := &playv1.CharacterVitals{Resources: []*playv1.ResourceUsage{{Key: emberUses, Total: 2, Used: 2}}}
	got := tableOptionsFor(factsOfSpell(burningFacts()), mods, vitals)
	byKey := map[string]*rulesv1.SpellTableOption{}
	for _, o := range got {
		byKey[o.GetKey()] = o
	}
	if len(got) != 4 || byKey["cantrips"] != nil {
		t.Fatalf("options = %v, want the four that fit a leveled spell of damage", got)
	}
	if o := byKey["swap"]; o.GetAlwaysOn() || len(o.GetDamageTypeKeys()) != 1 || o.GetDamageTypeKeys()[0] != coldType || !o.GetAllowed() {
		t.Errorf("the swap = %v, want a free choice of cold", o)
	}
	if o := byKey["free"]; !o.GetAlwaysOn() {
		t.Errorf("the extra dice with no resource = %v, want always on", o)
	}
	if o := byKey["paid"]; o.GetAlwaysOn() || o.GetAllowed() || o.GetUsesLeft() != 0 || o.GetDisabledReasonPt() == "" || o.GetResource() != emberUses {
		t.Errorf("the paid bonus with no use left = %v, want it off with a reason", o)
	}
}

// ---- end to end (these need the database) ----

type optionTable struct{ *armed }

// newOptionTable makes a table subclass of the Wizard whose level 2 feature has the effects, and Pensantus, a Wizard 3
// who has it (Burning Hands, 3d6 of fire, and two levels of slots), with Toren and Brisa as in newCasters.
func newOptionTable(t *testing.T, effects ...*rulesv1.TableEffect) *optionTable {
	t.Helper()
	g := &optionTable{}
	g.armed = newArmedWith(t, func(a *armed) {
		sub := &rulesv1.TableSubclass{
			NamePt: "Tecelão de Brasas", ClassKey: "class:wizard", DescPt: []string{"Brasas."},
			Levels: []*rulesv1.TableSubclassLevel{{Level: 2, Features: []*rulesv1.TableFeature{{NamePt: emberFeature, DescPt: []string{"Opção de magia."}, Effects: effects}}}},
		}
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSubclass{TableSubclass: sub},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry() error = %v", err)
		}
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, RaceKey: "race:gnome",
			Classes: []*charactersv1.ClassLevel{{
				ClassKey: "class:wizard", Level: 3, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: res.Msg.GetEntry().GetKey()},
			}},
			CantripKeys: []string{fireBolt}, KnownSpellKeys: []string{magicMissileSpell, shieldSpell, sleepSpell, burningHands}, PreparedSpellKeys: []string{magicMissileSpell, shieldSpell, sleepSpell, burningHands},
		}}}
		made, err := a.ana.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Pensantus", Sheet: sheet,
		}))
		if err != nil {
			t.Fatalf("CreateCharacter(Pensantus) error = %v", err)
		}
		a.pens = made.Msg.GetCharacter()
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{maceKey}, []string{sacredFlame}, nil,
			[]string{cureWounds, healingWord, guidingBolt})
	})
	return g
}

// optionFight starts the casters' combat with the Goblin alone to the north of Pensantus, so a cone reaches it and nobody else.
func (g *optionTable) optionFight(t *testing.T) *playv1.Encounter {
	t.Helper()
	e := g.castersFight(t, 1)
	g.stand(t, map[string][2]int32{"Goblin": {5, 3}, "Capitão Goblin": {9, 9}})
	return e
}

func (g *optionTable) featureKey(t *testing.T) string {
	t.Helper()
	for _, f := range g.pens.GetDerived().GetFeatures() {
		if f.GetName() == emberFeature {
			return f.GetKey()
		}
	}
	t.Fatalf("Pensantus has no %s among %v", emberFeature, g.pens.GetDerived().GetFeatures())
	return ""
}

func (g *optionTable) usesLeft(t *testing.T) int32 {
	t.Helper()
	left, _ := poolOf(g.vitals(t, g.pens), emberUses)
	return left
}

// burn casts Burning Hands at the slot on the Goblin with the options ticked, and answers the pending damages.
func (g *optionTable) burn(t *testing.T, e *playv1.Encounter, slot int32, ticked ...*playv1.SpellOptionChoice) (*playv1.CastSpellResponse, error) {
	t.Helper()
	g.h.roller.queue(5) // the Goblin's save
	return g.cast(t, g.ana, e, "Pensantus", burningHands, slotOfLevel(slot), g.at(t, "Goblin"), func(r *playv1.CastSpellRequest) { r.SpellOptions = ticked })
}

func pendingOf(cast *playv1.CastSpellResponse, typ string) *playv1.PendingDamage {
	for _, p := range cast.GetCast().GetPendingDamages() {
		if p.GetDamageTypeKey() == typ {
			return p
		}
	}
	return nil
}

func TestAnAlwaysOnBonusDamageIsAddedToTheSpellsDamageWithoutTicking(t *testing.T) {
	t.Parallel()
	g := newOptionTable(t, &rulesv1.TableEffect{Type: "spell_modifier", AppliesTo: "school:evocation", SpellChange: rules.SpellChangeBonusDamage, Value: "2"})
	e := g.optionFight(t)
	cast, err := g.burn(t, e, 1)
	if err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	if p := pendingOf(cast, fireType); p == nil || len(cast.GetCast().GetPendingDamages()) != 1 || p.GetDiceCount() != 3 || p.GetDiceSides() != 6 || p.GetBonus() != 2 {
		t.Fatalf("pending damages = %v, want 3d6 + 2 of fire", cast.GetCast().GetPendingDamages())
	}
	// A spell the option does not fit is left alone: Magic Missile is evocation too, so it gets the bonus; Sleep rolls hit points.
	opts := g.mustOptions(t, g.ana, g.get(t, g.ana), "Pensantus")
	if o := spellOption(opts, burningHands); len(o.GetTableOptions()) != 1 || !o.GetTableOptions()[0].GetAlwaysOn() || o.GetTableOptions()[0].GetKey() != g.featureKey(t) {
		t.Errorf("Burning Hands's table options = %v, want the always-on bonus", o.GetTableOptions())
	}
	if o := spellOption(opts, sleepSpell); len(o.GetTableOptions()) != 0 {
		t.Errorf("Sleep's table options = %v, want none: it deals no damage", o.GetTableOptions())
	}
}

func TestATickedBonusSpendsAUseAndAddsAPartOfItsOwnType(t *testing.T) {
	t.Parallel()
	g := newOptionTable(t, emberResource(), &rulesv1.TableEffect{
		Type: "spell_modifier", AppliesTo: "damage_spell", SpellChange: rules.SpellChangeBonusDamage, Value: "3", DamageType: coldType, Resource: emberUses,
	})
	e := g.optionFight(t)
	key := g.featureKey(t)
	// Not ticked: nothing changes and no use is spent.
	plain, err := g.burn(t, e, 1)
	if err != nil || len(plain.GetCast().GetPendingDamages()) != 1 || g.usesLeft(t) != 2 {
		t.Fatalf("an untricked cast: %d pending damages, %d uses left, error %v; want one part and 2 uses", len(plain.GetCast().GetPendingDamages()), g.usesLeft(t), err)
	}
	g.undoes(t, "a cast with the option ticked", func() {
		cast, err := g.burn(t, e, 1, &playv1.SpellOptionChoice{Key: key})
		if err != nil {
			t.Fatalf("CastSpell(ticked) error = %v", err)
		}
		fire, cold := pendingOf(cast, fireType), pendingOf(cast, coldType)
		if fire == nil || cold == nil || fire.GetBonus() != 0 || cold.GetBonus() != 3 || cold.GetDiceCount() != 0 {
			t.Fatalf("pending damages = %v, want 3d6 of fire and a flat 3 of cold", cast.GetCast().GetPendingDamages())
		}
		if got := g.usesLeft(t); got != 1 {
			t.Errorf("uses left = %d, want 1", got)
		}
	})
	if got := g.usesLeft(t); got != 2 {
		t.Errorf("uses left after the undo = %d, want 2", got)
	}
}

func TestATickedOptionWithoutUsesLeftIsRefusedAndSpendsNothing(t *testing.T) {
	t.Parallel()
	g := newOptionTable(t, emberResource(), &rulesv1.TableEffect{
		Type: "spell_modifier", AppliesTo: "damage_spell", SpellChange: rules.SpellChangeBonusDamage, Value: "1", Resource: emberUses,
	})
	e := g.optionFight(t)
	key := g.featureKey(t)
	g.correct(t, g.pens, func(r *playv1.AdjustCharacterVitalsRequest) {
		r.ResourcesUsed = []*playv1.ResourceUsed{{Key: emberUses, Used: 2}}
	})
	_, err := g.burn(t, e, 1, &playv1.SpellOptionChoice{Key: key})
	wantBlockedBy(t, "a cast with an option with no use left", err, blockedNoUses)
	if got := usedSlots(g.vitals(t, g.pens), 1); got != 0 {
		t.Errorf("a refused cast spent a slot: %d used", got)
	}
	opts := g.mustOptions(t, g.ana, e, "Pensantus")
	if o := spellOption(opts, burningHands).GetTableOptions(); len(o) != 1 || o[0].GetAllowed() || o[0].GetUsesLeft() != 0 {
		t.Errorf("table options = %v, want the option off with no uses", o)
	}
}

func TestATypeSwapIsPickedAtTheCastAmongTheTypesTheSpellCanTake(t *testing.T) {
	t.Parallel()
	g := newOptionTable(t, &rulesv1.TableEffect{
		Type: "spell_modifier", AppliesTo: "school:evocation", SpellChange: rules.SpellChangeDamageTypeSwap, From: []string{fireType, coldType, "damage-type:lightning"},
	})
	e := g.optionFight(t)
	key := g.featureKey(t)
	opts := g.mustOptions(t, g.ana, e, "Pensantus")
	o := spellOption(opts, burningHands).GetTableOptions()
	if len(o) != 1 || o[0].GetAlwaysOn() || len(o[0].GetDamageTypeKeys()) != 2 {
		t.Fatalf("table options = %v, want a swap offering cold and lightning", o)
	}
	for _, bad := range []string{"", fireType, "damage-type:acid"} {
		if _, err := g.burn(t, e, 1, &playv1.SpellOptionChoice{Key: key, DamageTypeKey: bad}); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("a swap to %q: %v, want invalid_argument", bad, err)
		}
	}
	if got := usedSlots(g.vitals(t, g.pens), 1); got != 0 {
		t.Errorf("a refused swap spent a slot: %d used", got)
	}
	cast, err := g.burn(t, e, 1, &playv1.SpellOptionChoice{Key: key, DamageTypeKey: coldType})
	if err != nil {
		t.Fatalf("CastSpell(swap to cold) error = %v", err)
	}
	if p := pendingOf(cast, coldType); p == nil || pendingOf(cast, fireType) != nil || p.GetDiceCount() != 3 {
		t.Errorf("pending damages = %v, want 3d6 of cold and no fire", cast.GetCast().GetPendingDamages())
	}
}

func TestExtraDicePerSlotLevelOverTheSpellsOwnGoToTheDamage(t *testing.T) {
	t.Parallel()
	g := newOptionTable(t, &rulesv1.TableEffect{Type: "spell_modifier", AppliesTo: "damage_spell", SpellChange: rules.SpellChangeExtraDice, Count: 1})
	e := g.optionFight(t)
	// Burning Hands is 3d6 at the 1st level and 4d6 at the 2nd: the option adds a die for the level over the spell's own.
	cast, err := g.burn(t, e, 2)
	if err != nil {
		t.Fatalf("CastSpell(at the 2nd level) error = %v", err)
	}
	if p := pendingOf(cast, fireType); p == nil || p.GetDiceCount() != 5 {
		t.Errorf("pending damages = %v, want 5d6 (4d6 and one more)", cast.GetCast().GetPendingDamages())
	}
}
