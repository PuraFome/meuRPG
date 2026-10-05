package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	maplink "github.com/PuraFome/meuRPG/backend/internal/maps/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// Combat on a map with the fog of war on (MR-036, RN-10, RN-20, Etapa 9, slice
// 9.7). A player sees only the NPC combatants their character sees now (with
// "Visão do grupo", the party); an NPC they do not see is, for them, a hidden
// combatant: not in the order, no name, no state, no square, its turn "Vez do
// mestre", and not found when they name it. Players' characters and their
// creatures are never hidden (the party knows where its people are). The master
// sees everyone. A map without the fog behaves as before.
//
// How it is built:
//
//   - The maps module says what each player sees (FogSource.CombatSight); this
//     file turns that into the set of combatants a viewer does not see
//     (combatViewer.unseen), and every place that asked "does the viewer see it?"
//     (combatViewer.sees) now gets the fog's answer too: the order, the turn, the
//     targets, the pending damage, the reaction prompts, the offers, the move's
//     plan, and the not_found of naming an NPC.
//   - The sight is read before a change opens its transaction (write), never inside
//     it: the maps module asks this one where the combatants stand, and a read of a
//     row the transaction already changed would wait for it.
//   - The log keeps, with each event, who could see it when it happened
//     (actionEvent.SeenBy): a line stays visible to those who saw its NPCs then, and
//     one they did not see never appears later, whatever the view is now.
//   - The live events that carry a combatant (combatant_moved, turn_changed) go to
//     each player in the form they may see, or as the content-free
//     encounter_changed hint (publishCombatantMoved, publishTurnChanged).

// FogSource is what a combat asks of the maps module's fog of war. The maps module
// implements it (maps.Service) and cmd/api connects it with SetFog. A nil source
// is a world without the fog.
type FogSource interface {
	// CombatSight returns what the players see of the map now, or nil when the map
	// has no fog of war (or is gone).
	CombatSight(ctx context.Context, campaignID, mapID string) (maplink.CombatSight, error)
	// KnownTerrain returns the terrain a player knows of the map (the squares they
	// see now or remember) and true, or false when the map has no fog and the real
	// terrain is the player's too.
	KnownTerrain(ctx context.Context, campaignID, mapID, userID string) (grid.Terrain, bool, error)
	// CombatMoved tells the fog that a combatant moved on the map: what the move
	// showed is remembered and the players who see either square are told.
	CombatMoved(ctx context.Context, campaignID, mapID string, npc bool, from, to grid.Square)
}

// SetFog connects the maps module's fog of war. play and maps need each other, so
// cmd/api calls it once maps exists, before the server starts.
func (s *Service) SetFog(f FogSource) { s.fog = f }

// fogSight is the sight of one combat's map, read once for a request.
type fogSight struct {
	sight maplink.CombatSight
}

// fogSightOf reads what the players see of the encounter's map: nil when there is
// no fog on it (nothing to filter), and an error when it cannot be told, which
// fails the request rather than showing a player what they may not see.
func (s *Service) fogSightOf(ctx context.Context, campaignID string, enc playdb.Encounter) (*fogSight, error) {
	if s.fog == nil || enc.MapID == nil {
		return nil, nil
	}
	return s.fogSightOfMap(ctx, campaignID, *enc.MapID)
}

func (s *Service) fogSightOfMap(ctx context.Context, campaignID, mapID string) (*fogSight, error) {
	sight, err := s.fog.CombatSight(ctx, campaignID, mapID)
	if err != nil {
		return nil, fmt.Errorf("read what the players see: %w", err)
	}
	if sight == nil {
		return nil, nil
	}
	return &fogSight{sight: sight}, nil
}

// encounterMapOf is the map of the combat with the id, for a change that has not
// opened its transaction yet; "" when the combat has none or is not the campaign's.
func (s *Service) encounterMapOf(ctx context.Context, campaignID, encounterID string) (string, error) {
	var mapID *string
	err := s.pool.QueryRow(ctx, `
		SELECT e.map_id FROM encounters AS e
		JOIN game_sessions AS g ON g.id = e.game_session_id
		WHERE e.id = $1 AND g.campaign_id = $2`, encounterID, campaignID).Scan(&mapID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read the combat's map: %w", err)
	}
	return deref(mapID), nil
}

