package leaktest

import (
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
