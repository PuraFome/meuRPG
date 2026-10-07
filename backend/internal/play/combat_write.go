package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/internal/platform/tablerules"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The kinds of session_events rows a combat writes (session_events_kind_valid).
// Their payloads hold IDs and numbers only, never a name (docs/privacidade.md).
const (
	eventEncounterStarted    = "encounter_started"
	eventInitiativeSubmitted = "initiative_submitted"
	eventInitiativeOrderSet  = "initiative_order_set"
	eventCombatBegun         = "combat_begun"
	eventTurnEnded           = "turn_ended"
	eventCombatantMoved      = "combatant_moved"
	eventCombatantHiddenSet  = "combatant_hidden_set"
	eventCombatantsAdded     = "combatants_added"
	eventCombatantRemoved    = "combatant_removed"
	eventEncounterEnded      = "encounter_ended"
	// The actions of a turn (combat_actions.go, combat_undo.go).
	eventAttackRolled      = "attack_rolled"
	eventDamageRolled      = "damage_rolled"
	eventDamageApplied     = "damage_applied"
	eventDamageDiscarded   = "damage_discarded"
	eventActionTaken       = "action_taken"
	eventHitPointsAdjusted = "hit_points_adjusted"
	eventActionUndone      = "action_undone"
	// Spells, reactions, death saves and conditions (combat_spells.go,
	// combat_reactions.go, combat_death.go, combat_conditions.go).
	eventSpellCast        = "spell_cast"
	eventReactionUsed     = "reaction_used"
	eventReactionDeclined = "reaction_declined"
	eventDeathSaveRolled  = "death_save_rolled"
	eventDeathConfirmed   = "death_confirmed"
	eventConditionsSet    = "conditions_set"
)

// The kinds of Etapa 7: the XP awards (package progression writes them
// through AppendEvent) and the scenes. session_events_kind_valid lists them
// all, and TestSessionEventKindsMatchTheCheck keeps the two in step.
const (
	eventXPAwarded        = "xp_awarded"
	eventXPAwardUndone    = "xp_award_undone"
	eventMilestoneMarked  = "milestone_marked"
	eventSceneOpened      = "scene_opened"
	eventSceneClosed      = "scene_closed"
	eventSceneCheckRolled = "scene_check_rolled"
)

// The kinds of Etapa 8: a clue revealed to players (package maps writes it
// through AppendEvent) and the stage changing (MR-031).
const (
	eventClueRevealed = "clue_revealed"
	eventStageChanged = "stage_changed"
)

// The kinds of the second Etapa 8 wave (migration 00083). The master gave a
// character one more attempt at a scene action (scene_attempt_granted,
// MR-015); a member of a joint turn ended their part and the turn goes on
// (turn_part_ended, MR-013). The last part to end writes turn_ended, as a turn
// always did.
const (
	eventSceneAttemptGranted = "scene_attempt_granted"
	eventTurnPartEnded       = "turn_part_ended"
)

// The kinds of Etapa 9 (migration 00090), added in one migration before the
// slices that write them, so that slices built at the same time never fight
// over the CHECK. Traps and treasure are written by package maps through
// AppendEvent, the rest by this package.
const (
	eventTrapNoticed        = "trap_noticed"
	eventTrapSearched       = "trap_searched"
	eventTrapTriggered      = "trap_triggered"
	eventTrapDisarmed       = "trap_disarmed"
	eventTrapRevealed       = "trap_revealed"
	eventTreasureFound      = "treasure_found"
	eventTreasureUnfound    = "treasure_unfound"
	eventCoverSet           = "cover_set"
	eventSideSet            = "side_set"
	eventOpportunityOffered = "opportunity_offered"
	eventCreatureSummoned   = "creature_summoned"
	eventCreatureDismissed  = "creature_dismissed"
	eventWildShapeStarted   = "wild_shape_started"
	eventWildShapeEnded     = "wild_shape_ended"
	eventFamiliarSight      = "familiar_sight"
)

