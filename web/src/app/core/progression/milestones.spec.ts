import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import {
  type Milestone,
  MilestoneSchema,
  XPAwardSchema,
} from '../../../gen/meurpg/progression/v1/progression_pb';
import {
  givenConfirmation,
  leveledLine,
  markLines,
  markedIds,
  plannedCount,
  plannedOf,
  playerAnnouncement,
  reachedConfirmation,
  reachedEffect,
  reachedOf,
  undoMilestoneConsequence,
} from './milestones';

const mark = (names: string[], given = 'Samuel', at = new Date(2026, 9, 3, 22, 5)) =>
  create(XPAwardSchema, {
    id: `a-${names.join('')}`,
    givenByDisplayName: given,
    createdAt: timestampFromDate(at),
    canUndo: true,
    shares: names.map((n) => ({ characterId: n.toLowerCase(), characterName: n })),
  });

const planned = (id: string, text: string): Milestone => create(MilestoneSchema, { id, text });
const reached = (
  id: string,
  text: string,
  at: Date,
  marks = [mark(['Pensantus', 'Toren'])],
  over = {},
): Milestone =>
  create(MilestoneSchema, {
    id,
    text,
    reached: true,
    reachedAt: timestampFromDate(at),
    marks,
    ...over,
  });

describe('milestones (E8-14)', () => {
  it('splits the planned from the reached, the last reached first', () => {
    const list = [
      planned('p1', 'Salvar o mercador'),
      reached('r1', 'Chegar ao Vale Seco', new Date(2026, 9, 3, 22, 5)),
      reached('r2', 'Derrotar o Barão', new Date(2026, 9, 4, 20, 0)),
    ];
    expect(plannedOf(list).map((m) => m.id)).toEqual(['p1']);
    expect(reachedOf(list).map((m) => m.id)).toEqual(['r2', 'r1']);
  });

  it('an off-list milestone is reached, never planned', () => {
    const off = reached('x', 'Ponte salva', new Date(), [mark(['Brisa'])], { offList: true });
    expect(plannedOf([off])).toEqual([]);
    expect(reachedOf([off])).toHaveLength(1);
  });

  it('tells who marked and who got it later, for the master', () => {
    const m = reached('r', 'Chegar ao Vale Seco', new Date(), [
      mark(['Pensantus', 'Toren']),
      mark(['Brisa'], 'Samuel', new Date(2026, 9, 4, 19, 30)),
    ]);
    expect(markLines(m).map((l) => l.replace(/\u00a0/g, ' '))).toEqual([
      'Samuel marcou Pensantus e Toren',
      'Samuel deu a Brisa · 04/10 às 19:30',
    ]);
    expect([...markedIds(m)]).toEqual(['pensantus', 'toren', 'brisa']);
  });

  it('tells a player who levelled, in the singular and the plural', () => {
    expect(leveledLine(reached('r', 'x', new Date()))).toBe('Subiram de nível: Pensantus e Toren');
    expect(leveledLine(reached('r', 'x', new Date(), [mark(['Brisa'])]))).toBe(
      'Subiu de nível: Brisa',
    );
  });

  it('counts the planned ones, only for the master', () => {
    expect(plannedCount(3)).toBe('3 marcos · só você vê');
    expect(plannedCount(1)).toBe('1 marco · só você vê');
  });

  it('says the effect of marking, and who was left out', () => {
    expect(reachedConfirmation(['Pensantus', 'Toren'], ['Brisa'])).toBe(
      'Marco alcançado: Pensantus e Toren podem subir de nível. Brisa continua como estava.',
    );
    expect(reachedConfirmation(['Pensantus'], [])).toBe(
      'Marco alcançado: Pensantus pode subir de nível.',
    );
    expect(givenConfirmation(['Brisa'])).toBe('Brisa pode subir de nível com o marco.');
    expect(reachedEffect(0)).toBe('');
    expect(reachedEffect(1)).toBe('1 personagem pode subir de nível');
    expect(reachedEffect(2)).toBe('2 personagens podem subir de nível');
  });

  it('says what undoing does: back to planned, or stays reached for the others', () => {
    const only = reached('r', 'Chegar', new Date());
    expect(undoMilestoneConsequence(only, only.marks[0])).toContain(
      'volta para “Marcos planejados”',
    );
    expect(undoMilestoneConsequence(only, only.marks[0])).toContain('Fica registrado.');
    const two = reached('r', 'Chegar', new Date(), [mark(['Pensantus']), mark(['Brisa'])]);
    expect(undoMilestoneConsequence(two, two.marks[1])).toContain(
      'continua alcançado para os outros',
    );
    const off = reached('x', 'Ponte', new Date(), [mark(['Brisa'])], { offList: true });
    expect(undoMilestoneConsequence(off, off.marks[0])).toContain('some desta lista');
  });

  it('announces to a player only their own character', () => {
    const m = reached('r', 'Chegar ao Vale Seco', new Date());
    expect(playerAnnouncement(m, new Set(['pensantus']))).toBe(
      'O mestre marcou Chegar ao Vale Seco. Pensantus pode subir de nível.',
    );
    expect(playerAnnouncement(m, new Set(['brisa']))).toBe('O mestre marcou Chegar ao Vale Seco.');
  });
});
