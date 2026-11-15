package advantage

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// Characterization of the sentences: each source of an attack roll, a save or a check is frozen as the code wrote it.

func TestAdvantageSentencesFreezeEachSource(t *testing.T) {
	names := func(key string) string { return "<" + key + ">" }
	third := TextContext{Names: names, Target: "Orc", AllyLabel: "Lobo", Helper: "Mira"}
	self := TextContext{Self: true, Names: names, Target: "Orc", AllyLabel: "Lobo", Helper: "Mira"}
	bare := TextContext{Names: names}
	prone := "condition:prone"
	cases := []struct {
		name string
		ctx  TextContext
		src  combat.Source
		want string
	}{
		{"prone target, near", third, combat.Source{Kind: combat.SourceProneTarget, Effect: combat.ModeAdvantage, Condition: prone}, "Alvo <condition:prone> a 1,5 m: vantagem"},
		{"prone target, far", third, combat.Source{Kind: combat.SourceProneTarget, Effect: combat.ModeDisadvantage, Condition: prone}, "Alvo <condition:prone> a mais de 1,5 m: desvantagem"},
		{"prone attacker, third person", third, combat.Source{Kind: combat.SourceProneAttacker, Condition: prone}, "O atacante está <condition:prone>: desvantagem"},
		{"restrained attacker, reader rolls", self, combat.Source{Kind: combat.SourceRestrainedAttacker, Condition: "condition:restrained"}, "Você está <condition:restrained>: desvantagem"},
		{"blinded attacker", third, combat.Source{Kind: combat.SourceBlindedAttacker, Condition: "condition:blinded"}, "O atacante está <condition:blinded>: desvantagem"},
		{"poisoned attacker", third, combat.Source{Kind: combat.SourcePoisonedAttacker, Condition: "condition:poisoned"}, "O atacante está <condition:poisoned>: desvantagem"},
		{"frightened attacker", third, combat.Source{Kind: combat.SourceFrightenedAttacker, Condition: "condition:frightened"}, "O atacante está <condition:frightened>: desvantagem enquanto a fonte do medo está à vista"},
		{"invisible attacker", self, combat.Source{Kind: combat.SourceInvisibleAttacker, Condition: "condition:invisible"}, "Você está <condition:invisible>: vantagem"},
		{"restrained target", third, combat.Source{Kind: combat.SourceRestrainedTarget, Condition: "condition:restrained"}, "Alvo <condition:restrained>: vantagem"},
		{"blinded target", third, combat.Source{Kind: combat.SourceBlindedTarget, Condition: "condition:blinded"}, "Alvo <condition:blinded>: vantagem"},
		{"stunned target", third, combat.Source{Kind: combat.SourceStunnedTarget, Condition: "condition:stunned"}, "Alvo <condition:stunned>: vantagem"},
		{"paralyzed target", third, combat.Source{Kind: combat.SourceParalyzedTarget, Condition: "condition:paralyzed"}, "Alvo <condition:paralyzed>: vantagem"},
		{"unconscious target", third, combat.Source{Kind: combat.SourceUnconsciousTarget, Condition: "condition:unconscious"}, "Alvo <condition:unconscious>: vantagem"},
		{"petrified target", third, combat.Source{Kind: combat.SourcePetrifiedTarget, Condition: "condition:petrified"}, "Alvo <condition:petrified>: vantagem"},
		{"invisible target", third, combat.Source{Kind: combat.SourceInvisibleTarget, Condition: "condition:invisible"}, "Alvo <condition:invisible>: desvantagem"},
		{"reckless attacker", third, combat.Source{Kind: combat.SourceRecklessAttacker}, "<feature:reckless-attack>: vantagem nos ataques corpo a corpo com Força neste turno"},
		{"reckless target", third, combat.Source{Kind: combat.SourceRecklessTarget}, "Alvo em <feature:reckless-attack>: vantagem nos ataques contra ele"},
		{"dodging target", third, combat.Source{Kind: combat.SourceDodgingTarget}, "Alvo Esquivando e vendo o atacante: desvantagem"},
		{"magic resistance", self, combat.Source{Kind: combat.SourceMagicResistance}, "Resistência à Magia: vantagem em testes de resistência contra magias e outros efeitos mágicos"},
		{"sunlight attack of a Sensitivity", third, combat.Source{Kind: combat.SourceSunlightAttack, Condition: combat.TraitSunlightSensitivity}, "Luz do sol (Sensibilidade à Luz do Sol)"},
		{"sunlight attack of a Weakness", third, combat.Source{Kind: combat.SourceSunlightAttack, Condition: combat.TraitSunlightWeakness}, "Luz do sol (Fraqueza à Luz do Sol)"},
		{"the Sensitivity's Perception names the sight", third, combat.Source{Kind: combat.SourceSunlightPerception, Condition: combat.TraitSunlightSensitivity}, "Luz do sol, nos testes que dependem da visão (Sensibilidade à Luz do Sol)"},
		{"the Weakness's checks", third, combat.Source{Kind: combat.SourceSunlightPerception, Condition: combat.TraitSunlightWeakness}, "Luz do sol (Fraqueza à Luz do Sol)"},
		{"sunlight save", third, combat.Source{Kind: combat.SourceSunlightSave, Condition: combat.TraitSunlightWeakness}, "Luz do sol (Fraqueza à Luz do Sol)"},
		{"pack tactics names the ally to the master", third, combat.Source{Kind: combat.SourcePackTactics}, "Táticas de Matilha: um aliado (Lobo) está a 1,5 m de Orc e não está incapacitado"},
		{"pack tactics hides the ally from the roller", self, combat.Source{Kind: combat.SourcePackTactics}, "Táticas de Matilha: um aliado está a 1,5 m do alvo e não está incapacitado"},
		{"pack tactics without an ally label", bare, combat.Source{Kind: combat.SourcePackTactics}, "Táticas de Matilha: um aliado está a 1,5 m do alvo e não está incapacitado"},
		{"help names the helper", third, combat.Source{Kind: combat.SourceHelp}, "Ajuda de Mira: vantagem"},
		{"help of an unnamed ally", bare, combat.Source{Kind: combat.SourceHelp}, "Ajuda de um aliado: vantagem"},
		{"unseen attacker", third, combat.Source{Kind: combat.SourceUnseenAttacker}, "Atacante que o alvo não vê: vantagem"},
		{"unseen target", third, combat.Source{Kind: combat.SourceUnseenTarget}, "Alvo que o atacante não vê: desvantagem"},
		{"long range", third, combat.Source{Kind: combat.SourceLongRange}, "Além do alcance normal: desvantagem"},
		{"hostile nearby", third, combat.Source{Kind: combat.SourceHostileNearby}, "Um inimigo a 1,5 m de quem ataca à distância: desvantagem"},
		{"effect attack names the effect", third, combat.Source{Kind: combat.SourceEffectAttack, Condition: "spell:vicious-mockery"}, "<spell:vicious-mockery>: desvantagem neste ataque"},
		{"a save source falls to the other sentences", third, combat.Source{Kind: combat.SourceDodgingSave}, "Esquivando: vantagem em testes de resistência de Destreza"},
		{"a check source falls to the other sentences", third, combat.Source{Kind: combat.SourcePerceptionDim}, "Você vê o chão em penumbra: desvantagem na Percepção"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.ctx.Sentence(c.src); got != c.want {
				t.Errorf("sentence = %q, want %q", got, c.want)
			}
		})
	}
}
