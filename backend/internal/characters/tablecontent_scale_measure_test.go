package characters

import (
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
)

// bookRows are the stored entries of a table of n entries as the books make
// it, written the way the server stores them: mostly spells with a few
// paragraphs of text, then subclasses, subraces, races, feats, backgrounds and
// a few classes.
func bookRows(t testing.TB, n, extraEffects int) []charactersdb.CampaignContent {
	t.Helper()
	// The extra effects go to the subclasses', races', backgrounds' and feats'
	// features, the same number on each, of the books' mix of types.
	bodies := n/9 + n/14 + n/20 + n/12
	perBody := min(extraEffects/max(bodies, 1), 19)
	mix := func(k int) []*rulesv1.TableEffect {
		out := make([]*rulesv1.TableEffect, 0, perBody)
		for i := range perBody {
			switch (i + k) % 6 {
			case 0:
				out = append(out, &rulesv1.TableEffect{Type: "modifier", Target: "ac", Mode: "add", Value: fmt.Sprintf(`max(1, mod("str") + %d)`, (k+i)%100+1)})
			case 1:
				out = append(out, &rulesv1.TableEffect{Type: "proficiency", Proficiency: "skill:survival"})
			case 2:
				out = append(out, &rulesv1.TableEffect{Type: "resistance", Except: "damage-type:psychic", While: "rage"})
			case 3:
				out = append(out, &rulesv1.TableEffect{Type: "note", TextPt: fmt.Sprintf("Nota do livro %d.", k+i)})
			case 4:
				out = append(out, &rulesv1.TableEffect{Type: "choice", Choice: "skill", Count: 1})
			default:
				out = append(out, &rulesv1.TableEffect{Type: "resource", Resource: fmt.Sprintf("recurso_%d_%d", k, i), Max: "prof()", Recharge: "short_rest"})
			}
		}
		return out
	}
	classes, subclasses, races := n/70, n/9, n/14
	subraces, backgrounds, feats := n/12, n/20, n/12
	spells := n - classes - subclasses - races - subraces - backgrounds - feats
	var rows []charactersdb.CampaignContent
	add := func(kind rulesv1.TableContentKind, name string, body tableBody) string {
		key := entryKey(kind, name, rows)
		st, err := prepare(key, kind, body, nil)
		if err != nil {
			t.Fatalf("prepare(%s): %v", key, err)
		}
		now := time.Now()
		rows = append(rows, charactersdb.CampaignContent{
			ContentKey: key, Kind: kindPrefix(kind), NamePt: name, Data: st.data, Revision: 1, CreatedAt: now, UpdatedAt: now,
		})
		return key
	}
	var classKeys, raceKeys []string
	for i := range classes {
		name := fmt.Sprintf("Classe %d", i)
		classKeys = append(classKeys, add(rulesv1.TableContentKind_TABLE_CONTENT_KIND_CLASS, name, testClass(name)))
	}
	for i := range subclasses {
		name := fmt.Sprintf("Caminho %d", i)
		parent := "class:fighter"
		if i%2 == 0 {
			parent = classKeys[i%len(classKeys)]
		}
		sc := testSubclass(name, parent)
		sc.Levels[0].Features[0].Effects = append(sc.Levels[0].Features[0].Effects, mix(i)...)
		add(rulesv1.TableContentKind_TABLE_CONTENT_KIND_SUBCLASS, name, sc)
	}
	for i := range races {
		name := fmt.Sprintf("Raça %d", i)
		r := testRace(name)
		r.Traits[0].Effects = append(r.Traits[0].Effects, mix(i+1)...)
		raceKeys = append(raceKeys, add(rulesv1.TableContentKind_TABLE_CONTENT_KIND_RACE, name, r))
	}
	for i := range subraces {
		name := fmt.Sprintf("Sub-raça %d", i)
		add(rulesv1.TableContentKind_TABLE_CONTENT_KIND_SUBRACE, name, testSubrace(name, raceKeys[i%len(raceKeys)]))
	}
	for i := range backgrounds {
		name := fmt.Sprintf("Antecedente %d", i)
		bg := testBackground(name)
		bg.Feature.Effects = append(bg.Feature.Effects, mix(i+2)...)
		add(rulesv1.TableContentKind_TABLE_CONTENT_KIND_BACKGROUND, name, bg)
	}
	for i := range feats {
		name := fmt.Sprintf("Talento %d", i)
		f := testFeat(name)
		f.Effects = append(f.Effects, mix(i+3)...)
		add(rulesv1.TableContentKind_TABLE_CONTENT_KIND_FEAT, name, f)
	}
	for i := range spells {
		name := fmt.Sprintf("Magia %d", i)
		sp := testSpell(name, "class:wizard", classKeys[i%len(classKeys)])
		sp.Level = int32(1 + i%9)
		sp.DescPt = nil
		for p := range 3 {
			sp.DescPt = append(sp.DescPt, strings.Repeat(fmt.Sprintf("Texto %d do livro, parágrafo %d: a criatura alvo faz o que a descrição manda. ", i, p), 4))
		}
		add(rulesv1.TableContentKind_TABLE_CONTENT_KIND_SPELL, name, sp)
	}
	return rows
}

