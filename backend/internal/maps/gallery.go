package maps

import (
	"context"
	"errors"
	"fmt"
	"math"
	"uuid"

	"connectrpc.com/connect"
	"github.com/cockroachdb/cockroach-go/v2/crdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/images"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
)

// Every GalleryService method is the master's: the gallery is preparation,
// and a player could learn about a map before it is revealed (RN-10). Each
// handler starts with authz.RequireCampaignRole, whose error is already the
// right Connect error.

// ListGalleryImages implements mapsv1connect.GalleryServiceHandler.
func (s *Service) ListGalleryImages(
	ctx context.Context,
	req *connect.Request[mapsv1.ListGalleryImagesRequest],
) (*connect.Response[mapsv1.ListGalleryImagesResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	if s.blobs == nil {
		return nil, errImagesOff()
	}
	rows, err := s.queries.ListGalleryImages(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list the gallery", err)
	}
	usage, err := s.queries.GetGalleryUsage(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read the gallery usage", err)
	}
	res := &mapsv1.ListGalleryImagesResponse{Usage: s.usageToProto(usage)}
	for _, row := range rows {
		res.Images = append(res.Images, imageToProto(row))
	}
	return connect.NewResponse(res), nil
}

// RenameGalleryImage implements mapsv1connect.GalleryServiceHandler.
func (s *Service) RenameGalleryImage(
	ctx context.Context,
	req *connect.Request[mapsv1.RenameGalleryImageRequest],
) (*connect.Response[mapsv1.RenameGalleryImageResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	if s.blobs == nil {
		return nil, errImagesOff()
	}
	id, err := uuid.Parse(req.Msg.GetImageId())
	if err != nil {
		return nil, errImageNotFound()
	}
	name, err := names.Clean(req.Msg.GetName(), maxNameLength)
	if err != nil {
		// The message says the rule, never the name, which is free text.
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name %w", err))
	}

	var row mapsdb.GalleryImage
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		row, err = s.queries.WithTx(tx).RenameGalleryImage(ctx, mapsdb.RenameGalleryImageParams{
			CampaignID: m.CampaignID, ID: id.String(), Name: name,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errImageNotFound()
		}
		if err != nil {
			return fmt.Errorf("rename gallery image: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "rename a gallery image", err)
	}
	return connect.NewResponse(&mapsv1.RenameGalleryImageResponse{Image: imageToProto(row)}), nil
}

// DeleteGalleryImage implements mapsv1connect.GalleryServiceHandler. It
// deletes the row first, then the files: once the row is gone nobody can
// fetch the files, so a failure to delete them (logged) leaves nothing
// reachable behind.
func (s *Service) DeleteGalleryImage(
	ctx context.Context,
	req *connect.Request[mapsv1.DeleteGalleryImageRequest],
) (*connect.Response[mapsv1.DeleteGalleryImageResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	if s.blobs == nil {
		return nil, errImagesOff()
	}
	id, err := uuid.Parse(req.Msg.GetImageId())
	if err != nil {
		return nil, errImageNotFound()
	}

	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		_, err := s.queries.WithTx(tx).DeleteGalleryImage(ctx, mapsdb.DeleteGalleryImageParams{
			CampaignID: m.CampaignID, ID: id.String(),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errImageNotFound()
		}
		if err != nil {
			return fmt.Errorf("delete gallery image: %w", err)
		}
		return nil
	})
	// A map that uses the image keeps it: the maps' foreign key to
	// gallery_images is ON DELETE RESTRICT, so the delete fails with a
	// foreign key violation. (The maps table comes with the maps themselves;
	// this answer is ready for it.)
	if isForeignKeyViolation(err) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("a map uses this image; change the map's image first"))
	}
	if err != nil {
		return nil, s.dbError(ctx, "delete a gallery image", err)
	}
	s.deleteFiles(ctx, m.CampaignID, id.String())
	return connect.NewResponse(&mapsv1.DeleteGalleryImageResponse{}), nil
}

func imageToProto(r mapsdb.GalleryImage) *mapsv1.GalleryImage {
	return &mapsv1.GalleryImage{
		Id:           r.ID,
		CampaignId:   r.CampaignID,
		Name:         r.Name,
		ContentType:  r.ContentType,
		Width:        r.Width,
		Height:       r.Height,
		ByteSize:     r.ByteSize,
		CreatedAt:    timestamppb.New(r.CreatedAt),
		Url:          imageURL(r.ID),
		ThumbnailUrl: thumbnailURL(r.ID),
	}
}

func (s *Service) usageToProto(u mapsdb.GetGalleryUsageRow) *mapsv1.GalleryUsage {
	return &mapsv1.GalleryUsage{
		ImageCount:    u.ImageCount,
		MaxImages:     s.maxImages,
		ByteCount:     clampInt32(u.ByteCount),
		MaxBytes:      s.maxBytes,
		MaxImageBytes: images.MaxBytes,
	}
}

// clampInt32 fits a byte count into the API's int32. The quota keeps a
// gallery far below 2 GiB, the most an int32 holds; this only makes the
// conversion provably safe.
func clampInt32(n int64) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	if n < 0 {
		return 0
	}
	return int32(n)
}

// errImageNotFound is the answer for an image that is not in the campaign.
func errImageNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("image not found"))
}

// isForeignKeyViolation reports whether err is a foreign key violation
// (23503).
func isForeignKeyViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == "23503"
}

// dbError turns an error from the database, or from inside a transaction,
// into the Connect error the client gets, as in package play.
func (s *Service) dbError(ctx context.Context, action string, err error) error {
	if connectErr, ok := errors.AsType[*connect.Error](err); ok {
		return connectErr
	}
	s.logger.ErrorContext(ctx, "maps: cannot "+action, "error", err)
	if _, ok := errors.AsType[*crdb.MaxRetriesExceededError](err); ok {
		return connect.NewError(connect.CodeAborted, errors.New("too many changes at the same time, please try again"))
	}
	return connect.NewError(connect.CodeUnavailable, errors.New("cannot reach the database right now, please try again"))
}
