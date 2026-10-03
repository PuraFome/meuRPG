package notes

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	notesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/notes/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/notes/link"
	"github.com/PuraFome/meuRPG/backend/internal/notes/notesdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
)

// requirePlayer is the start of every handler: the caller must be an active
// player of the campaign. The campaign's master gets `not_found`, as for a
// note that is not there: the master has no notes, and nothing here tells them
// anything about the players' (RN-20, question 60).
func requirePlayer(ctx context.Context, campaignID string) (authz.Membership, error) {
	m, err := authz.RequireCampaignMember(ctx, campaignID)
	if err != nil {
		return authz.Membership{}, err
	}
	if m.Role != authz.RolePlayer {
		return authz.Membership{}, errNotFound()
	}
	return m, nil
}

// errNotFound is the answer for a note that is not the caller's, one that does
// not exist, and a caller who may not have notes. Its text is the same for all
// of them.
func errNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("note not found"))
}

// errBadScene is the answer for a scene tag the group did not discover: the
// same for a scene that does not exist, so a player never learns that an
// undiscovered scene is there.
func errBadScene() error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New("scene_point_id must be a scene the group discovered"))
}

// parseID returns an ID in canonical form, or false when it is not a UUID.
func parseID(raw string) (string, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", false
	}
	return id.String(), true
}

// cleanNoteText checks a note: 1 to 2,000 characters, line breaks allowed. The
// message says the rule, never the text.
func cleanNoteText(raw string) (string, error) {
	text, err := names.CleanText(raw, maxNoteText)
	if err == nil && text == "" {
		err = names.ErrEmpty
	}
	if err != nil {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("text %w", err))
	}
	return text, nil
}

// discovered reads the scenes the group discovered, by ID.
func (s *Service) discovered(ctx context.Context, campaignID string) (map[string]string, []link.Scene, error) {
	scenes, err := s.scenes.DiscoveredScenes(ctx, campaignID)
	if err != nil {
		return nil, nil, s.dbError(ctx, "read the discovered scenes", err)
	}
	names := make(map[string]string, len(scenes))
	for _, sc := range scenes {
		names[sc.ID] = sc.Name
	}
	return names, scenes, nil
}

// checkTag checks a scene tag from a request: empty for none, else a scene the
// group discovered. It returns the tag to store, nil for none.
func checkTag(raw string, discovered map[string]string) (*string, error) {
	if raw == "" {
		return nil, nil
	}
	id, ok := parseID(raw)
	if !ok {
		return nil, errBadScene()
	}
	if _, ok := discovered[id]; !ok {
		return nil, errBadScene()
	}
	return &id, nil
}

// noteToProto builds one of the caller's notes. The tag is shown only while its
// scene is discovered, so a name never travels for another scene.
func noteToProto(n notesdb.PlayerNote, discovered map[string]string) *notesv1.Note {
	out := &notesv1.Note{
		Id: n.ID, Kind: notesv1.NoteKind_NOTE_KIND_NOTE, Text: n.Text,
		CreatedAt: timestamppb.New(n.CreatedAt), UpdatedAt: timestamppb.New(n.UpdatedAt),
	}
	tag(out, n.ScenePointID, discovered)
	return out
}

func clueToProto(c link.Clue, discovered map[string]string) *notesv1.Note {
	out := &notesv1.Note{
		Id: c.ID, Kind: notesv1.NoteKind_NOTE_KIND_CLUE, Text: c.Text,
		CreatedAt: timestamppb.New(c.RevealedAt), UpdatedAt: timestamppb.New(c.RevealedAt),
	}
	if c.PointID != "" {
		p := c.PointID
		tag(out, &p, discovered)
	}
	return out
}

func tag(out *notesv1.Note, point *string, discovered map[string]string) {
	if point == nil {
		return
	}
	if name, ok := discovered[*point]; ok {
		out.ScenePointId, out.SceneName = *point, name
	}
}

