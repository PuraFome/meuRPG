package characters

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The database's text values for kind and status (characters_kind_valid,
// characters_status_valid).
const (
	kindPlayer    = "player"
	statusActive  = "active"
	statusDead    = "dead"
	statusPending = "pending"
)

// Conversions between the database's kinds and the API's.
var (
	kindToDB = map[charactersv1.CharacterKind]string{
		charactersv1.CharacterKind_CHARACTER_KIND_PLAYER: kindPlayer,
		charactersv1.CharacterKind_CHARACTER_KIND_ENEMY:  "enemy",
		charactersv1.CharacterKind_CHARACTER_KIND_BOSS:   "boss",
		charactersv1.CharacterKind_CHARACTER_KIND_MINION: "minion",
		charactersv1.CharacterKind_CHARACTER_KIND_STORY:  "story",
	}
	kindFromDB = map[string]charactersv1.CharacterKind{
		kindPlayer: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER,
		"enemy":    charactersv1.CharacterKind_CHARACTER_KIND_ENEMY,
		"boss":     charactersv1.CharacterKind_CHARACTER_KIND_BOSS,
		"minion":   charactersv1.CharacterKind_CHARACTER_KIND_MINION,
		"story":    charactersv1.CharacterKind_CHARACTER_KIND_STORY,
	}
)

// characterState computes a character's state from its columns
// (docs/produto/regras.md, "Ciclo de vida da ficha"). NPCs are never locked
// nor dead, so they are always drafts.
func characterState(status string, sheetLockedAt *time.Time) charactersv1.CharacterState {
	switch {
	case status == statusDead:
		return charactersv1.CharacterState_CHARACTER_STATE_DEAD
	case status == statusPending:
		return charactersv1.CharacterState_CHARACTER_STATE_PENDING
	case sheetLockedAt != nil:
		return charactersv1.CharacterState_CHARACTER_STATE_LOCKED
	default:
		return charactersv1.CharacterState_CHARACTER_STATE_DRAFT
	}
}

// isMaster says whether the caller is the campaign's master.
func isMaster(m authz.Membership) bool { return m.Role == authz.RoleMaster }

// canSee says whether the caller may see a character of their campaign: the
// master sees every one; a player, only their own player characters. Every
// other character is "not found" to them.
func canSee(m authz.Membership, kind string, playerUserID *string) bool {
	if isMaster(m) {
		return true
	}
	return kind == kindPlayer && playerUserID != nil && *playerUserID == m.UserID
}

// playerEditsSheet says whether the owning player may still change the
// sheet (RN-01): while it is a draft, or while it waits for the master's
// approval (RN-15, MR-024).
func playerEditsSheet(state charactersv1.CharacterState) bool {
	return state == charactersv1.CharacterState_CHARACTER_STATE_DRAFT ||
		state == charactersv1.CharacterState_CHARACTER_STATE_PENDING
}

// playerEditsStory says whether the owning player may change the story:
// whenever they may change the sheet, and afterwards while the master
// allows it (story_editing_allowed).
func playerEditsStory(state charactersv1.CharacterState, allowed bool) bool {
	return playerEditsSheet(state) || allowed
}

// blockedReason says why the owning player may not change the sheet now.
func blockedReason(state charactersv1.CharacterState) charactersv1.CharacterBlockedReason {
	if state == charactersv1.CharacterState_CHARACTER_STATE_DEAD {
		return charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CHARACTER_DEAD
	}
	return charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_SHEET_LOCKED
}

// characterToProto builds the Character a member sees: m is the caller,
// who may see the row (canSee), and playerDisplayName the owning player's
// display name. A full sheet gets its derived numbers.
func (s *Service) characterToProto(row charactersdb.Character, m authz.Membership, playerDisplayName string) (*charactersv1.Character, error) {
	sheet, err := loadSheet(row.ID, row.Sheet)
	if err != nil {
		return nil, err
	}
	story, err := loadStory(row.ID, row.Story)
	if err != nil {
		return nil, err
	}
	state := characterState(row.Status, row.SheetLockedAt)
	master := isMaster(m)
	player := row.Kind == kindPlayer
	c := &charactersv1.Character{
		Id:                   row.ID,
		CampaignId:           deref(row.CampaignID),
		Kind:                 kindFromDB[row.Kind],
		State:                state,
		Name:                 row.Name,
		PlayerUserId:         deref(row.PlayerUserID),
		PlayerDisplayName:    playerDisplayName,
		Sheet:                sheet,
		Story:                story,
		Revision:             row.Revision,
		SheetLockedAt:        timestamp(row.SheetLockedAt),
		DiedAt:               timestamp(row.DiedAt),
		CreatedAt:            timestamppb.New(row.CreatedAt),
		UpdatedAt:            timestamppb.New(row.UpdatedAt),
		CanEdit:              master || playerEditsSheet(state),
		CanEditStory:         master || playerEditsStory(state, row.StoryEditingAllowed),
		CanMarkDead:          master && player && state != charactersv1.CharacterState_CHARACTER_STATE_DEAD,
		CanAccessMasterNotes: master,
		StoryEditingAllowed:  row.StoryEditingAllowed,
		CanSetStoryEditing:   master && player,
	}
	if full := sheet.GetFull(); full != nil {
		c.Derived = derivedToProto(rules.Derive(buildOf(full), s.rules))
	}
	return c, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func timestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
