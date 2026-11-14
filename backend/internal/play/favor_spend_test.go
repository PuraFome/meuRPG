package play

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The spend of the Inspiration (favor_spend.go): the helper every roll path calls, its adapters and the hold that remembers it.

func TestFavorModeCancelsAgainstDisadvantage(t *testing.T) {
	t.Parallel()
	adv := combat.Source{Kind: combat.SourceProneTarget, Effect: combat.ModeAdvantage}
	dis := combat.Source{Kind: combat.SourcePoisonedAttacker, Effect: combat.ModeDisadvantage}
	for name, tt := range map[string]struct {
		sources []combat.Source
		want    combat.RollMode
	}{
		"none":                                {nil, combat.ModeAdvantage},
		"disadvantage only":                   {[]combat.Source{dis}, combat.ModeNormal},
		"two disadvantages":                   {[]combat.Source{dis, dis}, combat.ModeNormal},
		"advantage":                           {[]combat.Source{adv}, combat.ModeAdvantage},
		"advantage and disadvantage":          {[]combat.Source{adv, dis}, combat.ModeNormal},
		"two advantages and one disadvantage": {[]combat.Source{adv, adv, dis}, combat.ModeNormal},
	} {
		if got := favorMode(tt.sources); got != tt.want {
			t.Errorf("favorMode(%s) = %v, want %v", name, got, tt.want)
		}
	}
	if got := notesSources([]rollNote{{Kind: noteHelp, Adv: true}, {Kind: notePoisoned}, {Kind: noteEffectBonus, Value: 2}}); len(got) != 2 || got[0].Effect != combat.ModeAdvantage || got[1].Effect != combat.ModeDisadvantage {
		t.Errorf("notesSources = %+v, want an advantage and a disadvantage, and no bonus", got)
	}
}

// sourcesOf is the sources of a roll that has the mode: none for a normal one.
func sourcesOf(m combat.RollMode) []combat.Source {
	switch m {
	case combat.ModeAdvantage:
		return []combat.Source{{Kind: combat.SourceProneTarget, Effect: combat.ModeAdvantage}}
	case combat.ModeDisadvantage:
		return []combat.Source{{Kind: combat.SourcePoisonedAttacker, Effect: combat.ModeDisadvantage}}
	case combat.ModeNormal:
	}
	return nil
}

func TestFavorAdaptersCountTheSourceOnlyWhenUsed(t *testing.T) {
	t.Parallel()
	dis := combat.Source{Kind: combat.SourcePoisonedAttacker, Effect: combat.ModeDisadvantage}
	names := func(k string) string { return k }
	used := favorUse{Mode: combat.ModeNormal, Used: true}

	m := modeResult{Mode: combat.ModeDisadvantage, Sources: []combat.Source{dis}}
	if got := m.withFavor(favorUse{Mode: combat.ModeDisadvantage}, names); got.Mode != combat.ModeDisadvantage || len(got.Sources) != 1 || len(got.Shown) != 0 {
		t.Errorf("modeResult.withFavor without a spend = %+v, want it unchanged", got)
	}
	got := m.withFavor(used, names)
	if got.Mode != combat.ModeNormal || len(got.Sources) != 2 || got.Sources[1] != favorSource {
		t.Errorf("modeResult.withFavor = %+v, want a normal roll with the favor source last", got)
	}
	if len(got.Shown) != 1 || got.Shown[0].GetTextPt() != "Inspiração" || got.Shown[0].GetKind() != playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_FAVOR {
		t.Errorf("modeResult.withFavor shows %v, want the sentence Inspiração of kind FAVOR", got.Shown)
	}
	mixed := modeResult{Mode: combat.ModeNormal, Sources: append(sourcesOf(combat.ModeAdvantage), sourcesOf(combat.ModeDisadvantage)...)}
	if got := mixed.withFavor(favorUse{Mode: combat.ModeNormal, Used: true}, names); got.Mode != combat.ModeNormal {
		t.Errorf("modeResult.withFavor on canceled sources = %v, want normal", got.Mode)
	}
	if len(m.Sources) != 1 {
		t.Errorf("modeResult.withFavor changed the sources of the roll it was called on: %v", m.Sources)
	}

	cm := checkMode{Mode: combat.ModeNormal}
	if got := cm.withFavor(favorUse{Mode: combat.ModeNormal}); got.Mode != combat.ModeNormal || len(got.Sources) != 0 {
		t.Errorf("checkMode.withFavor without a spend = %+v, want it unchanged", got)
	}
	got2 := cm.withFavor(favorUse{Mode: combat.ModeAdvantage, Used: true})
	if got2.Mode != combat.ModeAdvantage || !slices.Equal(got2.Sources, []combat.Source{favorSource}) {
		t.Errorf("checkMode.withFavor = %+v, want advantage with the favor source", got2)
	}

	p := preparedCheck{notes: []rollNote{{Kind: notePoisoned, Label: labelPoisoned}}, mode: combat.CheckDisadvantage}
	if got := p.withFavor(favorUse{Mode: combat.ModeDisadvantage}); got.mode != combat.CheckDisadvantage || len(got.notes) != 1 {
		t.Errorf("preparedCheck.withFavor without a spend = %+v, want it unchanged", got)
	}
	got3 := p.withFavor(used)
	if got3.mode != combat.CheckNormal || len(got3.notes) != 2 || got3.notes[1] != (rollNote{Kind: noteFavor, Label: labelFavor, Adv: true}) {
		t.Errorf("preparedCheck.withFavor = %+v, want a normal check with the favor note last", got3)
	}
	if notesMode(got3.notes) != combat.CheckNormal {
		t.Errorf("notesMode of the favored notes = %v, want normal", notesMode(got3.notes))
	}
	if got4 := (preparedCheck{mode: combat.CheckNormal}).withFavor(favorUse{Mode: combat.ModeAdvantage, Used: true}); got4.mode != combat.CheckAdvantage {
		t.Errorf("preparedCheck.withFavor on a normal check = %v, want advantage", got4.mode)
	}
}

