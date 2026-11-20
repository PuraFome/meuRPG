import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { vi } from 'vitest';

import { CuttingWordsAnsweredSchema } from '../../../gen/meurpg/play/v1/resources_pb';
import { heldDie } from '../held-rolls/held-roll-testing';
import { CuttingFlow } from './cutting-flow';
import type { CutAnswer } from './npc-check-client';
import { cuttingOffer } from './npc-check-testing';

function answered(cut: boolean, face = 4, usesLeft = 2) {
  return create(CuttingWordsAnsweredSchema, {
    cut,
    usesLeft,
    die: cut ? { diceSides: 8, faces: [face] } : undefined,
  });
}

function flowWith(answer = vi.fn(() => Promise.resolve(answered(true)))) {
  let n = 0;
  const flow = new CuttingFlow(
    { answer },
    () => 'c1',
    () => `k${++n}`,
  );
  return { flow, answer };
}

describe('CuttingFlow', () => {
  it('asks the question of the first cutting offer and ignores the other held rolls', () => {
    const { flow } = flowWith();
    flow.sync([heldDie(11, 1, 6, { holdId: 'other' }), cuttingOffer()]);
    expect(flow.offer()?.holdId).toBe('hold-1');
    expect(flow.phase()).toBe('asking');
    flow.sync([heldDie(11, 1, 6, { holdId: 'other' })]);
    expect(flow.phase()).toBe('skipped');
  });

  it('cuts with the app dice and reads its own die and the uses left, nothing of the creature', async () => {
    const { flow, answer } = flowWith();
    flow.sync([cuttingOffer()]);
    await flow.answer(true, null);
    expect(answer).toHaveBeenCalledWith('c1', 'hold-1', { cut: true, die: { inApp: true } }, 'k1');
    expect(flow.phase()).toBe('result');
    expect(flow.result()?.tag).toBe('Você cortou: −4');
    expect(flow.result()?.chips).toEqual(['Gastou 1 Inspiração de Bardo', 'Restam 2']);
  });

  it('cuts with the face of a physical die', async () => {
    const { flow, answer } = flowWith();
    flow.sync([cuttingOffer()]);
    await flow.answer(true, 5);
    expect((answer.mock.calls[0] as unknown as [string, string, CutAnswer])[2]).toEqual({
      cut: true,
      die: { face: 5 },
    });
  });

  it('lets it pass: nothing is spent and there is no result to read', async () => {
    const { flow, answer } = flowWith(vi.fn(() => Promise.resolve(answered(false))));
    flow.sync([cuttingOffer()]);
    await flow.answer(false, null);
    expect((answer.mock.calls[0] as unknown as [string, string, CutAnswer])[2]).toEqual({
      cut: false,
    });
    expect(flow.phase()).toBe('passed');
    expect(flow.result()).toBeNull();
  });

  it('says the master went on without the answer when the offer goes while it waits', () => {
    const { flow } = flowWith();
    flow.sync([cuttingOffer()]);
    flow.sync([]);
    expect(flow.phase()).toBe('skipped');
    expect(flow.offer()?.holdId).toBe('hold-1');
  });

  it("does not take the offer going for the master's while its own answer is on the way", async () => {
    let finish: (v: ReturnType<typeof answered>) => void = () => undefined;
    const { flow } = flowWith(
      vi.fn(() => new Promise<ReturnType<typeof answered>>((r) => (finish = r))),
    );
    flow.sync([cuttingOffer()]);
    const sent = flow.answer(true, null);
    expect(flow.busy()).toBe(true);
    flow.sync([]);
    expect(flow.phase()).toBe('asking');
    finish(answered(true));
    await sent;
    expect(flow.phase()).toBe('result');
    expect(flow.busy()).toBe(false);
  });

  it('reads "já foi respondida" for an answer that came after the master went on, and spent nothing', async () => {
    const { flow } = flowWith(
      vi.fn(() => Promise.reject(new ConnectError('closed', Code.FailedPrecondition))),
    );
    flow.sync([cuttingOffer()]);
    await flow.answer(true, null);
    expect(flow.phase()).toBe('late');
    expect(flow.error()?.lead).toBe('Essa pergunta já foi respondida.');
  });

  it('keeps the question and the same key when the answer fails for another reason', async () => {
    const answer = vi
      .fn()
      .mockRejectedValueOnce(new ConnectError('down', Code.Unavailable))
      .mockResolvedValueOnce(answered(true));
    const { flow } = flowWith(answer);
    flow.sync([cuttingOffer()]);
    await flow.answer(true, null);
    expect(flow.phase()).toBe('asking');
    expect(flow.error()?.lead).toBe('Não deu para cortar agora.');
    await flow.answer(true, null);
    expect(answer.mock.calls[0][3]).toBe(answer.mock.calls[1][3]);
    expect(flow.phase()).toBe('result');
    expect(flow.error()).toBeNull();
  });

  it('does not ask again what was answered, and asks the next hold', async () => {
    const { flow } = flowWith();
    flow.sync([cuttingOffer()]);
    await flow.answer(true, null);
    flow.acknowledge();
    expect(flow.offer()).toBeNull();
    flow.sync([cuttingOffer()]);
    expect(flow.offer()).toBeNull();
    flow.sync([cuttingOffer({ holdId: 'hold-2' })]);
    expect(flow.offer()?.holdId).toBe('hold-2');
    expect(flow.phase()).toBe('asking');
  });
});
