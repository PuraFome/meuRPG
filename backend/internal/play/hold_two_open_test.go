package play

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// A hold has no column that names its window: the request it stores does (window_id). A reactor can have two windows of one kind open
// (two damages that each ask a concentration save, two effects that each ask a save), and the one that closes forgets its own question
// and leaves the other's, with the Inspiration it spent. The second window and its hold are copies of the first's in the table, as a
// second damage or effect writes them: the closers are the functions the real ones run.

// copyRow inserts a copy of a row of the table with the expressions of the columns that change (the seq counts on).
func (h *savedHold) copyRow(t *testing.T, table, id string, set map[string]string, args ...any) string {
	t.Helper()
	rows, err := h.f.h.pool.Query(t.Context(), `SELECT column_name FROM information_schema.columns WHERE table_name = $1 AND column_name <> 'seq' ORDER BY ordinal_position`, table)
	if err != nil {
		t.Fatalf("columns of %s: %v", table, err)
	}
	var cols, exprs []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatalf("scan: %v", err)
		}
		cols = append(cols, c)
		if e, ok := set[c]; ok {
			exprs = append(exprs, e)
		} else {
			exprs = append(exprs, c)
		}
	}
	rows.Close()
	var out string
	q := `INSERT INTO ` + table + ` (` + strings.Join(cols, ",") + `) SELECT ` + strings.Join(exprs, ",") + ` FROM ` + table + ` WHERE id = $1 RETURNING id::text`
	if err := h.f.h.pool.QueryRow(t.Context(), q, append([]any{id}, args...)...).Scan(&out); err != nil {
		t.Fatalf("copy a row of %s: %v", table, err)
	}
	return out
}

// twoWindows is a held save whose reactor has a second window of the kind open, with its own held question when withHold.
type twoWindows struct {
	*savedHold
	windowKind string
	a, b       string // the windows
	holdB      string
}

func (h *savedHold) second(t *testing.T, windowKind string, withHold bool) *twoWindows {
	t.Helper()
	w := &twoWindows{savedHold: h, windowKind: windowKind}
	if err := h.f.h.pool.QueryRow(t.Context(), `SELECT id::text FROM reaction_windows WHERE kind = $1 AND status = 'open'`, windowKind).Scan(&w.a); err != nil {
		t.Fatalf("the open window: %v", err)
	}
	w.b = h.copyRow(t, "reaction_windows", w.a, map[string]string{"id": "gen_random_uuid()", "group_id": "gen_random_uuid()", "trigger": `jsonb_set(trigger, '{pending}', '"damage-b"')`})
	if withHold {
		var raw []byte
		if err := h.f.h.pool.QueryRow(t.Context(), `SELECT request FROM inspiration_holds WHERE id = $1`, h.id).Scan(&raw); err != nil {
			t.Fatalf("the request of the hold: %v", err)
		}
		var req proto.Message
		switch h.kind {
		case string(heldConcentrationSave):
			req = &playv1.ResolveConcentrationSaveRequest{}
		default:
			req = &playv1.RollEffectSaveRequest{}
		}
		if err := proto.Unmarshal(raw, req); err != nil {
			t.Fatalf("read the request: %v", err)
		}
		switch r := req.(type) {
		case *playv1.ResolveConcentrationSaveRequest:
			r.WindowId = w.b
		case *playv1.RollEffectSaveRequest:
			r.WindowId = w.b
		}
		body, err := proto.Marshal(req)
		if err != nil {
			t.Fatalf("write the request: %v", err)
		}
		w.holdB = h.copyRow(t, "inspiration_holds", h.id, map[string]string{"id": "gen_random_uuid()", "idempotency_key": "gen_random_uuid()", "request": "$2", "favor_spent": "false"}, body)
	}
	return w
}

