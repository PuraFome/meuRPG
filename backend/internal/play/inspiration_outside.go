package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// Bardic Inspiration out of a combat (SRD 5.1, Bard 1). The die a character holds between
// fights is a row of character_inspiration with the game time it has left (10 minutes, moved
// by the master's clock like the effects that last). It is not an effect of character_effects:
// those are made by a spell or an action and are added to a roll by themselves, while this die
// is the player's to decide on after the d20, and in a combat it is the combatant's own die
// (combatants.inspiration_*). The row moves onto the combatant when a combat takes the
// character in and back when it ends, whole, so there is never a copy.

// eventInspirationGiven is the session event of a die given out of a combat.
const eventInspirationGiven = "inspiration_given"

// outsideDieSeconds is how long the die lasts: 10 minutes of game time (100 rounds).
const outsideDieSeconds = rules.BardicInspirationRounds * rules.SecondsPerRound

type inspirationGivenEvent struct {
	Target string `json:"target"`
	Sides  int32  `json:"sides"`
}

// The kinds of roll that wait for an answer about the die (inspiration_holds.kind).
const (
	holdSceneCheck = "scene_check"
	holdGroupCheck = "group_check"
)

// forcedOutside is the answer to a held roll, carried to the roll path it settles.
type forcedOutside struct {
	holdID, answerKey string
	use               bool
	face              int32
}

type forcedOutsideKey struct{}

func withOutside(ctx context.Context, f *forcedOutside) context.Context {
	return context.WithValue(ctx, forcedOutsideKey{}, f)
}

func outsideOf(ctx context.Context) *forcedOutside {
	f, _ := ctx.Value(forcedOutsideKey{}).(*forcedOutside)
	return f
}

// outsideStep is what a roll path of a character out of a combat gets back from outsideRoll.
type outsideStep struct {
	// Held says the d20 was rolled and kept in a hold: the path writes nothing and answers
	// with Offer.
	Held  bool
	Offer *playv1.OutsideInspirationOffer
	// Settled says the path is the answer to a hold: Roll is its d20 (with the die added when
	// it was used) and Bonus the die.
	Settled bool
	Roll    d20Roll
	Bonus   *playv1.BonusDie
}

