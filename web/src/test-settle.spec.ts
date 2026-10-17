import { timer } from 'rxjs';

import { nextTurn, settle } from './test-settle';

describe('test-settle', () => {
  afterEach(() => vi.useRealTimers());

  it('waits one real turn when the clock is real', async () => {
    const real = vi.spyOn(globalThis, 'setTimeout');
    await nextTurn();
    expect(real).toHaveBeenCalledWith(expect.any(Function), 0);
    real.mockRestore();
  });

  it('does not hang on a fake clock a spec left on: it advances that clock', async () => {
    vi.useFakeTimers();
    let ran = false;
    // rxjs `timer` schedules on the faked clock, as a component's own timer would.
    timer(0).subscribe(() => (ran = true));
    await nextTurn();
    expect(ran).toBe(true);
  });

  it('does not hang when only Date is faked, as the puzzle specs do', async () => {
    vi.useFakeTimers({ toFake: ['Date'] });
    await nextTurn();
  });

  it('settles a fixture: one render before and one after the turns', async () => {
    const fixture = { detectChanges: vi.fn(), whenStable: vi.fn().mockResolvedValue(undefined) };
    await settle(fixture, 2);
    expect(fixture.detectChanges).toHaveBeenCalledTimes(3);
    expect(fixture.whenStable).toHaveBeenCalledTimes(2);
  });
});
