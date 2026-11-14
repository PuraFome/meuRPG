package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The master's undo of the Inspiration (RN-43): a grant, an accepted pass, and the answer to a pass that kept it with the giver. Only
// the ones written in a combat are the combat's actions (favorInCombat): outside one nothing is undone, and the master takes the
// Inspiration back with a grant and a pass.

// favorUndoKinds are the favor events an undo takes back.
var favorUndoKinds = []string{eventFavorGranted, eventFavorPassed, eventFavorAskRefused, eventFavorPassCanceled}

// favorUndoPayload is what the undo reads of a favor event: the character granted, or the pass's giver and receiver.
type favorUndoPayload struct {
	CharacterID     string `json:"character_id"`
	FromCharacterID string `json:"from_character_id"`
	ToCharacterID   string `json:"to_character_id"`
}

// errUndoSpent refuses the undo of an Inspiration the character spent since: the target changed after the action (the reason the
// other undos use when something moved what they would put back), naming the combatant.
func errUndoSpent(combatantID string) error {
	return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_UNDO_TARGET_CHANGED,
		"the inspiration was spent after the action: nothing was undone",
		func(b *playv1.EncounterBlocked) { b.CombatantIds = []string{combatantID} })
}

// takeBackFavor puts the Inspiration back as the event found it, in the undo's transaction: one step for the whole pass (the receiver
// loses it and the giver has it again, or nothing changes). It returns the vitals of the characters it changed, for the table.
func (s *Service) takeBackFavor(ctx context.Context, c *combatTx, kind string, payload []byte, ev actionEvent) ([]*playv1.CharacterVitals, error) {
	var p favorUndoPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("read the favor event: %w", err)
	}
	campaignID := c.session.CampaignID
	var ids []string
	switch kind {
	case eventFavorGranted:
		if _, err := c.q.SpendCharacterFavor(ctx, playdb.SpendCharacterFavorParams{CharacterID: p.CharacterID, CampaignID: campaignID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errUndoSpent(ev.Actor)
			}
			return nil, fmt.Errorf("take the granted inspiration back: %w", err)
		}
		ids = []string{p.CharacterID}
	case eventFavorPassed:
		if _, err := c.q.SpendCharacterFavor(ctx, playdb.SpendCharacterFavorParams{CharacterID: p.ToCharacterID, CampaignID: campaignID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errUndoSpent(ev.Target)
			}
			return nil, fmt.Errorf("take the passed inspiration back: %w", err)
		}
		if _, err := c.q.InsertCharacterFavor(ctx, playdb.InsertCharacterFavorParams{CharacterID: p.FromCharacterID, CampaignID: campaignID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) { // the giver has another by now: the transaction rolls the receiver's back
				return nil, favorBlocked(playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_ALREADY_HAS, "the giver has the inspiration again")
			}
			return nil, fmt.Errorf("give the giver the inspiration back: %w", err)
		}
		ids = []string{p.FromCharacterID, p.ToCharacterID}
	default: // an answer that moved nothing: only its line goes
		return nil, nil
	}
	var out []*playv1.CharacterVitals
	for _, id := range ids {
		v, err := s.vitals.GetVitalsTx(ctx, c.tx, campaignID, id)
		if err != nil {
			continue // a character whose vitals cannot be read tells nothing
		}
		out = append(out, v)
	}
	return out, nil
}

// publishFavorUndo tells what the undo of a favor event changed: the master, and the owners the event was told to, that the combat and its
// log changed. No other player hears of it.
func (s *Service) publishFavorUndo(campaignID string, enc playdb.Encounter, ev actionEvent) {
	s.publishEncounterChangedToMaster(campaignID, enc)
	s.send(campaignID, live.Event{Audience: live.Audience{Master: true}, Message: combatLogChangedMessage(enc.ID, true)})
	if ev.Secret {
		return
	}
	for _, u := range slices.Compact(slices.Sorted(slices.Values(ev.SeenBy))) {
		s.send(campaignID, live.Event{Audience: live.Audience{UserID: u}, Message: combatLogChangedMessage(enc.ID, false)})
	}
}
