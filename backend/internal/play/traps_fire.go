package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	maplink "github.com/PuraFome/meuRPG/backend/internal/maps/link"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The master firing a trap, and a token landing in one (MR-035, D5): in a combat
// that runs on the trap's map the firing is a combat change (combat_traps.go);
// otherwise it is written here, with the damage to a player's character waiting for
// the master in trap_damages. NPCs outside a combat have no hit points stored, so
// what a trap does to one is only a line of the history.

// maxTrapTargets is how many creatures one firing may catch.
const maxTrapTargets = 40

// FireTrap implements playv1connect.PlayServiceHandler.
func (s *Service) FireTrap(
	ctx context.Context,
	req *connect.Request[playv1.FireTrapRequest],
) (*connect.Response[playv1.FireTrapResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	mapID, err := uuid.Parse(req.Msg.GetMapId())
	if err != nil {
		return nil, errTrapNotFound()
	}
	pointID, err := uuid.Parse(req.Msg.GetPointId())
	if err != nil {
		return nil, errTrapNotFound()
	}
	targets := req.Msg.GetTargetIds()
	if len(targets) > maxTrapTargets {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("target_ids must have at most %d ids", maxTrapTargets))
	}
	for _, id := range targets {
		if _, err := uuid.Parse(id); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target_ids must be UUIDs"))
		}
	}
	extend := req.Msg.GetExtendFiringId()
	if extend != "" {
		if _, err := uuid.Parse(extend); err != nil {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("firing not found"))
		}
		if len(targets) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("extend_firing_id needs the target_ids to add"))
		}
	}
	if s.traps == nil {
		return nil, errNoTraps()
	}
	traps, err := s.traps.Traps(ctx, nil, m.CampaignID, mapID.String())
	if err != nil {
		return nil, s.dbError(ctx, "read the map's traps", err)
	}
	i := slices.IndexFunc(traps, func(t maplink.Trap) bool { return t.PointID == pointID.String() })
	if i < 0 {
		return nil, errTrapNotFound()
	}
	trap := traps[i]
	session, err := s.openSession(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	enc, inCombat, err := s.runningEncounterOn(ctx, session.ID, trap.MapID)
	if err != nil {
		return nil, s.dbError(ctx, "find the combat", err)
	}
	if extend != "" {
		// The firing to extend is this trap's, from this session, in the same place (a
		// combat, or outside one), and the trap is as it left it.
		row, err := s.queries.GetSessionEventByID(ctx, playdb.GetSessionEventByIDParams{GameSessionID: session.ID, ID: extend})
		var host actionEvent
		if err == nil {
			err = json.Unmarshal(row.Payload, &host)
		}
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (row.Kind != eventTrapTriggered || host.Trap == nil || host.Trap.PointID != trap.PointID)) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("firing not found"))
		}
		if err != nil {
			return nil, s.dbError(ctx, "read the firing", err)
		}
		if host.Trap.ExtendsID != "" || (inCombat && (row.EncounterID == nil || *row.EncounterID != enc.ID)) || (!inCombat && row.EncounterID != nil) || trap.State != "triggered" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("that firing cannot be extended now"))
		}
	}
	if inCombat {
		return s.fireByHandInCombat(ctx, m, key, trap, enc, targets, extend)
	}
	fired, firingID, repeated, err := s.fireOutsideCombat(ctx, m.CampaignID, m.UserID, key, trap, targets, true, extend)
	if err != nil {
		return nil, s.dbError(ctx, "fire a trap", err)
	}
	if !repeated {
		s.afterFiring(ctx, m.CampaignID, fired, playdb.Encounter{})
	}
	names, err := s.characterLabels(ctx, m.CampaignID, fired)
	if err != nil {
		return nil, s.dbError(ctx, "read the characters' names", err)
	}
	if extend != "" {
		firingID = extend
	}
	return connect.NewResponse(&playv1.FireTrapResponse{Firing: firingProto(fired, firingID, trap.Name, trapView{master: true, label: func(id string) string { return names[id] }})}), nil
}

