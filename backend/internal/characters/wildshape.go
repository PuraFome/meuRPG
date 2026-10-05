package characters

import (
	"context"
	"errors"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	maplink "github.com/PuraFome/meuRPG/backend/internal/maps/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Wild Shape and the familiar's eyes (MR-037, MR-036, Etapa 9). The form lives on
// the character's vitals (character_vitals.wild_shape_beast and wild_shape_hp),
// and this file is the one place that reads it back into the character's numbers:
// a druid in a beast form fights, moves and sees as the beast (rules.Content.
// WildShapeDerived does the arithmetic, never this package), and keeps its own
// hit points waiting. The play module decides when the form starts and ends
// (the action it costs, the damage that ends it) and calls the methods here, in
// its own transaction, through play.VitalsKeeper.

// wildShapeAction is what every feature that grants Wild Shape is called: the
// action is taken through AssumeWildShape, not TakeAction, because it needs the
// beast. In a beast form the action is gone from the turn options.
const wildShapeAction = "feature:wild-shape"

// derive is the character's derived sheet, in its beast form if it is in one: the
// beast's armor class, hit points, speeds, attacks and senses, and no spells. A
// form that is no longer allowed or known (the content changed) is ignored, so the
// character never loses its own numbers.
func (s *Service) derive(full *charactersv1.FullSheet, beast *string) rules.Derived {
	d := rules.Derive(buildOf(full), s.rules)
	if beast == nil || *beast == "" {
		return d
	}
	shaped, err := s.rules.WildShapeDerived(d, *beast)
	if err != nil {
		return d
	}
	// The Wild Shape action itself is not offered again while in the form.
	shaped.Actions = slices.DeleteFunc(slices.Clone(shaped.Actions), func(a rules.Action) bool {
		return strings.HasPrefix(a.Key, wildShapeAction)
	})
	return shaped
}

// beastSize is the size of a beast as a link.Character.Size.
func (s *Service) beastSize(beast string) string {
	if c, ok := s.rules.CreatureByKey(beast); ok {
		return strings.ToLower(c.Size)
	}
	return "medium"
}

// ListWildShapeForms implements charactersv1connect.CharacterServiceHandler.
func (s *Service) ListWildShapeForms(
	ctx context.Context,
	req *connect.Request[charactersv1.ListWildShapeFormsRequest],
) (*connect.Response[charactersv1.ListWildShapeFormsResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	id, ok := parseUUID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}
	row, err := s.queries.GetCharacter(ctx, charactersdb.GetCharacterParams{CampaignID: m.CampaignID, ID: id})
	if err != nil {
		return nil, notFoundOrDB(ctx, s, err, "read a character")
	}
	if !canSee(m, row.Kind, row.Status, row.PlayerUserID) {
		return nil, errCharacterNotFound()
	}
	if row.Kind != kindPlayer {
		return nil, invalidArgument(fieldErr("character_id", "is an NPC: only a player's character has Wild Shape"))
	}
	sheet, err := loadSheet(row.ID, row.Sheet)
	if err != nil {
		return nil, s.dbError(ctx, "read a character", err)
	}
	out := &charactersv1.ListWildShapeFormsResponse{}
	if full := sheet.GetFull(); full != nil {
		b := buildOf(full)
		if limit, ok := s.rules.WildShapeLimitFor(b); ok {
			out.MaxCr, out.NoFly, out.NoSwim = limit.MaxCR, limit.NoFly, limit.NoSwim
		}
		for _, e := range s.rules.WildShapeForms(b) {
			out.Forms = append(out.Forms, creatureSummaryToProto(e))
		}
	}
	return connect.NewResponse(out), nil
}

// notFoundOrDB is not_found for no row, and the database error otherwise.
func notFoundOrDB(ctx context.Context, s *Service, err error, what string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errCharacterNotFound()
	}
	return s.dbError(ctx, what, err)
}

