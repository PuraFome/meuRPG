import { DestroyRef, Injectable, computed, inject, signal } from '@angular/core';

import { sessionClosed } from '../combat/combat-errors';
import { FavorClient } from './favor-client';
import { favorBlocked, favorBlockedMessage } from './favor-errors';

/** What the master's grant says when the session is over: the only thing the board words for it. */
export const FAVOR_NO_SESSION = 'Só numa sessão aberta.';
/** A grant that failed for a reason the server does not name (the network, an error). */
export const FAVOR_GRANT_FAILED = 'Não foi possível dar a Inspiração. Tente de novo.';
/** The reason under a dashed "Dar Inspiração": "Pensantus já tem Inspiração." */
export function favorHoldsText(name: string): string {
  return `${name} já tem Inspiração.`;
}

/** How long the line after a grant stays on the row before it is emptied, in ms. */
export const FAVOR_NOTE_MS = 5_000;

/** A line about one character: what the grant answered. */
export interface FavorNote {
  readonly characterId: string;
  readonly text: string;
  readonly refused: boolean;
}

/**
 * Who has the master's Inspiração in the open session (RN-43), read with `GetFavor` and read again on the hint the server tells:
 * `vitals_changed` (a grant tells nothing else, see "Who hears and who reads" in the favor architecture note), on every
 * `ready` and after a lost hint. The master's read has every holder of the campaign; a player's only their own characters, so the
 * same code shows the master the badges and the player the chip, and never anything about another player (RN-10).
 *
 * A read that fails shows no badge and no chip, and a roll goes on without the Inspiração (`failed` lets its sheet say so). The
 * character sheet provides its own copy and calls `follow`. The live session uses the root copy, so the roll sheets it opens
 * (a dialog or a bottom sheet is built from the root injector, never from the page's) read the same state as the page; it calls
 * `follow` when it opens a session and `stop` when it leaves. `providedIn: 'root'` also keeps a component that is tested alone
 * working, with nobody followed.
 */
@Injectable({ providedIn: 'root' })
export class FavorState {
  private readonly client = inject(FavorClient);

  /** The campaign being followed: empty when there is no open session, and the grant is off with it. */
  readonly campaignId = signal('');
  /** The holders the last read gave, by character id, with the character's name. Empty while the read has not come or failed. */
  readonly holders = signal<ReadonlyMap<string, string>>(new Map());
  /** The last read failed (the network, an error): the roll sheets say so, dashed, instead of hiding the box (board 581-07, item 13).
   * `false` while no read has come yet, and again after a read that works. */
  readonly failed = signal(false);
  /** How many characters have it ("1 com Inspiração" on the master's Grupo tab). */
  readonly count = computed(() => this.holders().size);
  /** The character a player has just received it for, until they dismiss the notice or no longer have it; empty otherwise. */
  readonly gained = signal('');
  /** The characters whose grant is on its way: a second tap does not send another. */
  readonly granting = signal<ReadonlySet<string>>(new Set());
  /** What the last grant answered, for a polite live region on the row of that character. */
  readonly note = signal<FavorNote | null>(null);

  private seq = 0;
  private reading = false;
  private again = false;
  /** The last read that worked, to tell a new holder from one the player already had; `null` until the first one. */
  private lastGood: ReadonlySet<string> | null = null;
  private noteTimer: ReturnType<typeof setTimeout> | undefined;

  constructor() {
    inject(DestroyRef).onDestroy(() => clearTimeout(this.noteTimer));
  }

  /** Whether `characterId` has it, as far as the reader may know. */
  has(characterId: string): boolean {
    return this.holders().has(characterId);
  }

  /** Follows the open session of `campaignId` and reads who has it; the same campaign again just reads again. */
  follow(campaignId: string): void {
    if (campaignId === this.campaignId()) {
      void this.refresh();
      return;
    }
    this.reset();
    this.campaignId.set(campaignId);
    void this.refresh();
  }

  /** The session is over, or the page left: nothing is known and nothing is offered. */
  stop(): void {
    this.reset();
    this.campaignId.set('');
  }

  /** Reads who has it. A read already on its way runs once more when it ends, so a burst of hints is one read behind the last one. */
  async refresh(): Promise<void> {
    const campaignId = this.campaignId();
    if (campaignId === '') {
      return;
    }
    if (this.reading) {
      this.again = true;
      return;
    }
    this.reading = true;
    const seq = this.seq;
    try {
      const list = await this.client.read(campaignId);
      if (seq === this.seq) {
        this.take(new Map(list.map((h) => [h.characterId, h.name])));
        this.failed.set(false);
      }
    } catch {
      // A read that fails shows no chip and no badge, and a roll goes on without the Inspiração: its sheet says "Não deu para
      // ler a Inspiração agora" (the box, dashed). The next hint reads again.
      if (seq === this.seq) {
        this.holders.set(new Map());
        this.failed.set(true);
      }
    } finally {
      if (seq === this.seq) {
        this.reading = false;
      }
    }
    if (this.again && seq === this.seq) {
      this.again = false;
      await this.refresh();
    }
  }

  /** The master's grant. Never throws: the answer is in `note`. */
  async grant(characterId: string, name: string): Promise<void> {
    const campaignId = this.campaignId();
    if (campaignId === '') {
      this.say(characterId, FAVOR_NO_SESSION, true);
      return;
    }
    if (this.granting().has(characterId)) {
      return;
    }
    this.granting.update((s) => new Set(s).add(characterId));
    try {
      await this.client.grant(campaignId, characterId, crypto.randomUUID());
      this.say(characterId, `${name} ganhou Inspiração.`, false);
    } catch (err) {
      this.say(characterId, this.refusal(err), true);
    } finally {
      this.granting.update((s) => {
        const next = new Set(s);
        next.delete(characterId);
        return next;
      });
    }
    // A grant, or a refusal because it was already there: either way the master's read is what is true now.
    await this.refresh();
  }

  /** The player closed the notice. */
  dismissGained(): void {
    this.gained.set('');
  }

  private take(next: ReadonlyMap<string, string>): void {
    const before = this.lastGood;
    this.lastGood = new Set(next.keys());
    this.holders.set(next);
    // A holder that was not there at the last read that worked is a grant (not the first read: it had it already).
    if (before !== null) {
      const fresh = [...next.keys()].find((id) => !before.has(id));
      if (fresh !== undefined) {
        this.gained.set(fresh);
      }
    }
    if (this.gained() !== '' && !next.has(this.gained())) {
      this.gained.set('');
    }
  }

  private refusal(err: unknown): string {
    if (sessionClosed(err)) {
      return FAVOR_NO_SESSION;
    }
    const blocked = favorBlocked(err);
    return blocked ? favorBlockedMessage(blocked.reason) : FAVOR_GRANT_FAILED;
  }

  private say(characterId: string, text: string, refused: boolean): void {
    this.note.set({ characterId, text, refused });
    clearTimeout(this.noteTimer);
    this.noteTimer = setTimeout(() => this.note.set(null), FAVOR_NOTE_MS);
  }

  private reset(): void {
    this.seq++;
    this.reading = false;
    this.again = false;
    this.lastGood = null;
    this.holders.set(new Map());
    this.failed.set(false);
    this.gained.set('');
    this.granting.set(new Set());
    clearTimeout(this.noteTimer);
    this.note.set(null);
  }
}
