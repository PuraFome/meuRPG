package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The writers of vitals, resources, rests and Revivify (#735, part 2, 4b) go through Service.tx: the session's number moves once
// with the write, the events carry it, a repeat of the key moves nothing and tells again what the first call told (to the
// audiences it reached at most), and a refused call tells nothing and moves nothing.

// hintRig is a table with a stream open for each person, and the three phases every keyed writer goes through.
type hintRig struct {
	t         *testing.T
	h         *harness
	session   string
	listeners map[string]heardBy
	// quiet are the people whose streams must hear nothing of the write: it is none of their business.
	quiet []string
}

func newHintRig(t *testing.T, h *harness, campaignID string, master *user, players map[string]*user, quiet ...string) *hintRig {
	t.Helper()
	open, err := h.svc.queries.GetOpenGameSession(t.Context(), campaignID)
	if err != nil {
		t.Fatalf("GetOpenGameSession() error = %v", err)
	}
	g := &hintRig{t: t, h: h, session: open.ID, listeners: map[string]heardBy{"master": listen(t, h, campaignID, master.id, true)}, quiet: quiet}
	for name, u := range players {
		g.listeners[name] = listen(t, h, campaignID, u.id, false)
	}
	g.drain() // what the fixture's own writes sent
	return g
}

func (g *hintRig) seq() int64 { return g.h.hintSeq(g.t, g.session) }

func (g *hintRig) drain() map[string][]*playv1.WatchGameSessionResponse {
	g.t.Helper()
	out := map[string][]*playv1.WatchGameSessionResponse{}
	for name, l := range g.listeners {
		out[name] = l.heard(g.t, g.h)
	}
	return out
}

func kindsOf(heard map[string][]*playv1.WatchGameSessionResponse) map[string][]string {
	out := map[string][]string{}
	for name, evs := range heard {
		out[name] = eventKinds(evs)
	}
	return out
}

// quietHeard fails for a person of the quiet list who heard something.
func (g *hintRig) quietHeard(phase string, heard map[string][]*playv1.WatchGameSessionResponse) {
	g.t.Helper()
	for _, name := range g.quiet {
		if got := heard[name]; len(got) != 0 {
			g.t.Errorf("%s: %s heard %v, want nothing", phase, name, eventKinds(got))
		}
	}
}

// first is the write that happens: the number moves by one, and every event it tells carries it.
func (g *hintRig) first(call func() error, wantEvents bool) map[string][]string {
	g.t.Helper()
	before := g.seq()
	if err := call(); err != nil {
		g.t.Fatalf("the write: error = %v", err)
	}
	if got, want := g.seq(), before+1; got != want {
		g.t.Errorf("hint_seq = %d after the write, want %d (one more)", got, want)
	}
	heard := g.drain()
	told := 0
	for name, evs := range heard {
		for _, ev := range evs {
			told++
			if ev.GetSessionSeq() != before+1 {
				g.t.Errorf("%s heard %T with number %d, want the stored %d", name, ev.GetEvent(), ev.GetSessionSeq(), before+1)
			}
		}
	}
	if wantEvents && told == 0 {
		g.t.Errorf("nobody heard the write")
	}
	g.quietHeard("the write", heard)
	return kindsOf(heard)
}

// replay is the repeat of the key: nothing moves, and each person hears again what the first call told them (more is allowed,
// to those who heard something at all).
func (g *hintRig) replay(call func() error, first map[string][]string) {
	g.t.Helper()
	stored := g.seq()
	if err := call(); err != nil {
		g.t.Fatalf("the repeat: error = %v", err)
	}
	if got := g.seq(); got != stored {
		g.t.Errorf("hint_seq = %d after the repeat, want %d: a repeat writes nothing", got, stored)
	}
	heard := g.drain()
	for name, evs := range heard {
		for _, ev := range evs {
			if ev.GetSessionSeq() != stored {
				g.t.Errorf("%s heard %T on the repeat with number %d, want the stored %d", name, ev.GetEvent(), ev.GetSessionSeq(), stored)
			}
		}
		if len(first[name]) == 0 && len(evs) != 0 {
			g.t.Errorf("%s heard %v on the repeat but nothing of the first call: the repeat reaches more people", name, eventKinds(evs))
		}
		again := eventKinds(evs)
		for _, k := range first[name] {
			if !slices.Contains(again, k) {
				g.t.Errorf("%s heard %v on the repeat, missing %s that the first call told", name, again, k)
			}
		}
	}
	g.quietHeard("the repeat", heard)
}

// refused is a call the writer turns down: it tells nothing and moves nothing.
func (g *hintRig) refused(call func() error) {
	g.t.Helper()
	before := g.seq()
	if err := call(); err == nil {
		g.t.Fatalf("the call was not refused")
	}
	if got := g.seq(); got != before {
		g.t.Errorf("hint_seq = %d after a refused call, want %d", got, before)
	}
	for name, evs := range g.drain() {
		if len(evs) != 0 {
			g.t.Errorf("%s heard %v of a refused call", name, eventKinds(evs))
		}
	}
}

// keyed runs the three phases of a writer that takes an idempotency key: the write, its repeat and the key used for another request.
func (g *hintRig) keyed(call, again, other func() error) {
	g.t.Helper()
	var first map[string][]string
	g.t.Run("the write moves the number once and the events carry it", func(_ *testing.T) {
		first = g.first(call, true)
	})
	g.t.Run("the repeat of the key moves nothing and tells again", func(_ *testing.T) { g.replay(again, first) })
	g.t.Run("the key used for another request tells nothing", func(_ *testing.T) { g.refused(other) })
}

func TestAdjustCharacterVitalsHints(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	req := &playv1.AdjustCharacterVitalsRequest{
		CampaignId: r.campaign, CharacterId: r.fighter.GetId(), IdempotencyKey: newKey(),
		ExpectedRevision: r.master.revisionIfOpen(r.campaign, r.fighter.GetId()), HitPointsCurrent: proto.Int32(3),
	}
	send := func(msg *playv1.AdjustCharacterVitalsRequest) func() error {
		return func() error {
			_, err := r.master.play.AdjustCharacterVitals(t.Context(), connect.NewRequest(proto.Clone(msg).(*playv1.AdjustCharacterVitalsRequest)))
			return err
		}
	}
	other := proto.Clone(req).(*playv1.AdjustCharacterVitalsRequest)
	other.HitPointsCurrent = proto.Int32(4)
	g := newHintRig(t, r.h, r.campaign, r.master, map[string]*user{"Ana": r.ana, "Bia": r.bia, "Caio": r.caio}, "Bia", "Caio")
	g.keyed(send(req), send(req), send(other))
	// A correction that does not say which vitals it read is refused (aborted): it tells nothing.
	stale := proto.Clone(req).(*playv1.AdjustCharacterVitalsRequest)
	stale.IdempotencyKey, stale.ExpectedRevision = newKey(), nil
	g.refused(send(stale))
}

func TestTakeRestHints(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	r.use(t, r.fighter, func(q *playv1.AdjustCharacterVitalsRequest) {
		q.ResourcesUsed = []*playv1.ResourceUsed{{Key: "second_wind", Used: 1}}
	})
	req := &playv1.TakeRestRequest{CampaignId: r.campaign, Kind: playv1.RestKind_REST_KIND_SHORT, IdempotencyKey: newKey()}
	send := func(msg *playv1.TakeRestRequest) func() error {
		return func() error {
			_, err := r.master.resource.TakeRest(t.Context(), connect.NewRequest(proto.Clone(msg).(*playv1.TakeRestRequest)))
			return err
		}
	}
	other := proto.Clone(req).(*playv1.TakeRestRequest)
	other.Kind = playv1.RestKind_REST_KIND_LONG
	g := newHintRig(t, r.h, r.campaign, r.master, map[string]*user{"Ana": r.ana, "Caio": r.caio}, "Caio")
	g.keyed(send(req), send(req), send(other))
}

func TestSpendHitDiceHints(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	r.use(t, r.fighter, func(q *playv1.AdjustCharacterVitalsRequest) { q.HitPointsCurrent = proto.Int32(2) })
	req := &playv1.SpendHitDiceRequest{
		CampaignId: r.campaign, CharacterId: r.fighter.GetId(), Faces: 10, IdempotencyKey: newKey(),
		Roll: &playv1.SpendHitDiceRequest_RollInApp{RollInApp: true},
	}
	send := func(msg *playv1.SpendHitDiceRequest) func() error {
		return func() error {
			r.h.roller.queue(5)
			_, err := r.ana.resource.SpendHitDice(t.Context(), connect.NewRequest(proto.Clone(msg).(*playv1.SpendHitDiceRequest)))
			return err
		}
	}
	other := proto.Clone(req).(*playv1.SpendHitDiceRequest)
	other.Faces = 6
	g := newHintRig(t, r.h, r.campaign, r.master, map[string]*user{"Ana": r.ana, "Bia": r.bia}, "Bia")
	g.keyed(send(req), send(req), send(other))
}

func TestUseArcaneRecoveryHints(t *testing.T) {
	t.Parallel()
	r := newRecoveryTable(t)
	mage := r.wizard(t, r.ana, "Elora", 3)
	r.expend(t, mage, map[int32]int32{1: 4, 2: 2})
	r.shortRest(t)
	req := &playv1.UseArcaneRecoveryRequest{
		CampaignId: r.campaign, CharacterId: mage.GetId(), IdempotencyKey: newKey(), Slots: []*playv1.SpellSlotBack{{Level: 1, Count: 2}},
	}
	send := func(msg *playv1.UseArcaneRecoveryRequest) func() error {
		return func() error {
			_, err := r.ana.resource.UseArcaneRecovery(t.Context(), connect.NewRequest(proto.Clone(msg).(*playv1.UseArcaneRecoveryRequest)))
			return err
		}
	}
	other := proto.Clone(req).(*playv1.UseArcaneRecoveryRequest)
	other.Slots = []*playv1.SpellSlotBack{{Level: 1, Count: 1}}
	g := newHintRig(t, r.h, r.campaign, r.master, map[string]*user{"Ana": r.ana, "Bia": r.bia}, "Bia")
	g.keyed(send(req), send(req), send(other))
}

// The keeper spends the use and gives the slots back before the short rest is looked for: the refusal rolls it all back and
// tells nobody.
func TestUseArcaneRecoveryRefusedAfterTheWriteTellsNothing(t *testing.T) {
	t.Parallel()
	r := newRecoveryTable(t)
	mage := r.wizard(t, r.ana, "Elora", 3)
	r.expend(t, mage, map[int32]int32{1: 4})
	g := newHintRig(t, r.h, r.campaign, r.master, map[string]*user{"Ana": r.ana, "Bia": r.bia})
	g.refused(func() error {
		_, err := r.recoverSlots(t, r.ana, mage, newKey(), map[int32]int32{1: 2})
		wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_SHORT_REST)
		return err
	})
	if v := r.vitals(t, r.ana, mage); slotsUsed(v, 1) != 4 || usedOf(v, "arcane_recovery") != 0 {
		t.Errorf("after the refusal: 1st-level slots used %d, uses spent %d; want 4 and 0", slotsUsed(v, 1), usedOf(v, "arcane_recovery"))
	}
}