// sightForWrite reads the sight a change needs: nil for a change that creates a
// combat, one whose combat has no map, or a map with no fog.
func (s *Service) sightForWrite(ctx context.Context, w combatWrite) (*fogSight, error) {
	if s.fog == nil || w.encounterID == "" {
		return nil, nil
	}
	mapID, err := s.encounterMapOf(ctx, w.m.CampaignID, w.encounterID)
	if err != nil || mapID == "" {
		return nil, err
	}
	return s.fogSightOfMap(ctx, w.m.CampaignID, mapID)
}

// seesNPC says whether the player sees the NPC combatant. An NPC with no square
// (a combat in setup, an NPC the master has not placed) is seen by no player.
func (f *fogSight) seesNPC(userID string, c playdb.Combatant) bool {
	return placed(c) && f.sight.Sees(userID, squareOfCombatant(c))
}

// unseenFor lists the NPC combatants the player does not see. Players' characters
// and creatures are never in it (D6).
func (f *fogSight) unseenFor(userID string, cs []playdb.Combatant) map[string]bool {
	out := map[string]bool{}
	for _, c := range cs {
		if c.Kind == kindNPC && !f.seesNPC(userID, c) {
			out[c.ID] = true
		}
	}
	return out
}

// viewerFor is the viewer of a read outside a change: the caller, with the
// combatants of the fog map they do not see.
func (s *Service) viewerFor(ctx context.Context, m authz.Membership, enc playdb.Encounter, cs []playdb.Combatant) (combatViewer, error) {
	v, _, err := s.viewerWith(ctx, m, enc, cs)
	return v, err
}

// viewerWith is viewerFor that also returns the sight it read (nil for the master
// and for a map without the fog), for a read that needs more of it.
func (s *Service) viewerWith(ctx context.Context, m authz.Membership, enc playdb.Encounter, cs []playdb.Combatant) (combatViewer, *fogSight, error) {
	v := viewerOf(m)
	if v.master {
		return v, nil, nil
	}
	f, err := s.fogSightOf(ctx, m.CampaignID, enc)
	if err != nil {
		return v, nil, err
	}
	if f != nil {
		v.unseen = f.unseenFor(v.userID, cs)
	}
	return v, f, nil
}

// viewer is the viewer of a change, inside its transaction: the caller, with the
// combatants of the fog map they do not see, from the sight read before the
// transaction opened. cs are the combatants as the change reads them.
func (c *combatTx) viewer(m authz.Membership, cs []playdb.Combatant) combatViewer {
	v := viewerOf(m)
	if !v.master && c.sight != nil {
		v.unseen = c.sight.unseenFor(v.userID, cs)
	}
	return v
}

// ---- the move: what a player plans on ----

// knownTerrainOf reads the terrain a player knows of the combat's map (the squares
// they see now or remember), or nil for the master, for a map without the fog and
// for a combat without a map: they plan on the real terrain. It reads from the
// pool, so a change calls it before its transaction opens.
func (s *Service) knownTerrainOf(ctx context.Context, campaignID string, mapID *string, v combatViewer) (*grid.Terrain, error) {
	if v.master || s.fog == nil || mapID == nil || v.userID == "" {
		return nil, nil
	}
	known, fogged, err := s.fog.KnownTerrain(ctx, campaignID, *mapID, v.userID)
	if err != nil {
		return nil, fmt.Errorf("read the terrain the player knows: %w", err)
	}
	if !fogged {
		return nil, nil
	}
	return &known, nil
}

// planOn is the terrain a move is planned on: the player's known terrain on a fog
// map, plain floor in the dark (D1), else the real one. The real move runs on the
// real terrain and is cut short where it is blocked, and no refusal ever names a
// wall or a creature the player does not see.
func planOn(actual grid.Terrain, known *grid.Terrain) grid.Terrain {
	if known == nil || known.Grid != actual.Grid {
		return actual
	}
	return *known
}

// ---- who sees the mover: opportunity attacks ----

