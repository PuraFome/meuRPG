package characters

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
)

// Every handler starts with one explicit check: authz.RequireCampaignMember
// when any member may call it, authz.RequireCampaignRole when only the
// master may. The check's error is already the right Connect error, so
// handlers return it as is. What a player may see and change is decided
// next, from the character's row: canSee, playerEditsSheet and
// playerEditsStory (access.go).

// CreateCharacter implements charactersv1connect.CharacterServiceHandler.
func (s *Service) CreateCharacter(
	ctx context.Context,
	req *connect.Request[charactersv1.CreateCharacterRequest],
) (*connect.Response[charactersv1.CreateCharacterResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	kind, ok := kindToDB[req.Msg.GetKind()]
	if !ok {
		return nil, invalidArgument(fieldErr("kind", "is required"))
	}
	// A player creates their own character; the master creates NPCs
	// (RN-04). A master is not a player of their own campaign.
	switch {
	case kind == kindPlayer && isMaster(m):
		return nil, errPermission("only a player creates a player character; the master creates NPCs")
	case kind != kindPlayer && !isMaster(m):
		return nil, errPermission("only the campaign's master creates NPCs")
	}

	name, err := names.Clean(req.Msg.GetName(), MaxNameLength)
	if err != nil {
		return nil, invalidArgument(&fieldError{field: "name", err: err})
	}
	sheet, err := s.checkSheet(req.Msg.GetSheet())
	if err == nil {
		err = checkSheetKind(kind, sheet)
	}
	if err != nil {
		return nil, invalidArgument(err)
	}
	story, err := checkStory(req.Msg.GetStory())
	if err != nil {
		return nil, invalidArgument(err)
	}
	params := charactersdb.InsertCharacterParams{CampaignID: m.CampaignID, Kind: kind, Name: name, Now: s.now()}
	if params.Sheet, err = storeJSON.Marshal(sheet); err != nil {
		return nil, s.dbError(ctx, "encode a sheet", err)
	}
	if params.Story, err = storeJSON.Marshal(story); err != nil {
		return nil, s.dbError(ctx, "encode a story", err)
	}
	if kind == kindPlayer {
		params.PlayerUserID = &m.UserID
	} else {
		params.MasterUserID = &m.UserID // the NPC is the master's (RN-04)
	}

	var row charactersdb.Character
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		if kind == kindPlayer {
			// RN-03: one living character per player per campaign. This
			// check gives the clear error; the unique index is what makes it
			// hold when two calls race (below).
			living, err := q.GetLivingPlayerCharacterID(ctx, charactersdb.GetLivingPlayerCharacterIDParams{
				CampaignID: m.CampaignID, PlayerUserID: m.UserID,
			})
			switch {
			case err == nil:
				return errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_LIVING_CHARACTER_EXISTS, living)
			case !errors.Is(err, pgx.ErrNoRows):
				return wrap("find the living character", err)
			}
		}
		var err error
		row, err = q.InsertCharacter(ctx, params)
		if err != nil {
			return wrap("insert character", err)
		}
		return nil
	})
	if isUniqueViolation(err, "characters_one_living_player_character") {
		// Another CreateCharacter of the same player won the race: answer
		// as the check above would have.
		living, lookupErr := s.queries.GetLivingPlayerCharacterID(ctx, charactersdb.GetLivingPlayerCharacterIDParams{
			CampaignID: m.CampaignID, PlayerUserID: m.UserID,
		})
		if lookupErr != nil {
			living = "" // it died in between; the reason still holds
		}
		return nil, errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_LIVING_CHARACTER_EXISTS, living)
	}
	if err != nil {
		return nil, s.dbError(ctx, "create a character", err)
	}
	c, err := s.character(ctx, row, m)
	if err != nil {
		return nil, s.dbError(ctx, "read a new character", err)
	}
	return connect.NewResponse(&charactersv1.CreateCharacterResponse{Character: c}), nil
}

// GetCharacter implements charactersv1connect.CharacterServiceHandler.
func (s *Service) GetCharacter(
	ctx context.Context,
	req *connect.Request[charactersv1.GetCharacterRequest],
) (*connect.Response[charactersv1.GetCharacterResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	id, ok := parseUUID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}
	row, err := s.queries.GetCharacter(ctx, charactersdb.GetCharacterParams{CampaignID: m.CampaignID, ID: id})
	if err != nil {
		return nil, s.dbError(ctx, "get a character", err)
	}
	if !canSee(m, row.Kind, row.PlayerUserID) {
		return nil, errCharacterNotFound()
	}
	c, err := s.character(ctx, row, m)
	if err != nil {
		return nil, s.dbError(ctx, "read a character", err)
	}
	return connect.NewResponse(&charactersv1.GetCharacterResponse{Character: c}), nil
}

