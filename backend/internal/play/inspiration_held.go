package play

import (
	"context"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The held roll of the d20 rolls that are not a weapon attack (SRD 5.1, Bard 1: the creature
// "can wait until after it rolls the d20 before deciding to use the Bardic Inspiration die, but
// must decide before the GM says whether the roll succeeds"). They use the mechanism of the scene
// check (outsideRoll, inspiration_holds, AnswerOutsideInspiration), whose die is the character's
// out of a combat and its combatant's in one (dieOf): the roll path rolls the d20 and keeps it
// with the request before it writes anything, the player is asked, and the answer runs the same
// request again with the d20 already rolled. The paths that hold are the ones of a player's own
// character: the master's rolls, and the saves the app rolls for a target in a caster's call
// (a spell's saving throw, a hit rider's), do not wait for a die.

// spellAttackPre is the d20 of a spell attack settled before the cast spent anything, when the
// answer to a Bardic Inspiration question brings it back: spellAttack takes the mode and the roll
// from here instead of rolling.
type spellAttackPre struct {
	target string
	truth  modeResult
	choice modeChoice
	step   outsideStep
	// die is the combatant's die as it was before the answer used it, for the undo of the cast.
	die heldDie
}

// holdSpellAttack is asked by a cast with one spell attack before anything is spent. A caster that
// is a player's character and holds a die gets the d20 rolled and kept: it answers the question
// to put to the player, and the cast writes nothing. The answer's run of the cast gets the settled
// roll in modes.pre and answers nil; so does every cast that holds no die.
func (s *Service) holdSpellAttack(
	ctx context.Context, c *combatTx, m authz.Membership, req *playv1.CastSpellRequest, key string,
	caster, target playdb.Combatant, sp link.Spell, in rollInput, modes *spellModes,
) (*playv1.OutsideInspirationOffer, error) {
	if m.Role == authz.RoleMaster || caster.Kind != kindPlayer || !backedByCharacter(caster) {
		return nil, nil //nolint:nilnil // no question: the master and the NPCs roll without a die
	}
	var before heldDie
	if outsideOf(ctx) != nil {
		before, _, _ = s.dieOf(ctx, c.q, m.CampaignID, caster.CharacterID) //nolint:errcheck // read again by outsideRoll, which reports it
	} else if _, found, err := s.dieOf(ctx, c.q, m.CampaignID, caster.CharacterID); err != nil || !found {
		return nil, err
	}
	// The mode is worked out as spellAttack does, without writing while the roll only waits and as the
	// roll was made with when it is settled.
	forced := outsideOf(ctx) != nil
	ask := modes.ask
	ask.dry, ask.trusted = !forced, forced
	truth := modes.in.facts(c.enc, c.sight).attackMode(caster, target, modes.traits, shapeOfSpell(sp), actsNow(c.enc, caster), modes.v, modes.names)
	choice, err := s.chooseMode(ctx, c, modes.v, caster, target, sp.Key, truth.Mode, ask)
	if err != nil {
		return nil, err
	}
	step, err := s.outsideRoll(ctx, c.tx, c.q, m, holdSpellAttack, caster.CharacterID, key, req, in, sp.ToHit, choice.Mode, false)
	switch {
	case err != nil:
		return nil, err
	case step.Held:
		return step.Offer, nil
	case step.Settled:
		modes.pre = &spellAttackPre{target: target.ID, truth: truth, choice: choice, step: step, die: before}
	}
	return nil, nil //nolint:nilnil // settled, or no die after all: the cast goes on
}

// dieUsed is the Bardic Inspiration die the cast's spell attack used, for the undo; nil when none.
func (m *spellModes) dieUsed() *resourceEvent {
	if m == nil || m.pre == nil || m.pre.step.Bonus == nil || m.pre.die.Combatant == nil {
		return nil
	}
	b, who := m.pre.step.Bonus, m.pre.die.Combatant
	return &resourceEvent{Kind: resBardicUse, Sides: b.Sides, Face: b.Face, FromID: deref(who.InspirationFrom), ExpiresRound: derefInt32(who.InspirationExpiresRound)}
}

// saveWithDie is asked by the roll of a saving throw a player makes for their own combatant (an
// effect's, a concentration's) once it knows the bonus and the mode, before anything is written.
// A step that is Held says the d20 was rolled and kept, and the path answers the question and
// writes nothing; Settled brings the d20 back with the die added; the zero step is a roll as
// always (the master's, an NPC's, a character that holds no die).
func (s *Service) saveWithDie(
	ctx context.Context, c *combatTx, m authz.Membership, kind string, reactor playdb.Combatant, key string,
	raw proto.Message, in rollInput, bonus int, mode combat.RollMode,
) (outsideStep, error) {
	if m.Role == authz.RoleMaster || reactor.Kind != kindPlayer || !backedByCharacter(reactor) || reactor.UserID == nil || *reactor.UserID != m.UserID {
		return outsideStep{}, nil
	}
	return s.outsideRoll(ctx, c.tx, c.q, m, kind, reactor.CharacterID, key, raw, in, bonus, mode, false)
}
