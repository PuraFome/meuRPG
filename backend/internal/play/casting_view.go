package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// What the casts outside a combat show each person (RN-10, RN-20): the master reads
// every cast; a player reads the casts of the characters they may see, never one an NPC
// that is not on the stage cast, and the hit points, healing and armor class of a target
// only when the target is their own character.

// castLens is who is looking at the casts, and the characters they name.
type castLens struct {
	master bool
	userID string
	chars  map[string]link.Character
	name   func(key string) string
}

// lensFor reads the characters and the spell names the rows name.
func (s *Service) lensFor(ctx context.Context, m authz.Membership, rows []playdb.SpellCast) (castLens, error) {
	l := castLens{master: m.Role == authz.RoleMaster, userID: m.UserID, chars: map[string]link.Character{}}
	var ids []string
	for _, r := range rows {
		ids = append(ids, r.CasterID)
		for _, t := range castTargetsOf(r) {
			ids = append(ids, t.ID)
		}
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) > 0 {
		chars, err := s.roster.SessionCharacters(ctx, nil, m.CampaignID, ids)
		if err != nil {
			return l, err
		}
		for _, ch := range chars {
			l.chars[ch.ID] = ch
		}
	}
	names, err := s.roster.ContentNames(ctx, nil, m.CampaignID)
	if err != nil {
		return l, err
	}
	l.name = names
	return l, nil
}

// sees says the viewer may read the cast at all.
func (l castLens) sees(r playdb.SpellCast) bool { return l.master || !r.Secret }

// mine says the viewer's player plays the character.
func (l castLens) mine(characterID string) bool {
	ch, ok := l.chars[characterID]
	return ok && ch.Player && ch.PlayerUserID != "" && ch.PlayerUserID == l.userID
}

var castStatusToProto = map[string]playv1.OutsideCastStatus{
	castCasting: playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_CASTING,
	castActive:  playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_ACTIVE,
	castEnded:   playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_ENDED,
	castFailed:  playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_FAILED,
}

var castEndToProto = map[string]playv1.OutsideCastEnd{
	endInstant:       playv1.OutsideCastEnd_OUTSIDE_CAST_END_INSTANT,
	endDismissed:     playv1.OutsideCastEnd_OUTSIDE_CAST_END_DISMISSED,
	endConcentration: playv1.OutsideCastEnd_OUTSIDE_CAST_END_CONCENTRATION,
	endRest:          playv1.OutsideCastEnd_OUTSIDE_CAST_END_REST,
	endInterrupted:   playv1.OutsideCastEnd_OUTSIDE_CAST_END_INTERRUPTED,
	endCasterGone:    playv1.OutsideCastEnd_OUTSIDE_CAST_END_CASTER_GONE,
}

var castEffectToProto = map[string]playv1.CastEffect{
	castNarrated: playv1.CastEffect_CAST_EFFECT_NARRATED,
	castHeal:     playv1.CastEffect_CAST_EFFECT_HEAL,
	castTempHP:   playv1.CastEffect_CAST_EFFECT_TEMPORARY_HIT_POINTS,
	castMaxHP:    playv1.CastEffect_CAST_EFFECT_MAX_HIT_POINTS,
	castArmor:    playv1.CastEffect_CAST_EFFECT_ARMOR_CLASS,
}

var restToProto = map[string]playv1.RestThatEnds{
	"short": playv1.RestThatEnds_REST_THAT_ENDS_SHORT,
	"long":  playv1.RestThatEnds_REST_THAT_ENDS_LONG,
}

