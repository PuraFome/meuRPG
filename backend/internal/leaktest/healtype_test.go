package leaktest

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The creature type in the spells of life (SRD 5.1, RN-02, RN-10): Cure Wounds does nothing to an undead and Blight does
// nothing to an undead and makes a plant save with disadvantage. The slot is spent and a player reads only that nothing
// happened (or the save, with the roll's mode out of his reach): the reason in the app's words, the type behind it and the
// source that names the plant are the master's alone. Caio's paladin casts Cure Wounds on the skeleton and Ana's wizard
// casts Blight on the skeleton and on the shrub; the master casts the same for them, and reads what they may not (the
// positive controls).

const (
	lifeMagicReason = "Sem efeito em construtos e mortos-vivos"
	plantSource     = "Criatura do tipo planta"
	typeSourceKind  = "ADVANTAGE_SOURCE_KIND_TARGET_CREATURE_TYPE"
)

// lifeSpell casts a spell of the table as p for the caster, at the target by label.
func (rt *rollsTable) lifeSpell(t *testing.T, p *person, caster, spell string, slot int32, target string) (*playv1.SpellTargetResult, reply) {
	t.Helper()
	res := must(p.combat.CastSpell(t.Context(), rq(&playv1.CastSpellRequest{
		CampaignId: rt.campaign, EncounterId: rt.encounter.GetId(), CasterId: rt.ids[caster], SpellKey: spell, Slot: &playv1.SpellSlot{Level: slot},
		Targets: []*playv1.SpellTarget{{CombatantId: rt.ids[target]}}, IdempotencyKey: newKey(),
	})))
	body, err := protojson.Marshal(res)
	if err != nil {
		t.Fatalf("marshal the cast: %v", err)
	}
	r := reply{status: 200, body: body, msg: res}
	rt.got.keep(p, r)
	if p != rt.master {
		for _, f := range rt.inspect(p, r, nil) {
			t.Errorf("%s reads the cast of %s: %s", p.name, spell, f)
		}
	}
	if got := res.GetCast().GetTargets(); len(got) != 1 {
		t.Fatalf("the cast of %s has %d targets, want 1", spell, len(got))
	}
	return res.GetCast().GetTargets()[0], r
}

// undoLast has the master take the last action back.
func (rt *rollsTable) undoLast(t *testing.T) {
	t.Helper()
	must(rt.master.combat.UndoLastAction(t.Context(), rq(&playv1.UndoLastActionRequest{
		CampaignId: rt.campaign, EncounterId: rt.encounter.GetId(), ExpectedEventId: rt.logOf(rt.master).GetUndoableEventId(), IdempotencyKey: newKey(),
	})))
}

