package play

import (
	"context"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/platform/sessiontell"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The session events of an Artificer's infusions (the characters module's character_infusions). Each payload holds ids and
// keys, never a name or free text. characters.ReviewHost.InfusionChanged writes them (InfusionChanged).
const (
	// eventInfusionInfused: an item was infused.
	eventInfusionInfused = "infusion_infused"
	// eventInfusionMoved: an infusion was put on another item.
	eventInfusionMoved = "infusion_moved"
	// eventInfusionEnded: an infusion ended.
	eventInfusionEnded = "infusion_ended"
)

// infusionEvent is the payload of the three infusion events: the record, the artificer, the holder and the infusion's
// option, with the item it is on. Ended is why a record ended (infusion_ended only).
type infusionEvent struct {
	RecordID    string `json:"record_id"`
	ArtificerID string `json:"artificer_character_id"`
	HolderID    string `json:"holder_character_id"`
	Infusion    string `json:"infusion_key"`
	ItemKind    string `json:"item_kind"`
	ItemKey     string `json:"item_key"`
	Ended       string `json:"ended_reason,omitempty"`
}

// infusionKinds are the session event kinds of the changes a write reports.
var infusionKinds = map[link.InfusionChangeKind]string{
	link.InfusionInfused: eventInfusionInfused,
	link.InfusionMoved:   eventInfusionMoved,
	link.InfusionEnded:   eventInfusionEnded,
}

// InfusionChanged implements characters.ReviewHost: the changes of an artificer's infusions, in one write.
//
// With an open session each change is one session event of its kind, in the order of the call, about the artificer, with no
// combat of its own: no read lists these events (the combat's log reads the events of a combat and makes a line of the kinds it
// knows, and these are not among them), so no player reads one, and the master's undo meets them only as events that close its
// chain. With no open session the records and the hint of the characters module still happen, and nothing is written here.
//
// The open combat, running or in setup, that holds the artificer or the holder as a combatant changes: its revision goes up and its
// streams are told, so an open screen reads its turn and its attack options again (they are worked out from the sheet and the records
// at every read). A combatant the players cannot see tells only the master. The combat's log gets no line.
//
// It runs inside characters' InSession, with the session's row already held: what it tells goes out through t after the commit.
func (s *Service) InfusionChanged(ctx context.Context, tx pgx.Tx, campaignID, actorUserID string, changes []link.InfusionChange, t sessiontell.Tell) error {
	q := s.queries.WithTx(tx)
	session, enc, found, err := openCombatOf(ctx, q, campaignID)
	if err != nil || session.ID == "" {
		return err // no open session: no event, and no combat
	}
	for _, c := range changes {
		kind, ok := infusionKinds[c.Kind]
		if !ok {
			return fmt.Errorf("an infusion change of kind %q", c.Kind)
		}
		if err := s.appendKeyedEvent(ctx, q, keyedEvent{sessionID: session.ID, kind: kind, userID: actorUserID, characterID: &c.ArtificerID}, infusionEventOf(c)); err != nil {
			return err
		}
	}
	if !found {
		return nil
	}
	return s.infusionCombatChanged(ctx, q, campaignID, enc, changes, t)
}

// infusionEventOf is the payload of a change's session event.
func infusionEventOf(c link.InfusionChange) infusionEvent {
	return infusionEvent{
		RecordID: c.RecordID, ArtificerID: c.ArtificerID, HolderID: c.HolderID, Infusion: c.InfusionKey,
		ItemKind: c.ItemKind, ItemKey: c.ItemKey, Ended: c.EndedReason,
	}
}

// infusionCombatChanged touches the combat and tells it changed when a character the changes name fights in it.
func (s *Service) infusionCombatChanged(ctx context.Context, q *playdb.Queries, campaignID string, enc playdb.Encounter, changes []link.InfusionChange, t sessiontell.Tell) error {
	cs, err := q.ListCombatants(ctx, enc.ID)
	if err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	var fighting []playdb.Combatant
	for _, c := range changes {
		for _, id := range []string{c.ArtificerID, c.HolderID} {
			if i := slices.IndexFunc(cs, func(x playdb.Combatant) bool { return x.CharacterID == id && !x.Dismissed }); i >= 0 {
				fighting = append(fighting, cs[i])
			}
		}
	}
	if len(fighting) == 0 {
		return nil
	}
	touched, err := q.TouchEncounter(ctx, enc.ID)
	if err != nil {
		return fmt.Errorf("touch the encounter: %w", err)
	}
	seen := slices.ContainsFunc(fighting, func(x playdb.Combatant) bool { return !x.Hidden })
	st := tellOf(t)
	st.touchSteps()
	st.later(func(ctx context.Context, say sayFn) {
		if seen {
			s.tellEncounterChanged(ctx, say, campaignID, touched)
			return
		}
		say(campaignID, live.Event{Audience: live.Audience{Master: true}, Message: encounterChangedMessage(touched)})
	})
	return nil
}
