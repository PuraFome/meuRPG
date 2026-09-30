package campaigns

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1/campaignsv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/campaigns/campaignsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
)

// The campaign document (MR-018) is one Markdown text per campaign, which
// only the campaign's master reads and writes: prep notes, spoilers
// included. Letting players read it, or parts of it, is Samuel's question
// 27; until he answers, the default holds: master only.
//
// The server stores the text and never parses it. The web app renders it
// from a parsed token tree, never as HTML, and resolves the links inside it
// (mapa:<id>, ficha:<id>, imagem:<id>) through the ordinary authorized
// calls, so the IDs in the text grant nothing (campaign_document.proto).
//
// Saves use a revision, like the character sheet: the master sends the
// revision they read, and a save only goes through if nobody saved in
// between. A campaign without a row in campaign_documents has an empty
// document at revision 0.

// MaxDocumentBytes is the largest campaign document: 200 KiB of UTF-8,
// counted in bytes. The campaign_documents_body_size CHECK says the same.
const MaxDocumentBytes = 200 << 10

// The compiler checks that Service also implements the document's service.
var _ campaignsv1connect.CampaignDocumentServiceHandler = (*Service)(nil)

// GetCampaignDocument implements
// campaignsv1connect.CampaignDocumentServiceHandler.
func (s *Service) GetCampaignDocument(
	ctx context.Context,
	req *connect.Request[campaignsv1.GetCampaignDocumentRequest],
) (*connect.Response[campaignsv1.GetCampaignDocumentResponse], error) {
	// Only the master (question 27's default): a player gets
	// permission_denied, anyone else not_found.
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	doc, err := s.queries.GetCampaignDocument(ctx, m.CampaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Never saved: an empty document at revision 0.
		return connect.NewResponse(&campaignsv1.GetCampaignDocumentResponse{
			Document: &campaignsv1.CampaignDocument{CampaignId: m.CampaignID},
		}), nil
	}
	if err != nil {
		return nil, s.dbError(ctx, "get the campaign document", err)
	}
	res, err := s.documentToProto(ctx, doc)
	if err != nil {
		return nil, s.dbError(ctx, "get the campaign document", err)
	}
	return connect.NewResponse(&campaignsv1.GetCampaignDocumentResponse{Document: res}), nil
}

// UpdateCampaignDocument implements
// campaignsv1connect.CampaignDocumentServiceHandler.
func (s *Service) UpdateCampaignDocument(
	ctx context.Context,
	req *connect.Request[campaignsv1.UpdateCampaignDocumentRequest],
) (*connect.Response[campaignsv1.UpdateCampaignDocumentResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	expected := req.Msg.GetExpectedRevision()
	if expected < 0 {
		return nil, invalidArgument("expected_revision", errors.New("must not be negative"))
	}
	body, err := cleanDocument(req.Msg.GetBody())
	if err != nil {
		return nil, invalidArgument("body", err)
	}

	now := s.now()
	var saved campaignsdb.CampaignDocument
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		// Compare-and-swap in one statement: the first save inserts the
		// row, every later one updates it only while the revision is
		// still the one the master read. Either way, no row means the
		// revision is stale.
		if expected == 0 {
			saved, err = q.InsertCampaignDocument(ctx, campaignsdb.InsertCampaignDocumentParams{
				CampaignID: m.CampaignID,
				Body:       body,
				UpdatedAt:  now,
				UpdatedBy:  &m.UserID,
			})
		} else {
			saved, err = q.UpdateCampaignDocument(ctx, campaignsdb.UpdateCampaignDocumentParams{
				CampaignID:       m.CampaignID,
				ExpectedRevision: expected,
				Body:             body,
				UpdatedAt:        now,
				UpdatedBy:        &m.UserID,
			})
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			if err != nil {
				return fmt.Errorf("save the campaign document: %w", err)
			}
			return nil
		}

		// The revision moved. If it moved because of this very save (the
		// same text, by the same person, one revision up), a response got
		// lost or the button was clicked twice: answer with the saved
		// document instead of a conflict.
		current, err := q.GetCampaignDocument(ctx, m.CampaignID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return errStaleDocument() // expected > 0, but it was never saved
		case err != nil:
			return fmt.Errorf("read the campaign document: %w", err)
		case current.Revision == expected+1 && current.Body == body &&
			current.UpdatedBy != nil && *current.UpdatedBy == m.UserID:
			saved = current
			return nil
		default:
			return errStaleDocument()
		}
	})
	if err != nil {
		return nil, s.dbError(ctx, "save the campaign document", err)
	}
	res, err := s.documentToProto(ctx, saved)
	if err != nil {
		return nil, s.dbError(ctx, "save the campaign document", err)
	}
	return connect.NewResponse(&campaignsv1.UpdateCampaignDocumentResponse{Document: res}), nil
}

// documentToProto builds the CampaignDocument the master sees, with the
// display name of whoever saved it last (for "Editado por Samuel").
func (s *Service) documentToProto(ctx context.Context, doc campaignsdb.CampaignDocument) (*campaignsv1.CampaignDocument, error) {
	res := &campaignsv1.CampaignDocument{
		CampaignId: doc.CampaignID,
		Body:       doc.Body,
		Revision:   doc.Revision,
		UpdatedAt:  timestamppb.New(doc.UpdatedAt),
	}
	// updated_by is NULL once that account is deleted: no name then.
	if doc.UpdatedBy != nil {
		displayNames, err := s.profiles.DisplayNames(ctx, []string{*doc.UpdatedBy})
		if err != nil {
			return nil, fmt.Errorf("read the editor's display name: %w", err)
		}
		res.UpdatedByDisplayName = displayNames[*doc.UpdatedBy]
	}
	return res, nil
}

// errStaleDocument is UpdateCampaignDocument's answer when the document was
// saved after the caller read it.
func errStaleDocument() error {
	return connect.NewError(connect.CodeAborted, errors.New("the campaign document changed since it was read; reload it"))
}

// cleanDocument checks a campaign document's body and returns it as it will
// be stored, or an error whose message can be shown to the user (it never
// repeats the text).
//
// It is names.CleanText with two differences, both because the body is
// Markdown for an editor: nothing is trimmed (spaces at the start of a line
// mean something in Markdown, and the editor should get back exactly what
// it saved), and the limit is in bytes, which is what the database stores.
// Like CleanText, it turns Windows (\r\n) and old Mac (\r) line breaks
// into \n, keeps tabs, and refuses every other control character and the
// invisible characters that change text direction.
func cleanDocument(body string) (string, error) {
	if !utf8.ValidString(body) {
		return "", errors.New("must be valid UTF-8")
	}
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	if n := len(body); n > MaxDocumentBytes {
		return "", fmt.Errorf("must be at most %d bytes, got %d", MaxDocumentBytes, n)
	}
	for _, r := range body {
		if r == '\n' || r == '\t' {
			continue
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) {
			return "", fmt.Errorf("must not contain the character %U", r)
		}
	}
	return body, nil
}