// ListNotes implements notesv1connect.NotesServiceHandler.
func (s *Service) ListNotes(
	ctx context.Context,
	req *connect.Request[notesv1.ListNotesRequest],
) (*connect.Response[notesv1.ListNotesResponse], error) {
	m, err := requirePlayer(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListPlayerNotes(ctx, notesdb.ListPlayerNotesParams{CampaignID: m.CampaignID, AuthorUserID: m.UserID})
	if err != nil {
		return nil, s.dbError(ctx, "list the notes", err)
	}
	clues, err := s.scenes.ReceivedClues(ctx, m.CampaignID, m.UserID)
	if err != nil {
		return nil, s.dbError(ctx, "list the received clues", err)
	}
	discovered, _, err := s.discovered(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}

	// Newest first: a note by its last write, a clue by its arrival.
	type entry struct {
		at   time.Time
		note *notesv1.Note
	}
	entries := make([]entry, 0, len(rows)+len(clues))
	for _, r := range rows {
		entries = append(entries, entry{r.UpdatedAt, noteToProto(r, discovered)})
	}
	for _, c := range clues {
		entries = append(entries, entry{c.RevealedAt, clueToProto(c, discovered)})
	}
	slices.SortStableFunc(entries, func(a, b entry) int { return b.at.Compare(a.at) })

	filter := req.Msg.GetScenePointId()
	res := &notesv1.ListNotesResponse{NoteCount: int32(len(rows)), MaxNotes: MaxNotes} //nolint:gosec // G115: at most 300 notes
	for _, e := range entries {
		if filter != "" && e.note.GetScenePointId() != filter {
			continue
		}
		res.Notes = append(res.Notes, e.note)
	}
	return connect.NewResponse(res), nil
}

// CreateNote implements notesv1connect.NotesServiceHandler.
func (s *Service) CreateNote(
	ctx context.Context,
	req *connect.Request[notesv1.CreateNoteRequest],
) (*connect.Response[notesv1.CreateNoteResponse], error) {
	m, err := requirePlayer(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	text, err := cleanNoteText(req.Msg.GetText())
	if err != nil {
		return nil, err
	}
	discovered, _, err := s.discovered(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	scene, err := checkTag(req.Msg.GetScenePointId(), discovered)
	if err != nil {
		return nil, err
	}

	var created notesdb.PlayerNote
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		count, err := q.CountPlayerNotes(ctx, notesdb.CountPlayerNotesParams{CampaignID: m.CampaignID, AuthorUserID: m.UserID})
		if err != nil {
			return fmt.Errorf("count the notes: %w", err)
		}
		if count >= MaxNotes {
			return connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("you already have %d notes in this campaign", MaxNotes))
		}
		created, err = q.InsertPlayerNote(ctx, notesdb.InsertPlayerNoteParams{
			CampaignID: m.CampaignID, AuthorUserID: m.UserID, Text: text, ScenePointID: scene, Now: s.now(),
		})
		if err != nil {
			return fmt.Errorf("insert note: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "create a note", err)
	}
	return connect.NewResponse(&notesv1.CreateNoteResponse{Note: noteToProto(created, discovered)}), nil
}

// UpdateNote implements notesv1connect.NotesServiceHandler.
func (s *Service) UpdateNote(
	ctx context.Context,
	req *connect.Request[notesv1.UpdateNoteRequest],
) (*connect.Response[notesv1.UpdateNoteResponse], error) {
	m, err := requirePlayer(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	noteID, ok := parseID(req.Msg.GetNoteId())
	if !ok {
		return nil, errNotFound()
	}
	msg := req.Msg
	if msg.Text == nil && msg.ScenePointId == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("nothing to change"))
	}
	var text *string
	if msg.Text != nil {
		t, err := cleanNoteText(msg.GetText())
		if err != nil {
			return nil, err
		}
		text = &t
	}
	discovered, _, err := s.discovered(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	var scene *string
	if msg.ScenePointId != nil {
		if scene, err = checkTag(msg.GetScenePointId(), discovered); err != nil {
			return nil, err
		}
	}

	var changed notesdb.PlayerNote
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		// The author is part of the lookup: another player's note, or a
		// clue's ID, is "not found".
		n, err := q.GetPlayerNoteForUpdate(ctx, notesdb.GetPlayerNoteForUpdateParams{CampaignID: m.CampaignID, AuthorUserID: m.UserID, ID: noteID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound()
		}
		if err != nil {
			return fmt.Errorf("find note: %w", err)
		}
		params := notesdb.UpdatePlayerNoteParams{
			CampaignID: m.CampaignID, AuthorUserID: m.UserID, ID: noteID, Text: n.Text, ScenePointID: n.ScenePointID, Now: s.now(),
		}
		if text != nil {
			params.Text = *text
		}
		if msg.ScenePointId != nil {
			params.ScenePointID = scene // nil removes the tag
		}
		if changed, err = q.UpdatePlayerNote(ctx, params); err != nil {
			return fmt.Errorf("update note: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "update a note", err)
	}
	return connect.NewResponse(&notesv1.UpdateNoteResponse{Note: noteToProto(changed, discovered)}), nil
}

// DeleteNote implements notesv1connect.NotesServiceHandler.
func (s *Service) DeleteNote(
	ctx context.Context,
	req *connect.Request[notesv1.DeleteNoteRequest],
) (*connect.Response[notesv1.DeleteNoteResponse], error) {
	m, err := requirePlayer(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	noteID, ok := parseID(req.Msg.GetNoteId())
	if !ok {
		return nil, errNotFound()
	}
	var n int64
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		n, err = s.queries.WithTx(tx).DeletePlayerNote(ctx, notesdb.DeletePlayerNoteParams{CampaignID: m.CampaignID, AuthorUserID: m.UserID, ID: noteID})
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "delete a note", err)
	}
	if n == 0 {
		return nil, errNotFound()
	}
	return connect.NewResponse(&notesv1.DeleteNoteResponse{}), nil
}

// ListNoteScenes implements notesv1connect.NotesServiceHandler.
func (s *Service) ListNoteScenes(
	ctx context.Context,
	req *connect.Request[notesv1.ListNoteScenesRequest],
) (*connect.Response[notesv1.ListNoteScenesResponse], error) {
	m, err := requirePlayer(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	_, scenes, err := s.discovered(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	res := &notesv1.ListNoteScenesResponse{}
	for _, sc := range scenes {
		res.Scenes = append(res.Scenes, &notesv1.NoteScene{Id: sc.ID, Name: sc.Name})
	}
	return connect.NewResponse(res), nil
}
