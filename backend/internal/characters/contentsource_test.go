package characters

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// contentAsk is one call to ContentSource.For.
type contentAsk struct {
	campaignID string
	inTx       bool
}

// recordingSource is the SRD source that remembers who asked for what: the guard
// that every read of the content asks for its own campaign's (MR-025, ADR-0018),
// and that a read inside a transaction passes it (PR #121).
type recordingSource struct {
	inner ContentSource
	mu    sync.Mutex
	asks  []contentAsk
}

func (r *recordingSource) For(ctx context.Context, tx pgx.Tx, campaignID string) (*rules.Content, TableRules, error) {
	r.mu.Lock()
	r.asks = append(r.asks, contentAsk{campaignID: campaignID, inTx: tx != nil})
	r.mu.Unlock()
	return r.inner.For(ctx, tx, campaignID)
}

// take returns what was asked since the last take.
func (r *recordingSource) take() []contentAsk {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.asks
	r.asks = nil
	return out
}

func TestSRDSourceGivesEveryCampaignTheSRDAndTheDefaults(t *testing.T) {
	t.Parallel()
	c := loadRules(t)
	src := NewSRDSource(c)
	for _, campaign := range []string{"", "a", "b"} {
		got, tr, err := src.For(t.Context(), nil, campaign)
		if err != nil || got != c {
			t.Fatalf("For(%q) = %p, %v; want the SRD content", campaign, got, err)
		}
		if !reflect.DeepEqual(tr, TableRules{}) || tr.HitPoints != HitPointsPlayerChooses {
			t.Errorf("For(%q) table rules = %+v, want the zero value (the SRD defaults)", campaign, tr)
		}
	}
}

func TestEveryReadAsksForItsOwnCampaignsContent(t *testing.T) {
	dice := &testDiceRules{}
	dice.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_PLAYER_CHOOSES))
	var rec *recordingSource
	h := newHarnessWith(t, func(c *Config) {
		rec = &recordingSource{inner: c.Content}
		c.Content, c.Dice = rec, dice
	})
	h.svc.SetLevelUps(xpLevelUps{})
	master, owner := h.newUser("Samuel"), h.newUser("Dona")
	campA := h.newCampaign(master, "Mirathel", owner)
	campB := h.newCampaign(master, "Outra mesa", owner)

	sheet := pensantusSheet()
	sheet.GetFull().ExperiencePoints = 2700 // level 4 is his
	pcA := owner.create(t, campA, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus A", sheet)
	pcB := owner.create(t, campB, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus B", pensantusSheet())
	h.lockSheets(campA) // the level-up needs the locked sheet
	pcA = owner.get(t, campA, pcA.GetId())

	ctx := t.Context()
	steps := []struct {
		name     string
		campaign string
		wantTx   bool // some ask passed its transaction
		allTx    bool // every ask did: the whole step runs in one
		run      func() error
	}{
		{"GetCharacter", campB, false, false, func() error {
			_, err := owner.api.GetCharacter(ctx, connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: campB, CharacterId: pcB.GetId()}))
			return err
		}},
		{"ListContent", campB, false, false, func() error {
			_, err := owner.content.ListContent(ctx, connect.NewRequest(&rulesv1.ListContentRequest{CampaignId: campB}))
			return err
		}},
		{"UpdateCharacter validates with its campaign's content", campB, false, false, func() error {
			_, err := owner.update(t, pcB, "Pensantus B", pensantusSheet())
			return err
		}},
		{"GetLevelUpOptions", campA, false, false, func() error {
			_, err := owner.api.GetLevelUpOptions(ctx, connect.NewRequest(&charactersv1.GetLevelUpOptionsRequest{CampaignId: campA, CharacterId: pcA.GetId()}))
			return err
		}},
		{"LevelUpCharacter writes with its transaction", campA, true, false, func() error {
			_, err := owner.api.LevelUpCharacter(ctx, connect.NewRequest(&charactersv1.LevelUpCharacterRequest{
				CampaignId: campA, CharacterId: pcA.GetId(), Revision: pcA.GetRevision(), Choices: pensantusLevelUp(),
			}))
			return err
		}},
		{"CombatSheet (a direct call of play's seam)", campB, false, false, func() error {
			_, err := h.svc.CombatSheet(ctx, nil, campB, pcB.GetId())
			return err
		}},
		{"CombatSheet inside a transaction", campB, true, true, func() error {
			return db.InTx(ctx, h.pool, func(tx pgx.Tx) error {
				_, err := h.svc.CombatSheet(ctx, tx, campB, pcB.GetId())
				return err
			})
		}},
	}
	rec.take()
	for _, tt := range steps {
		if err := tt.run(); err != nil {
			t.Fatalf("%s: error = %v", tt.name, err)
		}
		asks := rec.take()
		if len(asks) == 0 {
			t.Errorf("%s: the content source was never asked", tt.name)
		}
		sawTx := false
		for _, a := range asks {
			if tt.allTx && !a.inTx {
				t.Errorf("%s: an ask had no transaction (asks %+v)", tt.name, asks)
			}
			if a.campaignID != tt.campaign {
				t.Errorf("%s: asked for campaign %q, want %q", tt.name, a.campaignID, tt.campaign)
			}
			sawTx = sawTx || a.inTx
		}
		if sawTx != tt.wantTx {
			t.Errorf("%s: some ask passed its transaction = %v, want %v (asks %+v)", tt.name, sawTx, tt.wantTx, asks)
		}
	}
}

func TestCatalogIsBuiltOncePerContentAndBounded(t *testing.T) {
	t.Parallel()
	s := offlineService(t)
	a := loadRules(t)
	first := s.catalogFor(a, true)
	if again := s.catalogFor(a, true); first != again { // the same content: one catalog
		t.Error("catalogFor() built the catalog of one content twice")
	}
	if first == nil || len(first.GetRaces()) == 0 {
		t.Error("catalogFor() has no races")
	}
	b, err := rules.LoadSRD() // another content (another pointer): its own catalog
	if err != nil {
		t.Fatal(err)
	}
	if s.catalogFor(b, true) == first {
		t.Error("two contents share a catalog")
	}
	for range maxCatalogs { // more contents than the cache keeps: the oldest goes
		c, err := rules.LoadSRD()
		if err != nil {
			t.Fatal(err)
		}
		s.catalogFor(c, true)
	}
	s.catalogs.mu.Lock()
	n, kept := len(s.catalogs.catalog), s.catalogs.catalog[a]
	s.catalogs.mu.Unlock()
	if n > maxCatalogs || kept != nil {
		t.Errorf("the cache keeps %d catalogs (the first still there: %v), want at most %d and the oldest dropped", n, kept != nil, maxCatalogs)
	}
}
