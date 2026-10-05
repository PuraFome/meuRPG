package characters

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"connectrpc.com/connect"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The SRD creatures (MR-037) are public rules, like the catalog and the spell
// details, so there is nothing to read from the database: these two methods
// check access and turn package rules' answers into messages.

const (
	defaultCreaturesPage = 50
	maxCreaturesPage     = 100
	maxCreatureQuery     = 100
)

var errUnknownCreature = errors.New("creature not found")

// ListCreatures implements rulesv1connect.ContentServiceHandler.
func (s *Service) ListCreatures(
	ctx context.Context,
	req *connect.Request[rulesv1.ListCreaturesRequest],
) (*connect.Response[rulesv1.ListCreaturesResponse], error) {
	// Active members only: a pending member needs no creature (RN-15).
	if _, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId()); err != nil {
		return nil, err
	}
	if utf8.RuneCountInString(req.Msg.GetQuery()) > maxCreatureQuery {
		return nil, invalidArgument(fieldErr("query", "must have at most %d characters", maxCreatureQuery))
	}
	size := req.Msg.GetPageSize()
	switch {
	case size == 0:
		size = defaultCreaturesPage
	case size < 0 || size > maxCreaturesPage:
		return nil, invalidArgument(fieldErr("page_size", "must be 1 to %d", maxCreaturesPage))
	}
	offset := 0
	if token := req.Msg.GetPageToken(); token != "" {
		var ok bool
		if offset, ok = parseCreaturesToken(token, creaturesFilterID(req.Msg)); !ok {
			return nil, invalidArgument(fieldErr("page_token", "is not a token of this list"))
		}
	}
	list, err := s.rules.ListCreatures(rules.CreatureFilter{
		Query: req.Msg.GetQuery(), Type: req.Msg.GetType(), MaxCR: req.Msg.GetMaxCr(),
		NoFly: req.Msg.GetNoFly(), NoSwim: req.Msg.GetNoSwim(),
	})
	if err != nil {
		return nil, invalidArgument(fieldErr("max_cr", "is not one of the SRD's challenge ratings"))
	}
	res := &rulesv1.ListCreaturesResponse{Total: i32(len(list))}
	if offset > len(list) {
		return nil, invalidArgument(fieldErr("page_token", "is not a token of this list"))
	}
	end := min(offset+int(size), len(list))
	for _, e := range list[offset:end] {
		res.Creatures = append(res.Creatures, creatureSummaryToProto(e))
	}
	if end < len(list) {
		res.NextPageToken = creaturesToken(end, creaturesFilterID(req.Msg))
	}
	return connect.NewResponse(res), nil
}

// GetCreature implements rulesv1connect.ContentServiceHandler.
func (s *Service) GetCreature(
	ctx context.Context,
	req *connect.Request[rulesv1.GetCreatureRequest],
) (*connect.Response[rulesv1.GetCreatureResponse], error) {
	if _, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId()); err != nil {
		return nil, err
	}
	c, ok := s.rules.CreatureByKey(req.Msg.GetKey())
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errUnknownCreature)
	}
	return connect.NewResponse(&rulesv1.GetCreatureResponse{Creature: creatureToProto(c)}), nil
}

// creaturesFilterID is a short fingerprint of the filters, so a page token only
// works for the list it came from.
func creaturesFilterID(r *rulesv1.ListCreaturesRequest) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		r.GetQuery(), r.GetType(), r.GetMaxCr(), strconv.FormatBool(r.GetNoFly()), strconv.FormatBool(r.GetNoSwim()),
	}, "\x00")))
	return hex.EncodeToString(sum[:6])
}

// The page token is the offset of the next row and the fingerprint of the
// filters, written as base64 text so a client treats it as opaque.
func creaturesToken(offset int, filters string) string {
	return base64.RawURLEncoding.EncodeToString([]byte("c" + strconv.Itoa(offset) + ":" + filters))
}

func parseCreaturesToken(token, filters string) (int, bool) {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, false
	}
	rest, found := strings.CutPrefix(string(b), "c")
	n, got, hasFilters := strings.Cut(rest, ":")
	offset, err := strconv.Atoi(n)
	return offset, found && hasFilters && got == filters && err == nil && offset >= 0
}

func creatureSummaryToProto(e rules.CreatureEntry) *rulesv1.CreatureSummary {
	return &rulesv1.CreatureSummary{
		Key: e.Key, Name: e.Name, NamePt: e.NamePT, Size: e.Size, SizePt: e.SizeNamePT,
		Type: e.Type, TypePt: e.TypeNamePT, Subtype: e.Subtype,
		ChallengeRating: e.ChallengeRating, Xp: i32(e.XP), CanFly: e.CanFly, CanSwim: e.CanSwim,
	}
}

