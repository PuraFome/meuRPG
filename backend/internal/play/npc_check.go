package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	maplink "github.com/PuraFome/meuRPG/backend/internal/maps/link"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/cutting"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Cutting Words (SRD 5.1, College of Lore) on an ability check a creature of the master's side makes outside a combat. The master
// rolls the creature's d20 (RollNpcCheck); the bards of the party that may cut it are asked one at a time through a held roll of the
// kind npc_check (inspiration_holds, with one npc_check_asks row for each bard); the check settles when the last one answers or the
// master goes on without them (SkipNpcCheckAsks). Only the use of Bardic Inspiration is spent: outside a combat there is no reaction.
// The rules over plain values are in package cutting; the master's roll is secret (RN-10, RN-20): only the master reads it.

const (
	// eventNpcCheckHeld is the master's roll while a bard is asked; eventNpcCheckResolved when the check settles.
	eventNpcCheckHeld     = "npc_check_held"
	eventNpcCheckResolved = "npc_check_resolved"
)

// npcCheckAskLine is one bard asked, as the master's history keeps it.
type npcCheckAskLine struct {
	Bard  string `json:"bard"`
	Name  string `json:"name"`
	State string `json:"state"`
	Sides int32  `json:"sides"`
	Face  int32  `json:"face,omitempty"`
}

// npcCheckEvent is the payload of npc_check_held and npc_check_resolved. Total is the d20 plus the bonus while the check is held and
// what the check came to once it settled.
type npcCheckEvent struct {
	CharacterID string            `json:"character_id"`
	Name        string            `json:"name"`
	MonsterKey  string            `json:"monster_key,omitempty"`
	Check       string            `json:"check"`
	D20         int32             `json:"d20"`
	Modifier    int32             `json:"modifier"`
	Known       bool              `json:"known"`
	Physical    bool              `json:"physical"`
	HoldID      string            `json:"hold_id,omitempty"`
	Total       int32             `json:"total"`
	Cut         int32             `json:"cut,omitempty"`
	Asked       []npcCheckAskLine `json:"asked,omitempty"`
	Skipped     bool              `json:"skipped,omitempty"`
	// Nobody is why no bard was asked (cutting.Nobody*), kept with the roll so the master reads it again after a reload.
	Nobody string `json:"nobody,omitempty"`
}

// The states of an ask as the rows and the history spell them (package cutting).
const (
	askAsked  = cutting.StateAsked
	askCut    = cutting.StateCut
	askPassed = cutting.StatePassed
	askClosed = cutting.StateClosed
)

var askStateProto = map[string]playv1.NpcCheckAskState{
	askAsked: playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_WAITING, askCut: playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_CUT,
	askPassed: playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_PASSED, askClosed: playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_CLOSED,
}

// read is the master's read of the check: the d20 and its bonus before any die comes off, the bards asked and, once settled, the total.
func (e npcCheckEvent) read(s *Service, settled bool) *playv1.NpcCheckRead {
	out := &playv1.NpcCheckRead{
		HoldId: e.HoldID, CreatureLabel: e.Name, CheckKey: e.Check, CheckName: s.roster.SceneCheckName(e.Check),
		D20:           diceRoll(1, d20Sides, []int32{e.D20}, e.Modifier, clamp32(int(e.D20)+int(e.Modifier), -heldBonusLimit, heldBonusLimit), e.Physical),
		ModifierKnown: e.Known, Settled: settled,
	}
	for _, a := range e.Asked {
		out.Asks = append(out.Asks, &playv1.NpcCheckAsk{CharacterId: a.Bard, CharacterName: a.Name, State: askStateProto[a.State], Sides: a.Sides, Face: a.Face})
	}
	if settled {
		out.Total, out.Cut = e.Total, e.Cut
	}
	out.NobodyReason = nobodyReasonProto[e.Nobody]
	return out
}