// outsideRoll is asked by the roll path of a character that is not in a running combat, once
// it knows the modifier and the mode of the roll. A character that holds no die rolls as
// always (the zero step). One that holds a die has the d20 rolled and kept with the request
// (a retry of the request finds it), and the player is asked; the answer calls the path again
// with the d20 already rolled, and here the die is rolled and added, or kept, and the hold
// is answered. It runs in the roll's transaction.
func (s *Service) outsideRoll(
	ctx context.Context, tx pgx.Tx, q *playdb.Queries, m authz.Membership, kind, characterID, key string,
	raw proto.Message, in rollInput, modifier int, mode combat.RollMode, reliable bool,
) (outsideStep, error) {
	die, err := q.GetCharacterInspiration(ctx, playdb.GetCharacterInspirationParams{CharacterID: characterID, CampaignID: m.CampaignID})
	f := outsideOf(ctx)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if f == nil {
			return outsideStep{}, nil
		}
		if f.use {
			return outsideStep{}, errDieGone()
		}
	case err != nil:
		return outsideStep{}, fmt.Errorf("find the die of the character: %w", err)
	}
	if f != nil {
		hold, err := q.GetInspirationHold(ctx, playdb.GetInspirationHoldParams{CampaignID: m.CampaignID, ID: f.holdID})
		if err != nil {
			return outsideStep{}, fmt.Errorf("find the held roll: %w", err)
		}
		if hold.AnswerKey != nil && *hold.AnswerKey != f.answerKey {
			return outsideStep{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("the roll was answered already"))
		}
		faces := intsOf(hold.Faces)
		roll := d20Roll{Faces: faces, Index: int(clamp32(int(hold.Counted), 0, len(faces)-1)), Physical: hold.Physical}
		roll.Total = faces[roll.Index] + modifier
		roll = reliableD20(roll, modifier, reliable)
		step := outsideStep{Settled: true}
		if f.use {
			if f.face < 1 || f.face > die.Sides {
				return outsideStep{}, connect.NewError(connect.CodeInvalidArgument, errors.New("typed_face must be a face of the die"))
			}
			if n, err := q.DeleteCharacterInspiration(ctx, characterID); err != nil {
				return outsideStep{}, fmt.Errorf("use the die: %w", err)
			} else if n == 0 {
				return outsideStep{}, errDieGone()
			}
			roll.Total += int(f.face)
			step.Bonus = &playv1.BonusDie{SourceKey: bardicInspirationFt, Sides: die.Sides, Face: f.face, Used: true}
		}
		if err := q.AnswerInspirationHold(ctx, playdb.AnswerInspirationHoldParams{ID: hold.ID, AnswerKey: &f.answerKey}); err != nil {
			return outsideStep{}, fmt.Errorf("answer the held roll: %w", err)
		}
		step.Roll = roll
		return step, nil
	}
	hold, err := q.GetInspirationHoldByKey(ctx, playdb.GetInspirationHoldByKeyParams{CampaignID: m.CampaignID, IdempotencyKey: key})
	switch {
	case err == nil:
		return outsideStep{Held: true, Offer: s.offerOfHold(ctx, tx, hold, die)}, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return outsideStep{}, fmt.Errorf("find the held roll: %w", err)
	}
	open, err := q.ListInspirationHoldsOfCharacter(ctx, characterID)
	if err != nil {
		return outsideStep{}, fmt.Errorf("list the held rolls: %w", err)
	}
	if len(open) > 0 {
		return outsideStep{}, resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_INSPIRATION_PENDING, "answer the Bardic Inspiration question of the roll first")
	}
	roll, err := s.d20With(in, modifier, mode)
	if err != nil {
		return outsideStep{}, err
	}
	roll = reliableD20(roll, modifier, reliable)
	body, err := proto.Marshal(raw)
	if err != nil {
		return outsideStep{}, fmt.Errorf("keep the request: %w", err)
	}
	hold, err = q.InsertInspirationHold(ctx, playdb.InsertInspirationHoldParams{
		CampaignID: m.CampaignID, CharacterID: characterID, UserID: m.UserID, Kind: kind, IdempotencyKey: key, Request: body,
		Faces: faces32(roll.Faces), Modifier: clamp32(modifier, -1000, 1000), Total: clamp32(roll.Total, -1000, 1000),
		Counted: clamp32(roll.Index, 0, 1), Physical: roll.Physical, CreatedAt: s.now(),
	})
	if err != nil {
		return outsideStep{}, fmt.Errorf("hold the roll: %w", err)
	}
	return outsideStep{Held: true, Offer: s.offerOfHold(ctx, tx, hold, die)}, nil
}

func errDieGone() error {
	return connect.NewError(connect.CodeFailedPrecondition, errors.New("the character holds no die any more"))
}

// offerOfHold is the question a hold asks: the d20 with the check's bonus, and the die.
func (s *Service) offerOfHold(ctx context.Context, tx pgx.Tx, hold playdb.InspirationHold, die playdb.CharacterInspiration) *playv1.OutsideInspirationOffer {
	roll := diceRoll(1, 20, hold.Faces, hold.Modifier, hold.Total, hold.Physical)
	if len(hold.Faces) == pairDice {
		roll = diceRoll(pairDice, 20, hold.Faces, hold.Modifier, hold.Total, hold.Physical)
		roll.CountedIndex = clamp32(int(hold.Counted), 0, 1)
		markTreated(roll, hold.Modifier, hold.Total)
	}
	return &playv1.OutsideInspirationOffer{HoldId: hold.ID, Sides: die.Sides, FromName: s.nameOf(ctx, tx, hold.CampaignID, die.SourceCharacterID), D20: roll}
}

// nameOf is the name of a character of the table, "" when it is gone.
func (s *Service) nameOf(ctx context.Context, tx pgx.Tx, campaignID string, id *string) string {
	if id == nil {
		return ""
	}
	chars, err := s.roster.CombatCharacters(ctx, tx, campaignID, []string{*id})
	if err != nil || len(chars) != 1 {
		return ""
	}
	return chars[0].Name
}

