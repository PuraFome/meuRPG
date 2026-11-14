import { TestBed } from '@angular/core/testing';

import { saveFlow } from './save-flow';
import type { SheetHandle } from './sheet-host';

function setup(run: () => Promise<string>) {
  const closed: (boolean | undefined)[] = [];
  const locks: boolean[] = [];
  const sheet: SheetHandle<unknown, boolean> = {
    data: null,
    inSheet: false,
    close: (r) => closed.push(r),
    lock: (l) => locks.push(l),
  };
  const flow = TestBed.runInInjectionContext(() => saveFlow(sheet, run));
  TestBed.tick();
  return { flow, closed, locks };
}

describe('saveFlow', () => {
  it('locks the sheet while the save is in the air and closes it with true on success', async () => {
    let release!: (why: string) => void;
    const { flow, closed, locks } = setup(() => new Promise<string>((r) => (release = r)));
    const saving = flow.save();
    expect(flow.busy()).toBe(true);
    TestBed.tick();
    expect(locks.at(-1)).toBe(true);
    release('');
    await saving;
    TestBed.tick();
    expect(flow.busy()).toBe(false);
    expect(locks.at(-1)).toBe(false);
    expect(closed).toEqual([true]);
  });

  it('says the refusal, stays open and unlocks', async () => {
    const { flow, closed } = setup(() => Promise.resolve('Não deu.'));
    await flow.save();
    expect(flow.error()).toBe('Não deu.');
    expect(flow.busy()).toBe(false);
    expect(closed).toEqual([]);
  });

  it('clears the old refusal when it saves again', async () => {
    let why = 'Não deu.';
    const { flow, closed } = setup(() => Promise.resolve(why));
    await flow.save();
    why = '';
    await flow.save();
    expect(flow.error()).toBe('');
    expect(closed).toEqual([true]);
  });

  it('runs once when saved twice while the first is in the air', async () => {
    let release!: (why: string) => void;
    let runs = 0;
    const { flow } = setup(() => {
      runs++;
      return new Promise<string>((r) => (release = r));
    });
    const first = flow.save();
    const second = flow.save();
    release('');
    await Promise.all([first, second]);
    expect(runs).toBe(1);
  });

  it('does not close on cancel while busy, and closes when idle', async () => {
    let release!: (why: string) => void;
    const { flow, closed } = setup(() => new Promise<string>((r) => (release = r)));
    const saving = flow.save();
    flow.cancel();
    expect(closed).toEqual([]);
    release('x');
    await saving;
    flow.cancel();
    expect(closed).toEqual([undefined]);
  });
});
