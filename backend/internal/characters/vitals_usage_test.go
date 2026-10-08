package characters

import (
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
)

func (h *harness) adjustVitals(campaign, characterID string, req *playv1.AdjustCharacterVitalsRequest) {
	h.t.Helper()
	err := db.InTx(h.t.Context(), h.pool, func(tx pgx.Tx) error {
		_, _, err := h.svc.AdjustVitals(h.t.Context(), tx, campaign, characterID, req)
		return err
	})
	if err != nil {
		h.t.Fatalf("AdjustVitals() error = %v", err)
	}
}

// wizardAt is Pensantus at a level, with what only a higher level gives removed.
func wizardAt(level int32) *charactersv1.CharacterSheet {
	s := pensantusSheet()
	f := s.GetFull()
	f.Classes[0].Level = level
	if level < 2 {
		f.Classes[0].Subclass = nil
		keep := func(keys []string) []string {
			return slices.DeleteFunc(slices.Clone(keys), func(k string) bool {
				return slices.Contains([]string{"spell:scorching-ray", "spell:web"}, k)
			})
		}
		f.KnownSpellKeys, f.PreparedSpellKeys = keep(f.KnownSpellKeys), keep(f.PreparedSpellKeys)
	}
	return s
}

// Usage is clamped when read, never when written: an edit of something else while the sheet has fewer slots keeps the stored usage of the slots it lost.
func TestAdjustVitalsKeepsUsageOfSlotsTheSheetLost(t *testing.T) {
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	pc := player.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", wizardAt(5))
	h.adjustVitals(campaign, pc.GetId(), &playv1.AdjustCharacterVitalsRequest{
		SpellSlotsUsed: []*playv1.SpellSlotsUsed{{Level: 3, Used: 1}},
	})
	pc, err := master.update(t, pc, pc.GetName(), wizardAt(1))
	if err != nil {
		t.Fatalf("lower the sheet: %v", err)
	}
	tmp := int32(5)
	h.adjustVitals(campaign, pc.GetId(), &playv1.AdjustCharacterVitalsRequest{HitPointsTemporary: &tmp})
	if _, err := master.update(t, pc, pc.GetName(), wizardAt(5)); err != nil {
		t.Fatalf("raise the sheet: %v", err)
	}
	v, err := h.svc.GetVitals(t.Context(), campaign, pc.GetId())
	if err != nil {
		t.Fatalf("GetVitals() error = %v", err)
	}
	var used int32 = -1
	for _, s := range v.GetSpellSlots() {
		if s.GetLevel() == 3 {
			used = s.GetUsed()
		}
	}
	if used != 1 {
		t.Errorf("3rd-level slots used = %d, want 1 (stored usage must survive an unrelated edit)", used)
	}
}

// The same holds for resources: a resource the sheet lacks for now, and a use above today's total, stay as stored.
func TestAdjustVitalsKeepsStoredResourceUsage(t *testing.T) {
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	pc := player.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", wizardAt(5))
	h.adjustVitals(campaign, pc.GetId(), &playv1.AdjustCharacterVitalsRequest{HitPointsCurrent: new(int32(1))})
	if _, err := h.pool.Exec(t.Context(),
		`UPDATE character_vitals SET resources_used = '{"gone-for-now": 2}'::JSONB WHERE character_id = $1`, pc.GetId()); err != nil {
		t.Fatalf("store usage: %v", err)
	}
	tmp := int32(3)
	h.adjustVitals(campaign, pc.GetId(), &playv1.AdjustCharacterVitalsRequest{HitPointsTemporary: &tmp})
	var got int32
	if err := h.pool.QueryRow(t.Context(),
		`SELECT COALESCE((resources_used->>'gone-for-now')::INT4, 0) FROM character_vitals WHERE character_id = $1`, pc.GetId()).Scan(&got); err != nil {
		t.Fatalf("read usage: %v", err)
	}
	if got != 2 {
		t.Errorf("stored usage of a resource the sheet lacks = %d, want 2", got)
	}
}

// A character whose hit points were never set has full hit points, however high
// the maximum goes: a first write of something else (temporary hit points, a
// slot, the familiar's sight) creates the vitals row without pinning the current
// hit points to the maximum of that moment, so a level-up after it shows the same
// as for a character with no row.
func TestAFirstVitalsWriteThatLeavesHitPointsAloneKeepsThemFull(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	players := make([]*user, 5)
	for i := range players {
		players[i] = h.newUser("Jogador")
	}
	campaign := h.newCampaign(master, "Mirathel", players...)
	next := 0
	create := func(name string) *charactersv1.Character { // each player has one living character
		next++
		return players[next-1].create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, name, wizardAt(1))
	}
	untouched := create("Sem linha")
	byTemporary := create("Com PV temporários")
	bySlot := create("Com espaço gasto")
	bySight := create("Com olhos do familiar")
	tmp := int32(2)
	h.adjustVitals(campaign, byTemporary.GetId(), &playv1.AdjustCharacterVitalsRequest{HitPointsTemporary: &tmp})
	h.adjustVitals(campaign, bySlot.GetId(), &playv1.AdjustCharacterVitalsRequest{SpellSlotsUsed: []*playv1.SpellSlotsUsed{{Level: 1, Used: 1}}})
	if err := db.InTx(t.Context(), h.pool, func(tx pgx.Tx) error {
		_, err := h.svc.SetFamiliarSight(t.Context(), tx, campaign, bySight.GetId(), "", false, nil)
		return err
	}); err != nil {
		t.Fatalf("SetFamiliarSight() error = %v", err)
	}

	for _, c := range []*charactersv1.Character{untouched, byTemporary, bySlot, bySight} {
		if _, err := master.update(t, c, c.GetName(), wizardAt(5)); err != nil {
			t.Fatalf("raise %s: %v", c.GetName(), err)
		}
		v, err := h.svc.GetVitals(t.Context(), campaign, c.GetId())
		if err != nil {
			t.Fatalf("GetVitals(%s) error = %v", c.GetName(), err)
		}
		if v.GetHitPointsMax() <= 9 || v.GetHitPointsCurrent() != v.GetHitPointsMax() {
			t.Errorf("%s after the level-up: %d/%d hit points, want full (%d)", c.GetName(), v.GetHitPointsCurrent(), v.GetHitPointsMax(), v.GetHitPointsMax())
		}
	}
	// What was set rises with the maximum: a wound is still a wound after the level-up.
	wounded := create("Ferida")
	h.adjustVitals(campaign, wounded.GetId(), &playv1.AdjustCharacterVitalsRequest{HitPointsCurrent: new(int32(4))})
	if _, err := master.update(t, wounded, wounded.GetName(), wizardAt(5)); err != nil {
		t.Fatalf("raise the wounded: %v", err)
	}
	v, err := h.svc.GetVitals(t.Context(), campaign, wounded.GetId())
	if want := 4 + v.GetHitPointsMax() - wounded.GetDerived().GetHitPointsMax(); err != nil || v.GetHitPointsCurrent() != want {
		t.Errorf("a wounded character after the level-up: %d hit points (%v), want %d: the 4 it had plus what the maximum gained", v.GetHitPointsCurrent(), err, want)
	}
}