// GiveBardicInspirationOutside implements playv1connect.ResourceServiceHandler.
func (s *Service) GiveBardicInspirationOutside(
	ctx context.Context,
	req *connect.Request[playv1.GiveBardicInspirationOutsideRequest],
) (*connect.Response[playv1.GiveBardicInspirationOutsideResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	target, err := parseCombatID(req.Msg.GetTargetCharacterId(), "character")
	if err != nil {
		return nil, err
	}
	hash := idem.Hash(req.Msg)
	var after *playv1.CharacterVitals
	var bard link.Character
	var told bool
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		after, told = nil, false
		q := s.queries.WithTx(tx)
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		done, again, err := eventByKey(ctx, q, session.ID, key, eventInspirationGiven, hash)
		if err != nil {
			return err
		}
		if again {
			if done.ActorUserID == nil || *done.ActorUserID != m.UserID {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key was already used for another change"))
			}
			var ok bool
			if bard, ok, err = s.myCharacter(ctx, tx, m); err != nil || !ok {
				return err
			}
			return nil
		}
		if enc, err := q.GetOpenEncounter(ctx, session.ID); err == nil && enc.Status == statusActive {
			return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_COMBAT_OPEN, "a combat is running: give the die in it")
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("find the open encounter: %w", err)
		}
		who, has, err := s.myCharacter(ctx, tx, m)
		if err != nil {
			return err
		}
		if !has {
			return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NOT_AVAILABLE, "the character does not have this feature")
		}
		bard = who
		sheet, err := s.roster.CombatSheet(ctx, tx, m.CampaignID, who.ID)
		if err != nil {
			return err
		}
		if sheet.BardicDie == 0 {
			return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NOT_AVAILABLE, "the character does not have this feature")
		}
		party, err := s.roster.CombatParty(ctx, tx, m.CampaignID)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(party, func(c link.Character) bool { return c.ID == target && c.Player && !c.Reserved })
		if i < 0 {
			return connect.NewError(connect.CodeNotFound, errors.New("character not found"))
		}
		refused := resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_TARGET_REFUSED, "the creature cannot get a die")
		if target == who.ID {
			return refused
		}
		if _, err := q.GetCharacterInspiration(ctx, playdb.GetCharacterInspirationParams{CharacterID: target, CampaignID: m.CampaignID}); err == nil {
			return refused
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("find the die of the target: %w", err)
		}
		now, err := s.vitals.GetVitalsTx(ctx, tx, m.CampaignID, who.ID)
		if err != nil {
			return err
		}
		if r := resourceOf(now, rules.BardicInspirationKey); r == nil || r.GetUsed() >= r.GetTotal() {
			return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_USES_LEFT, "no use of Bardic Inspiration is left")
		}
		c, err := s.openTx(ctx, combatTx{tx: tx, q: q, session: session, now: s.now(), characterID: &who.ID, hash: hash})
		if err != nil {
			return err
		}
		if after, err = s.spendResource(ctx, c, who.ID, rules.BardicInspirationKey, 1); err != nil {
			return err
		}
		if err := q.InsertCharacterInspiration(ctx, playdb.InsertCharacterInspirationParams{
			CharacterID: target, CampaignID: m.CampaignID, SourceCharacterID: &who.ID,
			Sides: clamp32(sheet.BardicDie, 6, 12), SecondsLeft: outsideDieSeconds, CreatedAt: c.now,
		}); err != nil {
			return fmt.Errorf("give the die: %w", err)
		}
		if _, err := insertSceneEvent(ctx, c, eventInspirationGiven, &m.UserID, &key, inspirationGivenEvent{Target: target, Sides: clamp32(sheet.BardicDie, 6, 12)}); err != nil {
			return err
		}
		told = true
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "give bardic inspiration", err)
	}
	if told {
		s.publishCastsChanged(m.CampaignID, false) // the table's hint to read the dice held again
		s.publishVitals(m.CampaignID, after)
		if v, err := s.vitals.GetVitals(ctx, m.CampaignID, target); err == nil {
			s.publishVitals(m.CampaignID, v)
		}
	}
	if after == nil {
		if v, err := s.vitals.GetVitals(ctx, m.CampaignID, bard.ID); err == nil {
			after = v
		}
	}
	out := &playv1.GiveBardicInspirationOutsideResponse{Vitals: after}
	if row, err := s.queries.GetCharacterInspiration(ctx, playdb.GetCharacterInspirationParams{CharacterID: target, CampaignID: m.CampaignID}); err == nil {
		out.Die = s.dieViewOf(ctx, nil, row, "")
	}
	return connect.NewResponse(out), nil
}

