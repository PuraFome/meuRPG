package maps

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/maps/images"
	"github.com/PuraFome/meuRPG/backend/internal/maps/images/gen"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/internal/platform/safego"
)

// The goroutine of an image request: the call to the model, the storing of the picture,
// and the refund when it ends without one (steps 2 and 3 of imagegen.go).

// CancelGenerations is the shutdown: the generation goroutines stop, a request
// still waiting for a call slot is refunded at once, a long poll answers, and a
// call already sent is cut off (it follows the expiry of a sent request: its
// slot stays spent). Call WaitForGenerations after it.
func (s *Service) CancelGenerations() { s.cancelBase() }

// WaitForGenerations waits for the requests in flight, up to ctx: the shutdown
// (and a test's cleanup) uses it, so no goroutine writes after the pool closes.
func (s *Service) WaitForGenerations(ctx context.Context) {
	done := make(chan struct{})
	go func() { s.generations.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

// generationWaiters wakes the long polls of one instance: a request's goroutine
// (or a cancel) calls notify, and every GetImageGeneration waiting on it reads
// the row again. One instance only: Cloud Run runs max-instances 1
// (docs/operations/README.md); with more, the poll still ends at its deadline.
type generationWaiters struct {
	mu sync.Mutex
	m  map[string]map[chan struct{}]struct{}
}

// register returns a channel closed by the next notify of id, and a function
// that forgets it.
func (w *generationWaiters) register(id string) (<-chan struct{}, func()) {
	ch := make(chan struct{})
	w.mu.Lock()
	if w.m == nil {
		w.m = map[string]map[chan struct{}]struct{}{}
	}
	if w.m[id] == nil {
		w.m[id] = map[chan struct{}]struct{}{}
	}
	w.m[id][ch] = struct{}{}
	w.mu.Unlock()
	return ch, func() {
		w.mu.Lock()
		delete(w.m[id], ch)
		if len(w.m[id]) == 0 {
			delete(w.m, id)
		}
		w.mu.Unlock()
	}
}

func (w *generationWaiters) notify(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for ch := range w.m[id] {
		close(ch)
		delete(w.m[id], ch)
	}
}

// errRequestClosed: the request can no longer take an image (it ended another
// way, or already has one). The gallery insert that asked is rolled back.
var errRequestClosed = errors.New("maps: the image request is closed")

// run is a request's goroutine: it waits for a call slot, marks the request
// sent, calls the model outside any transaction, stores the picture together
// with the request's end, and records how it ended. It never uses the caller's
// context (the master may close the page): it derives from the service's.
func (s *Service) run(campaignID, id string, prep prepared) {
	defer s.generations.Done()
	defer s.pending.Add(-1)
	defer s.waiters.notify(id)
	ctx, cancel := context.WithTimeout(s.baseCtx, generationTimeout)
	defer cancel()
	// This goroutine decodes and draws what a model sent back, and a panic in a
	// goroutine the server started itself would take the whole process down (the
	// net/http recover only covers a handler's own goroutine). So a panic fails
	// this one request: logged with its stack, recorded as failed. Declared last,
	// so it runs first, before the waiters are woken and the slot is given back.
	defer safego.Recover(s.logger, "image request", func() { s.finishFailed(campaignID, id, stateFailed, reasonUnavailable) })
	release, ok := s.acquireCallSlot(ctx, campaignID, id)
	defer release()
	if !ok {
		return
	}
	row, request, ok := s.readyToSend(ctx, campaignID, id, prep)
	if !ok {
		return
	}
	picture, err := s.generator.Generate(ctx, request)
	if err != nil {
		s.endWithoutPicture(ctx, campaignID, id, err)
		return
	}
	s.storePicture(ctx, campaignID, id, row, picture.Data)
}

// acquireCallSlot waits for one of the calls to the model that may run at once. ok is
// false when the request ends there: a slot freed by the shutdown itself, or the
// request's own end before it had one (never sent: the slot goes back at once). The
// release is always safe to defer.
func (s *Service) acquireCallSlot(ctx context.Context, campaignID, id string) (release func(), ok bool) {
	select {
	case s.generating <- struct{}{}:
		release = func() { <-s.generating }
		if s.baseCtx.Err() != nil { // a slot freed by the shutdown itself
			s.finishFailed(campaignID, id, stateFailed, reasonShutdown)
			return release, false
		}
		return release, true
	case <-ctx.Done():
		s.finishFailed(campaignID, id, stateFailed, endReason(s.baseCtx))
		return func() {}, false
	}
}

// readyToSend reads the pending request, builds what the model gets and marks the
// request as sent. ok is false when there is nothing to send: it was canceled before
// it started, it cannot be built, the shutdown came, or a Cancel came first.
func (s *Service) readyToSend(ctx context.Context, campaignID, id string, prep prepared) (row mapsdb.ImageRequest, request gen.Request, ok bool) {
	row, err := s.queries.GetImageRequest(ctx, mapsdb.GetImageRequestParams{CampaignID: campaignID, ID: id})
	if err != nil {
		s.logger.ErrorContext(ctx, "maps: cannot read an image request to run it", "error", err)
		s.finishFailed(campaignID, id, stateFailed, reasonUnavailable)
		return row, request, false
	}
	if row.Status != statePending {
		return row, request, false // canceled before it started
	}
	request, reason, err := s.modelRequest(ctx, row, prep)
	if err != nil {
		s.logger.ErrorContext(ctx, "maps: cannot build an image request", "error", err, "reason", reason)
		s.finishFailed(campaignID, id, stateFailed, reason)
		return row, request, false
	}
	if s.baseCtx.Err() != nil {
		s.finishFailed(campaignID, id, stateFailed, reasonShutdown)
		return row, request, false
	}
	// The request leaves now. A Cancel that came first wins: nothing is sent.
	sent, err := s.queries.MarkImageRequestSent(ctx, mapsdb.MarkImageRequestSentParams{CampaignID: campaignID, ID: id, SentAt: new(s.now())})
	if err != nil {
		s.logger.ErrorContext(ctx, "maps: cannot mark an image request as sent", "error", err)
		s.finishFailed(campaignID, id, stateFailed, reasonUnavailable)
		return row, request, false
	}
	return row, request, sent != 0
}

// endWithoutPicture records how a call that returned an error ended. The slot goes
// back only when the call surely was not billed.
func (s *Service) endWithoutPicture(ctx context.Context, campaignID, id string, err error) {
	if s.baseCtx.Err() != nil {
		// Cut off by the shutdown: the call may have been billed, so the slot
		// is not given back. The request expires as a sent one (expireStale).
		return
	}
	state, why := stateFailed, reasonUnavailable
	certainlyFree := false // the slot goes back only when the call surely was not billed
	switch refused := (*gen.RefusedError)(nil); {
	case errors.As(err, &refused):
		state, why, certainlyFree = stateRefused, reasonRefused, true
	case errors.Is(err, gen.ErrNoImage):
		why, certainlyFree = reasonNoImage, true
	case errors.Is(err, gen.ErrNotAuthorized):
		// The operator's problem: the key was refused. Say it loudly, once.
		why, certainlyFree = reasonServiceOff, true
		s.logger.ErrorContext(ctx, "maps: the image service refused the API key", "error", err)
	}
	// The error says a status or a kind, never the text (package gen).
	s.logger.WarnContext(ctx, "maps: an image request ended without a picture", "reason", why, "error", err)
	if certainlyFree {
		s.finishFailed(campaignID, id, state, why)
	} else {
		// A timeout, a cut connection or an answer that cannot be read may have been billed.
		s.finishSpent(campaignID, id, why)
	}
}

// storePicture stores the picture the model returned together with the request's end,
// on a context of its own, even in a shutdown.
func (s *Service) storePicture(ctx context.Context, campaignID, id string, row mapsdb.ImageRequest, data []byte) {
	fin, cancelFin := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancelFin()
	kind := s.generatedKindOf(fin, row)
	res, err := s.processPicture(fin, row, kind, data)
	if err != nil {
		s.logger.WarnContext(fin, "maps: the generated picture cannot be used", "error", err)
		s.finishSpent(campaignID, id, reasonNoImage)
		return
	}
	var parent *string
	if row.Kind == kindEdit {
		parent = row.SourceImageID
	}
	name := row.ImageName
	if name == "" {
		name = "Imagem " + strconv.Itoa(int(row.Number))
	}
	// The gallery row and the request's end go in one transaction, allowed once.
	stored, err := s.storeImage(fin, campaignID, row.RequestedBy, name, res, true, kind, parent,
		func(ctx context.Context, q *mapsdb.Queries, imageID string) error {
			n, err := q.FinishImageRequestDone(ctx, mapsdb.FinishImageRequestDoneParams{CampaignID: campaignID, ID: id, ImageID: &imageID, Now: new(s.now())})
			if err != nil {
				return fmt.Errorf("finish the request: %w", err)
			}
			if n == 0 {
				return errRequestClosed
			}
			return nil
		})
	if err != nil {
		s.storeFailed(fin, campaignID, id, err)
		return
	}
	s.logger.InfoContext(fin, "maps: an image was generated", "campaign", campaignID, "request", id, "image", stored.ID)
	// The background context has no request fields, so the campaign is explicit.
	logging.Event(fin, s.logger, "image.generated", slog.String("campaign_id", campaignID), slog.String("generation_id", id), slog.String("image_id", stored.ID), slog.String("kind", kind))
}

// processPicture makes the gallery picture of what the model returned, by the way the
// request was made.
func (s *Service) processPicture(ctx context.Context, row mapsdb.ImageRequest, kind string, data []byte) (*images.Result, error) {
	switch {
	case row.Kind == kindTexturedMap:
		// The textured map is the map's rectangle of the padded canvas, in the size of the map's image.
		return s.cropToMap(ctx, data, row)
	case row.Kind == kindEdit && kind == kindTexturedMap && row.MapWidth != nil && row.MapHeight != nil:
		// An edit of a textured map keeps the map's size, so it can be used as the map's image too.
		return s.fitToMap(ctx, data, row)
	default:
		return s.process(ctx, data)
	}
}

// storeFailed records why a picture the model returned (the call was billed) could not
// be stored: the slot stays spent, unless the request was closed meanwhile.
func (s *Service) storeFailed(ctx context.Context, campaignID, id string, err error) {
	why := reasonUnavailable
	if he, ok := errors.AsType[*httpError](err); ok && he.reason == ReasonQuota {
		why = reasonGalleryFull
	}
	if errors.Is(err, errRequestClosed) {
		s.logger.WarnContext(ctx, "maps: a generated picture arrived for a request that is closed; it was dropped")
		return
	}
	s.logger.ErrorContext(ctx, "maps: cannot store a generated image", "error", err, "reason", why)
	s.finishSpent(campaignID, id, why)
}

// generatedKindOf is the way a request's picture was made, as the gallery records it: the
// request's own kind, and for an edit the way of the image it adjusts (a chain keeps the way
// of its root). An edit that carries a map basis is a textured map's even if its source is gone.
func (s *Service) generatedKindOf(ctx context.Context, row mapsdb.ImageRequest) string {
	if row.Kind != kindEdit {
		return row.Kind
	}
	if row.SourceImageID != nil {
		if src, err := s.queries.GetGalleryImageInCampaign(ctx, mapsdb.GetGalleryImageInCampaignParams{CampaignID: row.CampaignID, ID: *row.SourceImageID}); err == nil {
			return src.GeneratedKind
		}
	}
	if row.MapID != nil {
		return kindTexturedMap
	}
	return ""
}

// endReason says why a request that never left stops: the shutdown, or its
// own timeout.
func endReason(base context.Context) string {
	if base.Err() != nil {
		return reasonShutdown
	}
	return reasonTimeout
}

// finishFailed ends a request that made no picture: the slot goes back.
func (s *Service) finishFailed(campaignID, id, state, reason string) {
	s.finish(campaignID, id, state, reason, true)
}

// finishSpent ends a request whose picture the model returned but the server could
// not store: the call was made, so the slot stays spent.
func (s *Service) finishSpent(campaignID, id, reason string) {
	s.finish(campaignID, id, stateFailed, reason, false)
}

func (s *Service) finish(campaignID, id, state, reason string, refund bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var err error
	if refund {
		err = s.queries.FinishImageRequestFailed(ctx, mapsdb.FinishImageRequestFailedParams{
			Now: new(s.now()), Status: state, Reason: reason, CampaignID: campaignID, ID: id,
		})
	} else {
		err = s.queries.FinishImageRequestSpent(ctx, mapsdb.FinishImageRequestSpentParams{
			Now: new(s.now()), Status: state, Reason: reason, CampaignID: campaignID, ID: id,
		})
	}
	if err != nil {
		s.logger.ErrorContext(ctx, "maps: cannot record a failed image request", "error", err)
		return
	}
	// Never the prompt: the ids, and the closed reason.
	name := "image.failed"
	if state == stateRefused {
		name = "image.refused"
	}
	logging.Event(ctx, s.logger, name, slog.String("campaign_id", campaignID), slog.String("generation_id", id), slog.String("reason", reason))
}

// modelRequest builds what goes to the model from the stored request and the
// images prepared before the slot was reserved (shrunk, from the campaign's own
// gallery, ADR-0012). The text is the master's; the only thing added is the
// style's name. Nothing else goes: not a name, an e-mail or a sheet.
func (s *Service) modelRequest(ctx context.Context, row mapsdb.ImageRequest, prep prepared) (gen.Request, string, error) {
	req := gen.Request{AspectRatio: row.AspectRatio, Style: stylePhrases[styleFromKey(row.Style)], References: prep.references}
	if row.Kind == kindEdit {
		edit, err := s.editOf(ctx, row, prep)
		if err != nil {
			return gen.Request{}, reasonImageMissing, err
		}
		req.Edit = edit
	} else {
		req.Prompt = row.Prompt
	}
	if isMapKind(row.Kind) {
		if prep.drawing == nil {
			return gen.Request{}, reasonImageMissing, errors.New("a request made from a map has no drawing")
		}
		req.Drawing, req.Layout, req.Rooms = prep.drawing, layoutOf(row.Kind), prep.rooms
	}
	return req, "", nil
}

// editOf collects an edit's inputs: the previous image, the text that made the
// chain's first image, the adjustments since, and the new instruction.
func (s *Service) editOf(ctx context.Context, row mapsdb.ImageRequest, prep prepared) (*gen.Edit, error) {
	if row.SourceImageID == nil || prep.previous == nil {
		return nil, errors.New("an edit has no source image")
	}
	chain, err := s.queries.ListImageChain(ctx, mapsdb.ListImageChainParams{CampaignID: row.CampaignID, ID: *row.SourceImageID})
	if err != nil {
		return nil, fmt.Errorf("read the edit chain: %w", err)
	}
	byID := map[string]mapsdb.ListImageChainRow{}
	for _, c := range chain {
		byID[c.ID] = c
	}
	// The path from the source up to the chain's root, then turned around.
	var path []mapsdb.ListImageChainRow
	for cur, ok := byID[*row.SourceImageID]; ok; {
		path = append(path, cur)
		if cur.ParentImageID == nil {
			break
		}
		cur, ok = byID[*cur.ParentImageID]
	}
	edit := &gen.Edit{Previous: *prep.previous, Instruction: row.Prompt}
	for i, p := range slices.Backward(path) {
		if i == len(path)-1 {
			edit.Original = p.Prompt
		} else {
			edit.Adjustments = append(edit.Adjustments, p.Prompt)
		}
	}
	return edit, nil
}

// nonNil is ids, or an empty list: the columns are NOT NULL, and a nil slice is NULL.
func nonNil(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}
