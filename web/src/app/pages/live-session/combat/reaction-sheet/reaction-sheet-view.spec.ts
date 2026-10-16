import { reactionWindow } from '../../../../core/combat/combat-testing';
import type { SlotRow } from '../../../../core/combat/cast-flow';
import {
  RACIAL_LEVEL,
  answerOf,
  escapeAction,
  firstManeuverKey,
  initialPicked,
  maxTargetsOf,
  nextSlot,
  slotAfter,
  slotRowsOf,
  throwBackCard,
} from './reaction-sheet-view';

const win = (case_: string | undefined, value: object | undefined) =>
  reactionWindow({ id: 'w', prompt: { case: case_, value } as never } as never);
const row = (level: number, over: Partial<SlotRow> = {}): SlotRow => ({
  level,
  pact: false,
  free: 2,
  total: 3,
  used: 1,
  enabled: true,
  title: `${level}`,
  count: '',
  ...over,
});

describe('reaction-sheet-view', () => {
  it('has no slot rows for a window without a list', () => {
    expect(slotRowsOf(win('cuttingWords', {}), [], null)).toEqual([]);
  });

  it('starts Queda Suave with the allies that fall, as many as it can save', () => {
    const fall = win('featherFall', {
      maxTargets: 1,
      falling: [
        { combatantId: 'a', label: 'A', ally: false },
        { combatantId: 'b', label: 'B', ally: true },
        { combatantId: 'c', label: 'C', ally: true },
      ],
    });
    expect([...initialPicked(fall)]).toEqual(['b']);
    expect(maxTargetsOf(fall)).toBe(1);
    expect(maxTargetsOf(win('featherFall', { maxTargets: 0, falling: [] }))).toBe(1);
    expect(initialPicked(win('shield', {})).size).toBe(0);
    expect(maxTargetsOf(win('shield', {}))).toBe(1);
  });

  it('picks the first maneuver, or none', () => {
    expect(firstManeuverKey(win('maneuverReduce', { options: [{ key: 'k' }] }))).toBe('k');
    expect(firstManeuverKey(win('maneuverReduce', { options: [] }))).toBe('');
    expect(firstManeuverKey(win('shield', {}))).toBe('');
  });

  it('draws the card of the second step', () => {
    expect(throwBackCard(4)).toMatchObject({ subtitle: 'Rodada 4', throwBack: true });
  });

  it('collects the answer: a pass, a slot, the Infernal Legacy, the creatures and the maneuver', () => {
    const asked = win('shield', {});
    const c = { asked, row: row(2), creatureIds: null, maneuverKey: '' };
    expect(answerOf(false, c)).toEqual({ use: false });
    expect(answerOf(true, c)).toEqual({ use: true, slot: { level: 2, pact: false } });
    const rebuke = { ...c, asked: win('hellishRebuke', {}), row: row(RACIAL_LEVEL) };
    expect(answerOf(true, rebuke)).toEqual({ use: true, useRacial: true });
    const all = { ...c, creatureIds: ['a'], maneuverKey: 'm' };
    expect(answerOf(true, all, { inApp: true })).toEqual({
      use: true,
      slot: { level: 2, pact: false },
      creatureIds: ['a'],
      maneuverKey: 'm',
      die: { inApp: true },
    });
  });

  it('tells the slot the answer spent', () => {
    expect(slotAfter({ use: true, slot: { level: 2, pact: false } }, row(2))).toEqual({
      level: 2,
      free: 1,
      total: 3,
    });
    expect(
      slotAfter({ use: true, slot: { level: 2, pact: false } }, row(2, { free: 0 }))?.free,
    ).toBe(0);
    expect(slotAfter({ use: true }, row(2))).toBeNull();
    expect(slotAfter({ use: true, slot: { level: 2, pact: false } }, null)).toBeNull();
  });

  it('moves to the next enabled slot and wraps, or nowhere', () => {
    const rows = [row(1), row(2, { enabled: false }), row(3)];
    expect(nextSlot(rows, rows[0], true)?.level).toBe(3);
    expect(nextSlot(rows, rows[0], false)?.level).toBe(3);
    expect(nextSlot(rows, rows[2], true)?.level).toBe(1);
    expect(nextSlot(rows, null, true)).toBeNull();
    expect(nextSlot([row(1)], row(1), true)).toBeNull();
  });

  it('decides what Esc does at each stage', () => {
    expect(escapeAction('ask', false, false)).toBe('pass');
    expect(escapeAction('ask', true, false)).toBe('none');
    expect(escapeAction('roll', false, false)).toBe('back');
    expect(escapeAction('result', false, true)).toBe('keep');
    expect(escapeAction('result', false, false)).toBe('close');
    expect(escapeAction('gone', false, false)).toBe('close');
  });
});