var nobodyReasonProto = map[string]playv1.NpcCheckNobodyReason{
	cutting.NobodyNoBard:      playv1.NpcCheckNobodyReason_NPC_CHECK_NOBODY_REASON_NO_BARD_CAN_CUT,
	cutting.NobodyCharmImmune: playv1.NpcCheckNobodyReason_NPC_CHECK_NOBODY_REASON_CHARM_IMMUNE,
	cutting.NobodyTokenHidden: playv1.NpcCheckNobodyReason_NPC_CHECK_NOBODY_REASON_TOKEN_HIDDEN,
	cutting.NobodySees:        playv1.NpcCheckNobodyReason_NPC_CHECK_NOBODY_REASON_NOBODY_SEES,
}

// tellNpcCheck collects the hint that the held rolls changed for the master and for the bards whose question changed: nobody else hears
// of a check (RN-10).
func tellNpcCheck(h *hints, campaignID string, bards ...string) {
	h.tell(campaignID, heldRollsChangedEvent(live.Audience{Master: true}))
	for _, id := range slices.Compact(slices.Sorted(slices.Values(bards))) {
		if id != "" {
			h.tell(campaignID, heldRollsChangedEvent(live.Audience{UserID: id, NotMaster: true}))
		}
	}
}

// npcCheckCall is a RollNpcCheck being made.
type npcCheckCall struct {
	s     *Service
	m     authz.Membership
	req   *playv1.RollNpcCheckRequest
	key   string
	hash  *string
	in    rollInput
	mapID string
	sight maplink.CombatSight
	out   *npcCheckOut
}

// npcCheckOut is what the transaction made: the master's read and whether a bard is asked.
type npcCheckOut struct {
	read *playv1.NpcCheckRead
	held bool
}

// RollNpcCheck implements playv1connect.PlayServiceHandler.
func (s *Service) RollNpcCheck(ctx context.Context, req *connect.Request[playv1.RollNpcCheckRequest]) (*connect.Response[playv1.RollNpcCheckResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	call, err := s.newNpcCheckCall(m, req.Msg)
	if err != nil {
		return nil, err
	}
	var out npcCheckOut
	call.out = &out
	for try := 1; ; try++ {
		// The sight is read before the transaction opens, never inside it (the maps module reads through the pool).
		if call.mapID, call.sight, err = s.npcCheckSight(ctx, m.CampaignID); err != nil {
			return nil, err
		}
		err = s.txOpen(ctx, m.CampaignID, func(tx pgx.Tx, h *hints, session playdb.GameSession) error {
			out = npcCheckOut{}
			return call.run(ctx, tx, h, session)
		})
		if errors.Is(err, errSightStale) && try < maxSightTries {
			continue
		}
		break
	}
	if err != nil {
		return nil, s.dbError(ctx, "roll an NPC's check", err)
	}
	return connect.NewResponse(&playv1.RollNpcCheckResponse{Held: out.held, Check: out.read}), nil
}

// newNpcCheckCall reads the request: the key, the check and how the d20 comes.
func (s *Service) newNpcCheckCall(m authz.Membership, req *playv1.RollNpcCheckRequest) (*npcCheckCall, error) {
	key, err := parseKey(req.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	if !cutting.ValidCheck(req.GetCheckKey()) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("check_key must be an ability or a skill check"))
	}
	if req.GetNpcCharacterId() == "" && req.GetMonsterKey() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name an NPC or a creature"))
	}
	var in rollInput
	switch r := req.GetRoll().(type) {
	case *playv1.RollNpcCheckRequest_RollInApp:
		if !r.RollInApp {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("roll_in_app must be true"))
		}
		in.inApp = true
	case *playv1.RollNpcCheckRequest_D20Face:
		in.typed = int(r.D20Face)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("set roll_in_app or d20_face"))
	}
	return &npcCheckCall{s: s, m: m, req: req, key: key, hash: idem.Hash(req), in: in}, nil
}

