package maps

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/images"
	"github.com/PuraFome/meuRPG/backend/internal/maps/images/gen"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/maps/refimg"
	"github.com/PuraFome/meuRPG/backend/internal/platform/blob"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
)

// How an image request starts: the status, the checks of the references and the prompt,
// and the short transaction that reserves a monthly slot (step 1 of imagegen.go).

// freshStatus reads the month after giving back the slots of requests that
// were lost (expireStale).
func (s *Service) freshStatus(ctx context.Context, campaignID string) (*mapsv1.ImageGenerationStatus, error) {
	var status *mapsv1.ImageGenerationStatus
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		if err := s.expireStale(ctx, q, campaignID); err != nil {
			return err
		}
		var err error
		status, err = s.statusIn(ctx, q, campaignID)
		return err
	})
	return status, err
}

// expireStale ends the campaign's requests that lost their server: one that
// never left refunds its slot; one that left fails as timed out and the slot
// stays spent (it may have been billed).
func (s *Service) expireStale(ctx context.Context, q *mapsdb.Queries, campaignID string) error {
	now, before := s.now(), s.now().Add(-staleAfter)
	if err := q.ExpireUnsentImageRequests(ctx, mapsdb.ExpireUnsentImageRequestsParams{Now: &now, CampaignID: campaignID, Before: before}); err != nil {
		return fmt.Errorf("expire unsent requests: %w", err)
	}
	if err := q.ExpireSentImageRequests(ctx, mapsdb.ExpireSentImageRequestsParams{Now: &now, CampaignID: campaignID, Before: &before}); err != nil {
		return fmt.Errorf("expire sent requests: %w", err)
	}
	return nil
}

// prepared are the images a call carries, shrunk.
type prepared struct {
	references []gen.Reference // objects first, then characters
	previous   *gen.Image      // an edit's image
	// For a request made from a map (prepareMap): the drawing, the NPCs' portraits
	// that were added to the characters, the map as it was seen, the ratio the server
	// picked (the textured map's) and the rooms of a generated dungeon's textured map.
	drawing   *gen.Image
	npcImages []string
	basis     *mapBasis
	ratio     string
	rooms     []string
	// name is the default name of a picture made from a map (the map's, and the way).
	name string
}

// sameImageRequest says whether the request a key found is the one being retried: an image
// request from before the hash was kept has none, and is replayed as it was.
func sameImageRequest(existing mapsdb.ImageRequest, n newRequest) error {
	if existing.IdempotencyHash == nil || (n.hash != nil && *existing.IdempotencyHash == *n.hash) {
		return nil
	}
	return idem.ErrReused()
}

// begin checks what can be checked cheaply, prepares the images, and reserves
// the slot. The order spares the server: an earlier try with the same key is
// answered first, a server with generation off or a month with no slot left is
// refused before any image is read, and the images (read from the blob store
// and shrunk, one at a time in the upload slot) are prepared before the slot is
// reserved, so a request too big for the service never costs one.
func (s *Service) begin(ctx context.Context, n newRequest, campaignID string) (mapsdb.ImageRequest, *mapsv1.ImageGenerationStatus, error) {
	if existing, err := s.queries.GetImageRequestByKey(ctx, mapsdb.GetImageRequestByKeyParams{CampaignID: campaignID, IdempotencyKey: n.key}); err == nil {
		if err := sameImageRequest(existing, n); err != nil {
			return mapsdb.ImageRequest{}, nil, err
		}
		status, err := s.freshStatus(ctx, campaignID)
		if err != nil {
			return mapsdb.ImageRequest{}, nil, s.dbError(ctx, "read the image generation status", err)
		}
		return existing, status, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return mapsdb.ImageRequest{}, nil, s.dbError(ctx, "find an image request", err)
	}
	status, err := s.freshStatus(ctx, campaignID)
	if err != nil {
		return mapsdb.ImageRequest{}, nil, s.dbError(ctx, "read the image generation status", err)
	}
	switch {
	case !status.GetEnabled():
		return mapsdb.ImageRequest{}, nil, errBlocked(mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_OFF, status)
	case status.GetRemaining() <= 0:
		return mapsdb.ImageRequest{}, nil, errBlocked(mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_LIMIT_REACHED, status)
	}
	// The server's cap of the day, before any image is read and shrunk. reserve
	// checks it again in its transaction, which is the one that counts.
	if made, err := s.queries.CountImageRequestsSince(ctx, dayStart(s.now())); err != nil {
		return mapsdb.ImageRequest{}, nil, s.dbError(ctx, "count the day's generated images", err)
	} else if made >= s.dailyImages {
		return mapsdb.ImageRequest{}, nil, errBlocked(mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_DAILY_LIMIT_REACHED, status)
	}
	prep, err := s.prepare(ctx, campaignID, n, status)
	if err != nil {
		return mapsdb.ImageRequest{}, nil, err
	}
	n.prepared = prep
	// The NPCs' portraits are character references like any other: the row records them.
	n.characters = append(append([]string{}, n.characters...), prep.npcImages...)
	if prep.ratio != "" {
		n.ratio = prep.ratio
	}
	if n.name == "" {
		n.name = prep.name
	}
	stub := gen.Request{Prompt: n.prompt, References: prep.references, Edit: editStub(n, prep), Drawing: prep.drawing, Rooms: prep.rooms}
	if prep.drawing != nil {
		stub.Layout = layoutOf(n.kind)
	}
	if size := stub.BodySize(); size > gen.MaxBodyBytes {
		return mapsdb.ImageRequest{}, nil, errBlocked(mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_REQUEST_TOO_LARGE, status)
	}
	return s.reserve(ctx, n, campaignID)
}

