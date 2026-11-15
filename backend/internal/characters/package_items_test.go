package characters

import (
	"testing"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// A package's item state is judged against the sheet and the rules content that come with it, as the calls that set it are.
func TestAPackagesItemStateIsChecked(t *testing.T) {
	t.Parallel()
	content := loadSRD(t)
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{MagicItems: []*charactersv1.SheetMagicItem{
		{Id: "m1", Key: hammerKey}, {Id: "m2", Key: goatsKey}, {Id: "m5", Key: "item:bag-of-holding"}, {Id: "m6", Key: wandKey},
	}}}}
	charge := func(entry string, used int32, lost charactersv1.ItemLoss, dawns int32, pending bool) []*pkgv1.PackageItemCharge {
		return []*pkgv1.PackageItemCharge{{EntryId: entry, ChargesUsed: used, Lost: lost, DawnsWaited: dawns, LastRollPending: pending}}
	}
	none := charactersv1.ItemLoss_ITEM_LOSS_UNSPECIFIED
	tests := []struct {
		name    string
		charges []*pkgv1.PackageItemCharge
		ammo    []*pkgv1.PackageAmmunition
		want    bool
	}{
		{"spent charges", charge("m1", 2, none, 0, false), nil, true},
		{"the goats' wait", charge("m2", 24, none, 3, false), nil, true},
		{"a lost item", charge("m1", 5, charactersv1.ItemLoss_ITEM_LOSS_GONE, 0, false), nil, true},
		{"a wand that waits for its d20", charge("m6", 7, none, 0, true), nil, true},
		{"arrows", nil, []*pkgv1.PackageAmmunition{{AmmoKey: "equipment:arrow", Quantity: 20}}, true},
		{"an entry the sheet lacks", charge("m9", 1, none, 0, false), nil, false},
		{"an item with no charges", charge("m5", 1, none, 0, false), nil, false},
		{"more spent than the maximum", charge("m1", 6, none, 0, false), nil, false},
		{"a negative count", charge("m1", -1, none, 0, false), nil, false},
		{"dawns on an item that counts none", charge("m1", 1, none, 1, false), nil, false},
		{"more dawns than the wait", charge("m2", 24, none, 8, false), nil, false},
		{"a d20 awaited by an item with none", charge("m1", 5, none, 0, true), nil, false},
		{"a d20 awaited with charges left", charge("m6", 3, none, 0, true), nil, false},
		{"an unknown loss", charge("m1", 5, charactersv1.ItemLoss(9), 0, false), nil, false},
		{"an entry named twice", append(charge("m1", 1, none, 0, false), charge("m1", 2, none, 0, false)...), nil, false},
		{"a kind that is no ammunition", nil, []*pkgv1.PackageAmmunition{{AmmoKey: "equipment:longsword", Quantity: 1}}, false},
		{"too many pieces", nil, []*pkgv1.PackageAmmunition{{AmmoKey: "equipment:arrow", Quantity: MaxAmmunition + 1}}, false},
		{"a kind named twice", nil, []*pkgv1.PackageAmmunition{{AmmoKey: "equipment:arrow", Quantity: 1}, {AmmoKey: "equipment:arrow", Quantity: 2}}, false},
	}
	for _, tt := range tests {
		if _, got := stageItemState(content, sheet, tt.charges, tt.ammo); got != tt.want {
			t.Errorf("%s: stageItemState() = %v, want %v", tt.name, got, tt.want)
		}
	}
	basic := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Basic{Basic: &charactersv1.BasicSheet{}}}
	if _, ok := stageItemState(content, basic, charge("m1", 1, none, 0, false), nil); ok {
		t.Error("a basic sheet has no items: its charges were accepted")
	}
	if st, ok := stageItemState(content, basic, nil, nil); !ok || len(st.charges)+len(st.ammo) != 0 {
		t.Error("a sheet with no state was refused")
	}
}
