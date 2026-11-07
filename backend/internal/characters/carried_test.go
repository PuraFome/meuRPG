package characters

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

const (
	chainMail = "equipment:chain-mail"
	shieldKey = "equipment:shield"
)

func loadSRD(t *testing.T) *rules.Content {
	t.Helper()
	c, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD: %v", err)
	}
	return c
}

func wizardBuild(armor string, shield bool, carried ...string) rules.Build {
	return rules.Build{
		BaseScores: map[rules.Ability]int{rules.STR: 15, rules.DEX: 14, rules.CON: 13, rules.INT: 12, rules.WIS: 10, rules.CHA: 8},
		Race:       "race:human", Background: "background:acolyte",
		Classes: []rules.ClassLevel{{Class: "class:wizard", Level: 1}},
		Armor:   armor, Shield: shield, Carried: carried,
	}
}

// Carried armor is not worn: Mage Armor applies and gives 13 + DEX.
func TestMageArmorOfReadsEquippedArmor(t *testing.T) {
	t.Parallel()
	c := loadSRD(t)
	if got := mageArmorOf(wizardBuild(chainMail, false), c); !got.Wears {
		t.Errorf("equipped armor: %+v, want Wears", got)
	}
	got := mageArmorOf(wizardBuild(chainMail, false, chainMail), c)
	if got.Wears || got.AC != 15 {
		t.Errorf("carried armor: %+v, want AC 15 and no Wears", got)
	}
}

// A carried shield is not in the owner's formula environment.
func TestOwnerEnvOfReadsEquippedShield(t *testing.T) {
	t.Parallel()
	c := loadSRD(t)
	if !ownerEnvOf(wizardBuild("", true), c).Shield() {
		t.Errorf("equipped shield: Shield() = false")
	}
	if ownerEnvOf(wizardBuild("", true, shieldKey), c).Shield() {
		t.Errorf("carried shield: Shield() = true")
	}
}

// The Protection style needs an equipped shield.
func TestReactionProtectionNeedsEquippedShield(t *testing.T) {
	t.Parallel()
	d := rules.Derived{Features: []rules.Feature{{Key: "feature:fighting-style-protection"}}}
	for _, tt := range []struct {
		name    string
		carried []string
		want    bool
	}{{"equipped", nil, true}, {"carried", []string{shieldKey}, false}} {
		full := &charactersv1.FullSheet{Shield: true, CarriedKeys: tt.carried}
		if got := fullReactionStats(d, full).Protection; got != tt.want {
			t.Errorf("%s: Protection = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// A save that only moves a weapon between carried and equipped is no gear change for
// Mage Armor; armor and shield are read as equipped.
func TestGearChangeOfReadsEquippedGear(t *testing.T) {
	t.Parallel()
	mk := func(armor string, shield bool, weapons []string, carried ...string) *charactersv1.FullSheet {
		return &charactersv1.FullSheet{ArmorKey: armor, Shield: shield, WeaponKeys: weapons, CarriedKeys: carried}
	}
	sword := []string{"equipment:longsword"}
	tests := []struct {
		name         string
		stored, next *charactersv1.FullSheet
		want         gearChange
	}{
		{"same", mk(chainMail, true, sword), mk(chainMail, true, sword), gearUnchanged},
		{"weapon carried", mk(chainMail, true, sword), mk(chainMail, true, sword, sword[0]), gearUnchanged},
		{"armor donned", mk("", false, nil), mk(chainMail, false, nil), gearArmorWorn},
		{"armor equipped from carried", mk(chainMail, false, nil, chainMail), mk(chainMail, false, nil), gearArmorWorn},
		{"armor put in carried", mk(chainMail, false, nil), mk(chainMail, false, nil, chainMail), gearChanged},
		{"shield put in carried", mk("", true, nil), mk("", true, nil, shieldKey), gearChanged},
		{"shield taken", mk("", true, nil), mk("", false, nil), gearChanged},
	}
	for _, tt := range tests {
		if got := gearChangeOf(tt.stored, tt.next); got != tt.want {
			t.Errorf("%s: %d, want %d", tt.name, got, tt.want)
		}
	}
}