// view is the cast as the viewer reads it.
func (l castLens) view(r playdb.SpellCast) *playv1.OutsideCast {
	caster := l.chars[r.CasterID]
	out := &playv1.OutsideCast{
		Id: r.ID, SpellKey: r.SpellKey, SpellNamePt: l.name(r.SpellKey), CasterId: r.CasterID, CasterName: caster.Name,
		CasterIsNpc: !caster.Player, Status: castStatusToProto[r.Status], Ritual: r.Ritual, SlotLevel: r.SlotLevel, SlotPact: r.SlotPact,
		CastingMinutes: r.CastingMinutes, Concentrating: r.Concentrating, Lasts: r.Lasts,
		StartedAt: timestamppb.New(r.StartedAt), DiceCount: r.DiceCount, DiceSides: r.DiceSides, Faces: r.RollFaces,
		RollTotal: r.RollTotal, Physical: r.Physical,
	}
	if r.DurationSeconds != nil {
		out.DurationSeconds = *r.DurationSeconds
	}
	if r.RestEnds != nil {
		out.RestEnds = restToProto[*r.RestEnds]
	}
	if r.CastAt != nil {
		out.CastAt = timestamppb.New(*r.CastAt)
	}
	if r.EndedAt != nil {
		out.EndedAt = timestamppb.New(*r.EndedAt)
	}
	if r.EndReason != nil {
		out.EndReason = castEndToProto[*r.EndReason]
	}
	if l.master {
		out.Secret = r.Secret
	}
	if l.master || l.mine(r.CasterID) {
		out.CreatureIds = r.CreatureIds
	}
	for _, t := range castTargetsOf(r) {
		ch := l.chars[t.ID]
		ot := &playv1.OutsideCastTarget{CharacterId: t.ID, Name: ch.Name, Npc: !ch.Player, Effect: castEffectToProto[t.Effect]}
		if l.master || l.mine(t.ID) { // RN-20: the hit points and the armor class are the target's own
			ot.Amount, ot.HitPointsBefore, ot.HitPointsAfter, ot.ArmorClass = t.Amount, t.Before, t.After, t.AC
		}
		out.Targets = append(out.Targets, ot)
	}
	return out
}

// filterVitals keeps the vitals the caller may read: the master reads every one, a
// player their own character's.
func filterVitals(m authz.Membership, vitals []*playv1.CharacterVitals) []*playv1.CharacterVitals {
	if m.Role == authz.RoleMaster {
		return vitals
	}
	var out []*playv1.CharacterVitals
	for _, v := range vitals {
		if v != nil && v.GetPlayerUserId() == m.UserID {
			out = append(out, v)
		}
	}
	return out
}

// publishCastsChanged tells the streams the casts changed: the master always, the players
// unless the cast is one only the master reads.
func (s *Service) publishCastsChanged(campaignID string, secret bool) {
	aud := live.Audience{Everyone: true}
	if secret {
		aud = live.Audience{Master: true}
	}
	s.hub.Publish(campaignID, live.Event{
		Audience: aud,
		Message: &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_SpellCastsChanged_{
			SpellCastsChanged: &playv1.WatchGameSessionResponse_SpellCastsChanged{},
		}},
	})
}

// afterCastChange tells the streams what a cast change did (after the commit) and reads
// the cast as the caller sees it. A retry of a change already made tells them too: the
// first call may have died before telling, and a hint only says "read again".
func (s *Service) afterCastChange(ctx context.Context, m authz.Membership, res combatResult, made castOutcome, _ string) (*playv1.OutsideCast, error) {
	pctx, stop := afterCommit(ctx)
	defer stop()
	ev := made.ev
	if res.repeated {
		var err error
		if ev, err = readCastEvent(res.payload); err != nil {
			return nil, s.dbError(ctx, "read the cast", err)
		}
	}
	id := ev.CastID
	if id == "" {
		id = made.row.ID
	}
	if !res.repeated {
		for _, v := range made.vitals {
			s.publishVitals(m.CampaignID, v)
		}
		if made.guard.caster.PlayerUserID != "" && (len(made.created) > 0 || len(made.dismissed) > 0) {
			s.publishCreaturesChanged(m.CampaignID, made.guard.caster.PlayerUserID)
		}
	}
	row, err := s.queries.GetSpellCast(pctx, playdb.GetSpellCastParams{ID: id, CampaignID: m.CampaignID})
	if err != nil {
		return nil, s.dbError(ctx, "read the cast", err)
	}
	if res.kind != "" || res.repeated {
		s.publishCastsChanged(m.CampaignID, row.Secret)
	}
	lens, err := s.lensFor(pctx, m, []playdb.SpellCast{row})
	if err != nil {
		return nil, s.dbError(ctx, "read the cast", err)
	}
	return lens.view(row), nil
}

