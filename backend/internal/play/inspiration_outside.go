package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
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

// The kinds of roll that wait for an answer about the die (inspiration_holds.kind). The first
// four are rolled out of a combat (the trap search and the hint try also by a character in one,
// when the die is its combatant's); the last three only in a combat, by a player's character.
const (
	holdSceneCheck    = "scene_check"
	holdGroupCheck    = "group_check"
	holdTrapSearch    = "trap_search"
	holdHintTry       = "hint_try"
	holdSpellAttack   = "spell_attack"
	holdEffectSave    = "effect_save"
	holdConcentration = "concentration_save"
)

// offerOfHold is the question a hold asks: the d20 with the check's bonus, and the die.
func (s *Service) offerOfHold(ctx context.Context, tx pgx.Tx, hold playdb.InspirationHold, die heldDie) *playv1.OutsideInspirationOffer {
	return &playv1.OutsideInspirationOffer{HoldId: hold.ID, Sides: die.Sides, FromName: s.nameOf(ctx, tx, hold.CampaignID, die.Source), D20: outsideD20(hold)}
}

// outsideD20 is the d20 of a hold with the check's bonus as the modifier: both dice of a roll with
// advantage or disadvantage, the one that counts marked, and the die Lucky rolled again once the
// player did (the hold's faces are already the new ones then).
func outsideD20(hold playdb.InspirationHold) *playv1.DiceRoll {
	roll := diceRoll(1, 20, hold.Faces, hold.Modifier, hold.Total, hold.Physical)
	if len(hold.Faces) == pairDice {
		roll = diceRoll(pairDice, 20, hold.Faces, hold.Modifier, hold.Total, hold.Physical)
		roll.CountedIndex = clamp32(int(hold.Counted), 0, 1)
		markTreated(roll, hold.Modifier, hold.Total)
	}
	roll.Rerolled = outsideLuckyOf(hold).proto()
	return roll
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
		if bard, after, err = s.giveDie(ctx, tx, q, m, session, key, hash, target); err != nil {
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

// giveDie is the write of GiveBardicInspirationOutside, in its transaction: the checks, the use spent, the die stored
// and the event under the key. It answers the bard and its vitals after the use.
func (s *Service) giveDie(
	ctx context.Context, tx pgx.Tx, q *playdb.Queries, m authz.Membership, session playdb.GameSession, key string, hash *string, target string,
) (bard link.Character, after *playv1.CharacterVitals, err error) {
	if enc, err := q.GetOpenEncounter(ctx, session.ID); err == nil && enc.Status == statusActive {
		return bard, nil, resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_COMBAT_OPEN, "a combat is running: give the die in it")
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return bard, nil, fmt.Errorf("find the open encounter: %w", err)
	}
	who, has, err := s.myCharacter(ctx, tx, m)
	if err != nil {
		return bard, nil, err
	}
	if !has {
		return bard, nil, resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NOT_AVAILABLE, "the character does not have this feature")
	}
	bard = who
	sheet, err := s.roster.CombatSheet(ctx, tx, m.CampaignID, who.ID)
	if err != nil {
		return bard, nil, err
	}
	if sheet.BardicDie == 0 {
		return bard, nil, resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NOT_AVAILABLE, "the character does not have this feature")
	}
	party, err := s.roster.CombatParty(ctx, tx, m.CampaignID)
	if err != nil {
		return bard, nil, err
	}
	i := slices.IndexFunc(party, func(c link.Character) bool { return c.ID == target && c.Player && !c.Reserved })
	if i < 0 {
		return bard, nil, connect.NewError(connect.CodeNotFound, errors.New("character not found"))
	}
	refused := resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_TARGET_REFUSED, "the creature cannot get a die")
	if target == who.ID {
		return bard, nil, refused
	}
	if _, err := q.GetCharacterInspiration(ctx, playdb.GetCharacterInspirationParams{CharacterID: target, CampaignID: m.CampaignID}); err == nil {
		return bard, nil, refused
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return bard, nil, fmt.Errorf("find the die of the target: %w", err)
	}
	now, err := s.vitals.GetVitalsTx(ctx, tx, m.CampaignID, who.ID)
	if err != nil {
		return bard, nil, err
	}
	if r := resourceOf(now, rules.BardicInspirationKey); r == nil || r.GetUsed() >= r.GetTotal() {
		return bard, nil, resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_USES_LEFT, "no use of Bardic Inspiration is left")
	}
	c, err := s.openTx(ctx, combatTx{tx: tx, q: q, session: session, now: s.now(), characterID: &who.ID, hash: hash})
	if err != nil {
		return bard, nil, err
	}
	if after, err = s.spendResource(ctx, c, who.ID, rules.BardicInspirationKey, 1); err != nil {
		return bard, nil, err
	}
	if err := q.InsertCharacterInspiration(ctx, playdb.InsertCharacterInspirationParams{
		CharacterID: target, CampaignID: m.CampaignID, SourceCharacterID: &who.ID,
		Sides: clamp32(sheet.BardicDie, 6, 12), SecondsLeft: outsideDieSeconds, CreatedAt: c.now,
	}); err != nil {
		return bard, nil, fmt.Errorf("give the die: %w", err)
	}
	if _, err := insertSceneEvent(ctx, c, eventInspirationGiven, &m.UserID, &key, inspirationGivenEvent{Target: target, Sides: clamp32(sheet.BardicDie, 6, 12)}); err != nil {
		return bard, nil, err
	}
	return bard, after, nil
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
	// The rolls that wait for the die, wherever the die is: on the character, or on its combatant in a
	// combat; and the ones that wait for the halfling's Lucky question first, die or not.
	die, found, holds, err := s.heldRollsOf(ctx, m.CampaignID, me.ID)
	if err != nil {
		return nil, s.dbError(ctx, "list the held rolls", err)
	}
	for _, h := range holds {
		switch {
		case outsideLuckyPending(h):
			out.LuckyOffers = append(out.LuckyOffers, outsideLuckyOfferOf(h))
		case found:
			out.Offers = append(out.Offers, s.offerOfHold(ctx, nil, h, die))
		}
	}
	sheet, err := s.roster.CombatSheet(ctx, nil, m.CampaignID, me.ID)
	if err != nil || sheet.BardicDie == 0 {
		return connect.NewResponse(out), nil //nolint:nilerr // a sheet that cannot be read has no feature to show
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

// heldRollsOf lists the rolls of a character that wait for an answer, with the die it holds
// (found): about the die, or the halfling's Lucky question, which needs no die. A stale roll (its
// combat round is over, or its die is gone) is left out.
func (s *Service) heldRollsOf(ctx context.Context, campaignID, characterID string) (heldDie, bool, []playdb.InspirationHold, error) {
	die, found, err := s.dieOf(ctx, s.queries, campaignID, characterID)
	if err != nil {
		return heldDie{}, false, nil, err
	}
	all, err := s.queries.ListInspirationHoldsOfCharacter(ctx, characterID)
	if err != nil {
		return heldDie{}, false, nil, fmt.Errorf("list the held rolls: %w", err)
	}
	return die, found, slices.DeleteFunc(all, func(h playdb.InspirationHold) bool { return staleHold(h, die, found) }), nil
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

// outsideDieRollOf reads how the Bardic Inspiration die of AnswerOutsideInspiration comes, when the
// player uses it: rolled in the app or typed from a real die.
func outsideDieRollOf(msg *playv1.AnswerOutsideInspirationRequest) (rollInput, error) {
	if !msg.GetUse() {
		return rollInput{}, nil
	}
	var p rollPick
	switch roll := msg.GetRoll().(type) {
	case *playv1.AnswerOutsideInspirationRequest_RollInApp:
		p.app = &roll.RollInApp
	case *playv1.AnswerOutsideInspirationRequest_TypedFace:
		p.face = &roll.TypedFace
	}
	return rollOfAnswer(p, askDie)
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
	in, err := outsideDieRollOf(req.Msg)
	if err != nil {
		return nil, err
	}
	out, err := s.answerHeld(ctx, m, answerRequest{
		ref: holdRef{Table: inspirationHolds, Campaign: m.CampaignID, ID: holdID}, key: key, question: askDie, yes: req.Msg.GetUse(), in: in,
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(outsideInspirationResultOf(out)), nil
}

// outsideInspirationResultOf is the response of the path that was run again, in the arm of the oneof it
// belongs to.
func outsideInspirationResultOf(out any) *playv1.AnswerOutsideInspirationResponse {
	res := &playv1.AnswerOutsideInspirationResponse{}
	switch o := out.(type) {
	case *playv1.RollSceneCheckResponse:
		res.Result = &playv1.AnswerOutsideInspirationResponse_SceneCheck{SceneCheck: o}
	case *playv1.RollGroupCheckResponse:
		res.Result = &playv1.AnswerOutsideInspirationResponse_GroupCheck{GroupCheck: o}
	case *playv1.SearchForTrapsResponse:
		res.Result = &playv1.AnswerOutsideInspirationResponse_TrapSearch{TrapSearch: o}
	case *playv1.TryPuzzleHintResponse:
		res.Result = &playv1.AnswerOutsideInspirationResponse_HintTry{HintTry: o}
	case *playv1.CastSpellResponse:
		res.Result = &playv1.AnswerOutsideInspirationResponse_SpellCast{SpellCast: o}
	case *playv1.RollEffectSaveResponse:
		res.Result = &playv1.AnswerOutsideInspirationResponse_EffectSave{EffectSave: o}
	case *playv1.ResolveConcentrationSaveResponse:
		res.Result = &playv1.AnswerOutsideInspirationResponse_ConcentrationSave{ConcentrationSave: o}
	}
	return res
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
		// The rolls held in the combat have nothing to settle once it is over (or the character is out of it).
		if err := c.q.DeleteInspirationHoldsOfCharacter(ctx, a.CharacterID); err != nil {
			return fmt.Errorf("forget the rolls held in the combat: %w", err)
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
	if err := c.q.DeleteOrphanInspirationHolds(ctx, c.session.CampaignID); err != nil {
		return fmt.Errorf("forget the rolls that waited for a die that ran out: %w", err)
	}
	if err := c.q.AdvanceCharacterInspiration(ctx, playdb.AdvanceCharacterInspirationParams{CampaignID: c.session.CampaignID, Column2: seconds}); err != nil {
		return fmt.Errorf("move the dice's time on: %w", err)
	}
	return nil
}