// The kind of Etapa 10 (migration 00121): a move opened a closed door (MR-010,
// RN-26). Written by this package, in the move's transaction.
const eventDoorOpened = "door_opened"

// combatWrite describes one change to a combat: who makes it, the idempotency
// key, the kind of event it becomes, and the combat it is about (empty when
// the change creates it).
type combatWrite struct {
	m           authz.Membership
	key         string
	kind        string
	encounterID string
	// altKind is the other kind the change may write instead of kind (EndTurn
	// writes turn_part_ended when the turn does not pass yet): a retry of the
	// change under the same key may find either.
	altKind string
}

// combatTx is what a change works with inside its transaction: the open
// session, locked, and the combat. characterID, when the closure sets it,
// is the character the event is about.
type combatTx struct {
	tx          pgx.Tx
	q           *playdb.Queries
	session     playdb.GameSession
	enc         playdb.Encounter
	now         time.Time
	characterID *string
	// kind is the kind of the event the change writes: the one in combatWrite,
	// unless the closure sets it to the altKind.
	kind string
	// castID is the id the pending damages of the spell being cast share.
	castID string
	// actorUserID is who makes the change, for the events a change writes besides
	// its own (the creatures it summons or dismisses).
	actorUserID string
	// svc is the service the change runs in, for what a turn starting does (ending a
	// familiar's sight); nil in the few places that make a combatTx outside write.
	svc *Service
	// told are the vitals of the characters whose Wild Shape form or familiar sight
	// ended: write tells the streams and the fog after the commit (MR-036).
	told []*playv1.CharacterVitals
	// master says the master makes the change: the turn that waits for an
	// opportunity attack's answer never stops him.
	master bool
	// sight is what the players see of the combat's map, read before the
	// transaction opened (nil without the fog of war): it filters what the change
	// tells its caller and stamps who could see the event (combat_fog.go).
	sight *fogSight
	// stamped is the last event insertEvent stamped with who could see it.
	stamped *actionEvent
	// rules are the table's rules (RN-24), read in this transaction when it
	// opened: a critical hit and who sees the death saves follow them from the
	// next roll on, and are never kept longer than the change.
	rules tablerules.Rules
}

// combatResult is what a change leaves for the handler: the session, and
// whether the call was a retry of a change already made.
type combatResult struct {
	session playdb.GameSession
	// payload is the payload of the event a retried change wrote the first
	// time, so a handler can answer a retry with the same numbers.
	payload []byte
	// encounterID is the combat the change was about: the one named in the
	// request, or the one StartEncounter created. Empty only for a retried
	// start, whose combat the retry does not know.
	encounterID string
	repeated    bool
	// kind, characterID and event say what the change wrote, for its log line:
	// the kind of session event, the character it is about, and its payload when
	// that is an actionEvent. Empty for a retry, which wrote nothing.
	kind        string
	characterID string
	event       *actionEvent
	// sight is what the players saw of the combat's map before the change (nil without
	// the fog), and stamped the event the change wrote with who could see it: the
	// publishers use both (withFogMemo).
	sight   *fogSight
	stamped *actionEvent
}

// write runs one change to a combat, as AdjustCharacterVitals runs a vitals
// correction: one transaction locks the open session's row, checks the
// idempotency key, lets do change the rows, and appends the session event.
// do returns the event's payload, or nil when nothing changed, and then no
// event is written. Only after the commit does the handler publish.
func (s *Service) write(ctx context.Context, w combatWrite, do func(c *combatTx) (payload any, err error)) (combatResult, error) {
	// What the players see is read first, never inside the transaction: the maps
	// module asks this one where the combatants stand, and that read would wait for
	// the rows the change has already written. The sight is read with the combat's
	// revision; once the transaction holds the session lock the revision is checked
	// again, and a change that got in between makes the sight stale: it is read again
	// (at most maxSightTries times, then the change is aborted).
	for try := 1; ; try++ {
		sight, err := s.sightForWrite(ctx, w)
		if err != nil {
			return combatResult{}, s.dbError(ctx, "work out what the players see", err)
		}
		if s.afterSightRead != nil {
			s.afterSightRead(w.encounterID) // a test changes the combat here
		}
		res, err := s.writeOnce(ctx, w, sight, do)
		if errors.Is(err, errSightStale) {
			if try >= maxSightTries {
				return combatResult{}, connect.NewError(connect.CodeAborted, errors.New("the combat keeps changing: try again"))
			}
			continue
		}
		if err == nil {
			// Here, after the transaction returned, so a retried transaction
			// logs once.
			s.logCombatEvent(ctx, res)
		}
		return res, err
	}
}