// closeWindowOf closes the window as the invalid-window sweep does, by itself.
func (w *twoWindows) closeWindowOf(t *testing.T, id string) {
	t.Helper()
	m := authz.Membership{CampaignID: w.f.campaignID, UserID: w.f.master.id, Role: authz.RoleMaster}
	encounterID := w.f.e.GetId()
	_, err := w.f.h.svc.write(t.Context(), combatWrite{m: m, key: newKey(), kind: eventConditionsSet, encounterID: encounterID}, func(cx *combatTx) (any, error) {
		win, err := cx.q.GetReactionWindow(t.Context(), playdb.GetReactionWindowParams{EncounterID: encounterID, ID: id})
		if err != nil {
			return nil, err
		}
		if _, err := w.f.h.svc.closeWindow(t.Context(), cx, win, windowClosed, reaction.ReasonTriggerGone, windowOutcome{}, ""); err != nil {
			return nil, err
		}
		return actionEvent{Round: cx.enc.Round}, nil
	})
	if err != nil {
		t.Fatalf("close the window %s: %v", id, err)
	}
}

// undoDamage runs what the undo of a damage runs for the pending named.
func (w *twoWindows) undoDamage(t *testing.T, pending string) {
	t.Helper()
	m := authz.Membership{CampaignID: w.f.campaignID, UserID: w.f.master.id, Role: authz.RoleMaster}
	_, err := w.f.h.svc.write(t.Context(), combatWrite{m: m, key: newKey(), kind: eventConditionsSet, encounterID: w.f.e.GetId()}, func(cx *combatTx) (any, error) {
		if err := closeDamageWindows(t.Context(), cx, eventDamageRolled, actionEvent{Pending: pending}); err != nil {
			return nil, err
		}
		return actionEvent{Round: cx.enc.Round}, nil
	})
	if err != nil {
		t.Fatalf("take back a damage: %v", err)
	}
}

func (w *twoWindows) stays(t *testing.T, what string) {
	t.Helper()
	if !w.f.holdExists(t, w.id) {
		t.Errorf("%s: the question of the other window is gone", what)
	}
	if w.spent && w.f.favorHolds(t) {
		t.Errorf("%s: the Inspiration the other window's question spent came back", what)
	}
	answer := luckyAnswer(17)
	if w.hasDie {
		answer = dieAnswer(true)
	}
	user := w.f.ana
	if w.kind == string(heldEffectSave) {
		user = w.f.caio
	}
	if _, err := w.f.answerHeldRoll(t, user, w.id, answer); err != nil {
		t.Errorf("%s: the question of the other window can no longer be answered: %v", what, err)
	}
}

func TestHeldConcentrationSaveOfOneWindowSurvivesTheCloseOfAnother(t *testing.T) {
	t.Parallel()
	for _, k := range heldSaveKinds {
		t.Run("the other is taken back with "+k.name, func(t *testing.T) {
			t.Parallel()
			w := concentrationHold(t, k.die, k.spent).second(t, "concentration_save", false)
			w.undoDamage(t, "damage-b")
			w.stays(t, "the damage of the other window undone")
		})
		t.Run("the other closes by itself with "+k.name, func(t *testing.T) {
			t.Parallel()
			w := concentrationHold(t, k.die, k.spent).second(t, "concentration_save", false)
			w.closeWindowOf(t, w.b)
			w.stays(t, "the other window closed")
		})
		t.Run("this one closes and the other keeps its own with "+k.name, func(t *testing.T) {
			t.Parallel()
			w := concentrationHold(t, k.die, k.spent).second(t, "concentration_save", true)
			w.closeWindowOf(t, w.a)
			if w.f.holdExists(t, w.id) {
				t.Errorf("the question of the closed window is still there")
			}
			if !w.f.holdExists(t, w.holdB) {
				t.Errorf("the question of the other window went with it")
			}
		})
	}
}

func TestHeldEffectSaveOfOneWindowSurvivesTheCloseOfAnother(t *testing.T) {
	t.Parallel()
	for _, k := range heldSaveKinds {
		t.Run("the other closes by itself with "+k.name, func(t *testing.T) {
			t.Parallel()
			h, _ := effectHold(t, k.die, k.spent)
			w := h.second(t, "effect_save", false)
			w.closeWindowOf(t, w.b)
			w.stays(t, "the other window closed")
		})
		t.Run("this one closes and the other keeps its own with "+k.name, func(t *testing.T) {
			t.Parallel()
			h, _ := effectHold(t, k.die, k.spent)
			w := h.second(t, "effect_save", true)
			w.closeWindowOf(t, w.a)
			if w.f.holdExists(t, w.id) {
				t.Errorf("the question of the closed window is still there")
			}
			if !w.f.holdExists(t, w.holdB) {
				t.Errorf("the question of the other window went with it")
			}
		})
	}
}
