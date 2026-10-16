import { Injector, afterNextRender, signal } from '@angular/core';
import type { Observable } from 'rxjs';

import type { ImageGenerationStatus } from '../../../gen/meurpg/maps/v1/imagegen_pb';
import { newRequestKey } from '../../core/images/bytes-url';
import type { ImageGenClient } from '../../core/images/imagegen-client';
import { blockedOf, invalidField } from '../../core/images/imagegen-errors';
import { ImageRun } from '../../core/images/imagegen-run';
import type { RunFor, Stage } from './image-generate.types';

/** What the flow tells the dialog, and what it asks of it. */
export interface RunFlowDeps {
  readonly api: ImageGenClient;
  readonly campaignId: string;
  readonly injector: Injector;
  readonly stage: () => Stage;
  /** The request is in the air: the dialog shows the wait. */
  readonly started: () => void;
  /** The server's count of the month, as the request saw it. */
  readonly status: (s: ImageGenerationStatus) => void;
  /** The picture came back. */
  readonly done: (run: ImageRun) => void;
  /** A request that ended without a picture: the words, and whether it failed (or was canceled). */
  readonly ended: (text: string, failed: boolean) => void;
  readonly finish: () => void;
}

/** The request in the air: its run, its idempotency key and the question that closing the dialog asks while it goes. */
export class RunFlow {
  readonly run = signal<ImageRun | null>(null);
  readonly runFor = signal<RunFor>('generate');
  readonly closing = signal(false);
  /** The key of the try in the air: kept through a lost answer, dropped once the server answered. */
  private pendingKey: { readonly sig: string; readonly key: string } | null = null;

  constructor(private readonly deps: RunFlowDeps) {}

  /** The key of the try with this content: the same while nothing came back and nothing changed; a new one otherwise. */
  keyFor(signature: string): string {
    if (this.pendingKey?.sig !== signature) {
      this.pendingKey = { sig: signature, key: newRequestKey() };
    }
    return this.pendingKey.key;
  }

  /** A typed refusal is an answer: the next try is a new request. A lost answer is not: the key stays, and a retry never spends two slots. */
  dropKeyOnAnswer(err: unknown): void {
    if (blockedOf(err) !== null || invalidField(err) !== null) {
      this.pendingKey = null;
    }
  }

  begin(
    ask: (signal: AbortSignal) => ReturnType<ImageGenClient['generateScene']>,
    refused: (err: unknown) => void,
  ): void {
    const run = new ImageRun(this.deps.api, this.deps.campaignId, ask);
    this.closing.set(false);
    this.run.set(run);
    this.deps.started();
    run.start().catch((err: unknown) => {
      // Refused before anything was reserved: back to where it came from, with the reason.
      this.run.set(null);
      refused(err);
    });
  }

  /** The run moved to a new phase. */
  onPhase(run: ImageRun, phase: ReturnType<ImageRun['phase']>): void {
    const s = run.status();
    if (s) {
      this.deps.status(s);
    }
    if (phase !== 'sending') {
      // The server has the request (or the master canceled it): the next try is another request, with its own key.
      this.pendingKey = null;
    }
    if (phase === 'done') {
      this.deps.done(run);
    } else if (phase === 'failed') {
      this.deps.ended(run.failure() ?? '', true);
    } else if (phase === 'canceled') {
      this.deps.ended(run.note() ?? '', false);
    }
  }

  cancel(): void {
    void this.run()?.cancel();
  }

  askClose(): void {
    this.closing.set(true);
    // The question is read at once, and its answers are in the footer.
    afterNextRender(() => document.getElementById('gen-close-t')?.focus(), {
      injector: this.deps.injector,
    });
  }

  keepWaiting(): void {
    this.closing.set(false);
  }

  /** Esc, the ✕ and the backdrop: a request still going asks first (E10-07 4); anything else closes. */
  requestClose(): void {
    if (this.deps.stage() === 'running' && this.run()?.running()) {
      this.askClose();
      return;
    }
    this.deps.finish();
  }

  /** "Parar de esperar" in the question: stops the wait and closes. */
  async stopAndClose(): Promise<void> {
    const run = this.run();
    if (run?.running()) {
      await run.cancel();
    }
    this.deps.finish();
  }
}

/** What a dialog or a bottom sheet reference lets the dialog take over: Esc and the backdrop go through `requestClose`. */
export interface CloseGuard {
  disableClose: boolean | undefined;
  keydownEvents(): Observable<KeyboardEvent>;
  backdropClick(): Observable<MouseEvent>;
}

/** Closing must always go through `requestClose`: a request in the air is asked about first. */
export function guardClose(ref: CloseGuard | null, flow: RunFlow): void {
  if (!ref) {
    return;
  }
  ref.disableClose = true;
  ref
    .keydownEvents()
    .subscribe(
      (e) => e.key === 'Escape' && (flow.closing() ? flow.keepWaiting() : flow.requestClose()),
    );
  ref.backdropClick().subscribe(() => flow.requestClose());
}