// writeOnce is one try of write, with the sight it was given.
func (s *Service) writeOnce(ctx context.Context, w combatWrite, sight *fogSight, do func(c *combatTx) (payload any, err error)) (combatResult, error) {
	var res combatResult
	var last *actionEvent
	var ended *combatTx // the change's transaction, for what it leaves to tell
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		res, ended = combatResult{encounterID: w.encounterID, sight: sight}, nil
		last = nil
		q := s.queries.WithTx(tx)
		session, err := q.GetOpenGameSessionForUpdate(ctx, w.m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession() // MR-013: during the session
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		res.session = session

		done, err := q.GetSessionEventByIdempotencyKey(ctx, playdb.GetSessionEventByIdempotencyKeyParams{
			GameSessionID: session.ID, IdempotencyKey: &w.key,
		})
		switch {
		case err == nil:
			if done.Kind != w.kind && (w.altKind == "" || done.Kind != w.altKind) {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key was already used for another change"))
			}
			// A key is its author's: another member replaying it would be handed the
			// answer to a change that was not theirs (the vitals of someone else's
			// character, a roll).
			if done.ActorUserID == nil || *done.ActorUserID != w.m.UserID {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key was already used for another change"))
			}
			res.repeated = true // a retry of a change already made
			res.payload = done.Payload
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("find the event of this idempotency key: %w", err)
		}

		c := &combatTx{tx: tx, q: q, session: session, now: s.now(), kind: w.kind, actorUserID: w.m.UserID, svc: s, master: w.m.Role == authz.RoleMaster, sight: sight}
		if c.rules, err = s.tableRules(ctx, tx, w.m.CampaignID); err != nil {
			return err
		}
		if w.encounterID != "" {
			c.enc, err = q.GetEncounterInSession(ctx, playdb.GetEncounterInSessionParams{GameSessionID: session.ID, ID: w.encounterID})
			if errors.Is(err, pgx.ErrNoRows) {
				return connect.NewError(connect.CodeNotFound, errors.New("encounter not found"))
			}
			if err != nil {
				return fmt.Errorf("find the encounter: %w", err)
			}
			if sight != nil && c.enc.Revision != sight.rev {
				return errSightStale
			}
		}
		payload, err := do(c)
		if err != nil || payload == nil {
			return err
		}
		// A change to a combatant's hit points may defeat a creature or give one back
		// (MR-037): the creatures follow, before the change's own event is written.
		if err := s.syncCreatures(ctx, c); err != nil {
			return err
		}
		// The offers nobody can answer any more stop holding the mover's turn.
		if err := s.pruneOffers(ctx, c); err != nil {
			return err
		}
		res.encounterID = c.enc.ID
		res.kind = c.kind
		res.characterID = deref(c.characterID)
		res.event = nil
		if ev, ok := payload.(actionEvent); ok {
			res.event = &ev
		}
		ended = c
		c.stamped = nil
		if err := insertEvent(ctx, c, c.kind, &w.m.UserID, &w.key, payload); err != nil {
			return err
		}
		last = c.stamped
		return nil
	})
	if err != nil {
		return combatResult{}, err
	}
	res.stamped = last
	// A turn that started ended some familiar's sight (MR-036): the player's vitals
	// and the fog's view change.
	if ended != nil && len(ended.told) > 0 {
		for _, v := range ended.told {
			s.publishVitals(w.m.CampaignID, v)
		}
		s.maps.VisionChanged(ctx, w.m.CampaignID, deref(ended.enc.MapID))
	}
	return res, nil
}

