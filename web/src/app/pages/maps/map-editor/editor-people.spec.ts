import { describe, expect, it, vi } from 'vitest';

import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { mapMessage, mapToken } from '../../../core/maps/maps-testing';
import type { RosterEntry } from '../../../core/maps/roster-client';
import {
  cluePlayers,
  finderRows,
  loadRoster,
  notOnMap,
  occupantsOf,
  selectedPointOf,
  selectedTokenOf,
} from './editor-people';

const entry = (id: string, over: Partial<RosterEntry> = {}): RosterEntry =>
  ({
    id,
    name: `Nome ${id}`,
    kind: CharacterKind.PLAYER,
    playerUserId: `u-${id}`,
    classSummary: 'Mago 3',
    raceName: '',
    playerName: 'Vinicius',
    ...over,
  }) as RosterEntry;

describe('editor people', () => {
  const everyone = [
    entry('a'),
    entry('b', { kind: CharacterKind.UNSPECIFIED, playerUserId: '' }),
    entry('c', { playerName: null, classSummary: '' }),
  ];

  it('lists only the characters a player plays, for the clues and for who found a treasure', () => {
    expect(cluePlayers(everyone)).toEqual([
      { id: 'a', name: 'Nome a', playerName: 'Vinicius' },
      { id: 'c', name: 'Nome c', playerName: '' },
    ]);
    expect(finderRows(everyone)).toEqual([
      { id: 'a', name: 'Nome a', sub: 'Mago 3 · Vinicius' },
      { id: 'c', name: 'Nome c', sub: '' },
    ]);
  });

  it('offers the characters that are not on the map', () => {
    expect(notOnMap(everyone, [mapToken('a', 'A')]).map((c) => c.id)).toEqual(['b', 'c']);
  });

  it('puts each token on its square, and nobody on a map without a grid', () => {
    const map = mapMessage('m', 'M', { gridColumns: 10, gridRows: 10 });
    expect(
      occupantsOf(map, [
        mapToken('a', 'A', { xBp: 0, yBp: 9999 }),
        mapToken('b', 'B', { xBp: 10000, yBp: 5000 }),
      ]),
    ).toEqual([
      { col: 0, row: 9, name: 'A' },
      { col: 9, row: 5, name: 'B' },
    ]);
    expect(occupantsOf(mapMessage('m', 'M', { gridColumns: 0 }), [mapToken('a', 'A')])).toEqual([]);
    expect(occupantsOf(null, [mapToken('a', 'A')])).toEqual([]);
  });

  it('finds the chosen point or token, and nothing for the other kind', () => {
    const points = [mapMessage('p', 'P')] as never[];
    const tokens = [mapToken('a', 'A'), mapToken('c', 'Goblin', { creatureId: 'cr' })];
    expect(selectedPointOf(points, { kind: 'point', id: 'p' })).toBe(points[0]);
    expect(selectedPointOf(points, { kind: 'token', id: 'p' })).toBeNull();
    expect(selectedPointOf(points, null)).toBeNull();
    expect(selectedTokenOf(tokens, { kind: 'token', id: 'cr' })).toBe(tokens[1]);
    expect(selectedTokenOf(tokens, { kind: 'token', id: 'zz' })).toBeNull();
    expect(selectedTokenOf(tokens, { kind: 'point', id: 'a' })).toBeNull();
  });

  it('keeps what it had when the roster or the light names cannot be read', async () => {
    const sink = {
      everyone: { set: vi.fn() },
      rosterLoaded: { set: vi.fn() },
      lightNames: { set: vi.fn() },
    };
    loadRoster(
      { list: () => Promise.reject(new Error('x')) },
      { list: () => Promise.reject(new Error('x')) },
      'c1',
      sink as never,
    );
    await Promise.resolve();
    await Promise.resolve();
    expect(sink.everyone.set).not.toHaveBeenCalled();
    expect(sink.lightNames.set).not.toHaveBeenCalled();
  });
});
