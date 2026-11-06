package play

import (
	"context"
	"errors"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The table dice a player adds to a roll of its own character (the superiority_die effect of applies check,
// save, initiative and attack, rules/maneuvers.go). The request names the maneuver (maneuver_key); the die is rolled
// with the roll, as the dice an effect adds are (Bênção), and one use of its resource is spent when the roll is
// written. The dice an effect adds and this one are listed in that order, which is the order of the physical
// faces the player types in extra_die_faces.

// maneuverFits says the maneuver goes on a roll of the kind: a check of the skill and the ability (its targets,
// when it lists any), a save of the ability (its ability, when it names one), the initiative or a weapon attack.
func maneuverFits(m link.Maneuver, applies, skillKey, ability string) bool {
	if m.Applies != applies {
		return false
	}
	switch applies {
	case rules.ManeuverCheck:
		return len(m.Targets) == 0 || slices.ContainsFunc(m.Targets, func(t string) bool {
			return t == "check."+ability || (strings.HasPrefix(skillKey, "skill:") && t == "skill."+strings.TrimPrefix(skillKey, "skill:"))
		})
	case rules.ManeuverSave:
		return m.Ability == "" || m.Ability == ability
	case rules.ManeuverAttack:
		return true // any weapon attack; the attack roll refuses a spell attack and an NPC attacker (checkManeuver)
	}
	return true
}

// maneuverForRoll finds the maneuver the player named and checks it goes on this roll.
func maneuverForRoll(sheet link.Sheet, key, applies, skillKey, ability string) (link.Maneuver, error) {
	i := slices.IndexFunc(sheet.Traits.Maneuvers, func(m link.Maneuver) bool { return m.Key == key })
	if i < 0 || !maneuverFits(sheet.Traits.Maneuvers[i], applies, skillKey, ability) {
		return link.Maneuver{}, connect.NewError(connect.CodeInvalidArgument, errors.New("maneuver_key is not a die of the character that goes on this roll"))
	}
	return sheet.Traits.Maneuvers[i], nil
}

// maneuverOfRoll reads the maneuver a roll names from the character's sheet and checks it has a use left. Nil
// for a roll that names none.
func (s *Service) maneuverOfRoll(ctx context.Context, tx pgx.Tx, campaignID, characterID, key, applies, skillKey, ability string) (*link.Maneuver, error) {
	if key == "" {
		return nil, nil
	}
	sheet, err := s.roster.CombatSheet(ctx, tx, campaignID, characterID)
	if err != nil {
		return nil, err
	}
	m, err := maneuverForRoll(sheet, key, applies, skillKey, ability)
	if err != nil {
		return nil, err
	}
	vit, err := s.vitals.GetVitalsTx(ctx, tx, campaignID, characterID)
	if err != nil {
		return nil, err
	}
	if left, _ := poolLeft(vit, m.Resource); left <= 0 {
		return nil, resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_USES_LEFT, "no uses left")
	}
	return &m, nil
}

// maneuverDice is the die a roll names as the list of dice an effect adds, so it is rolled, typed, shown and
// added with them; empty for a roll that names none.
func (s *Service) maneuverDice(ctx context.Context, tx pgx.Tx, campaignID, characterID, key, applies, skillKey, ability string) ([]effectDie, error) {
	m, err := s.maneuverOfRoll(ctx, tx, campaignID, characterID, key, applies, skillKey, ability)
	if err != nil || m == nil {
		return nil, err
	}
	return []effectDie{{Key: m.Key, Faces: m.Sides, Sign: 1, ManeuverResource: m.Resource, CharacterID: characterID}}, nil
}

// spendManeuverDice spends one use of the resource of each maneuver die the roll used, and answers the resources
// spent for the event (the undo gives them back).
func (s *Service) spendManeuverDice(ctx context.Context, c *combatTx, characterID string, rolled []effectDie) ([]string, error) {
	var spent []string
	for _, d := range rolled {
		if d.ManeuverResource == "" {
			continue
		}
		if _, err := s.spendResource(ctx, c, characterID, d.ManeuverResource, 1); err != nil {
			return nil, err
		}
		spent = append(spent, d.ManeuverResource)
	}
	return spent, nil
}

// initiativeDie is the table die a player added to its initiative: its key and the face it showed.
type initiativeDie struct {
	key  string
	face int32
}

// initiativeManeuver rolls the table die the initiative names (in the app, or the face typed for a physical die)
// and spends a use of its resource. It runs once the d20 is settled, so a roll that waits for a question rolls
// and spends nothing.
func (s *Service) initiativeManeuver(ctx context.Context, c *combatTx, call initiativeCall, target playdb.Combatant) (initiativeDie, error) {
	key := call.msg.GetManeuverKey()
	if key == "" {
		return initiativeDie{}, nil
	}
	if target.Kind != kindPlayer || !backedByCharacter(target) {
		return initiativeDie{}, connect.NewError(connect.CodeInvalidArgument, errors.New("only a player's character adds a maneuver die"))
	}
	m, err := s.maneuverOfRoll(ctx, c.tx, c.session.CampaignID, target.CharacterID, key, rules.ManeuverInitiative, "", "")
	if err != nil {
		return initiativeDie{}, err
	}
	face, err := s.rollManeuverDie(*m, call.in.inApp, call.msg.GetManeuverFace())
	if err != nil {
		return initiativeDie{}, err
	}
	if _, err := s.spendResource(ctx, c, target.CharacterID, m.Resource, 1); err != nil {
		return initiativeDie{}, err
	}
	return initiativeDie{key: key, face: face}, nil
}
