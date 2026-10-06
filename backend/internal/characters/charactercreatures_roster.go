package characters

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	maplink "github.com/PuraFome/meuRPG/backend/internal/maps/link"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// What the play module reads and writes of a character's creatures (MR-037,
// Etapa 9), through play.CombatRoster: recording a casting, checking what a
// spell may summon, joining a combat, concentration ending, the combat writing
// hit points back, and the numbers a creature's turn is worked out from. The
// methods that write take the caller's transaction and no caller: they run
// after play's own authorization check. The rules do every number; this file
// reads and copies them.

// creatureRows turns the rows of a list into creature views.
func creatureViews[T any](rows []T, pick func(T) (charactersdb.CharacterCreature, *string)) []creatureView {
	out := make([]creatureView, 0, len(rows))
	for _, r := range rows {
		c, owner := pick(r)
		out = append(out, creatureView{CharacterCreature: c, ownerUserID: owner})
	}
	return out
}

// CharacterCreatures implements play.CombatRoster: the live creatures of the
// given characters, oldest first, inside tx (a combat's start reads them in
// its own transaction).
func (s *Service) CharacterCreatures(ctx context.Context, tx pgx.Tx, campaignID string, characterIDs []string) ([]link.Creature, error) {
	if len(characterIDs) == 0 {
		return nil, nil
	}
	rows, err := s.queriesIn(tx).ListLiveCreaturesOfCharacters(ctx, charactersdb.ListLiveCreaturesOfCharactersParams{CampaignID: campaignID, CharacterIds: characterIDs})
	if err != nil {
		return nil, wrap("list the creatures", err)
	}
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return nil, wrap("read rules content", err)
	}
	var out []link.Creature
	for _, v := range creatureViews(rows, func(r charactersdb.ListLiveCreaturesOfCharactersRow) (charactersdb.CharacterCreature, *string) {
		return r.CharacterCreature, r.PlayerUserID
	}) {
		out = append(out, creatureOf(content, v))
	}
	return out, nil
}

// ConcentrationCreatures implements play.CombatRoster: the live creatures of a
// character that last only while it concentrates.
func (s *Service) ConcentrationCreatures(ctx context.Context, tx pgx.Tx, campaignID, characterID string) ([]link.Creature, error) {
	rows, err := s.queries.WithTx(tx).ListLiveCreaturesOnConcentration(ctx, charactersdb.ListLiveCreaturesOnConcentrationParams{CampaignID: campaignID, CharacterID: characterID})
	if err != nil {
		return nil, wrap("list the creatures that depend on a concentration", err)
	}
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return nil, wrap("read rules content", err)
	}
	var out []link.Creature
	for _, v := range creatureViews(rows, func(r charactersdb.ListLiveCreaturesOnConcentrationRow) (charactersdb.CharacterCreature, *string) {
		return r.CharacterCreature, r.PlayerUserID
	}) {
		out = append(out, creatureOf(content, v))
	}
	return out, nil
}

