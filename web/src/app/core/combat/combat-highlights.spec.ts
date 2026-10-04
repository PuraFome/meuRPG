import { create } from '@bufbuild/protobuf';

import {
  CharacterHighlightsSchema,
  GetCombatHighlightsResponseSchema,
  HighlightCategorySchema,
  HighlightKind,
  HighlightWinnerSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { highlightRows, highlightTiles, highlightValue, ownNumbers } from './combat-highlights';

const nbsp = ' ';

function category(kind: HighlightKind, value: number, ...winners: [string, string][]) {
  return create(HighlightCategorySchema, {
    kind,
    value,
    winners: winners.map(([characterId, name]) => create(HighlightWinnerSchema, { characterId, name })),
  });
}

/** The artboards' combat: Toren 23 de dano, Brisa 24 recebido, Pensantus e Toren 2 golpes finais each. */
const response = create(GetCombatHighlightsResponseSchema, {
  categories: [
    category(HighlightKind.MOST_DAMAGE, 23, ['toren', 'Toren']),
    category(HighlightKind.TANK, 24, ['brisa', 'Brisa']),
    category(HighlightKind.FINAL_BLOW, 2, ['pens', 'Pensantus'], ['toren', 'Toren']),
  ],
  characters: [
    create(CharacterHighlightsSchema, { characterId: 'pens', name: 'Pensantus', damageDealt: 17, finalBlows: 2 }),
    create(CharacterHighlightsSchema, { characterId: 'toren', name: 'Toren', damageDealt: 23, damageTaken: 10, finalBlows: 2 }),
  ],
});

describe('combat highlights', () => {
  it('writes each number with its unit tied to it', () => {
    expect(highlightValue(HighlightKind.MOST_DAMAGE, 23)).toBe(`23${nbsp}de${nbsp}dano`);
    expect(highlightValue(HighlightKind.TANK, 24)).toBe(`24${nbsp}de${nbsp}dano`);
    expect(highlightValue(HighlightKind.MOST_HEALING, 9)).toBe(`9${nbsp}de${nbsp}cura`);
    expect(highlightValue(HighlightKind.FINAL_BLOW, 1)).toBe(`1${nbsp}inimigo`);
    expect(highlightValue(HighlightKind.FINAL_BLOW, 2)).toBe(`2${nbsp}inimigos`);
    expect(highlightValue(HighlightKind.CRITICAL_HITS, 1)).toBe(`1${nbsp}crítico`);
    expect(highlightValue(HighlightKind.CRITICAL_HITS, 3)).toBe(`3${nbsp}críticos`);
    expect(highlightValue(HighlightKind.TREASURE_FOUND, 120)).toBe(`120${nbsp}PO`);
  });

  it('makes one tile per category the server sent, in its order, and none for the ones it left out', () => {
    const tiles = highlightTiles(response);
    expect(tiles.map((t) => t.label)).toEqual(['Mais dano causado', 'Tanque', 'Golpe final']);
    expect(tiles.map((t) => t.value)).toEqual([`23${nbsp}de${nbsp}dano`, `24${nbsp}de${nbsp}dano`, `2${nbsp}inimigos`]);
    expect(tiles[1].sub).toBe('mais dano recebido');
    expect(tiles[0].sub).toBe('');
    expect(tiles.some((t) => t.kind === HighlightKind.MOST_HEALING)).toBe(false);
  });

  it('names everyone in a tie', () => {
    const [damage, , finalBlow] = highlightTiles(response);
    expect(damage.names).toBe('Toren');
    expect(damage.tie).toBe(false);
    expect(finalBlow.names).toBe('Pensantus e Toren');
    expect(finalBlow.tie).toBe(true);
    const three = highlightTiles(
      create(GetCombatHighlightsResponseSchema, {
        categories: [category(HighlightKind.MOST_HEALING, 9, ['a', 'Brisa'], ['b', 'Toren'], ['c', 'Pensantus'])],
      }),
    );
    expect(three[0].names).toBe('Brisa, Toren e Pensantus');
  });

  it('has no tiles for a combat nobody hurt or healed in', () => {
    expect(highlightTiles(create(GetCombatHighlightsResponseSchema, {}))).toEqual([]);
  });

  it('keeps every row of the master\'s table, zeros included', () => {
    expect(highlightRows(response.characters)).toEqual([
      { characterId: 'pens', name: 'Pensantus', damageDealt: 17, healingDone: 0, damageTaken: 0, finalBlows: 2, criticalHits: 0 },
      { characterId: 'toren', name: 'Toren', damageDealt: 23, healingDone: 0, damageTaken: 10, finalBlows: 2, criticalHits: 0 },
    ]);
  });

  it('gives a player the numbers of their own row, zeros included, in the order of the card', () => {
    const own = [create(CharacterHighlightsSchema, { characterId: 'pens', name: 'Pensantus', damageDealt: 17, finalBlows: 2 })];
    expect(ownNumbers(own, 'pens')).toEqual([
      { label: 'Dano causado', value: '17' },
      { label: 'Dano recebido', value: '0' },
      { label: 'Golpes finais', value: '2' },
      { label: 'Cura', value: '0' },
    ]);
    expect(ownNumbers([], 'pens')).toEqual([]);
  });
});
