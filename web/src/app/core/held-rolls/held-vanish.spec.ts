import { forgetAnsweredHere, markAnsweredHere } from './held-roll-controller';
import { MASTER_ANSWERED_TEXT, HeldVanishWatch } from './held-vanish';
import { heldDie, heldLucky } from './held-roll-testing';

describe('HeldVanishWatch', () => {
  beforeEach(() => forgetAnsweredHere());

  it('says nothing while the question stays', () => {
    const watch = new HeldVanishWatch();
    watch.observe(heldLucky());
    watch.observe(heldLucky());
    expect(watch.notice()).toBe('');
  });

  it('says the master answered when the question vanishes and this browser did not answer it', () => {
    const watch = new HeldVanishWatch();
    watch.observe(heldLucky());
    watch.observe(null);
    expect(watch.notice()).toBe(MASTER_ANSWERED_TEXT);
    expect(MASTER_ANSWERED_TEXT).toBe('O mestre respondeu por você.');
  });

  it('says it when the master’s answer moved the roll on to the die’s question', () => {
    const watch = new HeldVanishWatch();
    watch.observe(heldLucky());
    watch.observe(heldDie());
    expect(watch.notice()).toBe(MASTER_ANSWERED_TEXT);
  });

  it('says nothing when the player answered it here', () => {
    const watch = new HeldVanishWatch();
    const question = heldLucky();
    watch.observe(question);
    markAnsweredHere(question);
    watch.observe(null);
    expect(watch.notice()).toBe('');
  });

  it('asks about the die separately: Lucky answered here does not hide the master’s answer to the die', () => {
    const watch = new HeldVanishWatch();
    const lucky = heldLucky();
    markAnsweredHere(lucky);
    watch.observe(lucky);
    watch.observe(heldDie());
    watch.observe(null);
    expect(watch.notice()).toBe(MASTER_ANSWERED_TEXT);
  });

  it('clears when a new question comes, and when dismissed', () => {
    const watch = new HeldVanishWatch();
    watch.observe(heldLucky());
    watch.observe(null);
    watch.observe(heldLucky([1], 5, { holdId: 'h2' }));
    expect(watch.notice()).toBe('');
    watch.observe(null);
    expect(watch.notice()).toBe(MASTER_ANSWERED_TEXT);
    watch.dismiss();
    expect(watch.notice()).toBe('');
  });
});
