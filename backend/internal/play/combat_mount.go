package play

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Mounted combat (SRD 5.1, "Mounted Combat"). The rider and its mount stay two combatants: the rider row names the mount it sits
// on (mounted_on_id), the mount row says whether it is a willing mount and how it obeys (mount_kind).
//
// The kinds of a mount: "" is not a mount, "controlled" obeys its rider and "independent" keeps its own turn.
const (
	mountKindNone        = ""
	mountKindControlled  = "controlled"
	mountKindIndependent = "independent"
)

// mountKindOf is the kind the request names, and false for a word that is not one of the three.
func mountKindOf(kind string) (string, bool) {
	switch kind {
	case mountKindNone, mountKindControlled, mountKindIndependent:
		return kind, true
	}
	return "", false
}

// SetCombatantMount implements playv1connect.CombatServiceHandler.
func (s *Service) SetCombatantMount(
	ctx context.Context,
	req *connect.Request[playv1.SetCombatantMountRequest],
) (*connect.Response[playv1.SetCombatantMountResponse], error) {
	call, err := readMasterTargetCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetIdempotencyKey(), req.Msg.GetEncounterId(), req.Msg.GetCombatantId())
	if err != nil {
		return nil, err
	}
	kind, ok := mountKindOf(req.Msg.GetKind())
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New(`kind must be "controlled", "independent" or ""`))
	}
	m := call.m
	res, err := s.write(ctx, combatWrite{m: m, key: call.key, hash: idem.Hash(req.Msg), kind: eventMountKindSet, encounterID: call.encID}, func(c *combatTx) (any, error) {
		return s.mountKindIn(ctx, c, call, kind)
	})
	if err != nil {
		return nil, s.dbError(ctx, "mark a combatant as a mount", err)
	}
	out, err := s.finish(ctx, m, res, s.changedFor(m.CampaignID, call.combID))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.SetCombatantMountResponse{Encounter: out}), nil
}

// mountKindIn is the change of SetCombatantMount: the master marks a combatant as a willing mount, or takes the mark off. A
// player's character is never a mount, a rider is not marked, and the mark stays on a mount that carries a rider.
func (s *Service) mountKindIn(ctx context.Context, c *combatTx, call masterTargetCall, kind string) (any, error) {
	if err := notEnded(c.enc); err != nil {
		return nil, err
	}
	target, err := s.masterTargetOf(ctx, c, call)
	if err != nil {
		return nil, err
	}
	if target.Kind == kindPlayer {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a player's character is not a mount"))
	}
	if target.MountedOnID != nil {
		return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ALREADY_MOUNTED, "a combatant that rides is not a mount: it gets off first",
			func(b *playv1.EncounterBlocked) { b.CombatantIds = []string{target.ID} })
	}
	if kind == mountKindNone {
		if err := mustCarryNobody(ctx, c, target.ID); err != nil {
			return nil, err
		}
	}
	if err := c.q.SetCombatantMountKind(ctx, playdb.SetCombatantMountKindParams{ID: target.ID, MountKind: kind}); err != nil {
		return nil, fmt.Errorf("set the mount kind: %w", err)
	}
	if err := touch(ctx, c); err != nil {
		return nil, err
	}
	c.characterID = &target.CharacterID
	return actionEvent{Round: c.enc.Round, Secret: target.Hidden, Actor: target.ID, Key: kind}, nil
}

// mustCarryNobody refuses with MOUNT_TAKEN when a combatant sits on the mount.
func mustCarryNobody(ctx context.Context, c *combatTx, mountID string) error {
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return err
	}
	if riderOf(cs, mountID) != nil {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_MOUNT_TAKEN, "the mount carries a rider: the rider gets off first",
			func(b *playv1.EncounterBlocked) { b.CombatantIds = []string{mountID} })
	}
	return nil
}

// riderOf is the combatant that sits on the mount, or nil.
func riderOf(cs []playdb.Combatant, mountID string) *playdb.Combatant {
	for i := range cs {
		if cs[i].MountedOnID != nil && *cs[i].MountedOnID == mountID {
			return &cs[i]
		}
	}
	return nil
}

// mountFor puts on out, the copy of c that the viewer reads, what the viewer may know of the mount and the rider (RN-10, RN-20).
// The link goes to a viewer who sees both rows, so a hidden mount leaves its rider plain and a hidden rider leaves its mount
// without one. The mark is the master's and the player's who controls the combatant: it says who the master made a mount of.
// cs is every combatant of the combat, in whatever order.
func mountFor(out *playv1.Combatant, c playdb.Combatant, cs []playdb.Combatant, v combatViewer) {
	if v.master || v.owns(c) {
		out.MountKind = c.MountKind
	}
	if c.MountedOnID != nil {
		for _, other := range cs {
			if other.ID == *c.MountedOnID && v.sees(other) {
				out.MountedOnId = other.ID
			}
		}
	}
	if rider := riderOf(cs, c.ID); rider != nil && v.sees(*rider) {
		out.RiderId = rider.ID
	}
}
