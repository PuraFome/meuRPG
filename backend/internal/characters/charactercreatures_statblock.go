package characters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/formula"
)

// The stat block snapshot of a creature. A table creature whose entry has formulas
// (rules.Content.CreatureHasFormulas) gets its numbers worked out for its owner when it is created, and
// they are kept in character_creatures.stat_block as the JSON of a rules.ResolvedCreature. Like hp_max, the
// snapshot does not follow the owner's later levels or an edit of the entry's formulas. The text of the
// creature (its name, traits and attack names) is always read from the content. A creature with no snapshot
// (an SRD creature, a table creature with no formula) reads its stat block from the content.

// ownerEnv is the formula environment of the character at the moment: its sheet derived with the
// campaign's content and its active states, and whether the shield it carries counts.
func (s *Service) ownerEnv(ctx context.Context, q *charactersdb.Queries, content *rules.Content, campaignID, characterID string) (*formula.Env, error) {
	rows, err := q.ListCombatCharacters(ctx, charactersdb.ListCombatCharactersParams{CampaignID: campaignID, Ids: []string{characterID}})
	if err != nil {
		return nil, s.dbError(ctx, "read a character for a creature", err)
	}
	if len(rows) == 0 || rows[0].Kind != kindPlayer {
		return nil, errCharacterNotFound()
	}
	sheet, err := loadSheet(rows[0].ID, rows[0].Sheet)
	if err != nil {
		return nil, s.dbError(ctx, "read a character for a creature", err)
	}
	if sheet.GetFull() == nil {
		return nil, errCharacterNotFound()
	}
	b := buildWithStates(sheet.GetFull(), stateKeysOf(rows[0].ActiveStates))
	d := rules.Derive(b, content)
	return rules.OwnerEnv(d, b.Shield && !d.ShieldIdle), nil
}

// lazyOwner is the formula environment of the character that is given creatures, worked out on the first
// creature with formulas and kept for the rest.
type lazyOwner struct {
	s                       *Service
	q                       *charactersdb.Queries
	content                 *rules.Content
	campaignID, characterID string
	env                     *formula.Env
}

// numbersOf is the snapshot and the hit points of a new creature of the content's key made with a slot of the
// circle (0 for none): the snapshot is nil for a creature with no formula, whose numbers are the content's.
func (l *lazyOwner) numbersOf(ctx context.Context, content *rules.Content, key string, slot int) (stat []byte, hp int32, err error) {
	d, ok := content.MonsterDerived(key)
	if !ok {
		return nil, 0, fmt.Errorf("%q is not a creature", key) // CheckSummon said it was
	}
	hpMax := d.HitPointsMax
	if content.CreatureHasFormulas(key) {
		if stat, hpMax, err = l.resolve(ctx, key, slot); err != nil {
			return nil, 0, err
		}
	}
	return stat, conv.Int32(max(hpMax, 1)), nil
}

// resolve is resolveForOwner for the owner, whose environment is worked out once.
func (l *lazyOwner) resolve(ctx context.Context, key string, slot int) (stat []byte, hp int, err error) {
	if l.env == nil {
		if l.env, err = l.s.ownerEnv(ctx, l.q, l.content, l.campaignID, l.characterID); err != nil {
			return nil, 0, err
		}
	}
	return resolveForOwner(l.content, l.env, key, slot)
}

// resolveForOwner works out the snapshot of the table creature key for the character and the circle of the
// slot it was made with (0 when it was not made by a slot). It returns the JSON to store and the creature's
// hit points.
func resolveForOwner(content *rules.Content, env *formula.Env, key string, slot int) (stat []byte, hp int, err error) {
	r, ok := content.ResolveCreature(key, env, slot)
	if !ok {
		return nil, 0, wrap("resolve a creature", fmt.Errorf("%q is not a table creature with formulas", key))
	}
	d, _, ok := content.ResolvedDerived(r)
	if !ok {
		return nil, 0, wrap("resolve a creature", fmt.Errorf("%q is not a table creature with formulas", key))
	}
	stat, err = json.Marshal(r)
	if err != nil {
		return nil, 0, wrap("keep a creature's numbers", err)
	}
	return stat, d.HitPointsMax, nil
}

// monsterDerivedOf is the stat block of a creature as a combat and a list read it: the snapshot's when the
// row has one that still fits the content's entry, the content's own otherwise.
func monsterDerivedOf(content *rules.Content, key string, statBlock []byte) (rules.Derived, bool) {
	if len(statBlock) > 0 {
		var r rules.ResolvedCreature
		if json.Unmarshal(statBlock, &r) == nil && r.Key == key {
			if d, _, ok := content.ResolvedDerived(r); ok {
				return d, true
			}
		}
	}
	return content.MonsterDerived(key)
}

// statBlockOf is the snapshot of the creature with the ID, or nil. A snapshot is used whenever the row has
// one, whatever the entry's formulas are now, as the list does; only a table creature can have one, so an
// SRD creature costs no query. The row of a table creature is read whatever its entry says now (an edit can
// drop the formulas of a creature that has a snapshot); a row that is not there (a creature deleted under its
// combatant) is no error: the entry's static numbers stand.
func (s *Service) statBlockOf(ctx context.Context, tx pgx.Tx, content *rules.Content, campaignID, creatureID, monsterKey string) ([]byte, error) {
	if creatureID == "" || !content.IsTableCreature(monsterKey) {
		return nil, nil
	}
	row, err := s.queriesIn(tx).GetCharacterCreature(ctx, charactersdb.GetCharacterCreatureParams{CampaignID: campaignID, ID: creatureID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, s.dbError(ctx, "read a creature's numbers", err)
	}
	return row.CharacterCreature.StatBlock, nil
}
