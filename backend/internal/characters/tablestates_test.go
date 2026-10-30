package characters

import (
	"slices"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The table's temporary states, weapon rules and pseudo-weapons through the table-content API (MR-025): the
// server accepts them, keeps every field, shows them on the menu, and refuses what is wrong at the field the
// editor draws. The rules package pins the checks; this proves the API carries them. Every name is invented.

// stanceSubclass is a table subclass of the SRD Fighter whose level 3 feature has the effects.
func stanceSubclass(effects ...*rulesv1.TableEffect) *rulesv1.TableSubclass {
	return &rulesv1.TableSubclass{
		NamePt: "Guardião de Teste", ClassKey: "class:fighter", DescPt: []string{"Uma postura firme."},
		Levels: []*rulesv1.TableSubclassLevel{{Level: 3, Features: []*rulesv1.TableFeature{{NamePt: "Postura Firme", DescPt: []string{"Fique firme."}, Effects: effects}}}},
	}
}

func validStanceEffects() []*rulesv1.TableEffect {
	return []*rulesv1.TableEffect{
		{Type: "resource", Resource: "posturas", Max: "1", Recharge: "short_rest"},
		{Type: "state", StateName: "postura", DurationRounds: "10", EndsOn: []string{"dismiss", "incapacitated"}, Resource: "posturas", Economy: "bonus_action"},
		{Type: "modifier", Target: "ac", Mode: "add", Value: "2", When: `active("postura")`},
		{Type: "resistance", Except: "damage-type:psychic", While: "postura"},
		{Type: "roll_mode", Roll: "advantage", Targets: []string{"save.dex"}, While: "postura"},
		{Type: "extra_damage", Applies: "any_target", Value: "2", While: "postura"},
		{Type: "weapon_rule", Weapon: "named", WeaponName: "Espada longa", AttackAbility: "cha", DamageAbility: "cha", AttackBonus: 1, DamageBonus: -1, ReachFt: 10, While: "postura"},
		{Type: "pseudo_weapon", AttackName: "Raio de Teste", Weapon: "ranged", DamageDice: "1d6", DamageType: "damage-type:force", Ability: "int", RangeFt: 60, While: "postura"},
		{Type: "pseudo_weapon", AttackName: "Lâmina de Teste", Weapon: "melee", DamageDice: "1d8", DamageType: "damage-type:radiant", Ability: "wis", ReachFt: 10},
	}
}

func TestTableContentKeepsEveryFieldOfTheStateAndWeaponEffects(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Samuel")
	campaign := h.newCampaign(master, "Mirathel")

	want := validStanceEffects()
	created := master.addEntry(t, campaign, stanceSubclass(want...))
	got := created.GetTableSubclass().GetLevels()[0].GetFeatures()[0].GetEffects()
	if len(got) != len(want) {
		t.Fatalf("%d effects came back, want %d", len(got), len(want))
	}
	for i := range want {
		if !proto.Equal(got[i], want[i]) {
			t.Errorf("effect %d = %v, want %v", i, got[i], want[i])
		}
	}
	// Read back from the list, the same.
	var listed *rulesv1.TableEntry
	for _, e := range master.entries(t, campaign) {
		if e.GetKey() == created.GetKey() {
			listed = e
		}
	}
	if listed == nil {
		t.Fatal("the entry is not in the list")
	}
	for i, e := range listed.GetTableSubclass().GetLevels()[0].GetFeatures()[0].GetEffects() {
		if !proto.Equal(e, want[i]) {
			t.Errorf("listed effect %d = %v, want %v", i, e, want[i])
		}
	}
}

func TestTableContentRefusesWrongStatesAndWeaponsAtTheirFields(t *testing.T) {
	t.Parallel()
	const at = "table_subclass.levels[0].features[0].effects["
	h := newHarness(t)
	master := h.newUser("Samuel")
	campaign := h.newCampaign(master, "Mirathel")

	edit := func(i int, change func(e *rulesv1.TableEffect)) *rulesv1.TableSubclass {
		effects := validStanceEffects()
		change(effects[i])
		return stanceSubclass(effects...)
	}
	cases := []struct {
		name          string
		body          *rulesv1.TableSubclass
		field, reason string
	}{
		{"a state name", edit(1, func(e *rulesv1.TableEffect) { e.StateName = "Minha Postura" }), at + "1].state_name", "bad_value"},
		{"a duration formula", edit(1, func(e *rulesv1.TableEffect) { e.DurationRounds = "level(" }), at + "1].duration_rounds", "bad_formula"},
		{"a way to end", edit(1, func(e *rulesv1.TableEffect) { e.EndsOn = []string{"never"} }), at + "1].ends_on", "bad_value"},
		{"an economy", edit(1, func(e *rulesv1.TableEffect) { e.Economy = "reaction" }), at + "1].economy", "bad_value"},
		{"a resource nothing defines", edit(1, func(e *rulesv1.TableEffect) { e.Resource = "fantasma" }), at + "1].resource", "dangling_reference"},
		{"a condition that reads a missing state", edit(2, func(e *rulesv1.TableEffect) { e.When = `active("fantasma")` }), at + "2].when", "dangling_reference"},
		{"a gate on a missing state", edit(3, func(e *rulesv1.TableEffect) { e.While = "fantasma" }), at + "3].while", "dangling_reference"},
		{"a gate on a roll mode", edit(4, func(e *rulesv1.TableEffect) { e.While = "fantasma" }), at + "4].while", "dangling_reference"},
		{"a gate on an extra damage", edit(5, func(e *rulesv1.TableEffect) { e.While = "rage" }), at + "5].while", "dangling_reference"},
		{"a weapon filter", edit(6, func(e *rulesv1.TableEffect) { e.Weapon = "heavy" }), at + "6].weapon", "bad_value"},
		{"a named weapon with no name", edit(6, func(e *rulesv1.TableEffect) { e.WeaponName = "" }), at + "6].weapon_name", "bad_value"},
		{"an ability", edit(6, func(e *rulesv1.TableEffect) { e.AttackAbility = "luck" }), at + "6].attack_ability", "bad_value"},
		{"a bonus", edit(6, func(e *rulesv1.TableEffect) { e.AttackBonus = 99 }), at + "6].attack_bonus", "limit"},
		{"a reach", edit(6, func(e *rulesv1.TableEffect) { e.ReachFt = 7 }), at + "6].reach_ft", "limit"},
		{"damage dice", edit(7, func(e *rulesv1.TableEffect) { e.DamageDice = "1d7" }), at + "7].damage_dice", "bad_value"},
		{"a damage type", edit(7, func(e *rulesv1.TableEffect) { e.DamageType = "damage-type:luck" }), at + "7].damage_type", "dangling_reference"},
		{"a range", edit(7, func(e *rulesv1.TableEffect) { e.RangeFt = 0 }), at + "7].range_ft", "limit"},
		{"a field of another type", edit(7, func(e *rulesv1.TableEffect) { e.AttackBonus = 1 }), at + "7].attack_bonus", "bad_value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(campaign, tc.body)))
			v := violationsOfErr(t, err)
			if !slices.ContainsFunc(v, func(x *rulesv1.TableContentViolation) bool {
				return x.GetField() == tc.field && x.GetReason() == tc.reason
			}) {
				t.Errorf("violations = %v, want one at %s with reason %s", v, tc.field, tc.reason)
			}
		})
	}
	if n := len(master.entries(t, campaign)); n != 0 {
		t.Errorf("%d entries after the refusals, want none", n)
	}
}

