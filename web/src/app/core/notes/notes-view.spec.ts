import {
  FILTER_ALL,
  FILTER_NONE,
  applyFilter,
  entryCount,
  filterOptions,
  noteCounter,
  noteStamp,
  sortNotes,
} from './notes-view';
import { note, scene } from './notes-testing';

describe('notes view', () => {
  const now = new Date(2026, 9, 3, 22, 0);

  it('writes when a note was written: today, yesterday, a day of this year, another year', () => {
    expect(noteStamp(new Date(2026, 9, 3, 21, 24), now)).toBe('Hoje, 21:24');
    expect(noteStamp(new Date(2026, 9, 2, 22, 3), now)).toBe('Ontem, 22:03');
    expect(noteStamp(new Date(2026, 8, 30, 18, 40), now)).toBe('30/09, 18:40');
    expect(noteStamp(new Date(2025, 11, 31, 9, 5), now)).toBe('31/12/2025, 09:05');
  });

  it('sorts newest first, by the last time written', () => {
    const list = sortNotes([
      note('a', 'a', new Date(2026, 9, 1)),
      note('b', 'b', new Date(2026, 9, 3)),
      note('c', 'c', new Date(2026, 9, 2)),
    ]);
    expect(list.map((n) => n.id)).toEqual(['b', 'c', 'a']);
  });

  const notes = [
    note('1', 'um', new Date(2026, 9, 3, 21, 24), {
      sceneId: 's1',
      sceneName: 'A carroça tombada',
    }),
    note('2', 'dois', new Date(2026, 9, 3, 21, 20), {
      sceneId: 's1',
      sceneName: 'A carroça tombada',
      clue: true,
    }),
    note('3', 'três', new Date(2026, 9, 1, 22, 3)),
    note('4', 'quatro', new Date(2026, 8, 30, 18, 40)),
  ];

  it('lists "Todas as anotações", the discovered scenes and "Sem cena", each with its count (the clue counts)', () => {
    const options = filterOptions(notes, [
      scene('s1', 'A carroça tombada'),
      scene('s2', 'A ponte do rio'),
    ]);
    expect(options.map((o) => `${o.label} ${o.count}`)).toEqual([
      'Todas as anotações 4',
      'A carroça tombada 2',
      'A ponte do rio 0',
      'Sem cena 2',
    ]);
  });

  it('filters by a scene or by none, and keeps the order', () => {
    expect(applyFilter(notes, FILTER_ALL)).toHaveLength(4);
    expect(applyFilter(notes, 's1').map((n) => n.id)).toEqual(['1', '2']);
    expect(applyFilter(notes, FILTER_NONE).map((n) => n.id)).toEqual(['3', '4']);
    expect(applyFilter(notes, 's2')).toEqual([]);
  });

  it('writes counts', () => {
    expect(noteCounter(45, 2000)).toBe('45 de 2.000');
    expect(entryCount(1)).toBe('1 anotação');
    expect(entryCount(4)).toBe('4 anotações');
  });
});
