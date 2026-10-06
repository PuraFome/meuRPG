package maps

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The painted layers of a map and its fog of war settings (MR-034, MR-036,
// Etapa 9, D2 and D6).
//
// Five layers sit on a map's grid: difficult terrain and walls (one bit a
// square), cover (two bits: none, half, three-quarters), light (two bits: not
// painted, dark, dim, bright) and doors (four bits: none, open, closed, locked,
// barred, secret; RN-26). Package rules/grid owns the byte layout; this file
// stores the bytes (table map_layers, one row a map, and a layer with nothing
// painted is NULL) and decides who reads what:
//
//   - the master reads all five, as painted;
//   - a player reads the walls, the difficult terrain, the cover and the doors of
//     a map they see, because those are things you can see, but never the light,
//     and the doors as they know them: a locked door reads closed (they learn it
//     is locked by trying) and a secret door is no door but a wall (the walls
//     layer they get has one there, the doors layer has nothing);
//   - while the map has the fog of war on, a player reads only the squares
//     their character sees now or remembers, and the walls next to a seen square
//     (fog.go says which; GetMapLayers answers fog_withheld, "this is a filtered
//     view"), and the master reads them as a player's character does when they
//     ask "Ver como".
//
// What clears them: a new grid (other columns) and a new image, because a layer
// is sized by the grid. clearLayers is the one place that does it, and where the
// players' memory of the map (map_vision_memory) is forgotten too, since a bitmap
// only fits the grid it was made on.

// maxPaintSquares is how many squares one PaintMapCells may carry. Painting is
// dragged, so the app sends small batches; this bounds what one call costs.
const maxPaintSquares = 400

// defaultLayerHintEvery is how often a map's layers may send their `map_changed`
// (Service.layerHintEvery): a drag paints dozens of batches a second, and each
// hint makes every watcher read the layers again.
const defaultLayerHintEvery = time.Second

// errMapBlocked is MapService's `failed_precondition`, with the MapBlocked
// detail that tells the app why.
func errMapBlocked(reason mapsv1.MapBlockedReason, msg string) error {
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	if detail, detailErr := connect.NewErrorDetail(&mapsv1.MapBlocked{Reason: reason}); detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

func errNoGrid() error {
	return errMapBlocked(mapsv1.MapBlockedReason_MAP_BLOCKED_REASON_NO_GRID, "the map has no grid")
}

// gridOf is the grid of a map row with its image's size: the zero Grid when the
// map has none.
func gridOf(columns *int32, width, height int32) grid.Grid {
	if columns == nil {
		return grid.Grid{}
	}
	return grid.Grid{Columns: int(*columns), Rows: grid.RowsFor(int(*columns), int(width), int(height))}
}

// layerSet is a map's five layers, decoded.
type layerSet struct {
	terrain *grid.Layer
	walls   *grid.Layer
	cover   *grid.CoverLayer
	light   *grid.LightLayer
	doors   *grid.DoorLayer
}

// loadLayers decodes the stored layers for the grid. A layer that is NULL or does
// not fit the grid (bytes of another grid, which clearLayers prevents) reads as
// nothing painted.
func loadLayers(row mapsdb.MapLayer, g grid.Grid) layerSet {
	set := layerSet{terrain: grid.NewLayer(g), walls: grid.NewLayer(g), cover: grid.NewCoverLayer(g), light: grid.NewLightLayer(g), doors: grid.NewDoorLayer(g)}
	if l, err := grid.DecodeLayer(g, row.DifficultTerrain); err == nil {
		set.terrain = l
	}
	if l, err := grid.DecodeLayer(g, row.Walls); err == nil {
		set.walls = l
	}
	if l, err := grid.DecodeCoverLayer(g, row.Cover); err == nil {
		set.cover = l
	}
	if l, err := grid.DecodeLightLayer(g, row.Light); err == nil {
		set.light = l
	}
	if l, err := grid.DecodeDoorLayer(g, row.Doors); err == nil {
		set.doors = l
	}
	return set
}

// forPlayers is the layers as a player who has not found out more reads them:
// the doors as they know them, and the secret ones as walls (RN-26). The light
// is theirs never, so it stays out too.
func (set layerSet) forPlayers() layerSet {
	walls, doors := set.doors.ForPlayers(set.walls)
	return layerSet{terrain: set.terrain, walls: walls, cover: set.cover, doors: doors}
}

// nilIfBlank stores a layer with nothing painted as NULL, so "empty" has one
// shape.
func nilIfBlank(b []byte) []byte {
	for _, x := range b {
		if x != 0 {
			return b
		}
	}
	return nil
}

// PaintMapCells implements mapsv1connect.MapServiceHandler.
func (s *Service) PaintMapCells(
	ctx context.Context,
	req *connect.Request[mapsv1.PaintMapCellsRequest],
) (*connect.Response[mapsv1.PaintMapCellsResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	layer, value := req.Msg.GetLayer(), req.Msg.GetValue()
	if err := checkPaint(layer, value); err != nil {
		return nil, err
	}
	squares := req.Msg.GetSquares()
	if len(squares) == 0 || len(squares) > maxPaintSquares {
		return nil, badSpec("squares must have 1 to %d squares", maxPaintSquares)
	}

	var revision, changed int32
	var mapRow mapsdb.Map
	playersToo := layer != mapsv1.MapLayer_MAP_LAYER_LIGHT // whether what changed is something the players read
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		changed = 0
		// The map's row lock makes two batches of paint take turns: each reads
		// the layers the other left.
		var err error
		if mapRow, err = q.GetMapForUpdate(ctx, mapsdb.GetMapForUpdateParams{CampaignID: m.CampaignID, ID: mapID}); errors.Is(err, pgx.ErrNoRows) {
			return errMapNotFound()
		} else if err != nil {
			return fmt.Errorf("find map: %w", err)
		}
		if mapRow.GridColumns == nil {
			return errNoGrid()
		}
		size, err := q.GetMapGrid(ctx, mapsdb.GetMapGridParams{CampaignID: m.CampaignID, ID: mapID})
		if err != nil {
			return fmt.Errorf("read the map's grid: %w", err)
		}
		g := gridOf(size.GridColumns, size.ImageWidth, size.ImageHeight)
		for i, sq := range squares {
			if !g.Contains(grid.Square{Col: int(sq.GetCol()), Row: int(sq.GetRow())}) {
				return badSpec("squares[%d] is outside the map's grid of %d x %d squares", i, g.Columns, g.Rows)
			}
		}
		stored, err := q.GetMapLayers(ctx, mapID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("read the layers: %w", err)
		}
		set := loadLayers(stored, g)
		asPlayersRead := playersRead(set)
		for _, sq := range squares {
			if paint(set, layer, int(sq.GetCol()), int(sq.GetRow()), value) {
				changed++
			}
		}
		// Locking or unlocking a door, or painting a secret door where a wall is, changes
		// nothing a player reads (a locked door reads closed, a secret one is a wall):
		// like the light, it stays out of the number they see and out of their hints (RN-10).
		playersToo = layer != mapsv1.MapLayer_MAP_LAYER_LIGHT &&
			(layer != mapsv1.MapLayer_MAP_LAYER_DOORS || !bytes.Equal(asPlayersRead, playersRead(set)))
		revision = mapRow.LayersRevision + mapRow.LightRevision
		if changed == 0 {
			return nil // painting what is already there changes nothing
		}
		err = q.UpsertMapLayers(ctx, mapsdb.UpsertMapLayersParams{
			MapID: mapID, DifficultTerrain: nilIfBlank(set.terrain.Encode()), Walls: nilIfBlank(set.walls.Encode()),
			Cover: nilIfBlank(set.cover.Encode()), Light: nilIfBlank(set.light.Encode()), Doors: nilIfBlank(set.doors.Encode()), Now: s.now(),
		})
		if err != nil {
			return fmt.Errorf("write the layers: %w", err)
		}
		// The light is the master's alone: it has its own counter, so the number
		// players see does not tell them when the master paints it. A change the
		// players cannot see (above) counts there too.
		if !playersToo {
			light, err := q.BumpMapLightRevision(ctx, mapID)
			if err != nil {
				return fmt.Errorf("bump the light's revision: %w", err)
			}
			revision = mapRow.LayersRevision + light
			return nil
		}
		layers, err := q.BumpMapLayersRevision(ctx, mapID)
		if err != nil {
			return fmt.Errorf("bump the layers' revision: %w", err)
		}
		revision = layers + mapRow.LightRevision
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "paint a map's squares", err)
	}
	if changed > 0 {
		s.layersChanged(ctx, m.CampaignID, mapRow, playersToo, layer == mapsv1.MapLayer_MAP_LAYER_LIGHT)
	}
	return connect.NewResponse(&mapsv1.PaintMapCellsResponse{LayersRevision: revision, Changed: changed}), nil
}

// playersRead is what the layers a player reads would say if the player read them
// whole: the walls and the doors as they know them (locked reads closed, a secret
// door is a wall). Two states with the same bytes look the same to every player.
func playersRead(set layerSet) []byte {
	p := set.forPlayers()
	return slices.Concat(p.walls.Encode(), p.doors.Encode())
}

// checkPaint checks that a value is one the layer holds.
func checkPaint(layer mapsv1.MapLayer, value int32) error {
	var highest int32
	switch layer {
	case mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN, mapsv1.MapLayer_MAP_LAYER_WALL:
		highest = 1
	case mapsv1.MapLayer_MAP_LAYER_COVER:
		highest = int32(grid.CoverThreeQuarters)
	case mapsv1.MapLayer_MAP_LAYER_LIGHT:
		highest = int32(grid.Bright)
	case mapsv1.MapLayer_MAP_LAYER_DOORS:
		highest = int32(grid.DoorSecret)
	default:
		return badSpec("layer is required")
	}
	if value < 0 || value > highest {
		return badSpec("value must be 0 to %d for this layer", highest)
	}
	return nil
}

// paint sets one square of a layer, and says whether its value changed.
func paint(set layerSet, layer mapsv1.MapLayer, col, row int, value int32) bool {
	switch layer {
	case mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN:
		return paintBit(set.terrain, col, row, value == 1)
	case mapsv1.MapLayer_MAP_LAYER_WALL:
		return paintBit(set.walls, col, row, value == 1)
	case mapsv1.MapLayer_MAP_LAYER_COVER:
		cover := grid.Cover(value) //nolint:gosec // G115: checkPaint kept it 0 to 2
		was := set.cover.Get(col, row)
		set.cover.Set(col, row, cover)
		return was != cover
	case mapsv1.MapLayer_MAP_LAYER_DOORS:
		door := grid.Door(value) //nolint:gosec // G115: checkPaint kept it 0 to 5
		was := set.doors.Get(col, row)
		set.doors.Set(col, row, door)
		return was != door
	default: // light
		light := grid.Light(value) //nolint:gosec // G115: checkPaint kept it 0 to 3
		was := set.light.Get(col, row)
		set.light.Set(col, row, light)
		return was != light
	}
}

func paintBit(l *grid.Layer, col, row int, on bool) bool {
	was := l.Get(col, row)
	l.Set(col, row, on)
	return was != on
}

// layersChanged tells the watchers that a map's layers changed. playersToo says
// the players read what changed, and seeingChanged that what the fog shows changes
// even if they do not read it (the light). The master
// always hears; the players only when they may read the layers (a map they see,
// no fog) and the layer is one they read (never the light). On a fog map the
// walls and the light decide what the players see: what each one sees, and
// remembers, is worked out again, and `vision_changed` goes to the players it
// changed (refreshVision); terrain and cover are theirs only on squares they
// know, and the hint of the walls and the light covers the squares they see.
func (s *Service) layersChanged(ctx context.Context, campaignID string, mapRow mapsdb.Map, playersToo, seeingChanged bool) {
	if fogged(mapRow) && !playersToo && !seeingChanged {
		s.layerHints.fire(mapRow.ID, s.layerHintEvery, false, func(bool) { s.publishMapChanged(campaignID, mapRow.ID, false) })
		return // only the master's own view of the layers changed
	}
	if fogged(mapRow) {
		s.layerHints.fire(mapRow.ID, s.layerHintEvery, false, func(bool) {
			s.publishMapChanged(campaignID, mapRow.ID, false)
			s.refreshVision(ctx, campaignID, mapRow.ID)
		})
		return
	}
	players := false
	if playersToo && !mapRow.FogEnabled {
		current, err := s.currentMap(ctx, campaignID)
		if err != nil {
			s.logger.ErrorContext(ctx, "maps: read the current map", "error", err)
		}
		players = playersSee(mapRow.ID, mapRow.RevealedAt, current)
	}
	s.layerHints.fire(mapRow.ID, s.layerHintEvery, players, func(players bool) {
		s.publishMapChanged(campaignID, mapRow.ID, players)
	})
}

// hintGate lets a key send at most one hint every interval: the first goes at
// once, and the ones that follow within the interval merge into a single hint
// sent when it ends, so the last change is never left unannounced.
type hintGate struct {
	mu   sync.Mutex
	keys map[string]*gateState
}

type gateState struct {
	last    time.Time
	timer   *time.Timer
	players bool // a hint is waiting, and whether it is for the players too
}

func (g *hintGate) fire(key string, every time.Duration, players bool, send func(players bool)) {
	g.mu.Lock()
	if g.keys == nil {
		g.keys = map[string]*gateState{}
	}
	st := g.keys[key]
	if st == nil {
		st = &gateState{}
		g.keys[key] = st
		g.forget(key, every) // the table stays small: idle keys go
	}
	if st.timer != nil { // a hint is already waiting: merge into it
		st.players = st.players || players
		g.mu.Unlock()
		return
	}
	if wait := every - time.Since(st.last); wait > 0 {
		st.players = players
		st.timer = time.AfterFunc(wait, func() {
			g.mu.Lock()
			p := st.players
			st.timer, st.players, st.last = nil, false, time.Now()
			g.mu.Unlock()
			send(p)
		})
		g.mu.Unlock()
		return
	}
	st.last = time.Now()
	g.mu.Unlock()
	send(players)
}

// forget drops the keys that have been idle for a minute, so the table does not
// grow with every map ever painted. The caller holds the lock.
func (g *hintGate) forget(except string, every time.Duration) {
	if len(g.keys) < 256 {
		return
	}
	for k, st := range g.keys {
		if k != except && st.timer == nil && time.Since(st.last) > max(time.Minute, every) {
			delete(g.keys, k)
		}
	}
}

// clearLayers deletes a map's painted layers inside the transaction and bumps
// their revision when there were any: its grid's columns or its image changed,
// and a layer only makes sense for the grid it was painted on (D2). The players'
// memory of the map goes too: it is a bitmap of that grid.
func clearLayers(ctx context.Context, q *mapsdb.Queries, mapID string) error {
	if err := clearVisionMemory(ctx, q, mapID); err != nil {
		return err
	}
	n, err := q.DeleteMapLayers(ctx, mapID)
	if err != nil {
		return fmt.Errorf("clear the layers: %w", err)
	}
	if n > 0 {
		if _, err := q.BumpMapLayersRevision(ctx, mapID); err != nil {
			return fmt.Errorf("bump the layers' revision: %w", err)
		}
	}
	return nil
}

// clearVisionMemory makes every player forget what they saw of the map: the map's
// vision epoch goes up (what was remembered under an older one reads as empty and
// is never written back) and the old rows go.
func clearVisionMemory(ctx context.Context, q *mapsdb.Queries, mapID string) error {
	if _, err := q.ClearMapVisionMemory(ctx, mapID); err != nil {
		return fmt.Errorf("forget what the players saw: %w", err)
	}
	if _, err := q.DeleteMapVisionMemory(ctx, mapID); err != nil {
		return fmt.Errorf("forget what the players saw: %w", err)
	}
	return nil
}

// refuseWhileCombat refuses with COMBAT_RUNNING when a combat runs on the map:
// changing its grid or image would clear the layers the fight stands on.
func (s *Service) refuseWhileCombat(ctx context.Context, tx pgx.Tx, campaignID, mapID string) error {
	running, err := s.combats.CombatRunsOnMap(ctx, tx, campaignID, mapID)
	if err != nil {
		return fmt.Errorf("read whether a combat runs on the map: %w", err)
	}
	if running {
		return errMapBlocked(mapsv1.MapBlockedReason_MAP_BLOCKED_REASON_COMBAT_RUNNING,
			"a combat is running on this map: it cannot be changed or deleted")
	}
	return nil
}

// GetMapLayers implements mapsv1connect.MapServiceHandler.
func (s *Service) GetMapLayers(
	ctx context.Context,
	req *connect.Request[mapsv1.GetMapLayersRequest],
) (*connect.Response[mapsv1.GetMapLayersResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	v, err := s.readerOf(ctx, m, req.Msg.GetAsCharacterId())
	if err != nil {
		return nil, err
	}
	cm, err := s.loadMaps(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read a map", err)
	}
	row, ok := cm.byID[mapID]
	if !ok || !v.seesMap(row.ID, row.RevealedAt) {
		return nil, errMapNotFound() // a hidden map is not found to a player (RN-10)
	}
	g := gridOf(row.GridColumns, row.ImageWidth, row.ImageHeight)
	res := &mapsv1.GetMapLayersResponse{GridColumns: int32(g.Columns), GridRows: int32(g.Rows), LayersRevision: row.LayersRevision} //nolint:gosec // G115: a grid is at most 200 x 400
	if v.master {
		res.LayersRevision += row.LightRevision
	}
	if !g.Valid() {
		return connect.NewResponse(res), nil
	}
	stored, err := s.queries.GetMapLayers(ctx, mapID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, s.dbError(ctx, "read a map's layers", err)
	}
	set := loadLayers(stored, g)
	if foggedFor(v, row) {
		// A player gets only the squares their character sees or remembers (RN-10).
		// The map's counter is withheld: it would tell when the master paints what
		// they do not see; the revision is a number of what they receive.
		pv, _, _, err := s.fogViewOf(ctx, row, v)
		if err != nil {
			return nil, s.dbError(ctx, "work out what a player sees", err)
		}
		set = filterLayers(set, pv)
		res.DifficultTerrain, res.Wall, res.Cover, res.Doors = nilIfBlank(set.terrain.Encode()), nilIfBlank(set.walls.Encode()), nilIfBlank(set.cover.Encode()), nilIfBlank(set.doors.Encode())
		res.LayersRevision = hashOf(res.DifficultTerrain, res.Wall, res.Cover, res.Doors)
		res.FogWithheld = true
		return connect.NewResponse(res), nil
	}
	if v.master {
		res.Light = nilIfBlank(set.light.Encode())
	} else {
		set = set.forPlayers() // a locked door reads closed and a secret one is a wall (RN-26)
	}
	res.DifficultTerrain, res.Wall, res.Cover, res.Doors = nilIfBlank(set.terrain.Encode()), nilIfBlank(set.walls.Encode()), nilIfBlank(set.cover.Encode()), nilIfBlank(set.doors.Encode())
	return connect.NewResponse(res), nil
}

// SetMapFog implements mapsv1connect.MapServiceHandler.
func (s *Service) SetMapFog(
	ctx context.Context,
	req *connect.Request[mapsv1.SetMapFogRequest],
) (*connect.Response[mapsv1.SetMapFogResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	msg := req.Msg
	if msg.FogEnabled == nil && msg.BaseLight == nil && msg.GroupVision == nil {
		return nil, badSpec("nothing to change")
	}
	var baseLight *string
	if msg.BaseLight != nil {
		word, ok := baseLightToDB[msg.GetBaseLight()]
		if !ok {
			return nil, badSpec("base_light must be DARK, DIM or BRIGHT")
		}
		baseLight = &word
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}

	// Turning the fog on keeps the raw image from the players (RN-10): an image
	// that is also used another way is copied first, and the map gets the copy.
	var imageCopy *fogCopy
	if msg.GetFogEnabled() {
		if row, err := s.queries.GetMap(ctx, mapsdb.GetMapParams{CampaignID: m.CampaignID, ID: mapID}); err == nil && !row.FogEnabled {
			if imageCopy, err = s.prepareFogCopy(ctx, m.CampaignID, mapID, row.ImageID); err != nil {
				return nil, s.dbError(ctx, "copy the map's image for the fog", err)
			}
		}
	}
	var before, after mapsdb.Map
	copied := false
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		copied = false
		if before, err = q.GetMapForUpdate(ctx, mapsdb.GetMapForUpdateParams{CampaignID: m.CampaignID, ID: mapID}); errors.Is(err, pgx.ErrNoRows) {
			return errMapNotFound()
		} else if err != nil {
			return fmt.Errorf("find map: %w", err)
		}
		if msg.GetFogEnabled() && before.GridColumns == nil {
			return errNoGrid() // the fog is made of squares
		}
		// The copy is used only if the map still has the image it was made from and
		// the fog is still off.
		if imageCopy != nil && !before.FogEnabled && before.ImageID == imageCopy.source.ID {
			if err := imageCopy.insert(ctx, q, s, m.CampaignID, m.UserID); err != nil {
				return err
			}
			if _, err := q.SetMapImageOnly(ctx, mapsdb.SetMapImageOnlyParams{CampaignID: m.CampaignID, ID: mapID, ImageID: imageCopy.id, Now: s.now()}); err != nil {
				return fmt.Errorf("give the map its copy of the image: %w", err)
			}
			copied = true
		}
		after, err = q.SetMapFog(ctx, mapsdb.SetMapFogParams{
			CampaignID: m.CampaignID, ID: mapID, FogEnabled: msg.FogEnabled, BaseLight: baseLight, GroupVision: msg.GroupVision, Now: s.now(),
		})
		if err != nil {
			return fmt.Errorf("set the fog's settings: %w", err)
		}
		return nil
	})
	if imageCopy != nil && (err != nil || !copied) {
		s.deleteFiles(ctx, m.CampaignID, imageCopy.id) // made for nothing
	}
	if err != nil {
		return nil, s.dbError(ctx, "set a map's fog", err)
	}
	// The players hear of what they read (the switch, "Visão do grupo"), not of the
	// base light, which is the master's.
	playersRead := before.FogEnabled != after.FogEnabled || before.GroupVision != after.GroupVision
	s.publishMapChanged(m.CampaignID, mapID, playersRead && playersSee(mapID, after.RevealedAt, current))
	// What everyone sees, and remembers, is worked out again (the base light is the
	// master's, but it moves what the squares look like).
	if after.FogEnabled {
		s.refreshVision(ctx, m.CampaignID, mapID)
	}
	out, err := s.masterMap(ctx, m, mapID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.SetMapFogResponse{Map: out}), nil
}

// The database's base lights (maps_base_light_valid) and the API's.
var (
	baseLightToDB = map[mapsv1.LightLevel]string{
		mapsv1.LightLevel_LIGHT_LEVEL_DARK:   "dark",
		mapsv1.LightLevel_LIGHT_LEVEL_DIM:    "dim",
		mapsv1.LightLevel_LIGHT_LEVEL_BRIGHT: "bright",
	}
	baseLightFromDB = map[string]mapsv1.LightLevel{
		"dark":   mapsv1.LightLevel_LIGHT_LEVEL_DARK,
		"dim":    mapsv1.LightLevel_LIGHT_LEVEL_DIM,
		"bright": mapsv1.LightLevel_LIGHT_LEVEL_BRIGHT,
	}
)