func TestTheCreatureTypeOfASpellOfLifeIsTheMastersAlone(t *testing.T) {
	rt := newRollsTableWith(t, func(rt *rollsTable) {
		w, ctx := rt.world, rt.t.Context()
		toren, pens := w.toren, w.pens
		full := toren.GetSheet().GetFull()
		full.PreparedSpellKeys = []string{"spell:cure-wounds"}
		must(w.caio.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
			CampaignId: w.campaign, CharacterId: toren.GetId(), Revision: toren.GetRevision(), Name: toren.GetName(),
			Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: full}},
		})))
		wizard := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 14, Intelligence: 16, Wisdom: 10, Charisma: 10}, RaceKey: "race:human",
			Classes:        []*charactersv1.ClassLevel{{ClassKey: "class:wizard", Level: 7}},
			KnownSpellKeys: []string{"spell:blight"}, PreparedSpellKeys: []string{"spell:blight"}, WeaponKeys: []string{dagger},
		}}}
		must(w.ana.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
			CampaignId: w.campaign, CharacterId: pens.GetId(), Revision: pens.GetRevision(), Name: pens.GetName(), Sheet: wizard,
		})))
	})
	w, m, caio, ana := rt.world, rt.master, rt.caio, rt.ana
	rt.buildRows()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the table: %v", r)
		}
	}()
	for _, n := range []string{lifeMagicReason, plantSource, typeSourceKind} {
		w.secrets.add(&canary{needle: n, kind: "life-magic-type"})
	}
	rt.encounter = rt.encounterOf(m)
	for _, label := range []string{labelToren, labelPens, "Esqueleto", "Arbusto"} {
		w.secrets.id("life-magic-combatant", rt.ids[label], ana, caio)
	}
	must(m.combat.MoveCombatant(t.Context(), rq(&playv1.MoveCombatantRequest{
		CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), CombatantId: rt.ids[labelPens], IdempotencyKey: newKey(), Col: 6, Row: 5, Forced: true,
	})))

	// 1. Cure Wounds on the skeleton: Caio reads that nothing happened, with no reason and no heal to roll.
	rt.toTurn(labelToren)
	got, _ := rt.lifeSpell(t, caio, labelToren, "spell:cure-wounds", 1, "Esqueleto")
	if got.GetLastingEffect() != playv1.LastingEffectOutcome_LASTING_EFFECT_OUTCOME_NO_EFFECT || got.GetNoEffectReasonMaster() != "" || got.GetPendingDamageId() != "" {
		t.Errorf("Caio reads %v, want no effect with no reason and no heal", got)
	}
	rt.ask(t, "Cure Wounds on an undead")
	// The master casts it again for Toren and reads why.
	rt.undoLast(t)
	if got, _ := rt.lifeSpell(t, m, labelToren, "spell:cure-wounds", 1, "Esqueleto"); got.GetNoEffectReasonMaster() != lifeMagicReason {
		t.Errorf("the master reads %q, want %q (the positive control)", got.GetNoEffectReasonMaster(), lifeMagicReason)
	}
	rt.ask(t, "the master cast Cure Wounds on an undead")

	// 2. Blight on the skeleton: no save, no damage, and the same words.
	rt.toTurn(labelPens)
	got, _ = rt.lifeSpell(t, ana, labelPens, "spell:blight", 4, "Esqueleto")
	if got.GetLastingEffect() != playv1.LastingEffectOutcome_LASTING_EFFECT_OUTCOME_NO_EFFECT || got.GetNoEffectReasonMaster() != "" || got.GetSave() != nil || got.GetPendingDamageId() != "" {
		t.Errorf("Ana reads %v, want no effect with no reason, no save and no damage", got)
	}
	rt.ask(t, "Blight on an undead")
	rt.undoLast(t)

	// 3. Blight on the shrub: the master reads the roll with disadvantage and why; Ana reads the save's outcome and nothing of its
	// mode, its sources or the type.
	got, _ = rt.lifeSpell(t, m, labelPens, "spell:blight", 4, "Arbusto")
	if got.GetSave().GetMode() != playv1.RollMode_ROLL_MODE_DISADVANTAGE || !hasSource(got.GetSave().GetSources(), playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_TARGET_CREATURE_TYPE) {
		t.Fatalf("the master reads the shrub's save as %v, want disadvantage with the type among its sources (the positive control)", got.GetSave())
	}
	rt.ask(t, "the master cast Blight on a plant")
	rt.undoLast(t)
	got, _ = rt.lifeSpell(t, ana, labelPens, "spell:blight", 4, "Arbusto")
	if got.GetSave() == nil || got.GetSave().GetMode() != playv1.RollMode_ROLL_MODE_UNSPECIFIED || len(got.GetSave().GetSources()) != 0 || got.GetSave().GetRoll() != nil {
		t.Errorf("Ana reads the shrub's save as %v, want its outcome alone: no roll, no mode, no source", got.GetSave())
	}
	rt.ask(t, "Blight on a plant")
	for _, p := range []*person{ana, caio} {
		for what, body := range map[string]string{"the combat": protojson.Format(rt.encounterOf(p)), "the log": protojson.Format(rt.logOf(p))} {
			for _, word := range []string{lifeMagicReason, plantSource, typeSourceKind, "construtos"} {
				if strings.Contains(body, word) {
					t.Errorf("%s reads %q in %s", p.name, word, what)
				}
			}
		}
	}
	t.Run("canaries are reachable", func(t *testing.T) { checkCanariesAreReachable(t, w, rt.got) })
}
