package maps

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
)

// Reading, canceling and listing the image requests, and what the app is told of one.

// GetImageGeneration implements mapsv1connect.ImageGenerationServiceHandler.
func (s *Service) GetImageGeneration(
	ctx context.Context,
	req *connect.Request[mapsv1.GetImageGenerationRequest],
) (*connect.Response[mapsv1.GetImageGenerationResponse], error) {
	m, err := requireGenerationMaster(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.Msg.GetGenerationId())
	if err != nil {
		return nil, errGenerationNotFound()
	}
	wait := time.Duration(min(max(req.Msg.GetWaitSeconds(), 0), maxLongPoll)) * time.Second
	// Register before the first read, so a change in between is not missed.
	var changed <-chan struct{}
	if wait > 0 {
		var unregister func()
		changed, unregister = s.waiters.register(id.String())
		defer unregister()
	}
	res, err := s.readGeneration(ctx, m.CampaignID, id.String())
	if err != nil {
		return nil, err
	}
	if wait > 0 && res.GetGeneration().GetState() == mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_PENDING {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-changed:
		case <-timer.C:
		case <-ctx.Done():
		case <-s.baseCtx.Done(): // the server is shutting down: answer now
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if res, err = s.readGeneration(ctx, m.CampaignID, id.String()); err != nil {
			return nil, err
		}
	}
	return connect.NewResponse(res), nil
}

// readGeneration reads a request (after expiring the lost ones) with its image.
func (s *Service) readGeneration(ctx context.Context, campaignID, id string) (*mapsv1.GetImageGenerationResponse, error) {
	var (
		row    mapsdb.ImageRequest
		img    *mapsdb.GalleryImage
		status *mapsv1.ImageGenerationStatus
	)
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		img = nil
		q := s.queries.WithTx(tx)
		if err := s.expireStale(ctx, q, campaignID); err != nil {
			return err
		}
		var err error
		row, err = q.GetImageRequest(ctx, mapsdb.GetImageRequestParams{CampaignID: campaignID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return errGenerationNotFound()
		}
		if err != nil {
			return fmt.Errorf("read the request: %w", err)
		}
		if row.ImageID != nil {
			g, err := q.GetGalleryImageInCampaign(ctx, mapsdb.GetGalleryImageInCampaignParams{CampaignID: campaignID, ID: *row.ImageID})
			if err != nil {
				return fmt.Errorf("read the generated image: %w", err)
			}
			img = &g
		}
		status, err = s.statusIn(ctx, q, campaignID)
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "read an image request", err)
	}
	res := &mapsv1.GetImageGenerationResponse{Generation: s.generationToProto(row), Status: status}
	if img != nil {
		res.Image = imageToProto(*img)
	}
	return res, nil
}

// CancelImageGeneration implements mapsv1connect.ImageGenerationServiceHandler.
func (s *Service) CancelImageGeneration(
	ctx context.Context,
	req *connect.Request[mapsv1.CancelImageGenerationRequest],
) (*connect.Response[mapsv1.CancelImageGenerationResponse], error) {
	m, err := requireGenerationMaster(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.Msg.GetGenerationId())
	if err != nil {
		return nil, errGenerationNotFound()
	}
	var (
		row    mapsdb.ImageRequest
		status *mapsv1.ImageGenerationStatus
	)
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		now := s.now()
		// Before it left: canceled, and the slot goes back. After: only the wait stops.
		n, err := q.CancelUnsentImageRequest(ctx, mapsdb.CancelUnsentImageRequestParams{CampaignID: m.CampaignID, ID: id.String(), FinishedAt: &now})
		if err != nil {
			return fmt.Errorf("cancel an unsent request: %w", err)
		}
		if n == 0 {
			if _, err := q.CancelSentImageRequest(ctx, mapsdb.CancelSentImageRequestParams{CampaignID: m.CampaignID, ID: id.String(), FinishedAt: &now}); err != nil {
				return fmt.Errorf("cancel a sent request: %w", err)
			}
		}
		row, err = q.GetImageRequest(ctx, mapsdb.GetImageRequestParams{CampaignID: m.CampaignID, ID: id.String()})
		if errors.Is(err, pgx.ErrNoRows) {
			return errGenerationNotFound()
		}
		if err != nil {
			return fmt.Errorf("read the request: %w", err)
		}
		status, err = s.statusIn(ctx, q, m.CampaignID)
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "cancel an image request", err)
	}
	s.waiters.notify(row.ID)
	return connect.NewResponse(&mapsv1.CancelImageGenerationResponse{Generation: s.generationToProto(row), Status: status}), nil
}

