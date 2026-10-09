package campaignpackage_test

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
)

// peak samples the process's memory while f runs: the Go heap in use and the
// resident set (VmHWM is the process's high-water mark, so only the growth is
// ours).
func peak(f func()) (heapMiB, rssGrowthMiB float64) {
	var mu sync.Mutex
	var base runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&base)
	var maxHeap uint64
	stop := make(chan struct{})
	done := make(chan struct{})
	rssBefore := hwm()
	go func() {
		defer close(done)
		var ms runtime.MemStats
		for {
			runtime.ReadMemStats(&ms)
			mu.Lock()
			maxHeap = max(maxHeap, ms.HeapInuse)
			mu.Unlock()
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	f()
	close(stop)
	<-done
	return float64(maxHeap-min(maxHeap, base.HeapInuse)) / (1 << 20), float64(hwm()-min(hwm(), rssBefore)) / (1 << 20)
}

// hwm is the resident set's high-water mark, in bytes.
func hwm() uint64 {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if rest, ok := strings.CutPrefix(line, "VmHWM:"); ok {
			kb, _ := strconv.ParseUint(strings.Fields(rest)[0], 10, 64)
			return kb << 10
		}
	}
	return 0
}

// exportOnly runs an export to its end without downloading it.
func (u *user) exportOnly(campaignID string) {
	u.h.t.Helper()
	ctx := u.h.t.Context()
	_ = must(u.pkg.StartCampaignExport(ctx, connect.NewRequest(&pkgv1.StartCampaignExportRequest{CampaignId: campaignID, IdempotencyKey: "measure"})))
	for {
		got := must(u.pkg.GetCampaignExport(ctx, connect.NewRequest(&pkgv1.GetCampaignExportRequest{CampaignId: campaignID})))
		if got.GetExport().GetState() != pkgv1.CampaignExportState_CAMPAIGN_EXPORT_STATE_RUNNING {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestMeasureExportAndImportOfALargeCampaign prints the time and the memory
// of an export and an import of 20 maps of 3 MB and 50 NPCs (docs/operations.md).
// It runs only with MEURPG_MEASURE=1.
func TestMeasureExportAndImportOfALargeCampaign(t *testing.T) {
	if os.Getenv("MEURPG_MEASURE") == "" {
		t.Skip("set MEURPG_MEASURE=1 to measure")
	}
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master, "Grande")
	ctx := t.Context()
	for i := range 20 {
		img := master.upload(campaign, "mapa"+strconv.Itoa(i)+".png", pngNoise(t, 1000, 1000, byte(i)))
		_ = must(master.maps.CreateMap(ctx, connect.NewRequest(&mapsv1.CreateMapRequest{CampaignId: campaign, Name: "Mapa " + strconv.Itoa(i), ImageId: img.GetId()})))
	}
	for i := range 50 {
		_ = must(master.characters.CreateCharacter(ctx, connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_STORY, Name: "NPC " + strconv.Itoa(i),
			Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Basic{Basic: &charactersv1.BasicSheet{HitPointsMax: 9, ArmorClass: 11, SpeedFt: 30, Description: strings.Repeat("Um NPC. ", 40)}}},
			Story: &charactersv1.CharacterStory{Backstory: strings.Repeat("Uma história longa. ", 100)},
		})))
	}
	runtime.GC()
	var zipped []byte
	start := time.Now()
	heap, rss := peak(func() { master.exportOnly(campaign) })
	zipped = master.export(campaign)
	t.Logf("EXPORT: %d bytes in %v; heap growth over the start %.0f MiB, resident growth %.0f MiB", len(zipped), time.Since(start).Round(time.Millisecond), heap, rss)
	runtime.GC()
	start = time.Now()
	heap, rss = peak(func() { _ = master.importPackage("grande.meurpg.zip", zipped) })
	t.Logf("IMPORT: %d bytes in %v; heap growth over the start %.0f MiB, resident growth %.0f MiB", len(zipped), time.Since(start).Round(time.Millisecond), heap, rss)
}