// reactorSees says whether an NPC or a creature that could make an opportunity
// attack sees the mover (D1b). Without the fog everyone does (the hidden mover is
// left out by the caller). With the fog: a player's character or creature sees
// what its player sees; an NPC sees from its own square with its own senses
// (darkvision, blindsight and truesight from the sheet or stat block; plain sight
// when it gives none), in the light of the map. at is the square the mover stood
// on, where it was when it left.
func (f *fogSight) reactorSees(r playdb.Combatant, sheet link.Sheet, at grid.Square) bool {
	if r.Kind != kindNPC {
		return r.UserID != nil && f.sight.Sees(*r.UserID, at)
	}
	return f.sight.CanSee(squareOfCombatant(r), vision.Senses{
		DarkvisionFt: sheet.Senses.DarkvisionFt, BlindsightFt: sheet.Senses.BlindsightFt, TruesightFt: sheet.Senses.TruesightFt,
	}, at)
}

// ---- the log: who could see it when it happened ----

// stamp writes into an event who could see its NPCs when it happened: Fogged, and
// SeenBy, the players (user IDs, never a name) who saw every NPC in it, at the
// square it stood on, or, for a move, on either square of it. A line with no NPC
// in it is left as it is. It is read from the sight taken before the change, which
// is how the table looked when it happened; it is never worked out again from a
// view that has changed.
func (c *combatTx) stamp(ctx context.Context, kind string, ev actionEvent) (actionEvent, error) {
	if c.sight == nil {
		return ev, nil
	}
	cs, err := c.q.ListCombatantsWithDismissed(ctx, c.enc.ID)
	if err != nil {
		return ev, fmt.Errorf("list the combatants: %w", err)
	}
	var npcSquares [][]grid.Square // for each NPC in the event, where it could have been seen
	for _, id := range ev.combatantIDs() {
		i := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.ID == id })
		if i < 0 || cs[i].Kind != kindNPC {
			continue
		}
		var squares []grid.Square
		if placed(cs[i]) {
			squares = append(squares, squareOfCombatant(cs[i]))
		}
		if kind == eventCombatantMoved && id == ev.Actor && ev.From != nil && ev.From.Placed {
			squares = append(squares, grid.Square{Col: int(ev.From.Col), Row: int(ev.From.Row)})
		}
		npcSquares = append(npcSquares, squares)
	}
	if len(npcSquares) == 0 {
		return ev, nil
	}
	ev.Fogged = true
	ev.SeenBy = nil
	for _, u := range c.sight.sight.Users() {
		all := true
		for _, squares := range npcSquares {
			if !slices.ContainsFunc(squares, func(sq grid.Square) bool { return c.sight.sight.Sees(u, sq) }) {
				all = false
				break
			}
		}
		if all {
			ev.SeenBy = append(ev.SeenBy, u)
		}
	}
	return ev, nil
}

// combatantIDs lists the combatants an event is about: who did it, who it was done
// to and every target of a spell or of a damage that settled several.
func (ev actionEvent) combatantIDs() []string {
	var ids []string
	add := func(id string) {
		if id != "" && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	add(ev.Actor)
	add(ev.Target)
	for _, h := range ev.Hits {
		add(h.Target)
	}
	for _, h := range ev.Settled {
		add(h.Target)
	}
	return ids
}

// seenByViewer says whether the viewer may have the event's line as far as the fog
// goes: an event written before the fog was on, or with no NPC in it, is not
// stamped and follows the old rules; a stamped one only goes to those who saw it.
func (ev actionEvent) seenByViewer(v combatViewer) bool {
	return v.master || !ev.Fogged || slices.Contains(ev.SeenBy, v.userID)
}

// ---- publishing ----

// The messages of the combat's live events, built in one place so that the master's,
// the players' and the fog's copies say the same thing.

func encounterChangedMessage(e playdb.Encounter) *playv1.WatchGameSessionResponse {
	return &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_EncounterChanged_{
		EncounterChanged: &playv1.WatchGameSessionResponse_EncounterChanged{EncounterId: e.ID, Revision: e.Revision},
	}}
}