// castAnswer is the answer of CastSpellOutsideCombat.
func (s *Service) castAnswer(
	ctx context.Context, m authz.Membership, res combatResult, made castOutcome,
	build func(cast *playv1.OutsideCast, vitals []*playv1.CharacterVitals, ev castEvent) *playv1.CastSpellOutsideCombatResponse,
) (*connect.Response[playv1.CastSpellOutsideCombatResponse], error) {
	cast, err := s.afterCastChange(ctx, m, res, made, "")
	if err != nil {
		return nil, err
	}
	ev := made.ev
	if res.repeated {
		ev, _ = readCastEvent(res.payload) // read once already, by afterCastChange
	}
	return connect.NewResponse(build(cast, filterVitals(m, made.vitals), ev)), nil
}

// castAnswerFinish is the answer of FinishCast.
func (s *Service) castAnswerFinish(ctx context.Context, m authz.Membership, res combatResult, made castOutcome) (*connect.Response[playv1.FinishCastResponse], error) {
	cast, err := s.afterCastChange(ctx, m, res, made, "")
	if err != nil {
		return nil, err
	}
	ev := made.ev
	if res.repeated {
		ev, _ = readCastEvent(res.payload)
	}
	return connect.NewResponse(&playv1.FinishCastResponse{
		Cast: cast, Vitals: filterVitals(m, made.vitals), CreatureIds: ev.Created, DismissedCreatureIds: ev.Dismissed, EndedCastIds: ev.Ended,
	}), nil
}

