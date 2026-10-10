package play

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Grapple, shove and escape (SRD 5.1, "Contests", "Grappling", "Shoving a Creature").
//
// A grapple or a shove is a special melee attack of the Attack action: it replaces one
// attack, and instead of an attack roll the two sides roll Strength (Athletics) against
// Strength (Athletics) or Dexterity (Acrobatics), the defender's choice. The initiator
// rolls first (StartContest, which spends the attack in the same transaction); the
// defender answers (RespondContest): the player for their own character, the master for
// an NPC. The higher total wins and a tie changes nothing. A won grapple makes the
// target Grappled (the condition, and combat_holds says who holds it); a won shove waits
// for its winner's choice between knocking prone and pushing 5 feet (ResolveShove); an
// escape is the grappled creature's action, against the grappler's Athletics or against
// the fixed escape DC of a grapple that came from an attack.
//
// What each side reads (RN-10, RN-20): the player reads their own roll and who won,
// never the total, the skill or the escape DC of an NPC, nor the other player's total.
//
// The contest waits without a timeout; the master may answer for anyone or close it.
// The wait is a reaction window of kind CONTEST (combat_contests_window.go).

// contestCaller reads what every contest call starts with: the caller, the idempotency
// key and the combat.
func contestCaller(ctx context.Context, campaignID, keyRaw, encounterRaw string) (m authz.Membership, key, encID string, err error) {
	if m, err = authz.RequireCampaignMember(ctx, campaignID); err != nil {
		return m, "", "", err
	}
	if key, err = parseKey(keyRaw); err != nil {
		return m, "", "", err
	}
	if encID, err = parseCombatID(encounterRaw, "encounter"); err != nil {
		return m, "", "", err
	}
	return m, key, encID, nil
}

// mustBeRunning refuses a contest call when the combat is not running.
func mustBeRunning(c *combatTx) error {
	if err := notEnded(c.enc); err != nil {
		return err
	}
	if c.enc.Status != statusActive {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ACTIVE, "the combat is not running")
	}
	return nil
}

// spendAttack spends one attack of the Attack action for a grapple or a shove: the action
// when it is the first, the next attack of the Attack action when Extra Attack leaves one
// (SRD 5.1, "Grappling": the special attack replaces one of them). A player is held to
// it; the master has the last word on an NPC and runs its attacks as he wants, as
// RollAttack does.
func spendAttack(ctx context.Context, c *combatTx, who playdb.Combatant, sheet link.Sheet, master bool) error {
	perAction := max(sheet.AttacksPerAction, 1)
	last, hasLast := lastAttack(sheet, who)
	castCantrip := hasLast && last.Spell // the action went to a cantrip: no Attack action attacks left
	turn := combat.TurnState{ActionUsed: who.ActionUsed, AttacksMade: int(who.AttacksMade)}
	if !master && (castCantrip || combat.AttacksLeft(perAction, turn) == 0) {
		if !castCantrip && who.AttacksMade > 0 && perAction > 1 {
			return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ATTACKS_USED, "the Attack action made all its attacks")
		}
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED, "the action of this turn is used")
	}
	if err := c.q.SetCombatantEconomy(ctx, playdb.SetCombatantEconomyParams{
		ID: who.ID, ActionUsed: true, BonusActionUsed: who.BonusActionUsed, ReactionUsed: who.ReactionUsed, Dashed: who.Dashed,
	}); err != nil {
		return fmt.Errorf("spend the action: %w", err)
	}
	if err := c.q.SetCombatantAttacksMade(ctx, playdb.SetCombatantAttacksMadeParams{ID: who.ID, AttacksMade: who.AttacksMade + 1}); err != nil {
		return fmt.Errorf("count the attack: %w", err)
	}
	if who.ActionAttackKey == nil {
		// The Attack action was taken (Flurry of Blows may follow); a later weapon attack
		// replaces this mark with its own.
		contestKey := combat.ContestAttackKey
		if err := c.q.SetCombatantAttackState(ctx, playdb.SetCombatantAttackStateParams{ID: who.ID, ActionAttackKey: &contestKey, BonusAttacksLeft: who.BonusAttacksLeft}); err != nil {
			return fmt.Errorf("mark the Attack action: %w", err)
		}
	}
	return nil
}