// SummonCreatures implements play.CombatRoster: it records a casting. A new
// familiar replaces the old one (the SRD allows one at a time), which is
// dismissed with the reason "replaced". The creatures start at full hit points.
// Blocked with CREATURE_LIMIT when the character would keep more than 40.
func (s *Service) SummonCreatures(ctx context.Context, tx pgx.Tx, sm link.Summon) (link.SummonResult, error) {
	q := s.queries.WithTx(tx)
	var res link.SummonResult
	content, err := s.contentFor(ctx, tx, sm.CampaignID)
	if err != nil {
		return res, wrap("read rules content", err)
	}
	if sm.Source == creatureSourceFamiliar {
		old, err := q.ListLiveFamiliars(ctx, charactersdb.ListLiveFamiliarsParams{CampaignID: sm.CampaignID, CharacterID: sm.CharacterID})
		if err != nil {
			return res, wrap("list the familiars", err)
		}
		if len(old) > 0 {
			ids := make([]string, 0, len(old))
			for _, o := range old {
				ids = append(ids, o.CharacterCreature.ID)
				res.Replaced = append(res.Replaced, creatureOf(content, creatureView{CharacterCreature: o.CharacterCreature, ownerUserID: o.PlayerUserID}))
			}
			if _, err := q.DismissCreatures(ctx, charactersdb.DismissCreaturesParams{CampaignID: sm.CampaignID, Ids: ids, Reason: "replaced", At: s.now()}); err != nil {
				return res, wrap("dismiss the old familiar", err)
			}
		}
	}
	if sm.Concentration {
		// A new concentration spell ends the old one, and the creatures that lasted
		// only while it did (RN-22), in or out of a combat.
		old, err := q.ListLiveCreaturesOnConcentration(ctx, charactersdb.ListLiveCreaturesOnConcentrationParams{CampaignID: sm.CampaignID, CharacterID: sm.CharacterID})
		if err != nil {
			return res, wrap("list the creatures that depend on a concentration", err)
		}
		if len(old) > 0 {
			ids := make([]string, 0, len(old))
			for _, o := range old {
				ids = append(ids, o.CharacterCreature.ID)
				res.Replaced = append(res.Replaced, creatureOf(content, creatureView{CharacterCreature: o.CharacterCreature, ownerUserID: o.PlayerUserID}))
			}
			if _, err := q.DismissCreatures(ctx, charactersdb.DismissCreaturesParams{CampaignID: sm.CampaignID, Ids: ids, Reason: "concentration", At: s.now()}); err != nil {
				return res, wrap("dismiss the old concentration's creatures", err)
			}
		}
	}
	live, err := q.CountLiveCreaturesOfCharacter(ctx, charactersdb.CountLiveCreaturesOfCharacterParams{CampaignID: sm.CampaignID, CharacterID: sm.CharacterID})
	if err != nil {
		return res, wrap("count the creatures", err)
	}
	if int(live)+len(sm.Creatures) > maxCreaturesPerCharacter {
		return res, errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CREATURE_LIMIT, sm.CharacterID)
	}
	labels := creatureNames(content, sm.Creatures)
	var cast *string
	if sm.Concentration {
		cast = &sm.GroupID
	}
	at := s.now()
	for i, spec := range sm.Creatures {
		d, ok := content.MonsterDerived(spec.MonsterKey)
		if !ok {
			return res, fmt.Errorf("%q is not a creature", spec.MonsterKey) // CheckSummon said it was
		}
		hp := int32(max(d.HitPointsMax, 1)) //nolint:gosec // an SRD creature has at most 676 hit points
		row, err := q.InsertCharacterCreature(ctx, charactersdb.InsertCharacterCreatureParams{
			CampaignID: sm.CampaignID, CharacterID: sm.CharacterID, MonsterKey: spec.MonsterKey, Name: labels[i], Source: sm.Source,
			Attack: spec.Attack, SummonGroupID: sm.GroupID, ConcentrationCastID: cast, HpCurrent: hp, HpMax: hp,
			// Each a microsecond after the last: the casting's order is the list's order, so "the
			// first creature of the group" is the same wherever it is read.
			CreatedAt: at.Add(time.Duration(i) * time.Microsecond),
		})
		if err != nil {
			return res, wrap("insert a creature", err)
		}
		res.Created = append(res.Created, creatureOf(content, creatureView{CharacterCreature: row}))
	}
	owner, err := q.GetCharacterCreature(ctx, charactersdb.GetCharacterCreatureParams{CampaignID: sm.CampaignID, ID: res.Created[0].ID})
	if err != nil {
		return res, wrap("read the owner", err)
	}
	for i := range res.Created {
		res.Created[i].OwnerUserID = deref(owner.PlayerUserID)
	}
	return res, nil
}

