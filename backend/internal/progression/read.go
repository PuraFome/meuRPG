package progression

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	progressionv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/progression/progressiondb"
)

// maxLevel is the highest level of a character (RN-12 stops at 20).
const maxLevel = 20

// The history's page sizes.
const (
	defaultPageSize = 20
	maxPageSize     = 50
)

var modeToProto = map[string]progressionv1.XPAwardMode{
	modeEnemies:   progressionv1.XPAwardMode_XP_AWARD_MODE_ENEMIES,
	modeGold:      progressionv1.XPAwardMode_XP_AWARD_MODE_GOLD,
	modeManual:    progressionv1.XPAwardMode_XP_AWARD_MODE_MANUAL,
	modeMilestone: progressionv1.XPAwardMode_XP_AWARD_MODE_MILESTONE,
}

// ListXPAwards implements progressionv1connect.ProgressionServiceHandler. Every
// member reads the whole history (question 50).
func (s *Service) ListXPAwards(
	ctx context.Context,
	req *connect.Request[progressionv1.ListXPAwardsRequest],
) (*connect.Response[progressionv1.ListXPAwardsResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	size := req.Msg.GetPageSize()
	switch {
	case size == 0:
		size = defaultPageSize
	case size < 0 || size > maxPageSize:
		return nil, invalidArgument("page_size", fmt.Errorf("must be 1 to %d", maxPageSize))
	}
	params := progressiondb.ListXPAwardsPageParams{CampaignID: m.CampaignID, PageSize: size + 1} // one more tells whether there is a next page
	if token := req.Msg.GetPageToken(); token != "" {
		at, id, ok := parsePageToken(token)
		if !ok {
			return nil, invalidArgument("page_token", errors.New("is not a token of this list"))
		}
		params.BeforeCreatedAt, params.BeforeID = &at, &id
	}
	// The page, its shares and treasures and the latest award not undone are one
	// moment: an undo or an award between them would show an award that is live
	// next to a "Desfazer" on another one.
	var rows []progressiondb.XpAward
	var data awardData
	var more bool
	err = db.ReadTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		if rows, err = q.ListXPAwardsPage(ctx, params); err != nil {
			return fmt.Errorf("list XP awards: %w", err)
		}
		more = int32(len(rows)) > size //nolint:gosec // at most maxPageSize+1
		if more {
			rows = rows[:size]
		}
		data, err = s.loadAwardData(ctx, q, m, rows)
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "read XP awards", err)
	}
	res := &progressionv1.ListXPAwardsResponse{}
	if more {
		last := rows[len(rows)-1]
		res.NextPageToken = pageToken(last.CreatedAt, last.ID)
	}
	if res.Awards, err = s.viewsOf(ctx, m, data, rows); err != nil {
		return nil, s.dbError(ctx, "read XP awards", err)
	}

	return connect.NewResponse(res), nil
}

// pageToken is the cursor of the history: the time and ID of the last award of
// a page, which is all the keyset query needs. It is opaque to the app.
func pageToken(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(at.UnixMicro(), 10) + ":" + id))
}

func parsePageToken(token string) (time.Time, string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return time.Time{}, "", false
	}
	micro, id, ok := strings.Cut(string(raw), ":")
	if !ok {
		return time.Time{}, "", false
	}
	n, err := strconv.ParseInt(micro, 10, 64)
	if err != nil {
		return time.Time{}, "", false
	}
	u, err := uuid.Parse(id)
	if err != nil {
		return time.Time{}, "", false
	}
	return time.UnixMicro(n).UTC(), u.String(), true
}

// awardData is what the history's entries read from the database besides the
// awards themselves: their shares and treasures, and the latest award not
// undone (for the master's can_undo).
type awardData struct {
	shares    []progressiondb.XpAwardShare
	treasures []progressiondb.XpAwardTreasure
	lastID    string
}

