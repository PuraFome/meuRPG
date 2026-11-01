package characters

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
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

// resolve is resolveForOwner for the owner, whose environment is worked out once.
func (l *lazyOwner) resolve(ctx context.Context, key string) (stat []byte, hp int, err error) {
	if l.env == nil {
		if l.env, err = l.s.ownerEnv(ctx, l.q, l.content, l.campaignID, l.characterID); err != nil {
			return nil, 0, err
		}
	}
	return resolveForOwner(l.content, l.env, key)
}

// resolveForOwner works out the snapshot of the table creature key for the character (the slot is 0: the
// creature was not made by a slot). It returns the JSON to store and the creature's hit points.
func resolveForOwner(content *rules.Content, env *formula.Env, key string) (stat []byte, hp int, err error) {
	r, ok := content.ResolveCreature(key, env, 0)
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
// SRD creature costs no query.
func (s *Service) statBlockOf(ctx context.Context, tx pgx.Tx, content *rules.Content, campaignID, creatureID, monsterKey string) ([]byte, error) {
	if creatureID == "" || !content.IsTableCreature(monsterKey) {
		return nil, nil
	}
	row, err := s.queriesIn(tx).GetCharacterCreature(ctx, charactersdb.GetCharacterCreatureParams{CampaignID: campaignID, ID: creatureID})
	if err != nil {
		return nil, s.dbError(ctx, "read a creature's numbers", err)
	}
	return row.CharacterCreature.StatBlock, nil
}
