package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Sculpt Spells (SRD 5.1, School of Evocation wizard, level 2): when the caster casts an
// evocation spell that affects other creatures it can see, it spares 1 + the spell's level
// of them: they succeed on the saving throw by themselves and take no damage when the spell
// halves on a success. These tests need the database (MEURPG_TEST_DATABASE_URL).

// newSculptCave is the area cave with Pensantus an evocation wizard (level 5).
func newSculptCave(t *testing.T) *cave {
	t.Helper()
	return newCaveSubclass(t, 5, "subclass:evocation", []string{fireball, lightningBolt, burningHands, thunderwave, sleepSpell})
}

// coneFight puts Burning Hands' cone (east of Pensantus at (17,7)) over Goblin 1, Goblin 2
// and Toren, the ally; Goblin 3 stands outside it.
func (c *cave) coneFight(t *testing.T) {
	t.Helper()
	c.areaFight(t)
	c.mustMove(t, c.master, "Pensantus", 17, 7)
	c.mustMove(t, c.master, "Goblin 1", 18, 7)
	c.mustMove(t, c.master, "Goblin 2", 20, 8)
	c.mustMove(t, c.master, "Goblin 3", 17, 4)
	c.mustMove(t, c.master, "Toren", 20, 6)
}

func sculpt(ids ...string) func(*playv1.CastSpellRequest) {
	return func(r *playv1.CastSpellRequest) { r.SculptedIds = ids }
}

func resultOf(t *testing.T, res *playv1.CastSpellResponse, id string) *playv1.SpellTargetResult {
	t.Helper()
	for _, tg := range res.GetCast().GetTargets() {
		if tg.GetCombatantId() == id {
			return tg
		}
	}
	t.Fatalf("the cast has no result for %s", id)
	return nil
}

// TestSculptSpellsSparesTheAllyInTheCone: the sculpted ally rolls no save and takes no
// damage; the goblins roll and take the damage as ever.
func TestSculptSpellsSparesTheAllyInTheCone(t *testing.T) {
	t.Parallel()
	c := newSculptCave(t)
	c.coneFight(t)

	toren := c.id(t, "Toren")
	c.h.roller.queue(10, 10) // only the two goblins roll: 10 against DC 14 fails
	res := c.mustCastArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(1, 0), sculpt(toren))

	spared := resultOf(t, res, toren)
	if !spared.GetSave().GetSculpted() || spared.GetSave().GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_SAVED {
		t.Errorf("Toren's save = %v, want sculpted and saved", spared.GetSave())
	}
	if spared.GetSave().GetRoll() != nil {
		t.Errorf("Toren's save has a roll %v, want none: the spared creature rolls no d20", spared.GetSave().GetRoll())
	}
	if spared.GetPendingDamageId() != "" {
		t.Errorf("Toren has pending damage %q, want none: the spell halves on a success and he was spared", spared.GetPendingDamageId())
	}
	for _, label := range []string{"Goblin 1", "Goblin 2"} {
		tg := resultOf(t, res, c.id(t, label))
		if tg.GetSave().GetSculpted() || tg.GetSave().GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_FAILED {
			t.Errorf("%s's save = %v, want a rolled, failed save", label, tg.GetSave())
		}
		if tg.GetPendingDamageId() == "" {
			t.Errorf("%s has no pending damage, want the spell's damage", label)
		}
	}
	// The log says it for everyone who reads the save lines.
	if sv := c.castTargetsOf(t, c.get(t, c.master))["Toren"].GetSave(); !sv.GetSculpted() {
		t.Errorf("the log's save of Toren = %v, want sculpted", sv)
	}
}

