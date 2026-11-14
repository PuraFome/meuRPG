package play

import (
	"encoding/json"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The notes of a roll follow the hiding rule (RN-10): a hidden note reads "Outra fonte" to a reader but the master, keeps its
// advantage or disadvantage and, for a bonus, its value; the master reads it in full. The flag is stored with the note, so a roll
// read after it was written follows the rule too.
func TestRollNotesFollowTheHidingRule(t *testing.T) {
	t.Parallel()
	poisoned := rollNote{Kind: notePoisoned, Label: labelPoisoned}
	effect := rollNote{Kind: "effect", Label: "Efeito ativo", Adv: true}
	bonus := rollNote{Kind: noteEffectBonus, Label: "Marca", Value: 3}
	hide := func(n rollNote) rollNote { n.Hidden = true; return n }
	cases := []struct {
		name   string
		note   rollNote
		master *playv1.RollNote
		reader *playv1.RollNote
	}{
		{"a shown circumstance", poisoned, &playv1.RollNote{Kind: notePoisoned, LabelPt: labelPoisoned}, &playv1.RollNote{Kind: notePoisoned, LabelPt: labelPoisoned}},
		{"a hidden disadvantage", hide(poisoned), &playv1.RollNote{Kind: notePoisoned, LabelPt: labelPoisoned}, &playv1.RollNote{Kind: "other", LabelPt: otherSourceTx}},
		{"a hidden advantage", hide(effect), &playv1.RollNote{Kind: "effect", LabelPt: "Efeito ativo", Advantage: true}, &playv1.RollNote{Kind: "other", LabelPt: otherSourceTx, Advantage: true}},
		{"a shown bonus", bonus, &playv1.RollNote{Kind: noteEffectBonus, LabelPt: "Marca +3"}, &playv1.RollNote{Kind: noteEffectBonus, LabelPt: "Marca +3"}},
		{"a hidden bonus", hide(bonus), &playv1.RollNote{Kind: noteEffectBonus, LabelPt: "Marca +3"}, &playv1.RollNote{Kind: noteEffectBonus, LabelPt: otherSourceTx + " +3"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stored, err := json.Marshal(c.note)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			var back rollNote
			if err := json.Unmarshal(stored, &back); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			for _, r := range []struct {
				who    string
				master bool
				want   *playv1.RollNote
			}{{"the master", true, c.master}, {"the reader", false, c.reader}} {
				if got := back.proto(r.master); got.GetKind() != r.want.GetKind() || got.GetLabelPt() != r.want.GetLabelPt() || got.GetAdvantage() != r.want.GetAdvantage() {
					t.Errorf("%s reads %v from the stored note %s, want %v", r.who, got, stored, r.want)
				}
			}
		})
	}
}
