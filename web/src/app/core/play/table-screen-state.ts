import { computed, signal } from '@angular/core';

import { clockText } from './table-screen-labels';
import { tableScreenErrorKind } from './table-screen-errors';
import { type TvScreenVm } from './table-screen-view';

/** The time source of the TV: the real one in the app, a hand-driven one in specs. */
export interface TvClock {
  now(): Date;
  /** Runs `fn` after `ms`; the returned function cancels it. */
  after(ms: number, fn: () => void): () => void;
}

export const systemClock: TvClock = {
  now: () => new Date(),
  after: (ms, fn) => {
    const timer = setTimeout(fn, ms);
    return () => clearTimeout(timer);
  },
};

/** After a failed first read the TV asks again. */
export const TV_RETRY_MS = 10_000;
/** A screen that lost its connection keeps showing the last good read this long, with a warning. */
export const TV_STALE_MS = 60_000;

/** What the TV page draws: one case per state of the board. */
export type TvPhase =
  | { readonly kind: 'loading'; readonly text: string }
  | { readonly kind: 'error'; readonly title: string; readonly detail: string }
  | {
      readonly kind: 'ready';
      readonly screen: TvScreenVm;
      /** The warning of a connection lost less than a minute ago; `null` while connected. */
      readonly late: string | null;
    }
  | { readonly kind: 'stale'; readonly title: string; readonly detail: string }
  | {
      readonly kind: 'ended';
      readonly title: string;
      readonly farewell: string;
      readonly hint: string;
    }
  | { readonly kind: 'no-access'; readonly title: string; readonly detail: string };

/**
 * The TV's lifecycle as signals: loading, the screen with an optional "late" warning, the hidden screen after a
 * minute without a connection, the end of the session and the refusal. The inputs are the watcher's: a good
 * read, a failed read, the connection coming and going, the end. The 10 s retry and the minute are the
 * clock's, so a spec drives them without timers.
 */
export class TvScreenState {
  private readonly screen = signal<TvScreenVm | null>(null);
  private readonly readFailing = signal(false);
  private readonly lostAt = signal<Date | null>(null);
  private readonly stale = signal(false);
  private readonly endedAt = signal<Date | null>(null);
  private readonly ending = signal(false);
  private readonly refused = signal(false);
  private cancelRetry: (() => void) | null = null;
  private cancelStale: (() => void) | null = null;

  /** `readAgain` is asked after a failed first read, once `TV_RETRY_MS` has passed. */
  constructor(
    private readonly clock: TvClock,
    private readonly readAgain: () => void,
  ) {}

  readonly phase = computed<TvPhase>(() => {
    const screen = this.screen();
    const lostAt = this.lostAt();
    if (this.refused()) {
      return {
        kind: 'no-access',
        title: 'Só o mestre abre a Tela da mesa.',
        detail: 'Ela é a tela da sala; você joga pela sessão no seu aparelho.',
      };
    }
    if (this.ending()) {
      return this.endedPhase(screen);
    }
    if (lostAt && this.stale()) {
      return {
        kind: 'stale',
        title: `Sem conexão desde ${clockText(lostAt)}`,
        detail: 'Esta tela escondeu o que mostrava. Ela volta sozinha quando a conexão voltar.',
      };
    }
    if (screen) {
      return {
        kind: 'ready',
        screen,
        late: lostAt
          ? `Esta tela pode estar atrasada desde ${clockText(lostAt)}. Ela continua assim por até 1 minuto.`
          : null,
      };
    }
    if (this.readFailing()) {
      return {
        kind: 'error',
        title: 'Não deu para abrir a Tela da mesa.',
        detail:
          'Tentando de novo em 10 s. Se não voltar, o mestre abre a tela outra vez pela sessão.',
      };
    }
    return { kind: 'loading', text: 'Carregando a mesa…' };
  });

  private endedPhase(screen: TvScreenVm | null): TvPhase {
    const at = this.endedAt();
    return {
      kind: 'ended',
      title: screen
        ? `A sessão ${screen.sessionNumber} terminou${at ? ` às ${clockText(at)}` : ''}.`
        : 'Não há sessão aberta.',
      farewell: screen ? `Até a próxima sessão de ${screen.campaignName}.` : '',
      hint: 'O mestre pode fechar esta aba.',
    };
  }

  /** A good read: the screen to draw. */
  loaded(screen: TvScreenVm): void {
    this.screen.set(screen);
    this.readFailing.set(false);
    this.dropRetry();
  }

  /** A failed read. With a screen already shown it stays (the connection state speaks); without one, the board's
   * error and a new read in `TV_RETRY_MS`. Refusals and the end are final. */
  readFailed(err: unknown): void {
    const kind = tableScreenErrorKind(err);
    if (kind === 'no-access') {
      this.refuse();
    } else if (kind === 'no-session') {
      this.ended();
    } else if (!this.screen()) {
      this.readFailing.set(true);
      this.dropRetry();
      this.cancelRetry = this.clock.after(TV_RETRY_MS, () => {
        this.cancelRetry = null;
        this.readAgain();
      });
    }
  }

  /** The stream is connected or not. A minute without it hides the screen. */
  connection(connected: boolean): void {
    if (connected) {
      this.cancelStale?.();
      this.cancelStale = null;
      this.lostAt.set(null);
      this.stale.set(false);
      return;
    }
    if (this.lostAt() !== null) {
      return;
    }
    this.lostAt.set(this.clock.now());
    this.cancelStale = this.clock.after(TV_STALE_MS, () => {
      this.cancelStale = null;
      this.stale.set(true);
    });
  }

  /** The session is over. */
  ended(): void {
    this.endedAt.set(this.clock.now());
    this.ending.set(true);
    this.settle();
  }

  /** `permission_denied`, `not_found`: this tab may not see the table. */
  refuse(): void {
    this.refused.set(true);
    this.settle();
  }

  /** Cancels the timers (the page is leaving). */
  dispose(): void {
    this.settle();
  }

  private settle(): void {
    this.dropRetry();
    this.cancelStale?.();
    this.cancelStale = null;
  }

  private dropRetry(): void {
    this.cancelRetry?.();
    this.cancelRetry = null;
  }
}
