package play

import (
	"context"
	"fmt"
	"math"
	"time"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

// heldPlayerFirst is how long, after a question is asked, only the roller's player may answer it: the master's
// controls wait for a present player to take the first chance (Vinicius, 16/10/2026). After it the first answer wins.
const heldPlayerFirst = 10 * time.Second

// playerFirst refuses the master's answer to the question of a player's roll while the player's own chance lasts, with
// the reason PLAYER_FIRST and the seconds left. The player's answer never waits, nor does the master's to a roll
// that no player rolls (an NPC's) or whose player has left the table: that is what the master's answer is for.
func (s *Service) playerFirst(ctx context.Context, m authz.Membership, h heldRoll) error {
	if m.Role != authz.RoleMaster {
		return nil
	}
	left := h.CreatedAt.Add(heldPlayerFirst).Sub(s.now())
	if left <= 0 {
		return nil
	}
	name, present, err := s.rollerPlayer(ctx, h)
	if err != nil || !present {
		return err
	}
	seconds := int32(math.Ceil(left.Seconds()))
	return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_PLAYER_FIRST,
		fmt.Sprintf("%s answers first: the master may answer in %d s", name, seconds),
		func(b *playv1.ResourceBlocked) { b.WaitSeconds = seconds })
}

// rollerPlayer says a player plays the roller of the hold, and the roller's name for the refusal: a combatant by its
// user (a creature carries its owner's), a character out of a combat by the player of the character.
func (s *Service) rollerPlayer(ctx context.Context, h heldRoll) (name string, present bool, err error) {
	if h.CombatantID != "" {
		who, found, err := s.holdCombatant(ctx, h)
		if err != nil || !found {
			return "", false, err
		}
		return who.Label, who.UserID != nil, nil
	}
	cs, err := s.roster.CombatCharacters(ctx, nil, h.CampaignID, []string{h.CharacterID})
	if err != nil {
		return "", false, s.dbError(ctx, "find the character", err)
	}
	if len(cs) != 1 {
		return "", false, nil
	}
	return cs[0].Name, cs[0].PlayerUserID != "", nil
}
