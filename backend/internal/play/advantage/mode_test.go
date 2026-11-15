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
