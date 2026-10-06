import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  GetImageGenerationResponseSchema,
  ImageGenerationFailure,
  ImageGenerationState,
} from '../../../gen/meurpg/maps/v1/imagegen_pb';
import type { ImageGenClient } from './imagegen-client';
import { SLOW_AFTER_SECONDS, ImageRun } from './imagegen-run';
import { FakeImageGenClient, done, generation, imageStatus } from './imagegen-testing';

const instant = () => Promise.resolve();

describe('ImageRun: from the click to the picture', () => {
  let api: FakeImageGenClient;

  function run(): ImageRun {
    return new ImageRun(api as unknown as ImageGenClient, 'camp-1', (signal) => api.generateScene('camp-1', {} as never, signal), instant);
  }

  beforeEach(() => {
    api = new FakeImageGenClient();
  });

  it('asks, then keeps one long poll after the other open (25 seconds each) until the request leaves PENDING, and ends with the picture', async () => {
    api.polls = [
      create(GetImageGenerationResponseSchema, { generation: generation() }),
      create(GetImageGenerationResponseSchema, { generation: generation() }),
      done('img-9', 3),
    ];
    const r = run();
    await r.start();
    expect(r.phase()).toBe('done');
    expect(r.image()?.id).toBe('img-9');
    expect(r.generation()?.number).toBe(3);
    expect(api.polled).toEqual([
      { generationId: 'gen-1', waitSeconds: 25 },
      { generationId: 'gen-1', waitSeconds: 25 },
      { generationId: 'gen-1', waitSeconds: 25 },
    ]);
    // The month after the slot was reserved, then after the picture.
    expect(r.status()?.remaining).toBe(16);
  });

  it('is waiting (with the long poll behind it) while the request is PENDING, and "sending" until the server answers', async () => {
    let release!: () => void;
    api.hold = new Promise<void>((r) => (release = r));
    api.polls = [done()];
    const r = run();
    expect(r.phase()).toBe('sending');
    const finished = r.start();
    await vi.waitFor(() => expect(r.phase()).toBe('waiting'));
    release();
    await finished;
    expect(r.phase()).toBe('done');
  });

  it('says "the service is slow" only after 60 seconds of waiting, counting whole seconds', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    try {
      let release!: () => void;
      api.hold = new Promise<void>((r) => (release = r));
      api.polls = [done()];
      const r = run();
      const finished = r.start();
      await vi.waitFor(() => expect(r.phase()).toBe('waiting'));
      vi.advanceTimersByTime(8000);
      expect(r.seconds()).toBe(8);
      expect(r.slow()).toBe(false);
      vi.advanceTimersByTime((SLOW_AFTER_SECONDS - 8) * 1000);
      expect(r.slow()).toBe(true);
      release();
      await finished;
      // The clock stops with the request.
      vi.advanceTimersByTime(5000);
      expect(r.seconds()).toBe(SLOW_AFTER_SECONDS);
      expect(r.slow()).toBe(false);
    } finally {
      vi.useRealTimers();
    }
  });

  it('rejects, with nothing reserved, when the server refuses the ask itself (the dialog goes back to the form with the reason)', async () => {
    api.started = new ConnectError('no', Code.FailedPrecondition);
    const r = run();
    await expect(r.start()).rejects.toBeInstanceOf(ConnectError);
    expect(api.polled).toEqual([]);
  });

  it('tells a refusal by the service as a failure in words, and says whether the slot came back', async () => {
    api.polls = [
      create(GetImageGenerationResponseSchema, {
        generation: generation({ state: ImageGenerationState.REFUSED, failure: ImageGenerationFailure.REFUSED, slotSpent: false }),
      }),
    ];
    const r = run();
    await r.start();
    expect(r.phase()).toBe('failed');
    expect(r.failure()).toBe('O serviço recusou este texto. Tente descrever a cena de outro jeito. Esta tentativa não gastou nenhuma imagem do mês.');
  });

  it('keeps waiting through a failed poll, tries again, and gives up after three in a row, saying the request is still there', async () => {
    api.polls = [new Error('network'), done()];
    const ok = run();
    await ok.start();
    expect(ok.phase()).toBe('done');

    api = new FakeImageGenClient();
    api.polls = [new Error('network')];
    const lost = run();
    await lost.start();
    expect(lost.phase()).toBe('failed');
    expect(lost.failure()).toContain('O pedido continua lá');
    expect(api.polled).toHaveLength(3);
  });

  describe('"Cancelar" and "Parar de esperar" (the same call; the words come from what the server answers)', () => {
    it('before the request left: the slot goes back and the note says so', async () => {
      let release!: () => void;
      api.hold = new Promise<void>((r) => (release = r));
      api.polls = [done()];
      const r = run();
      const finished = r.start();
      await vi.waitFor(() => expect(r.phase()).toBe('waiting'));
      await r.cancel();
      release();
      await finished;
      expect(api.canceled).toEqual(['gen-1']);
      expect(r.phase()).toBe('canceled');
      expect(r.note()).toBe('Pedido cancelado. A vaga do mês voltou.');
    });

    it('after it left: the wait stops, the slot stays spent and the picture, if it comes, goes to the gallery', async () => {
      let release!: () => void;
      api.hold = new Promise<void>((r) => (release = r));
      api.cancelResult = { generation: generation({ state: ImageGenerationState.CANCELED, slotSpent: true }), status: imageStatus() };
      const r = run();
      const finished = r.start();
      await vi.waitFor(() => expect(r.phase()).toBe('waiting'));
      await r.cancel();
      release();
      await finished;
      expect(r.phase()).toBe('canceled');
      expect(r.note()).toBe('Você parou de esperar. A vaga do mês continua gasta e a imagem, se chegar, vai para a galeria.');
    });

    it('pressed while the app\'s own ask is still on its way: waits for the answer and cancels the request it made, never leaving one behind', async () => {
      let answer!: () => void;
      const asking = new Promise<void>((r) => (answer = r));
      const slow = new ImageRun(
        api as unknown as ImageGenClient,
        'camp-1',
        async () => {
          await asking;
          return api.started as never;
        },
        instant,
      );
      const finished = slow.start();
      await slow.cancel();
      expect(slow.phase()).toBe('canceling');
      expect(api.canceled).toEqual([]);
      answer();
      await finished;
      expect(api.canceled).toEqual(['gen-1']);
      expect(slow.phase()).toBe('canceled');
    });

    it('when the request had already ended by the time the cancel arrived, says what happened (the picture) instead of "canceled"', async () => {
      let release!: () => void;
      api.hold = new Promise<void>((r) => (release = r));
      api.cancelResult = { generation: generation({ state: ImageGenerationState.DONE, imageId: 'img-7' }), status: imageStatus() };
      api.polls = [done('img-7', 1)];
      const r = run();
      const finished = r.start();
      await vi.waitFor(() => expect(r.phase()).toBe('waiting'));
      await r.cancel();
      release();
      await finished;
      expect(r.phase()).toBe('done');
      expect(r.image()?.id).toBe('img-7');
    });
  });
});

