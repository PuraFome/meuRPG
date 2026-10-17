package maps

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/images/gen"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
)

// ImageGenerationService (MR-039, RN-28, ADR-0019): pictures made by an image
// model, for the master. How a request goes:
//
//  1. A short transaction reserves one of the campaign's monthly slots, by
//     inserting the request row (reserve).
//  2. A goroutine calls the model, outside any transaction (run).
//  3. The picture is stored like an upload, as a hidden gallery image, or the
//     slot is refunded.
//
// The slot is spent once the request has left the server ("sent_at") and the
// picture is in the gallery, or the master canceled after it left. Anything
// that ends without a picture gives the slot back.

// DefaultMonthlyImages is how many images a campaign may generate per month
// (a proposal; docs/operations/README.md has the cost behind it).
const DefaultMonthlyImages = 20

// DefaultDailyImages is how many images the whole server may generate per day
// (Brazil's day), on top of each campaign's monthly limit: with one table and
// 20 a month, a day is never close, so the cap only bites when someone is
// abusing it, and it bounds the Gemini bill (docs/operations/README.md).
const DefaultDailyImages = 100

const (
	// maxPromptCharacters is the longest master text, in characters. The
	// image_requests_prompt_length CHECK says the same.
	maxPromptCharacters = 500
	// maxGenerating is how many calls to the model run at once on this server.
	maxGenerating = 1
	// maxPendingRequests is how many image requests may be alive at once on this
	// server, the one calling the model and the ones waiting for its slot. Each
	// keeps its shrunk references in memory (up to about 6 MiB) until its call goes
	// out, so the wait is bounded to what the instance has room for
	// (docs/operations/reference.md#memory-budgets).
	maxPendingRequests = 5
	// maxLongPoll is the longest wait_seconds of GetImageGeneration.
	maxLongPoll = 25
	// generationTimeout bounds a request from the goroutine's start to its end
	// (two attempts of the model's two minutes, and the storing).
	generationTimeout = 6 * time.Minute
	// staleAfter is when a request still pending is taken for lost (a restart).
	staleAfter = 10 * time.Minute
	// defaultSceneRatio is the ratio of a scene when the master picks none.
	defaultSceneRatio = "16:9"
)

// The request kinds and states as the table writes them.
const (
	kindScene = "scene"
	kindEdit  = "edit"

	statePending  = "pending"
	stateDone     = "done"
	stateRefused  = "refused"
	stateFailed   = "failed"
	stateCanceled = "canceled"
)

// The reasons a request ended without a picture, as the table writes them.
const (
	reasonNoImage      = "no_image"
	reasonRefused      = "refused"
	reasonUnavailable  = "unavailable"
	reasonGalleryFull  = "gallery_full"
	reasonImageMissing = "image_missing"
	reasonTimeout      = "timeout"
	reasonServiceOff   = "service_off"
	reasonShutdown     = "shutdown"
)

// brazil is the time zone of the month's count: Brazil has had no daylight
// saving time since 2019, so a fixed UTC-3 needs no tz database.
var brazil = time.FixedZone("BRT", -3*60*60)

// monthOf names the month t falls in ("2026-10") and says when the next one starts.
func monthOf(t time.Time) (key string, resets time.Time) {
	t = t.In(brazil)
	start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, brazil)
	return start.Format("2006-01"), start.AddDate(0, 1, 0)
}

// dayStart is the first instant of t's day in Brazil's time, where the daily
// cap on the whole server's images starts over.
func dayStart(t time.Time) time.Time {
	t = t.In(brazil)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, brazil)
}

// stylePhrases and ratios turn the API's enums into what goes to the model.
var stylePhrases = map[mapsv1.ImageStyle]string{
	mapsv1.ImageStyle_IMAGE_STYLE_OIL_PAINTING:      "oil painting",
	mapsv1.ImageStyle_IMAGE_STYLE_WATERCOLOR:        "watercolor",
	mapsv1.ImageStyle_IMAGE_STYLE_INK:               "ink drawing",
	mapsv1.ImageStyle_IMAGE_STYLE_DIGITAL_ART:       "digital art",
	mapsv1.ImageStyle_IMAGE_STYLE_BOOK_ILLUSTRATION: "book illustration",
}

