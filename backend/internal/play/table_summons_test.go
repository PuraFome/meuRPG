package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A table spell that summons table creatures (#578): the master's spell goes through the casting paths, the checks, the
// concentration, the replacement and the undo of the SRD's summons. The names are invented; the creature is a wolf with
// hit points of the slot. These tests need the database (MEURPG_TEST_DATABASE_URL).

const (
	tableSilverWolf = "creature:lobo-de-prata@mesa"
	tablePackSpell  = "spell:matilha-fiel@mesa"      // one wolf for each circle of the slot; each wolf rolls its own initiative
	tableBandSpell  = "spell:bando-unido@mesa"       // two wolves on a concentration; a new casting replaces the old; one roll for both
	tableHerdSpell  = "spell:alcateia-renovada@mesa" // two wolves, no concentration; a new casting replaces the old; each rolls its own
)

// tableSummonSpell is a 1st-circle spell of the wizard's list that summons the silver wolf.
func tableSummonSpell(name string, s *rulesv1.TableSummon) *rulesv1.TableSpell {
	sp := tableSpell(name, 1, "class:wizard", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF, tableTarget(rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_SELF))
	sp.Summon = s
	return sp
}

// newTableSummoners is the party of newSummoners with the table's two summoning spells, known and prepared by Pensantus.
func newTableSummoners(t *testing.T) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		if _, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableCreature{TableCreature: &rulesv1.TableCreature{
				NamePt: "Lobo de Prata", BaseKey: "monster:wolf", HitPointsFormula: "8 + 4 * slot()",
			}},
		})); err != nil {
			t.Fatalf("CreateTableEntry(creature) error = %v", err)
		}
		pack := tableSummonSpell("Matilha Fiel", &rulesv1.TableSummon{
			CreatureKeys: []string{tableSilverWolf}, Count: "slot()", MaxLive: 4,
			TurnMode: rulesv1.TableSummonTurnMode_TABLE_SUMMON_TURN_MODE_OWN_INITIATIVE,
		})
		band := tableSummonSpell("Bando Unido", &rulesv1.TableSummon{
			CreatureKeys: []string{tableSilverWolf}, Count: "2",
			TurnMode: rulesv1.TableSummonTurnMode_TABLE_SUMMON_TURN_MODE_OWNER_ROLL_GROUP, Replaces: rulesv1.TableSummonReplaces_TABLE_SUMMON_REPLACES_SAME_SUMMON,
		})
		herd := tableSummonSpell("Alcateia Renovada", &rulesv1.TableSummon{
			CreatureKeys: []string{tableSilverWolf}, Count: "2",
			TurnMode: rulesv1.TableSummonTurnMode_TABLE_SUMMON_TURN_MODE_OWN_INITIATIVE, Replaces: rulesv1.TableSummonReplaces_TABLE_SUMMON_REPLACES_SAME_SUMMON,
		})
		band.Concentration = true
		band.Duration = &rulesv1.TableSpellDuration{Kind: rulesv1.SpellDurationKind_SPELL_DURATION_KIND_TIMED, Amount: 1, Unit: rulesv1.SpellDurationUnit_SPELL_DURATION_UNIT_MINUTE, UpTo: true}
		for _, sp := range []*rulesv1.TableSpell{pack, band, herd} {
			if err := addTableSpell(t, a, sp); err != nil {
				t.Fatalf("CreateTableEntry(%s) error = %v", sp.GetNamePt(), err)
			}
		}
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 9,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{tablePackSpell, tableBandSpell, tableHerdSpell}, []string{tablePackSpell, tableBandSpell, tableHerdSpell})
		a.bri = a.bia.caster(t, a.campaignID, "Sálvia", "class:druid", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 12, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, nil, nil, nil, nil)
	})
}

func (a *armed) summonOptions(t *testing.T, u *user, c *charactersv1.Character) *charactersv1.GetSummonOptionsResponse {
	t.Helper()
	res, err := u.characters.GetSummonOptions(t.Context(), connect.NewRequest(&charactersv1.GetSummonOptionsRequest{CampaignId: a.campaignID, CharacterId: c.GetId()}))
	if err != nil {
		t.Fatalf("GetSummonOptions() error = %v", err)
	}
	return res.Msg
}

func wolves(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = tableSilverWolf
	}
	return out
}

