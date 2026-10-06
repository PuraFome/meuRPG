package characters

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// realisticRows are the stored entries of a table at the limit: 10 classes, 30
// subclasses, 20 races, 40 subraces, 40 backgrounds and 160 spells (300 in all),
// written the way the server stores them (keys made, data encoded).
func realisticRows(t testing.TB) []charactersdb.CampaignContent {
	t.Helper()
	var rows []charactersdb.CampaignContent
	add := func(kind rulesv1.TableContentKind, name string, body tableBody) string {
		key := entryKey(kind, name, rows)
		st, err := prepare(key, kind, body, nil)
		if err != nil {
			t.Fatalf("prepare(%s): %v", key, err)
		}
		now := time.Now()
		rows = append(rows, charactersdb.CampaignContent{
			ContentKey: key, Kind: kindPrefix(kind), NamePt: name, Data: st.data, Revision: i32(len(rows) + 1), CreatedAt: now, UpdatedAt: now,
		})
		return key
	}
	var classes, races []string
	for i := range 10 {
		classes = append(classes, add(rulesv1.TableContentKind_TABLE_CONTENT_KIND_CLASS, fmt.Sprintf("Classe %d", i), testClass(fmt.Sprintf("Classe %d", i))))
	}
	for i := range 30 {
		add(rulesv1.TableContentKind_TABLE_CONTENT_KIND_SUBCLASS, fmt.Sprintf("Caminho %d", i), testSubclass(fmt.Sprintf("Caminho %d", i), classes[i%len(classes)]))
	}
	for i := range 20 {
		races = append(races, add(rulesv1.TableContentKind_TABLE_CONTENT_KIND_RACE, fmt.Sprintf("Raça %d", i), testRace(fmt.Sprintf("Raça %d", i))))
	}
	for i := range 40 {
		add(rulesv1.TableContentKind_TABLE_CONTENT_KIND_SUBRACE, fmt.Sprintf("Sub-raça %d", i), testSubrace(fmt.Sprintf("Sub-raça %d", i), races[i%len(races)]))
	}
	for i := range 40 {
		add(rulesv1.TableContentKind_TABLE_CONTENT_KIND_BACKGROUND, fmt.Sprintf("Antecedente %d", i), testBackground(fmt.Sprintf("Antecedente %d", i)))
	}
	for i := range 160 {
		add(rulesv1.TableContentKind_TABLE_CONTENT_KIND_SPELL, fmt.Sprintf("Magia %d", i), testSpell(fmt.Sprintf("Magia %d", i), "class:wizard", classes[i%len(classes)]))
	}
	return rows
}

// TestLiveContentMemory measures what the live source keeps for a table at the
// 300-entry limit: the contents (minus the shared SRD), the catalogs ListContent
// serves for each (the master's and the players') and the time of a miss (the
// stored data decoded into an overlay, With). Run with MEURPG_MEASURE=1 and -v
// (docs/operacao.md).
func TestLiveContentMemory(t *testing.T) {
	if os.Getenv("MEURPG_MEASURE") == "" {
		t.Skip("set MEURPG_MEASURE=1 to measure")
	}
	srd := loadRules(t)
	rows := realisticRows(t)
	var data int
	for _, r := range rows {
		data += len(r.Data)
	}
	heap := func() uint64 {
		runtime.GC()
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}
	const n = maxLiveContents
	contents := make([]*rules.Content, 0, n)
	before := heap()
	start := time.Now()
	for range n {
		o, err := overlayOf(rows, 1)
		if err != nil {
			t.Fatal(err)
		}
		c, err := srd.With(o)
		if err != nil {
			t.Fatal(err)
		}
		contents = append(contents, c)
	}
	missTime := time.Since(start) / n
	afterContents := heap()
	var svc Service
	for _, c := range contents {
		svc.catalogFor(c, true)
		svc.catalogFor(c, false)
	}
	afterCatalogs := heap()
	wire := proto.Size(svc.catalogFor(contents[0], true))
	t.Logf("300 entries: %d KB of stored data; a miss (decode the data, build the overlay, With) takes %v", data/1024, missTime)
	t.Logf("%d contents keep %.2f MB each (%.1f MB together); their catalogs, the master's and the players', %.2f MB each (%.1f MB together)",
		n, float64(afterContents-before)/n/1e6, float64(afterContents-before)/1e6,
		float64(afterCatalogs-afterContents)/n/1e6, float64(afterCatalogs-afterContents)/1e6)
	t.Logf("a catalog is %d KB on the wire (spells, classes, races... with their names)", wire/1024)
	runtime.KeepAlive(contents)
	runtime.KeepAlive(&svc)
}