// creatureNames gives each creature of a casting its name: the one the player
// chose, or the creature's Portuguese name, numbered when several of the
// casting share it ("Lobo atroz 1", "Lobo atroz 2").
func creatureNames(c *rules.Content, specs []link.CreatureSpec) []string {
	total := map[string]int{}
	for _, sp := range specs {
		if sp.Name == "" {
			total[sp.MonsterKey]++
		}
	}
	seen := map[string]int{}
	out := make([]string, len(specs))
	for i, sp := range specs {
		if sp.Name != "" {
			out[i] = sp.Name
			continue
		}
		base := c.NamePT(sp.MonsterKey)
		if base == "" {
			base = sp.MonsterKey
		}
		if total[sp.MonsterKey] > 1 {
			seen[sp.MonsterKey]++
			suffix := " " + strconv.Itoa(seen[sp.MonsterKey])
			if r := []rune(base); len(r) > maxCreatureName-len(suffix) {
				base = string(r[:maxCreatureName-len(suffix)])
			}
			base += suffix
		}
		if n, err := names.Clean(base, maxCreatureName); err == nil {
			base = n
		}
		out[i] = base
	}
	return out
}

// DismissCreatures implements play.CombatRoster: the live ones of these
// creatures leave their owners with the reason ("concentration", "defeated",
// "replaced"...); it returns the IDs it dismissed.
func (s *Service) DismissCreatures(ctx context.Context, tx pgx.Tx, campaignID string, ids []string, reason string, at time.Time) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	out, err := s.queries.WithTx(tx).DismissCreatures(ctx, charactersdb.DismissCreaturesParams{CampaignID: campaignID, Ids: ids, Reason: reason, At: at})
	if err != nil {
		return nil, wrap("dismiss creatures", err)
	}
	return out, nil
}

// ReviveCreatures implements play.CombatRoster: the creatures dismissed for the
// reason are back (an undo); it returns the IDs it revived.
func (s *Service) ReviveCreatures(ctx context.Context, tx pgx.Tx, campaignID string, ids []string, reason string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	out, err := s.queries.WithTx(tx).ReviveCreatures(ctx, charactersdb.ReviveCreaturesParams{CampaignID: campaignID, Ids: ids, Reason: reason})
	if err != nil {
		return nil, wrap("revive creatures", err)
	}
	return out, nil
}

// DeleteCreatures implements play.CombatRoster: the creatures a casting made
// are gone for good (the master's undo of that casting).
func (s *Service) DeleteCreatures(ctx context.Context, tx pgx.Tx, campaignID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := s.queries.WithTx(tx).DeleteCreatures(ctx, charactersdb.DeleteCreaturesParams{CampaignID: campaignID, Ids: ids}); err != nil {
		return wrap("delete creatures", err)
	}
	return nil
}

// SyncCreatures implements play.CombatRoster: it keeps the creatures in step
// with the combat that holds them. A creature whose combatant is defeated (0
// hit points) is dismissed with the reason "defeated" and one that was
// dismissed as defeated and is up again (a heal, an undo) is back, each with
// the hit points of its combatant. It returns who changed.
func (s *Service) SyncCreatures(ctx context.Context, tx pgx.Tx, campaignID string, states []link.CreatureState, at time.Time) (link.CreatureChanges, error) {
	var out link.CreatureChanges
	if len(states) == 0 {
		return out, nil
	}
	q := s.queries.WithTx(tx)
	ids := make([]string, 0, len(states))
	for _, st := range states {
		ids = append(ids, st.ID)
	}
	rows, err := q.ListCreaturesByIDs(ctx, charactersdb.ListCreaturesByIDsParams{CampaignID: campaignID, Ids: ids})
	if err != nil {
		return out, wrap("read the creatures of a combat", err)
	}
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return out, wrap("read rules content", err)
	}
	byID := make(map[string]creatureView, len(rows))
	for _, r := range rows {
		byID[r.CharacterCreature.ID] = creatureView{CharacterCreature: r.CharacterCreature, ownerUserID: r.PlayerUserID}
	}
	for _, st := range states {
		v, ok := byID[st.ID]
		if !ok {
			continue
		}
		hp := int32(min(max(st.HP, 0), int(v.HpMax))) //nolint:gosec // clamped to the creature's maximum
		switch {
		case st.Defeated && v.live():
			if _, err := q.DismissCreatures(ctx, charactersdb.DismissCreaturesParams{CampaignID: campaignID, Ids: []string{v.ID}, Reason: "defeated", At: at}); err != nil {
				return out, wrap("dismiss a defeated creature", err)
			}
			v.HpCurrent = 0
			out.Dismissed = append(out.Dismissed, creatureOf(content, v))
		case !st.Defeated && !v.live() && v.DismissedReason != nil && *v.DismissedReason == "defeated":
			if _, err := q.ReviveCreatures(ctx, charactersdb.ReviveCreaturesParams{CampaignID: campaignID, Ids: []string{v.ID}, Reason: "defeated"}); err != nil {
				return out, wrap("revive a creature", err)
			}
			if err := q.SetCharacterCreatureHitPoints(ctx, charactersdb.SetCharacterCreatureHitPointsParams{CampaignID: campaignID, ID: v.ID, HpCurrent: hp}); err != nil {
				return out, wrap("restore a creature's hit points", err)
			}
			v.HpCurrent = hp
			out.Revived = append(out.Revived, creatureOf(content, v))
		}
	}
	return out, nil
}

