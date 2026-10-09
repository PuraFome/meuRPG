package play

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/encounter"
)

// The encounter builder (MR-043, RN-29; Etapa 10, slice 10.9c): the master measures a set of
// SRD creatures against the party, has one drawn from a seed, swaps a creature for another of
// the same XP, and keeps an encounter on a battle point of a map. The arithmetic is
// rules/encounter; the XP table is the SRD 5.2.1's "XP Budget per Character". Everything is
// the master's secret (RN-10): a player, a pending member and an outsider get `not_found`, and
// no read that serves a player touches the saved encounter (the table battle_encounters).

const (
	// maxEncounterEntries is how many kinds of creature an encounter lists.
	maxEncounterEntries = 20
	// maxEntryCount is the most monsters of one creature in an entry: a combat holds 40.
	maxEntryCount = 40
	// maxExtraParty is how many NPCs the master may add to the party for an encounter.
	maxExtraParty = 10
	// maxPartyNpcName is the longest name of an extra party member.
	maxPartyNpcName = 80
	// maxCreatureLevel is the top character level (rules.MaxLevel).
	maxCreatureLevel = rules.MaxLevel
)

// requireBuilder is the check at the top of every handler: the master of the campaign. A
// player, a pending member and an outsider all get the same `not_found`, so the builder and the
// saved encounters are not even known to exist (RN-10).
func requireBuilder(ctx context.Context, campaignID string) (authz.Membership, error) {
	m, err := authz.RequireCampaignMember(ctx, campaignID)
	if err != nil {
		return authz.Membership{}, err
	}
	if m.Role != authz.RoleMaster {
		return authz.Membership{}, connect.NewError(connect.CodeNotFound, errors.New("campaign not found"))
	}
	return m, nil
}

// n32 is a count or an XP as an int32: every number the builder sends is small (at most 20 kinds
// of at most 40 creatures, a few thousand XP a level), so the clamp never changes one.
func n32(n int) int32 { return clamp32(n, 0, math.MaxInt32) }

// errUnknownCreature is the refusal of a creature key that is not an SRD creature: a
// failed_precondition with the typed reason, so the app can tell the master to take it out
// (a saved encounter outlives a content change).
func errUnknownCreature(key string) error {
	err := connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%q is not an SRD creature", key))
	if detail, detailErr := connect.NewErrorDetail(&playv1.EncounterBuildBlocked{Reason: playv1.EncounterBuildBlockedReason_ENCOUNTER_BUILD_BLOCKED_REASON_UNKNOWN_CREATURE}); detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

func invalid(format string, a ...any) error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(format, a...))
}

// builderParty is who an encounter is measured against: the members and their levels.
type builderParty struct {
	members []*playv1.EncounterPartyMember
	levels  []int
	// fighters is how many combatants the party brings to a combat: its player characters and
	// their live creatures (a familiar, summoned animals), which join with their owners. The NPCs
	// the master adds are not combatants.
	fighters int
}