// spendAction spends the action of a combatant that tries to escape; a player with the
// action spent is refused.
func spendAction(ctx context.Context, c *combatTx, who playdb.Combatant, master bool) error {
	if who.ActionUsed && !master {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED, "the action of this turn is used")
	}
	if err := c.q.SetCombatantEconomy(ctx, playdb.SetCombatantEconomyParams{
		ID: who.ID, ActionUsed: true, BonusActionUsed: who.BonusActionUsed, ReactionUsed: who.ReactionUsed, Dashed: who.Dashed,
	}); err != nil {
		return fmt.Errorf("spend the action: %w", err)
	}
	return nil
}

// applyGrapple makes the target Grappled by the grappler, and keeps who holds it and the
// escape DC of a grapple that came from an attack.
func applyGrapple(ctx context.Context, c *combatTx, grappler, target playdb.Combatant, escapeDC *int32) error {
	if err := setConditionsOf(ctx, c, target, withCondition(target.Conditions, condGrappled)); err != nil {
		return err
	}
	if err := c.q.UpsertHold(ctx, playdb.UpsertHoldParams{
		GrappledID: target.ID, EncounterID: c.enc.ID, GrapplerID: grappler.ID, EscapeDc: escapeDC, CreatedAt: c.now,
	}); err != nil {
		return fmt.Errorf("keep the hold: %w", err)
	}
	return nil
}

// releaseHold ends a grapple: the condition and the hold go.
func releaseHold(ctx context.Context, c *combatTx, grappled playdb.Combatant) error {
	if err := setConditionsOf(ctx, c, grappled, withoutCondition(grappled.Conditions, condGrappled)); err != nil {
		return err
	}
	if err := c.q.DeleteHold(ctx, grappled.ID); err != nil {
		return fmt.Errorf("end the hold: %w", err)
	}
	return nil
}

// winnerKey is a contest's winner as the table stores it.
func winnerKey(w combat.ContestWinner) string {
	switch w {
	case combat.ContestInitiator:
		return winnerInitiator
	case combat.ContestDefender:
		return winnerDefender
	case combat.ContestTie:
	}
	return winnerTie
}

// settleContest applies what a contest decided: the grapple, the escape, or the shove that
// waits for its choice. It returns the status the contest has now and the log line (empty
// when there is none yet).
func settleContest(ctx context.Context, c *combatTx, row playdb.CombatContest, winner string, initiator, defender playdb.Combatant) (status, line string, err error) {
	won := winner == winnerInitiator
	switch row.Purpose {
	case purposeGrapple:
		if !won {
			return contestResolved, contestLineGrappleFailed, nil
		}
		return contestResolved, contestLineGrappled, applyGrapple(ctx, c, initiator, defender, row.EscapeDc)
	case purposeEscape:
		if !won {
			return contestResolved, contestLineEscapeFailed, nil
		}
		return contestResolved, contestLineEscaped, releaseHold(ctx, c, initiator)
	case purposeShove:
		if !won {
			return contestResolved, contestLineShoveFailed, nil
		}
		return contestAwaitingOutcome, "", nil
	}
	return contestResolved, "", nil
}

