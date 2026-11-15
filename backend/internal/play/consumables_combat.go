package play

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/uid"
	"github.com/PuraFome/meuRPG/backend/internal/play/consumable"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// A consumable used in a running combat (SRD 5.1, "Potions" and "Other Activity on Your Turn"): drinking or giving a potion is
// the user's action, taken on its turn like any action (the master takes it for the character, bound by the same turn and
// action). The receiver of a gift is within 5 feet of the giver and seen by it (a hidden creature is never one). A heal goes
// through the combat's own heal, and a resistance is a lasting effect of the receiver's combatant that lasts the hour in rounds.

// consumableFight is the combat a use happens in: its transaction, the combatants, the user's and the receiver's.
type consumableFight struct {
	c           *combatTx
	cs          []playdb.Combatant
	giver, recv playdb.Combatant
}

func errNotAdjacentOrSeen() error {
	return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_NOT_ADJACENT_OR_SEEN, "the receiver is not within 5 feet of the giver, or the giver does not see it")
}

func errItemGone() error {
	return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ITEM_GONE, "the item is no longer on the sheet")
}

// giverView is who sees for the giver: its player, also when the master acts for it (a creature the master hid, or the
// dark of a fog map, hides it from the giver as it does from the player).
func (s *Service) giverView(ctx context.Context, campaignID string, enc playdb.Encounter, cs []playdb.Combatant, giver playdb.Combatant) (combatViewer, error) {
	if giver.UserID == nil {
		return viewerOf(authz.Membership{CampaignID: campaignID, Role: authz.RoleMaster}), nil
	}
	v, _, err := s.viewerWith(ctx, authz.Membership{CampaignID: campaignID, UserID: *giver.UserID, Role: authz.RolePlayer}, enc, cs)
	return v, err
}

// giftReach is the distance to a receiver the giver may hand a potion to, and whether it may: the receiver is seen by the
// giver and within 5 feet (a combat with no map has no distance). The giver itself is always within reach.
func giftReach(enc playdb.Encounter, v combatViewer, giver, recv playdb.Combatant) (int32, bool) {
	switch {
	case recv.ID == giver.ID:
		return 0, true
	case !v.sees(recv):
		return 0, false
	case isTheatre(enc):
		return 0, true
	}
	d, both := distanceFt(giver, recv)
	return d, both && d <= combat.MeleeReachFt
}

// openFight opens the combat the use happens in and spends the action it costs, or nil when neither the user nor the
// receiver fights. A user or receiver that is the only one of the two in the combat, a receiver out of reach or unseen,
// a turn that is not the user's and an action already used refuse, with nothing written.
func (s *Service) openFight(ctx context.Context, tx pgx.Tx, q *playdb.Queries, h *hints, session playdb.GameSession, use consumableCall) (*consumableFight, error) {
	enc, cs, found, err := openCombatants(ctx, q, session.ID)
	if err != nil || !found || enc.Status != statusActive {
		return nil, err
	}
	giver, recv := wearerIn(cs, use.charText), wearerIn(cs, use.target)
	if giver == nil && recv == nil {
		return nil, nil
	}
	if giver == nil || recv == nil {
		return nil, errNotAdjacentOrSeen()
	}
	c, err := s.openTx(ctx, combatTx{tx: tx, q: q, session: session, enc: enc, now: s.now(), hints: h, actorUserID: use.m.UserID, svc: s})
	if err != nil {
		return nil, err
	}
	f := &consumableFight{c: c, cs: cs, giver: *giver, recv: *recv}
	if err := s.mustActNow(ctx, c, f.giver); err != nil {
		return nil, err
	}
	v, err := s.giverView(ctx, session.CampaignID, enc, cs, f.giver)
	if err != nil {
		return nil, err
	}
	if _, ok := giftReach(enc, v, f.giver, f.recv); !ok {
		return nil, errNotAdjacentOrSeen()
	}
	if _, err := s.payEquipWithAction(ctx, c, f.giver); err != nil {
		return nil, err
	}
	return f, nil
}

// name the combatants in the event: the user's combatant, and the receiver's when it is another.
func (f *consumableFight) name(ev *consumableUsedEvent) {
	ev.Round, ev.Actor = f.c.enc.Round, f.giver.ID
	if f.recv.ID != f.giver.ID {
		ev.TargetCombatant = f.recv.ID
	}
}

