package characters

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/progression/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Experience (MR-016, RN-09, RN-12). The XP of a player character is a number
// on its sheet (FullSheet.experience_points), so the sheet's owner, this
// module, keeps it. Package progression gives it and takes it back through
// its own interface (progression.Party), which this Service implements with
// Party, Names and AddExperience, like play's VitalsKeeper: the methods take
// no caller, they run after progression's own authorization check, and the
// master's notes never go through them (RN-11).
//
// The other way round, GetCharacter asks package progression whether the
// character can go up a level (LevelUps): the milestones are its tables.

// LevelUps tells whether a character can go up a level (RN-12). The
// progression module implements it, because the campaign's XP mode and the
// milestones' marks are its own; cmd/api connects it with SetLevelUps.
type LevelUps interface {
	// LevelUpReason says why the character can go up a level (XP, or a
	// MILESTONE), or returns LEVEL_UP_REASON_UNSPECIFIED when it cannot. level,
	// xp and nextLevelXP are the sheet's numbers. It reads inside tx when the
	// caller has one (nil: the pool).
	LevelUpReason(ctx context.Context, tx pgx.Tx, campaignID, characterID string, level, xp, nextLevelXP int32) (charactersv1.LevelUpReason, error)
}

// SetLevelUps connects the progression module, which needs this service to
// give XP, so it cannot be a Config field (the same arrangement as
// campaigns' SetCharacters). Without it, no character can level up.
func (s *Service) SetLevelUps(l LevelUps) { s.levelUps = l }

// fillLevelUp sets can_level_up on a character the caller may see: the master
// and the owning player get it, for a living player character only.
func (s *Service) fillLevelUp(ctx context.Context, c *charactersv1.Character, row charactersdb.Character, m authz.Membership) error {
	owner := !m.Pending && row.PlayerUserID != nil && *row.PlayerUserID == m.UserID
	if s.levelUps == nil || row.Kind != kindPlayer || row.Status != statusActive || (!isMaster(m) && !owner) {
		return nil
	}
	full := c.GetSheet().GetFull()
	if full == nil || c.GetDerived() == nil {
		return nil
	}
	reason, err := s.levelUps.LevelUpReason(ctx, nil, m.CampaignID, row.ID, c.GetDerived().GetTotalLevel(), full.GetExperiencePoints(), c.GetDerived().GetNextLevelXp())
	if err != nil {
		return wrap("check the level up", err)
	}
	c.LevelUpReason = reason
	c.CanLevelUp = reason != charactersv1.LevelUpReason_LEVEL_UP_REASON_UNSPECIFIED
	return nil
}

// Party returns the campaign's living, active player characters, oldest first,
// with the numbers XP needs from each sheet.
func (s *Service) Party(ctx context.Context, campaignID string) ([]link.Member, error) {
	rows, err := s.queries.ListCombatParty(ctx, campaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list the party", err)
	}
	out := make([]link.Member, 0, len(rows))
	for _, row := range rows {
		sheet, err := loadSheet(row.ID, row.Sheet)
		if err != nil {
			return nil, s.dbError(ctx, "list the party", err)
		}
		full := sheet.GetFull()
		if full == nil {
			continue // never: a player character has a full sheet
		}
		d := rules.Derive(buildOf(full), s.rules)
		out = append(out, link.Member{
			ID: row.ID, Name: row.Name, PlayerUserID: deref(row.PlayerUserID),
			Level: i32(d.TotalLevel), XP: full.GetExperiencePoints(), NextLevelXP: i32(d.NextLevelXP),
		})
	}
	return out, nil
}

// Names returns the names of those of ids that are characters of the campaign,
// by ID, whatever their kind or status.
func (s *Service) Names(ctx context.Context, campaignID string, ids []string) (map[string]string, error) {
	valid := make([]string, 0, len(ids))
	for _, id := range ids {
		if id, ok := parseUUID(id); ok {
			valid = append(valid, id)
		}
	}
	out := make(map[string]string, len(valid))
	if len(valid) == 0 {
		return out, nil
	}
	rows, err := s.queries.ListCharacterNames(ctx, charactersdb.ListCharacterNamesParams{CampaignID: campaignID, Ids: valid})
	if err != nil {
		return nil, s.dbError(ctx, "read character names", err)
	}
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out, nil
}

// maxExperience is the largest XP a sheet holds (maxExperiencePoints).
const maxExperience = int32(maxExperiencePoints)

// AddExperience adds delta (negative to take back) to a player character's XP
// inside tx, keeping it between 0 and 1,000,000, and returns the XP before and
// after. It works for a living or a dead player character of the campaign
// (taking back an award never skips a character that died since) and answers
// `not_found` for anything else. The sheet's lock (RN-01) does not stop it: it
// is the master's act, not the player's edit. The sheet's revision goes up, so
// an edit the player started before it is refused as stale instead of
// overwriting the XP.
func (s *Service) AddExperience(ctx context.Context, tx pgx.Tx, campaignID, characterID string, delta int32, at time.Time) (before, after int32, err error) {
	q := s.queries.WithTx(tx)
	row, err := q.GetCharacterForUpdate(ctx, charactersdb.GetCharacterForUpdateParams{CampaignID: campaignID, ID: characterID})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, errCharacterNotFound()
	}
	if err != nil {
		return 0, 0, wrap("lock a character", err)
	}
	if row.Kind != kindPlayer || row.Status == statusPending {
		return 0, 0, errCharacterNotFound()
	}
	sheet, err := loadSheet(row.ID, row.Sheet)
	if err != nil {
		return 0, 0, err
	}
	full := sheet.GetFull()
	if full == nil {
		return 0, 0, fmt.Errorf("%w: the sheet of character %s is not a full sheet", errCorruptDocument, row.ID)
	}
	before = full.GetExperiencePoints()
	after = int32(max(0, min(int64(maxExperience), int64(before)+int64(delta))))
	if after == before {
		return before, after, nil
	}
	full.ExperiencePoints = after
	doc, err := storeJSON.Marshal(sheet)
	if err != nil {
		return 0, 0, fmt.Errorf("encode a sheet: %w", err)
	}
	if _, err := q.UpdateCharacterSheet(ctx, charactersdb.UpdateCharacterSheetParams{
		CampaignID: campaignID, ID: characterID, Revision: row.Revision, Name: row.Name, Sheet: doc, Now: at,
	}); err != nil {
		return 0, 0, wrap("update the XP", err)
	}
	return before, after, nil
}