// StartContest implements playv1connect.ContestServiceHandler.
//
//nolint:gocognit,gocyclo // the steps of one change in one closure, as RollAttack's are; a helper would only pass the transaction around
func (s *Service) StartContest(
	ctx context.Context,
	req *connect.Request[playv1.StartContestRequest],
) (*connect.Response[playv1.StartContestResponse], error) {
	m, key, encID, err := contestCaller(ctx, req.Msg.GetCampaignId(), req.Msg.GetIdempotencyKey(), req.Msg.GetEncounterId())
	if err != nil {
		return nil, err
	}
	initiatorID, err := parseCombatID(req.Msg.GetInitiatorId(), "combatant")
	if err != nil {
		return nil, err
	}
	purpose, kind := req.Msg.GetPurpose(), req.Msg.GetKind()
	if kind == playv1.ContestKind_CONTEST_KIND_UNSPECIFIED {
		kind = playv1.ContestKind_CONTEST_KIND_CONTEST
	}
	var purposeKey string
	switch purpose {
	case playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE:
		purposeKey = purposeGrapple
	case playv1.ContestPurpose_CONTEST_PURPOSE_SHOVE:
		purposeKey = purposeShove
	case playv1.ContestPurpose_CONTEST_PURPOSE_ESCAPE:
		purposeKey = purposeEscape
	case playv1.ContestPurpose_CONTEST_PURPOSE_UNSPECIFIED:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("purpose must be GRAPPLE, SHOVE or ESCAPE"))
	}
	byAttack := kind == playv1.ContestKind_CONTEST_KIND_ESCAPE_DC
	if byAttack && (purpose != playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE || m.Role != authz.RoleMaster) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kind ESCAPE_DC is the master's grapple that came from an attack"))
	}
	escapeDC := req.Msg.GetEscapeDc()
	if byAttack && (escapeDC < 1 || escapeDC > 40) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("escape_dc must be 1 to 40"))
	}
	if !byAttack && escapeDC != 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("escape_dc is only for kind ESCAPE_DC"))
	}
	var targetID string
	if purpose != playv1.ContestPurpose_CONTEST_PURPOSE_ESCAPE {
		if targetID, err = parseCombatID(req.Msg.GetTargetId(), "combatant"); err != nil {
			return nil, err
		}
	}
	skill := req.Msg.GetSkill()
	if skill == playv1.ContestSkill_CONTEST_SKILL_UNSPECIFIED {
		skill = playv1.ContestSkill_CONTEST_SKILL_ATHLETICS
	}
	if skill == playv1.ContestSkill_CONTEST_SKILL_ACROBATICS && purpose != playv1.ContestPurpose_CONTEST_PURPOSE_ESCAPE {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a grapple or a shove is a Strength (Athletics) check"))
	}
	var in checkInput
	if !byAttack {
		if in, err = parseCheckInput(req.Msg.GetRoll()); err != nil {
			return nil, err
		}
	}

	var made actionEvent
	v := viewerOf(m)
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventContestStarted, encounterID: encID}, func(c *combatTx) (any, error) {
		made = actionEvent{}
		if err := mustBeRunning(c); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		v = c.viewer(m, cs) // the fog: an NPC the player does not see is not found
		initiator, err := findCombatant(cs, initiatorID, v)
		if err != nil {
			return nil, err
		}
		if err := v.mayAct(initiator); err != nil {
			return nil, err
		}
		open, err := c.q.ListUnansweredContestsOf(ctx, playdb.ListUnansweredContestsOfParams{EncounterID: c.enc.ID, InitiatorID: initiator.ID})
		if err != nil {
			return nil, fmt.Errorf("list the open contests: %w", err)
		}
		if len(open) > 0 {
			return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_CONTEST_OPEN, "a contest of this combatant waits already")
		}
		if err := s.mustActNow(ctx, c, initiator); err != nil {
			return nil, err
		}
		holds, err := c.q.ListHolds(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the holds: %w", err)
		}

		var defender playdb.Combatant
		row := playdb.InsertContestParams{
			EncounterID: c.enc.ID, InitiatorID: initiator.ID, Round: c.enc.Round, CreatedAt: c.now, Purpose: purposeKey, Kind: contestKindContest,
		}
		var roll *contestRoll
		switch purpose {
		case playv1.ContestPurpose_CONTEST_PURPOSE_ESCAPE:
			hold, ok := holdOn(holdsOf(holds), initiator)
			if !ok {
				return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_GRAPPLED, "the combatant is not grappled")
			}
			grappler, ok := combatantByID(cs, hold.GrapplerID)
			if !ok {
				return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_GRAPPLED, "the combatant is not grappled")
			}
			defender = grappler
			if err := spendAction(ctx, c, initiator, v.master); err != nil {
				return nil, err
			}
			if hold.EscapeDc != nil {
				row.Kind, row.EscapeDc = contestKindEscapeDC, hold.EscapeDc
			}
		default:
			if defender, err = findCombatant(cs, targetID, v); err != nil {
				return nil, err
			}
			switch {
			case defender.ID == initiator.ID:
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a combatant cannot contest itself"))
			case defender.Defeated:
				return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_DEFEATED, "the target is defeated")
			}
			if byAttack {
				// A grapple that came from an attack: the attack was rolled already, so nothing is
				// spent and nobody rolls.
				row.Kind, row.EscapeDc = contestKindEscapeDC, &escapeDC
				break
			}
			if err := s.mustBeInReach(c, v, initiator, defender); err != nil {
				return nil, err
			}
			sheet, err := s.sheetOf(ctx, c.tx, m.CampaignID, initiator)
			if err != nil {
				return nil, err
			}
			if err := mayAttack(initiator, false); err != nil {
				return nil, err
			}
			if err := spendAttack(ctx, c, initiator, sheet, v.master); err != nil {
				return nil, err
			}
			// A grapple or a shove is an attack on the target: it keeps a rage going (SRD 5.1,
			// Rage: the rage ends if the turn ends without having "attacked a hostile creature")
			// and gives a hider's position away ("Unseen Attackers and Targets").
			if oppositeSide(initiator, defender) {
				states, err := s.readStates(ctx, c.tx, c.enc.ID)
				if err != nil {
					return nil, err
				}
				if hasState(states, initiator.ID, stateRage) && (!initiator.AttackedHostile || initiator.RageEndPending) {
					if err := c.q.SetCombatantRageFlags(ctx, playdb.SetCombatantRageFlagsParams{ID: initiator.ID, AttackedHostile: true, TookDamage: initiator.TookDamage}); err != nil {
						return nil, fmt.Errorf("note the attack on a hostile creature: %w", err)
					}
				}
			}
			if _, err := s.endHiding(ctx, c, initiator); err != nil {
				return nil, err
			}
		}
		row.DefenderID = defender.ID

		if !byAttack {
			if !v.master {
				if err := s.mustRollThisWay(ctx, c.tx, m, in.rollInput()); err != nil {
					return nil, err
				}
			}
			r, err := s.rollFor(ctx, c, initiator, skillKeyOf(skill), in, cs)
			if err != nil {
				return nil, err
			}
			r.ByMaster = v.master && !initiatorIsNPC(initiator)
			roll = &r
			if row.InitiatorRoll, err = encodeRoll(r); err != nil {
				return nil, err
			}
		}

		// Who won, when nothing waits: a grapple that came from an attack, and an escape
		// against a fixed DC.
		row.Status = contestAwaitingDefender
		var winner string
		switch {
		case byAttack:
			winner = winnerInitiator
		case row.Kind == contestKindEscapeDC:
			winner = winnerDefender
			if combat.MeetsEscapeDC(int(roll.Total), int(*row.EscapeDc)) {
				winner = winnerInitiator
			}
		}
		line := ""
		if winner != "" {
			row.Winner = &winner
			row.ResolvedAt = &c.now
		}
		inserted, err := c.q.InsertContest(ctx, row)
		if err != nil {
			return nil, fmt.Errorf("open the contest: %w", err)
		}
		if winner != "" {
			status, l, err := settleContest(ctx, c, inserted, winner, initiator, defender)
			if err != nil {
				return nil, err
			}
			line = l
			if status != contestResolved {
				// A fixed DC settles a grapple or an escape: nothing waits.
				status = contestResolved
			}
			if inserted, err = c.q.SetContestState(ctx, playdb.SetContestStateParams{EncounterID: c.enc.ID, ID: inserted.ID, Status: status, ShoveOutcome: nil, ResolvedAt: &c.now}); err != nil {
				return nil, fmt.Errorf("settle the contest: %w", err)
			}
		}
		if err := touch(ctx, c); err != nil {
			return nil, err
		}
		c.characterID = &initiator.CharacterID
		made = actionEvent{
			Round: c.enc.Round, Secret: secretOf(initiator, defender), Actor: initiator.ID, Target: defender.ID,
			Contest: &contestEvent{ContestID: inserted.ID, Purpose: purposeKey, Line: line, Waiting: winner == ""},
		}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "start a contest", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the contest", err)
	}
	return contestResponse(ctx, s, m, res, ev, func(out *playv1.Encounter, view *playv1.ContestView) *playv1.StartContestResponse {
		return &playv1.StartContestResponse{Encounter: out, Contest: view}
	})
}

