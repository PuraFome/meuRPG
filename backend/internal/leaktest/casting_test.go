package leaktest

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// buildCasting makes the NPC that casts outside a combat while the stage does not show it: its
// name, its id and its cast are the master's alone (RN-10). It runs before the combat starts, as a
// character in a combat casts there.
func (w *world) buildCasting() {
	ctx := w.t.Context()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, RaceKey: "race:human",
		Classes:        []*charactersv1.ClassLevel{{ClassKey: "class:wizard", Level: 3}},
		KnownSpellKeys: []string{"spell:mage-armor"}, PreparedSpellKeys: []string{"spell:mage-armor"},
	}}}
	w.casterNPC = must(w.master.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, Name: w.secrets.marker("hidden-caster-name"), Sheet: sheet,
	}))).GetCharacter()
	w.secrets.id("hidden-caster", w.casterNPC.GetId())
	cast := must(w.master.casting.CastSpellOutsideCombat(ctx, rq(&playv1.CastSpellOutsideCombatRequest{
		CampaignId: w.campaign, CasterCharacterId: w.casterNPC.GetId(), SpellKey: "spell:mage-armor", Slot: &playv1.SpellSlot{Level: 1},
		TargetIds: []string{w.toren.GetId()}, IdempotencyKey: newKey(),
	}))).GetCast()
	w.hiddenCast = w.secrets.id("hidden-cast", cast.GetId())
}

// buildHealing puts a Life cleric on the stage and has it heal Caio's character: the cast is public (the NPC is
// shown), but what it did to the target, the Disciple of Life's extra hit points among it, is the target's own and
// the master's (RN-20). The extra is a small number, so it is searched only in the fields that name it.
func (w *world) buildHealing() {
	ctx := w.t.Context()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 10, Constitution: 12, Intelligence: 10, Wisdom: 16, Charisma: 10}, RaceKey: "race:human",
		Classes:           []*charactersv1.ClassLevel{{ClassKey: "class:cleric", Level: 3, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: "subclass:life"}}},
		PreparedSpellKeys: []string{"spell:cure-wounds"},
	}}}
	w.healer = must(w.master.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, Name: w.secrets.public("healer-name"), Sheet: sheet,
	}))).GetCharacter()
	must(w.master.play.PutOnStage(ctx, rq(&playv1.PutOnStageRequest{CampaignId: w.campaign, CharacterId: w.healer.GetId()})))
	const slot = 2
	w.disciple = w.secrets.number("disciple-extra-hit-points", 2+slot, "extra")
	w.healCast = must(w.master.casting.CastSpellOutsideCombat(ctx, rq(&playv1.CastSpellOutsideCombatRequest{
		CampaignId: w.campaign, CasterCharacterId: w.healer.GetId(), SpellKey: "spell:cure-wounds", Slot: &playv1.SpellSlot{Level: slot},
		TargetIds: []string{w.toren.GetId()}, IdempotencyKey: newKey(),
		Roll: &playv1.CastSpellOutsideCombatRequest_RollInApp{RollInApp: true},
	}))).GetCast()
	w.secrets.allowNeedle("4", w.caio)
}

// TestAHealingCastShowsItsEffectOnlyToTheTargetAndTheMaster: the party reads that the Life cleric
// healed Caio's character, but what the cast did to the target (the effect, the amount, the Disciple of
// Life's extra hit points) is the target's player's and the master's (RN-20). Ana, whose character was
// not healed, reads none of it; Caio and the master are the positive control.
func TestAHealingCastShowsItsEffectOnlyToTheTargetAndTheMaster(t *testing.T) {
	w := newWorld(t)
	read := func(p *person) *playv1.OutsideCastTarget {
		for _, c := range must(p.casting.ListSpellCasts(t.Context(), rq(&playv1.ListSpellCastsRequest{CampaignId: w.campaign}))).GetLog() {
			if c.GetId() == w.healCast.GetId() {
				if len(c.GetTargets()) != 1 {
					t.Fatalf("%s reads %d targets, want 1", p.name, len(c.GetTargets()))
				}
				return c.GetTargets()[0]
			}
		}
		t.Fatalf("%s does not read the healing cast", p.name)
		return nil
	}
	for _, p := range []*person{w.master, w.caio} {
		got := read(p)
		if int64(got.GetExtraHitPoints()) != w.disciple || got.GetEffect() != playv1.CastEffect_CAST_EFFECT_HEAL {
			t.Errorf("%s reads effect %v and extra %d, want a heal with %d", p.name, got.GetEffect(), got.GetExtraHitPoints(), w.disciple)
		}
	}
	got := read(w.ana)
	if got.GetExtraHitPoints() != 0 || got.GetEffect() != playv1.CastEffect_CAST_EFFECT_UNSPECIFIED || got.GetAmount() != 0 {
		t.Errorf("Ana reads effect %v, amount %d and extra %d of a healing that was not hers", got.GetEffect(), got.GetAmount(), got.GetExtraHitPoints())
	}
}
