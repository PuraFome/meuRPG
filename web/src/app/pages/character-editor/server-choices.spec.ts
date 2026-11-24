import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { ChoicesPreviewVm } from './character-editor.types';
import { ServerChoices } from './server-choices';

const answerOf = (done: number) =>
  ({ groups: [], notOffered: [], done, total: 2 }) as unknown as ChoicesPreviewVm;

describe('ServerChoices', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('keeps the same answer object when the server says the same again, so nothing downstream asks again', async () => {
    let done = 1;
    const choices = new ServerChoices(() => Promise.resolve(answerOf(done)));
    choices.draftChanged(true);
    await vi.advanceTimersByTimeAsync(400);
    const first = choices.answer();
    expect(first?.done).toBe(1);

    choices.draftChanged(true);
    await vi.advanceTimersByTimeAsync(400);
    expect(choices.answer()).toBe(first);

    done = 2;
    choices.draftChanged(true);
    await vi.advanceTimersByTimeAsync(400);
    expect(choices.answer()).not.toBe(first);
    expect(choices.answer()?.done).toBe(2);
  });

  it('says whether an answer was asked for the input of the answer before it', async () => {
    let key = 'a';
    const choices = new ServerChoices(
      () => Promise.resolve(answerOf(1)),
      () => key,
    );
    const land = async () => {
      choices.draftChanged(true);
      await vi.advanceTimersByTimeAsync(400);
    };
    await land();
    expect(choices.sameInput).toBe(true);
    await land();
    expect(choices.sameInput).toBe(true);
    key = 'b';
    await land();
    expect(choices.sameInput).toBe(false);
    choices.stop();
    key = 'c';
    await land();
    expect(choices.sameInput).toBe(true);
  });
});
