package play

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// A spell attack cantrip readied (Fire Bolt, Ray of Frost): a spell, so its energy is held with concentration as any
// readied spell's (SRD 5.1, "Ready"), and an attack, so its release is RollAttack as a reaction, as a readied attack's.
// Its row is a spell's with no cast id (heldSpellAttackRow): it spends no slot, and nothing is cast at Ready.

// heldCast is the cast a SPELL holds, before its checks.
func (f *readyFlow) heldCast() *castFlow {
	t := f.take
	req := &playv1.CastSpellRequest{CampaignId: t.m.CampaignID, EncounterId: t.c.enc.ID, CasterId: t.who.ID, SpellKey: f.call.spellKey}
	return newCastFlow(t.s, t.c, t.m, req, castCall{encounterID: t.c.enc.ID, casterID: t.who.ID, spellKey: f.call.spellKey}, &castOut{})
}

// spellAttackOf is the caster's spell attack cantrip of the key, if it is one: an attack roll, not a save.
func spellAttackOf(opts *rulesv1.TurnOptions, key string) *rulesv1.AttackOption {
	for _, a := range opts.GetAttacks() {
		if a.GetAttack().GetKey() == key && a.GetAttack().GetKind() == rulesv1.AttackKind_ATTACK_KIND_SPELL && a.GetAttack().GetSaveDc() == 0 {
			return a
		}
	}
	return nil
}

// checkSpellAttack checks a spell attack cantrip at Ready: no slot, an attack the caster may make. Its reach, its target
// and its roll are RollAttack's at the release.
func (f *readyFlow) checkSpellAttack(ctx context.Context, cf *castFlow, a *rulesv1.AttackOption) error {
	if f.call.slotLevel != 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("a cantrip is cast with no slot"))
	}
	if !a.GetEnabled() {
		return errReadyAction("the spell attack cannot be made now")
	}
	if err := cf.findCaster(ctx); err != nil {
		return err
	}
	cf.sp.Key, cf.sp.Economy = f.call.spellKey, rules.EconomyAction
	f.spell, f.spellAttack, f.name = cf, true, a.GetAttack().GetNamePt()
	return nil
}

// heldSpellAttackRow says the row holds a spell attack cantrip.
func heldSpellAttackRow(row playdb.ReadiedAction) bool {
	return row.Kind == readiedSpell && row.SpellCastID == nil
}

// usedAs is the path the row's use takes: a spell attack cantrip's is an attack's.
func usedAs(row playdb.ReadiedAction) string {
	if heldSpellAttackRow(row) {
		return readiedAttack
	}
	return row.Kind
}
