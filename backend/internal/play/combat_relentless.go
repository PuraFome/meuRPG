package play

import (
	"slices"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// Relentless Endurance (SRD 5.1, Half-Orc): when the half-orc is reduced to 0 hit
// points but not killed outright, it drops to 1 hit point instead, once until it
// finishes a long rest. The app uses it by itself: a player always wants it, and
// the damage is already settled when the master applies it. The use is the
// character's "relentless_endurance" resource (the long rest gives it back).

// resRelentlessEndurance is the resource of the trait.
const resRelentlessEndurance = "relentless_endurance"

// The log line "Resistência Implacável: fica com 1 PV" is written by the web client
// from CombatLogDamage.relentless_endurance.

// usesSpentOf is how many uses of a resource the character has spent.
func usesSpentOf(now *playv1.CharacterVitals, key string) int32 {
	for _, r := range now.GetResources() {
		if r.GetKey() == key {
			return r.GetUsed()
		}
	}
	return 0
}

// relentlessEndurance says whether the damage that dmg describes (worked out with
// combat.ApplyDamage on the character's vitals now) is the one Relentless
// Endurance turns into 1 hit point: the character has the use left, the damage
// carried it from above 0 to 0, and what was left over was less than its hit point
// maximum (SRD 5.1, "Instant Death": that much or more kills outright). A druid in a
// beast form is not reduced to 0 yet: the beast takes the damage.
func relentlessEndurance(now *playv1.CharacterVitals, dmg combat.DamageResult) bool {
	if !dmg.FellToZero || now.GetWildShape() != nil || dmg.Excess >= int(now.GetHitPointsMax()) {
		return false
	}
	return slices.ContainsFunc(now.GetResources(), func(r *playv1.ResourceUsage) bool {
		return r.GetKey() == resRelentlessEndurance && r.GetUsed() < r.GetTotal()
	})
}