// WriteBackCreatures implements play.CombatRoster: a combat's end (or a
// creature leaving it) writes each creature's hit points back. A creature at 0
// is dismissed ("defeated") if it is not already.
func (s *Service) WriteBackCreatures(ctx context.Context, tx pgx.Tx, campaignID string, states []link.CreatureState, at time.Time) error {
	if len(states) == 0 {
		return nil
	}
	q := s.queries.WithTx(tx)
	for _, st := range states {
		id, ok := parseUUID(st.ID)
		if !ok {
			continue
		}
		row, err := q.GetCharacterCreature(ctx, charactersdb.GetCharacterCreatureParams{CampaignID: campaignID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			continue // dismissed for good meanwhile
		}
		if err != nil {
			return wrap("read a creature", err)
		}
		hp := int32(min(max(st.HP, 0), int(row.CharacterCreature.HpMax))) //nolint:gosec // clamped to the creature's maximum
		if st.Defeated {
			if _, err := q.DismissCreatures(ctx, charactersdb.DismissCreaturesParams{CampaignID: campaignID, Ids: []string{id}, Reason: "defeated", At: at}); err != nil {
				return wrap("dismiss a defeated creature", err)
			}
			continue
		}
		if err := q.SetCharacterCreatureHitPoints(ctx, charactersdb.SetCharacterCreatureHitPointsParams{CampaignID: campaignID, ID: id, HpCurrent: hp}); err != nil {
			return wrap("write a creature's hit points back", err)
		}
	}
	return nil
}

// summonCharacter reads the living player's character that casts and its
// build; `not_found` for any other.
func (s *Service) summonCharacter(ctx context.Context, tx pgx.Tx, campaignID, characterID string) (rules.Build, error) {
	id, ok := parseUUID(characterID)
	if !ok {
		return rules.Build{}, errCharacterNotFound()
	}
	rows, err := s.queriesIn(tx).ListCombatCharacters(ctx, charactersdb.ListCombatCharactersParams{CampaignID: campaignID, Ids: []string{id}})
	if err != nil {
		return rules.Build{}, s.dbError(ctx, "read a character for a summon", err)
	}
	if len(rows) == 0 || rows[0].Kind != kindPlayer {
		return rules.Build{}, errCharacterNotFound()
	}
	sheet, err := loadSheet(rows[0].ID, rows[0].Sheet)
	if err != nil {
		return rules.Build{}, s.dbError(ctx, "read a character for a summon", err)
	}
	if sheet.GetFull() == nil {
		return rules.Build{}, errCharacterNotFound()
	}
	return buildOf(sheet.GetFull()), nil
}

// CheckSummon implements play.CombatRoster: what a casting of the spell by the
// character may do. circle is the slot's circle, 0 for the spell's own (a
// ritual). It returns what the spell and the sheet say (the spell is on the
// sheet, can be cast as a ritual...) and the creatures of the choice. A choice
// the spell does not allow comes back as one of rules.ErrSummonOption,
// ErrSummonCount, ErrSummonCreature or ErrSummonCircle, a spell that does not
// summon as rules.ErrNotSummonSpell, for play to turn into its own reasons.
func (s *Service) CheckSummon(ctx context.Context, tx pgx.Tx, campaignID, characterID, spellKey string, circle, option int, keys []string) (link.SummonSpell, error) {
	b, err := s.summonCharacter(ctx, tx, campaignID, characterID)
	if err != nil {
		return link.SummonSpell{}, err
	}
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return link.SummonSpell{}, wrap("read rules content", err)
	}
	det, ok := content.SpellDetails(spellKey)
	if !ok {
		return link.SummonSpell{}, connect.NewError(connect.CodeNotFound, errUnknownSpell)
	}
	if circle == 0 {
		circle = det.Spell.Level
	}
	choices, err := content.SummonOptions(spellKey, circle, b)
	if err != nil {
		return link.SummonSpell{}, err
	}
	out := link.SummonSpell{
		Ritual: choices.Ritual, Concentration: choices.Concentration, CastingUnit: choices.CastingTime.Unit, Level: det.Spell.Level,
	}
	st := s.summonStanding(rules.Derive(b, content), spellKey, choices.Ritual)
	out.Known, out.Prepared, out.CanRitual = st.known, st.prepared, st.canRitual
	made, err := content.CheckSummon(spellKey, circle, b, rules.SummonPick{Option: option, Creatures: keys})
	if err != nil {
		return out, err
	}
	for _, m := range made {
		out.Creatures = append(out.Creatures, link.SummonedForm{MonsterKey: m.Key, Attack: m.Attack})
	}
	return out, nil
}