// encounterParty reads the party: the campaign's living player characters (their total level)
// and the NPCs the master adds, with the level he gives them (question 86). It reads before any
// transaction of the caller.
func (s *Service) encounterParty(ctx context.Context, campaignID string, extra []*playv1.PartyNpc) (builderParty, error) {
	if len(extra) > maxExtraParty {
		return builderParty{}, invalid("extra_party has at most %d NPCs", maxExtraParty)
	}
	// What the master wrote, checked before the database is asked anything.
	ids := make([]string, 0, len(extra))
	for i, e := range extra {
		if e.GetLevel() < 1 || e.GetLevel() > maxCreatureLevel {
			return builderParty{}, invalid("extra_party[%d].level must be 1 to %d", i, maxCreatureLevel)
		}
		if strings.TrimSpace(e.GetName()) != "" || e.GetCharacterId() == "" {
			if _, err := names.Clean(e.GetName(), maxPartyNpcName); err != nil {
				return builderParty{}, invalid("extra_party[%d].name %v", i, err)
			}
		}
		if e.GetCharacterId() == "" {
			continue
		}
		id, err := uuid.Parse(e.GetCharacterId())
		if err != nil {
			return builderParty{}, invalid("extra_party[%d].character_id is not a character of the campaign", i)
		}
		if slices.Contains(ids, id.String()) {
			return builderParty{}, invalid("extra_party[%d] repeats a character", i)
		}
		ids = append(ids, id.String())
	}
	var found map[string]string // the NPC's name by id
	if len(ids) > 0 {
		chars, err := s.roster.CombatCharacters(ctx, nil, campaignID, ids)
		if err != nil {
			return builderParty{}, s.dbError(ctx, "read the NPCs of the party", err)
		}
		found = make(map[string]string, len(chars))
		for _, c := range chars {
			if !c.Player && !c.CombatOnly { // the NPC the app keeps for a creature is never a companion
				found[c.ID] = c.Name
			}
		}
	}
	players, err := s.roster.PartyLevels(ctx, nil, campaignID)
	if err != nil {
		return builderParty{}, s.dbError(ctx, "read the party", err)
	}
	p := builderParty{fighters: len(players)}
	if len(players) > 0 {
		owners := make([]string, len(players))
		for i, m := range players {
			owners[i] = m.ID
		}
		creatures, err := s.roster.CharacterCreatures(ctx, nil, campaignID, owners)
		if err != nil {
			return builderParty{}, s.dbError(ctx, "read the party's creatures", err)
		}
		p.fighters += len(creatures)
	}
	for _, m := range players {
		p.members = append(p.members, &playv1.EncounterPartyMember{CharacterId: m.ID, Name: m.Name, Level: n32(m.Level)})
		p.levels = append(p.levels, m.Level)
	}
	for i, e := range extra {
		name := strings.TrimSpace(e.GetName())
		if e.GetCharacterId() != "" {
			id, _ := uuid.Parse(e.GetCharacterId())
			npcName, ok := found[id.String()]
			if !ok {
				return builderParty{}, invalid("extra_party[%d].character_id is not an NPC of the campaign", i)
			}
			name = cmpName(name, npcName)
		}
		p.members = append(p.members, &playv1.EncounterPartyMember{CharacterId: e.GetCharacterId(), Name: name, Level: e.GetLevel(), Npc: true})
		p.levels = append(p.levels, int(e.GetLevel()))
	}
	return p, nil
}

// cmpName is the name the master gave a party NPC, or the NPC's own when he gave none.
func cmpName(given, own string) string {
	if given != "" {
		return given
	}
	return own
}

// encounterEntries reads and checks the creatures of a request: each an SRD creature, a count
// of 1 to 40 (unset: 1), a creature once, at most 20 kinds, and a base name of 1 to 30
// characters when there is one. The names, in the order of the entries, come back too.
func encounterEntries(c *rules.Content, in []*playv1.MonsterGroup) (entries []encounter.Entry, base []string, err error) {
	if len(in) > maxEncounterEntries {
		return nil, nil, invalid("an encounter has at most %d kinds of creature", maxEncounterEntries)
	}
	for i, g := range in {
		count := int(g.GetCount())
		if count == 0 {
			count = 1
		}
		switch {
		case !c.HasCreature(g.GetCreatureKey()):
			return nil, nil, errUnknownCreature(g.GetCreatureKey())
		case count < 1 || count > maxEntryCount:
			return nil, nil, invalid("entries[%d].count must be 1 to %d", i, maxEntryCount)
		case slices.ContainsFunc(entries, func(e encounter.Entry) bool { return e.Key == g.GetCreatureKey() }):
			return nil, nil, invalid("entries[%d] repeats a creature", i)
		}
		name := ""
		if strings.TrimSpace(g.GetName()) != "" {
			if name, err = names.Clean(g.GetName(), maxMonsterName); err != nil {
				return nil, nil, invalid("entries[%d].name %v", i, err)
			}
		}
		entries = append(entries, encounter.Entry{Key: g.GetCreatureKey(), Count: count})
		base = append(base, name)
	}
	return entries, base, nil
}