// initiatorIsNPC says the combatant is not a player's.
func initiatorIsNPC(c playdb.Combatant) bool { return c.UserID == nil }

// mustBeInReach checks, for a player, that the target is a size the combatant may grapple
// or shove and within its melee reach (SRD 5.1, "Grappling"). The master has the last
// word. Without a map nobody has a square and the master judges the reach.
func (s *Service) mustBeInReach(c *combatTx, v combatViewer, who, target playdb.Combatant) error {
	if v.master {
		return nil
	}
	if !combat.CanGrappleOrShove(who.Size, target.Size) {
		return errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_TARGET_TOO_BIG, "the target is more than one size larger")
	}
	if isTheatre(c.enc) {
		return nil
	}
	if !placed(who) || !placed(target) {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_PLACED, "the combatants must be on the map")
	}
	if dist, _ := distanceFt(who, target); dist > combat.MeleeReachFt {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH, "the target is beyond your reach",
			func(b *playv1.EncounterBlocked) { b.MissingFt = dist - combat.MeleeReachFt })
	}
	return nil
}

// RespondContest implements playv1connect.ContestServiceHandler.
//
//nolint:gocognit,gocyclo // the steps of one change in one closure, as RollAttack's are; a helper would only pass the transaction around
func (s *Service) RespondContest(
	ctx context.Context,
	req *connect.Request[playv1.RespondContestRequest],
) (*connect.Response[playv1.RespondContestResponse], error) {
	m, key, encID, err := contestCaller(ctx, req.Msg.GetCampaignId(), req.Msg.GetIdempotencyKey(), req.Msg.GetEncounterId())
	if err != nil {
		return nil, err
	}
	contestID, err := parseCombatID(req.Msg.GetContestId(), "contest")
	if err != nil {
		return nil, err
	}
	skill := req.Msg.GetSkill()
	if skill == playv1.ContestSkill_CONTEST_SKILL_UNSPECIFIED {
		skill = playv1.ContestSkill_CONTEST_SKILL_ATHLETICS
	}
	leave := req.Msg.GetDeferToMaster()
	var in checkInput
	if !leave {
		if in, err = parseCheckInput(req.Msg.GetRoll()); err != nil {
			return nil, err
		}
	} else if m.Role == authz.RoleMaster {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("the master rolls himself: leaveto_master is the player's"))
	}

	var made actionEvent
	kind := eventContestResolved
	if leave {
		kind = eventContestDeferred
	}
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: kind, encounterID: encID}, func(c *combatTx) (any, error) {
		made = actionEvent{}
		if err := mustBeRunning(c); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		v := c.viewer(m, cs)
		row, err := c.q.GetContest(ctx, playdb.GetContestParams{EncounterID: c.enc.ID, ID: contestID})
		if err != nil {
			return nil, contestNotFound(err)
		}
		initiator, okI := combatantByID(cs, row.InitiatorID)
		defender, okD := combatantByID(cs, row.DefenderID)
		if !okI || !okD {
			return nil, errNotAwaiting()
		}
		// A player who is not in the contest does not know it exists.
		if !v.master && !v.owns(defender) {
			if !v.owns(initiator) {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("contest not found"))
			}
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only the one that answers or the master may do this"))
		}
		if row.Status != contestAwaitingDefender || row.Kind != contestKindContest {
			return nil, errNotAwaiting()
		}
		previous, err := decodeRoll(row.DefenderRoll)
		if err != nil {
			return nil, err
		}
		deferred := previous != nil && previous.Deferred
		if deferred && !v.master {
			return nil, errNotAwaiting() // it waits for the master now
		}
		// An escape is contested by the grappler's Strength (Athletics) check.
		skillKey := skillKeyOf(skill)
		switch {
		case row.Purpose == purposeEscape:
			skillKey = skillAthletics
		case deferred:
			skillKey = previous.Skill // the player's choice
		}

		if leave {
			stored, err := encodeRoll(contestRoll{Skill: skillKey, Deferred: true})
			if err != nil {
				return nil, err
			}
			if _, err := c.q.SetContestAnswered(ctx, playdb.SetContestAnsweredParams{
				EncounterID: c.enc.ID, ID: row.ID, DefenderRoll: stored, Winner: nil, Status: contestAwaitingDefender, ResolvedAt: nil,
			}); err != nil {
				return nil, fmt.Errorf("leave the roll to the master: %w", err)
			}
		} else {
			if !v.master {
				if err := s.mustRollThisWay(ctx, c.tx, m, in.rollInput()); err != nil {
					return nil, err
				}
			}
			roll, err := s.rollFor(ctx, c, defender, skillKey, in, cs)
			if err != nil {
				return nil, err
			}
			roll.ByMaster = v.master && !initiatorIsNPC(defender)
			stored, err := encodeRoll(roll)
			if err != nil {
				return nil, err
			}
			iRoll, err := decodeRoll(row.InitiatorRoll)
			if err != nil || iRoll == nil {
				return nil, fmt.Errorf("read the initiator's roll of contest %s: %w", row.ID, err)
			}
			winner := winnerKey(combat.CompareContest(int(iRoll.Total), int(roll.Total)))
			status, line, err := settleContest(ctx, c, row, winner, initiator, defender)
			if err != nil {
				return nil, err
			}
			resolvedAt := &c.now
			if status == contestAwaitingOutcome {
				resolvedAt = nil
			}
			if _, err := c.q.SetContestAnswered(ctx, playdb.SetContestAnsweredParams{
				EncounterID: c.enc.ID, ID: row.ID, DefenderRoll: stored, Winner: &winner, Status: status, ResolvedAt: resolvedAt,
			}); err != nil {
				return nil, fmt.Errorf("answer the contest: %w", err)
			}
			made.Contest = &contestEvent{Line: line, Waiting: status == contestAwaitingOutcome}
		}
		if err := touch(ctx, c); err != nil {
			return nil, err
		}
		c.characterID = &defender.CharacterID
		contestPart := made.Contest
		if contestPart == nil {
			contestPart = &contestEvent{Waiting: true}
		}
		contestPart.ContestID, contestPart.Purpose = row.ID, row.Purpose
		made = actionEvent{
			Round: c.enc.Round, Secret: secretOf(initiator, defender), Actor: initiator.ID, Target: defender.ID, Contest: contestPart,
		}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "answer a contest", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the contest", err)
	}
	return contestResponse(ctx, s, m, res, ev, func(out *playv1.Encounter, view *playv1.ContestView) *playv1.RespondContestResponse {
		return &playv1.RespondContestResponse{Encounter: out, Contest: view}
	})
}