func namedKeysToProto(list []rules.NamedKey) []*rulesv1.CreatureNamedKey {
	var out []*rulesv1.CreatureNamedKey
	for _, k := range list {
		out = append(out, &rulesv1.CreatureNamedKey{Key: k.Key, NamePt: k.NamePT})
	}
	return out
}

func damageModsToProto(list []rules.CreatureDamageMod) []*rulesv1.CreatureDamageModifier {
	var out []*rulesv1.CreatureDamageModifier
	for _, d := range list {
		out = append(out, &rulesv1.CreatureDamageModifier{Types: namedKeysToProto(d.Types), Note: d.Note})
	}
	return out
}

func traitsToProto(list []rules.CreatureAbility) []*rulesv1.CreatureTrait {
	var out []*rulesv1.CreatureTrait
	for _, a := range list {
		out = append(out, &rulesv1.CreatureTrait{Name: a.Name, Text: a.Text, Usage: a.Usage})
	}
	return out
}

func creatureToProto(c rules.Creature) *rulesv1.Creature {
	out := &rulesv1.Creature{
		Summary: creatureSummaryToProto(c.CreatureEntry), Alignment: c.Alignment,
		ArmorClass: i32(c.ArmorClass), ArmorClassLabelPt: c.ArmorClassNamePT, ArmorClassNote: c.ArmorClassNote,
		HitPoints: i32(c.HitPoints), HitDice: c.HitDice, HitPointsRoll: c.HitPointsRoll,
		SpeedWalkFt: i32(c.SpeedWalkFt), SpeedFlyFt: i32(c.SpeedFlyFt), SpeedSwimFt: i32(c.SpeedSwimFt),
		SpeedClimbFt: i32(c.SpeedClimbFt), SpeedBurrowFt: i32(c.SpeedBurrowFt), Hover: c.Hover,
		Vulnerabilities: damageModsToProto(c.Vulnerabilities), Resistances: damageModsToProto(c.Resistances),
		Immunities:          damageModsToProto(c.Immunities),
		ConditionImmunities: namedKeysToProto(c.ConditionImmunities),
		PassivePerception:   i32(c.PassivePerception), Languages: c.Languages, ProficiencyBonus: i32(c.ProficiencyBonus),
		Traits: traitsToProto(c.Traits), Reactions: traitsToProto(c.Reactions), LegendaryActions: traitsToProto(c.LegendaryActions),
	}
	for _, a := range c.Abilities {
		out.Abilities = append(out.Abilities, &rulesv1.CreatureAbilityScore{
			Ability: abilityToProto[a.Ability], NamePt: a.NamePT, Score: i32(a.Score), Modifier: i32(a.Modifier),
		})
	}
	for _, s := range c.Saves {
		out.SavingThrows = append(out.SavingThrows, &rulesv1.CreatureBonus{Ability: abilityToProto[s.Ability], NamePt: s.NamePT, Bonus: i32(s.Bonus)})
	}
	for _, s := range c.Skills {
		out.Skills = append(out.Skills, &rulesv1.CreatureBonus{Ability: abilityToProto[s.Ability], Key: s.Key, NamePt: s.NamePT, Bonus: i32(s.Bonus)})
	}
	for _, s := range c.Senses {
		out.Senses = append(out.Senses, &rulesv1.Sense{Key: s.Source, NamePt: s.NamePT, RangeFt: i32(s.RangeFt), Sense: s.Key})
	}
	for _, a := range c.Actions {
		act := &rulesv1.CreatureAction{Name: a.Name, NamePt: a.NamePT, Text: a.Text, Usage: a.Usage, HasAttack: a.HasAttack, AttackBonus: i32(a.AttackBonus)}
		for _, d := range a.Damage {
			act.Damage = append(act.Damage, &rulesv1.CreatureDamagePart{Dice: d.Dice, DamageTypeKey: d.TypeKey, DamageTypePt: d.TypeNamePT})
		}
		if a.Save != nil {
			act.Save = &rulesv1.CreatureSave{Ability: abilityToProto[a.Save.Ability], Dc: i32(a.Save.DC), OnSuccess: saveSuccessToProto[a.Save.OnSuccess]}
		}
		for _, routine := range a.Multiattack {
			r := &rulesv1.CreatureMultiattackRoutine{}
			for _, x := range routine {
				r.Attacks = append(r.Attacks, &rulesv1.CreatureAttackCount{Name: x.Name, Count: i32(x.Count), Kind: x.Kind, CountText: x.Text})
			}
			act.Multiattack = append(act.Multiattack, r)
		}
		out.Actions = append(out.Actions, act)
	}
	return out
}
