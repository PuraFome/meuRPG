package play

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A table feature that summons table creatures (#578): the master's class feature goes through the casting paths, the
// checks, the concentration, the replacement and the undo of a summoning spell, and spends a use of the feature's
// resource where a spell spends a slot. The names are invented. These tests need the database (MEURPG_TEST_DATABASE_URL).

const featureUses = "chamados"

// featureSummonClass is a class whose level 1 feature brings two silver wolves (a formula of the proficiency bonus)
// with a use of "chamados", two a long rest. The wolves have hit points of slot(), which is 0 for a feature.
func featureSummonClass() *rulesv1.TableClass {
	c := &rulesv1.TableClass{
		NamePt: "Guardião da Matilha", HitDie: 8,
		SavingThrows: []rulesv1.Ability{rulesv1.Ability_ABILITY_CONSTITUTION, rulesv1.Ability_ABILITY_WISDOM},
		SkillChoose:  2, SkillFrom: []string{"skill:arcana", "skill:history", "skill:medicine", "skill:nature", "skill:survival"},
		Proficiencies: []string{"proficiency:light-armor", "proficiency:simple-weapons"}, SubclassLevel: 3,
	}
	for lvl := 1; lvl <= 20; lvl++ {
		row := &rulesv1.TableClassLevel{}
		if lvl == 1 {
			row.Features = []*rulesv1.TableFeature{{
				NamePt: "Chamado da Matilha", DescPt: []string{"O guardião chama a matilha."},
				Effects: []*rulesv1.TableEffect{{Type: "resource", Resource: featureUses, Max: "2", Recharge: "long_rest"}},
				Summon: &rulesv1.TableSummon{
					CreatureKeys: []string{tableSilverWolf}, Count: "1 + floor(prof() / 2) + slot()", MaxLive: 4,
					TurnMode: rulesv1.TableSummonTurnMode_TABLE_SUMMON_TURN_MODE_OWN_INITIATIVE, Replaces: rulesv1.TableSummonReplaces_TABLE_SUMMON_REPLACES_SAME_SUMMON,
					ResourceKey: featureUses, Concentration: true,
					Duration: &rulesv1.TableSpellDuration{Kind: rulesv1.SpellDurationKind_SPELL_DURATION_KIND_TIMED, Amount: 1, Unit: rulesv1.SpellDurationUnit_SPELL_DURATION_UNIT_MINUTE},
				},
			}}
		}
		c.Levels = append(c.Levels, row)
	}
	return c
}

// featureSummoners is the party of newSummoners with the silver wolf, and Gaia, a guardian of level 5 (proficiency +3, so two
// wolves) who also has a plain feature, for Toren's player to compare with.
type featureSummoners struct {
	*armed
	class string
	gaia  *charactersv1.Character
}

func newFeatureSummoners(t *testing.T) *featureSummoners {
	t.Helper()
	f := &featureSummoners{}
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
		create(&rulesv1.CreateTableEntryRequest{Body: &rulesv1.CreateTableEntryRequest_TableCreature{TableCreature: &rulesv1.TableCreature{
			NamePt: "Lobo de Prata", BaseKey: "monster:wolf", HitPointsFormula: "8 + 4 * slot()",
		}}})
		f.class = create(&rulesv1.CreateTableEntryRequest{Body: &rulesv1.CreateTableEntryRequest_TableClass{TableClass: featureSummonClass()}})
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		f.gaia = a.ana.heroSheet(t, a.campaignID, "Pensantus", &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8},
			RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: f.class, Level: 5}},
		})
		a.pens = f.gaia
		a.bri = a.bia.caster(t, a.campaignID, "Sálvia", "class:druid", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 12, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, nil, nil, nil, nil)
	})
	return f
}

// featureOptions is the options of the guardian's feature summon; the test fails if there is none.
func (f *featureSummoners) featureOptions(t *testing.T) *charactersv1.SummonSpellOptions {
	t.Helper()
	for _, s := range f.summonOptions(t, f.ana, f.gaia).GetSpells() {
		if s.GetGrantedBy() != "" {
			return s
		}
	}
	t.Fatalf("the options = %v, want the feature's summon", f.summonOptions(t, f.ana, f.gaia).GetSpells())
	return nil
}

