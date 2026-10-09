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
	entries := in.Pkg.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGES)
	files := in.Pkg.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE)
	if len(entries) == 0 {
		if len(files) > 0 {
			in.Problem(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_IMAGE, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_MISSING_ENTRY, 0)
		}
		return nil
	}
	if len(entries) != 1 || entries[0].GetPath() != campaignpackage.ImagesEntry {
		in.Problem(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_IMAGE, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNEXPECTED_ENTRY, 0)
		return nil
	}
	var list pkgv1.PackageImages
	if !in.Read(campaignpackage.ImagesEntry, &list) {
		return nil
	}
	s := p.s
	image := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_IMAGE
	if int32(len(list.GetImages())) > s.maxImages { //nolint:gosec // G115: a list of at most 2,000 entries
		in.Problem(image, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_LIMIT, int64(s.maxImages))
		return nil
	}
	base := s.now()
	used := map[string]bool{}
	var bytesTotal int64
	for i, img := range list.GetImages() {
		name, err := names.Clean(img.GetName(), maxNameLength)
		if err != nil {
			in.Problem(image, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
			continue
		}
		id, ok := in.IDs.Define(nsImage, img.GetId())
		if !ok || !generatedKinds[img.GetGeneratedKind()] || (img.GetGeneratedKind() != "" && !img.GetGenerated()) {
			in.Problem(image, name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
			continue
		}
		entry, ok := in.Pkg.Entry(img.GetFile())
		switch {
		case !ok || entry.GetKind() != pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE:
			in.Problem(image, name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_MISSING_ENTRY, 0)
			continue
		case used[img.GetFile()]:
			in.Problem(image, name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_DUPLICATE, 0)
			continue
		}
		used[img.GetFile()] = true
		si := &stagedImage{
			oldID: img.GetId(), id: id, name: name, generated: img.GetGenerated(), generatedKind: img.GetGeneratedKind(),
			createdAt: base.Add(-time.Duration(i) * orderStep), // the list is newest first
		}
		si.parentOld = in.IDs.Ref(nsImage, img.GetParentImageId(), image, name)
		si.copyOld = in.IDs.Ref(nsImage, img.GetCopyOfImageId(), image, name)
		si.parentOld, si.copyOld = img.GetParentImageId(), img.GetCopyOfImageId()
		if !p.stageFile(ctx, in, si, img.GetFile()) {
			continue
		}
		bytesTotal += int64(si.size)
		staged.list = append(staged.list, si)
		staged.byOld[si.oldID] = si
		in.Counts.Images++
		in.Counts.ImageBytes += entry.GetSize()
	}
	for _, f := range files {
		if !used[f.GetPath()] {
			in.Problem(image, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNEXPECTED_ENTRY, 0)
		}
	}
	if bytesTotal > int64(s.maxBytes) {
		in.Problem(image, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_LIMIT, int64(s.maxBytes))
	}
	return nil
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
	for _, si := range staged.list {
		if err := q.InsertImportedGalleryImage(ctx, mapsdb.InsertImportedGalleryImageParams{
			ID: si.id, CampaignID: in.CampaignID, UploadedBy: &in.UserID, Name: si.name, ContentType: si.contentType,
			Width: int32(si.width), Height: int32(si.height), ByteSize: int32(si.size), //nolint:gosec // G115: at most images.MaxSide and MaxBytes
			CreatedAt: si.createdAt, Generated: si.generated, GeneratedKind: si.generatedKind,
		}); err != nil {
			return fmt.Errorf("insert an imported image: %w", err)
		}
	}
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
	layers      [5][]byte
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

func (p *mapsPart) exportMap(ctx context.Context, q *mapsdb.Queries, m mapsdb.Map) (*pkgv1.PackageMap, error) {
	out := &pkgv1.PackageMap{
		Id: m.ID, Name: m.Name, ImageId: m.ImageID, GridFactor: m.GridFactor, Revealed: m.RevealedAt != nil,
		FogEnabled: m.FogEnabled, FogOnFirstGrid: m.FogOnFirstGrid, GroupVision: m.GroupVision, BaseLight: m.BaseLight,
	}
	if m.GridColumns != nil {
		out.GridColumns = *m.GridColumns
	}
	layers, err := q.GetMapLayers(ctx, m.ID)
	switch {
	case err == nil:
		out.Layers = &pkgv1.PackageLayers{
			DifficultTerrain: layers.DifficultTerrain, Walls: layers.Walls, Cover: layers.Cover, Light: layers.Light, Doors: layers.Doors,
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, fmt.Errorf("read a map's layers: %w", err)
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
		for _, a := range actions {
			if a.PointID == pt.ID {
				act := &pkgv1.PackageAction{Key: a.Key, Name: a.Name, MaxAttempts: a.MaxAttempts}
				if a.Dc != nil {
					act.Dc = *a.Dc
				}
				pp.Actions = append(pp.Actions, act)
			}
		}
		for _, c := range clues {
			if c.PointID == pt.ID {
				pp.Clues = append(pp.Clues, &pkgv1.PackageClue{Id: c.ID, Text: c.Text})
			}
		}
		out.Points = append(out.Points, pp)
	}
	return out, nil
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
		sm := p.stageMap(in, &pm, imgs, base.Add(time.Duration(i)*orderStep))
		if sm != nil {
			set.maps = append(set.maps, sm)
			in.Counts.Maps++
			for _, pt := range sm.points {
				in.Facts.PointMap[pt.oldID] = sm.oldID
				in.Facts.PointKind[pt.oldID] = pt.kind
				in.Facts.PointName[pt.oldID] = pt.name
			}
		}
	}
	in.Facts.DoorAt = func(mapID string, col, row int) bool {
		for _, sm := range set.maps {
			if sm.oldID != mapID || sm.doors == nil {
				continue
			}
			sq := grid.Square{Col: col, Row: row}
			return sm.grid.Contains(sq) && sm.doors.At(sq) != grid.DoorNone
		}
		return false
	}
	return nil
}

// stageMap checks one map; nil when it has a problem (recorded).
func (p *mapsPart) stageMap(in *campaignpackage.Import, pm *pkgv1.PackageMap, imgs *stagedImages, createdAt time.Time) *stagedMap {
	s := p.s
	mapKind := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_MAP
	name, err := names.Clean(pm.GetName(), maxNameLength)
	if err != nil {
		in.Problem(mapKind, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
		return nil
	}
	invalid := func() *stagedMap {
		in.Problem(mapKind, name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
		return nil
	}
	id, ok := in.IDs.Define(nsMap, pm.GetId())
	if !ok {
		return invalid()
	}
	sm := &stagedMap{
		oldID: pm.GetId(), id: id, name: name, factor: pm.GetGridFactor(), revealed: pm.GetRevealed(), fog: pm.GetFogEnabled(), onGrid: pm.GetFogOnFirstGrid(),
		group: pm.GetGroupVision(), baseLight: pm.GetBaseLight(), createdAt: createdAt,
	}
	sm.imageID = in.IDs.Ref(nsImage, pm.GetImageId(), mapKind, name)
	if sm.baseLight != "dark" && sm.baseLight != "dim" && sm.baseLight != "bright" {
		return invalid()
	}
	var g grid.Grid
	if cols := pm.GetGridColumns(); cols != 0 {
		drawn := cols / max(sm.factor, 1)
		if sm.factor < 1 || sm.factor > grid.MaxFactor || cols%sm.factor != 0 || drawn < minGridColumns || drawn > maxGridColumns {
			return invalid()
		}
		sm.columns = &cols
		if img := imgs.find(pm.GetImageId()); img != nil {
			engine, err := grid.EngineGrid(int(drawn), int(sm.factor), img.width, img.height)
			if err != nil || engine.Columns != int(cols) {
				return invalid()
			}
			g = engine
		}
	} else if sm.factor != 1 || sm.fog {
		return invalid() // no grid: no scale and no fog (SetMapFog needs a grid)
	}
	sm.grid = g
	if !p.stageLayers(sm, pm.GetLayers(), g) {
		return invalid()
	}
	if int32(len(pm.GetPoints())) > s.maxPoints { //nolint:gosec // G115: bounded by the entry's size
		in.Problem(mapKind, name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_LIMIT, int64(s.maxPoints))
		return nil
	}
	good := true
	for i, pp := range pm.GetPoints() {
		pt := p.stagePoint(in, pm, sm, pp, createdAt.Add(time.Duration(i)*time.Microsecond))
		if pt == nil {
			good = false
			continue
		}
		sm.points = append(sm.points, pt)
	}
	if !good {
		return nil
	}
	return sm
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
	raw := [5][]byte{l.GetDifficultTerrain(), l.GetWalls(), l.GetCover(), l.GetLight(), l.GetDoors()}
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
	checks := [5]func() error{
		func() error { _, err := grid.DecodeLayer(g, raw[0]); return err },
		func() error { _, err := grid.DecodeLayer(g, raw[1]); return err },
		func() error { _, err := grid.DecodeCoverLayer(g, raw[2]); return err },
		func() error { _, err := grid.DecodeLightLayer(g, raw[3]); return err },
		func() (err error) { sm.doors, err = grid.DecodeDoorLayer(g, raw[4]); return err },
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

// stagePoint checks one point and its actions and clues; nil when it has a
// problem (recorded).
func (p *mapsPart) stagePoint(in *campaignpackage.Import, pm *pkgv1.PackageMap, sm *stagedMap, pp *pkgv1.PackagePoint, createdAt time.Time) *stagedPoint {
	s := p.s
	kindOfPoint := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_POINT
	name, err := cleanName("name", pp.GetName())
	if err != nil {
		in.Problem(kindOfPoint, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
		return nil
	}
	kind, ok := kindToDB[pp.GetKind()]
	if ok && pp.GetKind() == mapsv1.MapPointKind_MAP_POINT_KIND_SCENE {
		kindOfPoint = pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_SCENE
	}
	bad := func(reason pkgv1.PackageProblemReason) *stagedPoint {
		in.Problem(kindOfPoint, name, reason, 0)
		return nil
	}
	if !ok {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	pt := &stagedPoint{oldID: pp.GetId(), kind: kind, name: name, x: pp.GetXBp(), y: pp.GetYBp(), showDC: pp.GetShowDc(), revealed: pp.GetRevealed(), createdAt: createdAt, mapID: sm.id}
	if pt.id, ok = in.IDs.Define(nsPoint, pp.GetId()); !ok {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	isScene := pp.GetKind() == mapsv1.MapPointKind_MAP_POINT_KIND_SCENE
	var errs [4]error
	pt.desc, errs[0] = cleanDescription(pp.GetDescription())
	pt.hooks, errs[1] = cleanHooks(pp.GetHooks())
	errs[2] = checkPosition(pp.GetXBp(), pp.GetYBp())
	if (pt.hooks != "" || pt.showDC) && !isScene {
		errs[3] = errOnlyScenesHaveHooks()
	}
	if err := errors.Join(errs[:]...); err != nil {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	if pp.GetTargetMapId() != "" {
		if !leads(kind) || pp.GetTargetMapId() == pm.GetId() {
			return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
		}
		target := in.IDs.Ref(nsMap, pp.GetTargetMapId(), kindOfPoint, name)
		if target != "" {
			pt.target = &target
		}
	}
	// A trap comes back armed, whatever state it was left in.
	trap := pp.GetTrap()
	if trap != nil {
		trap = proto.Clone(trap).(*mapsv1.TrapSpec) //nolint:forcetypeassert // Clone returns the same type
		trap.State = mapsv1.TrapState_TRAP_STATE_UNSPECIFIED
	}
	if pt.spec, err = s.specFor(kind, trap, pp.TreasureValuePo, pp.GetLight()); err != nil {
		return bad(specReason(err))
	}
	if pp.GetStairs() != "" {
		if kind != kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP] || (pp.GetStairs() != "up" && pp.GetStairs() != "down") {
			return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
		}
		stairs := pp.GetStairs()
		pt.stairs = &stairs
	}
	if !isScene && (len(pp.GetActions()) > 0 || len(pp.GetClues()) > 0) {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	if !p.stageScene(in, pt, pp, kindOfPoint) {
		return nil
	}
	switch pp.GetKind() {
	case mapsv1.MapPointKind_MAP_POINT_KIND_SCENE:
		in.Counts.Scenes++
	case mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE:
		in.Counts.BattlePoints++
	case mapsv1.MapPointKind_MAP_POINT_KIND_TREASURE:
		in.Counts.TreasurePoints++
	}
	return pt
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
		var revealed *time.Time
		if sm.revealed {
			revealed = &sm.createdAt
		}
		if err := q.InsertImportedMap(ctx, mapsdb.InsertImportedMapParams{
			ID: sm.id, CampaignID: in.CampaignID, Name: sm.name, ImageID: sm.imageID, RevealedAt: revealed, GridColumns: sm.columns,
			GridFactor: sm.factor, FogEnabled: sm.fog, FogOnFirstGrid: sm.onGrid, GroupVision: sm.group, BaseLight: sm.baseLight, Now: sm.createdAt,
		}); err != nil {
			return fmt.Errorf("insert an imported map: %w", err)
		}
		if sm.columns != nil && slicesAny(sm.layers[:]) {
			if err := q.UpsertMapLayers(ctx, mapsdb.UpsertMapLayersParams{
				MapID: sm.id, DifficultTerrain: sm.layers[0], Walls: sm.layers[1], Cover: sm.layers[2], Light: sm.layers[3], Doors: sm.layers[4], Now: sm.createdAt,
			}); err != nil {
				return fmt.Errorf("insert an imported map's layers: %w", err)
			}
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
			PointID: pt.id, Position: int32(i), Key: a.key, Name: a.name, Dc: a.dc, MaxAttempts: a.maxAttempts, Now: pt.createdAt, //nolint:gosec // G115: at most 20
		}); err != nil {
			return fmt.Errorf("insert an imported scene action: %w", err)
		}
	}
	for i, c := range pt.clues {
		if err := q.InsertImportedSceneClue(ctx, mapsdb.InsertImportedSceneClueParams{
			ID: c.id, PointID: pt.id, Position: int32(i), Text: c.text, Now: pt.createdAt, //nolint:gosec // G115: at most 30
		}); err != nil {
			return fmt.Errorf("insert an imported clue: %w", err)
		}
	}
	return nil
}
