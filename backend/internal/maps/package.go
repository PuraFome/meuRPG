package maps

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
	"github.com/PuraFome/meuRPG/backend/internal/maps/images"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The maps module's share of the campaign package (MR-050): the gallery with
// its image files, and the maps with their layers, points, scene actions and
// clues. Every value goes through the same checks the calls that make these
// things by hand use (cleanName, specFor, checkPosition, the scene check
// catalog, the grid and layer decoders), and every image through the upload's
// own pipeline (images.Process), so a package cannot put in a campaign what the
// master could not make.
//
// What a package leaves out is play: a map comes back as prepared, with its
// traps armed and its treasures unfound, and nothing a player found out, saw
// or remembered. Which maps and points are revealed is preparation, and stays.

// PackageImagesPart is the gallery's part of the package.
func (s *Service) PackageImagesPart() campaignpackage.Part { return &imagesPart{s: s} }

// PackageMapsPart is the maps' part of the package.
func (s *Service) PackageMapsPart() campaignpackage.Part { return &mapsPart{s: s} }

const (
	stashImages = "maps.images"
	stashMaps   = "maps.maps"
	nsImage     = "image"
	nsMap       = "map"
	nsPoint     = "point"
	nsClue      = "clue"
)

// orderStep is the time between two things of the same kind made from one
// package, so the lists keep the package's order.
const orderStep = time.Millisecond

// --- the gallery ------------------------------------------------------------

type imagesPart struct{ s *Service }

// stagedImage is an image read and checked, ready to be written.
type stagedImage struct {
	oldID, id, name string
	contentType     string
	width, height   int
	size            int
	generated       bool
	generatedKind   string
	parentOld       string
	copyOld         string
	createdAt       time.Time
}

type stagedImages struct {
	list  []*stagedImage
	byOld map[string]*stagedImage
}

// EstimateBytes implements campaignpackage.Estimator.
func (p *imagesPart) EstimateBytes(ctx context.Context, campaignID string) (int64, error) {
	usage, err := p.s.queries.GetGalleryUsage(ctx, campaignID)
	if err != nil {
		return 0, fmt.Errorf("read the gallery usage: %w", err)
	}
	return usage.ByteCount, nil
}

// Export implements campaignpackage.Part.
func (p *imagesPart) Export(ctx context.Context, tx pgx.Tx, campaignID string, snap *campaignpackage.Snapshot) error {
	rows, err := p.s.queries.WithTx(tx).ListGalleryImages(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("list the gallery: %w", err)
	}
	list := &pkgv1.PackageImages{}
	for _, r := range rows {
		file := campaignpackage.EntryName("images", snap.Next("images"), "")
		imageKey, _ := blobKeys(campaignID, r.ID)
		snap.AddBlob(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE, file, imageKey, int64(r.ByteSize))
		img := &pkgv1.PackageImage{Id: r.ID, Name: r.Name, File: file, Generated: r.Generated, GeneratedKind: r.GeneratedKind}
		if r.ParentImageID != nil {
			img.ParentImageId = *r.ParentImageID
		}
		if r.CopyOfImageID != nil {
			img.CopyOfImageId = *r.CopyOfImageID
		}
		list.Images = append(list.Images, img)
	}
	return snap.AddMessage(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGES, campaignpackage.ImagesEntry, list)
}

// generatedKinds are the ways a generated image can have been made.
var generatedKinds = map[string]bool{"": true, "scene": true, "map_scene": true, "isometric": true, "textured_map": true}

// Stage implements campaignpackage.Part. It reads every image file, one at a
// time, through the upload's pipeline (and its one-at-a-time memory slot), and
// with in.Commit it stores the re-encoded files under the new campaign.
func (p *imagesPart) Stage(ctx context.Context, in *campaignpackage.Import) error {
	staged := &stagedImages{byOld: map[string]*stagedImage{}}
	in.Set(stashImages, staged)
	list, ok := p.readImageList(in)
	if !ok {
		return nil
	}
	s := p.s
	base := s.now()
	used := map[string]bool{}
	var bytesTotal int64
	for i, img := range list.GetImages() {
		bytesTotal += p.stageListed(ctx, in, staged, used, img, base.Add(-time.Duration(i)*orderStep)) // the list is newest first
	}
	image := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_IMAGE
	for _, f := range in.Pkg.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE) {
		if !used[f.GetPath()] {
			in.Problem(image, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNEXPECTED_ENTRY, 0)
		}
	}
	if bytesTotal > int64(s.maxBytes) {
		in.Problem(image, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_LIMIT, int64(s.maxBytes))
	}
	return nil
}