// resourceOf is a resource of the character's vitals, nil when it has none.
func resourceOf(v *playv1.CharacterVitals, key string) *playv1.ResourceUsage {
	i := slices.IndexFunc(v.GetResources(), func(r *playv1.ResourceUsage) bool { return r.GetKey() == key })
	if i < 0 {
		return nil
	}
	return v.GetResources()[i]
}

// dieViewOf is the die a character holds, as the table reads it.
func (s *Service) dieViewOf(ctx context.Context, tx pgx.Tx, row playdb.CharacterInspiration, name string) *playv1.OutsideInspirationDie {
	return &playv1.OutsideInspirationDie{
		CharacterId: row.CharacterID, CharacterName: name, Sides: row.Sides, FromName: s.nameOf(ctx, tx, row.CampaignID, row.SourceCharacterID),
		SecondsLeft: row.SecondsLeft,
	}
}

// GetOutsideInspiration implements playv1connect.ResourceServiceHandler.
func (s *Service) GetOutsideInspiration(
	ctx context.Context,
	req *connect.Request[playv1.GetOutsideInspirationRequest],
) (*connect.Response[playv1.GetOutsideInspirationResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	party, err := s.roster.CombatParty(ctx, nil, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list the party", err)
	}
	rows, err := s.queries.ListCharacterInspirationOfCampaign(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list the dice", err)
	}
	nameOfChar := func(id string) string {
		if i := slices.IndexFunc(party, func(c link.Character) bool { return c.ID == id }); i >= 0 {
			return party[i].Name
		}
		return ""
	}
	out := &playv1.GetOutsideInspirationResponse{}
	if enc, err := s.openEncounterOf(ctx, m.CampaignID); err == nil && enc != nil && enc.Status == statusActive {
		out.CombatOpen = true
	}
	if m.Role == authz.RoleMaster {
		for _, r := range rows {
			out.Held = append(out.Held, s.dieViewOf(ctx, nil, r, nameOfChar(r.CharacterID)))
		}
		return connect.NewResponse(out), nil
	}
	i := slices.IndexFunc(party, func(c link.Character) bool { return c.PlayerUserID != "" && c.PlayerUserID == m.UserID })
	if i < 0 {
		return connect.NewResponse(out), nil
	}
	me := party[i]
	for _, r := range rows {
		if r.CharacterID == me.ID {
			out.Mine = s.dieViewOf(ctx, nil, r, me.Name)
		}
	}
	if out.Mine != nil {
		holds, err := s.queries.ListInspirationHoldsOfCharacter(ctx, me.ID)
		if err != nil {
			return nil, s.dbError(ctx, "list the held rolls", err)
		}
		for _, h := range holds {
			out.Offers = append(out.Offers, s.offerOfHold(ctx, nil, h, rows[slices.IndexFunc(rows, func(r playdb.CharacterInspiration) bool { return r.CharacterID == me.ID })]))
		}
	}
	sheet, err := s.roster.CombatSheet(ctx, nil, m.CampaignID, me.ID)
	if err != nil || sheet.BardicDie == 0 {
		return connect.NewResponse(out), nil
	}
	out.IsBard, out.Sides = true, clamp32(sheet.BardicDie, 0, 12)
	if v, err := s.vitals.GetVitals(ctx, m.CampaignID, me.ID); err == nil {
		if r := resourceOf(v, rules.BardicInspirationKey); r != nil {
			out.UsesMax, out.UsesLeft = r.GetTotal(), max(r.GetTotal()-r.GetUsed(), 0)
		}
	}
	for _, c := range party {
		if !c.Player || c.Reserved || c.ID == me.ID {
			continue
		}
		t := &playv1.BardicInspirationTarget{CharacterId: c.ID, Name: c.Name}
		if slices.ContainsFunc(rows, func(r playdb.CharacterInspiration) bool { return r.CharacterID == c.ID }) {
			t.DisabledReasonPt = rules.BardicInspirationRefusal(rules.BardicInspirationTarget{CanHear: true, HasDie: true})
		}
		out.Targets = append(out.Targets, t)
	}
	return connect.NewResponse(out), nil
}

// openEncounterOf is the open combat of the campaign's open game session, nil when none.
func (s *Service) openEncounterOf(ctx context.Context, campaignID string) (*playdb.Encounter, error) {
	session, err := s.queries.GetOpenGameSession(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // no session, no combat
	}
	if err != nil {
		return nil, err //nolint:wrapcheck // the caller wraps
	}
	enc, err := s.queries.GetOpenEncounter(ctx, session.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // no combat
	}
	if err != nil {
		return nil, err //nolint:wrapcheck // the caller wraps
	}
	return &enc, nil
}

// AnswerOutsideInspiration implements playv1connect.ResourceServiceHandler.
func (s *Service) AnswerOutsideInspiration(
	ctx context.Context,
	req *connect.Request[playv1.AnswerOutsideInspirationRequest],
) (*connect.Response[playv1.AnswerOutsideInspirationResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RolePlayer)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	holdID, err := parseCombatID(req.Msg.GetHoldId(), "held roll")
	if err != nil {
		return nil, err
	}
	var in rollInput
	if req.Msg.GetUse() {
		switch roll := req.Msg.GetRoll().(type) {
		case *playv1.AnswerOutsideInspirationRequest_RollInApp:
			if !roll.RollInApp {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("roll_in_app must be true"))
			}
			in.inApp = true
		case *playv1.AnswerOutsideInspirationRequest_TypedFace:
			in.typed = int(roll.TypedFace)
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("set roll_in_app or typed_face to use the die"))
		}
	}
	hold, err := s.queries.GetInspirationHold(ctx, playdb.GetInspirationHoldParams{CampaignID: m.CampaignID, ID: holdID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && hold.UserID != m.UserID) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("held roll not found"))
	}
	if err != nil {
		return nil, s.dbError(ctx, "find the held roll", err)
	}
	if hold.AnswerKey != nil && *hold.AnswerKey != key {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the roll was answered already"))
	}
	forced := &forcedOutside{holdID: hold.ID, answerKey: key}
	if req.Msg.GetUse() && hold.AnswerKey == nil {
		die, err := s.queries.GetCharacterInspiration(ctx, playdb.GetCharacterInspirationParams{CharacterID: hold.CharacterID, CampaignID: m.CampaignID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errDieGone()
		}
		if err != nil {
			return nil, s.dbError(ctx, "find the die", err)
		}
		if err := s.mustRollThisWay(ctx, nil, m, in); err != nil {
			return nil, err
		}
		face := in.typed
		if in.inApp {
			rolled, err := dice.Roll(s.roller, dice.Expr{Count: 1, Sides: int(die.Sides)})
			if err != nil {
				return nil, s.dbError(ctx, "roll the die", err)
			}
			face = rolled.Faces[0]
		} else if face < 1 || face > int(die.Sides) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("typed_face must be a face of the die"))
		}
		forced.use, forced.face = true, clamp32(face, 1, int(die.Sides))
	}
	switch hold.Kind {
	case holdSceneCheck:
		var orig playv1.RollSceneCheckRequest
		if err := proto.Unmarshal(hold.Request, &orig); err != nil {
			return nil, s.dbError(ctx, "read the held roll", err)
		}
		out, err := s.RollSceneCheck(withOutside(ctx, forced), connect.NewRequest(&orig))
		if err != nil {
			return nil, err
		}
		return connect.NewResponse(&playv1.AnswerOutsideInspirationResponse{Result: &playv1.AnswerOutsideInspirationResponse_SceneCheck{SceneCheck: out.Msg}}), nil
	case holdGroupCheck:
		var orig playv1.RollGroupCheckRequest
		if err := proto.Unmarshal(hold.Request, &orig); err != nil {
			return nil, s.dbError(ctx, "read the held roll", err)
		}
		out, err := s.RollGroupCheck(withOutside(ctx, forced), connect.NewRequest(&orig))
		if err != nil {
			return nil, err
		}
		return connect.NewResponse(&playv1.AnswerOutsideInspirationResponse{Result: &playv1.AnswerOutsideInspirationResponse_GroupCheck{GroupCheck: out.Msg}}), nil
	}
	return nil, connect.NewError(connect.CodeNotFound, errors.New("held roll not found"))
}

