package maps

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
)

// The light a character carries (MR-036, Etapa 9, D6): a light preset's key on
// the character's token, set by the player for their own character and by the
// master for anyone. It moves with the token. The master and the character's
// own player read it (MapToken.carried_light); what the other players see of it
// is its light, which the fog's read slice (9.4) works out from the key.

// SetCarriedLight implements mapsv1connect.MapServiceHandler.
func (s *Service) SetCarriedLight(
	ctx context.Context,
	req *connect.Request[mapsv1.SetCarriedLightRequest],
) (*connect.Response[mapsv1.SetCarriedLightResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	var carried *string // NULL for none
	if key := req.Msg.GetLightKey(); key != "" {
		if _, ok := s.rules.LightPreset(key); !ok {
			return nil, badSpec("light_key is not a light preset")
		}
		carried = &key
	}
	v, err := s.viewerOf(ctx, m)
	if err != nil {
		return nil, err
	}
	character, err := s.livingCharacter(ctx, m.CampaignID, req.Msg.GetCharacterId())
	if err != nil {
		return nil, err
	}
	if !v.master {
		// A player carries a light for their own character only. Another
		// player's character is known to the party, an NPC is not: the answer
		// for an NPC is the one for a character that does not exist.
		switch {
		case character.GetKind() != charactersv1.CharacterKind_CHARACTER_KIND_PLAYER:
			return nil, errCharacterNotFound()
		case character.GetPlayerUserId() != m.UserID:
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only the character's player or the master may do this"))
		}
	}

	var mapRow mapsdb.Map
	var token mapsdb.MapToken
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		if mapRow, err = s.campaignMap(ctx, q, m.CampaignID, mapID); err != nil {
			return err
		}
		if !v.seesMap(mapRow.ID, mapRow.RevealedAt) {
			return errMapNotFound() // a hidden map is not found to a player (RN-10)
		}
		// A hidden token is not there for a player: the answer is the one for no
		// token at all, so the call never tells them where their character is hidden.
		current, err := q.GetMapTokenForUpdate(ctx, mapsdb.GetMapTokenForUpdateParams{MapID: mapID, CharacterID: character.GetId()})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !v.master && current.Hidden) {
			return errTokenNotFound()
		}
		if err != nil {
			return fmt.Errorf("find token: %w", err)
		}
		token, err = q.SetMapTokenCarriedLight(ctx, mapsdb.SetMapTokenCarriedLightParams{
			MapID: mapID, CharacterID: character.GetId(), CarriedLight: carried, Now: s.now(),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errTokenNotFound()
		}
		if err != nil {
			return fmt.Errorf("set the carried light: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "set a carried light", err)
	}
	// The master reads the key, and so does the character's own player; nobody
	// else is told (what the others see changes with the fog's reads).
	s.publishMapChanged(m.CampaignID, mapID, false)
	if owner := character.GetPlayerUserId(); owner != "" && playersSee(mapID, mapRow.RevealedAt, v.currentMap) {
		s.live.PublishToUsers(m.CampaignID, []string{owner}, mapChangedEvent(mapID))
	}
	return connect.NewResponse(&mapsv1.SetCarriedLightResponse{Token: tokenToProto(token, character, v)}), nil
}
