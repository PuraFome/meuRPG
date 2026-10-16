import { CharacterKind } from '../../../gen/meurpg/characters/v1/characters_pb';
import type { CreatureSummary } from '../../../gen/meurpg/rules/v1/rules_pb';
import type { RosterEntry } from '../../core/maps/roster-client';
import {
  type BlockInput,
  PICK_MAX,
  blockedReason,
  copiesText,
  joinSpecs,
  startMonsters,
  withPicked,
} from './start-combat-plan';

const ready: BlockInput = {
  ready: true,
  adding: false,
  theatre: false,
  hasMap: true,
  hasGrid: true,
  noGridMap: false,
  nameOk: true,
  included: 2,
  npcTotal: 0,
  monsterTotal: 0,
  picked: 0,
  total: 2,
};

describe('blockedReason', () => {
  it('goes when everything is in place', () => {
    expect(blockedReason(ready)).toBeNull();
  });

  it('waits for the roster first', () => {
    expect(blockedReason({ ...ready, ready: false, hasMap: false })).toBe(
      'Carregando quem pode lutar.',
    );
  });

  it('asks for the map, then for its grid, unless the combat has no map', () => {
    expect(blockedReason({ ...ready, hasMap: false })).toBe(
      'Escolha o mapa atual da sessão primeiro.',
    );
    expect(blockedReason({ ...ready, hasGrid: false })).toBe('O mapa precisa de uma grade.');
    expect(blockedReason({ ...ready, noGridMap: true })).toBe('O mapa precisa de uma grade.');
    expect(blockedReason({ ...ready, theatre: true, hasMap: false, hasGrid: false })).toBeNull();
  });

  it('adding a combatant needs no map of its own, only the grid', () => {
    expect(blockedReason({ ...ready, adding: true, hasMap: false, npcTotal: 1 })).toBeNull();
    expect(blockedReason({ ...ready, adding: true, hasGrid: false, npcTotal: 1 })).toBe(
      'O mapa precisa de uma grade.',
    );
  });

  it('asks for a name and for who enters', () => {
    expect(blockedReason({ ...ready, nameOk: false })).toBe('Dê um nome ao combate.');
    expect(blockedReason({ ...ready, included: 0 })).toBe('Escolha quem do grupo entra.');
    expect(blockedReason({ ...ready, included: 0, monsterTotal: 2 })).toBeNull();
  });

  it('adding asks for NPCs or a picked monster, and never for a name', () => {
    const adding = { ...ready, adding: true, nameOk: false, included: 0 };
    expect(blockedReason(adding)).toBe('Escolha quantos NPCs entram.');
    expect(blockedReason({ ...adding, picked: 1 })).toBeNull();
  });

  it('refuses more than 40 combatants', () => {
    expect(blockedReason({ ...ready, total: 40 })).toBeNull();
    expect(blockedReason({ ...ready, total: 41 })).toBe('Um combate tem no máximo 40 combatentes.');
  });
});

describe('withPicked', () => {
  const bandit = { key: 'monster:bandit', namePt: 'Bandido' } as CreatureSummary;

  it('lists a new creature with one, and counts it up to the most one add takes', () => {
    let list = withPicked([], bandit);
    expect(list).toEqual([{ key: 'monster:bandit', namePt: 'Bandido', count: 1 }]);
    for (let i = 0; i < PICK_MAX + 3; i++) {
      list = withPicked(list, bandit);
    }
    expect(list[0].count).toBe(PICK_MAX);
  });
});

describe('copiesText', () => {
  it('numbers the copies', () => {
    expect(copiesText('Goblin', 1)).toBe('Goblin 1');
    expect(copiesText('Goblin', 2)).toBe('Goblin 1 e 2');
    expect(copiesText('Goblin', 4)).toBe('Goblin 1, 2, 3 e 4');
  });
});

describe('startMonsters', () => {
  it("sends the saved encounter's groups with their names, then the picked ones with none", () => {
    expect(
      startMonsters(
        [{ key: 'a', namePt: 'A', count: 2, name: 'Capanga' }],
        [{ key: 'b', namePt: 'B', count: 1 }],
      ),
    ).toEqual([
      { creatureKey: 'a', count: 2, name: 'Capanga' },
      { creatureKey: 'b', count: 1, name: '' },
    ]);
  });
});

describe('joinSpecs', () => {
  const entry = (id: string, over: Partial<RosterEntry> = {}) =>
    ({ id, kind: CharacterKind.PLAYER, reserved: false, ...over }) as RosterEntry;
  const input = {
    adding: false,
    players: [entry('p1'), entry('p2'), entry('p3', { reserved: true })],
    included: new Set(['p1', 'p3']),
    npcs: [entry('n1'), entry('n2')],
    counts: new Map([['n1', 2]]),
    hiddenAt: new Map([['n1', false]]),
  };

  it('lists the checked party members that can fight, then each NPC with a count', () => {
    expect(joinSpecs(input)).toEqual([
      { characterId: 'p1' },
      { characterId: 'n1', count: 2, hidden: false },
    ]);
  });

  it('adding leaves the party out, and an NPC starts hidden unless said otherwise', () => {
    expect(joinSpecs({ ...input, adding: true, hiddenAt: new Map() })).toEqual([
      { characterId: 'n1', count: 2, hidden: true },
    ]);
  });
});
