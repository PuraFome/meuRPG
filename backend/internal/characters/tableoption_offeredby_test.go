package characters

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
)

// The feature that offers a table option is named to a player only when the player may read the feature (RN-23).

func TestAPlayerCopyOfAnOptionNamesNoHiddenOfferingFeature(t *testing.T) {
	t.Parallel()
	const offerer = "feature:class-guardiao-3f9a1c--vigor@mesa"
	e := entryRow{
		row:  charactersdb.CampaignContent{ContentKey: "option:table_feature:postura@mesa"},
		kind: rulesv1.TableContentKind_TABLE_CONTENT_KIND_OPTION,
		body: &rulesv1.TableOption{NamePt: "Postura", Set: "table_feature", OfferedBy: offerer},
	}
	offeredBy := func(r entryRow) string { return r.body.(*rulesv1.TableOption).GetOfferedBy() }

	// The master reads everything, so nothing is hidden.
	if got := offeredBy(forPlayer(e, nil, nil)); got != offerer {
		t.Errorf("master: offered_by = %q, want %q", got, offerer)
	}
	// A player of a table where the class is archived.
	hiddenAll := func(k string) bool { return k == offerer }
	if got := offeredBy(forPlayer(e, hiddenAll, nil)); got != "" {
		t.Errorf("player, feature hidden: offered_by = %q, want it blank", got)
	}
	if got := offeredBy(e); got != offerer {
		t.Errorf("the stored body changed: offered_by = %q", got)
	}
	// A player who may read the feature keeps it.
	none := func(string) bool { return false }
	if got := offeredBy(forPlayer(e, none, nil)); got != offerer {
		t.Errorf("player, feature visible: offered_by = %q, want %q", got, offerer)
	}
}

// ListTableEntries is the only read that carries `offered_by`: no other answer to a player has the field.
func TestListTableEntriesNamesTheOfferingFeatureOnlyWhileAPlayerMayReadIt(t *testing.T) {
	t.Parallel()
	tb := newChoiceTable(t)
	sub := tb.master.addEntry(t, tb.campaign, &rulesv1.TableSubclass{
		NamePt: "Guarda do Corvo", ClassKey: "class:fighter",
		Levels: []*rulesv1.TableSubclassLevel{{Level: 3, Features: []*rulesv1.TableFeature{{
			NamePt: "Truques de lâmina", Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: "feature", Count: 1}},
			Options: []*rulesv1.TableFeature{{NamePt: "Corte baixo"}},
		}}}},
	})
	feature := sub.GetTableSubclass().GetLevels()[0].GetFeatures()[0]
	opt := tb.master.addEntry(t, tb.campaign, &rulesv1.TableOption{NamePt: "Golpe do Corvo", Set: "table_feature", OfferedBy: feature.GetKey()})

	list := func(u *user) (offeredBy string, body string) {
		t.Helper()
		res, err := u.table.ListTableEntries(t.Context(), connect.NewRequest(&rulesv1.ListTableEntriesRequest{CampaignId: tb.campaign}))
		if err != nil {
			t.Fatalf("ListTableEntries() error = %v", err)
		}
		for _, e := range res.Msg.GetEntries() {
			if e.GetKey() == opt.GetKey() {
				offeredBy = e.GetTableOption().GetOfferedBy()
			}
		}
		return offeredBy, protojson.Format(res.Msg)
	}
	reads := func(body string) bool {
		return strings.Contains(body, feature.GetKey()) || strings.Contains(body, "Truques de lâmina")
	}

	// While the subclass is live both read the key; the player reads it as the option's offering feature.
	if got, _ := list(tb.master); got != feature.GetKey() {
		t.Fatalf("master: offered_by = %q, want %q", got, feature.GetKey())
	}
	if got, _ := list(tb.ana); got != feature.GetKey() {
		t.Fatalf("player, live subclass: offered_by = %q, want %q", got, feature.GetKey())
	}
	// Archived or switched off, the subclass and what it offers are the master's: the player reads neither its key nor its name.
	for name, hide := range map[string]func(){
		"archived": func() {
			if _, err := tb.master.table.ArchiveTableEntry(t.Context(), connect.NewRequest(&rulesv1.ArchiveTableEntryRequest{CampaignId: tb.campaign, Key: sub.GetKey()})); err != nil {
				t.Fatalf("ArchiveTableEntry() error = %v", err)
			}
		},
		"off": func() {
			if _, err := tb.master.table.UnarchiveTableEntry(t.Context(), connect.NewRequest(&rulesv1.UnarchiveTableEntryRequest{CampaignId: tb.campaign, Key: sub.GetKey()})); err != nil {
				t.Fatalf("UnarchiveTableEntry() error = %v", err)
			}
			tb.master.setOff(t, tb.campaign, true, sub.GetKey())
		},
	} {
		hide()
		if got, body := list(tb.master); got != feature.GetKey() || !reads(body) {
			t.Errorf("%s, master: offered_by = %q, reads the feature = %v, want the key and the name", name, got, reads(body))
		}
		if got, body := list(tb.ana); got != "" || reads(body) {
			t.Errorf("%s, player: offered_by = %q, reads the feature = %v, want neither", name, got, reads(body))
		}
	}
}