// summaryOf is a creature as a list row shows it (the bestiary's CreatureSummary).
func summaryOf(e rules.CreatureEntry) *rulesv1.CreatureSummary {
	return &rulesv1.CreatureSummary{
		Key: e.Key, Name: e.Name, NamePt: e.NamePT, Size: e.Size, SizePt: e.SizeNamePT,
		Type: e.Type, TypePt: e.TypeNamePT, Subtype: e.Subtype, ChallengeRating: e.ChallengeRating, Xp: n32(e.XP),
		CanFly: e.CanFly, CanSwim: e.CanSwim, ArmorClass: n32(e.ArmorClass), HitPoints: n32(e.HitPoints),
	}
}

var bandOf = map[encounter.Band]playv1.EncounterBand{
	encounter.BandLow:      playv1.EncounterBand_ENCOUNTER_BAND_LOW,
	encounter.BandModerate: playv1.EncounterBand_ENCOUNTER_BAND_MODERATE,
	encounter.BandHigh:     playv1.EncounterBand_ENCOUNTER_BAND_HIGH,
	encounter.BandAbove:    playv1.EncounterBand_ENCOUNTER_BAND_ABOVE_HIGH,
}

// evaluationOf measures the entries against the party and writes the answer. base are the
// base names saved with the entries, or nil.
func evaluationOf(c *rules.Content, p builderParty, entries []encounter.Entry, base []string) (*playv1.EncounterEvaluation, error) {
	ev, err := c.EvaluateEncounter(p.levels, entries)
	if err != nil {
		return nil, fmt.Errorf("measure an encounter: %w", err)
	}
	out := &playv1.EncounterEvaluation{
		Party: p.members, Budget: &playv1.XpBudget{Low: n32(ev.Budget.Low), Moderate: n32(ev.Budget.Moderate), High: n32(ev.Budget.High)},
		TotalXp: n32(ev.TotalXP), CreatureCount: n32(ev.Creatures), Band: bandOf[ev.Band], OverXp: n32(ev.OverXP),
		MaxCr: ev.MaxCR, LowestLevel: n32(ev.LowestLevel),
	}
	aboveCap := false
	for i, l := range ev.Lines {
		line := &playv1.EncounterLine{Creature: summaryOf(l.Creature), Count: n32(l.Count), SubtotalXp: n32(l.Subtotal), AboveCap: l.AboveCap}
		if i < len(base) {
			line.Name = base[i]
		}
		out.Lines = append(out.Lines, line)
		aboveCap = aboveCap || l.AboveCap
	}
	if ev.Band == encounter.BandAbove && len(p.levels) > 0 {
		out.Warnings = append(out.Warnings, playv1.EncounterWarning_ENCOUNTER_WARNING_ABOVE_HIGH)
	}
	if aboveCap {
		out.Warnings = append(out.Warnings, playv1.EncounterWarning_ENCOUNTER_WARNING_ABOVE_CR_CAP)
	}
	if len(p.levels) == 0 {
		out.Warnings = append(out.Warnings, playv1.EncounterWarning_ENCOUNTER_WARNING_NO_PARTY)
	}
	if ev.Creatures+p.fighters > maxCombatants {
		out.Warnings = append(out.Warnings, playv1.EncounterWarning_ENCOUNTER_WARNING_TOO_MANY)
	}
	return out, nil
}