// npcCheckSight is the map of the open session and what its players see now (nil for a map with no fog or no map).
func (s *Service) npcCheckSight(ctx context.Context, campaignID string) (string, maplink.CombatSight, error) {
	open, err := s.queries.GetOpenGameSession(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (open.CurrentMapID == nil || s.fog == nil)) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, s.dbError(ctx, "find the open session", err)
	}
	sight, err := s.fog.CombatSight(ctx, campaignID, *open.CurrentMapID)
	if err != nil {
		return "", nil, fmt.Errorf("read what the players see: %w", err)
	}
	return *open.CurrentMapID, sight, nil
}

// run is the transaction of RollNpcCheck.
func (c *npcCheckCall) run(ctx context.Context, tx pgx.Tx, h *hints, session playdb.GameSession) error {
	s, q := c.s, c.s.queries.WithTx(tx)
	if s.fog != nil && conv.OrZero(session.CurrentMapID) != c.mapID {
		return errSightStale
	}
	if again, err := c.replay(ctx, q, h, session); again || err != nil {
		return err
	}
	if err := requireNoOpenCombat(ctx, q, session.ID); err != nil {
		return err
	}
	npc, err := c.creature(ctx, tx)
	if err != nil {
		return err
	}
	tc, err := s.openTx(ctx, combatTx{tx: tx, q: q, session: session, now: s.now(), characterID: &npc.ID, hash: c.hash})
	if err != nil {
		return err
	}
	// A check still held for the bards ends where the next one is rolled: the master has one question at a time (a retry of a key
	// replayed above and never gets here).
	if err := s.settleOpenNpcChecks(ctx, tc, h); err != nil {
		return err
	}
	opts, err := s.npcCheckNumbers(ctx, tx, c.m.CampaignID, npc, []string{c.req.GetCheckKey()})
	if err != nil {
		return err
	}
	opt := opts[0]
	face, res, err := s.dieRoll(c.in, d20Sides)
	if err != nil {
		return err
	}
	ev := npcCheckEvent{
		CharacterID: npc.ID, Name: npc.Name, MonsterKey: npc.MonsterKey, Check: c.req.GetCheckKey(), D20: clamp32(face, 1, d20Sides),
		Modifier: clamp32(opt.Bonus, -heldBonusLimit, heldBonusLimit), Known: opt.Known, Physical: res.Physical,
	}
	ev.Total = ev.D20 + ev.Modifier
	bards, nobody, err := s.npcCheckBards(ctx, tx, q, session, npc, c.req.GetBardSees(), c.sight)
	if err != nil {
		return err
	}
	if len(bards) == 0 { // nobody may cut it: it settles at once, and the master reads why
		ev.Nobody = nobody
		if _, err := insertSceneEvent(ctx, tc, eventNpcCheckResolved, &c.m.UserID, &c.key, ev); err != nil {
			return err
		}
		c.out.read = ev.read(s, true)
		return nil
	}
	return c.hold(ctx, tc, h, ev, bards)
}

// hold keeps the master's roll for the bards: the hold, one ask for each bard and the event under the request's key.
func (c *npcCheckCall) hold(ctx context.Context, tc *combatTx, h *hints, ev npcCheckEvent, bards []askedBard) error {
	body, err := proto.Marshal(c.req)
	if err != nil {
		return fmt.Errorf("keep the request: %w", err)
	}
	row, err := tc.q.InsertInspirationHold(ctx, playdb.InsertInspirationHoldParams{
		CampaignID: c.m.CampaignID, CharacterID: ev.CharacterID, UserID: c.m.UserID, Kind: string(heldNpcCheck), IdempotencyKey: c.key,
		Request: body, Faces: []int32{ev.D20}, Modifier: ev.Modifier, Total: ev.Total, Physical: ev.Physical, CreatedAt: tc.now,
	})
	if err != nil {
		return fmt.Errorf("hold the check: %w", err)
	}
	ev.HoldID = row.ID
	for i, b := range bards {
		if _, err := tc.q.InsertNpcCheckAsk(ctx, playdb.InsertNpcCheckAskParams{
			HoldID: row.ID, BardCharacterID: &b.char.ID, Position: conv.Int32(i), Sides: conv.Int32(b.sides), CreatureSeen: b.named, CreatedAt: tc.now,
		}); err != nil {
			return fmt.Errorf("ask a bard: %w", err)
		}
		ev.Asked = append(ev.Asked, npcCheckAskLine{Bard: b.char.ID, Name: b.char.Name, State: askAsked, Sides: conv.Int32(b.sides)})
	}
	if _, err := insertSceneEvent(ctx, tc, eventNpcCheckHeld, &c.m.UserID, &c.key, ev); err != nil {
		return err
	}
	tellNpcCheck(h, c.m.CampaignID, bards[0].char.PlayerUserID)
	c.out.read, c.out.held = ev.read(c.s, false), true
	return nil
}

