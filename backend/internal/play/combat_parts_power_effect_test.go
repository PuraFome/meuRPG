package play

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// A power attack and a spell's hit effect in the same attack: another player reads neither the trade's number nor the
// effect's dice, and both parts and both rolls are masked in the one view (RN-10).
func TestAPowerAttackAndAHitEffectInOneAttackAreBothMaskedForOtherPlayers(t *testing.T) {
	t.Parallel()
	power := powerFacts(powerTrade(true)).power.powerPart("damage-type:slashing")
	effect := partRecord{
		Key: tableMark, Label: "Marca de Nanquim", Kind: partAuto, Count: 7, Sides: 12, Source: tableMark, Available: true,
		Table: tableHitEffect, Effect: "row-1", Hide: true, Note: "O efeito acaba com este golpe.",
	}
	weapon := partRecord{Key: partWeapon, Label: "Espada", Kind: partWeapon, Count: 2, Sides: 6, Flat: 3, Available: true}
	pd := playdb.PendingDamage{
		Parts: mustJSON([]partRecord{weapon, power, effect}),
		PartRolls: mustJSON([]partRoll{
			{Key: partWeapon, Label: "Espada", Count: 2, Sides: 6, Flat: 3, Sum: 8, Counted: true},
			{Key: partPowerAttack, Label: powerName, Flat: powerBonus, Sum: powerBonus, Counted: true, Hide: true},
			{Key: tableMark, Label: "Marca de Nanquim", Count: 7, Sides: 12, Faces: []int32{5, 6}, Sum: 11, Counted: true, Effect: "row-1", Hide: true},
		}),
	}

	owner := partsProtoFor(pd, combat.CriticalDoubledDice, dtPT, true)
	if len(owner) != 3 || owner[1].GetFlat() != powerBonus || owner[1].GetLabelPt() != powerName || owner[2].GetDiceCount() != 7 || owner[2].GetLabelPt() != "Marca de Nanquim" {
		t.Fatalf("the owner reads %v, want the trade's number and the effect's dice whole", owner)
	}
	others := partsProtoFor(pd, combat.CriticalDoubledDice, dtPT, false)
	if len(others) != 3 {
		t.Fatalf("the others read %d parts, want 3", len(others))
	}
	if p := others[1]; p.GetFlat() != 0 || p.GetSourceKey() != "" || p.GetLabelPt() != otherSourceTx || p.GetReasonPt() != "" {
		t.Errorf("the others read the power part %v, want no number, no source, %q", p, otherSourceTx)
	}
	if e := others[2]; e.GetDiceCount() != 0 || e.GetDiceSides() != 0 || e.GetSourceKey() != "" || e.GetLabelPt() != otherSourceTx || e.GetNotePt() != "" {
		t.Errorf("the others read the effect part %v, want no dice, no source, no note, %q", e, otherSourceTx)
	}

	rolls := partRollsProto(pd, false, false, dtPT)
	if len(rolls) != 3 {
		t.Fatalf("the others read %d rolls, want 3", len(rolls))
	}
	if r := rolls[1]; r.GetFlat() != 0 || r.GetSum() != 0 || r.GetLabelPt() != otherSourceTx {
		t.Errorf("the others read the power roll %v, want no number", r)
	}
	if r := rolls[2]; r.GetDiceCount() != 0 || len(r.GetFaces()) != 0 || r.GetHitEffect() || r.GetLabelPt() != otherSourceTx {
		t.Errorf("the others read the effect roll %v, want no dice, no faces and no hit-effect flag", r)
	}
	if rolls := partRollsProto(pd, true, true, dtPT); rolls[1].GetFlat() != powerBonus || rolls[2].GetDiceCount() != 7 || !rolls[2].GetHitEffect() {
		t.Errorf("the owner reads %v, want both rolls whole", rolls)
	}
}