func TestGiveBardicInspirationOutsideHints(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	req := &playv1.GiveBardicInspirationOutsideRequest{CampaignId: a.campaignID, TargetCharacterId: a.toren.GetId(), IdempotencyKey: newKey()}
	send := func(msg *playv1.GiveBardicInspirationOutsideRequest) func() error {
		return func() error {
			_, err := a.bia.resource.GiveBardicInspirationOutside(t.Context(), connect.NewRequest(proto.Clone(msg).(*playv1.GiveBardicInspirationOutsideRequest)))
			return err
		}
	}
	other := proto.Clone(req).(*playv1.GiveBardicInspirationOutsideRequest)
	other.TargetCharacterId = a.pens.GetId()
	g := newHintRig(t, a.h, a.campaignID, a.master, map[string]*user{"Ana": a.ana, "Bia": a.bia, "Caio": a.caio})
	g.keyed(send(req), send(req), send(other))
}

func TestRequestRevivifyHints(t *testing.T) {
	t.Parallel()
	a := newVigil(t)
	key := newKey()
	g := newHintRig(t, a.h, a.campaignID, a.master, map[string]*user{"Ana": a.ana, "Bia": a.bia, "Caio": a.caio}, "Bia", "Caio")
	ask := func(target *charactersv1.Character) func() error {
		return func() error {
			_, err := a.ask(a.ana, a.pens, target, key)
			return err
		}
	}
	g.keyed(ask(a.toren), ask(a.toren), ask(a.bri))
}