// fireByHandInCombat is FireTrap while a combat runs on the trap's map.
func (s *Service) fireByHandInCombat(ctx context.Context, m authz.Membership, key string, trap maplink.Trap, enc playdb.Encounter, targetIDs []string, extend string) (*connect.Response[playv1.FireTrapResponse], error) {
	var made actionEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventTrapTriggered, encounterID: enc.ID}, func(c *combatTx) (any, error) {
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		var caught []playdb.Combatant
		if len(targetIDs) == 0 {
			caught = inArea(cs, trap)
		}
		for _, id := range targetIDs {
			who, err := findCombatant(cs, id, combatViewer{master: true})
			if err != nil {
				return nil, err
			}
			if !slices.ContainsFunc(caught, func(o playdb.Combatant) bool { return o.ID == who.ID }) {
				caught = append(caught, who)
			}
		}
		fired, err := s.fireInCombat(ctx, c, trap, caught, true, extend)
		if errors.Is(err, maplink.ErrTrapNotArmed) {
			return nil, errTrapNotArmed()
		}
		if err != nil {
			return nil, err
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		made = actionEvent{Round: c.enc.Round, Trap: fired}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "fire a trap", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the firing", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.afterFiring(ctx, m.CampaignID, ev.Trap, d.enc)
	})
	if err != nil {
		return nil, err
	}
	_ = out
	firingID := extend
	if firingID == "" {
		row, err := s.queries.GetSessionEventByIdempotencyKey(ctx, playdb.GetSessionEventByIdempotencyKeyParams{GameSessionID: res.session.ID, IdempotencyKey: &key})
		if err != nil {
			return nil, s.dbError(ctx, "read the firing", err)
		}
		firingID = row.ID
	}
	label := s.membersOf(ctx, res)
	return connect.NewResponse(&playv1.FireTrapResponse{Firing: firingProto(ev.Trap, firingID, trap.Name, trapView{master: true, label: func(id string) string {
		c, _ := label(id)
		return c.Label
	}})}), nil
}

// afterFiring tells the streams, after the commit, what a firing changed: the trap
// is public now (everyone who sees the map), the combat and its log changed, and
// the master's trap card has new damage to apply.
func (s *Service) afterFiring(ctx context.Context, campaignID string, fired *trapFireEvent, enc playdb.Encounter) {
	if fired == nil {
		return
	}
	s.traps.TrapChanged(ctx, campaignID, fired.MapID, fired.PointID)
	if enc.ID != "" {
		s.publishEncounterChanged(ctx, campaignID, enc)
		s.publishLogChanged(ctx, campaignID, enc.ID, true)
	}
	s.Publish(campaignID, false, mapChangedHint(fired.MapID)) // the master's trap card: damage waits there
}

// characterLabels returns the names of the characters a firing caught.
func (s *Service) characterLabels(ctx context.Context, campaignID string, fired *trapFireEvent) (map[string]string, error) {
	var ids []string
	for _, cc := range fired.Caught {
		ids = append(ids, cc.Target)
	}
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	chars, err := s.roster.SessionCharacters(ctx, campaignID, ids)
	if err != nil {
		return nil, err
	}
	for _, c := range chars {
		out[c.ID] = c.Name
	}
	return out, nil
}

// tokensOn is where the map's tokens stand, in squares, with the characters that
// live (a dead character's token stays on the map and is never caught).
func (s *Service) tokensOn(ctx context.Context, campaignID, mapID string) (map[string]grid.Square, []link.Character, error) {
	g, err := s.maps.MapGrid(ctx, campaignID, mapID)
	if err != nil {
		return nil, nil, err
	}
	tokens, err := s.maps.MapTokens(ctx, mapID)
	if err != nil {
		return nil, nil, err
	}
	if !g.OK() {
		return nil, nil, nil
	}
	gr := grid.Grid{Columns: int(g.Columns), Rows: int(g.Rows)}
	at := map[string]grid.Square{}
	ids := make([]string, 0, len(tokens))
	for _, t := range tokens {
		at[t.CharacterID] = gr.SquareOf(int(t.XBP), int(t.YBP))
		ids = append(ids, t.CharacterID)
	}
	living, err := s.roster.CombatCharacters(ctx, campaignID, ids)
	if err != nil {
		return nil, nil, err
	}
	return at, living, nil
}

