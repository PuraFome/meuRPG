package characters

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// TestUpdateCharacterWeaponMarksAreCheckedAndRoundTrip: a save with a mark for a weapon the sheet does
// not carry is refused with the field path of the mark, and a valid mark survives the save and the read.
func TestUpdateCharacterWeaponMarksAreCheckedAndRoundTrip(t *testing.T) {
	t.Parallel()
	et := newEditorTable(t, nil)
	pc := et.editorStandardFighter(t)

	notCarried := editorSaveOf(pc)
	notCarried.Sheet = editorClone(pc.GetSheet(), func(f *charactersv1.FullSheet) {
		f.WeaponKeys = []string{"equipment:dagger"}
		f.WeaponMarks = []*charactersv1.WeaponMark{{WeaponKey: "equipment:rapier", Silvered: true}}
	})
	_, err := editorUpdate(et.dona, notCarried)
	wantEditorRefusal(t, "a mark for a weapon not carried", err,
		`invalid_argument: sheet.full.weapon_marks[0].weapon_key is not one of the weapon_keys | InvalidField{field="sheet.full.weapon_marks[0].weapon_key"}`)

	valid := editorSaveOf(pc)
	valid.Sheet = editorClone(pc.GetSheet(), func(f *charactersv1.FullSheet) {
		f.WeaponKeys = []string{"equipment:dagger", "equipment:rapier"}
		f.WeaponMarks = []*charactersv1.WeaponMark{{WeaponKey: "equipment:rapier", Silvered: true, Magical: true}}
	})
	saved, err := editorUpdate(et.dona, valid)
	if err != nil {
		t.Fatalf("UpdateCharacter(valid mark) error = %v", err)
	}
	want := []*charactersv1.WeaponMark{{WeaponKey: "equipment:rapier", Silvered: true, Magical: true}}
	check := func(where string, got *charactersv1.Character) {
		t.Helper()
		marks := got.GetSheet().GetFull().GetWeaponMarks()
		if len(marks) != len(want) || !proto.Equal(marks[0], want[0]) {
			t.Errorf("%s: weapon_marks = %v, want %v", where, marks, want)
		}
	}
	check("the save's answer", saved)
	res, err := et.dona.api.GetCharacter(context.Background(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: et.campaign, CharacterId: pc.GetId()}))
	if err != nil {
		t.Fatalf("GetCharacter() error = %v", err)
	}
	check("the read after the save", res.Msg.GetCharacter())
}