describe('ImageRun: an answer lost on the way', () => {
  beforeEach(() => {
    ImageRun.retryDelayMs = 0;
  });
  afterEach(() => {
    ImageRun.retryDelayMs = 1000;
  });

  it('asks once more with the same key when the answer was lost (`unavailable`), and the server makes one request', async () => {
    const api = new FakeImageGenClient();
    api.loseNextAnswer = true;
    api.polls = [done('img-1', 1)];
    const run = new ImageRun(api as unknown as ImageGenClient, 'camp-1', (signal) => api.generateScene('camp-1', { idempotencyKey: 'k-1' } as never, signal), instant);
    await run.start();
    expect(api.keys).toEqual(['k-1', 'k-1']);
    expect(api.slotsSpent).toBe(1);
    expect(run.phase()).toBe('done');
  });

  it('gives up after the second lost answer: the ask is refused, and the key is the dialog\'s to keep', async () => {
    const api = new FakeImageGenClient();
    api.started = new ConnectError('down', Code.Unavailable);
    const run = new ImageRun(api as unknown as ImageGenClient, 'camp-1', (signal) => api.generateScene('camp-1', { idempotencyKey: 'k-2' } as never, signal), instant);
    await expect(run.start()).rejects.toBeInstanceOf(ConnectError);
    // The app asked twice, with the same key, and the key is the dialog's to keep for the next press.
    expect(api.keys).toEqual(['k-2', 'k-2']);
    expect(run.phase()).toBe('sending');
  });

  it('does not ask again for a refusal: a typed answer is an answer', async () => {
    const api = new FakeImageGenClient();
    api.started = new ConnectError('no', Code.FailedPrecondition);
    let calls = 0;
    const counted = new ImageRun(api as unknown as ImageGenClient, 'camp-1', (signal) => (calls++, api.generateScene('camp-1', { idempotencyKey: 'k-3' } as never, signal)), instant);
    await expect(counted.start()).rejects.toBeInstanceOf(ConnectError);
    expect(calls).toBe(1);
  });
});
