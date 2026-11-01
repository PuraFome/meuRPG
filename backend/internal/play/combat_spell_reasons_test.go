package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A disabled spell says why, naming the spell (#643). These tests need the database (MEURPG_TEST_DATABASE_URL).

// reasonOf is the reason of the spell's line (or the spell attack's) as the response carries it.
func reasonOf(t *testing.T, res *playv1.GetTurnOptionsResponse, key string) *rulesv1.DisabledReason {
	t.Helper()
	for _, sp := range res.GetOptions().GetSpells() {
		if sp.GetSpell().GetKey() == key {
			return sp.GetReason()
		}
	}
	for _, at := range res.GetOptions().GetAttacks() {
		if at.GetAttack().GetKey() == key {
			return at.GetReason()
		}
	}
	t.Fatalf("no spell or attack %s among the options", key)
	return nil
}

// wantSameReasonText checks the master and the player read the one sentence: no text only for the master.
func wantSameReasonText(t *testing.T, a *armed, player *user, e *playv1.Encounter, label, key string, code rulesv1.DisabledReasonCode, want string) {
	t.Helper()
	for who, u := range map[string]*user{"the player": player, "the master": a.master} {
		r := reasonOf(t, a.mustOptions(t, u, e, label), key)
		if r.GetCode() != code || r.GetTextPt() != want || r.GetTextMasterPt() != "" {
			t.Errorf("%s reads %s of %s as %v, want code %v with the text %q and no master text", who, label, key, r, code, want)
		}
	}
}

// TestSpellReasonsNameTheSpell: every reason a spell gets, but NO_SLOT, carries a sentence with the
// spell's name, the same for the master and for the player. NO_SLOT keeps the short word of the screen.
func TestSpellReasonsNameTheSpell(t *testing.T) {
	t.Parallel()
	code := rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_REACTION_ONLY_WHEN_HIT
	t.Run("REACTION_ONLY_WHEN_HIT", func(t *testing.T) {
		t.Parallel()
		a := newCasters(t)
		e := a.castersFight(t, 1)
		wantSameReasonText(t, a, a.ana, e, "Pensantus", shieldSpell, code, "Escudo Arcano é uma reação: o app pergunta quando um ataque acerta você.")
	})
	t.Run("REACTION_ONLY", func(t *testing.T) {
		t.Parallel()
		c := newCave(t)
		e := c.fight(t)
		c.passTo(t, e, "Pensantus")
		wantSameReasonText(t, c.armed, c.ana, e, "Pensantus", featherFallSpell, rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_REACTION_ONLY, "Queda Suave é uma reação: o app pergunta quando o gatilho acontece.")
	})
	t.Run("CASTING_TIME_TOO_LONG", func(t *testing.T) {
		t.Parallel()
		a := newSummoners(t)
		e := a.summonersFight(t)
		wantSameReasonText(t, a, a.ana, e, "Pensantus", findFamiliar, rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_CASTING_TIME_TOO_LONG, "Convocar Familiar demora demais para conjurar em um combate.")
	})
	t.Run("BONUS_ACTION_SPELL_LIMIT", func(t *testing.T) {
		t.Parallel()
		limit := rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_BONUS_ACTION_SPELL_LIMIT
		a := newCasters(t)
		e := a.passTo(t, a.castersFight(t, 1), "Brisa")
		a.mustCast(t, a.bia, e, "Brisa", healingWord, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)
		wantSameReasonText(t, a, a.bia, e, "Brisa", cureWounds, limit, "Você já conjurou uma magia de ação bônus neste turno: Curar Ferimentos não cabe, só um truque de 1 ação.")
	})
	t.Run("BONUS_ACTION_SPELL_LIMIT after an action spell", func(t *testing.T) {
		t.Parallel()
		limit := rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_BONUS_ACTION_SPELL_LIMIT
		a := newCasters(t)
		e := a.passTo(t, a.castersFight(t, 1), "Brisa")
		a.mustCast(t, a.bia, e, "Brisa", cureWounds, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)
		wantSameReasonText(t, a, a.bia, e, "Brisa", healingWord, limit, "Você já conjurou outra magia neste turno: junto com Palavra Curativa, uma magia de ação bônus, só cabe um truque de 1 ação.")
	})
	t.Run("ARMOR_NOT_PROFICIENT", func(t *testing.T) {
		t.Parallel()
		a := newUntrainedTable(t)
		e := a.untrainedFight(t)
		armor := rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ARMOR_NOT_PROFICIENT
		wantSameReasonText(t, a, a.caio, e, "Toren", magicMissileSpell, armor, "Armadura ou escudo sem proficiência: Mísseis Mágicos não pode ser conjurada.")
		wantSameReasonText(t, a, a.caio, e, "Toren", fireBolt, armor, "Armadura ou escudo sem proficiência: Raio de Fogo não pode ser conjurada.")
	})
	t.Run("NO_SLOT keeps the short word", func(t *testing.T) {
		t.Parallel()
		a := newCasters(t)
		e := a.castersFight(t, 1)
		a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) {
			r.SpellSlotsUsed = []*playv1.SpellSlotsUsed{{Level: 1, Used: 4}, {Level: 2, Used: 2}}
		})
		for who, u := range map[string]*user{"the player": a.ana, "the master": a.master} {
			r := reasonOf(t, a.mustOptions(t, u, e, "Pensantus"), burningHands)
			if r.GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NO_SLOT || r.GetTextPt() != "" || r.GetTextMasterPt() != "" {
				t.Errorf("%s reads the spell with no slot as %v, want NO_SLOT with no text", who, r)
			}
		}
	})
}