// loadAwardData reads awardData for awards through q, which a caller that read
// the awards in a read transaction passes bound to it, so the awards, their
// shares and the latest award not undone are one moment: an undo that commits
// between them would otherwise show a live award next to a "Desfazer" on an
// older one.
func (s *Service) loadAwardData(ctx context.Context, q *progressiondb.Queries, m authz.Membership, awards []progressiondb.XpAward) (awardData, error) {
	awardIDs := make([]string, 0, len(awards))
	for _, a := range awards {
		awardIDs = append(awardIDs, a.ID)
	}
	var d awardData
	var err error
	if d.shares, err = q.ListXPSharesOfAwards(ctx, awardIDs); err != nil {
		return d, fmt.Errorf("list the shares: %w", err)
	}
	if d.treasures, err = q.ListXPAwardTreasures(ctx, awardIDs); err != nil {
		return d, fmt.Errorf("list the converted treasures: %w", err)
	}
	if m.Role == authz.RoleMaster {
		if d.lastID, err = q.GetLastXPAwardID(ctx, m.CampaignID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return d, fmt.Errorf("find the last award: %w", err)
		}
	}
	return d, nil
}

// awardViews builds the history's entries for awards that were just read or
// written, in the order given. The rest of what they show is read in its own
// read transaction.
func (s *Service) awardViews(ctx context.Context, m authz.Membership, awards ...progressiondb.XpAward) ([]*progressionv1.XPAward, error) {
	var d awardData
	err := db.ReadTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		d, err = s.loadAwardData(ctx, s.queries.WithTx(tx), m, awards)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.viewsOf(ctx, m, d, awards)
}

// viewsOf builds the history's entries for the awards, in the order given:
// who gave each, the characters' names, the combat's name, who undid it. A
// name is the current one. Only the master gets can_undo, on the latest award
// not undone. The data was read in one snapshot (loadAwardData); the names come
// from other modules and are read here, outside it, and so are the marks that
// tell a player which undone reasons to hide.
func (s *Service) viewsOf(ctx context.Context, m authz.Membership, d awardData, awards []progressiondb.XpAward) ([]*progressionv1.XPAward, error) {
	names, err := s.readHistoryNames(ctx, m.CampaignID, awards, d.shares)
	if err != nil {
		return nil, err
	}
	reached, err := s.reachedForPlayer(ctx, m, awards)
	if err != nil {
		return nil, err
	}
	h := historyViews{
		master:      m.Role == authz.RoleMaster,
		lastID:      d.lastID,
		names:       names,
		sharesOf:    sharesByAward(d.shares),
		treasuresOf: treasuresByAward(d.treasures),
		reached:     reached,
	}
	out := make([]*progressionv1.XPAward, 0, len(awards))
	for _, a := range awards {
		out = append(out, h.view(a))
	}
	return out, nil
}

// historyNames is what the history's entries name from other modules, by ID.
type historyNames struct {
	characters map[string]string
	users      map[string]string
	encounters map[string]string
}

// readHistoryNames reads the names of the characters of the shares, of who
// gave and undid each award, and of the awards' combats, in that order.
func (s *Service) readHistoryNames(ctx context.Context, campaignID string, awards []progressiondb.XpAward, shares []progressiondb.XpAwardShare) (historyNames, error) {
	var userIDs, encounterIDs []string
	for _, a := range awards {
		if a.GivenBy != nil {
			userIDs = append(userIDs, *a.GivenBy)
		}
		if a.UndoneBy != nil {
			userIDs = append(userIDs, *a.UndoneBy)
		}
		if a.EncounterID != nil {
			encounterIDs = append(encounterIDs, *a.EncounterID)
		}
	}
	characterIDs := make([]string, 0, len(shares))
	for _, sh := range shares {
		characterIDs = append(characterIDs, sh.CharacterID)
	}
	var n historyNames
	var err error
	if n.characters, err = s.party.Names(ctx, campaignID, characterIDs); err != nil {
		return n, fmt.Errorf("read the characters' names: %w", err)
	}
	if n.users, err = s.profiles.DisplayNames(ctx, userIDs); err != nil {
		return n, fmt.Errorf("read display names: %w", err)
	}
	if n.encounters, err = s.combats.EncounterNames(ctx, campaignID, encounterIDs); err != nil {
		return n, fmt.Errorf("read the combats' names: %w", err)
	}
	return n, nil
}

