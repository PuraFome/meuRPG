package advantage

import (
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// ShownSources writes the sources of a saving throw or an ability check. The master reads the sun as itself; a player reads
// "Outra fonte" for it.
func ShownSources(sources []combat.Source, names func(string) string, master bool) []*playv1.AdvantageSource {
	return ShownSourcesHiding(sources, names, HideUnlessMaster(master))
}

// ShownSourcesHiding is ShownSources with the sources of an effect the master hides turned
// into "Outra fonte" (RN-10): hide says which.
func ShownSourcesHiding(sources []combat.Source, names func(string) string, hide func(combat.Source) bool) []*playv1.AdvantageSource {
	ctx := TextContext{Self: true, Names: names}
	var out []*playv1.AdvantageSource
	for _, src := range sources {
		out = append(out, SourceProto(src, ctx, hide))
	}
	return out
}

// SourceProto writes one source; one the viewer may not know the origin of reads
// "Outra fonte" with the effect and no key.
func SourceProto(src combat.Source, ctx TextContext, hide func(combat.Source) bool) *playv1.AdvantageSource {
	if hide != nil && hide(src) {
		text := "Outra fonte: vantagem"
		if src.Effect == combat.ModeDisadvantage {
			text = "Outra fonte: desvantagem"
		}
		return &playv1.AdvantageSource{Kind: playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_OTHER_SOURCE, Effect: ModeToProto[src.Effect], TextPt: text}
	}
	return &playv1.AdvantageSource{Kind: sourceKindToProto[src.Kind], Effect: ModeToProto[src.Effect], TextPt: ctx.Sentence(src)}
}
