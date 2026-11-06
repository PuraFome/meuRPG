package play

import (
	"encoding/json"
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

const wightHitPoints = 45

// The extra die of the weapon's type (Sneak Attack) takes the weapon's origin, part by part: against a wight
// (nonmagical weapons that aren't silvered) a plain rapier's whole 11 (3 and the Strength modifier 3 for the blade, the extra 5, all piercing) is halved to 5,
// a silvered or magical one's is not, and an adamantine mark does not help against the silver exemption.
func TestAnExtraDieOfTheWeaponsTypeTakesTheWeaponsOrigin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		mark *charactersv1.WeaponMark
		want int32
	}{
		{"plain", nil, wightHitPoints - 5},
		{"silvered", &charactersv1.WeaponMark{WeaponKey: rapier, Silvered: true}, wightHitPoints - 11},
		{"adamantine only", &charactersv1.WeaponMark{WeaponKey: rapier, Adamantine: true}, wightHitPoints - 5},
		{"magical", &charactersv1.WeaponMark{WeaponKey: rapier, Magical: true}, wightHitPoints - 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a, e, wight := partsFight(t, func(a *armed) {
				a.toren = a.caio.markedHeroOf(t, a.campaignID, "Toren", "class:rogue", 3, tt.mark)
			}, "monster:wight")
			a.setConditions(t, e, wight, "condition:restrained") // advantage: Sneak Attack is allowed
			hit := a.mustAttack(t, a.caio, e, "Toren", rapier, wight, func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{5, 18} })
			a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), func(r *playv1.RollDamageRequest) {
				r.ExtrasChosen = true
				r.SelectedExtras = []*playv1.SelectedExtra{{Key: "sneak-attack"}}
				r.TypedParts = []*playv1.TypedPart{{PartKey: "weapon", Sum: 3}, {PartKey: "sneak-attack", Sum: 5}}
			})
			if hp, _, _ := a.hp(t, wight); hp != tt.want {
				t.Errorf("the wight has %d hit points, want %d", hp, tt.want)
			}
		})
	}
}

// Divine Smite's radiant damage is not the weapon's: an unsilvered rapier hit on a wight (it resists nonmagical
// piercing) halves only the weapon's own part, the smite and its extra die against the undead land whole.
func TestDivineSmiteRadiantIsNotReducedByTheNonmagicalResistanceToThePiercing(t *testing.T) {
	t.Parallel()
	a, e, wight := partsFight(t, func(a *armed) {
		a.toren = a.caio.markedHeroOf(t, a.campaignID, "Toren", "class:paladin", 3, nil)
	}, "monster:wight")
	hit := a.mustAttack(t, a.caio, e, "Toren", rapier, wight, d20(15))
	a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), func(r *playv1.RollDamageRequest) {
		r.ExtrasChosen = true
		r.SelectedExtras = []*playv1.SelectedExtra{{Key: "divine-smite", SlotLevel: 1}}
		r.TypedParts = []*playv1.TypedPart{{PartKey: "weapon", Sum: 3}, {PartKey: "divine-smite", Sum: 9}, {PartKey: "divine-smite-extra", Sum: 4}}
	})
	// The weapon's 3 and its modifier (+3 Strength) are 6 piercing, halved to 3; the smite's 9 and 4 are radiant.
	if hp, _, _ := a.hp(t, wight); hp != wightHitPoints-(3+9+4) {
		t.Errorf("the wight has %d hit points, want %d (only the piercing halved)", hp, wightHitPoints-(3+9+4))
	}
}

// The master's "ignore_modifiers" leaves a source out of the stored steps (reapply): the wight's silvered-qualified
// resistance on a plain hit is the whole 9 again; another source's key, or an unmarked silvered hit, change nothing
// to what the groups say. (ApplyPendingDamage settles only a player's character, which a stat block never covers, so
// this is the function the call reads.)
func TestIgnoringTheSilveredQualifiedSourceLandsAPlainHitInFull(t *testing.T) {
	t.Parallel()
	const piercing = "damage-type:piercing"
	mods := []combat.Modifier{{Kind: combat.StepResistance, Source: "monster:wight", DamageTypes: []string{piercing}, Qualifier: combat.QualifierNonmagicalUnlessSilvered}}
	tests := []struct {
		name    string
		from    combat.Origin
		ignored []string
		want    int32
	}{
		{"a plain hit, nothing ignored", combat.Origin{Attack: true}, nil, 4},
		{"a plain hit, the wight ignored", combat.Origin{Attack: true}, []string{"monster:wight"}, 9},
		{"a plain hit, another source ignored", combat.Origin{Attack: true}, []string{"feature:rage"}, 4},
		{"a silvered hit", combat.Origin{Attack: true, Silvered: true}, nil, 9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			land := settleSteps(map[string]int{piercing: 9}, mods, nil, tt.from)
			if got := reapply(9, land.groups, tt.ignored); got != tt.want {
				t.Errorf("reapply = %d, want %d", got, tt.want)
			}
		})
	}
}

// Steps stored before the material qualifiers existed (an origin with no silvered or adamantine key, a modifier with
// the nonmagical qualifier) still read and work out the same: the removal of a part works the damage out again from
// them with originOfGroups, modsOfGroups and settleSteps, and an ignored source works out through reapply.
func TestStepsStoredBeforeTheMaterialQualifiersWorkOutTheSame(t *testing.T) {
	t.Parallel()
	const stored = `[{"damage_type":"damage-type:piercing","before":9,` +
		`"mods":[{"Kind":"resistance","Source":"monster:grick","DamageTypes":["damage-type:piercing"],"Qualifier":"nonmagical"}],` +
		`"origin":{"attack":true,"magical":false}}]`
	groups := readStepGroups([]byte(stored))
	want := []stepGroup{{
		DamageType: "damage-type:piercing", Before: 9, From: combat.Origin{Attack: true},
		Mods: []combat.Modifier{{Kind: combat.StepResistance, Source: "monster:grick", DamageTypes: []string{"damage-type:piercing"}, Qualifier: combat.QualifierNonmagical}},
	}}
	gotJSON, _ := json.Marshal(groups)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("the stored steps read as %s, want %s", gotJSON, wantJSON)
	}
	from := originOfGroups(groups)
	if from != (combat.Origin{Attack: true}) {
		t.Fatalf("origin = %+v, want an unmarked attack", from)
	}
	// The removal of a part: the counted damage (9 piercing, as when it was stored) is halved through the stored modifiers.
	land := settleSteps(map[string]int{"damage-type:piercing": 9}, modsOfGroups(groups), nil, from)
	if land.amount != 4 || !land.changed || len(land.groups) != 1 || land.groups[0].From != from {
		t.Errorf("landing = %+v, want 4, changed, one group with the same origin", land)
	}
	if got := reapply(9, groups, nil); got != 4 {
		t.Errorf("reapply = %d, want 4", got)
	}
	if got := reapply(9, groups, []string{"monster:grick"}); got != 9 {
		t.Errorf("reapply ignoring the source = %d, want 9", got)
	}
}
