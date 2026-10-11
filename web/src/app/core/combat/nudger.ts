import { DestroyRef, Injectable, inject, signal } from '@angular/core';
import { Code, ConnectError, createClient } from '@connectrpc/connect';

import { PlayService } from '../../../gen/meurpg/play/v1/play_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** How long a character stays "called": the server's own wait (`nudgeCooldown`, 10 s), so the button is back when it would work. */
export const NUDGE_COOLDOWN_MS = 10_000;
/** How long the confirmation (or the refusal) stays in the live region before it is emptied. */
export const NUDGE_MESSAGE_MS = 5_000;

export const NUDGE_DONE = 'Chamado.';
export const NUDGE_WAIT = 'Espere alguns segundos para chamar de novo.';
const NUDGE_NO_PLAYER = 'Esse personagem não tem jogador na mesa.';
const NUDGE_NO_SESSION = 'A sessão não está aberta.';
const NUDGE_FAILED = 'Não foi possível chamar a atenção. Tente de novo.';

/**
 * The master's "Chamar a atenção" (an MSN-style nudge): one call to the server, a confirmation for a polite live region and
 * a wait of ten seconds per character, so a double tap cannot buzz a phone twice. It lives in the root so the combat's
 * order list and the party panel share one wait and one message. The state is in memory only (no Web Storage).
 */
@Injectable({ providedIn: 'root' })
export class Nudger {
  private readonly client = createClient(PlayService, inject(CONNECT_TRANSPORT));
  private readonly timers = new Set<ReturnType<typeof setTimeout>>();
  private messageTimer: ReturnType<typeof setTimeout> | undefined;

  /** Characters called in the last ten seconds: their button is off. */
  readonly cooling = signal<ReadonlySet<string>>(new Set());
  /** What the live region says: "Chamado.", or why it was refused; empty when there is nothing to say. */
  readonly message = signal('');
  /** The character `message` is about, so only that character's button says it. */
  readonly messageFor = signal('');

  constructor() {
    inject(DestroyRef).onDestroy(() => {
      this.timers.forEach((t) => clearTimeout(t));
      clearTimeout(this.messageTimer);
    });
  }

  /** Calls the player of `characterId`. Never throws: the outcome is in `message`. */
  async call(campaignId: string, characterId: string): Promise<void> {
    if (this.cooling().has(characterId)) {
      return;
    }
    // Off at once, before the answer: a second tap while the call is in the air must not send another.
    this.cool(characterId);
    try {
      await this.client.nudgePlayer({ campaignId, characterId });
      this.say(characterId, NUDGE_DONE);
    } catch (err) {
      const code = ConnectError.from(err).code;
      if (code !== Code.ResourceExhausted) {
        this.uncool(characterId);
      }
      this.say(characterId, refusal(code));
    }
  }

  private cool(characterId: string): void {
    this.cooling.update((s) => new Set(s).add(characterId));
    const timer = setTimeout(() => {
      this.timers.delete(timer);
      this.uncool(characterId);
    }, NUDGE_COOLDOWN_MS);
    this.timers.add(timer);
  }

  private uncool(characterId: string): void {
    this.cooling.update((s) => {
      const next = new Set(s);
      next.delete(characterId);
      return next;
    });
  }

  private say(characterId: string, text: string): void {
    this.messageFor.set(characterId);
    this.message.set(text);
    clearTimeout(this.messageTimer);
    this.messageTimer = setTimeout(() => {
      this.message.set('');
      this.messageFor.set('');
    }, NUDGE_MESSAGE_MS);
  }
}

function refusal(code: Code): string {
  switch (code) {
    case Code.ResourceExhausted:
      return NUDGE_WAIT;
    case Code.NotFound:
      return NUDGE_NO_PLAYER;
    case Code.FailedPrecondition:
      return NUDGE_NO_SESSION;
    default:
      return NUDGE_FAILED;
  }
}