// replay answers a retry of the key with the first call's read, changing nothing. It says whether it did.
func (c *npcCheckCall) replay(ctx context.Context, q *playdb.Queries, h *hints, session playdb.GameSession) (bool, error) {
	done, err := q.GetSessionEventByIdempotencyKey(ctx, playdb.GetSessionEventByIdempotencyKeyParams{GameSessionID: session.ID, IdempotencyKey: &c.key})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("find the event of this idempotency key: %w", err)
	}
	if (done.Kind != eventNpcCheckHeld && done.Kind != eventNpcCheckResolved) || hashDiffers(done.IdempotencyHash, c.hash) ||
		done.ActorUserID == nil || *done.ActorUserID != c.m.UserID {
		return false, errKeyReused()
	}
	var ev npcCheckEvent
	if err := json.Unmarshal(done.Payload, &ev); err != nil {
		return false, fmt.Errorf("decode the check of event %s: %w", done.ID, err)
	}
	h.replay()
	view, held, err := c.s.npcCheckLive(ctx, q, c.m.CampaignID, ev, done.Kind == eventNpcCheckResolved)
	if err != nil {
		return false, err
	}
	c.out.read, c.out.held = view, held
	if held {
		tellNpcCheck(h, c.m.CampaignID)
	}
	return true, nil
}

