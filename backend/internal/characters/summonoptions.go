package characters

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// GetSummonOptions implements charactersv1connect.CharacterServiceHandler: what
// a character can summon from its sheet (MR-037). It is a read of the same
// rules CastSummon enforces (rules.SummonOptions for what a circle may bring,
// summonStanding for what the sheet lets the character cast), so the
// "Criaturas" panel keeps no copy of them. Same access as ListCharacterCreatures:
// the master and the owner's player; anyone else gets `not_found` (RN-20). It
// carries no hit points.
func (s *Service) GetSummonOptions(
	ctx context.Context,
	req *connect.Request[charactersv1.GetSummonOptionsRequest],
) (*connect.Response[charactersv1.GetSummonOptionsResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	id, ok := parseUUID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}
	// The character, its slots and the creatures a casting would send away are one
	// moment: a casting that commits between the reads would show the slot still
	// free next to the familiar it just made.
	out := &charactersv1.GetSummonOptionsResponse{}
	err = db.ReadTx(ctx, s.pool, func(tx pgx.Tx) error {
		out = &charactersv1.GetSummonOptionsResponse{} // the closure may run again
		q := s.queries.WithTx(tx)
		row, err := q.GetCharacter(ctx, charactersdb.GetCharacterParams{CampaignID: m.CampaignID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return errCharacterNotFound()
		}
		if err != nil {
			return wrap("read a character", err)
		}
		if !canSee(m, row.Kind, row.Status, row.PlayerUserID) {
			return errCharacterNotFound()
		}
		if row.Kind != kindPlayer {
			return invalidArgument(fieldErr("character_id", "is an NPC: only a player's character has creatures"))
		}
		b, err := s.summonCharacter(ctx, tx, m.CampaignID, id)
		if err != nil {
			if connect.CodeOf(err) == connect.CodeNotFound {
				return nil // no full sheet (or a dead character): nothing to cast
			}
			return err
		}
		vitals, err := s.getVitals(ctx, tx, m.CampaignID, id, false)
		if err != nil {
			return err
		}
		// The slots, lowest circle first, the pact magic slots after their circle.
		for _, u := range vitals.GetSpellSlots() {
			out.Slots = append(out.Slots, &charactersv1.SummonSlot{Level: u.GetLevel(), Total: u.GetTotal(), Free: u.GetTotal() - u.GetUsed()})
		}
		if p := vitals.GetPactSlots(); p != nil && p.GetTotal() > 0 {
			out.Slots = append(out.Slots, &charactersv1.SummonSlot{Level: p.GetSlotLevel(), Pact: true, Total: p.GetTotal(), Free: p.GetTotal() - p.GetUsed()})
			slices.SortStableFunc(out.Slots, func(a, c *charactersv1.SummonSlot) int { return int(a.GetLevel() - c.GetLevel()) })
		}
		content, err := s.contentFor(ctx, tx, m.CampaignID)
		if err != nil {
			return wrap("read rules content", err)
		}
		d := rules.Derive(b, content)
		for _, key := range content.SummonSpells() {
			spell, err := s.summonSpellOptions(ctx, q, content, m.CampaignID, id, b, d, key, out.Slots)
			if err != nil {
				return err
			}
			if spell != nil {
				out.Spells = append(out.Spells, spell)
			}
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "read the summon options", err)
	}
	return connect.NewResponse(out), nil
}

// summonSpellOptions is one spell's entry, or nil when the character cannot cast
// it (no ritual and no ready slot cast).
func (s *Service) summonSpellOptions(ctx context.Context, q *charactersdb.Queries, content *rules.Content, campaignID, characterID string, b rules.Build, d rules.Derived, key string, slots []*charactersv1.SummonSlot) (*charactersv1.SummonSpellOptions, error) {
	det, ok := content.SpellDetails(key)
	if !ok {
		return nil, nil
	}
	own, err := content.SummonOptions(key, det.Spell.Level, b)
	if err != nil {
		return nil, fmt.Errorf("summon options of %s: %w", key, err)
	}
	st := s.summonStanding(d, key, own.Ritual)
	canSlot := st.known && st.prepared
	if !st.canRitual && !canSlot {
		return nil, nil
	}
	out := &charactersv1.SummonSpellOptions{
		SpellKey: key, NamePt: content.NamePT(key), Level: int32(det.Spell.Level), CastingTimePt: castingTimePT(own.CastingTime), //nolint:gosec // a spell's circle is 1 to 9
		Ritual: own.Ritual, Concentration: own.Concentration, CanRitual: st.canRitual, CanCastWithSlot: canSlot,
	}
	circles := map[int]bool{}
	if st.canRitual {
		circles[det.Spell.Level] = true
	}
	if canSlot {
		for _, sl := range slots {
			if int(sl.GetLevel()) >= det.Spell.Level && sl.GetTotal() > 0 {
				circles[int(sl.GetLevel())] = true
			}
		}
	}
	levels := make([]int, 0, len(circles))
	for c := range circles {
		levels = append(levels, c)
	}
	slices.Sort(levels)
	for _, c := range levels {
		choices, err := content.SummonOptions(key, c, b)
		if err != nil {
			continue // a pact slot above the spell's reach, or a circle the spell does not take
		}
		sc := &charactersv1.SummonCircle{Circle: int32(c)} //nolint:gosec // 1 to 9
		for _, o := range choices.Options {
			opt := &charactersv1.SummonOption{Count: int32(o.Count), Type: o.Type, MaxCr: o.MaxCR, Attack: creatureAttackNumber[o.Attack]} //nolint:gosec // a few dozen at most
			for _, f := range o.Forms {
				opt.Forms = append(opt.Forms, &charactersv1.SummonForm{MonsterKey: f.Key, NamePt: content.NamePT(f.Key), Attack: creatureAttackNumber[f.Attack]})
			}
			sc.Options = append(sc.Options, opt)
		}
		out.Circles = append(out.Circles, sc)
	}
	// What a casting sends away: a new familiar replaces the old one, and a new
	// concentration ends the creatures of the old one (SummonCreatures).
	if key == "spell:find-familiar" {
		rows, err := q.ListLiveFamiliars(ctx, charactersdb.ListLiveFamiliarsParams{CampaignID: campaignID, CharacterID: characterID})
		if err != nil {
			return nil, wrap("list the familiars", err)
		}
		for _, r := range rows {
			out.Replaces = append(out.Replaces, replaced(content, r.CharacterCreature))
		}
	}
	if own.Concentration {
		rows, err := q.ListLiveCreaturesOnConcentration(ctx, charactersdb.ListLiveCreaturesOnConcentrationParams{CampaignID: campaignID, CharacterID: characterID})
		if err != nil {
			return nil, wrap("list the creatures that depend on a concentration", err)
		}
		for _, r := range rows {
			out.Replaces = append(out.Replaces, replaced(content, r.CharacterCreature))
		}
	}
	return out, nil
}

func replaced(content *rules.Content, c charactersdb.CharacterCreature) *charactersv1.ReplacedCreature {
	return &charactersv1.ReplacedCreature{Id: c.ID, Name: c.Name, MonsterKey: c.MonsterKey, MonsterNamePt: content.NamePT(c.MonsterKey)}
}

// castingTimePT says a summoning spell's casting time in Portuguese: "1 hora",
// "10 minutos", "1 ação", "1 reação".
func castingTimePT(t rules.CastingTime) string {
	n := max(t.Amount, 1)
	word := ""
	switch t.Unit {
	case rules.CastHour:
		word = map[bool]string{true: "hora", false: "horas"}[n == 1]
	case rules.CastMinute:
		word = map[bool]string{true: "minuto", false: "minutos"}[n == 1]
	case rules.CastAction:
		word = map[bool]string{true: "ação", false: "ações"}[n == 1]
	case rules.CastBonusAction:
		word = map[bool]string{true: "ação bônus", false: "ações bônus"}[n == 1]
	case rules.CastReaction:
		word = map[bool]string{true: "reação", false: "reações"}[n == 1]
	default:
		return t.Raw
	}
	return fmt.Sprintf("%d %s", n, word)
}
