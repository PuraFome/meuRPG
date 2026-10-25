package play

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// A table race with +2/+1 or +1/+1/+1 to place and Small or Medium to pick, whose trait halves the
// damage of nonmagical weapons, and table feats with immunity, vulnerability and a roll mode against a
// condition, end to end (MR-025): invented content, the engine reads only the effects.

// frostTable is a theatre fight with Ragna, a character of the table's race "Gelídeo", next to the goblin.
type frostTable struct {
	*armed
	race, thickSkin, glassSkin, stoneSoul, coldBlood string // the entries' keys
}

// newFrostTable makes the race and the feats, and the party: Ragna (the race, Small, +2 Strength and +1
// Dexterity), Pensantus and Brisa.
func newFrostTable(t *testing.T) (*frostTable, *playv1.Encounter) {
	t.Helper()
	f := &frostTable{}
	f.armed = newArmedWith(t, func(a *armed) {
		create := func(body *rulesv1.CreateTableEntryRequest) string {
			t.Helper()
			body.CampaignId = a.campaignID
			res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(body))
			if err != nil {
				t.Fatalf("CreateTableEntry() error = %v", err)
			}
			return res.Msg.GetEntry().GetKey()
		}
		feat := func(name string, effects ...*rulesv1.TableEffect) string {
			return create(&rulesv1.CreateTableEntryRequest{Body: &rulesv1.CreateTableEntryRequest_TableFeat{TableFeat: &rulesv1.TableFeat{
				NamePt: name, DescPt: []string{"Um talento da mesa."}, Effects: effects,
			}}})
		}
		f.race = create(&rulesv1.CreateTableEntryRequest{Body: &rulesv1.CreateTableEntryRequest_TableRace{TableRace: &rulesv1.TableRace{
			NamePt: "Gelídeo", Size: "Medium", SpeedFt: 30, Languages: []string{"language:common"},
			ChoiceBonusSets: []*rulesv1.BonusSet{{Bonuses: []int32{2, 1}}, {Bonuses: []int32{1, 1, 1}}},
			Sizes:           []string{"Small", "Medium"},
			Traits: []*rulesv1.TableFeature{{NamePt: "Pele de gelo", DescPt: []string{"Armas comuns pouco ferem."}, Effects: []*rulesv1.TableEffect{
				{Type: "resistance", DamageTypes: []string{"nonmagical_bps"}},
			}}},
		}}})
		f.glassSkin = feat("Pele de vidro", &rulesv1.TableEffect{Type: "resistance", Kind: "vulnerable", DamageTypes: []string{"damage-type:slashing"}})
		f.stoneSoul = feat("Alma de pedra", &rulesv1.TableEffect{Type: "resistance", Kind: "immune", DamageTypes: []string{"damage-type:slashing"}})
		f.coldBlood = feat("Sangue frio",
			&rulesv1.TableEffect{Type: "resistance", Kind: "immune", DamageTypes: []string{"damage-type:poison"}},
			&rulesv1.TableEffect{Type: "condition_immunity", Conditions: []string{"condition:poisoned"}},
			&rulesv1.TableEffect{Type: "roll_mode", Roll: "advantage", Targets: []string{"save.wis"}, AgainstCondition: "condition:frightened"},
		)
		a.toren = a.caio.heroSheet(t, a.campaignID, "Ragna", &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 14, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8},
			RaceKey:    f.race, Size: "Small", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 1}},
			ExtraAbilityBonuses: &rulesv1.AbilityScores{Strength: 2, Dexterity: 1},
			WeaponKeys:          []string{greataxe},
		})
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{rapier}, nil)
	})
	e := f.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: f.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Ragna": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Ragna": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {8, 8}},
	})
	return f, e
}

// heroSheet creates a player's character from a full sheet.
func (u *user) heroSheet(t *testing.T, campaignID, name string, full *charactersv1.FullSheet) *charactersv1.Character {
	t.Helper()
	res, err := u.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: name,
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: full}},
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