// npcCheckLive is the read of a check as it stands now: the payload of its event with the answers of the bards so far, from the
// hold; as the event kept it when the hold is gone. It says too whether the check is still held: its hold is there and nobody has
// settled it. A held event whose hold is gone (forgotten, never settled) reads as neither held nor settled: its total is not an outcome.
func (s *Service) npcCheckLive(ctx context.Context, q *playdb.Queries, campaignID string, ev npcCheckEvent, resolved bool) (*playv1.NpcCheckRead, bool, error) {
	if ev.HoldID == "" {
		return ev.read(s, resolved), false, nil
	}
	hold, err := q.GetInspirationHold(ctx, playdb.GetInspirationHoldParams{CampaignID: campaignID, ID: ev.HoldID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ev.read(s, resolved), false, nil // the hold is gone: the resolved event kept what was settled, or the skip
	}
	if err != nil {
		return nil, false, fmt.Errorf("find the held check: %w", err)
	}
	asks, err := q.ListNpcCheckAsks(ctx, hold.ID)
	if err != nil {
		return nil, false, fmt.Errorf("list the asks of the check: %w", err)
	}
	settled := hold.AnswerKey != nil
	return ev.withAsks(asks, settled).read(s, settled), !settled, nil
}

// withAsks is the event as the asks stand: each bard's state and die, and, once settled, the total with the dice taken off.
func (e npcCheckEvent) withAsks(asks []playdb.NpcCheckAsk, settled bool) npcCheckEvent {
	e.Asked = slices.Clone(e.Asked)
	var dice []int
	for _, a := range asks {
		i := int(a.Position) // the asks are in the order of the event's lines: the bard's character may be gone
		if i < 0 || i >= len(e.Asked) {
			continue
		}
		e.Asked[i].State = a.State
		if a.DieFace != nil && a.State == askCut {
			e.Asked[i].Face = *a.DieFace
			dice = append(dice, int(*a.DieFace))
		}
	}
	e.Total = e.D20 + e.Modifier
	if settled {
		total, cut := cutting.Settle(int(e.Total), dice)
		e.Total, e.Cut = clamp32(total, -heldBonusLimit, heldBonusLimit), clamp32(cut, 0, heldBonusLimit)
	}
	return e
}

// creature is the NPC that rolls: a living NPC of the campaign, or the hidden one the app keeps for an SRD creature.
func (c *npcCheckCall) creature(ctx context.Context, tx pgx.Tx) (link.Character, error) {
	s := c.s
	if id := c.req.GetNpcCharacterId(); id != "" {
		id, err := parseCombatID(id, "character")
		if err != nil {
			return link.Character{}, err
		}
		chars, err := s.roster.CombatCharacters(ctx, tx, c.m.CampaignID, []string{id})
		if err != nil {
			return link.Character{}, err
		}
		if len(chars) != 1 || chars[0].Player {
			return link.Character{}, errCharacterNotFound()
		}
		return chars[0], nil
	}
	npc, ok, err := s.roster.MonsterNpc(ctx, tx, c.m.CampaignID, c.m.UserID, c.req.GetMonsterKey(), s.now())
	if err != nil {
		return link.Character{}, err
	}
	if !ok {
		return link.Character{}, connect.NewError(connect.CodeNotFound, errors.New("creature not found"))
	}
	return npc, nil
}

// npcCheckNumbers is what the creature adds to each of the checks in keys, one option for each, in order: its stat block's number when it
// is an SRD creature (one CreatureChecks read for all the keys), its sheet's otherwise (one SceneOptions call for all the keys); an NPC typed
// by hand has none (Known false: it rolls +0).
func (s *Service) npcCheckNumbers(ctx context.Context, tx pgx.Tx, campaignID string, npc link.Character, keys []string) ([]link.SceneOption, error) {
	if npc.MonsterKey != "" {
		return s.roster.CreatureChecks(ctx, tx, campaignID, "", npc.MonsterKey, keys)
	}
	out := make([]link.SceneOption, len(keys))
	opts, err := s.roster.SceneOptions(ctx, tx, campaignID, npc.ID, keys)
	if err != nil || len(opts) != len(keys) {
		return out, err
	}
	return opts, nil
}

// askedBard is a bard that is asked about the check.
type askedBard struct {
	char  link.Character
	sides int
	named bool
}

// npcCheckBards lists the bards asked, in the order of the party: nobody when the creature is immune to being charmed. When it lists
// none it says why (cutting.WhyNobody).
func (s *Service) npcCheckBards(ctx context.Context, tx pgx.Tx, q *playdb.Queries, session playdb.GameSession, npc link.Character, boxSees bool, sight maplink.CombatSight) ([]askedBard, string, error) {
	campaignID := session.CampaignID
	st, err := s.roster.ReactionStats(ctx, tx, campaignID, npc.ID)
	if err != nil {
		return nil, "", err
	}
	party, err := s.roster.CombatParty(ctx, tx, campaignID)
	if err != nil {
		return nil, "", err
	}
	place, err := s.npcPlacement(ctx, tx, q, session, npc.ID)
	if err != nil {
		return nil, "", err
	}
	var out []askedBard
	qualified := 0
	for _, c := range party {
		if !c.Player || c.Reserved || c.PlayerUserID == "" {
			continue
		}
		b, ok, err := s.bardOf(ctx, tx, campaignID, c)
		if err != nil {
			return nil, "", err
		}
		if !ok {
			continue
		}
		qualified++
		if asked, ok := askedBardOf(c, b, place, boxSees, sight); ok && !st.CharmImmune {
			out = append(out, asked)
		}
	}
	return out, cutting.WhyNobody(qualified, len(out), st.CharmImmune, place.TokenHidden()), nil
}

// askedBardOf is the bard asked about the creature, when the bard may cut it and sees it.
func askedBardOf(c link.Character, b bardWith, place npcPlace, boxSees bool, sight maplink.CombatSight) (askedBard, bool) {
	seen := func(sq grid.Square) bool { return sight == nil || sight.Sees(c.PlayerUserID, sq) }
	inScene := place.scene.has(c.PlayerUserID)
	sees := place.SightOf(c.ID, boxSees, seen, place.onStage && inScene, inScene)
	if !sees.Sees() {
		return askedBard{}, false
	}
	return askedBard{char: c, sides: cutting.Die(b.level), named: sees.Named()}, true
}

// bardWith is a bard of the party and its level.
type bardWith struct{ level int }

// bardOf reads whether the character is a bard that is asked about a check, and its bard level.
func (s *Service) bardOf(ctx context.Context, tx pgx.Tx, campaignID string, c link.Character) (bardWith, bool, error) {
	rs, err := s.roster.ReactionStats(ctx, tx, campaignID, c.ID)
	if err != nil || !rs.CuttingWords {
		return bardWith{}, false, err
	}
	v, err := s.vitals.GetVitalsTx(ctx, tx, campaignID, c.ID)
	if err != nil {
		return bardWith{}, false, err
	}
	uses := 0
	if r := resourceOf(v, rules.BardicInspirationKey); r != nil {
		uses = int(r.GetTotal() - r.GetUsed())
	}
	bard := cutting.Bard{Feature: true, Ask: rs.CuttingAsk, HitPoints: int(v.GetHitPointsCurrent()), UsesLeft: uses}
	return bardWith{level: rs.BardLevel}, bard.Asks(), nil
}

// npcPlace is where the creature stands for the question of a check: its token on the session's map, the stage and who the scene is open
// to (the whole table when no scene is open).
type npcPlace struct {
	cutting.Place
	onStage bool
	scene   viewers
}

// npcPlacement reads the tokens of the session's current map (with a grid) and the stage the bards' players see.
func (s *Service) npcPlacement(ctx context.Context, tx pgx.Tx, q *playdb.Queries, session playdb.GameSession, npcID string) (npcPlace, error) {
	tokens, err := s.mapTokensOf(ctx, tx, session)
	if err != nil {
		return npcPlace{}, err
	}
	p := npcPlace{Place: cutting.Place{Tokens: tokens, NPC: npcID}}
	rows, err := q.ListShownStage(ctx, session.ID)
	if err != nil {
		return p, fmt.Errorf("list the stage: %w", err)
	}
	p.onStage = slices.ContainsFunc(rows, func(r playdb.StageNpc) bool { return r.CharacterID == npcID })
	p.scene, err = sceneViewersOf(ctx, q, session) // read whether or not the creature is on the stage: the box asks only the scene's players
	return p, err
}

// mapTokensOf reads the tokens of the session's current map, none when there is no map. On a map with no grid the tokens are still read
// (a hidden one stays hidden) but stand on no square: they are marked Unmeasured.
func (s *Service) mapTokensOf(ctx context.Context, tx pgx.Tx, session playdb.GameSession) (map[string]cutting.Token, error) {
	out := map[string]cutting.Token{}
	if session.CurrentMapID == nil {
		return out, nil
	}
	g, err := s.maps.MapGrid(ctx, tx, session.CampaignID, *session.CurrentMapID)
	if connect.CodeOf(err) == connect.CodeNotFound {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	tokens, err := s.maps.MapTokens(ctx, tx, *session.CurrentMapID)
	if err != nil {
		return nil, fmt.Errorf("read the tokens: %w", err)
	}
	cells := grid.Grid{Columns: int(g.Columns), Rows: int(g.Rows)}
	for _, t := range tokens {
		if t.CharacterID == "" {
			continue
		}
		tok := cutting.Token{Hidden: t.Hidden, Unmeasured: !g.OK()}
		if g.OK() {
			tok.Sq = cells.SquareOf(int(t.XBP), int(t.YBP))
		}
		out[t.CharacterID] = tok
	}
	return out, nil
}
