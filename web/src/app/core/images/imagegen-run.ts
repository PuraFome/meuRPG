import { computed, signal } from '@angular/core';
import { Code, ConnectError } from '@connectrpc/connect';

import type { GalleryImage } from '../../../gen/meurpg/maps/v1/gallery_pb';
import { type ImageGeneration, ImageGenerationState, type ImageGenerationStatus } from '../../../gen/meurpg/maps/v1/imagegen_pb';
import type { ImageGenClient, Started } from './imagegen-client';
import { failureText } from './imagegen-errors';

/**
 * - `sending`: the app's own call is on its way, the server has not answered yet ("Enviando o pedido…");
 * - `waiting`: the server has the request and keeps a long poll open while the picture is made ("Gerando a imagem…");
 * - `canceling`: "Cancelar" or "Parar de esperar" was pressed and the server has not answered;
 * - `done`: the picture is in the gallery;
 * - `failed`: the service refused or failed, or the wait was lost; `failure` says what to do;
 * - `canceled`: the master stopped waiting; `note` says what became of the slot and the picture.
 */
export type RunPhase = 'sending' | 'waiting' | 'canceling' | 'done' | 'failed' | 'canceled';

/** How long one long poll waits (the server's most). The server keeps the instance busy for exactly this, so no pause between the calls. */
export const POLL_SECONDS = 25;
/** After this the line says the service is slow (E10-07 4). */
export const SLOW_AFTER_SECONDS = 60;
/** A poll that fails (the network) is tried again this many times in a row before the wait is given up. */
const POLL_RETRIES = 3;

/** The words after a "Cancelar" or "Parar de esperar": what became of the slot and of the picture. */
export function canceledNote(generation: Pick<ImageGeneration, 'slotSpent'>): string {
  return generation.slotSpent
    ? 'Você parou de esperar. A vaga do mês continua gasta e a imagem, se chegar, vai para a galeria.'
    : 'Pedido cancelado. A vaga do mês voltou.';
}

/**
 * One request to the image service, from the click to the picture (MR-039, RN-28; E10-07 3 and 4). It asks (`begin`), then keeps one long
 * poll open (`GetImageGeneration` with `wait_seconds`, again and again) until the request leaves PENDING: the Cloud Run instance gets CPU
 * only while a request is in flight, so the poll is also what keeps the generation going. The browser makes nothing itself.
 *
 * "Cancelar" has two meanings and the screen never says one for the other. The app cannot see when the server sends the request out, so it
 * asks the server to cancel and says what came back: `slot_spent` false means the request never left and the slot is back; true means the
 * Google already has it, the slot stays spent and the picture, if it comes, goes to the gallery. A "Cancelar" pressed while the app's own
 * call is still on its way waits for the answer and cancels the request it made, so it never leaves one behind.
 *
 * Plain signals and no DOM: the dialog reads them. `sleep` is only for the pause between failed polls (a test passes an instant one).
 */
export class ImageRun {
  /** How long the app waits before it asks again after a lost answer (a test sets 0). */
  static retryDelayMs = 1000;

  readonly phase = signal<RunPhase>('sending');
  readonly generation = signal<ImageGeneration | null>(null);
  /** The month after the request reserved its slot (and after a cancel gave it back). */
  readonly status = signal<ImageGenerationStatus | null>(null);
  readonly image = signal<GalleryImage | null>(null);
  /** Why it ended without a picture. */
  readonly failure = signal<string | null>(null);
  /** After a cancel: what became of the slot. */
  readonly note = signal<string | null>(null);
  /** Whole seconds since the request went out. */
  readonly seconds = signal(0);
  readonly slow = computed(() => this.phase() === 'waiting' && this.seconds() >= SLOW_AFTER_SECONDS);
  readonly running = computed(() => this.phase() === 'sending' || this.phase() === 'waiting' || this.phase() === 'canceling');

  private readonly abort = new AbortController();
  private cancelWanted = false;
  private timer: ReturnType<typeof setInterval> | undefined;
  private destroyed = false;

  constructor(
    private readonly api: ImageGenClient,
    private readonly campaignId: string,
    private readonly begin: (signal: AbortSignal) => Promise<Started>,
    private readonly sleep: (ms: number) => Promise<void> = (ms) => new Promise((r) => setTimeout(r, ms)),
  ) {}

  /**
   * Asks and waits. Rejects only when the ask itself was refused (nothing was reserved: the dialog goes back to the form with the
   * reason); every later ending is a phase. Resolves when the request has ended.
   */
  async start(): Promise<void> {
    this.timer = setInterval(() => this.seconds.update((s) => s + 1), 1000);
    try {
      const { generation, status } = await this.ask();
      this.generation.set(generation);
      this.status.set(status ?? this.status());
      if (this.cancelWanted) {
        await this.sendCancel();
        return;
      }
      if (generation.state !== ImageGenerationState.PENDING) {
        await this.settle(generation);
        return;
      }
      this.phase.set('waiting');
      await this.wait(generation);
    } catch (err) {
      this.stopClock();
      if (this.phase() === 'sending' || this.phase() === 'canceling') {
        throw err;
      }
      this.fail('Perdemos a conexão com o servidor. O pedido continua lá: se a imagem chegar, ela aparece na galeria.');
    } finally {
      this.stopClock();
    }
  }

