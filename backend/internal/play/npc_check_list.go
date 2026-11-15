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
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/cutting"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// npcCheckOffer is the question of a held NPC check as the caller reads it: the master reads the roll and the bards asked; a player reads
// only the question of the bard asked now, if it is their own, with no number of the creature's and no name the bard's player may not
// know. Nobody else reads anything. A check whose session is over, or while a combat is open, is not listed.
func (l *heldList) npcCheckOffer(ctx context.Context, h heldRoll) (*playv1.HeldRollOffer, error) {
	s := l.s
	session, err := s.queries.GetOpenGameSession(ctx, h.CampaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, s.dbError(ctx, "find the open session", err)
	}
	if h.CreatedAt.Before(session.StartedAt) {
		return nil, nil
	}
	if enc, err := s.openEncounterOf(ctx, h.CampaignID); err != nil || enc != nil {
		return nil, err
	}
	asks, err := s.queries.ListNpcCheckAsks(ctx, h.ID)
	if err != nil {
		return nil, s.dbError(ctx, "list the asks of the check", err)
	}
	hold := playdb.InspirationHold{ID: h.ID, CampaignID: h.CampaignID, IdempotencyKey: h.Key}
	ev, err := npcCheckEventOf(ctx, s.queries, session, hold)
	if err != nil {
		return nil, s.dbError(ctx, "find the held check", err)
	}
	offer := &playv1.HeldRollOffer{HoldId: h.ID, Kind: playv1.HeldRollKind_HELD_ROLL_KIND_NPC_CHECK, CreatedAt: timestamppb.New(h.CreatedAt)}
	if l.m.Role == authz.RoleMaster {
		offer.RollerLabel, offer.D20 = ev.Name, heldD20Of(h)
		offer.Question = &playv1.HeldRollOffer_CuttingWords{CuttingWords: &playv1.CuttingWordsQuestion{Asks: ev.withAsks(asks, false).read(s, false).GetAsks()}}
		return offer, nil
	}
	cur := cutting.Current(askStates(asks))
	if cur < 0 {
		return nil, nil
	}
	mine, err := s.isMyCharacter(ctx, l.m, asks[cur].BardCharacterID)
	if err != nil || !mine {
		return nil, err
	}
	vit, err := s.vitals.GetVitals(ctx, h.CampaignID, asks[cur].BardCharacterID)
	if err != nil {
		return nil, s.dbError(ctx, "read the bard's vitals", err)
	}
	offer.RollerLabel = cutting.Label(ev.Name, asks[cur].CreatureSeen)
	offer.Question = &playv1.HeldRollOffer_CuttingWords{CuttingWords: &playv1.CuttingWordsQuestion{Sides: asks[cur].Sides, UsesLeft: bardUsesLeft(vit)}}
	return offer, nil
}

// ListNpcCheckPicker implements playv1connect.PlayServiceHandler.
func (s *Service) ListNpcCheckPicker(ctx context.Context, req *connect.Request[playv1.ListNpcCheckPickerRequest]) (*connect.Response[playv1.ListNpcCheckPickerResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	out := &playv1.ListNpcCheckPickerResponse{}
	seen := map[string]bool{}
	measures := func(string) bool { return false }
	add := func(c link.Character, source playv1.NpcCheckSource, hidden bool) error {
		if seen[c.ID] || c.Player {
			return nil
		}
		seen[c.ID] = true
		cand, err := s.npcCandidate(ctx, m.CampaignID, c, source, hidden)
		if err == nil {
			cand.MapMeasures = measures(c.ID)
			out.Candidates = append(out.Candidates, cand)
		}
		return err
	}
	session, err := s.queries.GetOpenGameSession(ctx, m.CampaignID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, s.dbError(ctx, "find the open session", err)
	}
	if err == nil {
		if measures, err = s.mapMeasures(ctx, session); err != nil {
			return nil, s.dbError(ctx, "read the placement of the bards", err)
		}
		if err := s.pickStageAndTokens(ctx, m.CampaignID, session, add); err != nil {
			return nil, s.dbError(ctx, "list the creatures", err)
		}
	}
	if err := s.pickMonster(ctx, m.CampaignID, req.Msg.GetMonsterKey(), out); err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

// mapMeasures says which creatures the session's current map measures against a bard: the creature has a token on it, and so has a
// bard that would be asked (see askedBardOf, without the sight). Then the server measures the 60 feet and the sight and the master's
// box is not read.
func (s *Service) mapMeasures(ctx context.Context, session playdb.GameSession) (func(string) bool, error) {
	var tokens map[string]cutting.Token
	var bards []string
	err := db.ReadTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if tokens, err = s.mapTokensOf(ctx, tx, session); err != nil || len(tokens) == 0 {
			return err
		}
		party, err := s.roster.CombatParty(ctx, tx, session.CampaignID)
		if err != nil {
			return err
		}
		for _, c := range party {
			if _, on := tokens[c.ID]; !on || !c.Player || c.Reserved || c.PlayerUserID == "" {
				continue
			}
			if _, asked, err := s.bardOf(ctx, tx, session.CampaignID, c); err != nil {
				return err
			} else if asked {
				bards = append(bards, c.ID)
			}
		}
		return nil
	})
	return func(npc string) bool {
		return slices.ContainsFunc(bards, cutting.Place{Tokens: tokens, NPC: npc}.Measures)
	}, err
}

