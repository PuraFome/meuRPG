package play

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// What package characters asks of the combat that holds a character's
// creatures (MR-037; characters.CreatureHost). The creatures are the
// characters module's; a combat is this one's, so dismissing a creature takes
// its combatant out of the fight from here, in the same transaction.

// CreaturesLeaving takes the combatants of the creatures out of the campaign's
// combat that is not ended, inside tx, with a combatant_removed event. It
// returns the combat's ID, empty when none of them was in one.
func (s *Service) CreaturesLeaving(ctx context.Context, tx pgx.Tx, campaignID, actorUserID string, creatureIDs []string, at time.Time) (string, error) {
	q := s.queries.WithTx(tx)
	session, err := q.GetOpenGameSessionForUpdate(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil // no session: no combat
	}
	if err != nil {
		return "", fmt.Errorf("lock the open session: %w", err)
	}
	enc, err := q.GetOpenEncounter(ctx, session.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("find the open encounter: %w", err)
	}
	cs, err := q.ListCombatants(ctx, enc.ID)
	if err != nil {
		return "", fmt.Errorf("list the combatants: %w", err)
	}
	leaving := slices.DeleteFunc(slices.Clone(cs), func(o playdb.Combatant) bool {
		return o.CreatureID == nil || !slices.Contains(creatureIDs, *o.CreatureID)
	})
	if len(leaving) == 0 {
		return "", nil
	}
	c := &combatTx{tx: tx, q: q, session: session, enc: enc, now: at, actorUserID: actorUserID, svc: s}
	// A creature that leaves keeps the hit points it has (it may be dismissed
	// right after, which the characters module settles).
	if err := s.writeBackCreatures(ctx, c, leaving); err != nil {
		return "", err
	}
	if _, _, err := s.dropCombatants(ctx, c, cs, leaving, false); err != nil {
		return "", err
	}
	if c.enc, err = q.TouchEncounter(ctx, enc.ID); err != nil {
		return "", fmt.Errorf("touch the encounter: %w", err)
	}
	if err := insertEvent(ctx, c, eventCombatantRemoved, &actorUserID, nil, actionEvent{Round: c.enc.Round, Reason: "creature_dismissed", Created: creatureIDsOf(leaving)}); err != nil {
		return "", err
	}
	return enc.ID, nil
}

// CreatureInCombat says whether the creature is a combatant of the campaign's
// combat that is not ended, inside tx.
func (s *Service) CreatureInCombat(ctx context.Context, tx pgx.Tx, campaignID, creatureID string) (bool, error) {
	rows, err := s.queries.WithTx(tx).ListOpenCombatantsOfCreatures(ctx, playdb.ListOpenCombatantsOfCreaturesParams{CampaignID: campaignID, CreatureIds: []string{creatureID}})
	if err != nil {
		return false, fmt.Errorf("find the creature in a combat: %w", err)
	}
	return len(rows) > 0, nil
}

// PublishCreaturesChanged tells the master and the owner's player that their
// creature lists changed. Call it after the commit.
func (s *Service) PublishCreaturesChanged(campaignID, ownerUserID string) {
	s.publishCreaturesChanged(campaignID, ownerUserID)
}

// PublishEncounterChanged tells every stream that the combat changed. Call it
// after the commit.
func (s *Service) PublishEncounterChanged(ctx context.Context, campaignID, encounterID string) {
	enc, err := s.queries.GetEncounterByID(ctx, encounterID)
	if err != nil {
		return // the combat is gone: nothing to tell
	}
	s.publishEncounterChanged(campaignID, enc)
	// A creature that left may have passed the turn, and the new turn ends a familiar's
	// sight (MR-036): the vitals and the fog hear of it too.
	if all, err := s.vitals.ListVitals(ctx, campaignID); err == nil {
		for _, v := range all {
			s.publishVitals(campaignID, v)
		}
	}
	s.maps.VisionChanged(ctx, campaignID, deref(enc.MapID))
}

// LockSession takes the open session's row (characters.CreatureHost): the lock
// every combat write takes first.
func (s *Service) LockSession(ctx context.Context, tx pgx.Tx, campaignID string) error {
	if _, err := s.queries.WithTx(tx).GetOpenGameSessionForUpdate(ctx, campaignID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("lock the open session: %w", err)
	}
	return nil
}

// CreatureRenamed gives the creature's combatant its new name (characters.CreatureHost).
func (s *Service) CreatureRenamed(ctx context.Context, tx pgx.Tx, campaignID, creatureID, name string) (string, error) {
	ids, err := s.queries.WithTx(tx).SetCreatureCombatantLabel(ctx, playdb.SetCreatureCombatantLabelParams{CreatureID: creatureID, Label: name, CampaignID: campaignID})
	if err != nil {
		return "", fmt.Errorf("rename the creature's combatant: %w", err)
	}
	if len(ids) == 0 {
		return "", nil
	}
	return ids[0], nil
}
