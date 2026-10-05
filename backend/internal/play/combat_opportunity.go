package play

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Opportunity attacks (MR-034, RN-21, question 69; Etapa 9, slice 9.6b). A move
// on foot that leaves the reach of a hostile combatant lands at once, and that
// combatant is offered one attack at the mover (an opportunity_offers row): its
// controller attacks or declines, or the master skips it (the player is
// offline). The mover's turn waits for the answers. The geometry is
// rules/grid's (LeavesReach); this file decides who reacts and keeps the offers.

// The states of an offer (opportunity_offers_state_valid).
const (
	offerPending  = "pending"
	offerAttacked = "attacked"
	offerDeclined = "declined"
	offerSkipped  = "skipped"
)

// cantReact are the conditions that take a creature's reaction away (SRD:
// incapacitated, and what includes it) or its sight of the mover (blinded).
var cantReact = []string{
	"condition:incapacitated", "condition:paralyzed", "condition:petrified", "condition:stunned", "condition:unconscious", "condition:blinded",
}

// candidate is a combatant that could make an opportunity attack, with the
// reach it has.
type candidate struct {
	who     playdb.Combatant
	reachFt int
}

// provoker is a candidate that a move leaves behind, and the square of the line
// where the mover left its reach.
type provoker struct {
	reactor playdb.Combatant
	left    grid.Square
}

// meleeReachOf is how far a sheet's melee attacks reach: the largest of their reaches
// (a sheet that gives none reaches 5 ft). An attack with a long range is a thrown
// weapon, whose range is not a reach.
func meleeReachOf(sheet link.Sheet) int {
	reach, found := meleeReachFt, false
	for _, a := range sheet.Attacks {
		if !meleeAttack(a) {
			continue
		}
		found = true
		if a.LongRangeFt == 0 {
			reach = max(reach, a.RangeFt)
		}
	}
	if !found {
		return 0
	}
	return reach
}

// opportunityReactors lists the combatants that could make an opportunity attack
// on the mover if it left their reach. exact is the server's own list (who is
// really offered the attack): hostile to the mover (the other side), placed, not
// defeated, awake (no incapacitating condition, not down), with the reaction
// unused, able to attack with it (a creature that may not, may not) and holding
// a melee attack, whose longest reach is the reach. They all see the mover unless
// it is hidden (until slice 9.7, "sees" is that), and a Blinded one does not. A
// reactor in the mover's own turn group may react too (the SRD allows reactions
// on any turn). Not exact is a player's warning in GetMoveOptions: only what the
// viewer sees (RN-20), so hostile, not defeated, visible, its visible conditions,
// and a 5 ft reach, never the reaction, the stat block or the vitals of an NPC.
// tx is the open transaction, or nil for a read.
func (s *Service) opportunityReactors(ctx context.Context, tx pgx.Tx, campaignID string, cs []playdb.Combatant, mover playdb.Combatant, v *combatViewer) ([]candidate, error) {
	if mover.Hidden || mover.Disengaged || !placed(mover) {
		return nil, nil
	}
	exact := v == nil || v.master
	var out []candidate
	for _, r := range cs {
		if r.ID == mover.ID || r.Defeated || !placed(r) || r.Side == mover.Side {
			continue
		}
		if v != nil && !v.sees(r) {
			continue
		}
		if slices.ContainsFunc(r.Conditions, func(k string) bool { return slices.Contains(cantReact, k) }) {
			continue
		}
		if !exact {
			out = append(out, candidate{who: r, reachFt: meleeReachFt})
			continue
		}
		if r.ReactionUsed || mayAttack(r, true) != nil {
			continue
		}
		down, err := s.isDown(ctx, tx, campaignID, r)
		if err != nil {
			return nil, err
		}
		if down {
			continue
		}
		sheet, err := s.sheetOf(ctx, tx, campaignID, r)
		if connect.CodeOf(err) == connect.CodeNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		if reach := meleeReachOf(sheet); reach > 0 {
			out = append(out, candidate{who: r, reachFt: reach})
		}
	}
	return out, nil
}