// The sentence of the Inspiration is the roller's own: no hide function turns it into "Outra fonte", for the master or the player.
func TestFavorSentenceIsNeverHidden(t *testing.T) {
	t.Parallel()
	names := func(k string) string { return k }
	hides := map[string]func(combat.Source) bool{
		"no hide":          nil,
		"hideUnlessMaster": hideUnlessMaster(false),
		"hidesSunlight":    hidesSunlight,
		"hiddenFrom":       modeFacts{}.hiddenFrom(combatViewer{}, playdb.Combatant{}, playdb.Combatant{}),
		"hideFor":          hideFor([]string{"condition:poisoned"}, true),
	}
	for name, hide := range hides {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			shown := sourceProto(favorSource, textContext{self: true, names: names}, hide)
			if shown.GetTextPt() != "Inspiração" || shown.GetKind() != playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_FAVOR || shown.GetEffect() != playv1.RollMode_ROLL_MODE_ADVANTAGE {
				t.Errorf("sourceProto = %v, want the sentence Inspiração of kind FAVOR with advantage", shown)
			}
			notes := notesOfSources([]combat.Source{favorSource}, hide)
			if len(notes) != 1 || notes[0].Label != "Inspiração" || notes[0].Kind != combat.SourceFavor || !notes[0].Adv {
				t.Errorf("notesOfSources = %+v, want the favor note", notes)
			}
		})
	}
	note := rollNote{Kind: noteFavor, Label: labelFavor, Adv: true}
	for _, master := range []bool{true, false} {
		if got := note.proto(master); got.GetLabelPt() != "Inspiração" || got.GetKind() != "favor" || !got.GetAdvantage() {
			t.Errorf("rollNote.proto(master=%v) = %v, want Inspiração", master, got)
		}
	}
}

func TestNewRollKeepsTheSpend(t *testing.T) {
	t.Parallel()
	for _, spent := range []bool{true, false} {
		ae := answering{spec: rollSpec{Kind: heldSceneCheck, Key: "k", FavorSpent: spent, Roller: heldRoller{CampaignID: "c", CharacterID: "p", UserID: "u"}}}
		if got := ae.newRoll(nil, d20Roll{Faces: []int{12}}, nil); got.FavorSpent != spent {
			t.Errorf("newRoll with FavorSpent=%v kept %v", spent, got.FavorSpent)
		}
	}
}

// spendTry is one call of the helper in a transaction of its own that commits whatever the helper answered, so that a
// refusal that left its mark would be found afterwards.
type spendTry struct {
	use favorUse
	err error
}