// shapedVitals reads a living player character's vitals and its numbers in a
// combat (speed, size, jumps: the beast's, in a form) inside the transaction.
func (s *Service) shapedVitals(ctx context.Context, q *charactersdb.Queries, campaignID, characterID string) (*playv1.CharacterVitals, link.Character, charactersdb.GetVitalsRow, error) {
	id, ok := parseUUID(characterID)
	if !ok {
		return nil, link.Character{}, charactersdb.GetVitalsRow{}, errCharacterNotFound()
	}
	row, err := q.GetVitals(ctx, charactersdb.GetVitalsParams{CampaignID: campaignID, ID: id})
	if err != nil {
		return nil, link.Character{}, row, notFoundOrWrap(err, "read the vitals")
	}
	m, err := s.maxima(row.ID, row.Sheet, row.WildShapeBeast)
	if err != nil {
		return nil, link.Character{}, row, err
	}
	body, err := s.combatCharacter(row.ID, kindPlayer, row.Name, row.PlayerUserID, row.Sheet, row.WildShapeBeast)
	if err != nil {
		return nil, link.Character{}, row, err
	}
	current := vitalsRow(row)
	if err := s.liveFamiliar(ctx, q, campaignID, &current); err != nil {
		return nil, link.Character{}, row, wrap("read the familiar", err)
	}
	return vitalsToProto(current, m), body, row, nil
}

func notFoundOrWrap(err error, what string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errCharacterNotFound()
	}
	return wrap(what, err)
}

// AssumeWildShape turns a druid into a beast inside tx (MR-037): the beast must be
// one its Wild Shape allows now, and the character in its own shape. The beast
// starts at its full hit points, a pool of its own; the character's wait. It
// returns the vitals before and after and the numbers a combat copies (speed,
// size, jumps). The errors are link.ErrBeastNotAllowed and link.ErrAlreadyInWildShape,
// and `not_found` for anything but a living, active player's character. It implements
// play.VitalsKeeper; the caller spends the use and the action.
func (s *Service) AssumeWildShape(ctx context.Context, tx pgx.Tx, campaignID, characterID, beast string) (before, after *playv1.CharacterVitals, body link.Character, err error) {
	q := s.queries.WithTx(tx)
	before, _, row, err := s.shapedVitals(ctx, q, campaignID, characterID)
	if err != nil {
		return nil, nil, link.Character{}, err
	}
	if row.WildShapeBeast != nil {
		return nil, nil, link.Character{}, link.ErrAlreadyInWildShape
	}
	sheet, err := loadSheet(row.ID, row.Sheet)
	if err != nil {
		return nil, nil, link.Character{}, err
	}
	derived, ok := s.rules.MonsterDerived(beast)
	if !ok || sheet.GetFull() == nil || !s.rules.WildShapeAllows(buildOf(sheet.GetFull()), beast) {
		return nil, nil, link.Character{}, link.ErrBeastNotAllowed
	}
	after, body, err = s.writeWildShape(ctx, q, campaignID, row, before, &beast, int32(max(derived.HitPointsMax, 1))) //nolint:gosec // G115: a stat block's hit points
	return before, after, body, err
}

// SetWildShape puts a druid's form as it says inside tx: the beast with its
// current hit points (1 or more), or its own shape for an empty beast. It is how
// the form ends and how an undo puts it back; it checks nothing about the beast
// (AssumeWildShape does). It implements play.VitalsKeeper.
func (s *Service) SetWildShape(ctx context.Context, tx pgx.Tx, campaignID, characterID, beast string, hp int32) (after *playv1.CharacterVitals, body link.Character, err error) {
	q := s.queries.WithTx(tx)
	before, _, row, err := s.shapedVitals(ctx, q, campaignID, characterID)
	if err != nil {
		return nil, link.Character{}, err
	}
	if beast == "" {
		return s.writeWildShape(ctx, q, campaignID, row, before, nil, 0)
	}
	if _, ok := s.rules.MonsterDerived(beast); !ok {
		return nil, link.Character{}, link.ErrBeastNotAllowed
	}
	return s.writeWildShape(ctx, q, campaignID, row, before, &beast, max(hp, 1))
}