// meleeAttack says an attack can be an opportunity attack: a melee weapon, rolled
// against the armor class.
func meleeAttack(a link.Attack) bool { return a.Melee && !a.Save }

// provokedBy filters the candidates down to the ones whose reach the straight
// move from one square to another leaves.
func provokedBy(cands []candidate, from, to grid.Square) []provoker {
	var out []provoker
	for _, r := range cands {
		if re := grid.LeavesReach(from, to, squareOfCombatant(r.who), r.reachFt); re.Leaves {
			out = append(out, provoker{reactor: r.who, left: re.LastInReach})
		}
	}
	return out
}

// offerOpportunities makes the offers a move that landed provokes, inside the
// move's transaction, and writes an opportunity_offered event for each (IDs and
// the square only). It returns the id the offers share, empty when there is none.
// The event is written before the move's own, which is what the undo acts on.
func (s *Service) offerOpportunities(ctx context.Context, c *combatTx, cs []playdb.Combatant, mover playdb.Combatant, from, to grid.Square) (string, error) {
	reactors, err := s.opportunityReactors(ctx, c.tx, c.session.CampaignID, cs, mover, nil)
	if err != nil {
		return "", err
	}
	provokers := provokedBy(reactors, from, to)
	if len(provokers) == 0 {
		return "", nil
	}
	moveID := uuid.New().String()
	for _, p := range provokers {
		offer, err := c.q.InsertOpportunityOffer(ctx, playdb.InsertOpportunityOfferParams{
			EncounterID: c.enc.ID, MoveID: moveID, MoverID: mover.ID, ReactorID: p.reactor.ID,
			LeftCol: clamp32(p.left.Col, 0, math.MaxInt32), LeftRow: clamp32(p.left.Row, 0, math.MaxInt32), CreatedAt: c.now,
		})
		if err != nil {
			return "", fmt.Errorf("offer the opportunity attack: %w", err)
		}
		if err := insertEvent(ctx, c, eventOpportunityOffered, &c.actorUserID, nil, actionEvent{
			Round: c.enc.Round, Secret: mover.Hidden || p.reactor.Hidden, Actor: mover.ID, Target: p.reactor.ID,
			OfferID: offer.ID, MoveID: moveID, Col: offer.LeftCol, Row: offer.LeftRow,
		}); err != nil {
			return "", err
		}
	}
	return moveID, nil
}

// mustNotWait refuses, for a player, what the turn cannot do while the mover
// waits: an offer on it nobody answered, or the damage of the attack an offer
// was answered with, still to roll, apply or discard. The wait holds the mover
// and the same player's other combatants in its turn group (a creature of the
// character, RN-21: "o jogador não age"). The master is never stopped: he can
// skip the offer, or act anyway.
func (s *Service) mustNotWait(ctx context.Context, c *combatTx, who playdb.Combatant) error {
	if c.master {
		return nil
	}
	offers, err := c.q.ListWaitingOpportunityOffers(ctx, c.enc.ID)
	if err != nil || len(offers) == 0 {
		if err != nil {
			return fmt.Errorf("list the opportunity offers: %w", err)
		}
		return nil
	}
	cs, err := c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	for _, o := range offers {
		i := slices.IndexFunc(cs, func(m playdb.Combatant) bool { return m.ID == o.MoverID })
		if i < 0 {
			continue
		}
		mover := cs[i]
		mine := mover.ID == who.ID || (inTurn(c.enc, who) && inTurn(c.enc, mover) && who.UserID != nil && mover.UserID != nil && *who.UserID == *mover.UserID)
		if mine {
			return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_OPPORTUNITY_PENDING, "an opportunity attack waits for its answer")
		}
	}
	return nil
}

