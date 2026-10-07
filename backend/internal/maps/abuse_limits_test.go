package maps

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/images/gen"
	"github.com/PuraFome/meuRPG/backend/internal/platform/ratelimit"
)

func TestDayStart(t *testing.T) {
	t.Parallel()
	// Brazil is UTC-3: 02:00 UTC on the 8th is still the 7th there.
	got := dayStart(time.Date(2026, 10, 8, 2, 0, 0, 0, time.UTC))
	if want := time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("dayStart(02:00 UTC) = %v, want %v", got, want)
	}
	got = dayStart(time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC))
	if want := time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("dayStart(03:00 UTC) = %v, want %v", got, want)
	}
}

// The server's cap of images per day: it counts every campaign together, a
// refused or failed request gives its slot back, and the refusal is a typed
// reason that is not the campaign's monthly limit.
func TestMR039_TheServersDailyCap(t *testing.T) {
	t.Parallel()
	h := newHarness(t, func(c *Config) {
		withFake(&gen.Fake{}, 20)(c)
		c.DailyImages = 2
	})
	master := h.newUser("Mestre")
	one, two := h.newCampaign(master), h.newCampaign(master)

	// A refusal by the model does not count.
	master.mustGenerate(one, "x "+gen.MarkerRefuse)
	master.mustGenerate(one, "um")
	master.mustGenerate(two, "dois")

	// The month has room in both campaigns, but the server's day is spent.
	_, err := master.generate(one, "três")
	wantGenerationBlocked(t, "GenerateSceneImage over the daily cap", err, mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_DAILY_LIMIT_REACHED)
	_, err = master.generate(two, "quatro")
	wantGenerationBlocked(t, "another campaign over the daily cap", err, mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_DAILY_LIMIT_REACHED)
	if got := master.imageStatus(one).GetRemaining(); got != 19 {
		t.Errorf("the monthly remaining of the campaign = %d, want 19 (the daily cap is not the month's)", got)
	}
}

// The per-user limits of the image routes: past the burst a user gets 429
// RATE_LIMITED with Retry-After (not the gallery's QUOTA), another user is
// not affected, and a refused upload stores nothing.
func TestRateLimitsOfTheImageRoutes(t *testing.T) {
	t.Parallel()
	var nowNanos atomic.Int64 // the handlers read it from other goroutines
	nowNanos.Store(time.Now().UnixNano())
	clock := func() time.Time { return time.Unix(0, nowNanos.Load()) }
	download := ratelimit.New(ratelimit.Config{
		PerClient: ratelimit.Rate{Burst: 2, Every: time.Minute}, Global: ratelimit.Rate{Burst: 100, Every: time.Second},
		MaxClients: 10, Now: clock,
	})
	upload := ratelimit.New(ratelimit.Config{
		PerClient: ratelimit.Rate{Burst: 1, Every: time.Minute}, Global: ratelimit.Rate{Burst: 100, Every: time.Second},
		MaxClients: 10, Now: clock,
	})
	h := newHarness(t, func(c *Config) { c.DownloadLimit, c.UploadLimit = download, upload })
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)

	img := master.mustUpload(campaign, "mapa.png", pngImage(t, 40, 40))
	second := master.upload(campaign, "outro.png", pngImage(t, 40, 40))
	if second.status != http.StatusTooManyRequests || second.header.Get("Retry-After") == "" || !strings.Contains(string(second.body), `"reason":"RATE_LIMITED"`) {
		t.Fatalf("the second upload: status %d, Retry-After %q, body %s", second.status, second.header.Get("Retry-After"), second.body)
	}
	if got := len(master.list(campaign).GetImages()); got != 1 {
		t.Fatalf("the gallery has %d images, want 1: a refused upload stores nothing", got)
	}

	for range 2 {
		if res := master.get(img.GetUrl()); res.status != http.StatusOK {
			t.Fatalf("download within the burst: status %d", res.status)
		}
	}
	res := master.get(img.GetThumbnailUrl())
	if res.status != http.StatusTooManyRequests || res.header.Get("Retry-After") != "60" {
		t.Fatalf("the third download: status %d, Retry-After %q", res.status, res.header.Get("Retry-After"))
	}
	// A user with no session is not this limit's business: the handler refuses it.
	if res := h.anonymous().get(img.GetUrl()); res.status != http.StatusUnauthorized {
		t.Fatalf("anonymous download: status %d, want 401", res.status)
	}
	// The bucket refills.
	nowNanos.Add(int64(time.Minute))
	if res := master.get(img.GetUrl()); res.status != http.StatusOK {
		t.Fatalf("after the refill: status %d", res.status)
	}
}