// pickMonster adds the SRD creature the master asked for, when the master asked for one.
func (s *Service) pickMonster(ctx context.Context, campaignID, key string, out *playv1.ListNpcCheckPickerResponse) error {
	if key == "" {
		return nil
	}
	cand, ok, err := s.monsterCandidate(ctx, campaignID, key)
	if err != nil {
		return s.dbError(ctx, "read the creature", err)
	}
	if !ok {
		return connect.NewError(connect.CodeNotFound, errors.New("creature not found"))
	}
	out.Candidates = append(out.Candidates, cand)
	return nil
}

// pickStageAndTokens offers the NPCs on the stage, the hidden ones marked, and those with a token on the session's current map.
func (s *Service) pickStageAndTokens(ctx context.Context, campaignID string, session playdb.GameSession, add func(link.Character, playv1.NpcCheckSource, bool) error) error {
	rows, err := s.queries.ListStage(ctx, session.ID)
	if err != nil {
		return fmt.Errorf("list the stage: %w", err)
	}
	chars, err := s.stageCharacters(ctx, campaignID, rows)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if c, ok := chars[r.CharacterID]; ok && !c.CombatOnly {
			if err := add(c, playv1.NpcCheckSource_NPC_CHECK_SOURCE_STAGE, r.Hidden); err != nil {
				return err
			}
		}
	}
	if session.CurrentMapID == nil {
		return nil
	}
	return s.pickTokens(ctx, campaignID, *session.CurrentMapID, add)
}

// pickTokens offers the NPCs with a token on the map.
func (s *Service) pickTokens(ctx context.Context, campaignID, mapID string, add func(link.Character, playv1.NpcCheckSource, bool) error) error {
	tokens, err := s.maps.MapTokens(ctx, nil, mapID)
	if err != nil {
		return fmt.Errorf("read the tokens: %w", err)
	}
	var ids []string
	hidden := map[string]bool{}
	for _, t := range tokens {
		if t.CharacterID != "" {
			ids, hidden[t.CharacterID] = append(ids, t.CharacterID), t.Hidden
		}
	}
	living, err := s.roster.CombatCharacters(ctx, nil, campaignID, ids)
	if err != nil {
		return err
	}
	for _, c := range living {
		if !c.CombatOnly {
			if err := add(c, playv1.NpcCheckSource_NPC_CHECK_SOURCE_TOKEN, hidden[c.ID]); err != nil {
				return err
			}
		}
	}
	return nil
}

// npcCandidate is an NPC of the table with its checks and the bonus the master reads.
func (s *Service) npcCandidate(ctx context.Context, campaignID string, c link.Character, source playv1.NpcCheckSource, hidden bool) (*playv1.NpcCheckCandidate, error) {
	cand := &playv1.NpcCheckCandidate{CharacterId: c.ID, MonsterKey: c.MonsterKey, Name: c.Name, Source: source, Hidden: hidden}
	return cand, s.fillChecks(ctx, campaignID, c, cand)
}

// monsterCandidate reads an SRD creature, which has no NPC of the table until it is rolled for.
func (s *Service) monsterCandidate(ctx context.Context, campaignID, key string) (*playv1.NpcCheckCandidate, bool, error) {
	content, err := s.roster.RulesContent(ctx, nil, campaignID)
	if err != nil {
		return nil, false, err
	}
	cr, ok := content.CreatureByKey(key)
	if !ok {
		return nil, false, nil
	}
	cand := &playv1.NpcCheckCandidate{MonsterKey: key, Name: cr.NamePT, Source: playv1.NpcCheckSource_NPC_CHECK_SOURCE_MONSTER}
	return cand, true, s.fillChecks(ctx, campaignID, link.Character{MonsterKey: key}, cand)
}

// fillChecks lists the creature's checks with what it adds to each.
func (s *Service) fillChecks(ctx context.Context, campaignID string, c link.Character, cand *playv1.NpcCheckCandidate) error {
	for _, key := range cutting.CheckKeys() {
		o, err := s.npcCheckNumbers(ctx, nil, campaignID, c, key)
		if err != nil {
			return err
		}
		cand.Checks = append(cand.Checks, &playv1.NpcCheckOption{Key: key, Name: s.roster.SceneCheckName(key), Bonus: clamp32(o.Bonus, -heldBonusLimit, heldBonusLimit), Known: o.Known})
	}
	cand.HasStatBlock = c.MonsterKey != "" || slices.ContainsFunc(cand.Checks, func(o *playv1.NpcCheckOption) bool { return o.Known })
	return nil
}
