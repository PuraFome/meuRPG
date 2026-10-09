package campaignpackage_test

import (
	"bytes"
	"testing"

	"connectrpc.com/connect"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
)

// smallCampaign is a campaign with an image, a map and a scene: enough to be a package.
func (h *harness) smallCampaign(master *user) (campaignID string) {
	h.t.Helper()
	ctx := h.t.Context()
	campaignID = h.newCampaign(master, "Mirathel")
	img := master.upload(campaignID, "Mapa antigo.png", pngImage(h.t, 400, 300, 40))
	m := must(master.maps.CreateMap(ctx, connect.NewRequest(&mapsv1.CreateMapRequest{CampaignId: campaignID, Name: "Vila", ImageId: img.GetId()})))
	_ = must(master.maps.CreateMapPoint(ctx, connect.NewRequest(&mapsv1.CreateMapPointRequest{
		CampaignId: campaignID, MapId: m.GetMap().GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "A ponte quebrada", XBp: 10, YBp: 10,
	})))
	return campaignID
}

// preview sends a package and reads what the server makes of it.
func (u *user) preview(name string, data []byte) *pkgv1.PreviewCampaignImportResponse {
	u.h.t.Helper()
	up := u.send(name, data)
	return must(u.pkg.PreviewCampaignImport(u.h.t.Context(), connect.NewRequest(&pkgv1.PreviewCampaignImportRequest{ImportId: up.GetId()})))
}

// only checks that the preview has exactly one problem, and returns it.
func only(t *testing.T, p *pkgv1.PreviewCampaignImportResponse) *pkgv1.PackageProblem {
	t.Helper()
	if len(p.GetProblems()) != 1 {
		t.Fatalf("problems = %v, want exactly one", p.GetProblems())
	}
	return p.GetProblems()[0]
}

func TestEveryRefusalOfAPackageNamesItsReason(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	good := master.export(h.smallCampaign(master))

	tests := []struct {
		name   string
		change func(c *crafted)
		raw    func(good []byte) []byte
		kind   pkgv1.PackageProblemKind
		reason pkgv1.PackageProblemReason
		limit  int64
		label  string
	}{
		{
			name: "a package of a newer format", kind: pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, reason: pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NEWER_VERSION,
			change: func(c *crafted) { c.manifest.FormatVersion = 2 },
		},
		{
			name: "an entry changed after the manifest was written", kind: pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, reason: pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_HASH_MISMATCH,
			change: func(c *crafted) {
				name := c.entryOf(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP, 0)
				c.files[name] = append(bytes.Clone(c.files[name][:len(c.files[name])-1]), ' ') // same length, other content
			},
			label: "maps/1.json",
		},
		{
			name: "a field the server does not know in an entry", kind: pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, reason: pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNKNOWN_FIELD,
			change: func(c *crafted) {
				c.set(campaignpackage.CampaignEntry, []byte(`{"name":"Mirathel","xp_mode":"XP_MODE_MILESTONES","owner_email":"someone@example.com","table_rules":{}}`))
			},
			label: campaignpackage.CampaignEntry,
		},
		{
			name: "an image over ten mebibytes, by the image's name", kind: pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_IMAGE, reason: pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_ENTRY_TOO_BIG,
			change: func(c *crafted) {
				c.set(c.entryOf(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE, 0), bytes.Repeat([]byte{7}, campaignpackage.MaxEntryBytes+1))
			},
			label: "Mapa antigo", limit: campaignpackage.MaxEntryBytes + 1,
		},
		{
			name: "an entry that squeezes more than the ratio allows", kind: pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, reason: pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_COMPRESSION_RATIO,
			change: func(c *crafted) {
				c.add(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE, "images/99", make([]byte, 8<<20))
			},
			label: "images/99", limit: campaignpackage.MaxRatio,
		},
		{
			name: "a name that climbs out of a folder", kind: pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, reason: pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_BAD_ENTRY_NAME,
			change: func(c *crafted) { c.add(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE, "../../etc/cron.d/x", []byte("boom")) },
			label:  "../../etc/cron.d/x",
		},
		{
			name: "an entry the zip does not have", kind: pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, reason: pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_MISSING_ENTRY,
			change: func(c *crafted) {
				name := c.entryOf(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP, 0)
				delete(c.files, name)
				c.order = slicesDelete(c.order, name)
			},
			label: "maps/1.json",
		},
		{
			name: "an image the gallery lists and the package lacks", kind: pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_IMAGE, reason: pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_MISSING_ENTRY,
			change: func(c *crafted) { c.drop(c.entryOf(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE, 0)) },
			label:  "Mapa antigo",
		},
		{
			name: "a file that is not an image", kind: pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_IMAGE, reason: pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_IMAGE_UNREADABLE,
			change: func(c *crafted) { c.set(c.entryOf(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE, 0), []byte("<svg onload=alert(1)>")) },
			label:  "Mapa antigo", limit: 10 << 20,
		},
		{
			name: "something that is not a package at all", kind: pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, reason: pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_A_PACKAGE,
			raw: func([]byte) []byte { return []byte("just some text, not a zip file") },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := bytes.Clone(good)
			if tc.raw != nil {
				data = tc.raw(good)
			} else {
				c := unpack(t, good)
				tc.change(c)
				data = c.bytes()
			}
			p := only(t, master.preview("x.meurpg.zip", data))
			if p.GetKind() != tc.kind || p.GetReason() != tc.reason {
				t.Fatalf("problem = %v, want %v / %v", p, tc.kind, tc.reason)
			}
			if tc.label != "" && p.GetName() != tc.label {
				t.Errorf("problem names %q, want %q", p.GetName(), tc.label)
			}
			if tc.limit != 0 && p.GetLimit() != tc.limit {
				t.Errorf("problem limit = %d, want %d", p.GetLimit(), tc.limit)
			}
		})
	}
}