func turnChangedMessage(e playdb.Encounter, turn turnView) *playv1.WatchGameSessionResponse {
	return &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_TurnChanged_{
		TurnChanged: &playv1.WatchGameSessionResponse_TurnChanged{
			EncounterId: e.ID, Round: e.Round, CurrentCombatantId: turn.currentID, MasterTurn: turn.masterTurn,
		},
	}}
}

func combatantMovedMessage(e playdb.Encounter, c playdb.Combatant) *playv1.WatchGameSessionResponse {
	return &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_CombatantMoved_{
		CombatantMoved: &playv1.WatchGameSessionResponse_CombatantMoved{
			EncounterId: e.ID, CombatantId: c.ID, Col: *c.GridCol, Row: *c.GridRow,
		},
	}}
}

// publishTurnChangedToPlayers tells the players who is on turn. Without the fog it is
// one event for all of them; with it, each player gets the turn as they see it (an
// NPC they do not see is "Vez do mestre"). When what they see cannot be read they
// are told nothing here, and read the combat again on the encounter_changed that
// every change of the turn also publishes.
func (s *Service) publishTurnChangedToPlayers(ctx context.Context, campaignID string, d *encounterData) {
	f, err := s.fogSightOf(ctx, campaignID, d.enc)
	switch {
	case err != nil:
		s.logger.ErrorContext(ctx, "play: cannot work out what the players see in a combat", "error", err)
	case f == nil:
		s.hub.Publish(campaignID, live.Event{Audience: live.Audience{Players: true}, Message: turnChangedMessage(d.enc, d.turnFor(combatViewer{}))})
	default:
		for _, u := range f.sight.Users() {
			turn := d.turnFor(combatViewer{userID: u, unseen: f.unseenFor(u, d.cs)})
			s.hub.Publish(campaignID, live.Event{Audience: live.Audience{UserID: u}, Message: turnChangedMessage(d.enc, turn)})
		}
	}
}

// publishMovedToPlayers tells the players that a combatant moved. A player's
// character or creature, and any combatant on a map without the fog, go to every
// player as before; an NPC of a fog map goes only to the players who see its new
// square, and the others get the content-free encounter_changed, so one who saw it
// leave reads the combat again and finds it gone.
func (s *Service) publishMovedToPlayers(ctx context.Context, campaignID string, e playdb.Encounter, c playdb.Combatant) {
	all := live.Event{Audience: live.Audience{Players: true}, Message: combatantMovedMessage(e, c)}
	if c.Kind != kindNPC {
		s.hub.Publish(campaignID, all)
		return
	}
	f, err := s.fogSightOf(ctx, campaignID, e)
	switch {
	case err != nil:
		s.logger.ErrorContext(ctx, "play: cannot work out what the players see in a combat", "error", err)
		s.hub.Publish(campaignID, live.Event{Audience: live.Audience{Players: true}, Message: encounterChangedMessage(e)})
	case f == nil:
		s.hub.Publish(campaignID, all)
	default:
		for _, u := range f.sight.Users() {
			msg := encounterChangedMessage(e)
			if f.seesNPC(u, c) {
				msg = combatantMovedMessage(e, c)
			}
			s.hub.Publish(campaignID, live.Event{Audience: live.Audience{UserID: u}, Message: msg})
		}
	}
}

// positionChanged is the hook after a combat move that landed (and after the undo
// of one): the fog remembers what it showed and tells the players who see the
// squares. from and to are the squares the combatant left and reached; a combatant
// that had none stands where it was put.
func (s *Service) positionChanged(ctx context.Context, campaignID string, e playdb.Encounter, c playdb.Combatant, from *grid.Square) {
	if s.fog == nil || e.MapID == nil || !placed(c) {
		return
	}
	to := squareOfCombatant(c)
	if from == nil {
		from = &to
	}
	s.fog.CombatMoved(ctx, campaignID, *e.MapID, c.Kind == kindNPC, *from, to)
}

// squareOfState is the square a move event's From says the combatant stood on, nil
// when it had none.
func squareOfState(st *moveState) *grid.Square {
	if st == nil || !st.Placed {
		return nil
	}
	return &grid.Square{Col: int(st.Col), Row: int(st.Row)}
}
