import { vi } from 'vitest';

import type { TargetRule } from '../../../../core/combat/cast-flow';
import type { PickRow } from '../../../../core/combat/target-pick';
import { type PickSession, TargetMode } from './target-mode';

const rows: PickRow[] = [
  { id: 'g1', label: 'Goblin 1', reason: '', note: '' },
  { id: 'g2', label: 'Goblin 2', reason: '', note: 'Longe: desvantagem' },
  { id: 'g3', label: 'Goblin 3', reason: 'cobertura total', note: '' },
];
const SINGLE: TargetRule = { kind: 'single', max: 1, min: 1 };
const DARTS: TargetRule = { kind: 'darts', max: 3, min: 1 };

function start(rule: TargetRule = SINGLE) {
  const open = vi.fn();
  const origin = document.createElement('button');
  document.body.append(origin);
  const mode = new TargetMode((run) => run());
  const session: PickSession = {
    key: 'spell:fire-bolt',
    name: 'Raio de Fogo',
    rows,
    rule,
    detail: '+5 para acertar · 1d10 de fogo',
    foe: true,
    origin,
    open,
  };
  mode.begin(session);
  return { mode, open, origin };
}

describe('TargetMode', () => {
  it('starts with nothing chosen and Confirmar not ready', () => {
    const { mode } = start();
    expect(mode.active()).toBe(true);
    expect(mode.ready()).toBe(false);
    expect(mode.words()?.title).toBe('Escolha o alvo · Raio de Fogo');
    expect(mode.marks()?.map((m) => m.count)).toEqual([0, 0, 0]);
  });

  it('names the target tapped, replaces it on another tap, and is ready', () => {
    const { mode } = start();
    mode.tap('g1');
    expect(mode.words()?.title).toBe('Raio de Fogo em Goblin 1');
    expect(mode.ready()).toBe(true);
    mode.tap('g2');
    expect(mode.marks()?.map((m) => m.count)).toEqual([0, 1, 0]);
    expect(mode.status()).toBe('Goblin 2 é o alvo');
  });

  it('answers a tap on a dimmed creature with the reason, and chooses nothing', () => {
    const { mode } = start();
    mode.tap('g3');
    expect(mode.note()).toEqual({ id: 'g3', text: 'Goblin 3: cobertura total' });
    expect(mode.picks()).toEqual([]);
    mode.tap('g1');
    expect(mode.note()).toBeNull();
  });

  it('puts a dart for each tap, undoes the last and is ready with all of them placed', () => {
    const { mode } = start(DARTS);
    mode.tap('g1');
    mode.tap('g1');
    expect(mode.words()?.title).toBe('Raio de Fogo · 2 de 3 dardos');
    expect(mode.status()).toBe('2 de 3 dardos');
    expect(mode.marks()?.[0].count).toBe(2);
    expect(mode.ready()).toBe(false);
    mode.tap('g2');
    expect(mode.ready()).toBe(true);
    mode.undo();
    expect(mode.picks()).toEqual(['g1', 'g1']);
    expect(mode.ready()).toBe(false);
  });

  it('cancels back to the action’s button', () => {
    const { mode, origin, open } = start();
    const focus = vi.spyOn(origin, 'focus');
    mode.tap('g1');
    mode.cancel();
    expect(mode.active()).toBe(false);
    expect(focus).toHaveBeenCalled();
    expect(open).not.toHaveBeenCalled();
  });

  it('confirms: the sheet opens with the picks, once the focus is back on the button', () => {
    const { mode, open } = start();
    mode.confirm();
    expect(open).not.toHaveBeenCalled();
    mode.tap('g1');
    mode.confirm();
    expect(open).toHaveBeenCalledWith(['g1']);
    expect(mode.active()).toBe(false);
  });

  it('opens the list without a target', () => {
    const { mode, open } = start();
    mode.tap('g1');
    mode.list();
    expect(open).toHaveBeenCalledWith(null);
  });
});
