package campaignpackage_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
)

// bigPackage is a package of several parts: filler images that are valid.
func (h *harness) bigPackage(master *user, parts int) []byte {
	h.t.Helper()
	campaign := h.newCampaign(master, "Grande")
	// Noise does not compress, so a handful of images fill a few parts.
	for i := range parts * 3 {
		master.upload(campaign, "ruido"+strconv.Itoa(i)+".png", pngNoise(h.t, 700, 600, byte(i)))
	}
	return master.export(campaign)
}

func TestAnUploadResumesFromTheParts(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	data := h.bigPackage(master, 2)
	if len(data) <= campaignpackage.PartSize {
		t.Fatalf("the package has %d bytes, want it over one part (%d)", len(data), campaignpackage.PartSize)
	}
	ctx := t.Context()
	begun := must(master.pkg.BeginCampaignImport(ctx, connect.NewRequest(&pkgv1.BeginCampaignImportRequest{FileName: "g.meurpg.zip", TotalBytes: int64(len(data)), Fingerprint: "g:1"})))
	up := begun.GetUpload()
	if len(up.GetReceivedParts()) != 0 || up.GetPartCount() < 2 {
		t.Fatalf("upload = %v", up)
	}
	size := int(up.GetPartSize())
	// Part 1 goes; the connection drops; the same file is chosen again.
	if res := master.request(http.MethodPut, campaignpackage.PartsPath+up.GetId()+"/parts/1", data[:size]); res.status != http.StatusNoContent {
		t.Fatalf("part 1: %d %s", res.status, res.body)
	}
	again := must(master.pkg.BeginCampaignImport(ctx, connect.NewRequest(&pkgv1.BeginCampaignImportRequest{FileName: "g.meurpg.zip", TotalBytes: int64(len(data)), Fingerprint: "g:1"})))
	if again.GetUpload().GetId() != up.GetId() || len(again.GetUpload().GetReceivedParts()) != 1 || again.GetUpload().GetReceivedParts()[0] != 1 {
		t.Fatalf("the same file did not resume: %v", again.GetUpload())
	}
	// The preview of an incomplete upload says so.
	_, err := master.pkg.PreviewCampaignImport(ctx, connect.NewRequest(&pkgv1.PreviewCampaignImportRequest{ImportId: up.GetId()}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), "parts") {
		t.Fatalf("Preview of an incomplete upload error = %v", err)
	}
	// The rest goes, part 1 once more (a retry after a lost answer replaces itself).
	for n := 1; n <= int(up.GetPartCount()); n++ {
		lo, hi := (n-1)*size, min(n*size, len(data))
		if res := master.request(http.MethodPut, campaignpackage.PartsPath+up.GetId()+"/parts/"+strconv.Itoa(n), data[lo:hi]); res.status != http.StatusNoContent {
			t.Fatalf("part %d: %d %s", n, res.status, res.body)
		}
	}
	got := must(master.pkg.GetCampaignImport(ctx, connect.NewRequest(&pkgv1.GetCampaignImportRequest{})))
	if len(got.GetUpload().GetReceivedParts()) != int(up.GetPartCount()) || got.GetFingerprint() != "g:1" {
		t.Fatalf("GetCampaignImport = %v", got)
	}
	prev := must(master.pkg.PreviewCampaignImport(ctx, connect.NewRequest(&pkgv1.PreviewCampaignImportRequest{ImportId: up.GetId()})))
	if len(prev.GetProblems()) != 0 || prev.GetCounts().GetImages() != 6 {
		t.Fatalf("preview = %v", prev)
	}
}