// dieToCombat moves the die of a character that joined a combat onto its combatant, with the
// time it has left in whole rounds (59 seconds are 9 rounds); a die with less than a round
// left is gone.
func (s *Service) dieToCombat(ctx context.Context, c *combatTx, all, added []playdb.Combatant) error {
	for _, a := range added {
		if !backedByCharacter(a) {
			continue
		}
		row, err := c.q.GetCharacterInspiration(ctx, playdb.GetCharacterInspirationParams{CharacterID: a.CharacterID, CampaignID: c.session.CampaignID})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("find the die of a character: %w", err)
		}
		if rounds := roundsOfSeconds(row.SecondsLeft); rounds > 0 {
			sides, expires := row.Sides, clamp32(int(max(c.enc.Round, 1)+rounds), 1, 1<<30)
			var from *string
			if row.SourceCharacterID != nil {
				if j := slices.IndexFunc(all, func(o playdb.Combatant) bool { return o.CharacterID == *row.SourceCharacterID && backedByCharacter(o) }); j >= 0 {
					from = &all[j].ID
				}
			}
			if err := c.q.SetCombatantInspirationDie(ctx, playdb.SetCombatantInspirationDieParams{ID: a.ID, Sides: &sides, FromID: from, ExpiresRound: &expires}); err != nil {
				return fmt.Errorf("bring the die into the combat: %w", err)
			}
		}
		if _, err := c.q.DeleteCharacterInspiration(ctx, a.CharacterID); err != nil {
			return fmt.Errorf("take the die off the character: %w", err)
		}
		if err := c.q.DeleteInspirationHoldsOfCharacter(ctx, a.CharacterID); err != nil {
			return fmt.Errorf("forget the held rolls: %w", err)
		}
	}
	return nil
}