func TestConfirmRevivifyTimeHints(t *testing.T) {
	t.Parallel()
	a := newVigil(t)
	asked, err := a.ask(a.ana, a.pens, a.toren, newKey())
	if err != nil {
		t.Fatalf("RequestRevivify() error = %v", err)
	}
	key := newKey()
	answer := func(within bool) func() error {
		return func() error {
			_, err := a.answerRevivify(t, a.master, asked.GetId(), within, key)
			return err
		}
	}
	g := newHintRig(t, a.h, a.campaignID, a.master, map[string]*user{"Ana": a.ana, "Bia": a.bia, "Caio": a.caio}, "Bia")
	g.keyed(answer(true), answer(true), answer(false))
}

func TestSetRevivifyBlockedHints(t *testing.T) {
	t.Parallel()
	a := newVigil(t)
	g := newHintRig(t, a.h, a.campaignID, a.master, map[string]*user{"Ana": a.ana, "Bia": a.bia, "Caio": a.caio}, "Ana", "Bia", "Caio")
	// A dead character with no combat: the switch moves the number once and tells nobody.
	g.first(func() error { return a.switchRaw(t, switchCharacter(a.toren.GetId()), true) }, false)
	// A living character is refused: nothing moves.
	g.refused(func() error { return a.switchRaw(t, switchCharacter(a.bri.GetId()), true) })
}

func TestSetRevivifyBlockedInACombatHints(t *testing.T) {
	t.Parallel()
	a, e := newMortuary(t)
	a.dies(t, e, "Toren", a.toren)
	g := newHintRig(t, a.h, a.campaignID, a.master, map[string]*user{"Ana": a.ana, "Bia": a.bia, "Caio": a.caio})
	// A dead combatant: the switch moves the number once and tells the combat.
	g.first(func() error { return a.switchRaw(t, switchCombatant(a.id(t, "Toren")), true) }, true)
	// A living combatant is refused: nothing moves.
	g.refused(func() error { return a.switchRaw(t, switchCombatant(a.id(t, "Brisa")), true) })
}