// editStub is an edit's previous image in a Request, only to size the body.
func editStub(n newRequest, p prepared) *gen.Edit {
	if p.previous == nil {
		return nil
	}
	return &gen.Edit{Previous: *p.previous, Instruction: n.prompt}
}

// prepare reads the request's images from the blob store, as the campaign's own
// images (never a URL from the browser), and shrinks each one to a small JPEG.
// An image that is not the campaign's is `not_found`.
func (s *Service) prepare(ctx context.Context, campaignID string, n newRequest, status *mapsv1.ImageGenerationStatus) (prepared, error) {
	var p prepared
	if n.mapReq != nil {
		if err := s.prepareMap(ctx, campaignID, &n, &p, status); err != nil {
			return p, err
		}
	}
	if n.source != nil {
		if err := s.prepareSource(ctx, campaignID, *n.source, &p); err != nil {
			return p, err
		}
	}
	err := s.prepareReferences(ctx, campaignID, n, &p)
	return p, err
}

// prepareSource reads the image an edit adjusts, which must be a generated one of the
// campaign, into p.previous.
func (s *Service) prepareSource(ctx context.Context, campaignID, source string, p *prepared) error {
	src, err := s.queries.GetGalleryImageInCampaign(ctx, mapsdb.GetGalleryImageInCampaignParams{CampaignID: campaignID, ID: source})
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !src.Generated {
		return errImageNotFound()
	}
	if err != nil {
		return s.dbError(ctx, "find the image to edit", err)
	}
	img, err := s.shrunkImage(ctx, campaignID, source)
	if err != nil {
		return err
	}
	p.previous = &img
	return nil
}

// prepareReferences shrinks the object images, then the character images (the
// request's, then the NPCs' portraits prepareMap added), into p.references.
func (s *Service) prepareReferences(ctx context.Context, campaignID string, n newRequest, p *prepared) error {
	for _, character := range []bool{false, true} {
		ids := n.references
		if character {
			ids = append(append([]string{}, n.characters...), p.npcImages...)
		}
		for _, id := range ids {
			img, err := s.shrunkImage(ctx, campaignID, id)
			if err != nil {
				return err
			}
			p.references = append(p.references, gen.Reference{Image: img, Character: character})
		}
	}
	return nil
}