// dieToCharacters gives back to the characters the die their combatants hold when a combat ends
// or a character leaves it (only limits it to those combatants), with the rounds it had left,
// 6 seconds each.
func (s *Service) dieToCharacters(ctx context.Context, c *combatTx, cs, only []playdb.Combatant) error {
	for _, a := range cs {
		if !backedByCharacter(a) || !holdsDie(a, max(c.enc.Round, 1)) ||
			(only != nil && !slices.ContainsFunc(only, func(o playdb.Combatant) bool { return o.ID == a.ID })) {
			continue
		}
		left := *a.InspirationExpiresRound - max(c.enc.Round, 1)
		if left > 0 {
			var from *string
			if a.InspirationFrom != nil {
				if j := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.ID == *a.InspirationFrom }); j >= 0 && backedByCharacter(cs[j]) {
					from = &cs[j].CharacterID
				}
			}
			if err := c.q.InsertCharacterInspiration(ctx, playdb.InsertCharacterInspirationParams{
				CharacterID: a.CharacterID, CampaignID: c.session.CampaignID, SourceCharacterID: from, Sides: *a.InspirationSides,
				SecondsLeft: clamp32(int(left*rules.SecondsPerRound), 1, outsideDieSeconds), CreatedAt: c.now,
			}); err != nil {
				return fmt.Errorf("give the die back to the character: %w", err)
			}
		}
		if err := c.q.SetCombatantInspirationDie(ctx, playdb.SetCombatantInspirationDieParams{ID: a.ID}); err != nil {
			return fmt.Errorf("take the die off the combatant: %w", err)
		}
	}
	return nil
}

// dieAdvance moves game time on for the dice held out of a combat: the ones with no time left
// are gone.
func (s *Service) dieAdvance(ctx context.Context, c *combatTx, seconds int32) error {
	if err := c.q.DeleteExpiredCharacterInspiration(ctx, playdb.DeleteExpiredCharacterInspirationParams{CampaignID: c.session.CampaignID, Column2: seconds}); err != nil {
		return fmt.Errorf("end the dice whose time ran out: %w", err)
	}
	if err := c.q.AdvanceCharacterInspiration(ctx, playdb.AdvanceCharacterInspirationParams{CampaignID: c.session.CampaignID, Column2: seconds}); err != nil {
		return fmt.Errorf("move the dice's time on: %w", err)
	}
	return nil
}
