import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import type { RosterEntry } from '../../../core/maps/roster-client';
import {
  ConversationRoster,
  type RosterSource,
  chooserPlayers,
  directoryOf,
  speakersOf,
} from './conversation-roster';

function entry(id: string, name: string, extra: Partial<RosterEntry> = {}): RosterEntry {
  return {
    id,
    name,
    kind: CharacterKind.PLAYER,
    playerUserId: `u-${id}`,
    classSummary: '',
    raceName: '',
    playerName: null,
    portraitImageId: '',
    ...extra,
  };
}

const npc = (id: string, name: string) =>
  entry(id, name, { kind: CharacterKind.ENEMY, playerUserId: '' });

describe('chooserPlayers', () => {
  it('lists the player characters by account with the class and the race under the name', () => {
    const rows = chooserPlayers([
      entry('p', 'Pensantus', { classSummary: 'Mago 3', raceName: 'Gnomo das Rochas' }),
      npc('n', 'Bartolo'),
      entry('o', 'Órfão', { playerUserId: '' }),
      entry('b', 'Brisa', { classSummary: 'Ladina 2' }),
    ]);
    expect(rows).toEqual([
      { userId: 'u-p', name: 'Pensantus', sub: 'Mago 3 · Gnomo das Rochas' },
      { userId: 'u-b', name: 'Brisa', sub: 'Ladina 2' },
    ]);
  });
});

describe('speakersOf', () => {
  it('starts with the narrator, then the NPCs on stage, then the others by name', () => {
    const speakers = speakersOf(
      [entry('p', 'Pensantus'), npc('z', 'Zeca'), npc('b', 'Bartolo'), npc('h', 'Homem do canto')],
      new Set(['h']),
    );
    expect(speakers).toEqual([
      { value: '', label: 'Narrador', onStage: false },
      { value: 'h', label: 'Homem do canto', onStage: true },
      { value: 'b', label: 'Bartolo', onStage: false },
      { value: 'z', label: 'Zeca', onStage: false },
    ]);
  });
});

describe('directoryOf', () => {
  it('names an account by its character, else by the name the person chose', () => {
    const directory = directoryOf(
      [entry('p', 'Pensantus')],
      [
        { userId: 'u-p', name: 'Caio', master: false },
        { userId: 'u-x', name: 'Dani', master: false },
      ],
    );
    expect(directory.get('u-p')).toBe('Pensantus');
    expect(directory.get('u-x')).toBe('Dani');
  });
});

describe('ConversationRoster', () => {
  function setup(isMaster: boolean, stage: string[] = []) {
    const roster = vi.fn(() => Promise.resolve([entry('p', 'Pensantus'), npc('b', 'Bartolo')]));
    const members = vi.fn(() =>
      Promise.resolve([
        { userId: 'm', name: 'Mestre', master: true },
        { userId: 'u-p', name: 'Caio', master: false },
        { userId: 'u-q', name: 'Dani', master: false },
      ]),
    );
    const source: RosterSource = { roster, members };
    const model = new ConversationRoster(
      source,
      () => 'c1',
      () => isMaster,
      () => new Set(stage),
    );
    return { model, roster, members };
  }

  it('counts the players at the table, not the master', async () => {
    const { model } = setup(false);
    await model.load();
    expect(model.tableSize()).toBe(2);
  });

  it('gives a player no roster: the characters of others are not theirs to list', async () => {
    const { model, roster } = setup(false);
    await model.load();
    expect(roster).not.toHaveBeenCalled();
    expect(model.players()).toEqual([]);
  });

  it('gives the master the players, the speakers and and the directory', async () => {
    const stage = new Set<string>();
    const model = new ConversationRoster(
      {
        roster: () => Promise.resolve([entry('p', 'Pensantus'), npc('b', 'Bartolo')]),
        members: () => Promise.resolve([]),
      },
      () => 'c1',
      () => true,
      () => stage,
    );
    await model.load();
    expect(model.players().map((p) => p.name)).toEqual(['Pensantus']);
    expect(model.directory().get('u-p')).toBe('Pensantus');
    expect(model.speakers().map((s) => s.label)).toEqual(['Narrador', 'Bartolo']);
  });

  it('survives a read that fails: the lists stay empty', async () => {
    const model = new ConversationRoster(
      {
        roster: () => Promise.reject(new Error('x')),
        members: () => Promise.reject(new Error('x')),
      },
      () => 'c1',
      () => true,
      () => new Set(),
    );
    await model.load();
    expect(model.players()).toEqual([]);
    expect(model.speakers().map((s) => s.label)).toEqual(['Narrador']);
  });
});