// shrunkImage returns a gallery image as the small JPEG it travels as: its
// reference, kept next to it in the blob store (made at upload for an image
// bigger than images.ReferenceSide, so no later decode of the original). An
// image without one, an older one, is decoded once, inside the upload slot (one
// image decoded at a time on the whole server), and its reference is kept for
// next time; a small image has none to keep and is converted each time, which
// costs little.
func (s *Service) shrunkImage(ctx context.Context, campaignID, id string) (gen.Image, error) {
	if _, err := s.queries.GetGalleryImageInCampaign(ctx, mapsdb.GetGalleryImageInCampaignParams{CampaignID: campaignID, ID: id}); errors.Is(err, pgx.ErrNoRows) {
		return gen.Image{}, errImageNotFound()
	} else if err != nil {
		return gen.Image{}, s.dbError(ctx, "find a reference image", err)
	}
	refKey := referenceKey(campaignID, id)
	switch data, err := s.readBlob(ctx, refKey, images.MaxBytes); {
	case err == nil:
		return gen.Image{MimeType: images.JPEG, Data: data}, nil
	case !errors.Is(err, blob.ErrNotFound):
		return gen.Image{}, errStorage()
	}
	key, _ := blobKeys(campaignID, id)
	// The slot first, then the read: the file (at most 10 MiB) is in memory
	// only while it holds the slot, so waiting requests hold none.
	release, err := s.acquireProcessing(ctx)
	if err != nil {
		return gen.Image{}, err
	}
	data, err := s.readBlob(ctx, key, images.MaxBytes)
	if err != nil {
		release()
		s.logger.ErrorContext(ctx, "maps: cannot read an image file", "error", err)
		return gen.Image{}, errStorage()
	}
	if s.onReferenceDecode != nil {
		s.onReferenceDecode()
	}
	small, resized, err := images.Shrink(data, images.ReferenceSide, images.ReferenceQuality)
	release()
	if err != nil {
		s.logger.ErrorContext(ctx, "maps: cannot shrink a reference image", "error", err)
		return gen.Image{}, errStorage()
	}
	if resized {
		if err := s.blobs.Put(ctx, refKey, images.JPEG, bytes.NewReader(small)); err != nil {
			s.logger.WarnContext(ctx, "maps: cannot keep a reference image", "error", err)
		}
	}
	return gen.Image{MimeType: images.JPEG, Data: small}, nil
}

// readBlob reads a whole blob at once, into a buffer of its exact size (no
// growing, no second copy), refusing one over limit bytes.
func (s *Service) readBlob(ctx context.Context, key string, limit int64) ([]byte, error) {
	obj, err := s.blobs.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	defer func() { _ = obj.Close() }()
	if obj.Size > limit {
		return nil, errors.New("the file is too large")
	}
	data := make([]byte, obj.Size)
	if _, err := io.ReadFull(obj.Content, data); err != nil {
		return nil, err
	}
	return data, nil
}

// errTooManyImageRequests is `resource_exhausted` for an image request made while the
// server already has as many alive as it holds in memory.
func errTooManyImageRequests() error {
	return connect.NewError(connect.CodeResourceExhausted, errors.New("too many images are being made right now; wait for one to finish"))
}

// reserve is step 1: in one short transaction it finds the request an earlier
// try with the same key made, or checks the cap and the gallery and inserts the
// row, which reserves the slot. It starts the goroutine of a new request after
// the commit. Every read goes through the transaction (the #121 rule).
func (s *Service) reserve(ctx context.Context, n newRequest, campaignID string) (mapsdb.ImageRequest, *mapsv1.ImageGenerationStatus, error) {
	var res reservation
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		res, err = s.reserveTx(ctx, s.queries.WithTx(tx), &n, campaignID)
		return err
	})
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgUniqueViolation {
		// Two tries with the same key raced: the other one made the request.
		if existing, getErr := s.queries.GetImageRequestByKey(ctx, mapsdb.GetImageRequestByKeyParams{CampaignID: campaignID, IdempotencyKey: n.key}); getErr == nil {
			return s.racedReservation(ctx, existing, n, campaignID)
		}
	}
	if err != nil {
		return mapsdb.ImageRequest{}, nil, s.dbError(ctx, "reserve an image", err)
	}
	if res.created {
		logging.Event(ctx, s.logger, "image.requested", slog.String("generation_id", res.row.ID), slog.String("kind", n.kind))
		s.generations.Add(1)
		s.pending.Add(1) // run gives it back
		go s.run(campaignID, res.row.ID, n.prepared)
	}
	return res.row, res.status, nil
}

// pgUniqueViolation is the SQLSTATE of a unique index that refused a row.
const pgUniqueViolation = "23505"

// reservation is what one attempt of reserve's transaction leaves behind.
type reservation struct {
	row     mapsdb.ImageRequest
	status  *mapsv1.ImageGenerationStatus
	created bool
}