func TestTheEffectMenuOffersTheStateAndWeaponTypes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Samuel")
	campaign := h.newCampaign(master, "Mirathel")
	res, err := master.table.GetEffectMenu(t.Context(), connect.NewRequest(&rulesv1.GetEffectMenuRequest{CampaignId: campaign}))
	if err != nil {
		t.Fatalf("GetEffectMenu() error = %v", err)
	}
	types := map[string]*rulesv1.EffectMenuType{}
	for _, mt := range res.Msg.GetTypes() {
		types[mt.GetType()] = mt
	}
	for _, name := range []string{"state", "weapon_rule", "pseudo_weapon"} {
		mt := types[name]
		if mt == nil || mt.GetNamePt() == "" || mt.GetHintPt() == "" || len(mt.GetFields()) == 0 {
			t.Errorf("type %s = %v, want it on the menu with its words and fields", name, mt)
		}
	}
	for _, name := range []string{"roll_mode", "extra_damage", "resistance", "weapon_rule", "pseudo_weapon"} {
		var while *rulesv1.EffectMenuField
		for _, f := range types[name].GetFields() {
			if f.GetName() == "while" {
				while = f
			}
		}
		if while == nil || while.GetKind() != "state_ref" || while.GetRequired() {
			t.Errorf("%s: while = %v, want an optional state reference", name, while)
		}
	}
	lists := map[string]bool{}
	for _, l := range res.Msg.GetLists() {
		lists[l.GetName()] = len(l.GetValues()) > 0
	}
	for _, name := range []string{"state_ends", "state_economies", "weapon_filters", "weapon_kinds"} {
		if !lists[name] {
			t.Errorf("list %s is missing or empty", name)
		}
	}
	if !slices.ContainsFunc(res.Msg.GetHelpers(), func(h *rulesv1.FormulaHelper) bool { return h.GetCall() == `active("<estado>")` }) {
		t.Errorf("helpers = %v, want active(\"<estado>\")", res.Msg.GetHelpers())
	}
}