// edit is the master changing Ragna's sheet, which he may do at any time (MR-025).
func (f *frostTable) edit(t *testing.T, change func(full *charactersv1.FullSheet)) *charactersv1.Character {
	t.Helper()
	cur := f.master.character(t, f.toren)
	sheet := cur.GetSheet()
	change(sheet.GetFull())
	res, err := f.master.characters.UpdateCharacter(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterRequest{
		CampaignId: f.campaignID, CharacterId: cur.GetId(), Revision: cur.GetRevision(), Name: cur.GetName(), Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("UpdateCharacter(Ragna) error = %v", err)
	}
	f.toren = res.Msg.GetCharacter()
	return f.toren
}

func issueCodes(c *charactersv1.Character) []string {
	var out []string
	for _, is := range c.GetDerived().GetIssues() {
		out = append(out, is.GetCode())
	}
	return out
}

// hitRagna has the goblin hit Ragna with its sword: 1d6 + 2, the d6 being the one given, and returns the
// damage rolled and waiting for the master (the steps that change it, and the pending id).
func (f *frostTable) hitRagna(t *testing.T, e *playv1.Encounter, d6 int) *playv1.PendingDamage {
	t.Helper()
	f.h.roller.queue(15, d6)
	hit := f.mustAttack(t, f.master, e, "Goblin", sword, "Ragna", inAppRoll)
	return f.mustDamage(t, f.master, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage()
}

// TestMR025_ATableRaceShowsItsChoicesAndResistsInCombat: the sheet of Ragna shows the size and the
// bonuses she placed (no issue), an issue says when the placed bonuses cover none of the sets, and the
// goblin's nonmagical sword hurts her half, then (with the table's feats) double, then not at all.
func TestMR025_ATableRaceShowsItsChoicesAndResistsInCombat(t *testing.T) {
	t.Parallel()
	f, e := newFrostTable(t)

	d := f.toren.GetDerived()
	if d.GetSize() != "Small" {
		t.Errorf("Ragna's size = %q, want Small, the one she picked", d.GetSize())
	}
	if codes := issueCodes(f.toren); slices.Contains(codes, "race_bonus") || slices.Contains(codes, "race_size") {
		t.Errorf("Ragna's issues = %v, want no race_bonus and no race_size: +2 and +1 is one of the sets", codes)
	}
	if !slices.ContainsFunc(d.GetHints(), func(h *rulesv1.Hint) bool { return strings.Contains(h.GetText(), "não mágicos") }) {
		t.Errorf("Ragna's hints = %v, want the resistance to nonmagical weapon damage", d.GetHints())
	}
	// The other set, +1 in three abilities, is also right; two +1 cover neither; a size the race does not offer is an issue.
	covered := f.edit(t, func(full *charactersv1.FullSheet) {
		full.ExtraAbilityBonuses = &rulesv1.AbilityScores{Strength: 1, Dexterity: 1, Constitution: 1}
		full.Size = "Medium"
	})
	if codes := issueCodes(covered); slices.Contains(codes, "race_bonus") || covered.GetDerived().GetSize() != "Medium" {
		t.Errorf("with +1/+1/+1 and Medium: issues %v, size %q; want no race_bonus and Medium", codes, covered.GetDerived().GetSize())
	}
	short := f.edit(t, func(full *charactersv1.FullSheet) {
		full.ExtraAbilityBonuses = &rulesv1.AbilityScores{Strength: 1, Dexterity: 1}
		full.Size = "Large"
	})
	if codes := issueCodes(short); !slices.Contains(codes, "race_bonus") || !slices.Contains(codes, "race_size") || short.GetDerived().GetSize() != "Medium" {
		t.Errorf("with +1/+1 and Large: issues %v, size %q; want race_bonus, race_size and the race's own Medium", codes, short.GetDerived().GetSize())
	}
	f.edit(t, func(full *charactersv1.FullSheet) {
		full.ExtraAbilityBonuses = &rulesv1.AbilityScores{Strength: 2, Dexterity: 1}
		full.Size = "Small"
	})

	// The goblin's sword is a nonmagical weapon: 8 slashing damage lands as 4.
	e = f.passTo(t, e, "Goblin")
	dmg := f.hitRagna(t, e, 6)
	if steps := dmg.GetSteps(); len(steps) != 1 || steps[0].GetKind() != playv1.DamageStepKind_DAMAGE_STEP_KIND_RESISTANCE || steps[0].GetBefore() != 8 || steps[0].GetAfter() != 4 {
		t.Fatalf("steps = %v, want one resistance, 8 to 4", dmg.GetSteps())
	}
	if dmg.GetAmountAfterSteps() != 4 {
		t.Errorf("the damage after the steps = %d, want 4", dmg.GetAmountAfterSteps())
	}
	if _, err := f.settle(t, f.master, e, dmg.GetId(), true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}

	// The master gives her the table's feat of vulnerability to slashing: halved, then doubled (8 -> 4 -> 8).
	f.edit(t, func(full *charactersv1.FullSheet) { full.FeatKeys = []string{f.glassSkin} })
	e = f.nextGoblinTurn(t, e)
	dmg = f.hitRagna(t, e, 6)
	if steps := dmg.GetSteps(); len(steps) != 2 || steps[0].GetKind() != playv1.DamageStepKind_DAMAGE_STEP_KIND_RESISTANCE || steps[1].GetKind() != playv1.DamageStepKind_DAMAGE_STEP_KIND_VULNERABILITY || dmg.GetAmountAfterSteps() != 8 {
		t.Fatalf("steps = %v, after %d; want resistance then vulnerability, 8", dmg.GetSteps(), dmg.GetAmountAfterSteps())
	}
	if _, err := f.settle(t, f.master, e, dmg.GetId(), true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}

	// Immunity beats both.
	f.edit(t, func(full *charactersv1.FullSheet) { full.FeatKeys = []string{f.glassSkin, f.stoneSoul} })
	e = f.nextGoblinTurn(t, e)
	dmg = f.hitRagna(t, e, 6)
	if steps := dmg.GetSteps(); len(steps) != 1 || steps[0].GetKind() != playv1.DamageStepKind_DAMAGE_STEP_KIND_IMMUNITY || dmg.GetAmountAfterSteps() != 0 {
		t.Fatalf("steps = %v, after %d; want one immunity, 0", dmg.GetSteps(), dmg.GetAmountAfterSteps())
	}
	if _, err := f.settle(t, f.master, e, dmg.GetId(), true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	max := f.vitals(t, f.toren).GetHitPointsMax()
	if got := f.vitals(t, f.toren).GetHitPointsCurrent(); got != max-4-8 {
		t.Errorf("Ragna has %d hit points, want %d: 4, then 8, then nothing", got, max-4-8)
	}
}

// traitsOf are the combat traits of Ragna's sheet, as the combat reads them.
func (f *frostTable) traitsOf(t *testing.T) link.Traits {
	t.Helper()
	tx, err := f.h.pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer func() { _ = tx.Rollback(t.Context()) }()
	sheet, err := f.h.svc.roster.CombatSheet(t.Context(), tx, f.campaignID, f.toren.GetId())
	if err != nil {
		t.Fatalf("CombatSheet() error = %v", err)
	}
	return sheet.Traits
}

// nextGoblinTurn passes the turns to the goblin's next one.
func (f *frostTable) nextGoblinTurn(t *testing.T, e *playv1.Encounter) *playv1.Encounter {
	t.Helper()
	if _, err := f.endTurn(t, f.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	return f.passTo(t, f.get(t, f.master), "Goblin")
}

// A sheet that is immune to a condition by a table feat takes none of the effects that give it, and the
// roll mode against a condition reaches the combat as a real advantage on the save.
func TestMR025_ACharacterImmuneToAConditionTakesNoEffectThatGivesIt(t *testing.T) {
	t.Parallel()
	f, e := newFrostTable(t)
	_ = e
	f.edit(t, func(full *charactersv1.FullSheet) { full.FeatKeys = []string{f.coldBlood} })
	traits := f.traitsOf(t)
	if len(traits.ConditionImmunities) != 1 || !slices.Contains(traits.ConditionImmunities[0].Conditions, "condition:poisoned") {
		t.Fatalf("Ragna's condition immunities = %+v, want poisoned", traits.ConditionImmunities)
	}
	if !slices.ContainsFunc(traits.TaggedSaves, func(s link.TaggedSave) bool {
		return s.Against == "condition:frightened" && !s.Disadvantage && slices.Equal(s.Abilities, []string{"wis"})
	}) {
		t.Errorf("Ragna's tagged saves = %+v, want advantage on Wisdom saves against being frightened", traits.TaggedSaves)
	}
	if !sheetImmuneTo(traits, "condition:poisoned") || sheetImmuneTo(traits, "condition:paralyzed") {
		t.Error("the sheet is immune to the wrong conditions")
	}
	// The spells that give a condition skip the target that is immune to it, and say why.
	content, err := testRules()
	if err != nil {
		t.Fatalf("rules.LoadSRD() error = %v", err)
	}
	def, ok := content.CombatSpellEffect("spell:hold-person")
	if !ok || !slices.Contains(def.Conditions, "condition:paralyzed") {
		t.Fatalf("Hold Person's effect = %+v", def)
	}
	svc := &Service{}
	target := playdb.Combatant{ID: "ragna"}
	for _, tc := range []struct {
		name   string
		traits link.Traits
		want   string
	}{{"immune", link.Traits{ConditionImmunities: []link.ConditionImmunity{{Source: "feat:x", Conditions: []string{"condition:paralyzed"}}}}, lastingNoEffect}, {"not immune", traits, ""}} {
		made := &actionEvent{Hits: make([]castHit, 1)}
		svc.markSpellHits(content, def, nil, nil, map[string]link.Traits{"ragna": tc.traits}, []playdb.Combatant{target}, made)
		if got := made.Hits[0].Lasting; got != tc.want && !(tc.want == "" && got == lastingApplied) {
			t.Errorf("%s: Hold Person's hit = %q (%q)", tc.name, got, made.Hits[0].NoEffectWhy)
		}
		if tc.want != "" && made.Hits[0].NoEffectWhy != whyImmune {
			t.Errorf("%s: why = %q, want %q", tc.name, made.Hits[0].NoEffectWhy, whyImmune)
		}
	}
}