// combatEventNames are the names of the combat's log events, where the kind
// of the session event does not already read well as `combat.<kind>`.
var combatEventNames = map[string]string{
	eventEncounterStarted: "combat.started",
	eventCombatBegun:      "combat.begun",
	eventEncounterEnded:   "combat.ended",
	eventTurnEnded:        "combat.turn_passed",
	eventAttackRolled:     "combat.attack_resolved",
	eventSpellCast:        "combat.spell_resolved",
}

// logCombatEvent writes the DEBUG `event` line of a combat change that was
// committed (not a retry, which changed nothing). One place covers every
// change that goes through write: the session event's kind is the name, and
// the line carries ids and the attack's outcome only, never a name.
func (s *Service) logCombatEvent(ctx context.Context, res combatResult) {
	if res.kind == "" || res.repeated {
		return
	}
	name, ok := combatEventNames[res.kind]
	if !ok {
		name = "combat." + res.kind
	}
	attrs := []slog.Attr{slog.String("session_id", res.session.ID), slog.String("encounter_id", res.encounterID)}
	if res.characterID != "" {
		attrs = append(attrs, slog.String("character_id", res.characterID))
	}
	if ev := res.event; ev != nil {
		if ev.Actor != "" {
			attrs = append(attrs, slog.String("combatant_id", ev.Actor))
		}
		if ev.Target != "" {
			attrs = append(attrs, slog.String("target_id", ev.Target))
		}
		if ev.Outcome != "" {
			attrs = append(attrs, slog.String("outcome", ev.Outcome))
		}
		if ev.Round > 0 {
			attrs = append(attrs, slog.Int("round", int(ev.Round)))
		}
	}
	logging.Event(ctx, s.logger, name, attrs...)
}

// insertEvent appends a session event inside the change's transaction.
func insertEvent(ctx context.Context, c *combatTx, kind string, actor, key *string, payload any) error {
	if ev, ok := payload.(actionEvent); ok {
		var err error
		if kind != eventActionUndone { // an undo is no line: its hint goes to every player
			if ev, err = c.stamp(ctx, kind, ev); err != nil { // who could see it, on a fog map
				return err
			}
			c.stamped = &ev
		}
		payload = ev
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode the event payload: %w", err)
	}
	seq, err := c.q.NextSessionEventSeq(ctx, c.session.ID)
	if err != nil {
		return fmt.Errorf("next event number: %w", err)
	}
	// The combat log and the undo find the event by its combat.
	var encounterID *string
	if c.enc.ID != "" {
		encounterID = &c.enc.ID
	}
	if _, err := c.q.InsertSessionEvent(ctx, playdb.InsertSessionEventParams{
		GameSessionID: c.session.ID, Seq: seq, Kind: kind, ActorUserID: actor, CharacterID: c.characterID,
		Payload: body, IdempotencyKey: key, CreatedAt: c.now, EncounterID: encounterID,
	}); err != nil {
		return fmt.Errorf("insert session event: %w", err)
	}
	return nil
}

