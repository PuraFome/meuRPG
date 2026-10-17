package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Finding fix/lasting-effects-audience: an audience the server does not know is the master's
// alone, never everyone's (RN-10). The enum value 99 stands for a client newer than the server.
const unknownAudience = playv1.EffectAudience(99)

func TestAnUnknownEffectAudienceIsTheMastersAlone(t *testing.T) {
	t.Parallel()
	for _, own := range []string{rules.EffectVisibilityPublic, rules.EffectVisibilityOwner} {
		spec := effectSpec{audience: chosenAudience(unknownAudience, own)}
		visible, audience := spec.visibilityOf()
		if visible || audience != audienceOwner {
			t.Errorf("an unknown audience over %q stores visible=%v audience=%q, want hidden from players and owner", own, visible, audience)
		}
	}
	// The known choices keep their meaning.
	for choice, want := range map[playv1.EffectAudience]string{
		playv1.EffectAudience_EFFECT_AUDIENCE_ALL:   "all",
		playv1.EffectAudience_EFFECT_AUDIENCE_OWNER: audienceOwner,
	} {
		if visible, audience := (effectSpec{audience: chosenAudience(choice, rules.EffectVisibilityOwner)}).visibilityOf(); !visible || audience != want {
			t.Errorf("%v stores visible=%v audience=%q, want visible and %q", choice, visible, audience, want)
		}
	}
	if visible, audience := (effectSpec{audience: chosenAudience(playv1.EffectAudience_EFFECT_AUDIENCE_UNSPECIFIED, rules.EffectVisibilityOwner)}).visibilityOf(); !visible || audience != audienceOwner {
		t.Errorf("no choice over an owner effect stores visible=%v audience=%q, want the effect's own", visible, audience)
	}
}

func TestAnUnknownVisibilityAudienceHidesTheEffectFromPlayers(t *testing.T) {
	t.Parallel()
	in, err := readVisibilityInput(&playv1.SetLastingEffectVisibilityRequest{PlayerVisible: true, Audience: unknownAudience})
	if err != nil {
		t.Fatalf("readVisibilityInput() error = %v", err)
	}
	if visible, audience := (effectSpec{audience: in.audience}).visibilityOf(); visible || audience != audienceOwner {
		t.Errorf("an unknown audience is stored as visible=%v audience=%q, want hidden from players and owner", visible, audience)
	}
}

func TestTheAudienceOnTheWireFailsClosed(t *testing.T) {
	t.Parallel()
	for stored, want := range map[string]playv1.EffectAudience{
		"all":     playv1.EffectAudience_EFFECT_AUDIENCE_ALL,
		"owner":   playv1.EffectAudience_EFFECT_AUDIENCE_OWNER,
		"":        playv1.EffectAudience_EFFECT_AUDIENCE_OWNER,
		"unknown": playv1.EffectAudience_EFFECT_AUDIENCE_OWNER,
	} {
		if got := audienceProto(stored); got != want {
			t.Errorf("audienceProto(%q) = %v, want %v", stored, got, want)
		}
	}
}
