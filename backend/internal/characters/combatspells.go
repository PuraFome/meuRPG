package characters

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// What a cast reads from a spell and the caster's sheet (MR-014, Etapa 6): the
// numbers of the spell at the slot level, the caster's attack bonus and save
// DC, the target's saving throw bonus, and the death the master confirms. Like
// the rest of the combat's reads (combatturn.go), package play declares the
// interface (CombatRoster) and this Service implements it; the rules do every
// number and this file only copies them into the plain types of package link.

// CombatSpell implements play.CombatRoster: the spell as the caster casts it
// with a slot of slotLevel (0 for a cantrip). It does not say whether the
// caster may cast it: CombatTurnOptions does. `not_found` for a character that
// is not one of the campaign's living ones, or a spell the content does not
// have.
func (s *Service) CombatSpell(ctx context.Context, campaignID, characterID, spellKey string, slotLevel int) (link.Spell, error) {
	_, d, err := s.fighter(ctx, campaignID, characterID)
	if err != nil {
		return link.Spell{}, err
	}
	det, ok := s.rules.SpellDetails(spellKey)
	if !ok {
		return link.Spell{}, connect.NewError(connect.CodeNotFound, errUnknownSpell)
	}
	out := link.Spell{
		Key: det.Spell.Key, Name: det.Spell.NamePT, Level: det.Spell.Level, Concentration: det.Duration.Concentration,
		RangeKind: det.Range.Kind, RangeFt: det.Range.DistanceFt, AttackType: det.AttackType,
	}
	if out.Name == "" {
		out.Name = det.Spell.Name
	}
	switch det.CastingTime.Unit {
	case rules.CastAction:
		out.Economy = rules.EconomyAction
	case rules.CastBonusAction:
		out.Economy = rules.EconomyBonusAction
	case rules.CastReaction:
		out.Economy = rules.EconomyReaction
	}

	// The class that casts it: the first that has it on its list, or the first
	// class that casts at all (as the sheet's own attacks pick it).
	var mod int
	if sc := casterFor(d, det.Spell.Classes); sc != nil {
		mod = abilityMod(d, sc.Ability)
		if det.AttackType != "" {
			out.ToHit = sc.AttackBonus
		}
		out.SaveDC = sc.SaveDC
	}
	if det.Save != nil {
		out.SaveAbility, out.SaveOnSuccess = string(det.Save.Ability), det.Save.OnSuccess
	} else {
		out.SaveDC = 0
	}
	for _, roll := range det.DamageAt(slotLevel, d.TotalLevel) {
		// A damage without a type is no damage the engine rolls (Sono's pool of
		// hit points); one it cannot parse ("4d6 OR 5d6") is the master's.
		if roll.Parsed && roll.Type != "" && out.Damage == nil {
			out.Damage = &link.Dice{Count: roll.Dice.Count, Sides: roll.Dice.Sides, Bonus: roll.Dice.Bonus, DamageType: roll.Type}
		}
	}
	if heal, ok := det.HealAt(slotLevel); ok && heal.Parsed {
		h := &link.Dice{Count: heal.Dice.Count, Sides: heal.Dice.Sides, Bonus: heal.Dice.Bonus}
		if heal.Dice.AddsModifier {
			h.Bonus += mod
		}
		out.Heal = h
	}
	out.Area = isArea(det)
	out.ExtraTargetPerLevel = extraTargetRE.MatchString(strings.Join(det.HigherLevel, " "))
	return out, nil
}

// casterFor picks the Spellcasting of a class that has the spell on its list,
// or the first one; nil for a character that casts nothing.
func casterFor(d rules.Derived, classes []string) *rules.Spellcasting {
	for i := range d.Spellcasting {
		if slices.Contains(classes, d.Spellcasting[i].Class) {
			return &d.Spellcasting[i]
		}
	}
	if len(d.Spellcasting) > 0 {
		return &d.Spellcasting[0]
	}
	return nil
}

