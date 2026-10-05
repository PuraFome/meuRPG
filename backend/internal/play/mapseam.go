package play

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/maps/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// What the maps module asks of this one, beside the live session (onscreen.go)
// and AppendEvent (xp.go). Plain SQL, not sqlc, so this file adds nothing to the
// generated queries the combat slices share.

// CombatRunsOnMap says, inside tx, whether a combat that is not ended (in setup or active)
// runs on the map. The maps module asks it before it clears a map's painted
// layers (a new grid or a new image, MR-034): the fight stands on them. It
// implements maps.CombatMaps. The campaign is checked too, so a map of another
// campaign never matches.
func (s *Service) CombatRunsOnMap(ctx context.Context, tx pgx.Tx, campaignID, mapID string) (bool, error) {
	var running bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM encounters AS e
			JOIN game_sessions AS g ON g.id = e.game_session_id
			WHERE g.campaign_id = $1 AND e.map_id = $2 AND e.status <> 'ended'
		)`, campaignID, mapID).Scan(&running)
	if err != nil {
		return false, fmt.Errorf("read whether a combat runs on the map: %w", err)
	}
	return running, nil
}

// OpenSessionID returns the ID of the campaign's open game session, or "" when
// none is open. It locks the session's row inside tx, as AppendEvent does, so
// what the caller writes next belongs to a session that stays open until the
// transaction ends. The maps module remembers it on a treasure found (MR-041),
// for the session's summary. It implements maps.LiveSession.
func (s *Service) OpenSessionID(ctx context.Context, tx pgx.Tx, campaignID string) (string, error) {
	session, err := s.queries.WithTx(tx).GetOpenGameSessionForUpdate(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("lock the open session: %w", err)
	}
	return session.ID, nil
}

// CombatPositions says where the combatants of the combat running on the map
// stand, by character, in squares (MR-036, D6): while a fight runs the fog sees
// from the combatant's square, not the token's (a token only moves back at the
// combat's end). Running is false when no combat that is not ended is on the map.
// Only player characters have a square in the answer (an NPC's copies share a
// character, so it has none and its token's square stands for any light it
// carries); a combatant that has no square yet (a combat in setup) is left out
// too, and its token's square stands. The maps module asks for it, after its own
// authorization check. It reads inside tx when the caller has one (nil: the pool).
// It implements maps.CombatMaps.
func (s *Service) CombatPositions(ctx context.Context, tx pgx.Tx, campaignID, mapID string) (link.CombatPositions, error) {
	var conn playdb.DBTX = s.pool
	if tx != nil {
		conn = tx // inside the caller's transaction: no second connection (plain SQL, so not queriesIn)
	}
	rows, err := conn.Query(ctx, `
		SELECT c.character_id, c.kind, c.grid_col, c.grid_row, c.creature_id, c.dismissed
		FROM combatants AS c
		JOIN encounters AS e ON e.id = c.encounter_id
		JOIN game_sessions AS g ON g.id = e.game_session_id
		WHERE g.campaign_id = $1 AND e.map_id = $2 AND e.status <> 'ended'`, campaignID, mapID)
	if err != nil {
		return link.CombatPositions{}, fmt.Errorf("read the combatants' squares: %w", err)
	}
	defer rows.Close()
	out := link.CombatPositions{Positions: map[string]grid.Square{}, Creatures: map[string]grid.Square{}}
	for rows.Next() {
		var id, kind string
		var col, row *int32
		var creature *string
		var dismissed bool
		if err := rows.Scan(&id, &kind, &col, &row, &creature, &dismissed); err != nil {
			return link.CombatPositions{}, fmt.Errorf("read a combatant's square: %w", err)
		}
		out.Running = true
		// Only a player character is placed by character: copies of one NPC share a
		// character_id, so an NPC has no square here (slice 9.7 sees combatants by
		// their own ID). A light an NPC carries stays on its token during a combat.
		if kind == "player" && col != nil && row != nil {
			out.Positions[id] = grid.Square{Col: int(*col), Row: int(*row)}
		}
		// A creature has its own ID (a dismissed one is out of the fight and has no
		// square): the familiar's eyes see from it (MR-037).
		if kind == "creature" && creature != nil && !dismissed && col != nil && row != nil {
			out.Creatures[*creature] = grid.Square{Col: int(*col), Row: int(*row)}
		}
	}
	if err := rows.Err(); err != nil {
		return link.CombatPositions{}, fmt.Errorf("read the combatants' squares: %w", err)
	}
	return out, nil
}
