package play

import (
	"context"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

// The master's correction of the vitals says which revision it read (expected_revision): a
// correction replaces what is there, so one made over numbers that moved in between is refused
// (aborted) instead of undoing the change without a word. These tests need the database.

// revisionTable is a master, a player's character and an open session.
type revisionTable struct {
	h        *harness
	master   *user
	campaign string
	pc       *charactersv1.Character
}

func newRevisionTable(t *testing.T) *revisionTable {
	t.Helper()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	pc := player.createCharacter(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus")
	master.start(t, campaign)
	return &revisionTable{h: h, master: master, campaign: campaign, pc: pc}
}

// vitals reads the character's vitals as the master.
func (r *revisionTable) vitals(t *testing.T) *playv1.CharacterVitals {
	t.Helper()
	for _, v := range r.master.liveSession(t, r.campaign).GetVitals() {
		if v.GetCharacterId() == r.pc.GetId() {
			return v
		}
	}
	t.Fatalf("no vitals for %s", r.pc.GetName())
	return nil
}

// adjust sets the character's current hit points, over the revision the master read (nil: none).
func (r *revisionTable) adjust(ctx context.Context, key string, revision *int32, hp int32) (*playv1.CharacterVitals, error) {
	res, err := r.master.play.AdjustCharacterVitals(ctx, connect.NewRequest(&playv1.AdjustCharacterVitalsRequest{
		CampaignId: r.campaign, CharacterId: r.pc.GetId(), IdempotencyKey: key, HitPointsCurrent: proto.Int32(hp), ExpectedRevision: revision,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetVitals(), nil
}

func TestAdjustVitalsRefusesAMissingRevision(t *testing.T) {
	t.Parallel()
	r := newRevisionTable(t)
	before := r.vitals(t)

	_, err := r.adjust(t.Context(), newKey(), nil, before.GetHitPointsCurrent()-1)
	wantCode(t, "AdjustCharacterVitals without expected_revision", err, connect.CodeAborted)

	after := r.vitals(t)
	if !proto.Equal(before, after) {
		t.Errorf("vitals after the refused correction = %v, want them as they were (%v)", after, before)
	}
}

func TestAdjustVitalsRefusesAStaleRevision(t *testing.T) {
	t.Parallel()
	r := newRevisionTable(t)
	read := r.vitals(t)
	revision := read.GetRevision()
	first, second := read.GetHitPointsCurrent()-1, read.GetHitPointsCurrent()-2

	got, err := r.adjust(t.Context(), newKey(), proto.Int32(revision), first)
	if err != nil {
		t.Fatalf("AdjustCharacterVitals(revision %d) error = %v", revision, err)
	}
	if got.GetHitPointsCurrent() != first || got.GetRevision() != revision+1 {
		t.Fatalf("vitals after the first correction = %d hit points, revision %d; want %d, %d", got.GetHitPointsCurrent(), got.GetRevision(), first, revision+1)
	}

	// The second correction read the same numbers: it is the one that is late.
	_, err = r.adjust(t.Context(), newKey(), proto.Int32(revision), second)
	wantCode(t, "the second AdjustCharacterVitals from the same revision", err, connect.CodeAborted)
	now := r.vitals(t)
	if now.GetHitPointsCurrent() != first || now.GetRevision() != revision+1 {
		t.Errorf("vitals after the refused correction = %d hit points, revision %d; want the first one's %d, %d", now.GetHitPointsCurrent(), now.GetRevision(), first, revision+1)
	}

	// Read again, the same correction goes through.
	got, err = r.adjust(t.Context(), newKey(), proto.Int32(now.GetRevision()), second)
	if err != nil || got.GetHitPointsCurrent() != second || got.GetRevision() != revision+2 {
		t.Errorf("AdjustCharacterVitals(after reading again) = %v, %v; want %d hit points, revision %d", got, err, second, revision+2)
	}
}

func TestAdjustVitalsConcurrentCorrectionsFromOneRevisionLetOneWin(t *testing.T) {
	t.Parallel()
	r := newRevisionTable(t)
	read := r.vitals(t)
	revision := read.GetRevision()
	hps := []int32{read.GetHitPointsCurrent() - 1, read.GetHitPointsCurrent() - 2}

	type outcome struct {
		hp  int32
		err error
	}
	outcomes := make(chan outcome, len(hps))
	start := dbtest.NewBarrier(len(hps))
	var wg sync.WaitGroup
	for _, hp := range hps {
		wg.Go(func() {
			start.Wait()
			_, err := r.adjust(context.Background(), newKey(), proto.Int32(revision), hp)
			outcomes <- outcome{hp, err}
		})
	}
	wg.Wait()
	close(outcomes)

	var winners []int32
	refused := 0
	for o := range outcomes {
		switch {
		case o.err == nil:
			winners = append(winners, o.hp)
		case connect.CodeOf(o.err) == connect.CodeAborted:
			refused++
		default:
			t.Errorf("AdjustCharacterVitals(%d hit points) error = %v, want success or aborted", o.hp, o.err)
		}
	}
	if len(winners) != 1 || refused != 1 {
		t.Fatalf("%d corrections went through and %d were aborted, want exactly 1 and 1", len(winners), refused)
	}
	now := r.vitals(t)
	if now.GetHitPointsCurrent() != winners[0] || now.GetRevision() != revision+1 {
		t.Errorf("vitals = %d hit points, revision %d; want the winner's %d, revision %d", now.GetHitPointsCurrent(), now.GetRevision(), winners[0], revision+1)
	}
}

// Damage the master applies in a combat changes the vitals too: a correction made over what was
// read before it is refused, and does not give the hit points back.
func TestAdjustVitalsRefusesARevisionOlderThanADamageOfTheCombat(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.capitao.GetId(), Hidden: new(false)}},
		npcRolls: []int{19},
		players:  map[string]int32{"Toren": 12, "Pensantus": 8, "Brisa": 1},
		at:       map[string][2]int32{"Capitão Goblin": {6, 3}, "Toren": {5, 3}, "Pensantus": {10, 3}, "Brisa": {8, 8}},
	})
	read := a.vitals(t, a.toren)

	// 1d20 (15) + 4 hits Toren (CA 12); 1d6 (3) + 2 = 5 piercing, applied by the master.
	a.h.roller.queue(15, 3)
	hit := a.mustAttack(t, a.master, e, "Capitão Goblin", sword, "Toren", inAppRoll)
	pending := a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage()
	if applied, err := a.settle(t, a.master, e, pending.GetId(), true); err != nil || applied.GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED {
		t.Fatalf("ApplyPendingDamage() = %v, %v; want applied", applied, err)
	}
	hurt := a.vitals(t, a.toren)
	if hurt.GetHitPointsCurrent() != read.GetHitPointsCurrent()-5 || hurt.GetRevision() <= read.GetRevision() {
		t.Fatalf("Toren after the damage = %d hit points, revision %d; want %d, and a revision above %d",
			hurt.GetHitPointsCurrent(), hurt.GetRevision(), read.GetHitPointsCurrent()-5, read.GetRevision())
	}

	adjust := func(revision int32) error {
		_, err := a.master.adjust(t, a.campaignID, a.toren.GetId(), func(r *playv1.AdjustCharacterVitalsRequest) {
			r.HitPointsCurrent = proto.Int32(read.GetHitPointsCurrent())
			r.ExpectedRevision = proto.Int32(revision)
		})
		return err
	}
	wantCode(t, "AdjustCharacterVitals over the revision read before the damage", adjust(read.GetRevision()), connect.CodeAborted)
	if got := a.vitals(t, a.toren); got.GetHitPointsCurrent() != hurt.GetHitPointsCurrent() || got.GetRevision() != hurt.GetRevision() {
		t.Errorf("Toren after the refused correction = %d hit points, revision %d; want %d, %d", got.GetHitPointsCurrent(), got.GetRevision(), hurt.GetHitPointsCurrent(), hurt.GetRevision())
	}
	if err := adjust(hurt.GetRevision()); err != nil {
		t.Fatalf("AdjustCharacterVitals(the revision read after the damage) error = %v", err)
	}
	if got := a.vitals(t, a.toren); got.GetHitPointsCurrent() != read.GetHitPointsCurrent() {
		t.Errorf("Toren's hit points = %d, want %d (the correction)", got.GetHitPointsCurrent(), read.GetHitPointsCurrent())
	}
}

func TestAdjustVitalsRetryIsAnsweredEvenThoughTheRevisionMoved(t *testing.T) {
	t.Parallel()
	r := newRevisionTable(t)
	read := r.vitals(t)
	revision := read.GetRevision()
	hp := read.GetHitPointsCurrent() - 1
	key := newKey()

	first, err := r.adjust(t.Context(), key, proto.Int32(revision), hp)
	if err != nil {
		t.Fatalf("AdjustCharacterVitals() error = %v", err)
	}
	// Another correction moves the revision on.
	moved, err := r.adjust(t.Context(), newKey(), proto.Int32(first.GetRevision()), hp-1)
	if err != nil {
		t.Fatalf("AdjustCharacterVitals(the second correction) error = %v", err)
	}

	// The retry carries the revision it had read, which is behind now: it is answered with the vitals as
	// they are, and does not undo the second correction.
	again, err := r.adjust(t.Context(), key, proto.Int32(revision), hp)
	if err != nil {
		t.Fatalf("the retry error = %v, want the current vitals", err)
	}
	if !proto.Equal(again, moved) || again.GetHitPointsCurrent() != hp-1 || again.GetRevision() != revision+2 {
		t.Errorf("the retry = %v; want the current vitals (%d hit points, revision %d)", again, hp-1, revision+2)
	}
	if now := r.vitals(t); !proto.Equal(now, moved) {
		t.Errorf("vitals after the retry = %v, want them as they were (%v)", now, moved)
	}
}