// racedReservation answers a try whose insert lost the race for its key to
// another try: the request that one made, if it is the same request.
func (s *Service) racedReservation(ctx context.Context, existing mapsdb.ImageRequest, n newRequest, campaignID string) (mapsdb.ImageRequest, *mapsv1.ImageGenerationStatus, error) {
	if err := sameImageRequest(existing, n); err != nil {
		return mapsdb.ImageRequest{}, nil, err
	}
	status, err := s.freshStatus(ctx, campaignID)
	if err != nil {
		return mapsdb.ImageRequest{}, nil, s.dbError(ctx, "read the image generation status", err)
	}
	return existing, status, nil
}

// reserveTx is one attempt of reserve's transaction: the replay of an earlier
// try with the same key, or the checks in their order and the insert.
func (s *Service) reserveTx(ctx context.Context, q *mapsdb.Queries, n *newRequest, campaignID string) (reservation, error) {
	// A retry with the same key: the first request, whatever it became.
	existing, err := q.GetImageRequestByKey(ctx, mapsdb.GetImageRequestByKeyParams{CampaignID: campaignID, IdempotencyKey: n.key})
	if err == nil {
		if err := sameImageRequest(existing, *n); err != nil {
			return reservation{}, err
		}
		status, err := s.statusIn(ctx, q, campaignID)
		return reservation{row: existing, status: status}, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return reservation{}, fmt.Errorf("find the request by its key: %w", err)
	}
	if err := s.expireStale(ctx, q, campaignID); err != nil {
		return reservation{}, err
	}
	status, err := s.statusIn(ctx, q, campaignID)
	if err != nil {
		return reservation{}, err
	}
	if err := s.checkRoom(ctx, q, campaignID, status); err != nil {
		return reservation{}, err
	}
	if err := checkReferences(ctx, q, campaignID, *n); err != nil {
		return reservation{}, err
	}
	var inherited *basisParams
	if n.kind == kindEdit {
		if inherited, err = adjustEdit(ctx, q, campaignID, n); err != nil {
			return reservation{}, err
		}
	}
	// The server holds only so many requests alive. Two requests that pass this at
	// once may both be taken: the cap is a bound on memory, not an exact count.
	if s.pending.Load() >= maxPendingRequests {
		return reservation{}, errTooManyImageRequests()
	}
	row, err := s.insertRequest(ctx, q, campaignID, *n, inherited)
	if err != nil {
		return reservation{}, err
	}
	status.UsedThisMonth++
	status.Remaining = max(status.GetRemaining()-1, 0)
	return reservation{row: row, status: status, created: true}, nil
}

// checkRoom refuses a request the service is off for, or the month's, the
// day's or the gallery's room has none left for.
func (s *Service) checkRoom(ctx context.Context, q *mapsdb.Queries, campaignID string, status *mapsv1.ImageGenerationStatus) error {
	switch {
	case !status.GetEnabled():
		return errBlocked(mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_OFF, status)
	case status.GetRemaining() <= 0:
		return errBlocked(mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_LIMIT_REACHED, status)
	}
	// The server's cap of the day, counted in the transaction that inserts, so
	// two requests for the last slot cannot both get it.
	made, err := q.CountImageRequestsSince(ctx, dayStart(s.now()))
	if err != nil {
		return fmt.Errorf("count the day's generated images: %w", err)
	}
	if made >= s.dailyImages {
		return errBlocked(mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_DAILY_LIMIT_REACHED, status)
	}
	usage, err := q.GetGalleryUsage(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("read the gallery usage: %w", err)
	}
	open, err := q.CountOpenImageRequests(ctx, mapsdb.CountOpenImageRequestsParams{CampaignID: campaignID, CreatedAt: s.now().Add(-staleAfter)})
	if err != nil {
		return fmt.Errorf("count the requests in flight: %w", err)
	}
	// The requests in flight will each put an image in the gallery: they count
	// against its room, so a full gallery cannot fail after the model was paid.
	if usage.ImageCount+open >= s.maxImages || usage.ByteCount >= int64(s.maxBytes) {
		return errBlocked(mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_GALLERY_FULL, status)
	}
	return nil
}