// ListSpellCasts implements playv1connect.CastingServiceHandler.
func (s *Service) ListSpellCasts(
	ctx context.Context,
	req *connect.Request[playv1.ListSpellCastsRequest],
) (*connect.Response[playv1.ListSpellCastsResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	session, err := s.queries.GetOpenGameSession(ctx, m.CampaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNoOpenSession()
	}
	if err != nil {
		return nil, s.dbError(ctx, "find the open session", err)
	}
	var only string
	if raw := req.Msg.GetCharacterId(); raw != "" {
		id, ok := parseID(raw)
		if !ok {
			return nil, errCharacterNotFound()
		}
		only = id
		if m.Role != authz.RoleMaster {
			if err := s.playerMaySee(ctx, m, session.ID, id); err != nil {
				return nil, err
			}
		}
	}
	live, err := s.queries.ListLiveSpellCasts(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list the casts", err)
	}
	logged, err := s.queries.ListSessionSpellCasts(ctx, playdb.ListSessionSpellCastsParams{GameSessionID: session.ID, Limit: maxCastsShown})
	if err != nil {
		return nil, s.dbError(ctx, "list the casts", err)
	}
	lens, err := s.lensFor(ctx, m, append(slices.Clone(live), logged...))
	if err != nil {
		return nil, s.dbError(ctx, "list the casts", err)
	}
	out := &playv1.ListSpellCastsResponse{}
	about := func(r playdb.SpellCast) bool {
		return lens.sees(r) && (only == "" || r.CasterID == only || slices.ContainsFunc(castTargetsOf(r), func(t castTarget) bool { return t.ID == only }))
	}
	for _, r := range live {
		if about(r) {
			out.Active = append(out.Active, lens.view(r))
		}
	}
	for _, r := range logged {
		if about(r) {
			out.Log = append(out.Log, lens.view(r))
		}
	}
	return connect.NewResponse(out), nil
}

// playerMaySee checks a player may read about the character: a player character, or an NPC
// on the stage. Any other is not found, as one that does not exist (RN-10).
func (s *Service) playerMaySee(ctx context.Context, m authz.Membership, sessionID, characterID string) error {
	chars, err := s.roster.CombatCharacters(ctx, nil, m.CampaignID, []string{characterID})
	if err != nil {
		return err
	}
	if len(chars) == 0 || chars[0].CombatOnly {
		return errCharacterNotFound()
	}
	if chars[0].Player {
		return nil
	}
	stage, err := stageOf(ctx, s.queries, sessionID)
	if err != nil {
		return s.dbError(ctx, "list the stage", err)
	}
	if !stage[characterID] {
		return errCharacterNotFound()
	}
	return nil
}

// GetCastingOptions implements playv1connect.CastingServiceHandler.
func (s *Service) GetCastingOptions(
	ctx context.Context,
	req *connect.Request[playv1.GetCastingOptionsRequest],
) (*connect.Response[playv1.GetCastingOptionsResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	session, err := s.queries.GetOpenGameSession(ctx, m.CampaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNoOpenSession()
	}
	if err != nil {
		return nil, s.dbError(ctx, "find the open session", err)
	}
	id, ok := parseID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}
	master := m.Role == authz.RoleMaster
	chars, err := s.roster.CombatCharacters(ctx, nil, m.CampaignID, []string{id})
	if err != nil {
		return nil, err
	}
	if len(chars) == 0 || chars[0].CombatOnly {
		return nil, errCharacterNotFound()
	}
	caster := chars[0]
	switch {
	case caster.Player && !master && caster.PlayerUserID != m.UserID:
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only the character's player or the master may cast for it"))
	case !caster.Player && !master:
		return nil, errCharacterNotFound()
	}
	out := &playv1.GetCastingOptionsResponse{}
	if out.Spells, err = s.roster.CastingOptions(ctx, nil, m.CampaignID, id); err != nil {
		return nil, err
	}
	if out.InCombat, err = s.inCombat(ctx, s.queries, session.ID, id); err != nil {
		return nil, s.dbError(ctx, "find the combat", err)
	}
	if caster.Player {
		v, err := s.vitals.GetVitals(ctx, m.CampaignID, id)
		if err != nil {
			return nil, err
		}
		out.WildShape = noSpellsIn(v, caster.CastsInBeastForm) != nil
	}
	if out.Targets, err = s.castTargetsFor(ctx, m, session.ID); err != nil {
		return nil, err
	}
	live, err := s.queries.ListLiveSpellCastsOfCaster(ctx, id)
	if err != nil {
		return nil, s.dbError(ctx, "list the caster's casts", err)
	}
	lens, err := s.lensFor(ctx, m, live)
	if err != nil {
		return nil, s.dbError(ctx, "list the caster's casts", err)
	}
	for _, r := range live {
		switch {
		case r.Status == castCasting:
			out.Casting = lens.view(r)
		case r.Concentrating:
			out.Concentrating = lens.view(r)
		}
	}
	return connect.NewResponse(out), nil
}

// castTargetsFor lists the characters a caller may pick as targets: the player
// characters of the party and the NPCs on the stage.
func (s *Service) castTargetsFor(ctx context.Context, m authz.Membership, sessionID string) ([]*playv1.CastingTarget, error) {
	party, err := s.roster.CombatParty(ctx, nil, m.CampaignID)
	if err != nil {
		return nil, err
	}
	var out []*playv1.CastingTarget
	for _, p := range party {
		out = append(out, &playv1.CastingTarget{CharacterId: p.ID, Name: p.Name})
	}
	stage, err := stageOf(ctx, s.queries, sessionID)
	if err != nil {
		return nil, s.dbError(ctx, "list the stage", err)
	}
	if len(stage) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(stage))
	for id := range stage {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	npcs, err := s.roster.CombatCharacters(ctx, nil, m.CampaignID, ids)
	if err != nil {
		return nil, err
	}
	for _, n := range npcs {
		if !n.Player && !n.CombatOnly {
			out = append(out, &playv1.CastingTarget{CharacterId: n.ID, Name: n.Name, Npc: true})
		}
	}
	return out, nil
}

var _ = fmt.Sprintf