// TestSculptSpellsCountsOneMorePerSpellLevel: the limit is 1 + the level the spell is cast
// at, so a 1st-level slot spares 2 and a 2nd-level slot spares 3.
func TestSculptSpellsCountsOneMorePerSpellLevel(t *testing.T) {
	t.Parallel()
	c := newSculptCave(t)
	c.coneFight(t)

	ids := []string{c.id(t, "Toren"), c.id(t, "Goblin 1"), c.id(t, "Goblin 2")}
	if _, err := c.castArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(1, 0), sculpt(ids...)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("3 creatures with a 1st-level slot: error = %v, want invalid_argument (the limit is 2)", err)
	}
	if used := usedSlots(c.vitals(t, c.pens), 1); used != 0 {
		t.Errorf("1st-level slots used = %d after a refusal, want 0", used)
	}
	res := c.mustCastArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(2), nil, toward(1, 0), sculpt(ids...))
	for _, id := range ids {
		if !resultOf(t, res, id).GetSave().GetSculpted() {
			t.Errorf("%s was not spared by the upcast", id)
		}
	}
}

// TestSculptSpellsRefusals: a creature the spell does not reach, the caster, a repeated one,
// a spell that is not of evocation and a caster without the feature.
func TestSculptSpellsRefusals(t *testing.T) {
	t.Parallel()
	c := newSculptCave(t)
	c.coneFight(t)

	cases := []struct {
		name  string
		spell string
		slot  *playv1.SpellSlot
		ids   []string
	}{
		{"a creature outside the cone", burningHands, slotOfLevel(1), []string{c.id(t, "Goblin 3")}},
		{"the caster", burningHands, slotOfLevel(1), []string{c.id(t, "Pensantus")}},
		{"the same creature twice", burningHands, slotOfLevel(1), []string{c.id(t, "Toren"), c.id(t, "Toren")}},
		{"a spell that is not of evocation", sleepSpell, slotOfLevel(1), []string{c.id(t, "Toren")}},
		{"not an id", burningHands, slotOfLevel(1), []string{"nope"}},
	}
	for _, tc := range cases {
		dir := toward(1, 0)
		if _, err := c.castArea(t, c.ana, "Pensantus", tc.spell, tc.slot, nil, dir, sculpt(tc.ids...)); err == nil {
			t.Errorf("CastSpell(%s) was accepted, want it refused", tc.name)
		} else if code := connect.CodeOf(err); code != connect.CodeInvalidArgument && code != connect.CodeNotFound {
			t.Errorf("CastSpell(%s) error = %v, want invalid_argument", tc.name, err)
		}
	}
	if used := usedSlots(c.vitals(t, c.pens), 1); used != 0 {
		t.Errorf("1st-level slots used = %d after the refusals, want 0", used)
	}
}

// TestSculptSpellsNeedsTheFeature: a wizard of no school (or another one) has nothing to
// spare creatures with.
func TestSculptSpellsNeedsTheFeature(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.coneFight(t)
	_, err := c.castArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(1, 0), sculpt(c.id(t, "Toren")))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("a caster without Sculpt Spells: error = %v, want invalid_argument", err)
	}
	// Without the list the same cast works, and Toren rolls like the others.
	c.h.roller.queue(10, 10, 10)
	res := c.mustCastArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(1, 0))
	if tg := resultOf(t, res, c.id(t, "Toren")); tg.GetSave().GetSculpted() || tg.GetPendingDamageId() == "" {
		t.Errorf("Toren's result = %v, want a rolled save and damage", tg)
	}
}

// TestSculptSpellsIsOfferedOnlyForEvocationSpells: the turn options say which spells take it.
func TestSculptSpellsIsOfferedOnlyForEvocationSpells(t *testing.T) {
	t.Parallel()
	c := newSculptCave(t)
	e := c.areaFight(t)
	opts, err := c.ana.combat.GetTurnOptions(t.Context(), connect.NewRequest(&playv1.GetTurnOptionsRequest{
		CampaignId: c.campaignID, EncounterId: e.GetId(), CombatantId: c.id(t, "Pensantus"),
	}))
	if err != nil {
		t.Fatalf("GetTurnOptions() error = %v", err)
	}
	got := map[string]bool{}
	for _, sp := range opts.Msg.GetOptions().GetSpells() {
		got[sp.GetSpell().GetKey()] = sp.GetSculptSpells()
	}
	if !got[burningHands] || !got[fireball] || got[sleepSpell] {
		t.Errorf("sculpt_spells by spell = %v, want true for Burning Hands and Fireball, false for Sleep", got)
	}
}