// contestNotFound is a contest that is not in the combat.
func contestNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return connect.NewError(connect.CodeNotFound, errors.New("contest not found"))
	}
	return fmt.Errorf("find the contest: %w", err)
}

// pushPlan is where a shove's push would send the target, and whether the square is free.
type pushPlan struct {
	// has says there is a square to push to (a map, both on it).
	has bool
	to  grid.Square
	// blocked says the square is not free as the viewer knows the map and the creatures;
	// why says by what.
	blocked bool
	why     playv1.ShoveBlockedReason
	// reallyBlocked says it is not free at all, whatever the viewer knows.
	reallyBlocked bool
}

// pushPlanOf works out the push of a shove from the shover to the target: the square 5
// feet straight away from the shover, free when it is on the grid, not a wall or a column
// and not held by another creature. A player is told only what they know of the map and
// see of the creatures (RN-10): a push that really is blocked by something they do not
// see is offered and does not move the target.
func (s *Service) pushPlanOf(terrain, plan grid.Terrain, enc playdb.Encounter, cs []playdb.Combatant, shover, target playdb.Combatant, v combatViewer) pushPlan {
	if isTheatre(enc) || !placed(shover) || !placed(target) {
		return pushPlan{}
	}
	col, row := combat.PushStep(int(*shover.GridCol), int(*shover.GridRow), int(*target.GridCol), int(*target.GridRow))
	to := grid.Square{Col: col, Row: row}
	out := pushPlan{has: true, to: to}
	from := squareOfCombatant(target)
	mover := moverOf(target)
	wallIn := func(t grid.Terrain) bool { return t.Move(from, to, grid.OccupantMap{}, mover).Blocked }
	creatureAt := func(visible bool) bool {
		for _, o := range cs {
			if o.ID == target.ID || o.Defeated || !placed(o) || squareOfCombatant(o) != to {
				continue
			}
			if !visible || v.sees(o) {
				return true
			}
		}
		return false
	}
	switch {
	case wallIn(plan):
		out.blocked, out.why = true, playv1.ShoveBlockedReason_SHOVE_BLOCKED_REASON_WALL
	case creatureAt(true):
		out.blocked, out.why = true, playv1.ShoveBlockedReason_SHOVE_BLOCKED_REASON_CREATURE
	}
	out.reallyBlocked = out.blocked || wallIn(terrain) || creatureAt(false)
	return out
}