// reachedForPlayer is, for a player whose page has an undone milestone mark,
// the milestones that are reached now (nil for the master, and when there is
// no such mark). RN-10: the text of a milestone is its reason, and a planned
// milestone is the master's alone. A milestone that was marked and undone is
// planned again, so a player does not read the reason of its undone awards
// until it is reached again.
func (s *Service) reachedForPlayer(ctx context.Context, m authz.Membership, awards []progressiondb.XpAward) (map[string]bool, error) {
	if m.Role == authz.RoleMaster || !slices.ContainsFunc(awards, isUndoneMark) {
		return nil, nil
	}
	live, err := s.queries.ListLiveMilestoneAwards(ctx, m.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("list the marks: %w", err)
	}
	reached := map[string]bool{}
	for _, l := range live {
		reached[deref(l.MilestoneID)] = true
	}
	return reached, nil
}

// isUndoneMark is an undone award of a planned milestone.
func isUndoneMark(a progressiondb.XpAward) bool {
	return a.MilestoneID != nil && a.UndoneAt != nil
}

// sharesByAward groups the shares by award, in the order read.
func sharesByAward(shares []progressiondb.XpAwardShare) map[string][]progressiondb.XpAwardShare {
	byAward := map[string][]progressiondb.XpAwardShare{}
	for _, sh := range shares {
		byAward[sh.AwardID] = append(byAward[sh.AwardID], sh)
	}
	return byAward
}

// treasuresByAward groups the converted treasures by award, in the order read.
func treasuresByAward(treasures []progressiondb.XpAwardTreasure) map[string][]*progressionv1.XPAwardTreasure {
	byAward := map[string][]*progressionv1.XPAwardTreasure{}
	for _, tr := range treasures {
		byAward[tr.AwardID] = append(byAward[tr.AwardID], &progressionv1.XPAwardTreasure{PointId: tr.PointID, ValuePo: tr.ValuePo})
	}
	return byAward
}

// historyViews is what every entry of one read is built from.
type historyViews struct {
	master      bool
	lastID      string // the latest award not undone, for the master's can_undo
	names       historyNames
	sharesOf    map[string][]progressiondb.XpAwardShare
	treasuresOf map[string][]*progressionv1.XPAwardTreasure
	reached     map[string]bool // see reachedForPlayer
}

// view is one award as the caller may read it.
func (h historyViews) view(a progressiondb.XpAward) *progressionv1.XPAward {
	treasures := h.treasuresOf[a.ID]
	v := &progressionv1.XPAward{
		Id:                  a.ID,
		Mode:                modeToProto[a.Mode],
		Reason:              a.Reason,
		GivenByUserId:       deref(a.GivenBy),
		GivenByDisplayName:  h.names.users[deref(a.GivenBy)],
		CreatedAt:           timestamppb.New(a.CreatedAt),
		EncounterId:         deref(a.EncounterID),
		TotalXp:             a.TotalXp,
		Undone:              a.UndoneAt != nil,
		UndoneByDisplayName: h.names.users[deref(a.UndoneBy)],
		CanUndo:             h.master && a.ID == h.lastID && a.UndoneAt == nil,
		TreasureCount:       int32(len(treasures)), //nolint:gosec // at most maxAwardTreasures
	}
	if !h.master && isUndoneMark(a) && !h.reached[*a.MilestoneID] {
		v.Reason = ""
	}
	if h.master {
		v.Treasures = treasures // RN-10: a player gets the count, never which treasures
	}
	if a.EncounterID != nil {
		v.EncounterName = h.names.encounters[*a.EncounterID]
	}
	if a.Gold != nil {
		v.Gold = *a.Gold
	}
	if a.UndoneAt != nil {
		v.UndoneAt = timestamppb.New(*a.UndoneAt)
	}
	for _, sh := range h.sharesOf[a.ID] {
		v.Shares = append(v.Shares, &progressionv1.XPShare{CharacterId: sh.CharacterID, CharacterName: h.names.characters[sh.CharacterID], Xp: sh.Xp})
	}
	return v
}