  /**
   * The ask, tried once more when the answer was lost on the way (`unavailable`): the same key goes again, so a request the server
   * already made comes back instead of a second one (the server answers a key it knows with the request it made, and no new slot).
   */
  private async ask(): Promise<Started> {
    try {
      return await this.begin(this.abort.signal);
    } catch (err) {
      if (ConnectError.from(err, Code.Unknown).code !== Code.Unavailable || this.cancelWanted) {
        throw err;
      }
      await this.sleep(ImageRun.retryDelayMs);
      return this.begin(this.abort.signal);
    }
  }

  /** "Cancelar" (before the request left) and "Parar de esperar" (after): the same call, told apart by what the server answers. */
  async cancel(): Promise<void> {
    if (!this.running() || this.phase() === 'canceling') {
      return;
    }
    if (this.phase() === 'sending') {
      // The call that asks has not answered: cancel what it made once it does.
      this.cancelWanted = true;
      this.phase.set('canceling');
      return;
    }
    await this.sendCancel();
  }

  /** The dialog closed with the request still going: the poll stops, the request goes on in the server. */
  destroy(): void {
    this.destroyed = true;
    // The call that asks is not cut off while it is still on its way: a request it already made must still be canceled if the master asked.
    if (this.phase() !== 'sending' && this.phase() !== 'canceling') {
      this.abort.abort();
    }
    this.stopClock();
  }

  private async sendCancel(): Promise<void> {
    const generation = this.generation();
    if (!generation) {
      return;
    }
    this.phase.set('canceling');
    this.abort.abort();
    try {
      const done = await this.api.cancel(this.campaignId, generation.id);
      this.generation.set(done.generation);
      this.status.set(done.status ?? this.status());
      if (done.generation.state === ImageGenerationState.CANCELED) {
        this.note.set(canceledNote(done.generation));
        this.phase.set('canceled');
        return;
      }
      // It ended before the cancel arrived (a picture, or a refusal): say that instead.
      await this.settle(done.generation);
    } catch (err) {
      if (ConnectError.from(err).code === Code.NotFound) {
        this.fail('Esse pedido não existe mais.');
      } else {
        this.fail('Não deu para cancelar: perdemos a conexão com o servidor. Veja a galeria daqui a pouco para saber se a imagem chegou.');
      }
    }
  }

  /** One long poll after the other, until the request leaves PENDING. */
  private async wait(first: ImageGeneration): Promise<void> {
    let failures = 0;
    let current = first;
    while (!this.destroyed && this.phase() === 'waiting') {
      try {
        const res = await this.api.poll(this.campaignId, current.id, POLL_SECONDS, this.abort.signal);
        failures = 0;
        this.status.set(res.status ?? this.status());
        if (res.generation) {
          current = res.generation;
          this.generation.set(res.generation);
        }
        if (current.state !== ImageGenerationState.PENDING) {
          await this.settle(current, res.image ?? null);
          return;
        }
      } catch (err) {
        if (this.destroyed || this.phase() !== 'waiting') {
          return;
        }
        if (ConnectError.from(err).code === Code.NotFound) {
          this.fail('Esse pedido não existe mais.');
          return;
        }
        if (++failures >= POLL_RETRIES) {
          this.fail('Perdemos a conexão com o servidor. O pedido continua lá: se a imagem chegar, ela aparece na galeria.');
          return;
        }
        await this.sleep(2000);
      }
    }
  }

  /** The request ended: a picture, or the words for what went wrong. */
  private async settle(generation: ImageGeneration, image: GalleryImage | null = null): Promise<void> {
    this.stopClock();
    switch (generation.state) {
      case ImageGenerationState.DONE: {
        const picture = image ?? (await this.api.poll(this.campaignId, generation.id, 0)).image ?? null;
        if (!picture) {
          this.fail('A imagem foi feita, mas não deu para abri-la. Procure por ela na galeria.');
          return;
        }
        this.image.set(picture);
        this.phase.set('done');
        return;
      }
      case ImageGenerationState.CANCELED:
        this.note.set(canceledNote(generation));
        this.phase.set('canceled');
        return;
      default:
        this.fail(failureText(generation));
    }
  }

  private fail(text: string): void {
    this.stopClock();
    this.failure.set(text);
    this.phase.set('failed');
  }

  private stopClock(): void {
    if (this.timer !== undefined) {
      clearInterval(this.timer);
      this.timer = undefined;
    }
  }
}