// The options, the checks, the formulas and the limits of a table spell cast outside a combat.
func TestMR578_ATableSpellSummonsItsCreaturesOutsideACombat(t *testing.T) {
	t.Parallel()
	a := newTableSummoners(t)

	opts := a.summonOptions(t, a.ana, a.pens)
	var pack *charactersv1.SummonSpellOptions
	for _, s := range opts.GetSpells() {
		if s.GetSpellKey() == tablePackSpell {
			pack = s
		}
	}
	if pack == nil || !pack.GetCanCastWithSlot() || len(pack.GetCircles()) == 0 {
		t.Fatalf("the options = %v, want the table spell castable with a slot", opts.GetSpells())
	}
	for _, c := range pack.GetCircles() {
		if got := c.GetOptions()[0]; got.GetCount() != c.GetCircle() || len(got.GetForms()) != 1 || got.GetForms()[0].GetMonsterKey() != tableSilverWolf {
			t.Errorf("circle %d: option = %v, want %d silver wolves", c.GetCircle(), got, c.GetCircle())
		}
	}

	// The checks of the SRD: the wrong number of creatures, and a creature the spell does not list.
	_, err := a.castSummon(t, a.ana, a.pens, tablePackSpell, slotOfLevel(2), 0, wolves(3))
	wantBlockedBy(t, "CastSummon(three wolves with a 2nd circle slot)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_SUMMON_CHOICE_INVALID)
	_, err = a.castSummon(t, a.ana, a.pens, tablePackSpell, slotOfLevel(2), 0, []string{"monster:wolf", "monster:wolf"})
	wantBlockedBy(t, "CastSummon(an SRD wolf the spell does not list)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_SUMMON_CHOICE_INVALID)
	if usedSlots(a.vitals(t, a.pens), 2) != 0 {
		t.Fatal("a refused summon spent a slot")
	}

	// The cast: the count is the circle, the wolves have the hit points of the formula with that circle, each its own group.
	res := a.mustCastSummon(t, a.ana, a.pens, tablePackSpell, slotOfLevel(2), 0, wolves(2))
	list := a.mustCreatures(t, a.ana, a.pens)
	if len(res.GetCreatureIds()) != 2 || len(list) != 2 || usedSlots(a.vitals(t, a.pens), 2) != 1 {
		t.Fatalf("the cast = %v, creatures %v; want two wolves and a slot spent", res, creatureNames(list))
	}
	for _, c := range list {
		if c.GetSource() != charactersv1.CreatureSource_CREATURE_SOURCE_TABLE || c.GetMonsterKey() != tableSilverWolf || c.GetHitPointsMax() != 16 || c.GetDependsOnConcentration() {
			t.Errorf("creature = %v, want a table wolf of 16 hit points (8 + 4 * 2), not on a concentration", c)
		}
	}
	if list[0].GetSummonGroupId() == list[1].GetSummonGroupId() {
		t.Errorf("the wolves share the group %q, want one group each (each rolls its own initiative)", list[0].GetSummonGroupId())
	}

	// The limit of the spell: four at most. Two more fit; three more do not, and the slot of the refused cast stays.
	_, err = a.castSummon(t, a.ana, a.pens, tablePackSpell, slotOfLevel(3), 0, wolves(3))
	if got := charactersBlocked(t, err); got != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CREATURE_LIMIT {
		t.Fatalf("CastSummon(over max_live) reason = %v, want CREATURE_LIMIT", got)
	}
	if usedSlots(a.vitals(t, a.pens), 3) != 0 || len(a.mustCreatures(t, a.ana, a.pens)) != 2 {
		t.Error("a cast over the limit must spend nothing and make nothing")
	}
	a.mustCastSummon(t, a.ana, a.pens, tablePackSpell, slotOfLevel(2), 0, wolves(2))
	if got := a.mustCreatures(t, a.ana, a.pens); len(got) != 4 {
		t.Errorf("creatures = %d, want 4", len(got))
	}
}

// A spell that replaces its own creatures and needs a concentration: the old ones go ("replaced"), and the new casting
// is the character's concentration like Conjure Animals'.
func TestMR578_ATableSpellReplacesItsOwnCreaturesAndDependsOnConcentration(t *testing.T) {
	t.Parallel()
	a := newTableSummoners(t)
	a.mustCastSummon(t, a.ana, a.pens, tableBandSpell, slotOfLevel(1), 0, wolves(2))
	first := a.mustCreatures(t, a.ana, a.pens)
	if len(first) != 2 || !first[0].GetDependsOnConcentration() || first[0].GetSummonGroupId() != first[1].GetSummonGroupId() || first[0].GetHitPointsMax() != 12 {
		t.Fatalf("creatures = %v, want two wolves of 12 hit points on a concentration, one group", first)
	}
	for _, s := range a.summonOptions(t, a.ana, a.pens).GetSpells() {
		if s.GetSpellKey() == tableBandSpell && len(s.GetReplaces()) != 2 {
			t.Errorf("the options say %d replaced creatures, want the 2 wolves", len(s.GetReplaces()))
		}
	}
	// The pack spell (no replacement, no concentration) keeps the old wolves; the band spell sends them away.
	a.mustCastSummon(t, a.ana, a.pens, tablePackSpell, slotOfLevel(1), 0, wolves(1))
	res := a.mustCastSummon(t, a.ana, a.pens, tableBandSpell, slotOfLevel(2), 0, wolves(2))
	if len(res.GetDismissedCreatureIds()) != 2 {
		t.Errorf("dismissed = %v, want the first band's two wolves", res.GetDismissedCreatureIds())
	}
	if got := a.dismissalReasons(t); !slices.Equal(got, []string{"replaced", "replaced"}) {
		t.Errorf("dismissal reasons = %v, want \"replaced\" for each wolf of the spell's own old casting", got)
	}
	if got := a.mustCreatures(t, a.ana, a.pens); len(got) != 3 || got[0].GetId() == first[0].GetId() {
		t.Errorf("creatures = %v, want the pack's wolf and the new band", creatureNames(got))
	}
}

// In a combat the creatures join the order and the map: a spell that rolls its own initiative rolls once for each creature, a
// group spell once for all of them; the undo gives the slot back and takes the creatures away.
func TestMR578_ATableSpellSummonsInACombatAndTheUndoTakesItBack(t *testing.T) {
	t.Parallel()
	a := newTableSummoners(t)
	e := a.summonersFight(t)

	// Pensantus is first (20): his own wolves roll 4 and 16, with the wolf's +2 Dexterity.
	a.h.roller.queue(4, 16)
	res, err := a.cast(t, a.ana, e, "Pensantus", tablePackSpell, slotOfLevel(2), nil, func(r *playv1.CastSpellRequest) {
		r.Roll = &playv1.CastSpellRequest_RollInApp{RollInApp: true}
		r.Summon = &playv1.SummonChoice{CreatureKeys: wolves(2)}
	})
	if err != nil {
		t.Fatalf("CastSpell(%s) error = %v", tablePackSpell, err)
	}
	if len(res.GetSummonedCombatantIds()) != 2 || usedSlots(a.vitals(t, a.pens), 2) != 1 {
		t.Fatalf("CastSpell() = %v, want two combatants and the 2nd-circle slot spent", res)
	}
	e = a.get(t, a.ana)
	w1, w2 := byLabel(t, e, "Lobo de Prata 1"), byLabel(t, e, "Lobo de Prata 2")
	if w1.GetInitiative() != 6 || w2.GetInitiative() != 18 || w1.GetInitiativeFace() != 4 || w2.GetInitiativeFace() != 16 {
		t.Errorf("initiatives = %d (face %d) and %d (face %d), want 6 (4) and 18 (16): each rolled its own", w1.GetInitiative(), w1.GetInitiativeFace(), w2.GetInitiative(), w2.GetInitiativeFace())
	}
	if w1.GetSummonGroupId() == w2.GetSummonGroupId() || w1.GetHitPointsMax() != 16 || !w1.GetControlledByMe() || !w1.GetPlaced() || w1.GetOwnerCharacterId() != a.pens.GetId() {
		t.Errorf("wolf = %v, want a group of its own, 16 hit points, on the map and played by Pensantus's player", w1)
	}
	a.undoLast(t, e)
	if got := labels(a.get(t, a.master)); len(got) != 4 {
		t.Errorf("combatants = %v, want the wolves gone", got)
	}
	if usedSlots(a.vitals(t, a.pens), 2) != 0 || len(a.mustCreatures(t, a.ana, a.pens)) != 0 {
		t.Error("the undo must give the slot back and take the creatures away")
	}

	// The group spell: both wolves take the one roll, face 12 + 2 = 14, and Pensantus concentrates.
	res, err = a.cast(t, a.ana, a.get(t, a.master), "Pensantus", tableBandSpell, slotOfLevel(1), nil, func(r *playv1.CastSpellRequest) {
		r.Roll = &playv1.CastSpellRequest_D20Face{D20Face: 12}
		r.Summon = &playv1.SummonChoice{CreatureKeys: wolves(2)}
	})
	if err != nil {
		t.Fatalf("CastSpell(%s) error = %v", tableBandSpell, err)
	}
	e = a.get(t, a.ana)
	b1, b2 := byLabel(t, e, "Lobo de Prata 1"), byLabel(t, e, "Lobo de Prata 2")
	if len(res.GetSummonedCombatantIds()) != 2 || b1.GetSummonGroupId() != b2.GetSummonGroupId() || b1.GetInitiative() != 14 || b2.GetInitiative() != 14 || b1.GetHitPointsMax() != 12 {
		t.Errorf("wolves = %v and %v, want one group rolled 14 with 12 hit points", b1, b2)
	}
	if pens := byLabel(t, e, "Pensantus"); pens.GetConcentrationSpell() != tableBandSpell {
		t.Errorf("Pensantus concentrates on %q, want the band spell", pens.GetConcentrationSpell())
	}
	a.undoLast(t, e)
	if usedSlots(a.vitals(t, a.pens), 1) != 0 || len(a.mustCreatures(t, a.ana, a.pens)) != 0 || byLabel(t, a.get(t, a.master), "Pensantus").GetConcentrationSpell() != "" {
		t.Error("the undo must give the slot and the concentration back and take the creatures away")
	}
}

// dismissalReasons are the reasons of the creature_dismissed events, in order.
func (a *armed) dismissalReasons(t *testing.T) []string {
	t.Helper()
	rows, err := a.h.pool.Query(t.Context(), `SELECT payload->>'reason' FROM session_events WHERE kind = 'creature_dismissed' ORDER BY seq`)
	if err != nil {
		t.Fatalf("read the dismissals: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, r)
	}
	return out
}

// creatureCombatants are the ids of the creatures in the combat.
func creatureCombatants(e *playv1.Encounter) []string {
	var out []string
	for _, c := range e.GetCombatants() {
		if c.GetKind() == playv1.CombatantKind_COMBATANT_KIND_CREATURE {
			out = append(out, c.GetId())
		}
	}
	slices.Sort(out)
	return out
}

// A table spell that replaces its own creatures, in a combat and with no concentration: the old combatants leave the
// order and the map, the line says "replaced", and an undo of the new casting brings the old ones back.
func TestMR578_ATableSpellInACombatReplacesItsOwnCombatantsAndTheUndoBringsThemBack(t *testing.T) {
	t.Parallel()
	a := newTableSummoners(t)
	e := a.summonersFight(t)
	castHerd := func(e *playv1.Encounter, faces ...int) {
		t.Helper()
		a.h.roller.queue(faces...)
		if _, err := a.cast(t, a.ana, e, "Pensantus", tableHerdSpell, slotOfLevel(1), nil, func(r *playv1.CastSpellRequest) {
			r.Roll = &playv1.CastSpellRequest_RollInApp{RollInApp: true}
			r.Summon = &playv1.SummonChoice{CreatureKeys: wolves(2)}
		}); err != nil {
			t.Fatalf("CastSpell(%s) error = %v", tableHerdSpell, err)
		}
	}
	// A typed d20 is one face: two creatures that each roll their own initiative need the roller of the app.
	_, err := a.cast(t, a.ana, e, "Pensantus", tableHerdSpell, slotOfLevel(1), nil, func(r *playv1.CastSpellRequest) {
		r.Roll = &playv1.CastSpellRequest_D20Face{D20Face: 12}
		r.Summon = &playv1.SummonChoice{CreatureKeys: wolves(2)}
	})
	wantBlockedBy(t, "CastSpell(one typed face for two initiatives)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_SUMMON_NEEDS_INITIATIVE)
	if usedSlots(a.vitals(t, a.pens), 1) != 0 {
		t.Error("a refused cast spent a slot")
	}
	castHerd(e, 4, 16)
	first := creatureCombatants(a.get(t, a.ana))
	oldCreatures := a.mustCreatures(t, a.ana, a.pens)
	if len(first) != 2 || len(oldCreatures) != 2 {
		t.Fatalf("after the first casting: combatants %v, creatures %d, want two of each", first, len(oldCreatures))
	}

	castHerd(a.turnOf(t, "Pensantus"), 9, 10)
	if got := a.dismissalReasons(t); !slices.Equal(got, []string{"replaced", "replaced"}) {
		t.Errorf("dismissal reasons = %v, want \"replaced\" for each old wolf", got)
	}
	e = a.get(t, a.ana)
	second := creatureCombatants(e)
	if len(second) != 2 || slices.ContainsFunc(second, func(id string) bool { return slices.Contains(first, id) }) {
		t.Fatalf("combatants = %v after the second casting, want only the two new ones (the old were %v)", second, first)
	}
	if got := len(a.mustCreatures(t, a.ana, a.pens)); got != 2 {
		t.Errorf("creatures = %d, want the two new ones", got)
	}
	a.undoLast(t, e)
	e = a.get(t, a.ana)
	if got := creatureCombatants(e); !slices.Equal(got, first) {
		t.Errorf("combatants = %v after the undo, want the first casting's %v back and the new ones gone", got, first)
	}
	back := a.mustCreatures(t, a.ana, a.pens)
	if len(back) != 2 || usedSlots(a.vitals(t, a.pens), 1) != 1 {
		t.Errorf("after the undo: %d creatures and %d slots spent, want the old two and the first casting's slot", len(back), usedSlots(a.vitals(t, a.pens), 1))
	}
}
