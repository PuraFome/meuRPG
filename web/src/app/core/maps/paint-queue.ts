import { signal } from '@angular/core';

import type { MapLayer } from '../../../gen/meurpg/maps/v1/maps_pb';
import type { Square } from '../combat/combat-grid';

/** The server takes at most this many squares a call (`PaintMapCells`). */
export const MAX_PAINT_BATCH = 400;

/** `saved`: nothing waits ("Tudo salvo"); `saving`: strokes wait or are on their way; `error`: a batch was refused. */
export type PaintSaveStatus = 'saved' | 'saving' | 'error';

interface Batch {
  readonly layer: MapLayer;
  readonly value: number;
  readonly squares: Map<number, Square>;
}

/**
 * The strokes of the master's painting, sent in batches (MR-034, E9-01): every square of a drag joins the
 * batch of its layer and value, and the batch goes out a moment later (or at once on `flush()`, at the end of a
 * stroke), at most 400 squares a call and one call at a time, in the order of the strokes, so the last write wins
 * as the server's rule says. `status` is the "Tudo salvo" tag. A refused batch stays at the head of the queue and
 * `retry()` sends it again. Plain TypeScript with a timer, so the batching is tested with fake time.
 */
export class PaintQueue {
  readonly status = signal<PaintSaveStatus>('saved');
  /** Why the last batch was refused, from `fail`; `null` otherwise. */
  readonly failure = signal<unknown>(null);

  private readonly batches: Batch[] = [];
  private timer: ReturnType<typeof setTimeout> | undefined;
  private running: Promise<void> | null = null;

  constructor(
    private readonly send: (layer: MapLayer, value: number, squares: readonly Square[]) => Promise<void>,
    private readonly delayMs = 150,
  ) {}

  /** Whether strokes wait or are being saved: the editor does not read the layers again meanwhile. */
  get busy(): boolean {
    return this.batches.length > 0 || this.running !== null;
  }

  add(layer: MapLayer, value: number, squares: readonly Square[]): void {
    if (squares.length === 0) {
      return;
    }
    const last = this.batches[this.batches.length - 1];
    const batch =
      last && last.layer === layer && last.value === value && !(this.running && this.batches.length === 1)
        ? last
        : this.openBatch(layer, value);
    for (const s of squares) {
      batch.squares.set(key(s), s);
    }
    this.status.set('saving');
    this.failure.set(null);
    clearTimeout(this.timer);
    this.timer = setTimeout(() => void this.flush(), this.delayMs);
  }

  private openBatch(layer: MapLayer, value: number): Batch {
    const batch: Batch = { layer, value, squares: new Map() };
    this.batches.push(batch);
    return batch;
  }

  /** Sends everything that waits, now. Resolves when the queue is empty or a batch was refused. */
  flush(): Promise<void> {
    clearTimeout(this.timer);
    this.running ??= this.drain().finally(() => {
      this.running = null;
    });
    return this.running;
  }

  /** Sends the refused batch again. */
  retry(): Promise<void> {
    if (this.status() === 'error') {
      this.status.set('saving');
      this.failure.set(null);
    }
    return this.flush();
  }

  private async drain(): Promise<void> {
    while (this.batches.length > 0) {
      const batch = this.batches[0];
      const squares = [...batch.squares.values()];
      for (let i = 0; i < squares.length; i += MAX_PAINT_BATCH) {
        try {
          await this.send(batch.layer, batch.value, squares.slice(i, i + MAX_PAINT_BATCH));
        } catch (err) {
          // What was sent stays sent: the head keeps only what is left.
          const left = new Map<number, Square>();
          for (const s of squares.slice(i)) {
            left.set(key(s), s);
          }
          this.batches[0] = { layer: batch.layer, value: batch.value, squares: left };
          this.failure.set(err);
          this.status.set('error');
          return;
        }
      }
      this.batches.shift();
    }
    this.status.set('saved');
  }
}

function key(s: Square): number {
  return s.row * 65_536 + s.col;
}
