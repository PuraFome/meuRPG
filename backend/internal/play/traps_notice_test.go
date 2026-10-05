package play

import (
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The passive notice reaches the player (E9-08 G) and a creature's token fires a trap
// (slice 9.14's two server gaps). These tests need the database
// (MEURPG_TEST_DATABASE_URL); the fixture is the trap rig (traps_test.go).

// settle reads a stream until it has been quiet for a moment: the events the server
// published reach the test's channel a little later (a goroutine reads the stream).
func (r *trapRig) settle(w *watcher) []*playv1.WatchGameSessionResponse {
	var out []*playv1.WatchGameSessionResponse
	for {
		select {
		case ev, ok := <-w.events:
			if !ok {
				return out
			}
			if ev.GetHeartbeat() == nil {
				out = append(out, ev)
			}
		case <-time.After(300 * time.Millisecond):
			return out
		}
	}
}

// TestRN10_ThePassiveNoticeReachesOnlyItsPlayer: when Toren notices a trap by passing
// near it, his player gets the `trap_noticed` hint and a notice line in the activity,
// and nobody else does: not the other players, not the master's stream (the master's
// card reads its own hints). Read as the app's JSON, the other players' streams and
// activity never carry the trap's ID or its name.
func TestRN10_ThePassiveNoticeReachesOnlyItsPlayer(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	pit := r.trap(t, "Fosso Dourado", 9, 7, func(s *mapsv1.TrapSpec) { s.NoticeDc = 10 })
	r.fight(t)
	master, caio, ana, bia := r.watch(t, r.master, r.campaignID), r.watch(t, r.caio, r.campaignID), r.watch(t, r.ana, r.campaignID), r.watch(t, r.bia, r.campaignID)
	r.settle(master)
	r.settle(caio)
	r.settle(ana)
	r.settle(bia)

	r.mustMove(t, r.caio, "Toren", 7, 7) // 10 ft from the pit: a passive 10 meets the DC 10

	noticed := func(evs []*playv1.WatchGameSessionResponse) (n int) {
		for _, ev := range evs {
			if ev.GetTrapNoticed() != nil {
				n++
			}
		}
		return n
	}
	got := r.settle(caio)
	if noticed(got) != 1 {
		t.Fatalf("Toren's player's stream has %d trap_noticed hints, want 1: %v", noticed(got), got)
	}
	for _, ev := range got {
		if n := ev.GetTrapNoticed(); n != nil && (n.GetMapId() != r.mapID || n.GetPointId() != pit.GetId()) {
			t.Errorf("trap_noticed = %v, want the map %s and the trap %s", n, r.mapID, pit.GetId())
		}
	}
	if n := noticed(r.settle(master)); n != 0 {
		t.Errorf("the master's stream has %d trap_noticed hints, want none", n)
	}
	for name, w := range map[string]*watcher{"Pensantus's player": ana, "Brisa's player": bia} {
		evs := r.settle(w)
		if noticed(evs) != 0 {
			t.Errorf("%s got a trap_noticed hint", name)
		}
		for _, ev := range evs {
			if js, _ := protojson.Marshal(ev); strings.Contains(string(js), pit.GetId()) || strings.Contains(string(js), "Fosso") {
				t.Errorf("%s's stream names the trap: %s", name, js)
			}
		}
	}

	// The activity: Toren's player reads the line, with the name and no DC; the master
	// reads it too; nobody else does.
	lines := r.activity(t, r.caio)
	if len(lines) != 1 || lines[0].GetNotice() == nil {
		t.Fatalf("Toren's player's activity = %v, want one notice", lines)
	}
	n := lines[0].GetNotice()
	if n.GetPointId() != pit.GetId() || n.GetTrapName() != "Fosso Dourado" || len(n.GetCharacterNames()) != 1 || n.GetCharacterNames()[0] != "Toren" {
		t.Errorf("the notice = %v, want Toren and the Fosso Dourado", n)
	}
	if js, _ := protojson.Marshal(lines[0]); strings.Contains(strings.ToLower(string(js)), "dc") {
		t.Errorf("the player's notice line carries a DC: %s", js)
	}
	if lines := r.activity(t, r.master); len(lines) != 1 || lines[0].GetNotice() == nil || lines[0].GetNotice().GetCharacterNames()[0] != "Toren" {
		t.Errorf("the master's activity = %v, want Toren's notice", lines)
	}
	for name, u := range map[string]*user{"Pensantus's player": r.ana, "Brisa's player": r.bia} {
		lines := r.activity(t, u)
		if len(lines) != 0 {
			t.Errorf("%s's activity = %v, want nothing", name, lines)
		}
		if js, _ := protojson.Marshal(&playv1.ListTrapActivityResponse{Activity: lines}); strings.Contains(string(js), pit.GetId()) {
			t.Errorf("%s's activity names the trap: %s", name, js)
		}
	}
}

// TestRN10_TheNoticeHintFollowsTheMapThePlayersSee: a token dropped on a map the
// players do not see notices nothing (traps_round2_test.go), so no hint is sent; and a
// trap that is noticed after the map is revealed is told to its player alone.
func TestRN10_TheNoticeHintFollowsTheMapThePlayersSee(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	m2 := r.secondMap(t)
	r.trapOn(t, m2, "Fosso Dourado", 5, 5, pit("1d4"), func(s *mapsv1.TrapSpec) { s.NoticeDc = 5 })
	caio, ana := r.watch(t, r.caio, r.campaignID), r.watch(t, r.ana, r.campaignID)
	r.placeOn(t, m2, r.toren.GetId(), 5, 6) // hidden map: nothing noticed, nothing said
	for _, ev := range append(r.settle(caio), r.settle(ana)...) {
		if ev.GetTrapNoticed() != nil {
			t.Fatalf("a hint %v for a notice on a map the players do not see", ev)
		}
	}
	if _, err := r.mc(r.master).SetMapRevealed(t.Context(), connect.NewRequest(&mapsv1.SetMapRevealedRequest{CampaignId: r.campaignID, MapId: m2, Revealed: true})); err != nil {
		t.Fatalf("SetMapRevealed() error = %v", err)
	}
	r.settle(caio)
	r.settle(ana)
	r.placeOn(t, m2, r.toren.GetId(), 5, 6) // revealed: Toren notices it
	var toToren, toPensantus int
	for _, ev := range r.settle(caio) {
		if n := ev.GetTrapNoticed(); n != nil && n.GetMapId() == m2 {
			toToren++
		}
	}
	for _, ev := range r.settle(ana) {
		if ev.GetTrapNoticed() != nil {
			toPensantus++
		}
	}
	if toToren != 1 || toPensantus != 0 {
		t.Errorf("hints to Toren's player = %d, to Pensantus's = %d; want 1 and 0", toToren, toPensantus)
	}
}

// TestMR035_ACreatureTokenDroppedInTheArea: with no combat on the map, the master
// dropping a creature of a player's character into an armed "Ao entrar na área" trap
// fires it, as a character's token does (TestMR035_ATokenDroppedInTheArea). The
// creature takes the damage as a line (it has no hit points stored outside a combat),
// nothing waits for the master, its owner's player reads the line with the dice and
// nobody else reads it. It does not fire twice.
func TestMR035_ACreatureTokenDroppedInTheArea(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	hole := r.trap(t, "Fosso Dourado", 12, 7, pit("1d6"))
	wolf := r.give(t, r.pens, "monster:wolf", "Nanquim")
	x, y := sq(12, 7)
	placeWolf := func() {
		t.Helper()
		if _, err := r.mc(r.master).PlaceMapToken(t.Context(), connect.NewRequest(&mapsv1.PlaceMapTokenRequest{
			CampaignId: r.campaignID, MapId: r.mapID, CreatureId: wolf.GetId(), XBp: x, YBp: y,
		})); err != nil {
			t.Fatalf("PlaceMapToken(creature) error = %v", err)
		}
	}
	state := func() mapsv1.TrapState { return r.point(t, r.master, hole.GetId()).GetTrap().GetState() }
	r.h.roller.queue(4)
	placeWolf()
	if got := state(); got != mapsv1.TrapState_TRAP_STATE_TRIGGERED {
		t.Fatalf("the trap after the wolf's token = %v, want triggered", got)
	}
	r.wantKnows(t, "Toren's player", r.caio, hole.GetId(), true) // it fired: public
	if d := r.trapDamages(t); len(d) != 0 {
		t.Errorf("trap damages = %v, want none waiting (a creature is a line only)", d)
	}

	lines := r.activity(t, r.master)
	if len(lines) != 1 || lines[0].GetFiring() == nil || len(lines[0].GetFiring().GetCaught()) != 1 {
		t.Fatalf("the master's activity = %v, want one firing that caught the wolf", lines)
	}
	got := lines[0].GetFiring().GetCaught()[0]
	if got.GetTargetLabel() != "Nanquim" || got.GetTargetId() != wolf.GetId() || got.GetCharacterId() != r.pens.GetId() ||
		len(got.GetDamages()) != 1 || got.GetDamages()[0].GetAmount() != 4 || got.GetDamages()[0].GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED {
		t.Errorf("what the trap did to the wolf = %v, want Nanquim (Pensantus's), 4 applied as a line", got)
	}

	// Pensantus's player reads the line, with the dice; Toren's player reads none.
	own := r.activity(t, r.ana)
	if len(own) != 1 || len(own[0].GetFiring().GetCaught()) != 1 || own[0].GetFiring().GetCaught()[0].GetTargetLabel() != "Nanquim" ||
		own[0].GetFiring().GetCaught()[0].GetDamages()[0].GetRoll() == nil {
		t.Errorf("the owner's activity = %v, want the wolf's line with its dice", own)
	}
	other := r.activity(t, r.caio)
	if js, _ := protojson.Marshal(&playv1.ListTrapActivityResponse{Activity: other}); len(other) != 0 || strings.Contains(string(js), "Nanquim") {
		t.Errorf("another player's activity = %s, want nothing of the wolf", js)
	}

	// It does not fire twice.
	placeWolf()
	if n := r.eventCount(t, "trap_triggered"); n != 1 {
		t.Errorf("trap_triggered events after dropping the wolf again = %d, want 1", n)
	}
}
