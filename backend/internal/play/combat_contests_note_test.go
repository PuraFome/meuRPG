package play

import (
	"encoding/json"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The notes of a roll follow the hiding rule (RN-10): a hidden note, and a note stored with no judgement of its source, reads "Outra fonte" to a reader but the master, keeps its
// advantage or disadvantage and, for a bonus, its value; the master reads it in full. The judgement is stored with the note, so a roll
// read after it was written follows the rule too.
func TestRollNotesFollowTheHidingRule(t *testing.T) {
	t.Parallel()
	poisoned := rollNote{Kind: notePoisoned, Label: labelPoisoned}
	effect := rollNote{Kind: "effect", Label: "Efeito ativo", Adv: true}
	bonus := rollNote{Kind: noteEffectBonus, Label: "Marca", Value: 3}
	hide := func(n rollNote) rollNote { return n.judged(true) }
	show := func(n rollNote) rollNote { return n.judged(false) }
	cases := []struct {
		name   string
		note   rollNote
		master *playv1.RollNote
		reader *playv1.RollNote
	}{
		{"a shown circumstance", show(poisoned), &playv1.RollNote{Kind: notePoisoned, LabelPt: labelPoisoned}, &playv1.RollNote{Kind: notePoisoned, LabelPt: labelPoisoned}},
		{"a hidden disadvantage", hide(poisoned), &playv1.RollNote{Kind: notePoisoned, LabelPt: labelPoisoned}, &playv1.RollNote{Kind: "other", LabelPt: otherSourceTx}},
		{"a hidden advantage", hide(effect), &playv1.RollNote{Kind: "effect", LabelPt: "Efeito ativo", Advantage: true}, &playv1.RollNote{Kind: "other", LabelPt: otherSourceTx, Advantage: true}},
		{"a shown bonus", show(bonus), &playv1.RollNote{Kind: noteEffectBonus, LabelPt: "Marca +3"}, &playv1.RollNote{Kind: noteEffectBonus, LabelPt: "Marca +3"}},
		// A note stored with no judgement reads as hidden to the reader, with its advantage and its bonus's value, and in full to the master.
		{"a circumstance with no judgement stored", poisoned, &playv1.RollNote{Kind: notePoisoned, LabelPt: labelPoisoned}, &playv1.RollNote{Kind: "other", LabelPt: otherSourceTx}},
		{"an advantage with no judgement stored", effect, &playv1.RollNote{Kind: "effect", LabelPt: "Efeito ativo", Advantage: true}, &playv1.RollNote{Kind: "other", LabelPt: otherSourceTx, Advantage: true}},
		{"a bonus with no judgement stored", bonus, &playv1.RollNote{Kind: noteEffectBonus, LabelPt: "Marca +3"}, &playv1.RollNote{Kind: noteEffectBonus, LabelPt: otherSourceTx + " +3"}},
		{"the roller's own Inspiração with no judgement stored", rollNote{Kind: noteFavor, Label: labelFavor, Adv: true}, &playv1.RollNote{Kind: noteFavor, LabelPt: labelFavor, Advantage: true}, &playv1.RollNote{Kind: noteFavor, LabelPt: labelFavor, Advantage: true}},
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
