package campaignpackage_test

import (
	"math"
	"testing"

	"connectrpc.com/connect"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters"
)

// The state of a character's magic items and ammunition that a package carries goes through the real import: a package
// whose numbers play could never have made is INVALID at the preview, and creating from it writes no row.
func TestTheItemStateOfACraftedPackageIsCheckedByTheRealImport(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	good := master.export(h.buildFixture(master).campaign)
	const (
		hammer, goats, wand = "m1", "m2", "m3" // the fixture's items: 5 charges, 24 charges back on the 7th dawn, 7 charges and a d20
		none                = charactersv1.ItemLoss_ITEM_LOSS_UNSPECIFIED
	)
	charge := func(entry string, used int32, lost charactersv1.ItemLoss, dawns int32, pending bool) *pkgv1.PackageItemCharge {
		return &pkgv1.PackageItemCharge{EntryId: entry, ChargesUsed: used, Lost: lost, DawnsWaited: dawns, LastRollPending: pending}
	}
	arrows := func(n int32) []*pkgv1.PackageAmmunition {
		return []*pkgv1.PackageAmmunition{{AmmoKey: "equipment:arrow", Quantity: n}}
	}
	tests := []struct {
		name    string
		charges []*pkgv1.PackageItemCharge
		ammo    []*pkgv1.PackageAmmunition
		valid   bool
	}{
		{"every charge of the hammer spent", []*pkgv1.PackageItemCharge{charge(hammer, 5, none, 0, false)}, nil, true},
		{"the goats at the last dawn they wait", []*pkgv1.PackageItemCharge{charge(goats, 24, none, 7, false)}, nil, true},
		{"the most ammunition a character holds", nil, arrows(characters.MaxAmmunition), true},
		{"one charge more than the hammer holds", []*pkgv1.PackageItemCharge{charge(hammer, 6, none, 0, false)}, nil, false},
		{"the largest count of spent charges", []*pkgv1.PackageItemCharge{charge(hammer, math.MaxInt32, none, 0, false)}, nil, false},
		{"the smallest count of spent charges", []*pkgv1.PackageItemCharge{charge(hammer, math.MinInt32, none, 0, false)}, nil, false},
		{"the largest count of dawns", []*pkgv1.PackageItemCharge{charge(goats, 24, none, math.MaxInt32, false)}, nil, false},
		{"the smallest count of dawns", []*pkgv1.PackageItemCharge{charge(goats, 24, none, math.MinInt32, false)}, nil, false},
		{"one dawn more than the goats wait", []*pkgv1.PackageItemCharge{charge(goats, 24, none, 8, false)}, nil, false},
		{"dawns counted by goats that still have a charge", []*pkgv1.PackageItemCharge{charge(goats, 23, none, 3, false)}, nil, false},
		{"an item lost while it waits for its d20", []*pkgv1.PackageItemCharge{charge(wand, 7, charactersv1.ItemLoss_ITEM_LOSS_GONE, 0, true)}, nil, false},
		{"an entry that is not on the sheet", []*pkgv1.PackageItemCharge{charge("m9", 1, none, 0, false)}, nil, false},
		{"one piece more than a character holds", nil, arrows(characters.MaxAmmunition + 1), false},
		{"the largest quantity of ammunition", nil, arrows(math.MaxInt32), false},
		{"the smallest quantity of ammunition", nil, arrows(math.MinInt32), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := unpack(t, good)
			name := c.entryOf(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_CHARACTER, 0)
			var ch pkgv1.PackageCharacter
			if why, ok := mustRead(c, name, &ch); !ok || ch.GetName() != "Pensantus" {
				t.Fatalf("the first character of the package is %q (%s), want Pensantus", ch.GetName(), why)
			}
			ch.ItemCharges, ch.Ammunition = tc.charges, tc.ammo
			c.set(name, mustMarshal(t, &ch))
			up := master.send("x.meurpg.zip", c.bytes())
			prev := must(master.pkg.PreviewCampaignImport(t.Context(), connect.NewRequest(&pkgv1.PreviewCampaignImportRequest{ImportId: up.GetId()})))
			const rows = `SELECT (SELECT count(*) FROM campaigns) + (SELECT count(*) FROM characters) + (SELECT count(*) FROM character_item_charges) + (SELECT count(*) FROM character_ammunition)`
			before := h.count(rows + `::INT`)
			if tc.valid {
				if len(prev.GetProblems()) != 0 {
					t.Fatalf("problems = %v, want none", prev.GetProblems())
				}
				res, err := master.pkg.CreateCampaignFromImport(t.Context(), connect.NewRequest(&pkgv1.CreateCampaignFromImportRequest{ImportId: up.GetId(), IdempotencyKey: "k-" + tc.name}))
				if err != nil {
					t.Fatalf("CreateCampaignFromImport() error = %v", err)
				}
				id := res.Msg.GetCampaign().GetId()
				stored := h.count(`SELECT (SELECT count(*) FROM character_item_charges ic JOIN characters c ON c.id = ic.character_id WHERE c.campaign_id = $1)
					+ (SELECT count(*) FROM character_ammunition a JOIN characters c ON c.id = a.character_id WHERE c.campaign_id = $1)::INT`, id)
				if stored != len(tc.charges)+len(tc.ammo) {
					t.Errorf("the import stored %d rows of item state, want %d", stored, len(tc.charges)+len(tc.ammo))
				}
				return
			}
			p := only(t, prev)
			if p.GetKind() != pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_CHARACTER || p.GetName() != "Pensantus" || p.GetReason() != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID {
				t.Fatalf("problem = %v, want the character Pensantus INVALID", p)
			}
			_, err := master.pkg.CreateCampaignFromImport(t.Context(), connect.NewRequest(&pkgv1.CreateCampaignFromImportRequest{ImportId: up.GetId(), IdempotencyKey: "k-" + tc.name}))
			if connect.CodeOf(err) != connect.CodeFailedPrecondition {
				t.Fatalf("CreateCampaignFromImport() error = %v, want failed_precondition", err)
			}
			if after := h.count(rows + `::INT`); after != before {
				t.Errorf("the refused package left %d rows more", after-before)
			}
		})
	}
}