// ListImageEdits implements mapsv1connect.ImageGenerationServiceHandler.
func (s *Service) ListImageEdits(
	ctx context.Context,
	req *connect.Request[mapsv1.ListImageEditsRequest],
) (*connect.Response[mapsv1.ListImageEditsResponse], error) {
	m, err := requireGenerationMaster(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.Msg.GetImageId())
	if err != nil {
		return nil, errImageNotFound()
	}
	rows, err := s.queries.ListImageChain(ctx, mapsdb.ListImageChainParams{CampaignID: m.CampaignID, ID: id.String()})
	if err != nil {
		return nil, s.dbError(ctx, "list the edit chain", err)
	}
	if len(rows) == 0 {
		return nil, errImageNotFound()
	}
	res := &mapsv1.ListImageEditsResponse{}
	for _, r := range rows {
		res.Edits = append(res.Edits, &mapsv1.ImageEdit{
			Image: imageToProto(mapsdb.GalleryImage{
				ID: r.ID, CampaignID: r.CampaignID, UploadedBy: r.UploadedBy, Name: r.Name, ContentType: r.ContentType,
				Width: r.Width, Height: r.Height, ByteSize: r.ByteSize, CreatedAt: r.CreatedAt,
				Generated: r.Generated, ParentImageID: r.ParentImageID, GeneratedKind: r.GeneratedKind,
			}),
			Prompt: r.Prompt, Number: r.Number,
		})
	}
	return connect.NewResponse(res), nil
}

func errGenerationNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("image request not found"))
}

// generationToProto turns a row into the API's message.
func (s *Service) generationToProto(r mapsdb.ImageRequest) *mapsv1.ImageGeneration {
	g := &mapsv1.ImageGeneration{
		Id: r.ID, CampaignId: r.CampaignID, Prompt: r.Prompt, Number: r.Number,
		Style: styleFromKey(r.Style), AspectRatio: ratioFromName(r.AspectRatio),
		ReferenceImageIds: r.ReferenceIds, CharacterImageIds: r.CharacterIds, SlotSpent: !r.Refunded,
		CreatedAt: timestamppb.New(r.CreatedAt),
	}
	switch r.Kind {
	case kindScene:
		g.Kind = mapsv1.ImageGenerationKind_IMAGE_GENERATION_KIND_SCENE
	case kindEdit:
		g.Kind = mapsv1.ImageGenerationKind_IMAGE_GENERATION_KIND_EDIT
	case kindMapScene:
		g.Kind = mapsv1.ImageGenerationKind_IMAGE_GENERATION_KIND_MAP_SCENE
	case kindIsometric:
		g.Kind = mapsv1.ImageGenerationKind_IMAGE_GENERATION_KIND_ISOMETRIC
	case kindTexturedMap:
		g.Kind = mapsv1.ImageGenerationKind_IMAGE_GENERATION_KIND_TEXTURED_MAP
	}
	if r.MapID != nil {
		g.MapId = *r.MapID
	}
	switch r.Status {
	case statePending:
		g.State = mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_PENDING
	case stateDone:
		g.State = mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_DONE
	case stateRefused:
		g.State = mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_REFUSED
	case stateFailed:
		g.State = mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_FAILED
	case stateCanceled:
		g.State = mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_CANCELED
	}
	g.Failure, g.ReasonPt = failureOf(r.Reason)
	if r.ImageID != nil {
		g.ImageId = *r.ImageID
	}
	if r.SourceImageID != nil {
		g.SourceImageId = *r.SourceImageID
	}
	if r.FinishedAt != nil {
		g.FinishedAt = timestamppb.New(*r.FinishedAt)
	}
	return g
}

// failureOf maps a stored reason to the enum and the Portuguese sentence. The
// sentences are ours: the model's own message never reaches the master.
func failureOf(reason string) (mapsv1.ImageGenerationFailure, string) {
	switch reason {
	case reasonNoImage:
		return mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_NO_IMAGE, "O serviço não gerou uma imagem. Tente descrever a cena de outro jeito."
	case reasonRefused:
		return mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_REFUSED, "O serviço recusou este pedido. Tente descrever a cena de outro jeito."
	case reasonUnavailable:
		return mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_UNAVAILABLE, "O serviço de imagens não respondeu. Tente de novo em instantes."
	case reasonGalleryFull:
		return mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_GALLERY_FULL, "A galeria está cheia: apague uma imagem e tente de novo."
	case reasonStorageFailed:
		return mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_STORAGE_FAILED, "A imagem foi feita, mas não deu para guardá-la. Tente de novo; esta não conta no seu limite."
	case reasonImageMissing:
		return mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_IMAGE_MISSING, "Uma das imagens do pedido foi apagada. Escolha de novo."
	case reasonServiceOff:
		return mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_SERVICE_OFF, "O serviço de imagens não está disponível."
	case reasonShutdown:
		return mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_UNAVAILABLE, "O servidor reiniciou antes de enviar o pedido. Tente de novo."
	case reasonTimeout:
		return mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_TIMEOUT, "O pedido se perdeu. Tente de novo."
	}
	return mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_UNSPECIFIED, ""
}