// writeWildShape saves the form (the beast with hp hit points, or none for the
// character's own shape), bumps the vitals' revision and reads the vitals and the
// body back.
func (s *Service) writeWildShape(ctx context.Context, q *charactersdb.Queries, campaignID string, row charactersdb.GetVitalsRow, before *playv1.CharacterVitals, beast *string, hp int32) (*playv1.CharacterVitals, link.Character, error) {
	var err error
	if beast != nil {
		err = q.SetWildShape(ctx, charactersdb.SetWildShapeParams{CharacterID: row.ID, Beast: *beast, Hp: hp, Now: s.now()})
	} else {
		err = q.ClearWildShape(ctx, row.ID)
	}
	if err != nil {
		return nil, link.Character{}, wrap("save the wild shape", err)
	}
	if _, err := q.TouchVitals(ctx, charactersdb.TouchVitalsParams{CharacterID: row.ID, HitPointsCurrent: before.GetHitPointsCurrent(), Now: s.now()}); err != nil {
		return nil, link.Character{}, wrap("save the vitals", err)
	}
	after, body, _, err := s.shapedVitals(ctx, q, campaignID, row.ID)
	return after, body, err
}

// FamiliarOf returns the character's live familiar (a creature whose source is the
// spell Encontrar Familiar), and false when it has none. It implements
// play.VitalsKeeper.
func (s *Service) FamiliarOf(ctx context.Context, tx pgx.Tx, campaignID, characterID string) (link.Creature, bool, error) {
	id, ok := parseUUID(characterID)
	if !ok {
		return link.Creature{}, false, nil
	}
	rows, err := s.queries.WithTx(tx).ListLiveCreaturesOfCharacters(ctx, charactersdb.ListLiveCreaturesOfCharactersParams{CampaignID: campaignID, CharacterIds: []string{id}})
	if err != nil {
		return link.Creature{}, false, wrap("list the creatures", err)
	}
	for _, r := range rows {
		if r.CharacterCreature.Source == creatureSourceFamiliar {
			return s.creatureOf(creatureView{CharacterCreature: r.CharacterCreature, ownerUserID: r.PlayerUserID}), true, nil
		}
	}
	return link.Creature{}, false, nil
}

// SetFamiliarSight records inside tx that the player looks through the creature's
// eyes (an empty creatureID: they stopped), whether it started in a combat and
// the conditions it gave the combatant, and returns the vitals after. It
// implements play.VitalsKeeper.
func (s *Service) SetFamiliarSight(ctx context.Context, tx pgx.Tx, campaignID, characterID, creatureID string, inCombat bool, conditions []string) (*playv1.CharacterVitals, error) {
	q := s.queries.WithTx(tx)
	before, _, row, err := s.shapedVitals(ctx, q, campaignID, characterID)
	if err != nil {
		return nil, err
	}
	params := charactersdb.SetFamiliarSightParams{
		CharacterID: row.ID, HitPointsCurrent: before.GetHitPointsCurrent(), InCombat: inCombat, Conditions: nonNilList(conditions), Now: s.now(),
	}
	if creatureID != "" {
		id, ok := parseUUID(creatureID)
		if !ok {
			return nil, errCreatureNotFound()
		}
		params.CreatureID = &id
	}
	if _, err := q.SetFamiliarSight(ctx, params); err != nil {
		return nil, wrap("save the familiar's sight", err)
	}
	after, _, _, err := s.shapedVitals(ctx, q, campaignID, row.ID)
	return after, err
}

func nonNilList(l []string) []string {
	if l == nil {
		return []string{}
	}
	return l
}

// MapCreatures returns those of ids that are live creatures of a living player's
// character of the campaign, oldest first: the ones that may have a token on a
// map (MR-037). A dismissed creature, or one whose owner died, is left out. It
// implements maps.CharacterDirectory.
func (s *Service) MapCreatures(ctx context.Context, campaignID string, ids []string) ([]maplink.MapCreature, error) {
	valid := make([]string, 0, len(ids))
	for _, id := range ids {
		if id, ok := parseUUID(id); ok {
			valid = append(valid, id)
		}
	}
	if len(valid) == 0 {
		return nil, nil
	}
	rows, err := s.queries.ListMapCreatures(ctx, charactersdb.ListMapCreaturesParams{CampaignID: campaignID, Ids: valid})
	if err != nil {
		return nil, s.dbError(ctx, "list the creatures on a map", err)
	}
	out := make([]maplink.MapCreature, 0, len(rows))
	for _, r := range rows {
		out = append(out, maplink.MapCreature{
			ID: r.ID, OwnerCharacterID: r.CharacterID, OwnerUserID: deref(r.PlayerUserID), Name: r.Name, MonsterKey: r.MonsterKey,
		})
	}
	return out, nil
}