func TestAPartWithTheWrongLengthIsRefusedAndKeptOut(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	data := h.bigPackage(master, 2)
	up := master.beginOnly("g.meurpg.zip", data)
	size := int(up.GetPartSize())
	for _, tc := range []struct {
		name string
		n    int
		body []byte
		want int
	}{
		{"too short", 1, data[:size-1], http.StatusBadRequest},
		{"too long", 1, append(append([]byte{}, data[:size]...), 'x'), http.StatusBadRequest},
		{"a part past the last", int(up.GetPartCount()) + 1, data[:size], http.StatusNotFound},
		{"part zero", 0, data[:size], http.StatusNotFound},
		{"not a number", -1, data[:size], http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := campaignpackage.PartsPath + up.GetId() + "/parts/" + strconv.Itoa(tc.n)
			if res := master.request(http.MethodPut, path, tc.body); res.status != tc.want {
				t.Fatalf("status = %d (%s), want %d", res.status, res.body, tc.want)
			}
		})
	}
	got := must(master.pkg.GetCampaignImport(t.Context(), connect.NewRequest(&pkgv1.GetCampaignImportRequest{})))
	if len(got.GetUpload().GetReceivedParts()) != 0 {
		t.Fatalf("a refused part was recorded: %v", got.GetUpload().GetReceivedParts())
	}
	if keys := h.storedKeys("imports/"); len(keys) != 0 {
		t.Fatalf("a refused part is in the store: %v", keys)
	}
}

// beginOnly starts an upload without sending a part.
func (u *user) beginOnly(name string, data []byte) *pkgv1.CampaignImport {
	u.h.t.Helper()
	res := must(u.pkg.BeginCampaignImport(u.h.t.Context(), connect.NewRequest(&pkgv1.BeginCampaignImportRequest{
		FileName: name, TotalBytes: int64(len(data)), Fingerprint: name + strconv.Itoa(len(data)),
	})))
	return res.GetUpload()
}