// summonStanding is what a character's sheet says about a summoning spell: the
// spell is on it (known), ready (prepared) and castable as a ritual. CheckSummon
// and GetSummonOptions share it, so the read and the cast agree.
type summonStanding struct{ known, prepared, canRitual bool }

// summonStanding works it out from the derived sheet; ritualSpell is whether the
// spell is a ritual at all.
func (s *Service) summonStanding(d rules.Derived, spellKey string, ritualSpell bool) summonStanding {
	var out summonStanding
	for _, cs := range d.Spells {
		if cs.Spell.Key != spellKey {
			continue
		}
		out.known, out.prepared = true, cs.Prepared
		// A ritual is cast from the spellbook (a wizard) or from the prepared
		// list (the other classes that cast rituals).
		for _, sc := range d.Spellcasting {
			if sc.Ritual && ritualSpell && (cs.Prepared || sc.Class == "class:wizard") {
				out.canRitual = true
			}
		}
	}
	// The Pact of the Chain lets a warlock cast Encontrar Familiar as a ritual
	// without having the spell (SRD 5.1): it is never prepared for a slot.
	if spellKey == "spell:find-familiar" && ritualSpell && slices.ContainsFunc(d.Features, func(f rules.Feature) bool { return f.Key == "feature:pact-of-the-chain" }) {
		out.known, out.canRitual = true, true
	}
	return out
}

// creatureDerived is the stat block of a creature as the combat reads it: the
// SRD creature's numbers, and what the spell lets it attack with. A creature
// that may not attack (a familiar) has no attacks and no Attack action; one that
// attacks only with its reaction keeps its attacks for the opportunity attack.
// No creature casts a spell here.
func creatureDerived(content *rules.Content, key, attack string) (rules.Derived, bool) {
	d, ok := content.MonsterDerived(key)
	if !ok {
		return rules.Derived{}, false
	}
	d.StandardActions = slices.DeleteFunc(slices.Clone(d.StandardActions), func(a rules.Action) bool {
		return a.Key == "standard:cast-a-spell" || (attack != "full" && a.Key == "standard:attack")
	})
	if attack == "none" {
		d.Attacks = nil
	}
	return d, true
}