func (f *favorParty) tryFavor(ctx context.Context, a favorAsk) spendTry {
	var out spendTry
	err := db.InTx(ctx, f.h.pool, func(tx pgx.Tx) error {
		out.use, out.err = f.h.svc.useFavor(ctx, f.h.svc.outsideGate(tx, f.h.svc.queries.WithTx(tx)), a)
		return nil
	})
	if err != nil {
		f.h.t.Fatalf("the transaction of the helper failed: %v", err)
	}
	return out
}

// ask is what a scene check of Toren's player asks of the helper.
func (f *favorParty) ask(mode combat.RollMode) favorAsk {
	return favorAsk{Want: true, CampaignID: f.campaignID, CharacterID: f.toren.GetId(), Own: true, Key: newKey(), Kind: heldSceneCheck, Sources: sourcesOf(mode), Rolls: true}
}

// holdOf keeps a held roll of Toren's under the key, with or without the Inspiration.
func (f *favorParty) holdOf(t *testing.T, key string, spent bool) heldRoll {
	t.Helper()
	var held heldRoll
	err := db.InTx(t.Context(), f.h.pool, func(tx pgx.Tx) error {
		env := f.h.svc.outsideGate(tx, f.h.svc.queries.WithTx(tx))
		var err error
		held, err = newHeldStores(env.q).insert(t.Context(), env, heldRoll{
			CampaignID: f.campaignID, CharacterID: f.toren.GetId(), UserID: f.caio.id, Key: key, Kind: heldSceneCheck,
			Request: []byte("{}"), Faces: []int{12}, Total: 12, FavorSpent: spent,
		})
		return err
	})
	if err != nil {
		t.Fatalf("hold the roll: %v", err)
	}
	return held
}

func (f *favorParty) favorHolds(t *testing.T) bool {
	t.Helper()
	return f.rows(t) == 1
}

func TestUseFavorDecisionTable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		mode       combat.RollMode
		edit       func(*favorAsk)
		want       combat.RollMode
		used       bool
		refusal    playv1.FavorBlockedReason
		hasFavor   bool // the character has the Inspiration before the call
		stillHolds bool // the character has it after
	}{
		{name: "not wanted", mode: combat.ModeNormal, edit: func(a *favorAsk) { a.Want = false }, want: combat.ModeNormal, hasFavor: true, stillHolds: true},
		{name: "not wanted by a character that has none", mode: combat.ModeNormal, edit: func(a *favorAsk) { a.Want = false }, want: combat.ModeNormal},
		{name: "normal becomes advantage", mode: combat.ModeNormal, want: combat.ModeAdvantage, used: true, hasFavor: true},
		{name: "disadvantage becomes normal", mode: combat.ModeDisadvantage, want: combat.ModeNormal, used: true, hasFavor: true},
		{name: "advantage adds nothing", mode: combat.ModeAdvantage, refusal: playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_ADDS_NOTHING, hasFavor: true, stillHolds: true},
		{name: "advantage and disadvantage cancel and add nothing", mode: combat.ModeNormal, edit: func(a *favorAsk) {
			a.Sources = append(sourcesOf(combat.ModeAdvantage), sourcesOf(combat.ModeDisadvantage)...)
		}, refusal: playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_ADDS_NOTHING, hasFavor: true, stillHolds: true},
		{name: "two advantages and a disadvantage add nothing", mode: combat.ModeNormal, edit: func(a *favorAsk) {
			a.Sources = append(sourcesOf(combat.ModeAdvantage), sourcesOf(combat.ModeAdvantage)[0], sourcesOf(combat.ModeDisadvantage)[0])
		}, refusal: playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_ADDS_NOTHING, hasFavor: true, stillHolds: true},
		{name: "none to spend", mode: combat.ModeNormal, refusal: playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_HAS_NONE},
		{name: "not the holder's own roll", mode: combat.ModeNormal, edit: func(a *favorAsk) { a.Own = false }, refusal: playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_NOT_THE_HOLDER, hasFavor: true, stillHolds: true},
		{name: "no d20 to roll", mode: combat.ModeNormal, edit: func(a *favorAsk) { a.Rolls = false }, refusal: playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_NOTHING_TO_ROLL, hasFavor: true, stillHolds: true},
		{name: "not the holder wins over no d20", mode: combat.ModeNormal, edit: func(a *favorAsk) { a.Own, a.Rolls = false, false }, refusal: playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_NOT_THE_HOLDER, hasFavor: true, stillHolds: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFavorParty(t)
			if tt.hasFavor {
				if err := f.grant(f.toren.GetId()); err != nil {
					t.Fatalf("grant: %v", err)
				}
			}
			a := f.ask(tt.mode)
			if tt.edit != nil {
				tt.edit(&a)
			}
			before := f.seq(t)
			got := f.tryFavor(t.Context(), a)
			if tt.refusal != playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_UNSPECIFIED {
				wantFavorBlocked(t, "useFavor", got.err, tt.refusal)
			} else if got.err != nil {
				t.Fatalf("useFavor error = %v", got.err)
			}
			if tt.refusal == playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_UNSPECIFIED && (got.use.Mode != tt.want || got.use.Used != tt.used || got.use.Spent != tt.used) {
				t.Errorf("useFavor = %+v, want mode %v used and spent %v", got.use, tt.want, tt.used)
			}
			if f.favorHolds(t) != tt.stillHolds {
				t.Errorf("the character has the Inspiration after = %v, want %v", f.favorHolds(t), tt.stillHolds)
			}
			if after := f.seq(t); after != before {
				t.Errorf("hint_seq moved from %d to %d: the helper moves no number", before, after)
			}
		})
	}
}