func TestAnUploadBelongsToTheAccountThatBeganIt(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	other := h.newUser("Outra pessoa")
	data := h.smallPackage(master)
	up := master.send("a.meurpg.zip", data)
	ctx := t.Context()
	// Another account cannot send a part, read, preview, create from or cancel it:
	// all of it reads as an upload that does not exist.
	if res := other.request(http.MethodPut, campaignpackage.PartsPath+up.GetId()+"/parts/1", data); res.status != http.StatusNotFound {
		t.Errorf("a part of someone else's upload: status %d", res.status)
	}
	if _, err := other.pkg.PreviewCampaignImport(ctx, connect.NewRequest(&pkgv1.PreviewCampaignImportRequest{ImportId: up.GetId()})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("Preview error = %v, want not_found", err)
	}
	if _, err := other.pkg.CreateCampaignFromImport(ctx, connect.NewRequest(&pkgv1.CreateCampaignFromImportRequest{ImportId: up.GetId(), IdempotencyKey: "k"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("Create error = %v, want not_found", err)
	}
	if _, err := other.pkg.CancelCampaignImport(ctx, connect.NewRequest(&pkgv1.CancelCampaignImportRequest{ImportId: up.GetId()})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("Cancel error = %v, want not_found", err)
	}
	if got := must(other.pkg.GetCampaignImport(ctx, connect.NewRequest(&pkgv1.GetCampaignImportRequest{}))); got.GetUpload() != nil {
		t.Errorf("another account sees an upload: %v", got.GetUpload())
	}
	// A person with no session gets nothing.
	anon := h.clients("")
	if res := anon.request(http.MethodPut, campaignpackage.PartsPath+up.GetId()+"/parts/1", data); res.status != http.StatusUnauthorized {
		t.Errorf("a part without a session: status %d, want 401", res.status)
	}
	// And the owner still can.
	if _, err := master.pkg.PreviewCampaignImport(ctx, connect.NewRequest(&pkgv1.PreviewCampaignImportRequest{ImportId: up.GetId()})); err != nil {
		t.Errorf("the owner's Preview error = %v", err)
	}
}

func TestAPersonHasOneUploadAtATime(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	first := master.send("a.meurpg.zip", h.smallPackage(master))
	if len(h.storedKeys("imports/")) == 0 {
		t.Fatal("the first upload left no part")
	}
	second := master.beginOnly("b.meurpg.zip", []byte("another file entirely"))
	if second.GetId() == first.GetId() {
		t.Fatal("another file reused the upload")
	}
	if keys := h.storedKeys("imports/" + first.GetId()); len(keys) != 0 {
		t.Fatalf("the first upload's parts were kept: %v", keys)
	}
	if h.count(`SELECT count(*)::INT FROM campaign_imports WHERE user_id = $1`, master.id) != 1 {
		t.Fatal("the person has more than one upload")
	}
	if _, err := master.pkg.PreviewCampaignImport(t.Context(), connect.NewRequest(&pkgv1.PreviewCampaignImportRequest{ImportId: first.GetId()})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("the dropped upload answers %v", err)
	}
}

func TestThePartsOfAnUploadExpireAnHourAfterTheLastOne(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	up := master.send("a.meurpg.zip", h.smallPackage(master))
	ctx := t.Context()
	h.clock.advance(59 * time.Minute)
	if _, err := master.pkg.PreviewCampaignImport(ctx, connect.NewRequest(&pkgv1.PreviewCampaignImportRequest{ImportId: up.GetId()})); err != nil {
		t.Fatalf("Preview at 59 minutes error = %v", err)
	}
	h.clock.advance(61 * time.Minute)
	if _, err := master.pkg.PreviewCampaignImport(ctx, connect.NewRequest(&pkgv1.PreviewCampaignImportRequest{ImportId: up.GetId()})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("Preview after an hour error = %v, want not_found", err)
	}
	if got := must(master.pkg.GetCampaignImport(ctx, connect.NewRequest(&pkgv1.GetCampaignImportRequest{}))); got.GetUpload() != nil {
		t.Fatalf("an expired upload is still offered: %v", got.GetUpload())
	}
	h.pkg.Sweep(ctx)
	if keys := h.storedKeys("imports/"); len(keys) != 0 {
		t.Fatalf("the sweep left parts: %v", keys)
	}
	if n := h.count(`SELECT count(*)::INT FROM campaign_imports`); n != 0 {
		t.Fatalf("the sweep left %d uploads", n)
	}
}

func TestAnUploadOverTheLimitIsRefusedBeforeAnyByteGoesUp(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	for _, size := range []int64{0, -1, campaignpackage.MaxPackageBytes + 1} {
		_, err := master.pkg.BeginCampaignImport(t.Context(), connect.NewRequest(&pkgv1.BeginCampaignImportRequest{FileName: "x.zip", TotalBytes: size, Fingerprint: "x"}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("Begin(%d bytes) error = %v, want invalid_argument", size, err)
		}
	}
	_, err := master.pkg.BeginCampaignImport(t.Context(), connect.NewRequest(&pkgv1.BeginCampaignImportRequest{FileName: " ", TotalBytes: 10, Fingerprint: "x"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("Begin with no name error = %v", err)
	}
	if !errors.Is(nil, nil) {
		t.Fatal("unreachable")
	}
}

func TestTheCampaignCapIsEnforcedBeforeTheUploadAndAtTheEnd(t *testing.T) {
	skip := false
	h := newHarness(t, func(o *options) {
		o.maxCampaigns = 2
		o.configure = func(c *campaignpackage.Config) { c.Creation = creationGate{Creation: c.Creation, skip: &skip} }
	})
	master := h.newUser("Mestre")
	data := h.smallPackage(master) // the campaign it came from is the first
	ctx := t.Context()
	up := master.send("a.meurpg.zip", data)
	created := must(master.pkg.CreateCampaignFromImport(ctx, connect.NewRequest(&pkgv1.CreateCampaignFromImportRequest{ImportId: up.GetId(), IdempotencyKey: "k-1"})))
	if created.GetCampaign().GetName() != "Mirathel" {
		t.Fatalf("campaign = %v", created.GetCampaign())
	}
	refused := func(err error) *campaignsv1.CampaignCreationRefused {
		t.Helper()
		if connect.CodeOf(err) != connect.CodeResourceExhausted {
			t.Fatalf("error = %v, want resource_exhausted", err)
		}
		for _, d := range detailsOf(err) {
			if v, derr := d.Value(); derr == nil {
				if r, ok := v.(*campaignsv1.CampaignCreationRefused); ok {
					return r
				}
			}
		}
		t.Fatalf("error %v has no CampaignCreationRefused detail", err)
		return nil
	}
	// At the cap now (the source and the import): refused before any byte goes up.
	_, err := master.pkg.BeginCampaignImport(ctx, connect.NewRequest(&pkgv1.BeginCampaignImportRequest{FileName: "b.zip", TotalBytes: int64(len(data)), Fingerprint: "b"}))
	if r := refused(err); r.GetReason() != campaignsv1.CampaignCreationRefusedReason_CAMPAIGN_CREATION_REFUSED_REASON_LIMIT_REACHED || r.GetMaxCampaigns() != 2 {
		t.Fatalf("detail = %v", r)
	}
	// With the early checks out of the way, the transaction that creates still counts, and nothing is left.
	skip = true
	late := master.send("c.meurpg.zip", data)
	files := len(h.storedKeys("campaigns/"))
	_, err = master.pkg.CreateCampaignFromImport(ctx, connect.NewRequest(&pkgv1.CreateCampaignFromImportRequest{ImportId: late.GetId(), IdempotencyKey: "k-2"}))
	refused(err)
	if n := h.count(`SELECT count(*)::INT FROM campaign_members WHERE user_id = $1 AND role = 'master'`, master.id); n != 2 {
		t.Fatalf("the master has %d campaigns, want 2", n)
	}
	if got := len(h.storedKeys("campaigns/")); got != files {
		t.Fatalf("a refused import left files behind: %d before, %d after", files, got)
	}
}

func TestACreateRetriedWithTheSameKeyMakesOneCampaign(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	up := master.send("a.meurpg.zip", h.smallPackage(master))
	ctx := t.Context()
	req := func() *connect.Request[pkgv1.CreateCampaignFromImportRequest] {
		return connect.NewRequest(&pkgv1.CreateCampaignFromImportRequest{ImportId: up.GetId(), IdempotencyKey: "retry-1"})
	}
	first := must(master.pkg.CreateCampaignFromImport(ctx, req()))
	second := must(master.pkg.CreateCampaignFromImport(ctx, req())) // the upload is gone by now
	if first.GetCampaign().GetId() != second.GetCampaign().GetId() {
		t.Fatalf("the retry made another campaign: %v / %v", first.GetCampaign().GetId(), second.GetCampaign().GetId())
	}
	if n := h.count(`SELECT count(*)::INT FROM campaigns WHERE name = 'Mirathel'`); n != 2 { // the source and one import
		t.Fatalf("campaigns named Mirathel = %d, want 2", n)
	}
	// The same key for another upload is another request.
	other := master.send("b.meurpg.zip", h.smallPackage(master))
	_, err := master.pkg.CreateCampaignFromImport(ctx, connect.NewRequest(&pkgv1.CreateCampaignFromImportRequest{ImportId: other.GetId(), IdempotencyKey: "retry-1"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("a key reused for another request: error = %v, want invalid_argument", err)
	}
	if _, err := master.pkg.CreateCampaignFromImport(ctx, connect.NewRequest(&pkgv1.CreateCampaignFromImportRequest{ImportId: other.GetId()})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("a create with no key: error = %v, want invalid_argument", err)
	}
}

func TestAFailureInTheLastKindLeavesNoRowAndNoFile(t *testing.T) {
	failing := false
	h := newHarness(t, func(o *options) {
		o.configure = func(c *campaignpackage.Config) {
			c.Parts = append(c.Parts, switchPart{on: &failing})
		}
	})
	master := h.newUser("Mestre")
	campaign := h.smallCampaign(master)
	zipped := master.export(campaign)
	campaignsBefore := h.count(`SELECT count(*)::INT FROM campaigns`)
	imagesBefore := h.count(`SELECT count(*)::INT FROM gallery_images`)
	mapsBefore := h.count(`SELECT count(*)::INT FROM maps`)
	filesBefore := len(h.storedKeys("campaigns/"))
	up := master.send("a.meurpg.zip", zipped)
	failing = true
	_, err := master.pkg.CreateCampaignFromImport(t.Context(), connect.NewRequest(&pkgv1.CreateCampaignFromImportRequest{ImportId: up.GetId(), IdempotencyKey: "k-fail"}))
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("error = %v, want internal", err)
	}
	if strings.Contains(err.Error(), "last kind") {
		t.Fatalf("the error carries the cause: %v", err)
	}
	for what, got := range map[string][2]int{
		"campaigns": {campaignsBefore, h.count(`SELECT count(*)::INT FROM campaigns`)},
		"images":    {imagesBefore, h.count(`SELECT count(*)::INT FROM gallery_images`)},
		"maps":      {mapsBefore, h.count(`SELECT count(*)::INT FROM maps`)},
		"files":     {filesBefore, len(h.storedKeys("campaigns/"))},
	} {
		if got[0] != got[1] {
			t.Errorf("%s: %d before, %d after the failed import", what, got[0], got[1])
		}
	}
	// "Tentar de novo" reuses the parts that were sent.
	failing = false
	got := must(master.pkg.CreateCampaignFromImport(t.Context(), connect.NewRequest(&pkgv1.CreateCampaignFromImportRequest{ImportId: up.GetId(), IdempotencyKey: "k-fail"})))
	if got.GetCampaign().GetName() != "Mirathel" {
		t.Fatalf("the retry created %v", got.GetCampaign())
	}
}

// switchPart fails its Apply while on is true.
type switchPart struct{ on *bool }

func (switchPart) Export(context.Context, pgx.Tx, string, *campaignpackage.Snapshot) error {
	return nil
}

func (switchPart) Stage(context.Context, *campaignpackage.Import) error { return nil }

func (p switchPart) Apply(context.Context, pgx.Tx, *campaignpackage.Import) error {
	if *p.on {
		return errors.New("the last kind failed")
	}
	return nil
}

func TestAnExportIsDownloadedByTheMasterOnly(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	player := h.newUser("Jogadora")
	stranger := h.newUser("Estranha")
	campaign := h.smallCampaign(master)
	h.invite(master, campaign, player)
	ctx := t.Context()

	started := must(master.pkg.StartCampaignExport(ctx, connect.NewRequest(&pkgv1.StartCampaignExportRequest{CampaignId: campaign, IdempotencyKey: "e-1"})))
	if started.GetExport().GetState() != pkgv1.CampaignExportState_CAMPAIGN_EXPORT_STATE_RUNNING && started.GetExport().GetState() != pkgv1.CampaignExportState_CAMPAIGN_EXPORT_STATE_DONE {
		t.Fatalf("export = %v", started.GetExport())
	}
	_ = master.export(campaign) // waits for one to be done
	done := must(master.pkg.GetCampaignExport(ctx, connect.NewRequest(&pkgv1.GetCampaignExportRequest{CampaignId: campaign})))
	e := done.GetExport()
	if e.GetFileName() != "Mirathel.meurpg.zip" || e.GetEntryCount() < 3 || e.GetByteSize() < 1 || done.GetLimitBytes() != campaignpackage.MaxPackageBytes || done.GetEstimatedBytes() < 1 {
		t.Fatalf("export = %v, estimate %d", e, done.GetEstimatedBytes())
	}
	path := e.GetDownloadPath()
	res := master.request(http.MethodGet, path, nil)
	if res.status != http.StatusOK {
		t.Fatalf("the master's download: %d %s", res.status, res.body)
	}
	for header, want := range map[string]string{
		"Content-Type":                 "application/zip",
		"Content-Disposition":          `attachment; filename="Mirathel.meurpg.zip"`,
		"Cache-Control":                "no-store",
		"X-Content-Type-Options":       "nosniff",
		"Content-Security-Policy":      "default-src 'none'",
		"Cross-Origin-Resource-Policy": "same-origin",
	} {
		if got := res.header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	// A player, a stranger and nobody get the same 404 (nobody learns the id exists).
	for name, u := range map[string]*user{"a player": player, "a stranger": stranger} {
		if res := u.request(http.MethodGet, path, nil); res.status != http.StatusNotFound {
			t.Errorf("%s downloads: status %d, want 404", name, res.status)
		}
	}
	if res := h.clients("").request(http.MethodGet, path, nil); res.status != http.StatusUnauthorized {
		t.Errorf("nobody downloads: status %d, want 401", res.status)
	}
	if res := master.request(http.MethodGet, campaignpackage.DownloadsPath+"0123456789abcdef0123456789abcdef", nil); res.status != http.StatusNotFound {
		t.Errorf("an id that does not exist: status %d", res.status)
	}
	for _, bad := range []string{"..", "../x", "short", strings.Repeat("g", 32), strings.Repeat("A", 32)} {
		if res := master.request(http.MethodGet, campaignpackage.DownloadsPath+bad, nil); res.status != http.StatusNotFound {
			t.Errorf("download %q: status %d, want 404", bad, res.status)
		}
	}
	// The API refuses a player and a stranger too.
	if _, err := player.pkg.StartCampaignExport(ctx, connect.NewRequest(&pkgv1.StartCampaignExportRequest{CampaignId: campaign, IdempotencyKey: "p-1"})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a player's StartCampaignExport error = %v, want permission_denied", err)
	}
	if _, err := stranger.pkg.GetCampaignExport(ctx, connect.NewRequest(&pkgv1.GetCampaignExportRequest{CampaignId: campaign})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a stranger's GetCampaignExport error = %v, want not_found", err)
	}
	// A master who lost the campaign cannot download either: the check is made on every request.
	if _, err := h.pool.Exec(ctx, `UPDATE campaign_members SET role = 'player' WHERE campaign_id = $1 AND user_id = $2`, campaign, master.id); err != nil {
		t.Fatal(err)
	}
	if res := master.request(http.MethodGet, path, nil); res.status != http.StatusNotFound {
		t.Errorf("a former master downloads: status %d, want 404", res.status)
	}
}

func TestAnExportIsKeptTwentyFourHoursThenDeleted(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.smallCampaign(master)
	_ = master.export(campaign)
	ctx := t.Context()
	if keys := h.storedKeys("campaigns/" + campaign + "/exports/"); len(keys) != 1 {
		t.Fatalf("export files = %v", keys)
	}
	h.clock.advance(23 * time.Hour)
	h.pkg.Sweep(ctx)
	got := must(master.pkg.GetCampaignExport(ctx, connect.NewRequest(&pkgv1.GetCampaignExportRequest{CampaignId: campaign})))
	if got.GetExport().GetState() != pkgv1.CampaignExportState_CAMPAIGN_EXPORT_STATE_DONE {
		t.Fatalf("at 23 hours the export is %v", got.GetExport())
	}
	path := got.GetExport().GetDownloadPath()
	h.clock.advance(2 * time.Hour)
	if res := master.request(http.MethodGet, path, nil); res.status != http.StatusNotFound {
		t.Errorf("an expired export downloads: status %d, want 404", res.status)
	}
	if got := must(master.pkg.GetCampaignExport(ctx, connect.NewRequest(&pkgv1.GetCampaignExportRequest{CampaignId: campaign}))); got.GetExport() != nil {
		t.Errorf("an expired export is offered: %v", got.GetExport())
	}
	h.pkg.Sweep(ctx)
	if keys := h.storedKeys("campaigns/" + campaign + "/exports/"); len(keys) != 0 {
		t.Errorf("the sweep left files: %v", keys)
	}
	if n := h.count(`SELECT count(*)::INT FROM campaign_exports`); n != 0 {
		t.Errorf("the sweep left %d export rows", n)
	}
}

func TestDeletingACampaignOrAnAccountDeletesItsExports(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.smallCampaign(master)
	_ = master.export(campaign)
	if err := h.pkg.DeleteCampaignPackages(t.Context(), campaign); err != nil {
		t.Fatal(err)
	}
	if keys := h.storedKeys("campaigns/" + campaign + "/exports/"); len(keys) != 0 {
		t.Fatalf("files left: %v", keys)
	}
	other := h.newUser("Outra")
	c2 := h.smallCampaign(other)
	_ = other.export(c2)
	up := other.send("a.zip", h.smallPackage(other))
	if err := h.pkg.DeleteUserPackages(t.Context(), other.id); err != nil {
		t.Fatal(err)
	}
	if keys := h.storedKeys(""); containsAny(keys, "exports/", "imports/"+up.GetId()) {
		t.Fatalf("files left: %v", keys)
	}
	if n := h.count(`SELECT count(*)::INT FROM campaign_exports WHERE requested_by = $1`, other.id); n != 0 {
		t.Fatalf("export rows left: %d", n)
	}
}

func containsAny(keys []string, parts ...string) bool {
	for _, k := range keys {
		for _, p := range parts {
			if strings.Contains(k, p) {
				return true
			}
		}
	}
	return false
}

func TestAnExportRetriedWithTheSameKeyIsTheSameExport(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.smallCampaign(master)
	ctx := t.Context()
	req := func(key string) *connect.Request[pkgv1.StartCampaignExportRequest] {
		return connect.NewRequest(&pkgv1.StartCampaignExportRequest{CampaignId: campaign, IdempotencyKey: key})
	}
	a := must(master.pkg.StartCampaignExport(ctx, req("same")))
	b := must(master.pkg.StartCampaignExport(ctx, req("same")))
	if a.GetExport().GetId() != b.GetExport().GetId() {
		t.Fatalf("the retry made another export: %s / %s", a.GetExport().GetId(), b.GetExport().GetId())
	}
	if validID := a.GetExport().GetId(); len(validID) != 32 {
		t.Fatalf("export id %q is not 32 hex digits", validID)
	}
	_ = master.export(campaign)
}

func TestAnExportThatPassesTheLimitFailsAsTooBig(t *testing.T) {
	h := newHarness(t, func(o *options) {
		o.configure = func(c *campaignpackage.Config) { c.Parts = append(c.Parts, hugePart{}) }
	})
	master := h.newUser("Mestre")
	campaign := h.smallCampaign(master)
	ctx := t.Context()
	_ = must(master.pkg.StartCampaignExport(ctx, connect.NewRequest(&pkgv1.StartCampaignExportRequest{CampaignId: campaign, IdempotencyKey: "big"})))
	deadline := time.Now().Add(time.Minute)
	for {
		got := must(master.pkg.GetCampaignExport(ctx, connect.NewRequest(&pkgv1.GetCampaignExportRequest{CampaignId: campaign})))
		if got.GetExport().GetState() == pkgv1.CampaignExportState_CAMPAIGN_EXPORT_STATE_FAILED {
			if got.GetExport().GetFailure() != pkgv1.CampaignExportFailure_CAMPAIGN_EXPORT_FAILURE_TOO_BIG {
				t.Fatalf("failure = %v", got.GetExport().GetFailure())
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the export never failed: %v", got.GetExport())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if keys := h.storedKeys("campaigns/" + campaign + "/exports/"); len(keys) != 0 {
		t.Fatalf("a failed export left files: %v", keys)
	}
}

// hugePart claims a file of 300 MiB, past the package's limit.
type hugePart struct{}

func (hugePart) Export(_ context.Context, _ pgx.Tx, _ string, s *campaignpackage.Snapshot) error {
	s.AddBlob(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE, "images/999", "campaigns/none/images/none", 300<<20)
	return nil
}
func (hugePart) Stage(context.Context, *campaignpackage.Import) error         { return nil }
func (hugePart) Apply(context.Context, pgx.Tx, *campaignpackage.Import) error { return nil }

func TestPackagesAreOffWithoutABlobStore(t *testing.T) {
	h := newHarness(t, func(o *options) { o.noBlobs = true })
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master, "Mirathel")
	ctx := t.Context()
	check := func(call string, err error) {
		t.Helper()
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("%s error = %v, want failed_precondition", call, err)
		}
	}
	_, err := master.pkg.StartCampaignExport(ctx, connect.NewRequest(&pkgv1.StartCampaignExportRequest{CampaignId: campaign, IdempotencyKey: "k"}))
	check("StartCampaignExport", err)
	_, err = master.pkg.BeginCampaignImport(ctx, connect.NewRequest(&pkgv1.BeginCampaignImportRequest{FileName: "a.zip", TotalBytes: 10, Fingerprint: "a"}))
	check("BeginCampaignImport", err)
}