func TestAPackageWithAnyProblemIsNotCreated(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	c := unpack(t, master.export(h.smallCampaign(master)))
	c.manifest.FormatVersion = 2
	up := master.send("novo.meurpg.zip", c.bytes())
	before := h.count(`SELECT count(*)::INT FROM campaigns`)
	_, err := master.pkg.CreateCampaignFromImport(t.Context(), connect.NewRequest(&pkgv1.CreateCampaignFromImportRequest{ImportId: up.GetId(), IdempotencyKey: "k-1"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("CreateCampaignFromImport() error = %v, want failed_precondition", err)
	}
	var blocked *pkgv1.CampaignPackageBlocked
	for _, d := range err.(*connect.Error).Details() {
		if v, derr := d.Value(); derr == nil {
			blocked, _ = v.(*pkgv1.CampaignPackageBlocked)
		}
	}
	if blocked.GetReason() != pkgv1.CampaignPackageBlockedReason_CAMPAIGN_PACKAGE_BLOCKED_REASON_HAS_PROBLEMS || blocked.GetPreview().GetProblems()[0].GetReason() != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NEWER_VERSION {
		t.Fatalf("detail = %v", blocked)
	}
	if after := h.count(`SELECT count(*)::INT FROM campaigns`); after != before {
		t.Fatalf("campaigns went from %d to %d", before, after)
	}
}

func TestEveryProblemOfAPackageIsListedNotJustTheFirst(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.smallCampaign(master)
	ctx := t.Context()
	// A second image, so two of them can be wrong.
	img := master.upload(campaign, "Outro mapa.png", pngImage(t, 300, 300, 77))
	_ = must(master.maps.CreateMap(ctx, connect.NewRequest(&mapsv1.CreateMapRequest{CampaignId: campaign, Name: "Torre", ImageId: img.GetId()})))
	c := unpack(t, master.export(campaign))
	c.set(c.entryOf(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE, 0), []byte("not an image"))
	c.set(c.entryOf(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE, 1), []byte("nor this"))
	p := master.preview("x.meurpg.zip", c.bytes())
	if len(p.GetProblems()) != 2 {
		t.Fatalf("problems = %v, want one for each image", p.GetProblems())
	}
}

func TestAReferenceToSomethingNotInThePackageIsAProblem(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	c := unpack(t, master.export(h.smallCampaign(master)))
	name := c.entryOf(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP, 0)
	var m pkgv1.PackageMap
	if reason, ok := mustRead(c, name, &m); !ok {
		t.Fatalf("read the map: %v", reason)
	}
	m.ImageId = "0b6f4a52-3b5e-4c55-9d0b-2a51f0c1e001" // an image the gallery does not have
	c.set(name, mustMarshal(t, &m))
	p := only(t, master.preview("x.meurpg.zip", c.bytes()))
	if p.GetKind() != pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_MAP || p.GetName() != "Vila" || p.GetReason() != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_IN_PACKAGE {
		t.Fatalf("problem = %v", p)
	}
}

func TestAPackageWithAnUnknownSceneActionNamesTheScene(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	c := unpack(t, master.export(h.smallCampaign(master)))
	name := c.entryOf(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP, 0)
	var m pkgv1.PackageMap
	if reason, ok := mustRead(c, name, &m); !ok {
		t.Fatalf("read the map: %v", reason)
	}
	m.Points[0].Actions = []*pkgv1.PackageAction{{Key: "skill:underwater-basket-weaving", Dc: 12, MaxAttempts: 1}}
	c.set(name, mustMarshal(t, &m))
	p := only(t, master.preview("x.meurpg.zip", c.bytes()))
	if p.GetKind() != pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_SCENE || p.GetName() != "A ponte quebrada" || p.GetReason() != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNKNOWN_CONTENT {
		t.Fatalf("problem = %v", p)
	}
}

func TestAnEntryFarOverTheLimitThatIsSmallInTheZipIsRefusedBeforeItIsRead(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	c := unpack(t, master.export(h.smallCampaign(master)))
	// 64 MiB of zeros are a few dozen KiB deflated: the declared size refuses it.
	c.add(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE, "images/98", make([]byte, 64<<20))
	p := only(t, master.preview("x.meurpg.zip", c.bytes()))
	if p.GetReason() != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_ENTRY_TOO_BIG {
		t.Fatalf("problem = %v", p)
	}
}

