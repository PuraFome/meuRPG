package play

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// changesHeard counts the encounter_changed messages in what a stream heard.
func changesHeard(heard []*playv1.WatchGameSessionResponse) int {
	n := 0
	for _, ev := range heard {
		if ev.GetEncounterChanged() != nil {
			n++
		}
	}
	return n
}

// The master's call of MarkReactionRolling on an open READIED window of a player's character begins the wait and nothing
// else (rolling stays the player's own): the master and the reactor's player are told, another player is not; a wait
// already begun, by a mark or by the master, writes and tells nothing, and a mark the player cleared leaves the wait as it was.
func TestTheMastersCallBeginsTheWaitOfAReadiedWindowAndNothingElse(t *testing.T) {
	t.Parallel()
	waitOf := func(a *armed, u *user) *playv1.ReactionWindow { return a.windowOf(t, u, kindReadied) }
	type who struct {
		master, reactor, other int
	}
	for _, tt := range []struct {
		name  string
		prior func(a *armed, e *playv1.Encounter, id string) // what the player did before the master's call
		told  who
	}{
		{"the wait had not begun", func(*armed, *playv1.Encounter, string) {}, who{1, 1, 0}},
		{"the player had marked", func(a *armed, e *playv1.Encounter, id string) {
			if err := a.markRolling(t, a.ana, e, id); err != nil {
				t.Fatal(err)
			}
		}, who{0, 0, 0}},
		{"the player marked and cleared", func(a *armed, e *playv1.Encounter, id string) {
			if err := a.markRolling(t, a.ana, e, id); err != nil {
				t.Fatal(err)
			}
			if err := a.clearRolling(t, a.ana, e, id); err != nil {
				t.Fatal(err)
			}
		}, who{0, 0, 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := newCasters(t)
			e := a.readyForRelease(t, magicMissileSpell, 1, "Capitão Goblin")
			w := a.mustFire(t, e, "Pensantus")
			tt.prior(a, e, w.GetId())
			before := waitOf(a, a.master)
			master, reactor, other := a.master.watch(t, a.campaignID), a.ana.watch(t, a.campaignID), a.caio.watch(t, a.campaignID)
			master.ready(t)
			reactor.ready(t)
			other.ready(t)
			a.streamsHeard(t, master, reactor, other) // what the earlier calls told
			logBefore := logLines(a.log(t, a.master, e))
			if err := a.markRolling(t, a.master, e, w.GetId()); err != nil {
				t.Fatalf("the master's call error = %v", err)
			}
			heard := a.streamsHeard(t, master, reactor, other)
			got := who{changesHeard(heard[0]), changesHeard(heard[1]), changesHeard(heard[2])}
			if (got.master > 0) != (tt.told.master > 0) || (got.reactor > 0) != (tt.told.reactor > 0) || got.other != 0 {
				t.Errorf("messages heard by the master, the reactor's player and another player = %v, want %v told", got, tt.told)
			}
			for who, u := range map[string]*user{"the master": a.master, "the reactor's player": a.ana} {
				after := waitOf(a, u)
				if after.GetWaitBeganAt() == nil {
					t.Fatalf("%s reads no beginning of the wait after the master's call", who)
				}
				if after.GetRolling() != before.GetRolling() {
					t.Errorf("%s reads rolling %v, want it as it was, %v", who, after.GetRolling(), before.GetRolling())
				}
				if before.GetWaitBeganAt() != nil && !proto.Equal(after.GetWaitBeganAt(), before.GetWaitBeganAt()) {
					t.Errorf("%s reads the wait began at %v, want it as it was, %v", who, after.GetWaitBeganAt(), before.GetWaitBeganAt())
				}
			}
			if n := logLines(a.log(t, a.master, e)); n != logBefore {
				t.Errorf("the master's call wrote %d log lines", n-logBefore)
			}
		})
	}
}

