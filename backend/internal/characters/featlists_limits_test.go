package characters

import (
	"strconv"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

const oversizedFeatList = 100_001

// oversizedFeatSheets are full sheets whose feat lists are far over the request size limits.
func oversizedFeatSheets() map[string]*charactersv1.FullSheet {
	keys := make([]string, oversizedFeatList)
	slots := make(map[string]string, oversizedFeatList)
	for i := range keys {
		keys[i] = "feat:" + strconv.Itoa(i)
		slots[keys[i]] = "slot"
	}
	return map[string]*charactersv1.FullSheet{
		"feat_keys":  {FeatKeys: keys},
		"feat_slots": {FeatSlots: slots},
		"both":       {FeatKeys: keys, FeatSlots: slots},
		"one over":   {FeatKeys: keys[:rules.MaxListLength+1]},
	}
}

// The shape of the feat lists is checked before anything walks them: with no rules content at all, the
// refusal is the size one (a sheet that got further would fail on the missing content).
func TestCheckFullSheetRefusesOversizedFeatListsFirst(t *testing.T) {
	t.Parallel()
	for name, full := range oversizedFeatSheets() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := checkFullSheet(nil, full)
			if err == nil {
				t.Fatal("checkFullSheet() error = nil, want the size refusal")
			}
			if got := connect.CodeOf(invalidArgument(err)); got != connect.CodeInvalidArgument {
				t.Fatalf("code = %v, want invalid_argument", got)
			}
		})
	}
}

// CreateCharacter, PreviewCharacter and UpdateCharacter refuse the oversized feat lists with invalid_argument.
func TestFeatListsOverTheLimitAreRefusedByTheRPCs(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, ana := h.newUser("Samuel"), h.newUser("Ana")
	campaign := h.newCampaign(master, "Mirathel", ana)
	player := charactersv1.CharacterKind_CHARACTER_KIND_PLAYER
	anas := ana.create(t, campaign, player, "Gimli", hillDwarfSheet(1))
	for name, full := range oversizedFeatSheets() {
		sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: full}}
		_, err := ana.api.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{CampaignId: campaign, Kind: player, Name: "Teste", Sheet: sheet}))
		wantCode(t, name+": CreateCharacter()", err, connect.CodeInvalidArgument)
		_, err = ana.preview(t, campaign, "", player, sheet)
		wantCode(t, name+": PreviewCharacter() new", err, connect.CodeInvalidArgument)
		_, err = ana.preview(t, campaign, anas.GetId(), 0, sheet)
		wantCode(t, name+": PreviewCharacter() edit", err, connect.CodeInvalidArgument)
		_, err = ana.update(t, anas, "Gimli", sheet)
		wantCode(t, name+": UpdateCharacter()", err, connect.CodeInvalidArgument)
	}
}