// fireOutsideCombat fires the trap while no combat runs on its map, in a
// transaction of its own, and returns the firing and whether it was a retry. key
// is "" for a firing nobody retries (a token landing); targetIDs are characters
// with tokens on the map, empty for every token in the area.
func (s *Service) fireOutsideCombat(ctx context.Context, campaignID, actorUserID, key string, trap maplink.Trap, targetIDs []string, manual bool, extend string) (*trapFireEvent, string, bool, error) {
	at, living, err := s.tokensOn(ctx, campaignID, trap.MapID)
	if err != nil {
		return nil, "", false, err
	}
	var caught []link.Character
	for _, ch := range living {
		sq, onMap := at[ch.ID]
		switch {
		case len(targetIDs) == 0 && onMap && trap.Covers(sq):
			caught = append(caught, ch)
		case slices.Contains(targetIDs, ch.ID):
			caught = append(caught, ch)
		}
	}
	for _, id := range targetIDs {
		if !slices.ContainsFunc(caught, func(ch link.Character) bool { return ch.ID == id }) {
			return nil, "", false, connect.NewError(connect.CodeInvalidArgument, errors.New("target_ids must be characters with a token on the trap's map"))
		}
	}

	var fired *trapFireEvent
	var firingID string
	var repeated bool
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		fired, firingID, repeated = nil, "", false
		session, err := q.GetOpenGameSessionForUpdate(ctx, campaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		if key != "" {
			done, err := q.GetSessionEventByIdempotencyKey(ctx, playdb.GetSessionEventByIdempotencyKeyParams{GameSessionID: session.ID, IdempotencyKey: &key})
			switch {
			case err == nil:
				if done.Kind != eventTrapTriggered {
					return connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key was already used for another change"))
				}
				var ev actionEvent
				if err := json.Unmarshal(done.Payload, &ev); err != nil {
					return fmt.Errorf("decode the event of %s: %w", done.ID, err)
				}
				fired, firingID, repeated = ev.Trap, done.ID, true
				return nil
			case !errors.Is(err, pgx.ErrNoRows):
				return fmt.Errorf("find the event of this idempotency key: %w", err)
			}
		}
		c := &combatTx{tx: tx, q: q, session: session, now: s.now(), actorUserID: actorUserID}
		if fired, err = s.fireOutside(ctx, c, trap, caught, manual, extend); err != nil {
			return err
		}
		var keyPtr *string
		if key != "" {
			keyPtr = &key
		}
		firingID, err = insertSceneEvent(ctx, c, eventTrapTriggered, &actorUserID, keyPtr, actionEvent{Trap: fired})
		return err
	})
	if errors.Is(err, maplink.ErrTrapNotArmed) {
		return nil, "", false, errTrapNotArmed()
	}
	return fired, firingID, repeated, err
}