// readImageList reads the package's list of images. It says whether there is a
// list to stage: a package with no list stages nothing (and has no image
// files), and one with a list that is not the one entry, or is too long, is a
// problem and stages nothing either.
func (p *imagesPart) readImageList(in *campaignpackage.Import) (*pkgv1.PackageImages, bool) {
	image := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_IMAGE
	entries := in.Pkg.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGES)
	if len(entries) == 0 {
		if len(in.Pkg.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE)) > 0 {
			in.Problem(image, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_MISSING_ENTRY, 0)
		}
		return nil, false
	}
	if len(entries) != 1 || entries[0].GetPath() != campaignpackage.ImagesEntry {
		in.Problem(image, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNEXPECTED_ENTRY, 0)
		return nil, false
	}
	var list pkgv1.PackageImages
	if !in.Read(campaignpackage.ImagesEntry, &list) {
		return nil, false
	}
	if int32(len(list.GetImages())) > p.s.maxImages { //nolint:gosec // G115: a list of at most 2,000 entries
		in.Problem(image, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_LIMIT, int64(p.s.maxImages))
		return nil, false
	}
	return &list, true
}

// stageListed checks one image of the list and stages it. It returns the bytes it
// adds to the gallery, 0 for an image with a problem (recorded).
func (p *imagesPart) stageListed(ctx context.Context, in *campaignpackage.Import, staged *stagedImages, used map[string]bool, img *pkgv1.PackageImage, createdAt time.Time) int64 {
	image := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_IMAGE
	name, err := names.Clean(img.GetName(), maxNameLength)
	if err != nil {
		in.Problem(image, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
		return 0
	}
	id, ok := in.IDs.Define(nsImage, img.GetId())
	if !ok || !generatedKinds[img.GetGeneratedKind()] || (img.GetGeneratedKind() != "" && !img.GetGenerated()) {
		in.Problem(image, name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
		return 0
	}
	entry, ok := claimImageFile(in, used, img.GetFile(), name)
	if !ok {
		return 0
	}
	si := &stagedImage{
		oldID: img.GetId(), id: id, name: name, generated: img.GetGenerated(), generatedKind: img.GetGeneratedKind(),
		createdAt: createdAt,
	}
	// Ref records that the package must define the two images; their new ids are looked
	// up by the old ones when the links are written (linkStagedImages).
	in.IDs.Ref(nsImage, img.GetParentImageId(), image, name)
	in.IDs.Ref(nsImage, img.GetCopyOfImageId(), image, name)
	si.parentOld, si.copyOld = img.GetParentImageId(), img.GetCopyOfImageId()
	if !p.stageFile(ctx, in, si, img.GetFile()) {
		return 0
	}
	staged.list = append(staged.list, si)
	staged.byOld[si.oldID] = si
	in.Counts.Images++
	in.Counts.ImageBytes += entry.GetSize()
	return int64(si.size)
}

// claimImageFile finds the image file an entry of the list names, which must be an
// image file that no other image of the list uses, and marks it used.
func claimImageFile(in *campaignpackage.Import, used map[string]bool, file, name string) (*pkgv1.PackageEntry, bool) {
	image := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_IMAGE
	entry, ok := in.Pkg.Entry(file)
	switch {
	case !ok || entry.GetKind() != pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE:
		in.Problem(image, name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_MISSING_ENTRY, 0)
		return nil, false
	case used[file]:
		in.Problem(image, name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_DUPLICATE, 0)
		return nil, false
	}
	used[file] = true
	return entry, true
}

// stageFile reads one image file through the upload's pipeline. It reports
// whether the image is good.
func (p *imagesPart) stageFile(ctx context.Context, in *campaignpackage.Import, si *stagedImage, file string) bool {
	s := p.s
	image := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_IMAGE
	data, err := in.Pkg.ReadBytes(file)
	if err != nil {
		in.Problem(image, si.name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_MISSING_ENTRY, 0)
		return false
	}
	res, err := s.process(ctx, data)
	data = nil //nolint:ineffassign,wastedassign // let the bytes go before the files are written
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		in.Problem(image, si.name, imageReason(err), images.MaxBytes)
		return false
	}
	si.contentType, si.width, si.height, si.size = res.ContentType, res.Width, res.Height, len(res.Data)
	if !in.Commit {
		return true
	}
	imageKey, thumbKey := blobKeys(in.CampaignID, si.id)
	files := []struct {
		key, contentType string
		content          []byte
	}{{imageKey, res.ContentType, res.Data}, {thumbKey, res.ContentType, res.Thumbnail}}
	if res.Reference != nil {
		files = append(files, struct {
			key, contentType string
			content          []byte
		}{referenceKey(in.CampaignID, si.id), images.JPEG, res.Reference})
	}
	for _, f := range files {
		if err := in.WriteBlob(ctx, f.key, f.contentType, bytes.NewReader(f.content)); err != nil {
			s.logger.ErrorContext(ctx, "maps: cannot store an imported image file", "error", err)
			in.Problem(image, si.name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
			return false
		}
	}
	return true
}

// imageReason is the problem an image Process refuses becomes.
func imageReason(err error) pkgv1.PackageProblemReason {
	switch {
	case errors.Is(err, images.ErrTooLarge):
		return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_ENTRY_TOO_BIG
	case errors.Is(err, images.ErrDimensions):
		return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_IMAGE_DIMENSIONS
	default:
		return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_IMAGE_UNREADABLE
	}
}

// Apply implements campaignpackage.Part.
func (p *imagesPart) Apply(ctx context.Context, tx pgx.Tx, in *campaignpackage.Import) error {
	staged, _ := in.Get(stashImages).(*stagedImages)
	if staged == nil {
		return nil
	}
	q := p.s.queries.WithTx(tx)
	if err := insertStagedImages(ctx, q, in, staged); err != nil {
		return err
	}
	return linkStagedImages(ctx, q, in, staged)
}

// insertStagedImages writes the rows of the staged images.
func insertStagedImages(ctx context.Context, q *mapsdb.Queries, in *campaignpackage.Import, staged *stagedImages) error {
	for _, si := range staged.list {
		if err := q.InsertImportedGalleryImage(ctx, mapsdb.InsertImportedGalleryImageParams{
			ID: si.id, CampaignID: in.CampaignID, UploadedBy: &in.UserID, Name: si.name, ContentType: si.contentType,
			Width: int32(si.width), Height: int32(si.height), ByteSize: int32(si.size), //nolint:gosec // G115: at most images.MaxSide and MaxBytes
			CreatedAt: si.createdAt, Generated: si.generated, GeneratedKind: si.generatedKind,
		}); err != nil {
			return fmt.Errorf("insert an imported image: %w", err)
		}
	}
	return nil
}

// linkStagedImages points the rows at their parent and at the image they are
// the copy of, once every row exists.
func linkStagedImages(ctx context.Context, q *mapsdb.Queries, in *campaignpackage.Import, staged *stagedImages) error {
	for _, si := range staged.list {
		parent, copyOf := staged.refs(in, si)
		if parent == nil && copyOf == nil {
			continue
		}
		if err := q.SetImportedImageRefs(ctx, mapsdb.SetImportedImageRefsParams{CampaignID: in.CampaignID, ID: si.id, ParentImageID: parent, CopyOfImageID: copyOf}); err != nil {
			return fmt.Errorf("link an imported image: %w", err)
		}
	}
	return nil
}

// refs are the new ids of an image's parent and of the image it is the copy of.
func (st *stagedImages) refs(in *campaignpackage.Import, si *stagedImage) (parent, copyOf *string) {
	if id, ok := in.IDs.Get(nsImage, si.parentOld); ok {
		parent = &id
	}
	if id, ok := in.IDs.Get(nsImage, si.copyOld); ok {
		copyOf = &id
	}
	return parent, copyOf
}

// --- the maps ---------------------------------------------------------------

type mapsPart struct{ s *Service }

type stagedPoint struct {
	oldID     string
	id, mapID string
	kind      string
	name      string
	desc      string
	hooks     string
	showDC    bool
	x, y      int32
	target    *string
	spec      pointSpec
	stairs    *string
	revealed  bool
	createdAt time.Time
	actions   []stagedAction
	clues     []stagedClue
}

type stagedAction struct {
	key, name   string
	dc          *int32
	maxAttempts int32
}

type stagedClue struct{ id, text string }

type stagedMap struct {
	oldID       string
	grid        grid.Grid
	doors       *grid.DoorLayer
	id, name    string
	imageID     string
	columns     *int32
	factor      int32
	revealed    bool
	fog, onGrid bool
	group       bool
	baseLight   string
	layers      [6][]byte
	points      []*stagedPoint
	createdAt   time.Time
}

type stagedMapSet struct{ maps []*stagedMap }

// Export implements campaignpackage.Part.
func (p *mapsPart) Export(ctx context.Context, tx pgx.Tx, campaignID string, snap *campaignpackage.Snapshot) error {
	q := p.s.queries.WithTx(tx)
	maps, err := q.ListMapsOfCampaign(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("list the maps: %w", err)
	}
	for _, m := range maps {
		out, err := p.exportMap(ctx, q, m)
		if err != nil {
			return err
		}
		name := campaignpackage.EntryName("maps", snap.Next("maps"), ".json")
		if err := snap.AddMessage(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP, name, out); err != nil {
			return err
		}
	}
	return nil
}

// exportMap builds a map's entry: the map, its layers and its points with the
// actions and clues of the scenes.
func (p *mapsPart) exportMap(ctx context.Context, q *mapsdb.Queries, m mapsdb.Map) (*pkgv1.PackageMap, error) {
	out := &pkgv1.PackageMap{
		Id: m.ID, Name: m.Name, ImageId: m.ImageID, GridFactor: m.GridFactor, Revealed: m.RevealedAt != nil,
		FogEnabled: m.FogEnabled, FogOnFirstGrid: m.FogOnFirstGrid, GroupVision: m.GroupVision, BaseLight: m.BaseLight,
	}
	if m.GridColumns != nil {
		out.GridColumns = *m.GridColumns
	}
	var err error
	if out.Layers, err = exportLayers(ctx, q, m.ID); err != nil {
		return nil, err
	}
	points, err := q.ListMapPoints(ctx, m.ID)
	if err != nil {
		return nil, fmt.Errorf("list a map's points: %w", err)
	}
	actions, err := q.ListSceneActionsOfMap(ctx, m.ID)
	if err != nil {
		return nil, fmt.Errorf("list a map's scene actions: %w", err)
	}
	clues, err := q.ListSceneCluesOfMap(ctx, m.ID)
	if err != nil {
		return nil, fmt.Errorf("list a map's clues: %w", err)
	}
	for _, pt := range points {
		pp, err := p.exportPoint(pt)
		if err != nil {
			return nil, err
		}
		pp.Actions, pp.Clues = exportSceneRows(pt.ID, actions, clues)
		out.Points = append(out.Points, pp)
	}
	return out, nil
}

// exportLayers reads a map's painted layers, nil when it has none.
func exportLayers(ctx context.Context, q *mapsdb.Queries, mapID string) (*pkgv1.PackageLayers, error) {
	layers, err := q.GetMapLayers(ctx, mapID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read a map's layers: %w", err)
	}
	return &pkgv1.PackageLayers{
		DifficultTerrain: layers.DifficultTerrain, Walls: layers.Walls, Cover: layers.Cover, Light: layers.Light, Doors: layers.Doors, NoPass: layers.NoPass,
	}, nil
}

// exportSceneRows picks a point's actions and clues out of the map's.
func exportSceneRows(pointID string, actions []mapsdb.SceneAction, clues []mapsdb.SceneClue) (outActions []*pkgv1.PackageAction, outClues []*pkgv1.PackageClue) {
	for _, a := range actions {
		if a.PointID != pointID {
			continue
		}
		act := &pkgv1.PackageAction{Key: a.Key, Name: a.Name, MaxAttempts: a.MaxAttempts}
		if a.Dc != nil {
			act.Dc = *a.Dc
		}
		outActions = append(outActions, act)
	}
	for _, c := range clues {
		if c.PointID == pointID {
			outClues = append(outClues, &pkgv1.PackageClue{Id: c.ID, Text: c.Text})
		}
	}
	return outActions, outClues
}

func (p *mapsPart) exportPoint(pt mapsdb.MapPoint) (*pkgv1.PackagePoint, error) {
	out := &pkgv1.PackagePoint{
		Id: pt.ID, Kind: kindFromDB[pt.Kind], Name: pt.Name, Description: pt.Description, XBp: pt.XBp, YBp: pt.YBp,
		Revealed: pt.RevealedAt != nil, Hooks: pt.Hooks, ShowDc: pt.ShowDc, TreasureValuePo: pt.TreasureValuePo,
	}
	if pt.TargetMapID != nil {
		out.TargetMapId = *pt.TargetMapID
	}
	if pt.Stairs != nil {
		out.Stairs = *pt.Stairs
	}
	if pt.Trap != nil {
		spec := &mapsv1.TrapSpec{}
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(pt.Trap, spec); err != nil {
			return nil, fmt.Errorf("read a stored trap: %w", err)
		}
		out.Trap = spec
	}
	if pt.Kind == kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_LIGHT] && pt.LightBrightFt != nil && pt.LightDimFt != nil {
		out.Light = &mapsv1.LightSpec{BrightFt: *pt.LightBrightFt, DimFt: *pt.LightDimFt}
		if pt.LightPreset != nil {
			out.Light.PresetKey = *pt.LightPreset
		}
	}
	return out, nil
}

// Stage implements campaignpackage.Part.
func (p *mapsPart) Stage(_ context.Context, in *campaignpackage.Import) error {
	s := p.s
	entries := in.Pkg.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP)
	set := &stagedMapSet{}
	in.Set(stashMaps, set)
	if int32(len(entries)) > s.maxMaps { //nolint:gosec // G115: at most 2,000
		in.Problem(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_MAP, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_LIMIT, int64(s.maxMaps))
		return nil
	}
	imgs, _ := in.Get(stashImages).(*stagedImages)
	base := s.now()
	for i, e := range entries {
		var pm pkgv1.PackageMap
		if !in.Read(e.GetPath(), &pm) {
			continue
		}
		if sm := p.stageMap(in, &pm, imgs, base.Add(time.Duration(i)*orderStep)); sm != nil {
			set.add(in, sm)
		}
	}
	in.Facts.DoorAt = set.hasDoorAt
	return nil
}