// abilityMod is the ability's modifier on the sheet.
func abilityMod(d rules.Derived, a rules.Ability) int {
	for _, as := range d.Abilities {
		if as.Ability == a {
			return as.Modifier
		}
	}
	return 0
}

// The SRD says in prose whether a spell hits an area and how many targets it
// takes; the engine reads the prose with two patterns, kept here so a wrong
// guess has one place to fix (the master is never held to the number of
// targets, so a wrong guess never blocks a table). An area is a shape ("20-foot
// radius", "15-foot cone", "100-foot-long line"), a point ("within 20 feet of a
// point") or "up to three creatures"; a spell that gets "one additional
// creature" at a higher level takes one more target for each level.
var (
	areaRE = regexp.MustCompile(`(?i)\b\d+-foot[- ](radius|cone|cube|line|square|sphere|cylinder|long|wide)|within \d+ feet of a point|\bup to (two|three|four|five|six|seven|eight|nine|ten|twelve) (other )?(creatures|humanoids|willing creatures)|\bcreatures of your choice`)
	// Magic Missile's darts and a spell attack's targets are not areas: they are
	// counted by the play module.
	extraTargetRE = regexp.MustCompile(`(?i)additional (creature|target|humanoid)|one additional`)
)

// isArea says whether a spell takes any number of targets: a spell attack
// takes one, a healing spell that is not mass or a prayer takes one, and a
// spell that comes out of the caster and damages (Mãos Flamejantes, Onda
// Trovejante) is an area; the rest is read from the SRD's text.
func isArea(det *rules.SpellDetails) bool {
	switch {
	case det.AttackType != "":
		return false
	case det.Spell.Key == "spell:magic-missile":
		return false
	case len(det.HealBySlotLevel) > 0:
		return strings.HasPrefix(det.Spell.Key, "spell:mass-") || det.Spell.Key == "spell:prayer-of-healing"
	case det.Range.Kind == rules.RangeSelf && (det.Save != nil || len(det.Damage) > 0):
		return true
	}
	return areaRE.MatchString(strings.Join(det.Description, " "))
}

// CombatSave implements play.CombatRoster: a creature's saving throw bonus
// against an ability. A full sheet has all six; a basic-sheet NPC has none, so
// the bonus is 0 and Known is false, and the log says so. `not_found` for a
// character that is not one of the campaign's living ones.
func (s *Service) CombatSave(ctx context.Context, campaignID, characterID, ability string) (link.Save, error) {
	_, d, err := s.fighter(ctx, campaignID, characterID)
	if err != nil {
		return link.Save{}, err
	}
	for _, st := range d.SavingThrows {
		if string(st.Ability) == ability {
			return link.Save{Bonus: st.Bonus, Known: true}, nil
		}
	}
	return link.Save{}, nil // a basic sheet: no saving throws derived
}

// MarkDead implements play.CombatRoster: the master's confirmation of a death
// in a combat (RN-03) is MarkCharacterDead's effect, inside the combat's
// transaction: the character changes status, never row, and its player may
// create another. It is idempotent, as MarkCharacterDead is.
func (s *Service) MarkDead(ctx context.Context, tx pgx.Tx, campaignID, characterID string, at time.Time) error {
	id, ok := parseUUID(characterID)
	if !ok {
		return errCharacterNotFound()
	}
	if _, err := s.queries.WithTx(tx).MarkCharacterDead(ctx, charactersdb.MarkCharacterDeadParams{CampaignID: campaignID, ID: id, Now: at}); err != nil {
		return wrap("mark dead", err)
	}
	return nil
}

// Conditions implements play.CombatRoster: the SRD's conditions the master may
// mark as labels (RN-22), with their Portuguese names.
func (s *Service) Conditions() []link.Named {
	var out []link.Named
	for _, c := range s.rules.Conditions() {
		out = append(out, link.Named{Key: c.Key, NamePT: c.NamePT})
	}
	return out
}

// NamePT implements play.CombatRoster: the Portuguese name of a content key,
// for a spell the combat shows (the one a character concentrates on, Escudo).
func (s *Service) NamePT(key string) string {
	return s.rules.NamePT(key)
}