// insertRows writes the rows straight into the campaign (each through the API
// would take minutes), at content revision 1.
func (h *harness) insertRows(campaign string, rows []charactersdb.CampaignContent) {
	h.t.Helper()
	for _, r := range rows {
		if _, err := h.pool.Exec(h.t.Context(), `INSERT INTO campaign_content (campaign_id, content_key, kind, name_pt, data, revision, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 1, now(), now())`, campaign, r.ContentKey, r.Kind, r.NamePt, r.Data); err != nil {
			h.t.Fatalf("insert %s: %v", r.ContentKey, err)
		}
	}
	if _, err := h.pool.Exec(h.t.Context(), "INSERT INTO campaign_content_state (campaign_id, revision, updated_at) VALUES ($1, 1, now()) ON CONFLICT (campaign_id) DO UPDATE SET revision = 1", campaign); err != nil {
		h.t.Fatalf("insert the revision: %v", err)
	}
}

// median runs fn reps times and returns the median duration and the average
// allocations and bytes of one run.
func median(reps int, fn func()) (d time.Duration, allocs, bytes uint64) {
	times := make([]time.Duration, 0, reps)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range reps {
		start := time.Now()
		fn()
		times = append(times, time.Since(start))
	}
	runtime.ReadMemStats(&after)
	slices.Sort(times)
	return times[len(times)/2], (after.Mallocs - before.Mallocs) / uint64(reps), (after.TotalAlloc - before.TotalAlloc) / uint64(reps)
}

// TestTableContentScale measures a campaign whose table has 300, 450, 600, 800
// and 1000 entries, through the services and the database: the cache miss and
// hit of the live source and what a cached content keeps, a sheet read, the
// level-up options, the "Magias" page, the lists the master and the players
// read, and the master's switches. Run it with MEURPG_MEASURE=1, without
// -race, and -v (docs/performance.md).
func TestTableContentScale(t *testing.T) {
	if os.Getenv("MEURPG_MEASURE") == "" {
		t.Skip("set MEURPG_MEASURE=1 to measure")
	}
	for _, n := range []int{300, 450, 600, 800, 1000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) { measureScale(t, n, 0) })
	}
	t.Run("1000 entries with 6000 effects", func(t *testing.T) { measureScale(t, 1000, 5000) })
}