// The master's clear is still refused, and so is his call on a window that is not READIED (see the Uncanny Dodge row of
// TestMarkingAReactionAsRolledIsTheReactorsAloneAndShowsToTheMaster); after the master began the wait the player's mark
// keeps that one wait and the player's clear works.
func TestAPlayersMarkAfterTheMastersCallKeepsTheOneWait(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.readyForRelease(t, magicMissileSpell, 1, "Capitão Goblin")
	w := a.mustFire(t, e, "Pensantus")
	wantCode(t, "the master's clear", a.clearRolling(t, a.master, e, w.GetId()), connect.CodePermissionDenied)
	if got := a.windowOf(t, a.master, kindReadied); got.GetWaitBeganAt() != nil {
		t.Fatalf("a refused clear began the wait at %v", got.GetWaitBeganAt())
	}
	if err := a.markRolling(t, a.master, e, w.GetId()); err != nil {
		t.Fatalf("the master's call: %v", err)
	}
	first := a.windowOf(t, a.master, kindReadied).GetWaitBeganAt()
	if first == nil {
		t.Fatal("the master's call did not begin the wait")
	}
	a.h.clock.advance(5 * time.Second)
	if err := a.markRolling(t, a.ana, e, w.GetId()); err != nil {
		t.Fatalf("the player's mark: %v", err)
	}
	for who, u := range map[string]*user{"the master": a.master, "the reactor's player": a.ana} {
		got := a.windowOf(t, u, kindReadied)
		if !got.GetRolling() || !proto.Equal(got.GetWaitBeganAt(), first) {
			t.Errorf("%s reads rolling %v and the wait began at %v, want the mark and the master's wait %v", who, got.GetRolling(), got.GetWaitBeganAt(), first)
		}
	}
	if err := a.clearRolling(t, a.ana, e, w.GetId()); err != nil {
		t.Fatalf("the player's clear: %v", err)
	}
	if got := a.windowOf(t, a.master, kindReadied); got.GetRolling() || !proto.Equal(got.GetWaitBeganAt(), first) {
		t.Errorf("after the clear the master reads rolling %v and the wait began at %v, want no mark and %v", got.GetRolling(), got.GetWaitBeganAt(), first)
	}
}

// The master's call on a READIED window of a player's creature (a summoned wolf: its user_id is the owner's player, so the
// guard on a reactor with no player does not catch it) is refused as well: a creature has no player to wait for. The window
// stays open and its wait does not begin.
func TestTheMastersCallOnAPlayersCreaturesReadiedWindowIsRefused(t *testing.T) {
	t.Parallel()
	a := newSummoners(t)
	e := a.summonersFight(t)
	e = a.toSalvia(t, e)
	if _, err := a.summonWolves(t, a.bia, e, 12); err != nil {
		t.Fatalf("CastSpell(Conjurar Animais) error = %v", err)
	}
	e = a.passTo(t, a.get(t, a.master), "Lobo atroz 1")
	wolf := byLabel(t, e, "Lobo atroz 1")
	attacks := a.mustOptions(t, a.master, e, "Lobo atroz 1").GetOptions().GetAttacks()
	if len(attacks) == 0 {
		t.Fatal("the wolf has no attack")
	}
	a.mustReady(t, a.master, &playv1.ReadyActionRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: wolf.GetId(), IdempotencyKey: newKey(),
		TriggerText: readyTrigger, Kind: playv1.ReadiedKind_READIED_KIND_ATTACK, ActionKey: attacks[0].GetAttack().GetKey(),
	})
	e = a.mustEndTurn(t, a.master, e)
	w := a.mustFire(t, e, "Lobo atroz 1")
	wantCode(t, "the master's call on a creature's READIED window", a.markRolling(t, a.master, e, w.GetId()), connect.CodePermissionDenied)
	got := a.windowOf(t, a.master, kindReadied)
	if got == nil || got.GetId() != w.GetId() || got.GetWaitBeganAt() != nil || got.GetRolling() {
		t.Errorf("the window after the refused call = %v, want it open with no wait begun and no mark", got)
	}
}