// add keeps a staged map and tells the later parts what it holds.
func (set *stagedMapSet) add(in *campaignpackage.Import, sm *stagedMap) {
	set.maps = append(set.maps, sm)
	in.Counts.Maps++
	for _, pt := range sm.points {
		in.Facts.PointMap[pt.oldID] = sm.oldID
		in.Facts.PointKind[pt.oldID] = pt.kind
		in.Facts.PointName[pt.oldID] = pt.name
	}
}

// hasDoorAt says whether a staged map (by its package id) has a door on a square.
func (set *stagedMapSet) hasDoorAt(mapID string, col, row int) bool {
	for _, sm := range set.maps {
		if sm.oldID != mapID || sm.doors == nil {
			continue
		}
		sq := grid.Square{Col: col, Row: row}
		return sm.grid.Contains(sq) && sm.doors.At(sq) != grid.DoorNone
	}
	return false
}

// stageMap checks one map; nil when it has a problem (recorded).
func (p *mapsPart) stageMap(in *campaignpackage.Import, pm *pkgv1.PackageMap, imgs *stagedImages, createdAt time.Time) *stagedMap {
	mapKind := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_MAP
	name, err := names.Clean(pm.GetName(), maxNameLength)
	if err != nil {
		in.Problem(mapKind, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
		return nil
	}
	sm, ok := p.stageMapShell(in, pm, imgs, name, createdAt)
	if !ok {
		in.Problem(mapKind, name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
		return nil
	}
	if !p.stageMapPoints(in, pm, sm) {
		return nil
	}
	return sm
}

// stageMapShell checks everything of a map but its points: the id, the image,
// the base light, the grid and the painted layers. It says false for a map that
// is invalid (the caller records it).
func (p *mapsPart) stageMapShell(in *campaignpackage.Import, pm *pkgv1.PackageMap, imgs *stagedImages, name string, createdAt time.Time) (*stagedMap, bool) {
	id, ok := in.IDs.Define(nsMap, pm.GetId())
	if !ok {
		return nil, false
	}
	sm := &stagedMap{
		oldID: pm.GetId(), id: id, name: name, factor: pm.GetGridFactor(), revealed: pm.GetRevealed(), fog: pm.GetFogEnabled(), onGrid: pm.GetFogOnFirstGrid(),
		group: pm.GetGroupVision(), baseLight: pm.GetBaseLight(), createdAt: createdAt,
	}
	sm.imageID = in.IDs.Ref(nsImage, pm.GetImageId(), pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_MAP, name)
	if sm.baseLight != "dark" && sm.baseLight != "dim" && sm.baseLight != "bright" {
		return nil, false
	}
	g, ok := stageMapGrid(sm, pm, imgs)
	if !ok {
		return nil, false
	}
	sm.grid = g
	if !p.stageLayers(sm, pm.GetLayers(), g) {
		return nil, false
	}
	return sm, true
}

// stageMapGrid checks a map's grid and scale, and returns the engine's grid when
// the image is known. A map with no grid has no scale and no fog (SetMapFog
// needs a grid).
func stageMapGrid(sm *stagedMap, pm *pkgv1.PackageMap, imgs *stagedImages) (grid.Grid, bool) {
	cols := pm.GetGridColumns()
	if cols == 0 {
		return grid.Grid{}, sm.factor == 1 && !sm.fog
	}
	drawn := cols / max(sm.factor, 1)
	if sm.factor < 1 || sm.factor > grid.MaxFactor || cols%sm.factor != 0 || drawn < minGridColumns || drawn > maxGridColumns {
		return grid.Grid{}, false
	}
	sm.columns = &cols
	img := imgs.find(pm.GetImageId())
	if img == nil {
		return grid.Grid{}, true
	}
	engine, err := grid.EngineGrid(int(drawn), int(sm.factor), img.width, img.height)
	if err != nil || engine.Columns != int(cols) {
		return grid.Grid{}, false
	}
	return engine, true
}

// stageMapPoints checks a map's points and keeps the good ones in sm. It says
// false when the map has too many, or any has a problem (each recorded).
func (p *mapsPart) stageMapPoints(in *campaignpackage.Import, pm *pkgv1.PackageMap, sm *stagedMap) bool {
	s := p.s
	if int32(len(pm.GetPoints())) > s.maxPoints { //nolint:gosec // G115: bounded by the entry's size
		in.Problem(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_MAP, sm.name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_LIMIT, int64(s.maxPoints))
		return false
	}
	good := true
	for i, pp := range pm.GetPoints() {
		pt := p.stagePoint(in, pm, sm, pp, sm.createdAt.Add(time.Duration(i)*time.Microsecond))
		if pt == nil {
			good = false
			continue
		}
		sm.points = append(sm.points, pt)
	}
	return good
}

// find returns the staged image with this package id.
func (st *stagedImages) find(old string) *stagedImage {
	if st == nil {
		return nil
	}
	return st.byOld[old]
}

// stageLayers checks the painted layers against the map's grid. When the grid
// is not known (its image was refused, which is a problem of its own) the
// bytes are only kept.
func (p *mapsPart) stageLayers(sm *stagedMap, l *pkgv1.PackageLayers, g grid.Grid) bool {
	raw := [6][]byte{l.GetDifficultTerrain(), l.GetWalls(), l.GetCover(), l.GetLight(), l.GetDoors(), l.GetNoPass()}
	if sm.columns == nil {
		for _, b := range raw {
			if nilIfBlank(b) != nil {
				return false
			}
		}
		return true
	}
	if g.Columns == 0 {
		sm.layers = raw
		return true
	}
	checks := [6]func() error{
		func() error { _, err := grid.DecodeLayer(g, raw[0]); return err },
		func() error { _, err := grid.DecodeLayer(g, raw[1]); return err },
		func() error { _, err := grid.DecodeCoverLayer(g, raw[2]); return err },
		func() error { _, err := grid.DecodeLightLayer(g, raw[3]); return err },
		func() (err error) { sm.doors, err = grid.DecodeDoorLayer(g, raw[4]); return err },
		func() error { _, err := grid.DecodeLayer(g, raw[5]); return err },
	}
	for i, check := range checks {
		if len(raw[i]) == 0 {
			continue
		}
		if check() != nil {
			return false
		}
		sm.layers[i] = nilIfBlank(raw[i])
	}
	return true
}

// specReason is the problem a refused trap, treasure or light spec becomes:
// naming something the rules do not have is told from any other mistake.
func specReason(err error) pkgv1.PackageProblemReason {
	msg := ""
	if ce, ok := errors.AsType[*connect.Error](err); ok {
		msg = ce.Message()
	}
	for _, unknown := range []string{"preset", "is not a damage type", "is not a condition"} {
		if strings.Contains(msg, unknown) {
			return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNKNOWN_CONTENT
		}
	}
	return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID
}

// reasonNone says a check of a point found nothing wrong.
const reasonNone = pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNSPECIFIED

// stagePoint checks one point and its actions and clues; nil when it has a
// problem (recorded).
func (p *mapsPart) stagePoint(in *campaignpackage.Import, pm *pkgv1.PackageMap, sm *stagedMap, pp *pkgv1.PackagePoint, createdAt time.Time) *stagedPoint {
	problemKind := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_POINT
	name, err := cleanName("name", pp.GetName())
	if err != nil {
		in.Problem(problemKind, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
		return nil
	}
	kind, ok := kindToDB[pp.GetKind()]
	if ok && pp.GetKind() == mapsv1.MapPointKind_MAP_POINT_KIND_SCENE {
		problemKind = pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_SCENE
	}
	if !ok {
		in.Problem(problemKind, name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
		return nil
	}
	pt := &stagedPoint{oldID: pp.GetId(), kind: kind, name: name, x: pp.GetXBp(), y: pp.GetYBp(), showDC: pp.GetShowDc(), revealed: pp.GetRevealed(), createdAt: createdAt, mapID: sm.id}
	if reason := p.checkPoint(in, pm, pt, pp, problemKind); reason != reasonNone {
		in.Problem(problemKind, name, reason, 0)
		return nil
	}
	if !p.stageScene(in, pt, pp, problemKind) {
		return nil
	}
	countPoint(in.Counts, pp.GetKind())
	return pt
}

// checkPoint fills the checked fields of a point and returns the first thing wrong with it.
func (p *mapsPart) checkPoint(in *campaignpackage.Import, pm *pkgv1.PackageMap, pt *stagedPoint, pp *pkgv1.PackagePoint, problemKind pkgv1.PackageProblemKind) pkgv1.PackageProblemReason {
	invalid := pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID
	var ok bool
	if pt.id, ok = in.IDs.Define(nsPoint, pp.GetId()); !ok {
		return invalid
	}
	if reason := checkPointText(pt, pp); reason != reasonNone {
		return reason
	}
	if reason := stagePointTarget(in, pm, pt, pp, problemKind); reason != reasonNone {
		return reason
	}
	if reason := p.checkPointSpec(pt, pp); reason != reasonNone {
		return reason
	}
	if reason := checkPointStairs(pt, pp); reason != reasonNone {
		return reason
	}
	if pp.GetKind() != mapsv1.MapPointKind_MAP_POINT_KIND_SCENE && (len(pp.GetActions()) > 0 || len(pp.GetClues()) > 0) {
		return invalid
	}
	return reasonNone
}

// checkPointText checks the point's description, hooks and position, and that only a scene
// has hooks or shows its DCs.
func checkPointText(pt *stagedPoint, pp *pkgv1.PackagePoint) pkgv1.PackageProblemReason {
	var errs [4]error
	pt.desc, errs[0] = cleanDescription(pp.GetDescription())
	pt.hooks, errs[1] = cleanHooks(pp.GetHooks())
	errs[2] = checkPosition(pp.GetXBp(), pp.GetYBp())
	if (pt.hooks != "" || pt.showDC) && pp.GetKind() != mapsv1.MapPointKind_MAP_POINT_KIND_SCENE {
		errs[3] = errOnlyScenesHaveHooks()
	}
	if errors.Join(errs[:]...) != nil {
		return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID
	}
	return reasonNone
}

// stagePointTarget checks the map a point leads to, and notes it for the new id.
func stagePointTarget(in *campaignpackage.Import, pm *pkgv1.PackageMap, pt *stagedPoint, pp *pkgv1.PackagePoint, problemKind pkgv1.PackageProblemKind) pkgv1.PackageProblemReason {
	if pp.GetTargetMapId() == "" {
		return reasonNone
	}
	if !leads(pt.kind) || pp.GetTargetMapId() == pm.GetId() {
		return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID
	}
	target := in.IDs.Ref(nsMap, pp.GetTargetMapId(), problemKind, pt.name)
	if target != "" {
		pt.target = &target
	}
	return reasonNone
}

// checkPointSpec checks the point's trap, treasure or light as the master's own
// calls do. A trap comes back armed, whatever state it was left in.
func (p *mapsPart) checkPointSpec(pt *stagedPoint, pp *pkgv1.PackagePoint) pkgv1.PackageProblemReason {
	trap := pp.GetTrap()
	if trap != nil {
		trap = proto.Clone(trap).(*mapsv1.TrapSpec) //nolint:forcetypeassert // Clone returns the same type
		trap.State = mapsv1.TrapState_TRAP_STATE_UNSPECIFIED
	}
	var err error
	if pt.spec, err = p.s.specFor(pt.kind, trap, pp.TreasureValuePo, pp.GetLight()); err != nil {
		return specReason(err)
	}
	return reasonNone
}

// checkPointStairs checks a submap point's stairs.
func checkPointStairs(pt *stagedPoint, pp *pkgv1.PackagePoint) pkgv1.PackageProblemReason {
	if pp.GetStairs() == "" {
		return reasonNone
	}
	if pt.kind != kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP] || (pp.GetStairs() != "up" && pp.GetStairs() != "down") {
		return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID
	}
	stairs := pp.GetStairs()
	pt.stairs = &stairs
	return reasonNone
}

// countPoint counts a staged point for the preview, by its kind.
func countPoint(counts *pkgv1.PackageCounts, kind mapsv1.MapPointKind) {
	switch kind {
	case mapsv1.MapPointKind_MAP_POINT_KIND_SCENE:
		counts.Scenes++
	case mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE:
		counts.BattlePoints++
	case mapsv1.MapPointKind_MAP_POINT_KIND_TREASURE:
		counts.TreasurePoints++
	}
}

// stageScene checks a scene's actions and clues.
func (p *mapsPart) stageScene(in *campaignpackage.Import, pt *stagedPoint, pp *pkgv1.PackagePoint, kind pkgv1.PackageProblemKind) bool {
	s := p.s
	if len(pp.GetActions()) > maxSceneActions || len(pp.GetClues()) > maxSceneClues {
		in.Problem(kind, pt.name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_LIMIT, maxSceneActions)
		return false
	}
	for _, a := range pp.GetActions() {
		if _, ok := s.checks.SceneCheckName(a.GetKey()); !ok {
			in.Problem(kind, pt.name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNKNOWN_CONTENT, 0)
			return false
		}
		name, nerr := cleanActionName(a.GetName())
		dc, derr := cleanDC(a.GetDc())
		attempts, aerr := cleanAttempts(a.GetMaxAttempts())
		if err := errors.Join(nerr, derr, aerr); err != nil {
			in.Problem(kind, pt.name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
			return false
		}
		pt.actions = append(pt.actions, stagedAction{key: a.GetKey(), name: name, dc: dc, maxAttempts: attempts})
	}
	for _, c := range pp.GetClues() {
		text, err := cleanClueText(c.GetText())
		id, ok := in.IDs.Define(nsClue, c.GetId())
		if err != nil || !ok {
			in.Problem(kind, pt.name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
			return false
		}
		pt.clues = append(pt.clues, stagedClue{id: id, text: text})
	}
	return true
}

// Apply implements campaignpackage.Part.
func (p *mapsPart) Apply(ctx context.Context, tx pgx.Tx, in *campaignpackage.Import) error {
	set, _ := in.Get(stashMaps).(*stagedMapSet)
	if set == nil {
		return nil
	}
	q := p.s.queries.WithTx(tx)
	// Every map first: a point may lead to a map that comes later in the package.
	for _, sm := range set.maps {
		if err := insertStagedMap(ctx, q, in.CampaignID, sm); err != nil {
			return err
		}
	}
	for _, sm := range set.maps {
		for _, pt := range sm.points {
			if err := p.applyPoint(ctx, q, sm, pt); err != nil {
				return err
			}
		}
	}
	return nil
}

// insertStagedMap writes a staged map's row and, when it has a grid and
// something painted, its layers.
func insertStagedMap(ctx context.Context, q *mapsdb.Queries, campaignID string, sm *stagedMap) error {
	var revealed *time.Time
	if sm.revealed {
		revealed = &sm.createdAt
	}
	if err := q.InsertImportedMap(ctx, mapsdb.InsertImportedMapParams{
		ID: sm.id, CampaignID: campaignID, Name: sm.name, ImageID: sm.imageID, RevealedAt: revealed, GridColumns: sm.columns,
		GridFactor: sm.factor, FogEnabled: sm.fog, FogOnFirstGrid: sm.onGrid, GroupVision: sm.group, BaseLight: sm.baseLight, Now: sm.createdAt,
	}); err != nil {
		return fmt.Errorf("insert an imported map: %w", err)
	}
	if sm.columns == nil || !slicesAny(sm.layers[:]) {
		return nil
	}
	if err := q.UpsertMapLayers(ctx, mapsdb.UpsertMapLayersParams{
		MapID: sm.id, DifficultTerrain: sm.layers[0], Walls: sm.layers[1], Cover: sm.layers[2], Light: sm.layers[3], Doors: sm.layers[4], NoPass: sm.layers[5], Now: sm.createdAt,
	}); err != nil {
		return fmt.Errorf("insert an imported map's layers: %w", err)
	}
	return nil
}

func slicesAny(layers [][]byte) bool {
	for _, b := range layers {
		if len(b) > 0 {
			return true
		}
	}
	return false
}

func (p *mapsPart) applyPoint(ctx context.Context, q *mapsdb.Queries, sm *stagedMap, pt *stagedPoint) error {
	var revealed *time.Time
	if pt.revealed {
		revealed = &pt.createdAt
	}
	if err := q.InsertImportedMapPoint(ctx, mapsdb.InsertImportedMapPointParams{
		ID: pt.id, MapID: sm.id, Kind: pt.kind, Name: pt.name, Description: pt.desc, Hooks: pt.hooks, ShowDc: pt.showDC,
		XBp: pt.x, YBp: pt.y, TargetMapID: pt.target, Trap: pt.spec.trap, TrapState: pt.spec.trapSt, TreasureValuePo: pt.spec.value,
		LightPreset: pt.spec.light.preset, LightBrightFt: pt.spec.lightBright(), LightDimFt: pt.spec.lightDim(),
		RevealedAt: revealed, Stairs: pt.stairs, Now: pt.createdAt,
	}); err != nil {
		return fmt.Errorf("insert an imported point: %w", err)
	}
	for i, a := range pt.actions {
		if _, err := q.InsertSceneAction(ctx, mapsdb.InsertSceneActionParams{
			PointID: pt.id, Position: int32(i), Key: a.key, Name: a.name, Dc: a.dc, MaxAttempts: a.maxAttempts, Now: pt.createdAt,
		}); err != nil {
			return fmt.Errorf("insert an imported scene action: %w", err)
		}
	}
	for i, c := range pt.clues {
		if err := q.InsertImportedSceneClue(ctx, mapsdb.InsertImportedSceneClueParams{
			ID: c.id, PointID: pt.id, Position: int32(i), Text: c.text, Now: pt.createdAt,
		}); err != nil {
			return fmt.Errorf("insert an imported clue: %w", err)
		}
	}
	return nil
}
