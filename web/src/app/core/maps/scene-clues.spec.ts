import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import { SceneClueSchema, type SceneClue } from '../../../gen/meurpg/maps/v1/maps_pb';
import {
  type CluePlayer,
  clueAudience,
  clueCount,
  clueTextError,
  listNames,
  moreClues,
  oneLine,
  playersWithout,
  revealLabel,
  revealSummary,
  revealedLine,
} from './scene-clues';

const PLAYERS: CluePlayer[] = [
  { id: 'p', name: 'Pensantus', playerName: 'Vinicius' },
  { id: 't', name: 'Toren', playerName: 'Caio' },
  { id: 'b', name: 'Brisa', playerName: 'Lia' },
];

function clue(revealedTo: { id: string; name?: string; at?: Date }[]): SceneClue {
  return create(SceneClueSchema, {
    id: 'c1',
    text: 'Uma pista',
    revealedTo: revealedTo.map((r) => ({
      characterId: r.id,
      characterName: r.name ?? '',
      revealedAt: timestampFromDate(r.at ?? new Date(2026, 9, 3, 21, 26)),
    })),
  });
}

/** The numbers are tied to their words with no-break spaces; the spec reads them as spaces. */
const plain = (text: string) => text.replace(/\u00a0/g, ' ');

describe('clue text', () => {
  it('asks for the words when there are none, and says how many to take off when there are too many', () => {
    expect(clueTextError('   ')).toBe('Escreva a pista antes de salvar. Ela pode ter até 500 caracteres.');
    expect(clueTextError('x'.repeat(500))).toBe('');
    expect(plain(clueTextError('x'.repeat(512)))).toBe('A pista passa de 500 caracteres: tem 512, tire 12.');
  });

  it('counts characters, not UTF-16 units', () => {
    expect(clueTextError('😀'.repeat(500))).toBe('');
  });

  it('flattens a pasted line break into a space', () => {
    expect(oneLine('uma\nlinha\r\n  outra')).toBe('uma linha outra');
  });

  it('counts the clues with the number tied to its word', () => {
    expect(clueCount(1)).toBe('1 pista');
    expect(clueCount(3)).toBe('3 pistas');
    expect(moreClues(28)).toBe('Mais 28 pistas na lista.');
  });
});

describe('who has a clue', () => {
  it('says "Ninguém ainda" for none', () => {
    expect(clueAudience(clue([]), PLAYERS)).toMatchObject({ kind: 'none', label: 'Ninguém ainda' });
  });

  it('says "Só Brisa" for one, with no article, and lists two', () => {
    expect(clueAudience(clue([{ id: 'b', name: 'Brisa' }]), PLAYERS).label).toBe('Só Brisa');
    expect(clueAudience(clue([{ id: 'b', name: 'Brisa' }, { id: 't', name: 'Toren' }]), PLAYERS).label).toBe('Só Brisa e Toren');
  });

  it('says "Todos" only when every player character has it', () => {
    const all = clue(PLAYERS.map((p) => ({ id: p.id, name: p.name })));
    expect(clueAudience(all, PLAYERS)).toMatchObject({ kind: 'all', label: 'Todos' });
    expect(clueAudience(clue([{ id: 'p' }, { id: 't' }]), PLAYERS).kind).toBe('some');
  });

  it('takes the name from the roster when the character no longer lives', () => {
    expect(clueAudience(clue([{ id: 'b' }]), PLAYERS).label).toBe('Só Brisa');
    expect(clueAudience(clue([{ id: 'zz' }]), PLAYERS).label).toBe('Só Personagem');
  });

  it('lists the names the way a person says them', () => {
    expect(listNames([])).toBe('');
    expect(listNames(['Brisa'])).toBe('Brisa');
    expect(listNames(['Brisa', 'Toren', 'Aldo'])).toBe('Brisa, Toren e Aldo');
  });

  it('tells the session line with the time of the last reveal, the time tied to "às"', () => {
    const all = clue(PLAYERS.map((p) => ({ id: p.id, at: new Date(2026, 9, 3, 21, 20) })));
    expect(revealedLine(all, PLAYERS)).toBe('Revelada para todos às 21:20');
    const some = clue([{ id: 'b', at: new Date(2026, 9, 3, 21, 26) }]);
    expect(revealedLine(some, PLAYERS)).toBe('Revelada só para Brisa às 21:26');
    expect(revealedLine(clue([]), PLAYERS)).toBe('Ninguém ainda');
  });

  it('lists who still can receive it', () => {
    expect(playersWithout(clue([{ id: 'b' }]), PLAYERS).map((p) => p.id)).toEqual(['p', 't']);
  });
});

describe('the reveal button', () => {
  const pick = (...ids: string[]) => PLAYERS.filter((p) => ids.includes(p.id));

  it('is the dashed "Revelar a pista" while nobody is checked', () => {
    expect(revealLabel([], 3)).toBe('Revelar a pista');
    expect(revealSummary([], 3)).toBe('Ninguém marcado. Escolha quem recebe a pista.');
  });

  it('names one player with no article, counts two, and says "todos" for all', () => {
    expect(revealLabel(pick('b'), 3)).toBe('Revelar para Brisa');
    expect(revealLabel(pick('b', 't'), 3)).toBe('Revelar para 2 jogadores');
    expect(revealLabel(pick('p', 't', 'b'), 3)).toBe('Revelar para todos');
    expect(revealLabel(pick('b'), 1)).toBe('Revelar para Brisa');
  });

  it('never says "todos" when someone already had the clue: "Revelar aos outros", or the names or the count', () => {
    // Brisa has it: Pensantus and Toren can still receive it (open 2 of 3 players).
    expect(revealLabel(pick('p', 't'), 2, 3)).toBe('Revelar aos outros');
    expect(revealLabel(pick('t'), 2, 3)).toBe('Revelar para Toren');
    expect(plain(revealSummary(pick('p', 't'), 2, 3))).toBe('A pista vai para os outros 2 jogadores.');
    // Only one is left: it is that one by name.
    expect(revealLabel(pick('t'), 1, 3)).toBe('Revelar para Toren');
    // Three to go among four players, two checked: a count.
    expect(revealLabel(pick('p', 't'), 3, 4)).toBe('Revelar para 2 jogadores');
    // Everyone of the campaign checked is "todos".
    expect(revealLabel(pick('p', 't', 'b'), 3, 3)).toBe('Revelar para todos');
  });

  it('says who gets it above the buttons', () => {
    expect(plain(revealSummary(pick('b'), 3))).toBe('A pista vai para 1 de 3 jogadores: Brisa.');
    expect(plain(revealSummary(pick('p', 't', 'b'), 3))).toBe('A pista vai para todos os 3 jogadores.');
  });
});