// castFeature calls CastSummon for the feature, with the choice of n silver wolves.
func (f *featureSummoners) castFeature(t *testing.T, u *user, opts *charactersv1.SummonSpellOptions, n int, edit ...func(*playv1.CastSummonRequest)) (*playv1.CastSummonResponse, error) {
	t.Helper()
	req := &playv1.CastSummonRequest{
		CampaignId: f.campaignID, CharacterId: f.gaia.GetId(), SpellKey: opts.GetSpellKey(), GrantedBy: opts.GetGrantedBy(), IdempotencyKey: newKey(),
		Summon: &playv1.SummonChoice{CreatureKeys: wolves(n)},
	}
	for _, e := range edit {
		e(req)
	}
	res, err := u.play.CastSummon(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// The options, the checks and the spend of a feature's summon cast outside a combat, and the rest that gives the uses back.
func TestMR578_ATableFeatureSummonsItsCreaturesOutsideACombatForAUse(t *testing.T) {
	t.Parallel()
	f := newFeatureSummoners(t)

	opts := f.featureOptions(t)
	if opts.GetResourceKey() != featureUses || opts.GetUsesLeft() != 2 || opts.GetCanCastWithSlot() || opts.GetCanRitual() || !opts.GetConcentration() || len(opts.GetCircles()) != 1 {
		t.Fatalf("options = %v, want the feature's summon with 2 uses of %q, no slot, no ritual, one circle, concentration", opts, featureUses)
	}
	if got := opts.GetCircles()[0].GetOptions()[0]; got.GetCount() != 2 || len(got.GetForms()) != 1 || got.GetForms()[0].GetMonsterKey() != tableSilverWolf {
		t.Fatalf("option = %v, want two silver wolves (1 + floor(3 / 2) + 0)", got)
	}
	if strings.Contains(opts.GetGrantedBy(), tableSilverWolf) || !strings.HasPrefix(opts.GetGrantedBy(), "feature:") {
		t.Errorf("granted_by = %q, want the feature's key", opts.GetGrantedBy())
	}

	// The shapes the call refuses: a slot or a ritual with a feature, a feature that does not give the summon, and
	// the checks of the spell (the wrong number of creatures). None spends the use.
	for name, edit := range map[string]func(*playv1.CastSummonRequest){
		"a slot":      func(r *playv1.CastSummonRequest) { r.Slot = slotOfLevel(1) },
		"a ritual":    func(r *playv1.CastSummonRequest) { r.Ritual = true },
		"no feature":  func(r *playv1.CastSummonRequest) { r.GrantedBy = "" },
		"another one": func(r *playv1.CastSummonRequest) { r.GrantedBy = "feature:outra@mesa" },
		// The feature's spell is not a spell of the character's own: with a slot and no feature it is refused too.
		"a slot, no feature":   func(r *playv1.CastSummonRequest) { r.GrantedBy, r.Slot = "", slotOfLevel(1) },
		"a ritual, no feature": func(r *playv1.CastSummonRequest) { r.GrantedBy, r.Ritual = "", true },
	} {
		if _, err := f.castFeature(t, f.ana, opts, 2, edit); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("CastSummon(%s) error = %v, want invalid_argument", name, err)
		}
	}
	_, err := f.castFeature(t, f.ana, opts, 3)
	wantBlockedBy(t, "CastSummon(three wolves of two)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_SUMMON_CHOICE_INVALID)
	if _, err := f.castFeature(t, f.caio, opts, 2); connect.CodeOf(err) == 0 {
		t.Error("another player cast the feature's summon")
	}
	if usedOf(f.vitals(t, f.gaia), featureUses) > 0 {
		t.Fatal("a refused summon spent a use")
	}

	// The cast: two wolves with 8 hit points (slot() is 0), a use spent, and they depend on the concentration.
	res, err := f.castFeature(t, f.ana, opts, 2)
	if err != nil {
		t.Fatalf("CastSummon() error = %v", err)
	}
	list := f.mustCreatures(t, f.ana, f.gaia)
	if len(res.GetCreatureIds()) != 2 || len(list) != 2 || usedOf(res.GetVitals(), featureUses) != 1 || usedOf(f.vitals(t, f.gaia), featureUses) != 1 {
		t.Fatalf("the cast = %v, creatures %v; want two wolves and one use spent", res, creatureNames(list))
	}
	for _, c := range list {
		if c.GetSource() != charactersv1.CreatureSource_CREATURE_SOURCE_TABLE || c.GetHitPointsMax() != 8 || !c.GetDependsOnConcentration() {
			t.Errorf("creature = %v, want a table wolf of 8 hit points (slot() is 0) on a concentration", c)
		}
	}
	if got := f.featureOptions(t); got.GetUsesLeft() != 1 || len(got.GetReplaces()) != 2 {
		t.Errorf("options after the cast: %d uses left, %d replaced; want 1 and the two wolves", got.GetUsesLeft(), len(got.GetReplaces()))
	}

	// The second casting replaces the first (and the concentration), the third has no use left.
	again, err := f.castFeature(t, f.ana, opts, 2)
	if err != nil || len(again.GetDismissedCreatureIds()) != 2 {
		t.Fatalf("CastSummon(second) = %v, %v; want the first two wolves dismissed", again, err)
	}
	_, err = f.castFeature(t, f.ana, opts, 2)
	wantBlockedBy(t, "CastSummon(no use left)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_USES)
	if got := f.featureOptions(t); got.GetUsesLeft() != 0 || len(got.GetCircles()) != 0 {
		t.Errorf("options with no use left: %d uses, %d circles; want none of either", got.GetUsesLeft(), len(got.GetCircles()))
	}

	// A long rest gives the uses back, as it does for the feature's other resources.
	if _, err := f.master.resource.TakeRest(t.Context(), connect.NewRequest(&playv1.TakeRestRequest{CampaignId: f.campaignID, Kind: playv1.RestKind_REST_KIND_LONG, IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("TakeRest(long) error = %v", err)
	}
	if got := f.featureOptions(t); got.GetUsesLeft() != 2 || len(got.GetCircles()) != 1 {
		t.Errorf("options after a long rest: %d uses, %d circles; want 2 and one", got.GetUsesLeft(), len(got.GetCircles()))
	}
}

// featureFight is the fight of the summoners with the guardian among them.
func (f *featureSummoners) featureFight(t *testing.T) *playv1.Encounter {
	t.Helper()
	return f.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: f.goblin.GetId()}},
		npcRolls: []int{2},
		players:  map[string]int32{"Pensantus": 20, "Sálvia": 15, "Toren": 12},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Sálvia": {6, 5}},
	})
}

// In a combat the feature's creatures join the order and the map with the concentration of the spell paths; the undo
// gives the use back, takes the creatures away and brings the replaced ones back.
func TestMR578_ATableFeatureSummonsInACombatAndTheUndoGivesTheUseBack(t *testing.T) {
	t.Parallel()
	f := newFeatureSummoners(t)
	opts := f.featureOptions(t)
	e := f.featureFight(t)

	castFeature := func(e *playv1.Encounter, faces ...int) (*playv1.CastSpellResponse, error) {
		t.Helper()
		f.h.roller.queue(faces...)
		return f.cast(t, f.ana, e, "Pensantus", opts.GetSpellKey(), nil, nil, func(r *playv1.CastSpellRequest) {
			r.GrantedBy = opts.GetGrantedBy()
			r.Roll = &playv1.CastSpellRequest_RollInApp{RollInApp: true}
			r.Summon = &playv1.SummonChoice{CreatureKeys: wolves(2)}
		})
	}
	// The same refusals as the slot's: a slot with a feature is no way to cast it, and the use is not spent.
	_, err := f.cast(t, f.ana, e, "Pensantus", opts.GetSpellKey(), slotOfLevel(1), nil, func(r *playv1.CastSpellRequest) {
		r.GrantedBy = opts.GetGrantedBy()
		r.Roll = &playv1.CastSpellRequest_RollInApp{RollInApp: true}
		r.Summon = &playv1.SummonChoice{CreatureKeys: wolves(2)}
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("CastSpell(a slot for a feature) error = %v, want invalid_argument", err)
	}
	if _, err := f.cast(t, f.ana, e, "Pensantus", opts.GetSpellKey(), nil, nil, func(r *playv1.CastSpellRequest) {
		r.Roll = &playv1.CastSpellRequest_RollInApp{RollInApp: true}
		r.Summon = &playv1.SummonChoice{CreatureKeys: wolves(2)}
	}); err == nil {
		t.Error("CastSpell(the feature's spell without granted_by) was accepted: only the feature gives it")
	}
	if usedOf(f.vitals(t, f.gaia), featureUses) > 0 {
		t.Fatal("a refused cast spent a use")
	}

	res, err := castFeature(e, 4, 16)
	if err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	if len(res.GetSummonedCombatantIds()) != 2 || usedOf(f.vitals(t, f.gaia), featureUses) != 1 {
		t.Fatalf("CastSpell() = %v, want two combatants and a use spent", res)
	}
	e = f.get(t, f.ana)
	w1 := byLabel(t, e, "Lobo de Prata 1")
	if w1.GetInitiative() != 6 || w1.GetHitPointsMax() != 8 || !w1.GetControlledByMe() || w1.GetOwnerCharacterId() != f.gaia.GetId() {
		t.Errorf("wolf = %v, want initiative 6 (its own roll), 8 hit points, played by the guardian's player", w1)
	}
	if pens := byLabel(t, e, "Pensantus"); pens.GetConcentrationSpell() != opts.GetSpellKey() {
		t.Errorf("the guardian concentrates on %q, want the feature's summon", pens.GetConcentrationSpell())
	}
	// The undo gives the use back and takes the wolves away; the cast is made again.
	f.undoLast(t, e)
	if got := creatureCombatants(f.get(t, f.ana)); len(got) != 0 || usedOf(f.vitals(t, f.gaia), featureUses) != 0 || len(f.mustCreatures(t, f.ana, f.gaia)) != 0 {
		t.Fatalf("after the undo: combatants %v, %d uses spent; want none of either", got, usedOf(f.vitals(t, f.gaia), featureUses))
	}
	if _, err := castFeature(f.get(t, f.ana), 4, 16); err != nil {
		t.Fatalf("CastSpell(again) error = %v", err)
	}
	e = f.get(t, f.ana)
	first := creatureCombatants(e)

	// A second casting replaces the first (the new concentration ends the old one): the old wolves leave, and an
	// undo brings them back with the use.
	if _, err := castFeature(f.turnOf(t, "Pensantus"), 9, 10); err != nil {
		t.Fatalf("CastSpell(second) error = %v", err)
	}
	if got := f.dismissalReasons(t); !slices.Equal(got, []string{"concentration", "concentration"}) {
		t.Errorf("dismissal reasons = %v, want \"concentration\" for each old wolf", got)
	}
	e = f.get(t, f.ana)
	if second := creatureCombatants(e); len(second) != 2 || slices.ContainsFunc(second, func(id string) bool { return slices.Contains(first, id) }) {
		t.Fatalf("combatants = %v, want only the new wolves (the old were %v)", second, first)
	}
	if usedOf(f.vitals(t, f.gaia), featureUses) != 2 {
		t.Fatalf("uses spent = %d, want 2", usedOf(f.vitals(t, f.gaia), featureUses))
	}
	f.undoLast(t, e)
	e = f.get(t, f.ana)
	if got := creatureCombatants(e); !slices.Equal(got, first) || usedOf(f.vitals(t, f.gaia), featureUses) != 1 {
		t.Errorf("after the undo: combatants %v, %d uses spent; want the first wolves %v and 1 use", got, usedOf(f.vitals(t, f.gaia), featureUses), first)
	}
}

// CastSpellOutsideCombat is a way to cast the feature's summon too: the grant, the summon choice and the use.
func TestMR578_ATableFeatureSummonsThroughCastSpellOutsideCombat(t *testing.T) {
	t.Parallel()
	f := newFeatureSummoners(t)
	opts := f.featureOptions(t)
	cast := func(edit ...func(*playv1.CastSpellOutsideCombatRequest)) (*playv1.CastSpellOutsideCombatResponse, error) {
		t.Helper()
		req := &playv1.CastSpellOutsideCombatRequest{
			CampaignId: f.campaignID, CasterCharacterId: f.gaia.GetId(), SpellKey: opts.GetSpellKey(), GrantedBy: opts.GetGrantedBy(),
			Summon: &playv1.SummonChoice{CreatureKeys: wolves(2)}, IdempotencyKey: newKey(),
		}
		for _, e := range edit {
			e(req)
		}
		res, err := f.ana.casting.CastSpellOutsideCombat(t.Context(), connect.NewRequest(req))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	if _, err := cast(func(r *playv1.CastSpellOutsideCombatRequest) { r.GrantedBy = "" }); err == nil {
		t.Error("the feature's summon was cast outside a combat without its feature")
	}
	if _, err := cast(func(r *playv1.CastSpellOutsideCombatRequest) { r.Summon = nil }); err == nil {
		t.Error("the feature's summon was cast without a summon choice")
	}
	res, err := cast()
	if err != nil {
		t.Fatalf("CastSpellOutsideCombat() error = %v", err)
	}
	if len(res.GetCreatureIds()) != 2 || usedOf(f.vitals(t, f.gaia), featureUses) != 1 || len(f.mustCreatures(t, f.ana, f.gaia)) != 2 {
		t.Errorf("the cast = %v, want two wolves and a use spent", res)
	}
}

// A feature's summon is no spell for a Counterspell to answer: the Mage next
// to the guardian has Counterspell and the slot, and the cast still happens at once.
func TestMR578_ATableFeatureSummonOpensNoCounterspellWindow(t *testing.T) {
	t.Parallel()
	f := newFeatureSummoners(t)
	opts := f.featureOptions(t)
	e := f.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: f.goblin.GetId()}}, npcRolls: []int{1},
		players: map[string]int32{"Pensantus": 20, "Sálvia": 15, "Toren": 12}, setup: true,
	})
	f.h.roller.queue(1)
	f.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) { r.CreatureKey, r.Count = "monster:mage", 1 })
	e = f.get(t, f.master)
	for label, sq := range map[string][2]int32{"Toren": {6, 5}, "Pensantus": {5, 5}, "Sálvia": {5, 6}, "Goblin": {12, 8}, "Mago": {7, 5}} {
		if _, err := f.master.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
			CampaignId: f.campaignID, EncounterId: e.GetId(), CombatantId: f.id(t, label), IdempotencyKey: newKey(), Col: sq[0], Row: sq[1],
		})); err != nil {
			t.Fatalf("MoveCombatant(%s) error = %v", label, err)
		}
	}
	for _, shown := range []string{"Mago", "Goblin"} {
		if _, err := f.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
			CampaignId: f.campaignID, EncounterId: e.GetId(), CombatantId: f.id(t, shown), IdempotencyKey: newKey(), Hidden: false,
		})); err != nil {
			t.Fatalf("SetCombatantHidden(%s) error = %v", shown, err)
		}
	}
	f.begin(t, e)
	e = f.turnOf(t, "Pensantus")

	f.h.roller.queue(4, 16)
	res, err := f.cast(t, f.ana, e, "Pensantus", opts.GetSpellKey(), nil, nil, func(r *playv1.CastSpellRequest) {
		r.GrantedBy = opts.GetGrantedBy()
		r.Roll = &playv1.CastSpellRequest_RollInApp{RollInApp: true}
		r.Summon = &playv1.SummonChoice{CreatureKeys: wolves(2)}
	})
	if err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	if len(res.GetSummonedCombatantIds()) != 2 || usedOf(f.vitals(t, f.gaia), featureUses) != 1 {
		t.Fatalf("CastSpell() = %v, want the wolves at once and a use spent", res)
	}
	if w := f.windowOf(t, f.master, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL); w != nil {
		t.Errorf("the master has a Counterspell window %v for a feature's summon", w)
	}
}
