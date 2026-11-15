package play

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/play/steed"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// A spell that targets only the caster also targets the bonded steed the caster rides (SRD 5.1, "Find Steed"). A spell with a lasting effect
// puts it on the steed's combatant as on the caster's; any other spell leaves the steed to the table, and the cast says so in one line.

// bondedSteedOf is the combatant of the bonded steed the player's character rides, or nil.
func (s *Service) bondedSteedOf(ctx context.Context, tx pgx.Tx, campaignID string, cs []playdb.Combatant, who playdb.Combatant) (*playdb.Combatant, error) {
	if who.Kind != kindPlayer || who.MountedOnID == nil {
		return nil, nil
	}
	mount := ridingOn(cs, who)
	if mount == nil || mount.CreatureID == nil {
		return nil, nil
	}
	owned, err := s.roster.CharacterCreatures(ctx, tx, campaignID, []string{who.CharacterID})
	if err != nil {
		return nil, err
	}
	for _, cr := range owned {
		if cr.ID == *mount.CreatureID && cr.Bonded {
			return mount, nil
		}
	}
	return nil, nil
}

// attachAlsoSteed marks the spells of the turn options that may also reach the steed: the ones that reach only the caster, while it
// rides its bonded steed.
func (s *Service) attachAlsoSteed(ctx context.Context, campaignID string, d *encounterData, who playdb.Combatant, opts *rulesv1.TurnOptions) error {
	if opts == nil || len(opts.GetSpells()) == 0 {
		return nil
	}
	mount, err := s.bondedSteedOf(ctx, nil, campaignID, d.cs, who)
	if err != nil || mount == nil {
		return err
	}
	content, err := s.roster.RulesContent(ctx, nil, campaignID)
	if err != nil {
		return err
	}
	for _, o := range opts.GetSpells() {
		o.AlsoSteed = steed.SelfTargeted(content, o.GetSpell().GetKey())
	}
	return nil
}

// checkAlsoSteed checks the request's also_steed: the spell reaches only the caster and the caster rides its bonded steed.
func (f *castFlow) checkAlsoSteed(ctx context.Context) error {
	if !f.req.GetAlsoSteed() {
		return nil
	}
	content, err := f.s.contentOf(ctx, f.c)
	if err != nil {
		return err
	}
	if !steed.SelfTargeted(content, f.call.spellKey) {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("also_steed is for a spell that reaches only the caster"))
	}
	mount, err := f.s.bondedSteedOf(ctx, f.c.tx, f.m.CampaignID, f.cs, f.caster)
	if err != nil {
		return err
	}
	if mount == nil {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("the caster does not ride its bonded steed"))
	}
	f.steed = mount
	f.steedTarget = false
	if def, ok := content.CombatSpellEffect(f.call.spellKey); ok && !hitsOnly(def) {
		f.steedTarget = true
	}
	f.steedNote = steed.NoteFor(f.steedTarget)
	return nil
}

// hitsOnly says the effect is the damage a weapon hit adds, which is the caster's alone.
func hitsOnly(def *rules.EffectDef) bool {
	_, hits := hitModifierOf(def.Modifiers)
	return hits
}

// aimAtSteed adds the steed to the targets of a spell whose lasting effect reaches it.
func (f *castFlow) aimAtSteed() {
	if f.steed != nil && f.steedTarget {
		f.targs, f.targets = append(f.targs, *f.steed), append(f.targets, castAim{id: f.steed.ID})
	}
}
