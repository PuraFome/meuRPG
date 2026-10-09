// Package choicetest is for the tests of other packages that create player characters
// with the sheets of a class or a race that asks for choices (a Fighting Style, a Pact
// Boon, a favored enemy...): the server refuses such a sheet while a choice is open
// (CHOICES_MISSING), so the tests' client makes the choices a player would, as the editor
// does, before it creates or saves the sheet. It only fills what is open: a sheet that
// already has its picks is sent as it is.
//
// The picks are the first the sheet can take that change no number of a fight (a fighting
// style that adds to AC or to an attack, an invocation that adds damage or a sense), so a
// test's expected numbers do not move because of them.
package choicetest

import (
	"context"
	"slices"
	"strings"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"
)

// numeric are the options that change a number or the actions of a fight: they are the
// last ones taken.
var numeric = []string{
	"archery", "defense", "dueling", "protection", "two-weapon-fighting", "agonizing-blast", "beguiling-influence",
	"devils-sight", "giant-killer", "evasion", "uncanny-dodge", "volley", "whirlwind",
}

const maxRounds = 12

// Interceptor makes the open choices of the sheet of a CreateCharacter or UpdateCharacter
// request that comes from a player's character. raw is a client of the same server that
// does not go through the interceptor.
func Interceptor(raw charactersv1connect.CharacterServiceClient) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			switch m := req.Any().(type) {
			case *charactersv1.CreateCharacterRequest:
				if m.GetKind() == charactersv1.CharacterKind_CHARACTER_KIND_PLAYER {
					fill(ctx, raw, m.GetCampaignId(), "", m.GetKind(), m.GetSheet(), req)
				}
			case *charactersv1.UpdateCharacterRequest:
				fill(ctx, raw, m.GetCampaignId(), m.GetCharacterId(), charactersv1.CharacterKind_CHARACTER_KIND_UNSPECIFIED, m.GetSheet(), req)
			}
			return next(ctx, req)
		}
	}
}

// fill appends to the sheet's picks the ones that complete its choices. An error from the
// preview leaves the sheet alone: the call that follows answers it.
func fill(ctx context.Context, raw charactersv1connect.CharacterServiceClient, campaignID, characterID string, kind charactersv1.CharacterKind, sheet *charactersv1.CharacterSheet, req connect.AnyRequest) {
	full := sheet.GetFull()
	if full == nil {
		return
	}
	for range maxRounds {
		preview := connect.NewRequest(&charactersv1.PreviewChoicesRequest{CampaignId: campaignID, CharacterId: characterID, Kind: kind, Sheet: sheet})
		res, err := raw.PreviewChoices(ctx, preview)
		if err != nil {
			return
		}
		added := false
		for _, g := range res.Msg.GetGroups() {
			for _, ch := range g.GetChoices() {
				added = take(ch, full) || added
			}
		}
		if !added {
			return
		}
	}
}

// take adds the picks of a choice that is open and reports whether it added any.
func take(ch *charactersv1.Choice, full *charactersv1.FullSheet) bool {
	need := int(ch.GetMissing())
	if need == 0 {
		return false
	}
	options := slices.Clone(ch.GetOptions())
	slices.SortStableFunc(options, func(a, b *charactersv1.ChoiceOption) int { return rank(a) - rank(b) })
	added := false
	for _, o := range options {
		if need == 0 {
			break
		}
		if o.GetReasonPt() != "" || o.GetNeedsText() || slices.Contains(ch.GetPicked(), o.GetKey()) || slices.Contains(full.FeatureChoiceKeys, o.GetStoredKey()) {
			continue
		}
		full.FeatureChoiceKeys = append(full.FeatureChoiceKeys, o.GetStoredKey())
		need--
		added = true
	}
	return added
}

func rank(o *charactersv1.ChoiceOption) int {
	for _, n := range numeric {
		if strings.Contains(o.GetKey(), n) {
			return 1
		}
	}
	return 0
}
