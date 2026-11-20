package advantage

import (
	"fmt"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// TextContext is who reads a sentence and what it names.
type TextContext struct {
	// Self says the reader plays the roller ("Você está Envenenado"); else a third
	// person ("Atacante Envenenado").
	Self  bool
	Names func(string) string
	// Attacker, Target and AllyLabel are the labels a sentence may name; the master
	// reads them, a player only the target they chose.
	Attacker, Target, AllyLabel string
	// Helper is the label of the creature whose Help holds.
	Helper string
}

// subject is "Você" or "O atacante".
func (c TextContext) subject() string {
	if c.Self {
		return "Você está"
	}
	return "O atacante está"
}

func (c TextContext) cond(key string) string { return c.Names(key) }

// conditionSentence names a condition source as "<subject> <condition>: <effect>".
func (c TextContext) conditionSentence(key, effect string) string {
	return fmt.Sprintf("%s %s: %s", c.subject(), c.cond(key), effect)
}

// ---- the sentences ----

// Sentence writes a source in Portuguese: an attack roll's here, the others in otherSentence.
func (c TextContext) Sentence(src combat.Source) string {
	if text, ok := sunlightSentence(src); ok {
		return text
	}
	if text, ok := c.attackerSentence(src); ok {
		return text
	}
	if text, ok := c.targetSentence(src); ok {
		return text
	}
	if text, ok := c.situationSentence(src); ok {
		return text
	}
	return c.otherSentence(src)
}

// The sunlight's sources are the master's (RN-20, RN-29): the sentence names the sun and the trait that makes it
// count, and a player is never given it (HidesSunlight).
const (
	sunlightSensitivityName = "Sensibilidade à Luz do Sol"
	sunlightWeaknessName    = "Fraqueza à Luz do Sol"
)

// sunlightSentence writes a sunlight source, false for any other kind.
func sunlightSentence(src combat.Source) (string, bool) {
	trait := sunlightSensitivityName
	if src.Condition == combat.TraitSunlightWeakness {
		trait = sunlightWeaknessName
	}
	switch src.Kind {
	case combat.SourceSunlightAttack, combat.SourceSunlightSave:
		return fmt.Sprintf("Luz do sol (%s)", trait), true
	case combat.SourceSunlightPerception:
		if src.Condition == combat.TraitSunlightWeakness {
			return fmt.Sprintf("Luz do sol (%s)", trait), true
		}
		return fmt.Sprintf("Luz do sol, nos testes que dependem da visão (%s)", trait), true
	}
	return "", false
}

// IsSunlight says the source is one of the sun's.
func IsSunlight(src combat.Source) bool {
	switch src.Kind {
	case combat.SourceSunlightAttack, combat.SourceSunlightPerception, combat.SourceSunlightSave:
		return true
	}
	return false
}

// HidesSunlight is the hide of whoever is not the master: the sun and the trait behind it are the master's, so a
// player reads "Outra fonte" for them, on their own roll too.
func HidesSunlight(src combat.Source) bool { return IsSunlight(src) }

// HidesFromPlayers is the hide of whoever is not the master: the sun's sources and the one that rests on the target's
// creature type (RN-10: a player learns nothing of an enemy's type), which a player reads as "Outra fonte".
func HidesFromPlayers(src combat.Source) bool {
	return IsSunlight(src) || src.Kind == combat.SourceTargetCreatureType
}

// HideUnlessMaster is HidesFromPlayers for a viewer: nil for the master, who reads everything.
func HideUnlessMaster(master bool) func(combat.Source) bool {
	if master {
		return nil
	}
	return HidesFromPlayers
}

// attackerSentence writes a source that comes from the one who attacks.
func (c TextContext) attackerSentence(src combat.Source) (string, bool) {
	switch src.Kind {
	case combat.SourceProneAttacker, combat.SourceRestrainedAttacker, combat.SourceBlindedAttacker, combat.SourcePoisonedAttacker:
		return c.conditionSentence(src.Condition, "desvantagem"), true
	case combat.SourceFrightenedAttacker:
		return c.conditionSentence(src.Condition, "desvantagem enquanto a fonte do medo está à vista"), true
	case combat.SourceInvisibleAttacker:
		return c.conditionSentence(src.Condition, "vantagem"), true
	case combat.SourceRecklessAttacker:
		return fmt.Sprintf("%s: vantagem nos ataques corpo a corpo com Força neste turno", c.Names("feature:reckless-attack")), true
	}
	return "", false
}

// targetSentence writes a source that comes from the one who is attacked.
func (c TextContext) targetSentence(src combat.Source) (string, bool) {
	switch src.Kind {
	case combat.SourceProneTarget:
		if src.Effect == combat.ModeAdvantage {
			return fmt.Sprintf("Alvo %s a 1,5 m: vantagem", c.cond(src.Condition)), true
		}
		return fmt.Sprintf("Alvo %s a mais de 1,5 m: desvantagem", c.cond(src.Condition)), true
	case combat.SourceRestrainedTarget, combat.SourceBlindedTarget, combat.SourceStunnedTarget,
		combat.SourceParalyzedTarget, combat.SourceUnconsciousTarget, combat.SourcePetrifiedTarget:
		return fmt.Sprintf("Alvo %s: vantagem", c.cond(src.Condition)), true
	case combat.SourceInvisibleTarget:
		return fmt.Sprintf("Alvo %s: desvantagem", c.cond(src.Condition)), true
	case combat.SourceRecklessTarget:
		return fmt.Sprintf("Alvo em %s: vantagem nos ataques contra ele", c.Names("feature:reckless-attack")), true
	case combat.SourceDodgingTarget:
		return "Alvo Esquivando e vendo o atacante: desvantagem", true
	}
	return "", false
}

// situationSentence writes a source that comes from the place of the attack: allies, sight,
// range and enemies nearby.
func (c TextContext) situationSentence(src combat.Source) (string, bool) {
	switch src.Kind {
	case combat.SourcePackTactics:
		if c.AllyLabel != "" && !c.Self {
			return fmt.Sprintf("Táticas de Matilha: um aliado (%s) está a 1,5 m de %s e não está incapacitado", c.AllyLabel, c.Target), true
		}
		return "Táticas de Matilha: um aliado está a 1,5 m do alvo e não está incapacitado", true
	case combat.SourceHelp:
		if c.Helper != "" {
			return fmt.Sprintf("Ajuda de %s: vantagem", c.Helper), true
		}
		return "Ajuda de um aliado: vantagem", true
	case combat.SourceUnseenAttacker:
		return "Atacante que o alvo não vê: vantagem", true
	case combat.SourceUnseenTarget:
		return "Alvo que o atacante não vê: desvantagem", true
	case combat.SourceLongRange:
		return "Além do alcance normal: desvantagem", true
	case combat.SourceHostileNearby:
		return "Um inimigo a 1,5 m de quem ataca à distância: desvantagem", true
	case combat.SourceEffectAttack:
		if src.Effect == combat.ModeAdvantage {
			return fmt.Sprintf("%s: vantagem neste ataque", c.Names(src.Condition)), true
		}
		return fmt.Sprintf("%s: desvantagem neste ataque", c.Names(src.Condition)), true
	}
	return "", false
}

// staticSentences are the sources of a saving throw, a check or an effect whose sentence names nothing.
var staticSentences = map[string]string{
	combat.SourceDodgingSave:      "Esquivando: vantagem em testes de resistência de Destreza",
	combat.SourceRageStrength:     "Em fúria: vantagem em testes e testes de resistência de Força",
	combat.SourceMagicResistance:  "Resistência à Magia: vantagem em testes de resistência contra magias e outros efeitos mágicos",
	combat.SourcePerceptionDim:    "Você vê o chão em penumbra: desvantagem na Percepção",
	combat.SourceExhaustionAttack: "Exaustão: desvantagem em jogadas de ataque",
	combat.SourceExhaustionSave:   "Exaustão: desvantagem em testes de resistência",
	combat.SourceExhaustionCheck:  "Exaustão: desvantagem em testes de habilidade",
	combat.SourceStealthArmor:     "Armadura: desvantagem em Furtividade",
	combat.SourceArmorUntrained:   LabelArmorUntrained + ": desvantagem em testes, ataques e testes de resistência de Força e Destreza",
	combat.SourceOutlinedTarget:   "Alvo delineado: vantagem se o atacante o vê",
	combat.SourceFavor:            LabelFavor,
}

// otherSentence writes, in Portuguese, a source of a saving throw or an ability check, or one an
// effect that lasts gives.
func (c TextContext) otherSentence(src combat.Source) string {
	if text, ok := staticSentences[src.Kind]; ok {
		return text
	}
	switch src.Kind {
	case combat.SourceDangerSense:
		return fmt.Sprintf("%s: vantagem em testes de resistência de Destreza contra efeitos que você vê", c.Names("feature:danger-sense"))
	case combat.SourceEffectSave:
		return "Efeito ativo: " + modeWord(src.Effect) + " no teste de resistência"
	case combat.SourceEffectCheck:
		return "Efeito ativo: " + modeWord(src.Effect) + " em testes de habilidade"
	case combat.SourceRestrainedSave:
		return fmt.Sprintf("%s: desvantagem em testes de resistência de Destreza", c.cond(src.Condition))
	case combat.SourcePoisonedCheck, combat.SourceFrightenedCheck:
		return fmt.Sprintf("%s: desvantagem em testes de habilidade", c.cond(src.Condition))
	case combat.SourceTargetCreatureType:
		return fmt.Sprintf("Criatura do tipo %s: %s no teste de resistência", creatureTypePT(src.Condition), modeWord(src.Effect))
	case combat.SourceTaggedSave:
		mode := "vantagem"
		if src.Effect == combat.ModeDisadvantage {
			mode = "desvantagem"
		}
		return fmt.Sprintf("%s: %s no teste de resistência %s", c.Names(src.Condition), mode, c.againstSentence(src.Against))
	}
	return ""
}

// creatureTypesPT are the SRD creature types in Portuguese, as the master reads them.
var creatureTypesPT = map[string]string{
	"aberration": "aberração", "beast": "fera", "celestial": "celestial", "construct": "construto", "dragon": "dragão",
	"elemental": "elemental", "fey": "fada", "fiend": "ínfero", "giant": "gigante", "humanoid": "humanoide",
	"monstrosity": "monstruosidade", "ooze": "gosma", "plant": "planta", "undead": "morto-vivo",
}

// creatureTypePT is the Portuguese name of an SRD creature type; the key itself for one the app does not know.
func creatureTypePT(key string) string {
	if pt, ok := creatureTypesPT[key]; ok {
		return pt
	}
	return key
}

// modeWord is "vantagem" or "desvantagem".
func modeWord(m combat.RollMode) string {
	if m == combat.ModeDisadvantage {
		return "desvantagem"
	}
	return "vantagem"
}

// againstSentence is what a saving throw with a racial advantage is against, in Portuguese.
func (c TextContext) againstSentence(against string) string {
	if strings.HasPrefix(against, "condition:") {
		return "contra ficar " + strings.ToLower(c.cond(against))
	}
	switch against {
	case combat.AgainstFrightened:
		return "contra ficar amedrontado"
	case combat.AgainstCharmed:
		return "contra ficar enfeitiçado"
	case combat.AgainstPoison:
		return "contra veneno"
	}
	return "contra magia"
}

// The labels that name a source of a roll in the log and in the sentences.
const (
	// LabelFavor names the master's Inspiration.
	LabelFavor = "Inspiração"
	// LabelArmorUntrained names the armor or shield the character is not proficient with.
	LabelArmorUntrained = "Armadura sem proficiência"
)