// ResolveShove implements playv1connect.ContestServiceHandler.
//
//nolint:gocognit,gocyclo // the steps of one change in one closure, as RollAttack's are; a helper would only pass the transaction around
func (s *Service) ResolveShove(
	ctx context.Context,
	req *connect.Request[playv1.ResolveShoveRequest],
) (*connect.Response[playv1.ResolveShoveResponse], error) {
	m, key, encID, err := contestCaller(ctx, req.Msg.GetCampaignId(), req.Msg.GetIdempotencyKey(), req.Msg.GetEncounterId())
	if err != nil {
		return nil, err
	}
	contestID, err := parseCombatID(req.Msg.GetContestId(), "contest")
	if err != nil {
		return nil, err
	}
	outcome := req.Msg.GetOutcome()
	if outcome == playv1.ShoveOutcome_SHOVE_OUTCOME_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("outcome must be PRONE, PUSH or STAYS"))
	}

	var made actionEvent
	v := viewerOf(m)
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventShoveResolved, encounterID: encID}, func(c *combatTx) (any, error) {
		made = actionEvent{}
		if err := mustBeRunning(c); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		v = c.viewer(m, cs)
		row, err := c.q.GetContest(ctx, playdb.GetContestParams{EncounterID: c.enc.ID, ID: contestID})
		if err != nil {
			return nil, contestNotFound(err)
		}
		shover, okI := combatantByID(cs, row.InitiatorID)
		target, okD := combatantByID(cs, row.DefenderID)
		if !okI || !okD {
			return nil, errNotAwaiting()
		}
		if !v.master && !v.owns(shover) {
			if !v.owns(target) {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("contest not found"))
			}
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only the one that won the shove or the master may do this"))
		}
		if row.Status != contestAwaitingOutcome || row.Purpose != purposeShove {
			return nil, errNotAwaiting()
		}
		terrain, err := s.terrainOf(ctx, c.tx, m.CampaignID, c.enc)
		if err != nil {
			return nil, err
		}
		known, err := c.sight.knownTerrain(ctx, c.tx, v)
		if err != nil {
			return nil, err
		}
		push := s.pushPlanOf(terrain, planOn(v, terrain, known), c.enc, cs, shover, target, v)

		var stored, line string
		switch outcome {
		case playv1.ShoveOutcome_SHOVE_OUTCOME_PRONE:
			stored, line = shoveProne, contestLineShoveProne
			if err := setConditionsOf(ctx, c, target, withCondition(target.Conditions, condProne)); err != nil {
				return nil, err
			}
		case playv1.ShoveOutcome_SHOVE_OUTCOME_PUSH:
			if push.blocked {
				return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_PUSH_BLOCKED, "the square behind the target is blocked")
			}
			stored, line = shovePush, contestLineShovePushed
			if push.has && push.reallyBlocked {
				// Something the shover does not see holds the square: the target does not leave its place.
				stored, line = shoveStays, contestLineShoveStays
			} else if push.has {
				if err := c.q.SetCombatantPlace(ctx, playdb.SetCombatantPlaceParams{
					ID: target.ID, GridCol: new(clamp32(push.to.Col, 0, math.MaxInt32)), GridRow: new(clamp32(push.to.Row, 0, math.MaxInt32)), CoverMark: "none",
				}); err != nil {
					return nil, fmt.Errorf("push the target: %w", err)
				}
			}
		case playv1.ShoveOutcome_SHOVE_OUTCOME_STAYS:
			if !push.has || !push.reallyBlocked {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("outcome STAYS is for a push that is blocked"))
			}
			stored, line = shoveStays, contestLineShoveStays
		}
		if _, err := c.q.SetContestState(ctx, playdb.SetContestStateParams{EncounterID: c.enc.ID, ID: row.ID, Status: contestResolved, ShoveOutcome: &stored, ResolvedAt: &c.now}); err != nil {
			return nil, fmt.Errorf("settle the shove: %w", err)
		}
		if err := touch(ctx, c); err != nil {
			return nil, err
		}
		c.characterID = &shover.CharacterID
		made = actionEvent{
			Round: c.enc.Round, Secret: secretOf(shover, target), Actor: shover.ID, Target: target.ID,
			Contest: &contestEvent{ContestID: row.ID, Purpose: purposeShove, Line: line},
		}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "resolve a shove", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the shove", err)
	}
	return contestResponse(ctx, s, m, res, ev, func(out *playv1.Encounter, view *playv1.ContestView) *playv1.ResolveShoveResponse {
		return &playv1.ResolveShoveResponse{Encounter: out, Contest: view}
	})
}