// ListCharacters implements charactersv1connect.CharacterServiceHandler.
func (s *Service) ListCharacters(
	ctx context.Context,
	req *connect.Request[charactersv1.ListCharactersRequest],
) (*connect.Response[charactersv1.ListCharactersResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	params := charactersdb.ListCharactersParams{CampaignID: m.CampaignID}
	if !isMaster(m) {
		params.PlayerUserID = &m.UserID // a player sees only their own
	}
	rows, err := s.queries.ListCharacters(ctx, params)
	if err != nil {
		return nil, s.dbError(ctx, "list characters", err)
	}

	var playerIDs []string
	for _, row := range rows {
		if row.PlayerUserID != nil {
			playerIDs = append(playerIDs, *row.PlayerUserID)
		}
	}
	displayNames, err := s.displayNames(ctx, playerIDs)
	if err != nil {
		return nil, s.dbError(ctx, "read display names", err)
	}

	res := &charactersv1.ListCharactersResponse{}
	for _, row := range rows {
		summary := &charactersv1.CharacterSummary{
			Id:                row.ID,
			Kind:              kindFromDB[row.Kind],
			State:             characterState(row.Status, row.SheetLockedAt),
			Name:              row.Name,
			PlayerUserId:      deref(row.PlayerUserID),
			PlayerDisplayName: displayNames[deref(row.PlayerUserID)],
			CreatedAt:         timestamppb.New(row.CreatedAt),
		}
		sheet, err := loadSheet(row.ID, row.Sheet)
		if err != nil {
			return nil, s.dbError(ctx, "list characters", err)
		}
		if full := sheet.GetFull(); full != nil {
			labels := s.rules.Summary(buildOf(full))
			summary.ClassSummary = labels.ClassSummaryPT
			summary.RaceNamePt = labels.RaceNamePT
		}
		res.Characters = append(res.Characters, summary)
	}
	return connect.NewResponse(res), nil
}

// UpdateCharacter implements charactersv1connect.CharacterServiceHandler.
func (s *Service) UpdateCharacter(
	ctx context.Context,
	req *connect.Request[charactersv1.UpdateCharacterRequest],
) (*connect.Response[charactersv1.UpdateCharacterResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	id, ok := parseUUID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}
	revision := req.Msg.GetRevision()
	if revision < 1 {
		return nil, invalidArgument(fieldErr("revision", "must be at least 1"))
	}
	name, err := names.Clean(req.Msg.GetName(), MaxNameLength)
	if err != nil {
		return nil, invalidArgument(&fieldError{field: "name", err: err})
	}
	sheet, err := s.checkSheet(req.Msg.GetSheet())
	if err != nil {
		return nil, invalidArgument(err)
	}
	sheetDoc, err := storeJSON.Marshal(sheet)
	if err != nil {
		return nil, s.dbError(ctx, "encode a sheet", err)
	}

	var row charactersdb.Character
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		current, err := visibleForUpdate(ctx, q, m, id)
		if err != nil {
			return err
		}
		// RN-01: after a game session starts, only the master edits the
		// sheet. This comes before the revision: retrying would not help.
		if state := characterState(current.Status, current.SheetLockedAt); !isMaster(m) && !playerEditsSheet(state) {
			return errBlocked(blockedReason(state), current.ID)
		}
		if err := checkSheetKind(current.Kind, sheet); err != nil {
			return invalidArgument(err)
		}
		if current.Revision != revision {
			return errStaleRevision()
		}
		row, err = q.UpdateCharacterSheet(ctx, charactersdb.UpdateCharacterSheetParams{
			CampaignID: m.CampaignID, ID: id, Revision: revision, Name: name, Sheet: sheetDoc, Now: s.now(),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errStaleRevision()
		}
		if err != nil {
			return wrap("update sheet", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "update a character", err)
	}
	c, err := s.character(ctx, row, m)
	if err != nil {
		return nil, s.dbError(ctx, "read an updated character", err)
	}
	return connect.NewResponse(&charactersv1.UpdateCharacterResponse{Character: c}), nil
}

// UpdateCharacterStory implements charactersv1connect.CharacterServiceHandler.
func (s *Service) UpdateCharacterStory(
	ctx context.Context,
	req *connect.Request[charactersv1.UpdateCharacterStoryRequest],
) (*connect.Response[charactersv1.UpdateCharacterStoryResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	id, ok := parseUUID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}
	revision := req.Msg.GetRevision()
	if revision < 1 {
		return nil, invalidArgument(fieldErr("revision", "must be at least 1"))
	}
	story, err := checkStory(req.Msg.GetStory())
	if err != nil {
		return nil, invalidArgument(err)
	}
	storyDoc, err := storeJSON.Marshal(story)
	if err != nil {
		return nil, s.dbError(ctx, "encode a story", err)
	}

	var row charactersdb.Character
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		current, err := visibleForUpdate(ctx, q, m, id)
		if err != nil {
			return err
		}
		// The story's own lock: the player edits it while the character is
		// a draft, and afterwards only while the master allows it (RN-01).
		state := characterState(current.Status, current.SheetLockedAt)
		if !isMaster(m) && !playerEditsStory(state, current.StoryEditingAllowed) {
			return errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_STORY_LOCKED, current.ID)
		}
		if current.Revision != revision {
			return errStaleRevision()
		}
		row, err = q.UpdateCharacterStory(ctx, charactersdb.UpdateCharacterStoryParams{
			CampaignID: m.CampaignID, ID: id, Revision: revision, Story: storyDoc, Now: s.now(),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errStaleRevision()
		}
		if err != nil {
			return wrap("update story", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "update a story", err)
	}
	c, err := s.character(ctx, row, m)
	if err != nil {
		return nil, s.dbError(ctx, "read an updated character", err)
	}
	return connect.NewResponse(&charactersv1.UpdateCharacterStoryResponse{Character: c}), nil
}

// SetStoryEditing implements charactersv1connect.CharacterServiceHandler.
func (s *Service) SetStoryEditing(
	ctx context.Context,
	req *connect.Request[charactersv1.SetStoryEditingRequest],
) (*connect.Response[charactersv1.SetStoryEditingResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	id, ok := parseUUID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}
	allowed := req.Msg.GetAllowed()

	var row charactersdb.Character
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		row, err = visibleForUpdate(ctx, q, m, id)
		if err != nil {
			return err
		}
		if row.Kind != kindPlayer {
			return invalidArgument(fieldErr("character_id", "is an NPC: the master always edits an NPC's story"))
		}
		if row.StoryEditingAllowed == allowed {
			return nil // already so: nothing to change
		}
		row, err = q.SetStoryEditing(ctx, charactersdb.SetStoryEditingParams{CampaignID: m.CampaignID, ID: id, Allowed: allowed})
		if err != nil {
			return wrap("set story editing", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "set story editing", err)
	}
	c, err := s.character(ctx, row, m)
	if err != nil {
		return nil, s.dbError(ctx, "read a character", err)
	}
	return connect.NewResponse(&charactersv1.SetStoryEditingResponse{Character: c}), nil
}

// MarkCharacterDead implements charactersv1connect.CharacterServiceHandler.
func (s *Service) MarkCharacterDead(
	ctx context.Context,
	req *connect.Request[charactersv1.MarkCharacterDeadRequest],
) (*connect.Response[charactersv1.MarkCharacterDeadResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	id, ok := parseUUID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}

	var row charactersdb.Character
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		current, err := visibleForUpdate(ctx, q, m, id)
		if err != nil {
			return err
		}
		if current.Kind != kindPlayer {
			return invalidArgument(fieldErr("character_id", "is an NPC: only player characters die"))
		}
		// RN-03: the character changes status, never row.
		row, err = q.MarkCharacterDead(ctx, charactersdb.MarkCharacterDeadParams{CampaignID: m.CampaignID, ID: id, Now: s.now()})
		if err != nil {
			return wrap("mark dead", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "mark a character dead", err)
	}
	c, err := s.character(ctx, row, m)
	if err != nil {
		return nil, s.dbError(ctx, "read a character", err)
	}
	return connect.NewResponse(&charactersv1.MarkCharacterDeadResponse{Character: c}), nil
}

// GetMasterNotes implements charactersv1connect.CharacterServiceHandler.
func (s *Service) GetMasterNotes(
	ctx context.Context,
	req *connect.Request[charactersv1.GetMasterNotesRequest],
) (*connect.Response[charactersv1.GetMasterNotesResponse], error) {
	// RN-11: only the master, and the notes never ride in any other
	// response.
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	id, ok := parseUUID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}
	exists, err := s.queries.CharacterIsInCampaign(ctx, charactersdb.CharacterIsInCampaignParams{CampaignID: m.CampaignID, ID: id})
	if err != nil {
		return nil, s.dbError(ctx, "find a character", err)
	}
	if !exists {
		return nil, errCharacterNotFound()
	}
	notes, err := s.queries.GetMasterNotes(ctx, charactersdb.GetMasterNotesParams{CampaignID: m.CampaignID, CharacterID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return connect.NewResponse(&charactersv1.GetMasterNotesResponse{}), nil // none written
	}
	if err != nil {
		return nil, s.dbError(ctx, "get master notes", err)
	}
	return connect.NewResponse(&charactersv1.GetMasterNotesResponse{
		Notes:     notes.Notes,
		UpdatedAt: timestamppb.New(notes.UpdatedAt),
	}), nil
}

// UpdateMasterNotes implements charactersv1connect.CharacterServiceHandler.
func (s *Service) UpdateMasterNotes(
	ctx context.Context,
	req *connect.Request[charactersv1.UpdateMasterNotesRequest],
) (*connect.Response[charactersv1.UpdateMasterNotesResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	id, ok := parseUUID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}
	text, err := names.CleanText(req.Msg.GetNotes(), MaxMasterNotesLength)
	if err != nil {
		return nil, invalidArgument(&fieldError{field: "notes", err: err})
	}

	res := &charactersv1.UpdateMasterNotesResponse{}
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		exists, err := q.CharacterIsInCampaign(ctx, charactersdb.CharacterIsInCampaignParams{CampaignID: m.CampaignID, ID: id})
		if err != nil {
			return wrap("find the character", err)
		}
		if !exists {
			return errCharacterNotFound()
		}
		if text == "" {
			// Empty notes are deleted, not stored.
			res.Notes, res.UpdatedAt = "", nil
			if err := q.DeleteMasterNotes(ctx, charactersdb.DeleteMasterNotesParams{CampaignID: m.CampaignID, CharacterID: id}); err != nil {
				return wrap("delete notes", err)
			}
			return nil
		}
		saved, err := q.UpsertMasterNotes(ctx, charactersdb.UpsertMasterNotesParams{
			CampaignID: m.CampaignID, CharacterID: id, Notes: text, UpdatedAt: s.now(),
		})
		if err != nil {
			return wrap("save notes", err)
		}
		res.Notes, res.UpdatedAt = saved.Notes, timestamppb.New(saved.UpdatedAt)
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "update master notes", err)
	}
	return connect.NewResponse(res), nil
}