// CreatureSheet implements play.CombatRoster: what an attack needs from a
// creature's stat block (its armor class, its attacks, the standard actions).
// False for a key that is not an SRD creature.
func (s *Service) CreatureSheet(ctx context.Context, tx pgx.Tx, campaignID, monsterKey, attack string) (link.Sheet, bool, error) {
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return link.Sheet{}, false, wrap("read rules content", err)
	}
	d, ok := creatureDerived(content, monsterKey, attack)
	if !ok {
		return link.Sheet{}, false, nil
	}
	out := link.Sheet{ArmorClass: d.ArmorClass, AttacksPerAction: max(d.AttacksPerAction, 1), Senses: senseRanges(d.Senses)}
	for _, a := range d.Attacks {
		name := a.NamePT
		if name == "" {
			name = a.Name
		}
		out.Attacks = append(out.Attacks, link.Attack{
			Key: a.Key, Name: name, Spell: a.Kind == "spell", ToHit: a.AttackBonus,
			DiceCount: a.DamageDice.Count, DiceSides: a.DamageDice.Sides, DiceBonus: a.DamageDice.Bonus,
			DamageType: a.DamageType, RangeFt: a.RangeFt, LongRangeFt: a.LongRangeFt, Melee: a.Melee,
		})
	}
	for _, a := range d.StandardActions {
		out.Actions = append(out.Actions, link.Action{Key: a.Key, Name: a.NamePT})
	}
	return out, true, nil
}

// CreatureTurnOptions implements play.CombatRoster: what the creature can do
// now, from its stat block and what it used this turn. A creature that attacks
// only with its reaction shows its attacks disabled (REACTION_ONLY): the app
// offers them when a move leaves its reach. False for a key that is not an SRD
// creature.
func (s *Service) CreatureTurnOptions(ctx context.Context, tx pgx.Tx, campaignID, monsterKey, attack string, turn link.Turn) (*rulesv1.TurnOptions, bool, error) {
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return nil, false, wrap("read rules content", err)
	}
	d, ok := creatureDerived(content, monsterKey, attack)
	if !ok {
		return nil, false, nil
	}
	d.SpeedWalkFt = turn.SpeedFt
	opts := combat.Options(d, combat.TurnState{
		ActionUsed: turn.ActionUsed, BonusActionUsed: turn.BonusActionUsed, ReactionUsed: turn.ReactionUsed,
		MovementUsedFt: turn.MovementUsedFt, Dashed: turn.Dashed, AttacksMade: turn.AttacksMade,
	}, combat.Usage{})
	out := turnOptionsToProto(opts)
	if attack == "reaction" {
		for _, a := range out.Attacks {
			a.Enabled = false
			a.Reason = &rulesv1.DisabledReason{Code: rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_REACTION_ONLY}
		}
	}
	return out, true, nil
}

// CreatureSave implements play.CombatRoster: a creature's saving throw bonus
// against an ability ("dex"), from its stat block (the listed bonus, or the
// ability modifier).
func (s *Service) CreatureSave(ctx context.Context, tx pgx.Tx, campaignID, monsterKey, ability string) (link.Save, error) {
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return link.Save{}, wrap("read rules content", err)
	}
	d, ok := content.MonsterDerived(monsterKey)
	if !ok {
		return link.Save{}, nil
	}
	for _, st := range d.SavingThrows {
		if string(st.Ability) == ability {
			return link.Save{Bonus: st.Bonus, Known: true}, nil
		}
	}
	return link.Save{}, nil
}

// CreatureEyes implements play.CombatRoster: what a creature notices with, from
// its stat block: the passive Perception it lists and its senses (MR-035, MR-037).
// False for a key that is not an SRD creature.
func (s *Service) CreatureEyes(ctx context.Context, tx pgx.Tx, campaignID, monsterKey string) (maplink.Eyes, bool, error) {
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return maplink.Eyes{}, false, wrap("read rules content", err)
	}
	d, ok := content.MonsterDerived(monsterKey)
	if !ok {
		return maplink.Eyes{}, false, nil
	}
	return maplink.Eyes{Passive: d.PassivePerception, Senses: sensesOf(d.Senses)}, true, nil
}