var ratioNames = map[mapsv1.ImageAspectRatio]string{
	mapsv1.ImageAspectRatio_IMAGE_ASPECT_RATIO_1_1:  "1:1",
	mapsv1.ImageAspectRatio_IMAGE_ASPECT_RATIO_3_2:  "3:2",
	mapsv1.ImageAspectRatio_IMAGE_ASPECT_RATIO_2_3:  "2:3",
	mapsv1.ImageAspectRatio_IMAGE_ASPECT_RATIO_3_4:  "3:4",
	mapsv1.ImageAspectRatio_IMAGE_ASPECT_RATIO_4_3:  "4:3",
	mapsv1.ImageAspectRatio_IMAGE_ASPECT_RATIO_4_5:  "4:5",
	mapsv1.ImageAspectRatio_IMAGE_ASPECT_RATIO_5_4:  "5:4",
	mapsv1.ImageAspectRatio_IMAGE_ASPECT_RATIO_9_16: "9:16",
	mapsv1.ImageAspectRatio_IMAGE_ASPECT_RATIO_16_9: "16:9",
	mapsv1.ImageAspectRatio_IMAGE_ASPECT_RATIO_21_9: "21:9",
}

// styleKeys are the names the table keeps for the styles ("" is none).
var styleKeys = map[mapsv1.ImageStyle]string{
	mapsv1.ImageStyle_IMAGE_STYLE_OIL_PAINTING:      "oil_painting",
	mapsv1.ImageStyle_IMAGE_STYLE_WATERCOLOR:        "watercolor",
	mapsv1.ImageStyle_IMAGE_STYLE_INK:               "ink",
	mapsv1.ImageStyle_IMAGE_STYLE_DIGITAL_ART:       "digital_art",
	mapsv1.ImageStyle_IMAGE_STYLE_BOOK_ILLUSTRATION: "book_illustration",
}

func styleFromKey(key string) mapsv1.ImageStyle {
	for style, k := range styleKeys {
		if k == key {
			return style
		}
	}
	return mapsv1.ImageStyle_IMAGE_STYLE_UNSPECIFIED
}

func ratioFromName(name string) mapsv1.ImageAspectRatio {
	for ratio, n := range ratioNames {
		if n == name {
			return ratio
		}
	}
	return mapsv1.ImageAspectRatio_IMAGE_ASPECT_RATIO_UNSPECIFIED
}

// requireGenerationMaster is the check at the top of every handler: the
// campaign's master. Everyone else, a player included, gets `not_found`, as
// for a hidden image: nothing says that this campaign generates images.
func requireGenerationMaster(ctx context.Context, campaignID string) (authz.Membership, error) {
	m, err := authz.RequireCampaignMember(ctx, campaignID)
	if err != nil {
		return authz.Membership{}, err
	}
	if m.Role != authz.RoleMaster {
		return authz.Membership{}, connect.NewError(connect.CodeNotFound, errors.New("campaign not found"))
	}
	return m, nil
}

// generationOn says whether this server can generate: a generator (the key, or
// the fake) and somewhere to keep the pictures.
func (s *Service) generationOn() bool { return s.generator != nil && s.blobs != nil }

// statusIn reads the campaign's month inside q's transaction or pool.
func (s *Service) statusIn(ctx context.Context, q *mapsdb.Queries, campaignID string) (*mapsv1.ImageGenerationStatus, error) {
	now := s.now()
	month, resets := monthOf(now)
	used, err := q.CountImageSlots(ctx, mapsdb.CountImageSlotsParams{CampaignID: campaignID, QuotaMonth: month})
	if err != nil {
		return nil, fmt.Errorf("count the month's images: %w", err)
	}
	return &mapsv1.ImageGenerationStatus{
		Enabled:                s.generationOn(),
		MonthlyLimit:           s.monthlyImages,
		UsedThisMonth:          used,
		Remaining:              max(s.monthlyImages-used, 0),
		Month:                  month,
		ResetsAt:               timestamppb.New(resets),
		MaxPromptCharacters:    maxPromptCharacters,
		MaxObjectReferences:    gen.MaxObjectReferences,
		MaxCharacterReferences: gen.MaxCharacterReferences,
	}, nil
}

// errBlocked is the failed_precondition with the ImageGenerationBlocked detail.
func errBlocked(reason mapsv1.ImageGenerationBlockedReason, status *mapsv1.ImageGenerationStatus) error {
	message := map[mapsv1.ImageGenerationBlockedReason]string{
		mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_OFF:                 "image generation is not on in this server",
		mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_LIMIT_REACHED:       "the campaign used its images of the month",
		mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_GALLERY_FULL:        "the campaign's gallery is full",
		mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_DAILY_LIMIT_REACHED: "the server made all its images of the day, try again tomorrow",
		mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_REQUEST_TOO_LARGE:   "the references are too big to send together",
	}[reason]
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(message))
	if detail, detailErr := connect.NewErrorDetail(&mapsv1.ImageGenerationBlocked{Reason: reason, Status: status}); detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

