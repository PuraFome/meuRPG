package play

import (
	"encoding/json"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

func TestAGoadIsOfferedAsADamageDieOnAWeaponHit(t *testing.T) {
	h := hitFacts{
		attacker: playdb.Combatant{Kind: kindPlayer},
		attack:   link.Attack{Weapon: true, DamageType: "damage-type:slashing"},
		sheet: link.Sheet{Traits: link.Traits{Maneuvers: []link.Maneuver{
			{Key: "feature:golpe-provocador@mesa", NamePT: "Golpe Provocador", Resource: "dados", Applies: rules.ManeuverGoad, Sides: 8, SaveDC: 13},
			{Key: "feature:golpe-de-pegada@mesa", Resource: "dados", Applies: rules.ManeuverGrapple, Sides: 8},
		}}},
		usesLeft: map[string]int32{"dados": 2},
	}
	got := h.maneuverParts()
	if len(got) != 1 || got[0].Key != "feature:golpe-provocador@mesa" || got[0].Sides != 8 || !got[0].Available || !got[0].Choosable {
		t.Fatalf("maneuverParts = %+v, want only the goad as a choosable d8", got)
	}
}

func TestAGoadedCreatureSparesItsSourceOnly(t *testing.T) {
	mods, _ := json.Marshal([]rules.EffectModifier{{Kind: rules.ModifierAttackDisadvantage, ExceptSource: true}})
	src, key := "goader", rules.EffectGoaded
	states := map[string][]playdb.CombatantState{"goaded": {{Kind: "effect", Modifiers: mods, SourceID: &src, SourceKey: &key}}}
	gotKey, spares := effectAttackDisadvantage(states, "goaded")
	if gotKey != rules.EffectGoaded || spares != "goader" {
		t.Fatalf("effectAttackDisadvantage = %q, %q; want the goaded key sparing the goader", gotKey, spares)
	}
}
