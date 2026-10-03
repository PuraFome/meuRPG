package characters

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// MR-016 (D1, question 44): an NPC's sheet carries the challenge rating ("ND")
// and the XP it gives when defeated. The server stores what it gets, checks the
// rating against the rules' list and the XP's range, and a player's character
// refuses both with an error (never a silent drop).

// withXP returns a copy of the sheet with the rating and the XP set, full or
// basic.
func withXP(sheet *charactersv1.CharacterSheet, rating string, xp int32) *charactersv1.CharacterSheet {
	out := proto.CloneOf(sheet)
	if f := out.GetFull(); f != nil {
		f.ChallengeRating, f.XpValue = rating, xp
	} else {
		out.GetBasic().ChallengeRating, out.GetBasic().XpValue = rating, xp
	}
	return out
}

func TestNPCSheetsStoreChallengeRatingAndXP(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre := h.newUser("Mestre")
	campaign := h.newCampaign(mestre, "Mirathel")

	for name, c := range map[string]struct {
		kind  charactersv1.CharacterKind
		sheet *charactersv1.CharacterSheet
	}{
		"enemy":  {charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, withXP(enemySheet(), "2", 450)},
		"boss":   {charactersv1.CharacterKind_CHARACTER_KIND_BOSS, withXP(enemySheet(), "1/8", 25)},
		"minion": {charactersv1.CharacterKind_CHARACTER_KIND_MINION, withXP(basicSheet(), "1/4", 50)},
	} {
		created := mestre.create(t, campaign, c.kind, name, c.sheet)
		got := mestre.get(t, campaign, created.GetId()).GetSheet()
		if !proto.Equal(got, created.GetSheet()) {
			t.Fatalf("%s: GetCharacter() sheet = %v, want the one saved", name, got)
		}
		cr, xp := got.GetFull().GetChallengeRating(), got.GetFull().GetXpValue()
		if got.GetBasic() != nil {
			cr, xp = got.GetBasic().GetChallengeRating(), got.GetBasic().GetXpValue()
		}
		if cr == "" || xp == 0 {
			t.Errorf("%s: rating %q and XP %d, want what was saved", name, cr, xp)
		}
	}

	// The master may type another value for the same rating, or none at all.
	minion := mestre.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_MINION, "Goblin", basicSheet())
	if _, err := mestre.update(t, minion, "Goblin", withXP(basicSheet(), "", 7)); err != nil {
		t.Errorf("UpdateCharacter(XP without a rating) error = %v, want allowed", err)
	}
}

func TestNPCChallengeRatingAndXPAreChecked(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre, jogador := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(mestre, "Mirathel", jogador)
	minion := charactersv1.CharacterKind_CHARACTER_KIND_MINION
	enemy := charactersv1.CharacterKind_CHARACTER_KIND_ENEMY
	player := charactersv1.CharacterKind_CHARACTER_KIND_PLAYER

	for _, c := range []struct {
		name   string
		kind   charactersv1.CharacterKind
		sheet  *charactersv1.CharacterSheet
		field  string // the field the error names
		caller *user
	}{
		{"a rating that is not on the rules list", minion, withXP(basicSheet(), "1/3", 10), "challenge_rating", mestre},
		{"a rating in decimals", enemy, withXP(enemySheet(), "0.5", 10), "challenge_rating", mestre},
		{"a rating above 30", enemy, withXP(enemySheet(), "31", 10), "challenge_rating", mestre},
		{"a negative XP", minion, withXP(basicSheet(), "1", -1), "xp_value", mestre},
		{"too much XP", enemy, withXP(enemySheet(), "30", 1_000_001), "xp_value", mestre},
		{"a rating on a player's character", player, withXP(pensantusSheet(), "1", 0), "challenge_rating", jogador},
		{"XP on a player's character", player, withXP(pensantusSheet(), "", 50), "xp_value", jogador},
	} {
		_, err := c.caller.api.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{CampaignId: campaign, Kind: c.kind, Name: "Teste", Sheet: c.sheet}))
		wantCode(t, "CreateCharacter("+c.name+")", err, connect.CodeInvalidArgument)
		if !strings.Contains(err.Error(), c.field) {
			t.Errorf("CreateCharacter(%s) error = %v, want it to name %s", c.name, err, c.field)
		}
	}

	// The same on an update, which must not let a player's character get one
	// either.
	pens := jogador.createPensantus(t, campaign)
	_, err := jogador.update(t, pens, "Pensantus", withXP(pens.GetSheet(), "1/2", 0))
	wantCode(t, "UpdateCharacter(a rating on a player's character)", err, connect.CodeInvalidArgument)
	_, err = mestre.update(t, pens, "Pensantus", withXP(pens.GetSheet(), "", 100))
	wantCode(t, "UpdateCharacter(XP on a player's character, by the master)", err, connect.CodeInvalidArgument)
}
