package play

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The Wild Shape lines of the combat log (MR-037, RN-20): the druid's change of form is a line
// for everyone who sees her; the damage the beast's fall carried over is her player's and the
// master's alone. These tests need the database (MEURPG_TEST_DATABASE_URL).

// shapeLines are the Wild Shape entries of a log, oldest first.
func shapeLines(log *playv1.ListCombatLogResponse) []*playv1.CombatLogEntry {
	var out []*playv1.CombatLogEntry
	for _, r := range log.GetRounds() {
		for _, e := range r.GetEntries() {
			if e.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_WILD_SHAPE {
				out = append([]*playv1.CombatLogEntry{e}, out...)
			}
		}
	}
	return out
}

func lastShapeLine(t *testing.T, lines []*playv1.CombatLogEntry) *playv1.CombatLogWildShape {
	t.Helper()
	if len(lines) == 0 {
		t.Fatal("the log has no Wild Shape line")
	}
	return lines[len(lines)-1].GetWildShape()
}

func TestMR037_WildShapeStartedAndLeftAreLogLines(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	a := s.armed
	s.fight(t)
	e := s.mustAssume(t, s.bia, s.bri, wolfKey).GetEncounter()
	got := lastShapeLine(t, shapeLines(a.log(t, a.master, e)))
	if !got.GetStarted() || got.GetBeastKey() != wolfKey || got.GetBeastNamePt() != "Lobo" {
		t.Errorf("started line = %v, want the wolf, started", got)
	}
	// Every player who sees her reads it (the party sees the wolf).
	if !got.GetStarted() || lastShapeLine(t, shapeLines(a.log(t, s.dani, e))).GetBeastNamePt() != "Lobo" {
		t.Errorf("another player's log lacks the started line")
	}
	s.mustLeave(t, s.bia, s.bri)
	e = a.get(t, a.master)
	left := lastShapeLine(t, shapeLines(a.log(t, a.master, e)))
	if left.GetStarted() || left.GetEndReason() != playv1.WildShapeEndReason_WILD_SHAPE_END_REASON_LEFT || left.GetCarriedDamage() != 0 {
		t.Errorf("left line = %v, want ended, reason left, nothing carried", left)
	}
}

func TestMR037_WildShapeDamageLineCarriesTheNumberOnlyToTheOwnerAndTheMaster(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	a := s.armed
	s.fight(t)
	e := s.mustAssume(t, s.bia, s.bri, wolfKey).GetEncounter()
	// The Capitão's critical hit: the wolf (11) falls and the rest passes to Sálvia.
	crit := a.mustAttack(t, a.master, e, "Capitão Goblin", sword, "Sálvia", func(r *playv1.RollAttackRequest) {
		r.Roll = &playv1.RollAttackRequest_D20Face{D20Face: 20}
		r.AsReaction = true
	})
	dmg := a.mustDamage(t, a.master, e, crit.GetPendingDamage().GetId(), typedDamage(12)).GetPendingDamage()
	if _, err := a.settle(t, a.master, e, dmg.GetId(), true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	carried := dmg.GetAmount() - 11
	e = a.get(t, a.master)
	for who, u := range map[string]*user{"the master": a.master, "the druid's player": s.bia} {
		got := lastShapeLine(t, shapeLines(a.log(t, u, e)))
		if got.GetStarted() || got.GetEndReason() != playv1.WildShapeEndReason_WILD_SHAPE_END_REASON_DAMAGE || got.GetCarriedDamage() != carried || carried <= 0 {
			t.Errorf("%s reads %v, want the beast's fall with %d carried", who, got, carried)
		}
	}
	// Another player reads that she is back, with no number (RN-20), as the app's JSON.
	other := a.log(t, s.dani, e)
	got := lastShapeLine(t, shapeLines(other))
	if got.GetEndReason() != playv1.WildShapeEndReason_WILD_SHAPE_END_REASON_DAMAGE || got.GetCarriedDamage() != 0 {
		t.Errorf("another player reads %v, want the reason and no carried damage", got)
	}
	raw, err := protojson.Marshal(other)
	if err != nil {
		t.Fatalf("encode the log: %v", err)
	}
	if json := string(raw); strings.Contains(json, "carriedDamage") {
		t.Errorf("another player's log JSON carries the damage: %s", json)
	}
}

func TestMR037_WildShapeLineByEachEndReason(t *testing.T) {
	t.Parallel()
	reasons := map[string]playv1.WildShapeEndReason{
		"master":      playv1.WildShapeEndReason_WILD_SHAPE_END_REASON_MASTER,
		"zero_hp":     playv1.WildShapeEndReason_WILD_SHAPE_END_REASON_ZERO_HP,
		"unconscious": playv1.WildShapeEndReason_WILD_SHAPE_END_REASON_UNCONSCIOUS,
	}
	for name, want := range reasons {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := newShapers(t)
			a := s.armed
			s.fight(t)
			e := s.mustAssume(t, s.bia, s.bri, wolfKey).GetEncounter()
			switch name {
			case "master":
				s.correct(t, s.bri, func(r *playv1.AdjustCharacterVitalsRequest) { r.WildShapeHitPointsCurrent = new(int32(0)) })
			case "zero_hp":
				s.correct(t, s.bri, func(r *playv1.AdjustCharacterVitalsRequest) { r.HitPointsCurrent = new(int32(0)) })
			case "unconscious":
				a.setConditions(t, e, "Sálvia", unconscious)
			}
			got := lastShapeLine(t, shapeLines(a.log(t, a.master, a.get(t, a.master))))
			if got.GetStarted() || got.GetEndReason() != want || got.GetBeastKey() != wolfKey {
				t.Errorf("line = %v, want ended, reason %v", got, want)
			}
			// Another player reads the same line, with no number.
			if other := lastShapeLine(t, shapeLines(a.log(t, s.dani, a.get(t, s.dani)))); other.GetEndReason() != want || other.GetCarriedDamage() != 0 {
				t.Errorf("another player reads %v, want the reason and nothing carried", other)
			}
		})
	}
}