// finish bumps the combat's revision and tells the streams of the combat and of its log once the transaction commits.
func (f *consumableFight) finish(ctx context.Context, s *Service, h *hints) error {
	if err := touch(ctx, f.c); err != nil {
		return err
	}
	campaignID, enc := f.c.session.CampaignID, f.c.enc
	h.after(func(ctx context.Context, say sayFn) {
		s.tellEncounterChanged(ctx, say, campaignID, enc)
		s.sayLogChange(ctx, say, campaignID, enc.ID, true, false)
	})
	return nil
}

// heal heals the receiver's combatant with the combat's heal and says the hit points it regained.
func (f *consumableFight) heal(ctx context.Context, s *Service, total int32) (int32, error) {
	hit, _, err := s.healCombatant(ctx, f.c, f.recv, total)
	return hit.Amount, err
}

// resist puts the potion's resistance on the receiver's combatant for the hour, in rounds; a second potion of the same kind
// ends the first and starts the hour again.
func (f *consumableFight) resist(ctx context.Context, s *Service, potion rules.PotionEffect, key string) error {
	group := uuid.New().String()
	if err := s.dropSameSpell(ctx, f.c, f.cs, f.recv.ID, key, group); err != nil {
		return err
	}
	_, err := s.addEffects(ctx, f.c, f.cs, effectSpec{
		key: key, sourceKind: "master", group: group, targets: []playdb.Combatant{f.recv}, audience: rules.EffectVisibilityOwner,
		dur:       durationSpec{Kind: rules.EffectDurationRounds, Rounds: clamp32(potion.Rounds, 0, 1<<20), Anchor: f.recv.ID},
		modifiers: []rules.EffectModifier{consumable.Resistance(potion)},
	})
	return err
}

// ListConsumableTargets implements playv1connect.ChargesServiceHandler: who the character may give the consumable to.
func (s *Service) ListConsumableTargets(ctx context.Context, req *connect.Request[playv1.ListConsumableTargetsRequest]) (*connect.Response[playv1.ListConsumableTargetsResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	charID, ok := uid.Canonical(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}
	if _, err := s.requireItemCharacter(ctx, nil, m, charID); err != nil {
		return nil, err
	}
	if _, err := s.vitals.ChargedItemRead(ctx, nil, m.CampaignID, charID, req.Msg.GetEntryId()); connect.CodeOf(err) == connect.CodeNotFound {
		return nil, errItemGone()
	} else if err != nil {
		return nil, err
	}
	rows, err := s.consumableTargets(ctx, m.CampaignID, charID)
	if err != nil {
		return nil, s.dbError(ctx, "list the consumable's targets", err)
	}
	return connect.NewResponse(&playv1.ListConsumableTargetsResponse{Targets: rows}), nil
}

// consumableTargets are the receivers of the character's gift: the creatures it reaches and sees in a running combat, else
// the living characters of the party.
func (s *Service) consumableTargets(ctx context.Context, campaignID, characterID string) ([]*playv1.ConsumableTarget, error) {
	if open, err := s.queries.GetOpenGameSession(ctx, campaignID); err == nil {
		enc, cs, found, err := openCombatants(ctx, s.queries, open.ID)
		if err != nil {
			return nil, err
		}
		if giver := wearerIn(cs, characterID); found && enc.Status == statusActive && giver != nil {
			return s.fightTargets(ctx, campaignID, enc, cs, *giver)
		}
	}
	party, err := s.roster.CombatParty(ctx, nil, campaignID)
	if err != nil {
		return nil, err
	}
	var out []*playv1.ConsumableTarget
	for _, p := range party {
		if p.ID == characterID {
			continue
		}
		v, err := s.vitals.GetVitals(ctx, campaignID, p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, consumable.Target(p.ID, p.Name, 0, v.GetHitPointsMax() > 0 && v.GetHitPointsCurrent() == 0))
	}
	return out, nil
}

// fightTargets are the player's characters of the combat that the giver reaches and sees.
func (s *Service) fightTargets(ctx context.Context, campaignID string, enc playdb.Encounter, cs []playdb.Combatant, giver playdb.Combatant) ([]*playv1.ConsumableTarget, error) {
	v, err := s.giverView(ctx, campaignID, enc, cs, giver)
	if err != nil {
		return nil, err
	}
	var out []*playv1.ConsumableTarget
	for _, c := range cs {
		if c.ID == giver.ID || c.Kind != kindPlayer || c.Dismissed {
			continue
		}
		d, ok := giftReach(enc, v, giver, c)
		if !ok {
			continue
		}
		down, err := s.isDown(ctx, nil, campaignID, c)
		if err != nil {
			return nil, err
		}
		out = append(out, consumable.Target(c.CharacterID, c.Label, d, down))
	}
	return out, nil
}