// The d20 is already rolled when a held roll exists under the request's key and was made without the Inspiration.
func TestUseFavorRefusesAfterTheRollOnAFreshRequest(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	if err := f.grant(f.toren.GetId()); err != nil {
		t.Fatalf("grant: %v", err)
	}
	a := f.ask(combat.ModeNormal)
	f.holdOf(t, a.Key, false)
	before := f.seq(t)
	got := f.tryFavor(t.Context(), a)
	wantFavorBlocked(t, "useFavor on a roll already made", got.err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_AFTER_THE_ROLL)
	if !f.favorHolds(t) || f.seq(t) != before {
		t.Errorf("the refusal left the Inspiration = %v and moved the hint number from %d to %d", f.favorHolds(t), before, f.seq(t))
	}
	// A hold of another kind, or one a reaction window kept, is not this request's roll.
	other := f.ask(combat.ModeNormal)
	other.Kind = heldHintTry
	f.holdOf(t, other.Key, false)
	if got := f.tryFavor(t.Context(), other); got.err != nil || !got.use.Spent {
		t.Errorf("useFavor under a hold of another kind = %+v, %v, want it spent", got.use, got.err)
	}
}

// A retry of a held roll made with the Inspiration finds the row gone: it counts without spending and never says HAS_NONE.
func TestUseFavorOnTheRetryOfAHeldRollReadsTheHold(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t) // Toren has none: the first call spent it
	a := f.ask(combat.ModeNormal)
	f.holdOf(t, a.Key, true)
	got := f.tryFavor(t.Context(), a)
	if got.err != nil || !got.use.Used || got.use.Spent || got.use.Mode != combat.ModeAdvantage {
		t.Fatalf("the retry = %+v, %v, want used, not spent, advantage", got.use, got.err)
	}
	if f.favorHolds(t) {
		t.Error("the retry gave the Inspiration back")
	}
	// The retry works the mode out again from the truth: a disadvantage is canceled, and an advantage stays.
	for in, want := range map[combat.RollMode]combat.RollMode{combat.ModeDisadvantage: combat.ModeNormal, combat.ModeAdvantage: combat.ModeAdvantage} {
		a.Sources = sourcesOf(in)
		if got := f.tryFavor(t.Context(), a); got.err != nil || got.use.Mode != want || !got.use.Used || got.use.Spent {
			t.Errorf("the retry on %v = %+v, %v, want mode %v used and not spent", in, got.use, got.err, want)
		}
	}
	// It does not need to be the holder's call: the hold decided it, so a retry that no longer says Own is not refused.
	a.Own, a.Rolls = false, false
	if got := f.tryFavor(t.Context(), a); got.err != nil || !got.use.Used {
		t.Errorf("the retry with no Own = %+v, %v, want it used", got.use, got.err)
	}
}

