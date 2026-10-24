package characters

import rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"

// refuseSpellOptions disables every spell and every cantrip with an attack roll in the options of
// a character that wears armor or a shield it is not proficient with: such a character cannot cast
// spells (SRD 5.1, "Armor"). It replaces the reason an option had, because this one outlasts them.
func refuseSpellOptions(o *rulesv1.TurnOptions) {
	reason := func() *rulesv1.DisabledReason {
		return &rulesv1.DisabledReason{Code: rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ARMOR_NOT_PROFICIENT}
	}
	for _, sp := range o.GetSpells() {
		// No slot is offered either: the reactions that cast a spell read the slots of the options.
		sp.Enabled, sp.Reason, sp.Slots = false, reason(), nil
	}
	for _, a := range o.GetAttacks() {
		if a.GetAttack().GetKind() == rulesv1.AttackKind_ATTACK_KIND_SPELL {
			a.Enabled, a.Reason = false, reason()
		}
	}
}