// fireOutside resolves the trap against characters, inside the transaction: the
// trap is triggered, the effect rolled, and the damage to a player's character
// waits for the master in trap_damages. An NPC has no hit points stored outside a
// combat: its damage is only in the event. Conditions are a reminder in it.
func (s *Service) fireOutside(ctx context.Context, c *combatTx, trap maplink.Trap, caught []link.Character, manual bool, extend string) (*trapFireEvent, error) {
	campaignID := c.session.CampaignID
	prev := trap
	ev := &trapFireEvent{PointID: trap.PointID, MapID: trap.MapID, Manual: manual, ExtendsID: extend}
	if extend == "" {
		var err error
		if prev, err = s.traps.TriggerTrap(ctx, c.tx, campaignID, trap.MapID, trap.PointID, c.now); err != nil {
			return nil, err
		}
		ev.PrevState, ev.PrevTriggeredAt = prev.State, prev.TriggeredAt
	}
	effect := prev.Spec.GetEffect()
	ability := trapSaveAbility(effect)
	targets := make([]trapTarget, len(caught))
	for i, ch := range caught {
		targets[i].id = ch.ID
		if trapNeedsAC(effect) {
			sheet, err := s.roster.CombatSheet(ctx, c.tx, campaignID, ch.ID)
			if err != nil {
				return nil, err
			}
			targets[i].armorClass = sheet.ArmorClass
		}
		if ability != "" {
			save, err := s.roster.CombatSave(ctx, campaignID, ch.ID, ability)
			if err != nil {
				return nil, err
			}
			targets[i].save, targets[i].saveKnown = save.Bonus, save.Known
		}
	}
	outcomes, err := resolveTrap(effect, targets, s.trapD20, s.trapDice)
	if err != nil {
		return nil, err
	}
	fireID := uuid.New().String()
	for i, o := range outcomes {
		ch := caught[i]
		cc := trapCaughtEvent{Target: ch.ID, Character: ch.ID, Player: ch.Player, Conditions: o.conditions, Saves: savesEventOf(o.saves)}
		for _, a := range o.attacks {
			outcome := outcomeMiss
			switch {
			case a.hit && a.critical:
				outcome = outcomeCrit
			case a.hit:
				outcome = outcomeHit
			}
			cc.Attacks = append(cc.Attacks, trapAttackEvent{D20: clampInt32(a.d20), Modifier: clampInt32(a.bonus), Total: clampInt32(a.total), Outcome: outcome, TargetAC: clampInt32(a.armorClass)})
		}
		for _, d := range o.damages {
			if d.amount <= 0 && !d.half {
				continue
			}
			de := trapDamageEvent{
				damageHit: damageHit{Target: ch.ID, Amount: clampInt32(d.amount), Half: d.half}, Type: d.damageType,
				DiceCount: clamp32(d.count, 0, 100), DiceSides: clamp32(d.sides, 0, 100), Bonus: clamp32(d.bonus, -1000, 1000),
				Faces: faces32(d.faces), RollTotal: clampInt32(d.rollTotal), Critical: d.critical,
			}
			if ch.Player {
				row, err := c.q.InsertTrapDamage(ctx, playdb.InsertTrapDamageParams{
					GameSessionID: c.session.ID, TrapPointID: trap.PointID, FireID: fireID, CharacterID: ch.ID, Critical: d.critical,
					DiceCount: de.DiceCount, DiceSides: de.DiceSides, DiceBonus: de.Bonus, DamageType: d.damageType, Faces: de.Faces,
					RollTotal: de.RollTotal, Half: d.half, Amount: de.Amount, CreatedAt: c.now,
				})
				if err != nil {
					return nil, fmt.Errorf("open the trap's damage: %w", err)
				}
				de.Pending = row.ID
			} else {
				de.Applied = true // an NPC outside a combat has no hit points: the line says what it took
			}
			cc.Damages = append(cc.Damages, de)
		}
		ev.Caught = append(ev.Caught, cc)
	}
	return ev, nil
}

// TokenDropped implements maps.TrapFirer: the master dropped a player's
// character's token on a square, and the armed "Ao entrar na área" traps whose area
// holds it fire (during a session, while no combat runs on the map; in a combat the
// character's moves are the combat's). A failure is the maps module's to log.
func (s *Service) TokenDropped(ctx context.Context, campaignID, mapID, characterID, actorUserID string, at grid.Square) error {
	if s.traps == nil {
		return nil
	}
	session, err := s.queries.GetOpenGameSession(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // only during a session: the vitals and the history need one
	}
	if err != nil {
		return fmt.Errorf("find the open session: %w", err)
	}
	if _, inCombat, err := s.runningEncounterOn(ctx, session.ID, mapID); err != nil || inCombat {
		return err
	}
	traps, err := s.traps.Traps(ctx, nil, campaignID, mapID)
	if err != nil {
		return err
	}
	for _, t := range traps {
		if !t.Armed() || !t.OnEnter || !t.Covers(at) {
			continue
		}
		targets := []string{characterID}
		if t.Spec.GetEffect().GetTargets() != rulesv1.TrapTargets_TRAP_TARGETS_MANUAL {
			targets = nil // everyone standing in the area, the one that landed included
		}
		fired, _, _, err := s.fireOutsideCombat(ctx, campaignID, actorUserID, "", t, targets, false, "")
		if err != nil && connect.CodeOf(err) == connect.CodeFailedPrecondition {
			continue // fired or disarmed since it was read: it does not fire twice
		}
		if err != nil {
			return err
		}
		s.afterFiring(ctx, campaignID, fired, playdb.Encounter{})
	}
	return nil
}
