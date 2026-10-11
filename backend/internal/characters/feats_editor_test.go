package characters

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// The master adds and removes the feats of a sheet in the sheet editor (MR-025): the feats a
// player has come from the guided level-up, never from the editor.

// TestMR025_TheMasterAddsAFeatInTheSheetEditor: the save keeps the feat, its effect reaches the
// derived sheet (the table feat adds 1 to the initiative) and it works with the table rule
// "Talentos" off, because the master decides.
func TestMR025_TheMasterAddsAFeatInTheSheetEditor(t *testing.T) {
	t.Parallel()
	ft := newFeatsTable(t)
	before := ft.pc.GetDerived().GetInitiative()

	sheet := proto.CloneOf(ft.pc.GetSheet())
	sheet.GetFull().FeatKeys = []string{ft.half.GetKey()}
	got, err := ft.master.update(t, ft.pc, ft.pc.GetName(), sheet)
	if err != nil {
		t.Fatalf("the master's UpdateCharacter() error = %v", err)
	}
	if !slices.Equal(got.GetSheet().GetFull().GetFeatKeys(), []string{ft.half.GetKey()}) {
		t.Errorf("feats after the save = %v, want the feat the master added", got.GetSheet().GetFull().GetFeatKeys())
	}
	if got.GetDerived().GetInitiative() != before+1 {
		t.Errorf("initiative = %d, want %d: the feat's effect applies", got.GetDerived().GetInitiative(), before+1)
	}
	listed := false
	for _, f := range got.GetDerived().GetFeatures() {
		listed = listed || f.GetKey() == ft.half.GetKey()
	}
	if !listed {
		t.Errorf("the sheet does not list the feat: %v", got.GetDerived().GetFeatures())
	}
	// An unknown feat is refused, as any unknown key.
	bad := proto.CloneOf(got.GetSheet())
	bad.GetFull().FeatKeys = []string{"feat:nao-existe"}
	if _, err := ft.master.update(t, got, got.GetName(), bad); err == nil {
		t.Error("the save accepted a feat the content does not have")
	}
}

// TestMR025_APlayerCannotAddOrRemoveAFeatInTheSheetEditor: a player's draft keeps its feats as
// they are; a save that adds or removes one, or a new sheet that brings one, is refused with
// permission_denied. The same save by the master works.
func TestMR025_APlayerCannotAddOrRemoveAFeatInTheSheetEditor(t *testing.T) {
	t.Parallel()
	ft := newFeatsTable(t)
	// A campaign where no game session has started: the player's draft is theirs to edit.
	draftCampaign := ft.h.newCampaign(ft.master, "Rascunho", ft.owner)
	draft := ft.owner.create(t, draftCampaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", pensantusSheet())

	// Adding.
	withFeat := proto.CloneOf(draft.GetSheet())
	withFeat.GetFull().FeatKeys = []string{"feat:grappler"}
	_, err := ft.owner.update(t, draft, draft.GetName(), withFeat)
	wantCode(t, "the player's UpdateCharacter(add a feat)", err, connect.CodePermissionDenied)
	if !strings.Contains(err.Error(), "master") {
		t.Errorf("the refusal does not say that the master does it: %v", err)
	}
	// A new sheet with a feat.
	newSheet := pensantusSheet()
	newSheet.GetFull().FeatKeys = []string{"feat:grappler"}
	_, err = ft.owner.api.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{CampaignId: draftCampaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Outro", Sheet: newSheet}))
	wantCode(t, "the player's CreateCharacter(with a feat)", err, connect.CodePermissionDenied)

	// The master adds it; the same player's save that keeps it works, and one that removes it is refused.
	byMaster, err := ft.master.update(t, draft, draft.GetName(), withFeat)
	if err != nil {
		t.Fatalf("the master's UpdateCharacter() error = %v", err)
	}
	kept := proto.CloneOf(byMaster.GetSheet())
	kept.GetFull().Languages = append(kept.GetFull().Languages, "Élfico")
	after, err := ft.owner.update(t, byMaster, byMaster.GetName(), kept)
	if err != nil {
		t.Fatalf("the player's save that keeps the feat: error = %v", err)
	}
	removed := proto.CloneOf(after.GetSheet())
	removed.GetFull().FeatKeys = nil
	_, err = ft.owner.update(t, after, after.GetName(), removed)
	wantCode(t, "the player's UpdateCharacter(remove a feat)", err, connect.CodePermissionDenied)
	if _, err := ft.master.update(t, after, after.GetName(), removed); err != nil {
		t.Errorf("the master's removal: error = %v", err)
	}
}

// TestMR025_RemovingAFeatTakenInPlaceOfAnImprovementDropsItsSlot: a feat taken at the level-up
// has a slot (the improvement it replaced); the master removes it in the editor, the slot goes
// with it, and the sheet lists the Ability Score Improvement again.
func TestMR025_RemovingAFeatTakenInPlaceOfAnImprovementDropsItsSlot(t *testing.T) {
	t.Parallel()
	ft := newFeatsTable(t)
	ft.allowFeats(t, true)
	up, err := ft.levelUp(ft.owner, ft.pc, ft.featChoice(ft.plain.GetKey(), nil))
	if err != nil {
		t.Fatalf("LevelUpCharacter() error = %v", err)
	}
	if len(up.GetSheet().GetFull().GetFeatSlots()) != 1 {
		t.Fatalf("slots after the level-up = %v, want one", up.GetSheet().GetFull().GetFeatSlots())
	}
	asi := func(c *charactersv1.Character) bool {
		for _, f := range c.GetDerived().GetFeatures() {
			if strings.Contains(f.GetKey(), "ability-score-improvement") {
				return true
			}
		}
		return false
	}
	if asi(up) {
		t.Fatal("the sheet lists the improvement the feat replaced")
	}

	// The editor sends the sheet as it read it, minus the feat (even with the slot left in it).
	sheet := proto.CloneOf(up.GetSheet())
	sheet.GetFull().FeatKeys = nil
	got, err := ft.master.update(t, up, up.GetName(), sheet)
	if err != nil {
		t.Fatalf("the master's UpdateCharacter() error = %v", err)
	}
	if len(got.GetSheet().GetFull().GetFeatKeys()) != 0 || len(got.GetSheet().GetFull().GetFeatSlots()) != 0 {
		t.Errorf("after the removal: feats %v, slots %v; want none", got.GetSheet().GetFull().GetFeatKeys(), got.GetSheet().GetFull().GetFeatSlots())
	}
	if !asi(got) {
		t.Errorf("the improvement did not come back: %v", got.GetDerived().GetFeatures())
	}
}