func measureScale(t *testing.T, n, extraEffects int) {
	tb := newLevelUpTable(t, 2700, 5)
	rows := bookRows(t, n, extraEffects)
	var stored int
	var tableKeys []string
	for _, r := range rows {
		stored += len(r.Data)
		tableKeys = append(tableKeys, r.ContentKey)
	}
	tb.h.insertRows(tb.campaign, rows)
	ctx := t.Context()

	// The live source: a miss (revision read, rows read and decoded, With), a hit,
	// and what a cached content keeps.
	src := NewTableSource(tb.h.pool, loadRules(t), nil)
	heap := func() uint64 {
		runtime.GC()
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}
	h0 := heap()
	start := time.Now()
	c, err := src.ContentFor(ctx, nil, tb.campaign)
	if err != nil {
		t.Fatal(err)
	}
	miss := time.Since(start)
	h1 := heap()
	hit, _, _ := median(20, func() {
		if _, err := src.ContentFor(ctx, nil, tb.campaign); err != nil {
			t.Fatal(err)
		}
	})
	runtime.KeepAlive(c)
	t.Logf("%4d entries, %d KiB stored: live source miss %v, hit %v, a cached content keeps %.2f MB", n, stored/1024, miss.Round(time.Millisecond), hit.Round(time.Microsecond), float64(h1-h0)/1e6)

	// A sheet read, hot (the service's own source misses once first).
	tb.owner.get(t, tb.campaign, tb.pc.GetId())
	d, a, b := median(20, func() { tb.owner.get(t, tb.campaign, tb.pc.GetId()) })
	t.Logf("%4d entries: sheet read (GetCharacter) %v, %d allocs, %d KiB", n, d.Round(time.Microsecond), a, b/1024)
	d, a, b = median(20, func() {
		if _, err := tb.options(tb.owner, tb.pc); err != nil {
			t.Fatal(err)
		}
	})
	t.Logf("%4d entries: level-up options read %v, %d allocs, %d KiB", n, d.Round(time.Microsecond), a, b/1024)

	// The "Magias" page: the whole list in pages of the largest size, and the first
	// page with a class filter, as a player.
	var pages int
	d, a, b = median(10, func() {
		pages = 0
		token := ""
		for {
			res := tb.owner.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: tb.campaign, PageSize: maxSpellsPage, PageToken: token})
			pages++
			if token = res.GetNextPageToken(); token == "" {
				return
			}
		}
	})
	t.Logf("%4d entries: Magias, every page (%d) %v, %d allocs, %d KiB", n, pages, d.Round(time.Microsecond), a, b/1024)
	d, _, _ = median(10, func() {
		tb.owner.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: tb.campaign, ClassKey: "class:wizard", PageSize: 100})
	})
	t.Logf("%4d entries: Magias, first page of 100 for the wizard %v", n, d.Round(time.Microsecond))

	// What the master and the players read.
	var listed *rulesv1.ListTableEntriesResponse
	d, _, _ = median(5, func() {
		res, err := tb.master.table.ListTableEntries(ctx, connect.NewRequest(&rulesv1.ListTableEntriesRequest{CampaignId: tb.campaign}))
		if err != nil {
			t.Fatal(err)
		}
		listed = res.Msg
	})
	t.Logf("%4d entries: ListTableEntries %v, %d KiB on the wire", n, d.Round(time.Microsecond), proto.Size(listed)/1024)
	exported, err := tb.master.table.ExportTableContent(ctx, connect.NewRequest(&rulesv1.ExportTableContentRequest{CampaignId: tb.campaign}))
	if err != nil {
		t.Fatal(err)
	}
	packJSON, err := storeJSON.Marshal(exported.Msg.GetPack())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%4d entries: the exported pack is %d KiB of JSON (the limit is %d KiB)", n, len(packJSON)/1024, MaxPackBytes/1024)
	var catalog *rulesv1.ListContentResponse
	d, _, _ = median(5, func() { catalog = tb.owner.catalog(t, tb.campaign) })
	t.Logf("%4d entries: ListContent as a player %v, %d KiB on the wire", n, d.Round(time.Microsecond), proto.Size(catalog)/1024)
	d, _, _ = median(5, func() { catalog = tb.master.catalog(t, tb.campaign) })
	t.Logf("%4d entries: ListContent as the master %v, %d KiB on the wire", n, d.Round(time.Microsecond), proto.Size(catalog)/1024)

	// The master's switches: the list, then the table's entries switched off and
	// on in one call (the call is refused above MaxOptionSwitches).
	var opts *rulesv1.ListOptionSwitchesResponse
	d, _, _ = median(5, func() {
		res, err := tb.master.table.ListOptionSwitches(ctx, connect.NewRequest(&rulesv1.ListOptionSwitchesRequest{CampaignId: tb.campaign}))
		if err != nil {
			t.Fatal(err)
		}
		opts = res.Msg
	})
	t.Logf("%4d entries: ListOptionSwitches %v, %d options, %d KiB on the wire", n, d.Round(time.Microsecond), len(opts.GetOptions()), proto.Size(opts)/1024)
	var srdKeys []string
	for _, o := range opts.GetOptions() {
		if !o.GetTable() {
			srdKeys = append(srdKeys, o.GetKey())
		}
	}
	for _, k := range []int{450, 700, 1000, 1358} {
		keys := append(slices.Clone(tableKeys), srdKeys...)
		if k > len(keys) {
			continue
		}
		keys = keys[:k]
		start := time.Now()
		res, err := tb.master.table.SetOptionSwitches(ctx, connect.NewRequest(switchReq(tb.campaign, true, keys...)))
		took := time.Since(start)
		if err != nil {
			t.Logf("%4d entries: SetOptionSwitches off of %d keys: %v", n, k, err)
			continue
		}
		start = time.Now()
		if _, err := tb.master.table.SetOptionSwitches(ctx, connect.NewRequest(switchReq(tb.campaign, false, keys...))); err != nil {
			t.Fatal(err)
		}
		t.Logf("%4d entries: SetOptionSwitches off of %d keys %v (%d changed, %d KiB answer), back on %v", n, k, took.Round(time.Millisecond), res.Msg.GetChanged(), proto.Size(res.Msg)/1024, time.Since(start).Round(time.Millisecond))
	}
}
