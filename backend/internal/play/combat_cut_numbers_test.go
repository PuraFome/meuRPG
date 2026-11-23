package play

import (
	"reflect"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// The two places that complete a Cutting Words answer from the roll it cut (completeAnswers when the roll asks nothing, completedBy when
// the log joins the roll that settled a question) read the same numbers off the same final event, and a cast with no single hit
// completes nothing in both.
func TestBothCallersCompleteACutLineWithTheSameNumbers(t *testing.T) {
	t.Parallel()
	hit := func(outcome string, ac, total int32) actionEvent {
		return actionEvent{Hits: []castHit{{Outcome: outcome, TargetAC: ac, Total: total}}}
	}
	for name, tc := range map[string]struct {
		kind  string
		final actionEvent
		want  cutNumbers
		ok    bool
	}{
		"a weapon attack":             {"attack", actionEvent{Outcome: outcomeMiss, TargetAC: 12, Total: 7}, cutNumbers{outcomeMiss, 12, 12, 7}, true},
		"a spell attack with one hit": {"cast", hit(outcomeMiss, 14, 9), cutNumbers{outcomeMiss, 14, 14, 9}, true},
		"a cast with no hit":          {"cast", actionEvent{}, cutNumbers{}, false},
		"a cast with two hits":        {"cast", actionEvent{Hits: []castHit{{Total: 3}, {Total: 4}}}, cutNumbers{}, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			const reduction = 5
			start := reactionEvent{Kind: string(reaction.CuttingWords), Roll: tc.kind, Asked: true, Reduction: reduction}
			joined, joinedOK := start.completedBy(tc.kind, tc.final)
			if joinedOK != tc.ok {
				t.Fatalf("completedBy() ok = %v, want %v", joinedOK, tc.ok)
			}

			answer := start
			answer.Asked = false
			c := &combatTx{answered: []answeredRef{{window: "w", ev: &answer}}}
			st := &replayState{hold: playdb.ReactionHold{Kind: tc.kind}, reduction: reduction}
			(&Service{}).completeAnswers(c, []playdb.ReactionWindow{{ID: "w"}}, st, tc.final)

			if !tc.ok {
				if !reflect.DeepEqual(joined, start) || answer.Outcome != "" || answer.After != 0 || answer.Before != 0 {
					t.Errorf("a roll with no numbers changed the line: completedBy %+v, completeAnswers %+v", joined, answer)
				}
				return
			}
			numbers := func(r reactionEvent) cutNumbers { return cutNumbers{r.Outcome, r.AC, r.Before, r.After} }
			if got := numbers(joined); got != tc.want {
				t.Errorf("completedBy() numbers = %+v, want %+v", got, tc.want)
			}
			if got := numbers(answer); got != tc.want {
				t.Errorf("completeAnswers() numbers = %+v, want %+v", got, tc.want)
			}
			if joined.Asked {
				t.Error("completedBy() still says Asked")
			}
		})
	}
}