// EvaluateEncounter implements playv1connect.EncounterServiceHandler.
func (s *Service) EvaluateEncounter(
	ctx context.Context,
	req *connect.Request[playv1.EvaluateEncounterRequest],
) (*connect.Response[playv1.EvaluateEncounterResponse], error) {
	m, err := requireBuilder(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	content, err := s.roster.RulesContent(ctx, nil, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read the rules content", err)
	}
	entries, _, err := encounterEntries(content, req.Msg.GetEntries())
	if err != nil {
		return nil, err
	}
	p, err := s.encounterParty(ctx, m.CampaignID, req.Msg.GetExtraParty())
	if err != nil {
		return nil, err
	}
	ev, err := evaluationOf(content, p, entries, nil)
	if err != nil {
		return nil, s.dbError(ctx, "measure an encounter", err)
	}
	return connect.NewResponse(&playv1.EvaluateEncounterResponse{Evaluation: ev}), nil
}

// errBuildBlocked is GenerateEncounter's failed_precondition, with its typed reason.
func errBuildBlocked(reason playv1.EncounterBuildBlockedReason, msg string) error {
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	if detail, detailErr := connect.NewErrorDetail(&playv1.EncounterBuildBlocked{Reason: reason}); detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

// GenerateEncounter implements playv1connect.EncounterServiceHandler.
func (s *Service) GenerateEncounter(
	ctx context.Context,
	req *connect.Request[playv1.GenerateEncounterRequest],
) (*connect.Response[playv1.GenerateEncounterResponse], error) {
	m, err := requireBuilder(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	var band encounter.Band
	switch req.Msg.GetBand() {
	case playv1.EncounterBand_ENCOUNTER_BAND_UNSPECIFIED, playv1.EncounterBand_ENCOUNTER_BAND_MODERATE:
		band = encounter.BandModerate
	case playv1.EncounterBand_ENCOUNTER_BAND_LOW:
		band = encounter.BandLow
	case playv1.EncounterBand_ENCOUNTER_BAND_HIGH:
		band = encounter.BandHigh
	default:
		return nil, invalid("band must be LOW, MODERATE or HIGH")
	}
	content, err := s.roster.RulesContent(ctx, nil, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read the rules content", err)
	}
	p, err := s.encounterParty(ctx, m.CampaignID, req.Msg.GetExtraParty())
	if err != nil {
		return nil, err
	}
	seed := req.Msg.GetSeed()
	if seed == 0 {
		// "Gerar outro": the server draws one, from 1 to 10000, with two d100.
		hi, err1 := s.roller.Roll(100)
		lo, err2 := s.roller.Roll(100)
		if err := errors.Join(err1, err2); err != nil {
			return nil, s.dbError(ctx, "draw a seed", err)
		}
		seed = uint32(n32(max(hi-1, 0)*100 + max(lo, 1))) //nolint:gosec // 1 to 10000, never negative
	}
	// The party's own combatants take room in the combat of 40: the encounter never fills it.
	room := min(encounter.DefaultMaxTotal, maxCombatants-p.fighters)
	if room < 1 {
		return nil, errBuildBlocked(playv1.EncounterBuildBlockedReason_ENCOUNTER_BUILD_BLOCKED_REASON_NOTHING_FITS, "the party fills the combat: no room for a creature")
	}
	ev, err := content.GenerateEncounter(p.levels, band, req.Msg.GetCreatureType(), seed, room)
	switch {
	case errors.Is(err, rules.ErrEncounterNoParty):
		return nil, errBuildBlocked(playv1.EncounterBuildBlockedReason_ENCOUNTER_BUILD_BLOCKED_REASON_NO_PARTY, "the party is empty: no living player character and no NPC added")
	case errors.Is(err, rules.ErrEncounterNothingFits):
		return nil, errBuildBlocked(playv1.EncounterBuildBlockedReason_ENCOUNTER_BUILD_BLOCKED_REASON_NOTHING_FITS, "no creature fits that difficulty")
	case errors.Is(err, rules.ErrEncounterType):
		return nil, invalid("creature_type is not an SRD creature type")
	case err != nil:
		return nil, s.dbError(ctx, "generate an encounter", err)
	}
	entries := make([]encounter.Entry, len(ev.Lines))
	for i, l := range ev.Lines {
		entries[i] = encounter.Entry{Key: l.Creature.Key, Count: l.Count}
	}
	out, err := evaluationOf(content, p, entries, nil)
	if err != nil {
		return nil, s.dbError(ctx, "measure an encounter", err)
	}
	return connect.NewResponse(&playv1.GenerateEncounterResponse{Evaluation: out, Seed: seed}), nil
}

// ListEncounterSwaps implements playv1connect.EncounterServiceHandler.
func (s *Service) ListEncounterSwaps(
	ctx context.Context,
	req *connect.Request[playv1.ListEncounterSwapsRequest],
) (*connect.Response[playv1.ListEncounterSwapsResponse], error) {
	m, err := requireBuilder(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	content, err := s.roster.RulesContent(ctx, nil, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read the rules content", err)
	}
	swaps, err := content.EncounterSwaps(req.Msg.GetCreatureKey(), req.Msg.GetCreatureType())
	switch {
	case errors.Is(err, rules.ErrEncounterCreature):
		return nil, invalid("creature_key is not an SRD creature")
	case errors.Is(err, rules.ErrEncounterType):
		return nil, invalid("creature_type is not an SRD creature type")
	case err != nil:
		return nil, s.dbError(ctx, "list the swaps", err)
	}
	out := &playv1.ListEncounterSwapsResponse{}
	for _, e := range swaps {
		out.Creatures = append(out.Creatures, summaryOf(e))
	}
	return connect.NewResponse(out), nil
}

// --- the encounter kept on a battle point ---

// normalBattleEncounter checks the encounter of a save and writes it the way it is kept: the
// counts and the modes explicit, the names cleaned, so that saving the same encounter twice
// writes the same thing.
func normalBattleEncounter(c *rules.Content, in *playv1.BattleEncounter) (*playv1.BattleEncounter, []encounter.Entry, []string, error) {
	if len(in.GetMonsters()) == 0 {
		return nil, nil, nil, invalid("encounter.monsters must have at least one creature")
	}
	entries, base, err := encounterEntries(c, in.GetMonsters())
	if err != nil {
		return nil, nil, nil, err
	}
	hp := in.GetHitPoints()
	switch hp {
	case playv1.MonsterHitPoints_MONSTER_HIT_POINTS_UNSPECIFIED:
		hp = playv1.MonsterHitPoints_MONSTER_HIT_POINTS_AVERAGE
	case playv1.MonsterHitPoints_MONSTER_HIT_POINTS_AVERAGE, playv1.MonsterHitPoints_MONSTER_HIT_POINTS_ROLLED:
	default:
		return nil, nil, nil, invalid("encounter.hit_points is not a known mode")
	}
	hidden := in.Hidden == nil || in.GetHidden() // as a new NPC: hidden
	out := &playv1.BattleEncounter{HitPoints: hp, Hidden: &hidden}
	for i, e := range entries {
		out.Monsters = append(out.Monsters, &playv1.MonsterGroup{CreatureKey: e.Key, Count: n32(e.Count), Name: base[i]})
	}
	return out, entries, base, nil
}

// battleEncounterOf reads a saved encounter back.
func battleEncounterOf(raw []byte) (*playv1.BattleEncounter, error) {
	var be playv1.BattleEncounter
	if err := protoIn.Unmarshal(raw, &be); err != nil {
		return nil, fmt.Errorf("read a saved encounter: %w", err)
	}
	return &be, nil
}

// savedEntries is the entries and base names of a saved encounter.
func savedEntries(be *playv1.BattleEncounter) ([]encounter.Entry, []string) {
	entries := make([]encounter.Entry, len(be.GetMonsters()))
	base := make([]string, len(be.GetMonsters()))
	for i, g := range be.GetMonsters() {
		entries[i] = encounter.Entry{Key: g.GetCreatureKey(), Count: int(g.GetCount())}
		base[i] = g.GetName()
	}
	return entries, base
}

// battlePointID checks that the point is a battle point of the campaign: not_found for any
// other (and for an ID that is not a UUID). It returns the point's map.
func (s *Service) battlePointID(ctx context.Context, campaignID, raw string) (pointID, mapID string, err error) {
	id, perr := uuid.Parse(raw)
	if perr != nil {
		return "", "", connect.NewError(connect.CodeNotFound, errors.New("point not found"))
	}
	point, err := s.maps.BattlePoint(ctx, nil, campaignID, id.String())
	if err != nil {
		return "", "", s.dbError(ctx, "find the battle point", err)
	}
	return id.String(), point.MapID, nil
}

// SaveBattleEncounter implements playv1connect.EncounterServiceHandler.
func (s *Service) SaveBattleEncounter(
	ctx context.Context,
	req *connect.Request[playv1.SaveBattleEncounterRequest],
) (*connect.Response[playv1.SaveBattleEncounterResponse], error) {
	m, err := requireBuilder(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	pointID, mapID, err := s.battlePointID(ctx, m.CampaignID, req.Msg.GetMapPointId())
	if err != nil {
		return nil, err
	}
	content, err := s.roster.RulesContent(ctx, nil, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read the rules content", err)
	}
	be, entries, base, err := normalBattleEncounter(content, req.Msg.GetEncounter())
	if err != nil {
		return nil, err
	}
	p, err := s.encounterParty(ctx, m.CampaignID, req.Msg.GetExtraParty())
	if err != nil {
		return nil, err
	}
	ev, err := evaluationOf(content, p, entries, base)
	if err != nil {
		return nil, s.dbError(ctx, "measure an encounter", err)
	}
	body := toJSON(be)

	var row playdb.BattleEncounter
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		// Saving what the point already keeps changes nothing (not even the time).
		old, err := q.GetBattleEncounter(ctx, playdb.GetBattleEncounterParams{CampaignID: m.CampaignID, MapPointID: pointID})
		if err == nil {
			if prev, perr := battleEncounterOf(old.Encounter); perr == nil && proto.Equal(prev, be) {
				row = old
				return nil
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("read the saved encounter: %w", err)
		}
		row, err = q.UpsertBattleEncounter(ctx, playdb.UpsertBattleEncounterParams{
			MapPointID: pointID, CampaignID: m.CampaignID, MapID: mapID, Encounter: body, CreatedAt: s.now(),
		})
		if err != nil {
			return fmt.Errorf("save the encounter: %w", err)
		}
		return nil
	})
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23503" {
		err = connect.NewError(connect.CodeNotFound, errors.New("point not found")) // the point went away meanwhile
	}
	if err != nil {
		return nil, s.dbError(ctx, "save an encounter", err)
	}
	return connect.NewResponse(&playv1.SaveBattleEncounterResponse{Encounter: be, Evaluation: ev, UpdatedAt: timestamppb.New(row.UpdatedAt)}), nil
}

// GetBattleEncounter implements playv1connect.EncounterServiceHandler.
func (s *Service) GetBattleEncounter(
	ctx context.Context,
	req *connect.Request[playv1.GetBattleEncounterRequest],
) (*connect.Response[playv1.GetBattleEncounterResponse], error) {
	m, err := requireBuilder(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	pointID, _, err := s.battlePointID(ctx, m.CampaignID, req.Msg.GetMapPointId())
	if err != nil {
		return nil, err
	}
	row, err := s.queries.GetBattleEncounter(ctx, playdb.GetBattleEncounterParams{CampaignID: m.CampaignID, MapPointID: pointID})
	if errors.Is(err, pgx.ErrNoRows) {
		return connect.NewResponse(&playv1.GetBattleEncounterResponse{}), nil
	}
	if err != nil {
		return nil, s.dbError(ctx, "read the saved encounter", err)
	}
	be, err := battleEncounterOf(row.Encounter)
	if err != nil {
		return nil, s.dbError(ctx, "read the saved encounter", err)
	}
	content, err := s.roster.RulesContent(ctx, nil, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read the rules content", err)
	}
	p, err := s.encounterParty(ctx, m.CampaignID, req.Msg.GetExtraParty())
	if err != nil {
		return nil, err
	}
	entries, base := savedEntries(be)
	// A creature that is no longer in the SRD (the content changed since the save) is left out of the
	// measure and reported, so the master can still read and fix the encounter.
	var unknown []string
	known, knownBase := entries[:0:0], base[:0:0]
	for i, e := range entries {
		if content.HasCreature(e.Key) {
			known, knownBase = append(known, e), append(knownBase, base[i])
		} else {
			unknown = append(unknown, e.Key)
		}
	}
	ev, err := evaluationOf(content, p, known, knownBase)
	if err != nil {
		return nil, s.dbError(ctx, "measure an encounter", err)
	}
	if len(unknown) > 0 {
		ev.Warnings = append(ev.Warnings, playv1.EncounterWarning_ENCOUNTER_WARNING_UNKNOWN_CREATURE)
	}
	return connect.NewResponse(&playv1.GetBattleEncounterResponse{Encounter: be, Evaluation: ev, UpdatedAt: timestamppb.New(row.UpdatedAt), UnknownCreatureKeys: unknown}), nil
}

// ClearBattleEncounter implements playv1connect.EncounterServiceHandler.
func (s *Service) ClearBattleEncounter(
	ctx context.Context,
	req *connect.Request[playv1.ClearBattleEncounterRequest],
) (*connect.Response[playv1.ClearBattleEncounterResponse], error) {
	m, err := requireBuilder(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	// The point must be a battle point of the campaign, unless it still keeps an encounter: a point
	// that stopped being a battle point must not hold one the master cannot clear.
	id, perr := uuid.Parse(req.Msg.GetMapPointId())
	if perr != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("point not found"))
	}
	var cleared int64
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		cleared, err = s.queries.WithTx(tx).DeleteBattleEncounter(ctx, playdb.DeleteBattleEncounterParams{CampaignID: m.CampaignID, MapPointID: id.String()})
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "clear a saved encounter", err)
	}
	if cleared == 0 {
		if _, _, err := s.battlePointID(ctx, m.CampaignID, id.String()); err != nil {
			return nil, err
		}
	}
	return connect.NewResponse(&playv1.ClearBattleEncounterResponse{}), nil
}

// ListBattleEncounters implements playv1connect.EncounterServiceHandler.
func (s *Service) ListBattleEncounters(
	ctx context.Context,
	req *connect.Request[playv1.ListBattleEncountersRequest],
) (*connect.Response[playv1.ListBattleEncountersResponse], error) {
	m, err := requireBuilder(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	mapID, perr := uuid.Parse(req.Msg.GetMapId())
	if perr != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("map not found"))
	}
	if _, err := s.maps.MapGrid(ctx, nil, m.CampaignID, mapID.String()); err != nil { // not_found for another campaign's map
		return nil, s.dbError(ctx, "find the map", err)
	}
	rows, err := s.queries.ListBattleEncounters(ctx, playdb.ListBattleEncountersParams{CampaignID: m.CampaignID, MapID: mapID.String()})
	if err != nil {
		return nil, s.dbError(ctx, "list the saved encounters", err)
	}
	out := &playv1.ListBattleEncountersResponse{}
	for _, r := range rows {
		be, err := battleEncounterOf(r.Encounter)
		if err != nil {
			return nil, s.dbError(ctx, "read a saved encounter", err)
		}
		n := 0
		for _, g := range be.GetMonsters() {
			n += int(g.GetCount())
		}
		out.Encounters = append(out.Encounters, &playv1.BattleEncounterSummary{MapPointId: r.MapPointID, CreatureCount: n32(n), UpdatedAt: timestamppb.New(r.UpdatedAt)})
	}
	return connect.NewResponse(out), nil
}
