package play

import (
	"slices"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The pass and the grant in a combat (favor_pass.go, combat_log_favor.go, combat_undo_favor.go): the log's lines, who reads each, the
// master's undo of the pass and of a grant. Toren is Caio's, Pensantus is Ana's and Brisa is Bia's, all in the casters' combat.
// Written under the rule that no database test runs in this task: these have not run.

// favorTexts are the sentences about the Inspiration u reads in the combat's log, oldest first.
func (f *favorFight) favorTexts(t *testing.T, u *user) []string {
	t.Helper()
	var out []string
	for _, l := range slices.Backward(logEntries(f.log(t, u, f.e))) {
		if l.GetFavor() != nil {
			out = append(out, l.GetFavor().GetTextPt())
		}
	}
	return out
}

// wantTexts checks what each reader reads: nothing for a name missing from want.
func (f *favorFight) wantTexts(t *testing.T, what string, want map[*user][]string) {
	t.Helper()
	for _, u := range []*user{f.master, f.caio, f.ana, f.bia} {
		got := f.favorTexts(t, u)
		if !slices.Equal(got, want[u]) {
			t.Errorf("%s: a reader reads %q, want %q", what, got, want[u])
		}
	}
}

// eventRow reads the encounter column of the newest event of the kind.
func (f *favorFight) eventEncounter(t *testing.T, kind string) (encounter *string, round int) {
	t.Helper()
	err := f.h.pool.QueryRow(t.Context(), `SELECT encounter_id::text, COALESCE((payload->>'round')::int, 0) FROM session_events WHERE kind = $1 AND game_session_id = $2 ORDER BY seq DESC LIMIT 1`, kind, f.sessionID).Scan(&encounter, &round)
	if err != nil {
		t.Fatalf("read the newest %s event: %v", kind, err)
	}
	return encounter, round
}

// Each answer to a pass in a combat is a line of its log, in the words of each reader; a player the pass is not about reads nothing.
func TestPassAnsweredInACombatWritesALinePerReader(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		do   func(t *testing.T, f *favorFight, id string)
		kind string
		want func(f *favorFight) map[*user][]string
	}{
		{"accepted", func(t *testing.T, f *favorFight, id string) {
			if _, err := f.answer(id, true); err != nil {
				t.Fatalf("accept: %v", err)
			}
		}, eventFavorPassed, func(f *favorFight) map[*user][]string {
			return map[*user][]string{
				f.master: {"Toren passou a Inspiração para Pensantus."},
				f.caio:   {"O mestre aceitou: a sua Inspiração foi para Pensantus."},
				f.ana:    {"Toren passou a Inspiração para você."},
			}
		}},
		{"refused", func(t *testing.T, f *favorFight, id string) {
			if _, err := f.answer(id, false); err != nil {
				t.Fatalf("refuse: %v", err)
			}
		}, eventFavorAskRefused, func(f *favorFight) map[*user][]string {
			return map[*user][]string{
				f.master: {"O mestre recusou o pedido de Toren: a Inspiração continua com Toren."},
				f.caio:   {"O mestre recusou: a Inspiração continua com você."},
			}
		}},
		{"canceled", func(t *testing.T, f *favorFight, id string) {
			if _, err := f.cancelAs(f.caio, id, newKey()); err != nil {
				t.Fatalf("cancel: %v", err)
			}
		}, eventFavorPassCanceled, func(f *favorFight) map[*user][]string {
			return map[*user][]string{
				f.master: {"Toren desistiu de passar a Inspiração para Pensantus."},
				f.caio:   {"Você desistiu de passar a Inspiração."},
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFavorFight(t)
			f.give(t, f.toren)
			p := f.mustAsk(t, f.toren.GetId(), f.pens.GetId())
			f.wantTexts(t, "the ask", nil) // an ask is no line
			tc.do(t, f, p.GetId())
			f.wantTexts(t, tc.name, tc.want(f))
			if enc, round := f.eventEncounter(t, tc.kind); enc == nil || *enc != f.e.GetId() || round < 1 {
				t.Errorf("the %s event carries encounter %v and round %d, want the open combat and its round", tc.kind, enc, round)
			}
		})
	}
}

// Outside a combat the answers write no line and carry no combat: the answers show in GetFavor and the live hint.
func TestPassOutsideACombatWritesNoLineAndNoCombat(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	if err := f.grant(f.toren.GetId()); err != nil {
		t.Fatal(err)
	}
	p := f.mustAsk(t, f.toren.GetId(), f.pens.GetId())
	if _, err := f.answer(p.GetId(), true); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{eventFavorGranted, eventFavorPassed} {
		var n int
		if err := f.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM session_events WHERE kind = $1 AND game_session_id = $2 AND encounter_id IS NOT NULL`, kind, f.sessionID).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s events with a combat outside one = %d (%v), want 0", kind, n, err)
		}
	}
}

// A pass answered with a hidden combatant in it is the master's line alone, whoever the hidden one is and even after the master shows it
// (RN-10): the same audience rule as the rest of the log.
func TestPassLineOfAHiddenCombatantIsTheMastersAlone(t *testing.T) {
	t.Parallel()
	for _, hidden := range []string{"Toren", "Pensantus"} {
		t.Run(hidden+" hidden", func(t *testing.T) {
			t.Parallel()
			f := newFavorFight(t)
			f.give(t, f.toren)
			p := f.mustAsk(t, f.toren.GetId(), f.pens.GetId())
			hideAgain(t, f.armed, f.e, hidden)
			if _, err := f.answer(p.GetId(), true); err != nil {
				t.Fatalf("accept: %v", err)
			}
			want := map[*user][]string{f.master: {"Toren passou a Inspiração para Pensantus."}}
			f.wantTexts(t, "the line with "+hidden+" hidden", want)
			showNPC(t, f.armed, f.e, hidden)
			f.wantTexts(t, "the line after "+hidden+" is shown", want)
		})
	}
}

// The master's undo of an accepted pass is one step: the receiver loses it and the giver has it again, the line goes, and the receiver
// reads the one sentence that names nobody. A third player hears nothing of it.
func TestUndoOfAnAcceptedPass(t *testing.T) {
	t.Parallel()
	f := newFavorFight(t)
	f.give(t, f.toren)
	p := f.mustAsk(t, f.toren.GetId(), f.pens.GetId())
	if _, err := f.answer(p.GetId(), true); err != nil {
		t.Fatal(err)
	}
	id := f.log(t, f.master, f.e).GetUndoableEventId()
	if id == "" {
		t.Fatal("the accepted pass is not the last action of the combat")
	}
	ws := f.passWatchers(t)
	before := f.seq(t)
	if err := f.undo(t, f.master, f.e, id); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	if !f.has(t, f.toren.GetId()) || f.has(t, f.pens.GetId()) {
		t.Errorf("rows: Toren %v, Pensantus %v, want it back with Toren and gone from Pensantus", f.has(t, f.toren.GetId()), f.has(t, f.pens.GetId()))
	}
	if now := f.seq(t); now != before+1 {
		t.Errorf("hint_seq = %d, want %d (one step)", now, before+1)
	}
	f.wantTexts(t, "the undone pass", map[*user][]string{f.ana: {"O mestre desfez a passagem: a Inspiração não está mais com você."}})
	if l := f.lineOf(t, f.ana, playv1.CombatLogKind_COMBAT_LOG_KIND_FAVOR_PASS_ANSWERED); l.GetActorId() != "" || l.GetActorLabel() != "" {
		t.Errorf("the receiver's line names %q %q, want nobody", l.GetActorId(), l.GetActorLabel())
	}
	for name, got := range f.heardBy(t, ws) {
		if name == "bia" && len(got) != 0 {
			t.Errorf("a third player heard %v of the undo, want nothing", got)
		}
	}
}

// The undo is refused, and changes nothing, when the receiver spent it (UNDO_TARGET_CHANGED, the reason the other undos use when the
// target changed) or when the giver has another by then (ALREADY_HAS).
func TestUndoOfAnAcceptedPassRefusals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change string
		check  func(t *testing.T, err error)
	}{
		{"the receiver spent it", `DELETE FROM character_favor WHERE character_id = $1`, func(t *testing.T, err error) {
			if got := blockedOf(t, err).GetReason(); got != playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_UNDO_TARGET_CHANGED {
				t.Errorf("reason = %v, want UNDO_TARGET_CHANGED", got)
			}
		}},
		{"the giver has another", `INSERT INTO character_favor (character_id, campaign_id) SELECT $1, campaign_id FROM characters WHERE id = $1`, func(t *testing.T, err error) {
			wantFavorBlocked(t, "the undo", err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_ALREADY_HAS)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFavorFight(t)
			f.give(t, f.toren)
			p := f.mustAsk(t, f.toren.GetId(), f.pens.GetId())
			if _, err := f.answer(p.GetId(), true); err != nil {
				t.Fatal(err)
			}
			id := f.log(t, f.master, f.e).GetUndoableEventId()
			target := f.pens.GetId()
			if tc.name == "the giver has another" {
				target = f.toren.GetId()
			}
			f.execSQL(t, tc.change, target)
			rows, seq := f.rows(t), f.seq(t)
			tc.check(t, f.undo(t, f.master, f.e, id))
			if f.rows(t) != rows || f.seq(t) != seq || f.eventsOf(t, eventActionUndone) != 0 {
				t.Errorf("the refused undo changed rows %d->%d, number %d->%d, undo events %d", rows, f.rows(t), seq, f.seq(t), f.eventsOf(t, eventActionUndone))
			}
		})
	}
}

// The undo of a refused or canceled pass only closes the line: nothing moves.
func TestUndoOfARefusedPassOnlyClosesTheLine(t *testing.T) {
	t.Parallel()
	f := newFavorFight(t)
	f.give(t, f.toren)
	p := f.mustAsk(t, f.toren.GetId(), f.pens.GetId())
	if _, err := f.answer(p.GetId(), false); err != nil {
		t.Fatal(err)
	}
	id := f.log(t, f.master, f.e).GetUndoableEventId()
	if id == "" {
		t.Fatal("the refusal is not the last action of the combat")
	}
	if err := f.undo(t, f.master, f.e, id); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	f.wantTexts(t, "the undone refusal", nil)
	if !f.has(t, f.toren.GetId()) || f.has(t, f.pens.GetId()) || f.passStatus(t, p.GetId()) != "refused" {
		t.Error("the undo of a refusal moved the Inspiration or the pass")
	}
}

// The master's undo of a grant takes it back, in the combat the grant was made in; a grant already spent is refused.
func TestUndoOfAGrant(t *testing.T) {
	t.Parallel()
	f := newFavorFight(t)
	f.give(t, f.toren)
	id := f.log(t, f.master, f.e).GetUndoableEventId()
	if id == "" {
		t.Fatal("the grant is not the last action of the combat")
	}
	if enc, _ := f.eventEncounter(t, eventFavorGranted); enc == nil {
		t.Fatal("the grant carries no combat")
	}
	f.execSQL(t, `DELETE FROM character_favor WHERE character_id = $1`, f.toren.GetId()) // spent
	if got := blockedOf(t, f.undo(t, f.master, f.e, id)).GetReason(); got != playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_UNDO_TARGET_CHANGED {
		t.Errorf("the undo of a spent grant = %v, want UNDO_TARGET_CHANGED", got)
	}
	f.give(t, f.pens)
	id = f.log(t, f.master, f.e).GetUndoableEventId()
	if err := f.undo(t, f.master, f.e, id); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	if f.has(t, f.pens.GetId()) {
		t.Error("the undo did not take the grant back")
	}
	if f.log(t, f.master, f.e).GetUndoableEventId() == id {
		t.Error("the undone grant is still the last action")
	}
}