// checkReferences refuses a request whose reference or character image is not
// in the campaign's gallery.
func checkReferences(ctx context.Context, q *mapsdb.Queries, campaignID string, n newRequest) error {
	for _, id := range append(append([]string{}, n.references...), n.characters...) {
		if _, err := q.GetGalleryImageInCampaign(ctx, mapsdb.GetGalleryImageInCampaignParams{CampaignID: campaignID, ID: id}); errors.Is(err, pgx.ErrNoRows) {
			return errImageNotFound()
		} else if err != nil {
			return fmt.Errorf("find a reference image: %w", err)
		}
	}
	return nil
}

// adjustEdit finds the image an edit adjusts, which must be a generated one of
// the campaign with its request: the edit keeps its style and ratio, and gets
// the default name. For a textured map it also returns the map basis the edit
// inherits.
func adjustEdit(ctx context.Context, q *mapsdb.Queries, campaignID string, n *newRequest) (*basisParams, error) {
	src, err := q.GetGalleryImageInCampaign(ctx, mapsdb.GetGalleryImageInCampaignParams{CampaignID: campaignID, ID: *n.source})
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !src.Generated {
		return nil, errImageNotFound()
	}
	if err != nil {
		return nil, fmt.Errorf("find the image to edit: %w", err)
	}
	from, err := q.GetImageRequestForImage(ctx, mapsdb.GetImageRequestForImageParams{CampaignID: campaignID, ImageID: n.source})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errImageNotFound()
	}
	if err != nil {
		return nil, fmt.Errorf("find the request of the image to edit: %w", err)
	}
	n.style, n.ratio = from.Style, from.AspectRatio
	if src.GeneratedKind == kindTexturedMap && from.MapWidth != nil && from.MapHeight != nil {
		// An edit of a textured map comes back at the map's own proportions (the original's ratio is the padded
		// canvas's, not the map's): the model ratio nearest the map image's, so little is cut and nothing is stretched.
		n.ratio = refimg.Closest(int(*from.MapWidth), int(*from.MapHeight))
	}
	if n.name == "" {
		n.name = suffixedName(src.Name, " (ajuste)")
	}
	if src.GeneratedKind != kindTexturedMap {
		return nil, nil
	}
	// An edit of a textured map is a textured map: it keeps the map it fits (the image, the grid, the
	// size and the walls it started from), so it is made at the map's size and "Usar como imagem do
	// mapa" checks the same things (RN-10).
	return &basisParams{
		mapID: from.MapID, imageID: from.MapImageID, gridColumns: from.MapGridColumns, gridFactor: from.MapGridFactor,
		width: from.MapWidth, height: from.MapHeight, planHash: from.MapPlanHash,
		pad: [4]*float64{from.PadX0, from.PadY0, from.PadX1, from.PadY1},
	}, nil
}

// insertRequest numbers the request and inserts its row, with the map basis of
// the edit it inherits, or else the prepared one.
func (s *Service) insertRequest(ctx context.Context, q *mapsdb.Queries, campaignID string, n newRequest, inherited *basisParams) (mapsdb.ImageRequest, error) {
	number, err := q.NextImageRequestNumber(ctx, campaignID)
	if err != nil {
		return mapsdb.ImageRequest{}, fmt.Errorf("number the request: %w", err)
	}
	month, _ := monthOf(s.now())
	id := uuid.New().String()
	basis := n.prepared.basis.params()
	if inherited != nil {
		basis = *inherited
	}
	row, err := q.InsertImageRequest(ctx, mapsdb.InsertImageRequestParams{
		ID: id, CampaignID: campaignID, RequestedBy: &n.requestedBy, IdempotencyKey: n.key,
		Kind: n.kind, Prompt: n.prompt, Style: n.style, AspectRatio: n.ratio, Model: s.generator.Model(),
		ReferenceIds: nonNil(n.references), CharacterIds: nonNil(n.characters), SourceImageID: n.source, Number: number, QuotaMonth: month, CreatedAt: s.now(),
		MapID: basis.mapID, MapImageID: basis.imageID, MapGridColumns: basis.gridColumns, MapGridFactor: basis.gridFactor, MapWidth: basis.width, MapHeight: basis.height,
		MapPlanHash: basis.planHash, PadX0: basis.pad[0], PadY0: basis.pad[1], PadX1: basis.pad[2], PadY1: basis.pad[3],
		ImageName: imageNameFor(n), IdempotencyHash: n.hash,
	})
	if err != nil {
		return mapsdb.ImageRequest{}, fmt.Errorf("insert the request: %w", err)
	}
	return row, nil
}
