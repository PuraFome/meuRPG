package play

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

// "Pôr no combate" (MR-042, RN-29): monsters of an SRD creature join a combat.
// Each is an NPC combatant, linked to an NPC of its own that the characters
// module makes from the creature (hidden from the master's list), so the rest of
// the combat treats it as any NPC: the players see its state word and never its
// hit points or armor class (RN-20), a hidden one is not sent at all, its
// attacks go through RollAttack and its XP counts by the creature's CR (RN-09).

// maxMonsterName is the longest base name: a label is the name, a space and a
// number up to 10, and a label never passes maxLabelLength.
const maxMonsterName = 30

// monstersEvent is what the event of an add keeps, ids and numbers only (never a
// name: the name is kept as a hash, to check a retry).
type monstersEvent struct {
	CreatureKey string `json:"creature_key"`
	Count       int    `json:"count"`
	NameHash    string `json:"name_hash"`
	Rolled      bool   `json:"rolled"`
	Hidden      bool   `json:"hidden"`
	// Items are the new monsters, in the order of their labels.
	Items []monsterItem `json:"items"`
}

// monsterItem is one new monster: its hit points and, when they were rolled, the
// dice, their faces and the bonus.
type monsterItem struct {
	ID        string  `json:"id"`
	HitPoints int32   `json:"hit_points"`
	Dice      string  `json:"dice,omitempty"`
	Faces     []int32 `json:"faces,omitempty"`
	Modifier  int32   `json:"modifier,omitempty"`
}

// sameAdd says whether a retry asks for what the event kept.
func (e *monstersEvent) sameAdd(o monstersEvent) bool {
	return e.CreatureKey == o.CreatureKey && e.Count == o.Count && e.NameHash == o.NameHash && e.Rolled == o.Rolled && e.Hidden == o.Hidden
}

// AddMonsters implements playv1connect.CombatServiceHandler.
func (s *Service) AddMonsters(
	ctx context.Context,
	req *connect.Request[playv1.AddMonstersRequest],
) (*connect.Response[playv1.AddMonstersResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	count := int(req.Msg.GetCount())
	if count == 0 {
		count = 1
	}
	if count < 1 || count > maxNPCCopies {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("count must be 1 to %d", maxNPCCopies))
	}
	var rolled bool
	switch req.Msg.GetHitPoints() {
	case playv1.MonsterHitPoints_MONSTER_HIT_POINTS_UNSPECIFIED, playv1.MonsterHitPoints_MONSTER_HIT_POINTS_AVERAGE:
	case playv1.MonsterHitPoints_MONSTER_HIT_POINTS_ROLLED:
		rolled = true
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("hit_points is not a known mode"))
	}
	hidden := req.Msg.Hidden == nil || req.Msg.GetHidden() // a new monster starts hidden, as any NPC

	// What the creature is, read before the write (never through the pool inside it).
	creatureKey := req.Msg.GetCreatureKey()
	hp, ok, err := s.roster.MonsterHitPoints(ctx, nil, m.CampaignID, creatureKey)
	if err != nil {
		return nil, s.dbError(ctx, "read a creature", err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("creature_key is not an SRD creature"))
	}
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		contentName, err := s.roster.ContentNames(ctx, nil, m.CampaignID)
		if err != nil {
			return nil, s.dbError(ctx, "read the content names", err)
		}
		pt := []rune(contentName(creatureKey))
		name = string(pt[:min(maxMonsterName, len(pt))])
	}
	if name, err = names.Clean(name, maxMonsterName); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name: %w", err))
	}
	sum := sha256.Sum256([]byte(name))
	asked := monstersEvent{CreatureKey: creatureKey, Count: count, NameHash: hex.EncodeToString(sum[:8]), Rolled: rolled, Hidden: hidden}

	var done monstersEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventCombatantsAdded, encounterID: encID}, func(c *combatTx) (any, error) {
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		if len(cs)+count > maxCombatants {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("a combat has at most %d combatants", maxCombatants))
		}
		npc, ok, err := s.roster.MonsterNpc(ctx, c.tx, m.CampaignID, m.UserID, creatureKey, c.now)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("creature_key is not an SRD creature"))
		}
		// The hit points of each copy, the hit dice before the d20 of the initiative.
		items := make([]monsterItem, count)
		points := make([]int, count)
		for i := range items {
			items[i].HitPoints = int32(hp.Average) //nolint:gosec // a creature's average, well inside int32
			if rolled && hp.DiceCount > 0 {
				if items[i], err = s.rollMonsterHitPoints(hp); err != nil {
					return nil, err
				}
			}
			points[i] = int(items[i].HitPoints)
		}
		parts := []planned{{char: npc, count: count, hidden: hidden, name: name, hitPoints: points}}
		_, news, err := s.addParticipants(ctx, c, link.Grid{Columns: c.enc.GridColumns, Rows: c.enc.GridRows}, cs, parts)
		if err != nil {
			return nil, err
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		done = asked
		done.Items = items
		for i, n := range news {
			done.Items[i].ID = n.ID
		}
		return actionEvent{Round: c.enc.Round, Monsters: &done}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "add monsters", err)
	}
	if res.repeated {
		// A retry: the same parameters get the same answer; any other is refused, as is
		// a key another kind of change (AddCombatants) used.
		ev, err := readEvent(res.payload)
		if err != nil || ev.Monsters == nil || !ev.Monsters.sameAdd(asked) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key was already used for another change"))
		}
		done = *ev.Monsters
	}
	out, err := s.finish(ctx, m, res, s.changed(m.CampaignID))
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(done.Items))
	for i, it := range done.Items {
		ids[i] = it.ID
	}
	return connect.NewResponse(&playv1.AddMonstersResponse{Encounter: out, CombatantIds: ids}), nil
}

// rollMonsterHitPoints rolls a creature's hit dice, in the app, and adds its
// bonus: at least 1.
func (s *Service) rollMonsterHitPoints(hp link.MonsterHitPoints) (monsterItem, error) {
	res, err := dice.Roll(s.roller, dice.Expr{Count: hp.DiceCount, Sides: hp.DiceSides})
	if err != nil {
		return monsterItem{}, fmt.Errorf("roll the hit points of a monster: %w", err)
	}
	faces := make([]int32, len(res.Faces))
	for i, f := range res.Faces {
		faces[i] = clamp32(f, 1, 1000)
	}
	expr := dice.Expr{Count: hp.DiceCount, Sides: hp.DiceSides, Modifier: hp.DiceBonus}
	return monsterItem{
		HitPoints: clamp32(max(res.Total+hp.DiceBonus, 1), 1, math.MaxInt32), Dice: expr.String(), Faces: faces, Modifier: clamp32(hp.DiceBonus, -1000, 1000),
	}, nil
}