// finish builds the handler's answer after a change: it reads the combat
// the change was about (the session's latest, for a retried start), lets
// publish tell the streams (unless the call was a retry, which changed
// nothing), and returns the combat as the caller sees it.
func (s *Service) finish(ctx context.Context, m authz.Membership, res combatResult, publish func(ctx context.Context, d *encounterData)) (*playv1.Encounter, error) {
	var enc playdb.Encounter
	var err error
	if res.encounterID != "" {
		enc, err = s.queries.GetEncounterInSession(ctx, playdb.GetEncounterInSessionParams{GameSessionID: res.session.ID, ID: res.encounterID})
	} else {
		enc, err = s.queries.GetLatestEncounter(ctx, res.session.ID)
	}
	if err != nil {
		return nil, s.dbError(ctx, "find the encounter", err)
	}
	d, err := loadEncounter(ctx, s.queries, enc)
	if err != nil {
		return nil, s.dbError(ctx, "read the encounter", err)
	}
	// The sight read after the commit is read once for the publishers and the answer;
	// the one read before it says who could see the old squares.
	ctx = withFogMemo(ctx, res.sight, res.stamped)
	if !res.repeated && publish != nil {
		publish(ctx, d)
	}
	return s.viewFor(ctx, m, d)
}

// changed is the usual publish: a hint that the combat changed.
func (s *Service) changed(campaignID string) func(ctx context.Context, d *encounterData) {
	return func(ctx context.Context, d *encounterData) { s.publishEncounterChanged(ctx, campaignID, d.enc) }
}

// parseKey reads an idempotency key.
func parseKey(raw string) (string, error) {
	key, err := uuid.Parse(raw)
	if err != nil {
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key must be a UUID"))
	}
	return key.String(), nil
}

// parseCombatID reads an ID of a combat or a combatant: not a UUID names
// nothing.
func parseCombatID(raw, what string) (string, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", connect.NewError(connect.CodeNotFound, errors.New(what+" not found"))
	}
	return id.String(), nil
}

func errCombatantNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("combatant not found"))
}

// errEncounter is CombatService's failed_precondition, with the
// EncounterBlocked detail that tells the app why. edit fills the detail's
// extra fields.
func errEncounter(reason playv1.EncounterBlockedReason, msg string, edit ...func(*playv1.EncounterBlocked)) error {
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	blocked := &playv1.EncounterBlocked{Reason: reason}
	for _, e := range edit {
		e(blocked)
	}
	if detail, detailErr := connect.NewErrorDetail(blocked); detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

// findCombatant returns the combatant with the ID, or `not_found`; for a
// player, a hidden one is not found either (RN-10), and one that is not
// theirs is `permission_denied`.
func findCombatant(cs []playdb.Combatant, id string, v combatViewer) (playdb.Combatant, error) {
	for _, c := range cs {
		if c.ID != id {
			continue
		}
		if !v.sees(c) {
			return playdb.Combatant{}, errCombatantNotFound()
		}
		return c, nil
	}
	return playdb.Combatant{}, errCombatantNotFound()
}

// mayAct checks that the viewer may act for the combatant: the master for
// anyone, a player for their own character only.
func (v combatViewer) mayAct(c playdb.Combatant) error {
	if v.master || v.owns(c) {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied, errors.New("only the combatant's player or the master may do this"))
}

// notEnded fails with ENCOUNTER_ENDED when the combat is over.
func notEnded(e playdb.Encounter) error {
	if e.Status == statusEnded {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ENCOUNTER_ENDED, "the combat has ended")
	}
	return nil
}

// saveOrder puts the combatants in the order ordered says, writing the
// places that changed, and returns the list with them set. before is the
// list as stored.
func saveOrder(ctx context.Context, q *playdb.Queries, before, ordered []playdb.Combatant) ([]playdb.Combatant, error) {
	stored := make(map[string]playdb.Combatant, len(before))
	for _, c := range before {
		stored[c.ID] = c
	}
	out := make([]playdb.Combatant, len(ordered))
	for i, c := range ordered {
		was := stored[c.ID]
		c.OrderIndex = int32(i) // at most 40
		if was.OrderIndex != c.OrderIndex || was.TieOrdered != c.TieOrdered {
			if err := q.SetCombatantOrder(ctx, playdb.SetCombatantOrderParams{ID: c.ID, OrderIndex: c.OrderIndex, TieOrdered: c.TieOrdered}); err != nil {
				return nil, fmt.Errorf("save the turn order: %w", err)
			}
		}
		out[i] = c
	}
	return out, nil
}