// GetCampaignExperience implements progressionv1connect.ProgressionServiceHandler.
func (s *Service) GetCampaignExperience(
	ctx context.Context,
	req *connect.Request[progressionv1.GetCampaignExperienceRequest],
) (*connect.Response[progressionv1.GetCampaignExperienceResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	mode, err := s.campaigns.CampaignXPMode(ctx, nil, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read the campaign's XP mode", err)
	}
	party, err := s.party.Party(ctx, nil, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read the party", err)
	}
	marks, err := s.queries.ListMilestoneMarks(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read the milestone marks", err)
	}
	markOf := make(map[string]int32, len(marks))
	for _, mk := range marks {
		markOf[mk.CharacterID] = mk.Level
	}
	var userIDs []string
	for _, p := range party {
		if p.PlayerUserID != "" {
			userIDs = append(userIDs, p.PlayerUserID)
		}
	}
	userNames, err := s.profiles.DisplayNames(ctx, userIDs)
	if err != nil {
		return nil, s.dbError(ctx, "read display names", err)
	}
	res := &progressionv1.GetCampaignExperienceResponse{XpMode: mode}
	for _, p := range party {
		reason := levelUpReason(mode, p.Level, p.XP, p.NextLevelXP, markOf[p.ID])
		res.Characters = append(res.Characters, &progressionv1.CharacterExperience{
			CharacterId: p.ID, Name: p.Name, PlayerUserId: p.PlayerUserID, PlayerDisplayName: userNames[p.PlayerUserID],
			Level: p.Level, ExperiencePoints: p.XP, NextLevelXp: p.NextLevelXP,
			CanLevelUp: reason != charactersv1.LevelUpReason_LEVEL_UP_REASON_UNSPECIFIED, LevelUpReason: reason,
		})
	}
	return connect.NewResponse(res), nil
}

// LevelUpReason implements characters.LevelUps: why the character can go up a
// level, from the sheet's numbers and the campaign's mode and milestones
// (RN-12).
func (s *Service) LevelUpReason(ctx context.Context, tx pgx.Tx, campaignID, characterID string, level, xp, nextLevelXP int32) (charactersv1.LevelUpReason, error) {
	mode, err := s.campaigns.CampaignXPMode(ctx, tx, campaignID)
	if err != nil {
		return charactersv1.LevelUpReason_LEVEL_UP_REASON_UNSPECIFIED, fmt.Errorf("read the campaign's XP mode: %w", err)
	}
	var mark int32
	if mode == campaignsv1.XpMode_XP_MODE_MILESTONES {
		if mark, err = s.queriesIn(tx).GetMilestoneMark(ctx, progressiondb.GetMilestoneMarkParams{CampaignID: campaignID, CharacterID: characterID}); err != nil {
			return charactersv1.LevelUpReason_LEVEL_UP_REASON_UNSPECIFIED, fmt.Errorf("read the milestone mark: %w", err)
		}
	}
	return levelUpReason(mode, level, xp, nextLevelXP, mark), nil
}

// levelUpReason is the rule of "Pode subir de nível" (RN-12, questions 48 and
// 49). In a campaign that counts XP, the sheet's XP reaching the next level's.
// In a milestones campaign, a milestone that marked the character and a level
// on the sheet that has not gone past the one it had then (markLevel, 0 for no
// mark). Nobody goes up from level 20.
func levelUpReason(mode campaignsv1.XpMode, level, xp, nextLevelXP, markLevel int32) charactersv1.LevelUpReason {
	switch {
	case level >= maxLevel || nextLevelXP <= 0:
		return charactersv1.LevelUpReason_LEVEL_UP_REASON_UNSPECIFIED
	case mode == campaignsv1.XpMode_XP_MODE_MILESTONES:
		if markLevel > 0 && level <= markLevel {
			return charactersv1.LevelUpReason_LEVEL_UP_REASON_MILESTONE
		}
	case xp >= nextLevelXP:
		return charactersv1.LevelUpReason_LEVEL_UP_REASON_XP
	}
	return charactersv1.LevelUpReason_LEVEL_UP_REASON_UNSPECIFIED
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