// pruneOffers skips the pending offers nobody can answer any more, after any
// change: the mover is defeated, or the reactor is gone, defeated, down,
// incapacitated or has no reaction left (spent on another offer, or on anything).
// A mover is never held by an offer nobody can answer. (A mover or a reactor
// that left the combat takes its offers with it: the foreign keys cascade.)
func (s *Service) pruneOffers(ctx context.Context, c *combatTx) error {
	if c.enc.ID == "" {
		return nil
	}
	pending, err := c.q.ListPendingOpportunityOffers(ctx, c.enc.ID)
	if err != nil || len(pending) == 0 {
		if err != nil {
			return fmt.Errorf("list the opportunity offers: %w", err)
		}
		return nil
	}
	cs, err := c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	byID := make(map[string]playdb.Combatant, len(cs))
	for _, m := range cs {
		byID[m.ID] = m
	}
	for _, o := range pending {
		mover, okM := byID[o.MoverID]
		r, okR := byID[o.ReactorID]
		dead := !okM || !okR || mover.Defeated || r.Defeated || r.ReactionUsed ||
			slices.ContainsFunc(r.Conditions, func(k string) bool { return slices.Contains(cantReact, k) })
		if !dead {
			down, err := s.isDown(ctx, c.tx, c.session.CampaignID, r)
			if err != nil {
				return err
			}
			dead = down
		}
		if dead {
			if _, err := c.q.SetOpportunityOfferState(ctx, playdb.SetOpportunityOfferStateParams{ID: o.ID, State: offerSkipped, AnsweredAt: &c.now}); err != nil {
				return fmt.Errorf("skip an offer nobody can answer: %w", err)
			}
		}
	}
	return nil
}

// parseOfferID reads an offer's id: one that is not a UUID is invalid_argument.
func parseOfferID(raw string) (string, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("opportunity_offer_id must be a UUID"))
	}
	return id.String(), nil
}

// errOfferGone is the aborted of an offer that is not there (the move was
// undone, or the turn passed) or was answered already: the screen reads the
// combat again.
func errOfferGone() error {
	return connect.NewError(connect.CodeAborted, errors.New("that opportunity offer was answered or is gone"))
}