// visibleForUpdate reads a character inside a transaction, locked until it
// ends, or answers not_found when it is not in the campaign or the caller
// may not see it.
func visibleForUpdate(ctx context.Context, q *charactersdb.Queries, m authz.Membership, id string) (charactersdb.Character, error) {
	row, err := q.GetCharacterForUpdate(ctx, charactersdb.GetCharacterForUpdateParams{CampaignID: m.CampaignID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, errCharacterNotFound()
	}
	if err != nil {
		return row, wrap("read character", err)
	}
	if !canSee(m, row.Kind, row.PlayerUserID) {
		return row, errCharacterNotFound()
	}
	return row, nil
}

// character builds the Character response for a row the caller may see,
// with its player's display name.
func (s *Service) character(ctx context.Context, row charactersdb.Character, m authz.Membership) (*charactersv1.Character, error) {
	var displayName string
	if row.PlayerUserID != nil {
		displayNames, err := s.displayNames(ctx, []string{*row.PlayerUserID})
		if err != nil {
			return nil, err
		}
		displayName = displayNames[*row.PlayerUserID]
	}
	return s.characterToProto(row, m, displayName)
}

// displayNames asks the identity module for players' display names.
func (s *Service) displayNames(ctx context.Context, userIDs []string) (map[string]string, error) {
	if len(userIDs) == 0 {
		return map[string]string{}, nil
	}
	displayNames, err := s.profiles.DisplayNames(ctx, userIDs)
	if err != nil {
		return nil, wrap("read display names", err)
	}
	return displayNames, nil
}