// CloseContest implements playv1connect.ContestServiceHandler.
func (s *Service) CloseContest(
	ctx context.Context,
	req *connect.Request[playv1.CloseContestRequest],
) (*connect.Response[playv1.CloseContestResponse], error) {
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
	contestID, err := parseCombatID(req.Msg.GetContestId(), "contest")
	if err != nil {
		return nil, err
	}
	var made actionEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventContestClosed, encounterID: encID}, func(c *combatTx) (any, error) {
		made = actionEvent{}
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		row, err := c.q.GetContest(ctx, playdb.GetContestParams{EncounterID: c.enc.ID, ID: contestID})
		if err != nil {
			return nil, contestNotFound(err)
		}
		if row.Status != contestAwaitingDefender && row.Status != contestAwaitingOutcome {
			return nil, errNotAwaiting()
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		initiator, _ := combatantByID(cs, row.InitiatorID)
		defender, _ := combatantByID(cs, row.DefenderID)
		if _, err := c.q.SetContestState(ctx, playdb.SetContestStateParams{EncounterID: c.enc.ID, ID: row.ID, Status: contestClosed, ShoveOutcome: row.ShoveOutcome, ResolvedAt: &c.now}); err != nil {
			return nil, fmt.Errorf("close the contest: %w", err)
		}
		if err := touch(ctx, c); err != nil {
			return nil, err
		}
		made = actionEvent{
			Round: c.enc.Round, Secret: secretOf(initiator, defender), Actor: initiator.ID, Target: defender.ID,
			Contest: &contestEvent{ContestID: row.ID, Purpose: row.Purpose, Line: contestLineClosed},
		}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "close a contest", err)
	}
	out, err := s.finish(ctx, m, res, s.changed(m.CampaignID))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.CloseContestResponse{Encounter: out}), nil
}