// The answer to a held roll runs the path again with the request that made it: it reads the hold, and never spends or refuses.
func TestUseFavorOnTheAnswerToAHeldRollReadsTheHold(t *testing.T) {
	t.Parallel()
	for _, spent := range []bool{true, false} {
		f := newFavorParty(t)
		if err := f.grant(f.toren.GetId()); err != nil {
			t.Fatalf("grant: %v", err)
		}
		a := f.ask(combat.ModeNormal)
		hold := f.holdOf(t, a.Key, spent)
		ctx := withHeldReply(t.Context(), heldReply{holdID: hold.ID, key: newKey()})
		for _, edit := range []func(*favorAsk){
			func(*favorAsk) {},
			func(a *favorAsk) { a.Own = false }, // the master answers: the roller is still who rolls
			func(a *favorAsk) { a.Sources = sourcesOf(combat.ModeAdvantage) },
			func(a *favorAsk) { a.Rolls = false },
		} {
			b := a
			edit(&b)
			got := f.tryFavor(ctx, b)
			if got.err != nil || got.use.Used != spent || got.use.Spent {
				t.Errorf("the answer with spent=%v and ask %+v = %+v, %v, want used=%v and not spent", spent, b, got.use, got.err, spent)
			}
		}
		if !f.favorHolds(t) {
			t.Errorf("an answer with spent=%v took the character's Inspiration", spent)
		}
	}
}

func TestHeldRollKeepsTheSpendInTheHold(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	for _, spent := range []bool{true, false} {
		key := newKey()
		f.holdOf(t, key, spent)
		got, found, err := newHeldStores(f.h.svc.queries).byKeyOf(t.Context(), heldSceneCheck, heldRoller{CampaignID: f.campaignID}, key)
		if err != nil || !found || got.FavorSpent != spent {
			t.Errorf("the hold read back = %+v, found %v, %v, want FavorSpent=%v", got, found, err, spent)
		}
	}
}

func TestUseFavorOfTwentyRacersSpendsItOnce(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 8) // the racers must overlap
	f := newFavorParty(t)
	if err := f.grant(f.toren.GetId()); err != nil {
		t.Fatalf("grant: %v", err)
	}
	const racers = 20
	results := make([]spendTry, racers)
	var start, wg sync.WaitGroup
	start.Add(1)
	for i := range racers {
		wg.Go(func() {
			start.Wait()
			a := f.ask(combat.ModeNormal)
			// As a path does: the error leaves the closure, so a serialization failure retries it and a refusal rolls back.
			results[i].err = db.InTx(t.Context(), f.h.pool, func(tx pgx.Tx) error {
				var err error
				results[i].use, err = f.h.svc.useFavor(t.Context(), f.h.svc.outsideGate(tx, f.h.svc.queries.WithTx(tx)), a)
				return err
			})
		})
	}
	start.Done()
	wg.Wait()
	won := 0
	for _, r := range results {
		if r.err == nil {
			won++
			continue
		}
		wantFavorBlocked(t, "a racing spend", r.err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_HAS_NONE)
	}
	if won != 1 {
		t.Errorf("%d of %d racing spends won, want exactly 1", won, racers)
	}
	if f.favorHolds(t) {
		t.Error("the Inspiration is still there after the spend")
	}
}

// A roll whose transaction fails gives the Inspiration back by itself: the row goes only when the roll is written.
func TestUseFavorIsPartOfTheCallersTransaction(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	if err := f.grant(f.toren.GetId()); err != nil {
		t.Fatalf("grant: %v", err)
	}
	errStop := context.Canceled
	err := db.InTx(t.Context(), f.h.pool, func(tx pgx.Tx) error {
		if _, err := f.h.svc.useFavor(t.Context(), f.h.svc.outsideGate(tx, f.h.svc.queries.WithTx(tx)), f.ask(combat.ModeNormal)); err != nil {
			return err
		}
		return errStop
	})
	if err == nil {
		t.Fatal("the transaction committed")
	}
	if !f.favorHolds(t) {
		t.Error("the spend of a roll that was not written is gone")
	}
}

func TestFavorGiveBackIsIdempotent(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	give := func() {
		t.Helper()
		if err := favorGiveBack(t.Context(), f.h.svc.queries, f.campaignID, f.toren.GetId()); err != nil {
			t.Fatalf("favorGiveBack() error = %v", err)
		}
	}
	give()
	give()
	if got := f.rows(t); got != 1 {
		t.Fatalf("character_favor has %d rows after two give-backs, want 1", got)
	}
	if got := f.tryFavor(t.Context(), f.ask(combat.ModeNormal)); got.err != nil || !got.use.Spent {
		t.Fatalf("spend after a give-back = %+v, %v", got.use, got.err)
	}
	give()
	if !f.favorHolds(t) {
		t.Error("the give-back did not put the Inspiration back")
	}
}
