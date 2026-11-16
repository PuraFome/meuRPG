package leaktest

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// holdsNeither fails when the answer holds the key or the text of the option no level offers.
func (w *world) holdsNeither(t *testing.T, what, body string) {
	t.Helper()
	for _, needle := range []string{w.unofferedKey, w.unofferedLabel} {
		if strings.Contains(body, needle) {
			t.Errorf("%s holds %q: %s", what, needle, shorten([]byte(body)))
		}
	}
}

// levelUpAnswers are the two ways of sending the key of an option no level offers.
func (w *world) levelUpAnswers() map[string]*charactersv1.LevelUpChoices {
	hp := &charactersv1.LevelUpHitPoints{Method: charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_AVERAGE}
	return map[string]*charactersv1.LevelUpChoices{
		"a pick": {ClassKey: "class:wizard", HitPoints: hp, FeatureChoiceKeys: []string{w.unofferedKey}},
		"a swap": {ClassKey: "class:wizard", HitPoints: hp, SwapFeatureOptions: []*charactersv1.LevelUpPickSwap{{Group: "feature:arcane-tradition", FromKey: "feature:arcane-tradition", ToKey: w.unofferedKey}}},
	}
}

// A level-up answers a key the level does not offer with the refusal alone: no sheet of afterwards and no sentence built from
// the key's entry, for the preview and for the level-up, whether the key is a pick or the option a swap takes. The master's
// preview reads as before (the positive control).
func TestALevelUpAnswersAKeyTheLevelDoesNotOfferWithTheRefusalAlone(t *testing.T) {
	w := newWorld(t)
	ctx := t.Context()
	for name, ch := range w.levelUpAnswers() {
		preview, err := w.ana.characters.PreviewLevelUp(ctx, rq(&charactersv1.PreviewLevelUpRequest{CampaignId: w.campaign, CharacterId: w.pens.GetId(), Choices: ch}))
		if err != nil {
			t.Fatalf("%s: preview error = %v", name, err)
		}
		res := preview.Msg
		if res.GetRefusal().GetField() == "" || res.GetAfter() != nil || res.GetMulticlassSummary() != nil {
			t.Errorf("%s: the preview answers refusal %v and a sheet %v, want the refusal alone", name, res.GetRefusal(), res.GetAfter() != nil)
		}
		w.holdsNeither(t, name+": the preview", protojson.Format(res))

		_, err = w.ana.characters.LevelUpCharacter(ctx, rq(&charactersv1.LevelUpCharacterRequest{CampaignId: w.campaign, CharacterId: w.pens.GetId(), Revision: w.pens.GetRevision(), Choices: ch}))
		if err == nil || connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("%s: the level-up answers %v, want a refusal", name, err)
			continue
		}
		answers := err.Error()
		for _, d := range connect.NewError(connect.CodeUnknown, err).Details() {
			if v, derr := d.Value(); derr == nil {
				answers += protojson.Format(v)
			}
		}
		w.holdsNeither(t, name+": the level-up", answers)
	}

	// The master reads the sheet of afterwards with the sentence of the key's entry.
	control, err := w.master.characters.PreviewLevelUp(ctx, rq(&charactersv1.PreviewLevelUpRequest{CampaignId: w.campaign, CharacterId: w.pens.GetId(), Choices: w.levelUpAnswers()["a pick"]}))
	if err != nil {
		t.Fatal(err)
	}
	if control.Msg.GetAfter() == nil || !strings.Contains(protojson.Format(control.Msg), w.unofferedLabel) {
		t.Errorf("the master's preview = %v, want the sheet of afterwards with the sentence of the entry", protojson.Format(control.Msg))
	}
}

// The sentence of a pick that no feature of the sheet offers is the same for a player whatever the pick's entry: the editor's
// preview, the choices preview and the stored sheet say "Uma opção não vale para este personagem." where the master reads the
// sentence of the entry.
func TestTheOfferedChoicesOfASheetAreToldTheSameToThePlayer(t *testing.T) {
	w := newWorld(t)
	ctx := t.Context()
	const generic = "Uma opção não vale para este personagem."
	cur := must(w.ana.characters.GetCharacter(ctx, rq(&charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.pens.GetId()}))).GetCharacter()
	sheet := proto.CloneOf(cur.GetSheet())
	sheet.GetFull().FeatureChoiceKeys = append(sheet.GetFull().FeatureChoiceKeys, w.unofferedKey)

	for who, p := range map[string]*person{"ana": w.ana, "master": w.master} {
		wantNamed := p == w.master
		preview, err := p.characters.PreviewCharacter(ctx, rq(&charactersv1.PreviewCharacterRequest{
			CampaignId: w.campaign, Kind: cur.GetKind(), Sheet: sheet, ForPlayer: wantNamed,
		}))
		if err != nil {
			t.Fatalf("%s: the editor's preview answers %v", who, err)
		}
		w.told(t, who+": the editor's preview", protojson.Format(preview.Msg), wantNamed, generic)

		choices, err := p.characters.PreviewChoices(ctx, rq(&charactersv1.PreviewChoicesRequest{CampaignId: w.campaign, CharacterId: w.pens.GetId(), Kind: cur.GetKind(), Sheet: sheet}))
		if err != nil {
			t.Fatalf("%s: the choices preview answers %v", who, err)
		}
		if body := protojson.Format(choices.Msg); strings.Contains(body, w.unofferedLabel) {
			t.Errorf("%s: the choices preview holds the text of the entry: %s", who, shorten([]byte(body)))
		}
	}

	// The stored sheet: the master puts the key on it, and each reads the sentence of the sheet's issues.
	must(w.master.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: w.campaign, CharacterId: w.pens.GetId(), Revision: cur.GetRevision(), Name: cur.GetName(), Sheet: sheet,
	})))
	for who, p := range map[string]*person{"ana": w.ana, "master": w.master} {
		got := must(p.characters.GetCharacter(ctx, rq(&charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.pens.GetId()})))
		w.told(t, who+": the stored sheet", protojson.Format(got.GetCharacter().GetDerived()), p == w.master, generic)
	}
}

// told checks the sentence of the issue about the option: the entry's own for the master, the generic one for a player, who
// reads nothing of the entry.
func (w *world) told(t *testing.T, what, body string, master bool, generic string) {
	t.Helper()
	if master {
		if !strings.Contains(body, w.unofferedLabel+" não vale para este personagem.") {
			t.Errorf("%s: the master reads no sentence of the entry: %s", what, shorten([]byte(body)))
		}
		return
	}
	if !strings.Contains(body, generic) {
		t.Errorf("%s: the player reads no generic sentence: %s", what, shorten([]byte(body)))
	}
	if strings.Contains(body, w.unofferedLabel) {
		t.Errorf("%s: holds the text of the entry: %s", what, shorten([]byte(body)))
	}
}