// pendingOffer finds the offer inside the change's transaction: aborted when it
// is not there or not pending.
func pendingOffer(ctx context.Context, c *combatTx, id string) (playdb.OpportunityOffer, error) {
	o, err := c.q.GetOpportunityOffer(ctx, playdb.GetOpportunityOfferParams{EncounterID: c.enc.ID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return o, errOfferGone()
	}
	if err != nil {
		return o, fmt.Errorf("find the opportunity offer: %w", err)
	}
	if o.State != offerPending {
		return o, errOfferGone()
	}
	return o, nil
}

// DeclineOpportunity implements playv1connect.CombatServiceHandler.
func (s *Service) DeclineOpportunity(
	ctx context.Context,
	req *connect.Request[playv1.DeclineOpportunityRequest],
) (*connect.Response[playv1.DeclineOpportunityResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	out, err := s.answerOffer(ctx, m, req.Msg.GetEncounterId(), req.Msg.GetOpportunityOfferId(), req.Msg.GetIdempotencyKey(), false)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.DeclineOpportunityResponse{Encounter: out}), nil
}

// SkipOpportunity implements playv1connect.CombatServiceHandler.
func (s *Service) SkipOpportunity(
	ctx context.Context,
	req *connect.Request[playv1.SkipOpportunityRequest],
) (*connect.Response[playv1.SkipOpportunityResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	out, err := s.answerOffer(ctx, m, req.Msg.GetEncounterId(), req.Msg.GetOpportunityOfferId(), req.Msg.GetIdempotencyKey(), true)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.SkipOpportunityResponse{Encounter: out}), nil
}

// answerOffer ends an offer without an attack: declined by its reactor's
// controller (or the master), or skipped by the master. It writes a
// reaction_declined event that the undo can take back.
func (s *Service) answerOffer(ctx context.Context, m authz.Membership, rawEncounter, rawOffer, rawKey string, skip bool) (*playv1.Encounter, error) {
	key, err := parseKey(rawKey)
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(rawEncounter, "encounter")
	if err != nil {
		return nil, err
	}
	offerID, err := parseOfferID(rawOffer)
	if err != nil {
		return nil, err
	}
	v := viewerOf(m)
	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventReactionDeclined, encounterID: encID}, func(c *combatTx) (any, error) {
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		o, err := c.q.GetOpportunityOffer(ctx, playdb.GetOpportunityOfferParams{EncounterID: c.enc.ID, ID: offerID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errOfferGone()
		}
		if err != nil {
			return nil, fmt.Errorf("find the opportunity offer: %w", err)
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		reactor, err := findCombatant(cs, o.ReactorID, v) // not_found for a reactor the player does not see
		if err != nil {
			return nil, err
		}
		if err := v.mayAct(reactor); err != nil {
			return nil, err
		}
		if o.State != offerPending {
			return nil, errOfferGone()
		}
		state := offerDeclined
		if skip {
			state = offerSkipped
		}
		if _, err := c.q.SetOpportunityOfferState(ctx, playdb.SetOpportunityOfferStateParams{ID: o.ID, State: state, AnsweredAt: &c.now}); err != nil {
			return nil, fmt.Errorf("answer the opportunity offer: %w", err)
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		c.characterID = &reactor.CharacterID
		mover, _ := findCombatant(cs, o.MoverID, combatViewer{master: true})
		return actionEvent{Round: c.enc.Round, Secret: secretOf(reactor, mover), Actor: reactor.ID, Target: o.MoverID, OfferID: o.ID, Skipped: skip}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "answer an opportunity offer", err)
	}
	return s.finish(ctx, m, res, s.changed(m.CampaignID))
}

// opportunityOffers lists the pending offers as the caller may see them (RN-10,
// RN-20): the master all, with names; a player the offers of the reactors they
// control (to answer) and the ones on a combatant of theirs, the reactor named
// only when they see it. Nobody else gets any.
func (s *Service) opportunityOffers(ctx context.Context, m authz.Membership, d *encounterData) ([]*playv1.OpportunityOffer, error) {
	if d.enc.Status != statusActive {
		return nil, nil
	}
	pending, err := s.queries.ListPendingOpportunityOffers(ctx, d.enc.ID)
	if err != nil {
		return nil, s.dbError(ctx, "list the opportunity offers", err)
	}
	v := viewerOf(m)
	find := func(id string) (playdb.Combatant, bool) {
		i := slices.IndexFunc(d.cs, func(c playdb.Combatant) bool { return c.ID == id })
		if i < 0 {
			return playdb.Combatant{}, false
		}
		return d.cs[i], true
	}
	var out []*playv1.OpportunityOffer
	for _, o := range pending {
		mover, ok1 := find(o.MoverID)
		reactor, ok2 := find(o.ReactorID)
		if !ok1 || !ok2 || !v.sees(mover) {
			continue
		}
		answers := v.master || v.owns(reactor)
		if !answers && !v.owns(mover) {
			continue
		}
		offer := &playv1.OpportunityOffer{Id: o.ID, MoverId: mover.ID, MoverLabel: mover.Label, ForYou: answers}
		if v.sees(reactor) {
			offer.ReactorId, offer.ReactorLabel = reactor.ID, reactor.Label
		}
		if answers {
			offer.LeftCol, offer.LeftRow = o.LeftCol, o.LeftRow
			if sheet, err := s.sheetOf(ctx, nil, m.CampaignID, reactor); err == nil {
				for _, a := range sheet.Attacks {
					if meleeAttack(a) {
						offer.Attacks = append(offer.Attacks, &playv1.OpportunityAttack{Key: a.Key, NamePt: a.Name})
					}
				}
			}
		}
		out = append(out, offer)
	}
	return out, nil
}

// markProvokes fills, on each square a move can reach, the reactors it would let
// attack, for the squares the caller's screen warns about. Only reactors the
// viewer sees (RN-10): a hidden one is never named. Nothing off the mover's turn
// (a placement offers nothing) and nothing after Disengage.
func (s *Service) markProvokes(ctx context.Context, m authz.Membership, enc playdb.Encounter, cs []playdb.Combatant, who playdb.Combatant, out *playv1.GetMoveOptionsResponse) error {
	if !actsNow(enc, who) || len(out.Reachable) == 0 {
		return nil
	}
	v := viewerOf(m)
	reactors, err := s.opportunityReactors(ctx, nil, m.CampaignID, cs, who, &v)
	if err != nil || len(reactors) == 0 {
		return err
	}
	from := squareOfCombatant(who)
	for _, r := range out.Reachable {
		for _, p := range provokedBy(reactors, from, grid.Square{Col: int(r.Col), Row: int(r.Row)}) {
			r.ProvokesReactorIds = append(r.ProvokesReactorIds, p.reactor.ID)
		}
	}
	return nil
}

// returnToReach is the 0 hit points rule: an opportunity attack that drops the
// mover to 0 hit points leaves it on the square of its line where it left the
// reach. pendingID is the damage that landed. It returns the mover's state
// before it went back (for the event, which the undo puts back), or nil when the
// damage is not an opportunity attack's. blocked says that square is taken by
// another combatant (it cannot be put there): the mover stays where it is, and
// the master's log says so. The caller checked that the damage dropped the mover
// from above 0 to 0.
func (s *Service) returnToReach(ctx context.Context, c *combatTx, pendingID string, mover playdb.Combatant) (back *moveState, blocked bool, err error) {
	o, err := c.q.GetOpportunityOfferByAttack(ctx, playdb.GetOpportunityOfferByAttackParams{EncounterID: c.enc.ID, AttackPendingID: &pendingID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("find the opportunity offer of the attack: %w", err)
	}
	if o.MoverID != mover.ID || !placed(mover) {
		return nil, false, nil
	}
	cs, err := c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return nil, false, fmt.Errorf("list the combatants: %w", err)
	}
	if slices.ContainsFunc(cs, func(m playdb.Combatant) bool {
		return m.ID != mover.ID && !m.Defeated && placed(m) && *m.GridCol == o.LeftCol && *m.GridRow == o.LeftRow
	}) {
		return nil, true, nil
	}
	before := moveStateOf(mover)
	if err := c.q.SetCombatantMove(ctx, playdb.SetCombatantMoveParams{
		ID: mover.ID, GridCol: &o.LeftCol, GridRow: &o.LeftRow,
		MovementUsedFt: mover.MovementUsedFt, MovementUsedDft: mover.MovementUsedDft, LastMoveDft: mover.LastMoveDft, CoverMark: mover.CoverMark,
	}); err != nil {
		return nil, false, fmt.Errorf("take the mover back to where it left the reach: %w", err)
	}
	return before, false, nil
}

// publishReturned tells the streams the mover went back, as a move.
func (s *Service) publishReturned(campaignID string, d *encounterData, ev actionEvent) {
	if ev.ReturnedFrom == nil {
		return
	}
	if i := slices.IndexFunc(d.cs, func(c playdb.Combatant) bool { return c.ID == ev.Target }); i >= 0 {
		s.publishCombatantMoved(campaignID, d.enc, d.cs[i])
	}
}

// offerWaits puts an offer back to pending, with no attack: the undo of its
// answer. An offer that is gone (the combatant left the fight) is skipped.
func offerWaits(ctx context.Context, c *combatTx, offerID string) error {
	if _, err := c.q.SetOpportunityOfferState(ctx, playdb.SetOpportunityOfferStateParams{ID: offerID, State: offerPending}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("put back the opportunity offer: %w", err)
	}
	return nil
}

// putBackMove puts a combatant back where it stood before the 0 hit points rule
// took it to the square it left the reach at: the undo of that damage. A nil
// state (the damage did not move anyone) and a combatant that is gone do nothing.
func putBackMove(ctx context.Context, c *combatTx, id string, st *moveState) error {
	if st == nil || !st.Placed {
		return nil
	}
	if err := c.q.SetCombatantMove(ctx, playdb.SetCombatantMoveParams{
		ID: id, GridCol: &st.Col, GridRow: &st.Row, MovementUsedFt: st.UsedDFt / 10, MovementUsedDft: st.UsedDFt, LastMoveDft: st.LastDFt, CoverMark: coverOrNone(st.Cover),
	}); err != nil {
		return fmt.Errorf("put the mover back: %w", err)
	}
	return nil
}

func coverOrNone(c string) string {
	if c == "" {
		return "none"
	}
	return c
}
