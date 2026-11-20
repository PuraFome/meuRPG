package advantage

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

func TestModeKeyRoundTripsAndReadsUnknownAsNormal(t *testing.T) {
	for _, m := range []combat.RollMode{combat.ModeNormal, combat.ModeAdvantage, combat.ModeDisadvantage} {
		if got := ModeOfKey(ModeKey(m)); got != m {
			t.Errorf("ModeOfKey(ModeKey(%v)) = %v", m, got)
		}
	}
	if got := ModeKey(combat.ModeAdvantage); got != "advantage" {
		t.Errorf("ModeKey(advantage) = %q: the database keeps this word", got)
	}
	if got := ModeOfKey("anything else"); got != combat.ModeNormal {
		t.Errorf("ModeOfKey(unknown) = %v, want normal", got)
	}
}

func TestModeFromProtoRefusesUnspecifiedAndMatchesTheMap(t *testing.T) {
	if _, ok := ModeFromProto(playv1.RollMode_ROLL_MODE_UNSPECIFIED); ok {
		t.Error("UNSPECIFIED must not read as a mode")
	}
	for m, p := range ModeToProto {
		if got, ok := ModeFromProto(p); !ok || got != m {
			t.Errorf("ModeFromProto(%v) = %v, %v, want %v", p, got, ok, m)
		}
	}
}

func TestSourceProtoHidesWhatTheViewerMayNotKnow(t *testing.T) {
	names := func(key string) string { return key }
	ctx := TextContext{Self: true, Names: names}
	sun := combat.Source{Kind: combat.SourceSunlightAttack, Effect: combat.ModeDisadvantage, Condition: combat.TraitSunlightWeakness}
	if got := SourceProto(sun, ctx, nil); got.GetTextPt() != "Luz do sol (Fraqueza à Luz do Sol)" {
		t.Errorf("the master reads %q", got.GetTextPt())
	}
	hidden := SourceProto(sun, ctx, HideUnlessMaster(false))
	if hidden.GetTextPt() != "Outra fonte: desvantagem" || hidden.GetKind() != playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_OTHER_SOURCE {
		t.Errorf("a player reads %q (%v), want the anonymous source", hidden.GetTextPt(), hidden.GetKind())
	}
	if shown := ShownSources([]combat.Source{sun}, names, true); len(shown) != 1 || shown[0].GetTextPt() != "Luz do sol (Fraqueza à Luz do Sol)" {
		t.Errorf("ShownSources for the master = %v", shown)
	}
	if shown := ShownSources([]combat.Source{sun}, names, false); shown[0].GetTextPt() != "Outra fonte: desvantagem" {
		t.Errorf("ShownSources for a player = %v", shown)
	}
}

// The roll that rests on the target's creature type (Blight on a plant) names the type to the master alone: a player reads the
// mode with "Outra fonte" and never the type (RN-10).
func TestSourceOfTheTargetCreatureTypeIsTheMastersAlone(t *testing.T) {
	names := func(key string) string { return key }
	ctx := TextContext{Self: true, Names: names}
	plant := combat.Source{Kind: combat.SourceTargetCreatureType, Effect: combat.ModeDisadvantage, Condition: "plant"}
	master := SourceProto(plant, ctx, HideUnlessMaster(true))
	if master.GetTextPt() != "Criatura do tipo planta: desvantagem no teste de resistência" || master.GetKind() != playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_TARGET_CREATURE_TYPE {
		t.Errorf("the master reads %q (%v)", master.GetTextPt(), master.GetKind())
	}
	player := SourceProto(plant, ctx, HideUnlessMaster(false))
	if player.GetTextPt() != "Outra fonte: desvantagem" || player.GetKind() != playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_OTHER_SOURCE || player.GetEffect() != playv1.RollMode_ROLL_MODE_DISADVANTAGE {
		t.Errorf("a player reads %q (%v), want the anonymous source with the same mode", player.GetTextPt(), player.GetKind())
	}
	if got := (TextContext{Names: names}).Sentence(combat.Source{Kind: combat.SourceTargetCreatureType, Effect: combat.ModeAdvantage, Condition: "undead"}); got != "Criatura do tipo morto-vivo: vantagem no teste de resistência" {
		t.Errorf("the sentence of an advantage for an undead = %q", got)
	}
}