// ReleaseGrapple implements playv1connect.ContestServiceHandler.
func (s *Service) ReleaseGrapple(
	ctx context.Context,
	req *connect.Request[playv1.ReleaseGrappleRequest],
) (*connect.Response[playv1.ReleaseGrappleResponse], error) {
	m, key, encID, err := contestCaller(ctx, req.Msg.GetCampaignId(), req.Msg.GetIdempotencyKey(), req.Msg.GetEncounterId())
	if err != nil {
		return nil, err
	}
	grappledID, err := parseCombatID(req.Msg.GetGrappledId(), "combatant")
	if err != nil {
		return nil, err
	}
	var made actionEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventGrappleReleased, encounterID: encID}, func(c *combatTx) (any, error) {
		made = actionEvent{}
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		v := c.viewer(m, cs)
		grappled, err := findCombatant(cs, grappledID, v)
		if err != nil {
			return nil, err
		}
		holds, err := c.q.ListHolds(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the holds: %w", err)
		}
		hold, ok := holdOn(holdsOf(holds), grappled)
		if !ok {
			return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_GRAPPLED, "the combatant is not grappled")
		}
		grappler, ok := combatantByID(cs, hold.GrapplerID)
		if !ok {
			return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_GRAPPLED, "the combatant is not grappled")
		}
		// The grappler lets go whenever it likes; no action is needed (SRD 5.1, "Grappling").
		if err := v.mayAct(grappler); err != nil {
			return nil, err
		}
		if err := releaseHold(ctx, c, grappled); err != nil {
			return nil, err
		}
		if err := touch(ctx, c); err != nil {
			return nil, err
		}
		c.characterID = &grappler.CharacterID
		made = actionEvent{
			Round: c.enc.Round, Secret: secretOf(grappler, grappled), Actor: grappler.ID, Target: grappled.ID,
			Contest: &contestEvent{Purpose: purposeGrapple, Line: contestLineReleased},
		}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "release a grapple", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the release", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChangedFor(ctx, m.CampaignID, d, ev.Actor, ev.Target)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, !ev.Secret)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.ReleaseGrappleResponse{Encounter: out}), nil
}

// pruneHolds ends the grapples that no longer hold, after any change of a combat (SRD 5.1,
// Conditions, Grappled): the condition ends when the grappler is incapacitated (or out of the
// fight), or when an effect removes the grappled creature from the grappler's reach (a push, a
// forced move, a teleport); and when the master took the condition off, the hold goes with it.
func (s *Service) pruneHolds(ctx context.Context, c *combatTx) error {
	if c.enc.ID == "" {
		return nil
	}
	holds, err := c.q.ListHolds(ctx, c.enc.ID)
	if err != nil || len(holds) == 0 {
		return err
	}
	cs, err := c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	by := combatantsByID(cs)
	for _, h := range holds {
		grappled, okG := by[h.GrappledID]
		grappler, okR := by[h.GrapplerID]
		ends := !okG || !okR || !isGrappled(grappled) || grappler.Defeated || grappled.Defeated ||
			slices.ContainsFunc(grappler.Conditions, func(k string) bool { return slices.Contains(incapacitating, k) })
		if !ends && (grappler.Kind == kindPlayer || grappled.Kind == kindPlayer) {
			// A player's character at 0 hit points, dead or not yet revived holds nobody and
			// is held by nobody (SRD 5.1, "Grappled": the grappler is incapacitated).
			for _, who := range []playdb.Combatant{grappler, grappled} {
				if who.Kind != kindPlayer {
					continue
				}
				down, err := s.isDown(ctx, c.tx, c.session.CampaignID, who)
				if err != nil {
					return err
				}
				ends = ends || down
			}
		}
		if !ends && !isTheatre(c.enc) && placed(grappled) && placed(grappler) {
			if dist, _ := distanceFt(grappler, grappled); dist > combat.MeleeReachFt {
				ends = true
			}
		}
		if !ends {
			continue
		}
		if okG && isGrappled(grappled) {
			if err := releaseHold(ctx, c, grappled); err != nil {
				return err
			}
			continue
		}
		if err := c.q.DeleteHold(ctx, h.GrappledID); err != nil {
			return fmt.Errorf("end the hold: %w", err)
		}
	}
	return nil
}