// TestRN20_SpellReasonsNameNoOtherCharactersSpell: a player's GetTurnOptions is for their own characters;
// another character's options, with the names of its spells, are refused and never leak. An NPC caster's
// are the master's.
func TestRN20_SpellReasonsNameNoOtherCharactersSpell(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFight(t, 1)

	// Positive control: the owner reads her own spell names in the reason.
	own := protojson.Format(a.mustOptions(t, a.ana, e, "Pensantus"))
	if !strings.Contains(own, "Escudo Arcano é uma reação") {
		t.Fatalf("Pensantus's player reads her options as %s, want the Shield sentence", own)
	}
	// Canary: another player is refused, and what Toren's player reads of his own turn has no one else's spell.
	_, err := a.options(t, a.caio, e, "Pensantus")
	wantCode(t, "another player's GetTurnOptions", err, connect.CodePermissionDenied)
	_, err = a.options(t, a.bia, e, "Pensantus")
	wantCode(t, "a third player's GetTurnOptions", err, connect.CodePermissionDenied)
	if err != nil && (strings.Contains(err.Error(), "Escudo") || strings.Contains(err.Error(), "Mísseis")) {
		t.Errorf("the refusal names a spell: %v", err)
	}
	if mine := protojson.Format(a.mustOptions(t, a.caio, e, "Toren")); strings.Contains(mine, "Escudo Arcano") || strings.Contains(mine, "Mísseis Mágicos") {
		t.Errorf("Toren's player reads Pensantus's spells in his options: %s", mine)
	}

	// An NPC caster the master hid: its options are the master's alone.
	b := newCasters(t)
	wizard := b.hiddenWizard(t)
	eb := b.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: wizard.GetId()}}, npcRolls: []int{20},
		players: map[string]int32{"Pensantus": 10, "Toren": 9, "Brisa": 8},
	}) // the Mago Oculto keeps its first turn, hidden
	master := protojson.Format(b.mustOptions(t, b.master, eb, "Mago Oculto"))
	if !strings.Contains(master, "Escudo Arcano é uma reação") || !strings.Contains(master, "Convocar Familiar demora demais") {
		t.Errorf("the master reads the NPC caster's options as %s, want the sentences naming its spells", master)
	}
	_, err = b.options(t, b.ana, eb, "Mago Oculto")
	wantCode(t, "a player's GetTurnOptions for a hidden NPC", err, connect.CodeNotFound)
	if err != nil && strings.Contains(err.Error(), "Escudo") {
		t.Errorf("the refusal names the NPC's spell: %v", err)
	}
}

// hiddenWizard is an enemy wizard who knows Shield and Find Familiar.
func (a *armed) hiddenWizard(t *testing.T) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 12, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 10}, RaceKey: "race:human",
		Classes: []*charactersv1.ClassLevel{classLevel("class:wizard", 5, "")}, KnownSpellKeys: []string{shieldSpell, findFamiliar}, PreparedSpellKeys: []string{shieldSpell, findFamiliar},
	}}}
	res, err := a.master.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, Name: "Mago Oculto", Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(Mago Oculto) error = %v", err)
	}
	return res.Msg.GetCharacter()
}
