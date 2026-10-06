package characters

import (
	"testing"

	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

func TestAttackFromLegacy(t *testing.T) {
	const (
		slashing  = charactersv1.DamageType_DAMAGE_TYPE_SLASHING
		piercing  = charactersv1.DamageType_DAMAGE_TYPE_PIERCING
		bludgeon  = charactersv1.DamageType_DAMAGE_TYPE_BLUDGEONING
		lightning = charactersv1.DamageType_DAMAGE_TYPE_LIGHTNING
		fire      = charactersv1.DamageType_DAMAGE_TYPE_FIRE
	)
	tests := []struct {
		damage              string
		count, sides, bonus int32
		dtype               charactersv1.DamageType
		ok                  bool
	}{
		{"1d6+2 cortante", 1, 6, 2, slashing, true},
		{"1d6 + 2 cortante", 1, 6, 2, slashing, true},
		{"2d8 perfurante", 2, 8, 0, piercing, true},
		{"1D4-1 Perfurante", 1, 4, -1, piercing, true},
		{"1d4 − 1 perfurante", 1, 4, -1, piercing, true}, // a real minus sign
		{"1d10+3 de concussão", 1, 10, 3, bludgeon, true},
		{"1d8+1 contundente", 1, 8, 1, bludgeon, true},
		{"1d6+2 de dano cortante", 0, 0, 0, 0, false}, // "dano" before the type is not a known shape
		{"1d6+2 slashing", 1, 6, 2, slashing, true},
		{"1d8 piercing damage", 1, 8, 0, piercing, true},
		{"2d6+4 elétrico", 2, 6, 4, lightning, true},
		{"1d6 fogo dano", 1, 6, 0, fire, true},
		{"  1d6+2   cortante  ", 1, 6, 2, slashing, true},
		{"", 0, 0, 0, 0, false},
		{"1d6+2", 0, 0, 0, 0, false},                  // no type
		{"1d6+2 mordida", 0, 0, 0, 0, false},          // not a damage type
		{"mordida 1d6 perfurante", 0, 0, 0, 0, false}, // starts with words
		{"1d20 cortante", 0, 0, 0, 0, false},          // d20 is not an attack die
		{"25d6 fogo", 0, 0, 0, 0, false},              // too many dice
		{"1d6+99 cortante", 0, 0, 0, 0, false},        // bonus out of range
		{"1d6+2 cortante e 1d4 fogo", 0, 0, 0, 0, false},
		{"1d6+99999999999999999999 fogo", 0, 0, 0, 0, false},
	}
	for _, tt := range tests {
		a, ok := attackFromLegacy(4, tt.damage)
		if ok != tt.ok {
			t.Errorf("attackFromLegacy(%q) ok = %v, want %v", tt.damage, ok, tt.ok)
			continue
		}
		if !ok {
			continue
		}
		if a.Name != "Ataque" || a.AttackBonus != 4 || a.DamageDiceCount != tt.count ||
			a.DamageDiceSides != tt.sides || a.DamageBonus != tt.bonus || a.DamageType != tt.dtype {
			t.Errorf("attackFromLegacy(%q) = %v", tt.damage, a)
		}
	}
}

func TestAttackFromLegacyRejectsAnOutOfRangeBonus(t *testing.T) {
	if _, ok := attackFromLegacy(25, "1d6 cortante"); ok {
		t.Error("an old attack bonus of +25 must not become an attack (the limit is +20)")
	}
}

// TestLoadSheetUpgradesALegacyAttack reads sheets as stored before Etapa 6.
func TestLoadSheetUpgradesALegacyAttack(t *testing.T) {
	t.Run("readable damage becomes an attack", func(t *testing.T) {
		sheet, err := loadSheet("c1", []byte(`{"basic":{"hit_points_max":7,"armor_class":15,"speed_ft":30,"attack_bonus":4,"damage":"1d6+2 cortante"}}`))
		if err != nil {
			t.Fatal(err)
		}
		want := &charactersv1.BasicSheet{HitPointsMax: 7, ArmorClass: 15, SpeedFt: 30, Attacks: []*charactersv1.BasicAttack{{
			Name: "Ataque", AttackBonus: 4, DamageDiceCount: 1, DamageDiceSides: 6, DamageBonus: 2,
			DamageType: charactersv1.DamageType_DAMAGE_TYPE_SLASHING,
		}}}
		if !proto.Equal(sheet.GetBasic(), want) {
			t.Errorf("basic = %v, want %v", sheet.GetBasic(), want)
		}
	})
	t.Run("unreadable damage stays as text", func(t *testing.T) {
		sheet, err := loadSheet("c1", []byte(`{"basic":{"hit_points_max":7,"armor_class":15,"attack_bonus":4,"damage":"mordida venenosa"}}`))
		if err != nil {
			t.Fatal(err)
		}
		b := sheet.GetBasic()
		if len(b.Attacks) != 0 || b.Damage != "mordida venenosa" || b.AttackBonus != 4 {
			t.Errorf("basic = %v, want the old fields untouched and no attacks", b)
		}
	})
	t.Run("a sheet with attacks is left alone", func(t *testing.T) {
		sheet, err := loadSheet("c1", []byte(`{"basic":{"hit_points_max":7,"armor_class":15,"damage":"1d4 fogo","attacks":[{"name":"Faca","damage_dice_count":1,"damage_dice_sides":4,"damage_type":"DAMAGE_TYPE_PIERCING"}]}}`))
		if err != nil {
			t.Fatal(err)
		}
		if b := sheet.GetBasic(); len(b.Attacks) != 1 || b.Attacks[0].Name != "Faca" {
			t.Errorf("basic = %v", b)
		}
	})
}

// TestCheckSheetClearsTheOldAttack: a write with attacks drops the old
// fields; one without keeps the old text, so saving never loses it.
func TestCheckSheetClearsTheOldAttack(t *testing.T) {
	with := basicSheet()
	with.GetBasic().Damage, with.GetBasic().AttackBonus = "mordida", 3
	got, err := checkSheet(loadRules(t), with)
	if err != nil {
		t.Fatal(err)
	}
	if b := got.GetBasic(); b.Damage != "" || b.AttackBonus != 0 || len(b.Attacks) != 1 {
		t.Errorf("with attacks: basic = %v", b)
	}
	without := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Basic{Basic: &charactersv1.BasicSheet{
		HitPointsMax: 5, ArmorClass: 10, Damage: "mordida", AttackBonus: 3,
	}}}
	got, err = checkSheet(loadRules(t), without)
	if err != nil {
		t.Fatal(err)
	}
	if b := got.GetBasic(); b.Damage != "mordida" || b.AttackBonus != 3 {
		t.Errorf("without attacks: basic = %v", b)
	}
}