// GetImageGenerationStatus implements mapsv1connect.ImageGenerationServiceHandler.
func (s *Service) GetImageGenerationStatus(
	ctx context.Context,
	req *connect.Request[mapsv1.GetImageGenerationStatusRequest],
) (*connect.Response[mapsv1.GetImageGenerationStatusResponse], error) {
	m, err := requireGenerationMaster(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	status, err := s.freshStatus(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read the image generation status", err)
	}
	return connect.NewResponse(&mapsv1.GetImageGenerationStatusResponse{Status: status}), nil
}

// newRequest is what a handler checked and reserve needs to insert a row.
type newRequest struct {
	kind string
	key  string
	// hash is the hash of the whole request but its key (idem.Hash): a retry of the key has the
	// very same one, and the key reused for another request is refused.
	hash        *string
	prompt      string
	style       string
	ratio       string
	references  []string // gallery image IDs of objects
	characters  []string // gallery image IDs of characters
	source      *string  // an edit's image
	requestedBy string
	// name is the gallery image's name: the master's, or the default the server made (never "Imagem N").
	name string
	// mapReq is set for a request made from a map (the three kinds of imagegen_map.go).
	mapReq *mapRequest
	// prepared are the shrunk images the call will carry (prepare), made before
	// the slot is reserved.
	prepared prepared
}

// GenerateSceneImage implements mapsv1connect.ImageGenerationServiceHandler.
func (s *Service) GenerateSceneImage(
	ctx context.Context,
	req *connect.Request[mapsv1.GenerateSceneImageRequest],
) (*connect.Response[mapsv1.GenerateSceneImageResponse], error) {
	m, err := requireGenerationMaster(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	key, err := cleanKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	prompt, err := cleanPrompt(req.Msg.GetPrompt(), "prompt")
	if err != nil {
		return nil, err
	}
	style, ratio, err := styleAndRatio(req.Msg.GetStyle(), req.Msg.GetAspectRatio())
	if err != nil {
		return nil, err
	}
	objects, err := cleanImageIDs(req.Msg.GetObjectImageIds(), gen.MaxObjectReferences, "object_image_ids")
	if err != nil {
		return nil, err
	}
	characters, err := cleanImageIDs(req.Msg.GetCharacterImageIds(), gen.MaxCharacterReferences, "character_image_ids")
	if err != nil {
		return nil, err
	}
	refs := append(append([]string{}, objects...), characters...)
	if hasDuplicates(refs) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("an image is listed twice as a reference"))
	}
	name, err := imageNameOf(req.Msg.GetName(), datedName("Arte da cena", s.now()))
	if err != nil {
		return nil, err
	}
	n := newRequest{
		kind: kindScene, key: key, hash: idem.Hash(req.Msg), prompt: prompt, style: style, ratio: ratio,
		references: objects, characters: characters, requestedBy: m.UserID, name: name,
	}
	row, status, err := s.begin(ctx, n, m.CampaignID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.GenerateSceneImageResponse{Generation: s.generationToProto(row), Status: status}), nil
}

// EditGeneratedImage implements mapsv1connect.ImageGenerationServiceHandler.
func (s *Service) EditGeneratedImage(
	ctx context.Context,
	req *connect.Request[mapsv1.EditGeneratedImageRequest],
) (*connect.Response[mapsv1.EditGeneratedImageResponse], error) {
	m, err := requireGenerationMaster(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	key, err := cleanKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	instruction, err := cleanPrompt(req.Msg.GetInstruction(), "instruction")
	if err != nil {
		return nil, err
	}
	imageID, err := uuid.Parse(req.Msg.GetImageId())
	if err != nil {
		return nil, errImageNotFound()
	}
	// An empty name is the source's, with " (ajuste)": reserve fills it in, with the source in hand.
	name, err := imageNameOf(req.Msg.GetName(), "")
	if err != nil {
		return nil, err
	}
	source := imageID.String()
	row, status, err := s.begin(ctx, newRequest{
		kind: kindEdit, key: key, hash: idem.Hash(req.Msg), prompt: instruction, source: &source, requestedBy: m.UserID, name: name,
	}, m.CampaignID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.EditGeneratedImageResponse{Generation: s.generationToProto(row), Status: status}), nil
}
