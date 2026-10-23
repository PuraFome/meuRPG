import { signal } from '@angular/core';

import { ActionKey, newKey } from '../connect/idempotency';
import type { FreeDiceClient } from './free-dice-client';
import { freeRollError } from './free-dice-errors';
import {
  DEFAULT_FORM,
  type FreeForm,
  type FreeRollVm,
  type FreeShortcut,
  MINE_LIMIT,
  TABLE_LIMIT,
  formFromRoll,
  mergeRolls,
  normalizeForm,
} from './free-roll';

/** The state of the two lists: `loading` until the first read answers. */
export type ListState = 'loading' | 'ready' | 'failed';

/** What the state needs from its page. */
export interface FreeDiceOptions {
  readonly campaignId: () => string;
  readonly isMaster: () => boolean;
  /** Another person's table roll arrived: the dice tumble on this screen. */
  readonly show: (roll: FreeRollVm) => void;
  readonly now?: () => Date;
  /** Mints the keys (a spec passes a counter). */
  readonly makeKey?: () => string;
}

/**
 * The free dice roller of the session page (MR-052): the form, the roll, the result, the caller's last rolls and the
 * table's log. Pure TypeScript with signals:
 *
 * - one roll at a time (`rolling`): a second tap while the server rolls does nothing, so a double tap rolls once;
 * - the key is made per tap and kept across a retry of the same dice (a lost answer cannot roll twice), and renewed
 *   once a roll worked, so the next roll, even of the same dice, is a new one;
 * - the table's rolls come by the stream: a roll heard while this person's own is on its way waits for the answer,
 *   so the roller never sees their own roll tumble on the table's dice, and a roll already known is ignored;
 * - a read of the lists never drops a roll made while it was on its way (the lists are merged by id).
 */
export class FreeDiceState {
  readonly form = signal<FreeForm>(DEFAULT_FORM);
  /** The roll the sheet shows as the result card; `null` while the form is on. */
  readonly result = signal<FreeRollVm | null>(null);
  readonly rolling = signal(false);
  /** What to say about a roll that did not happen (empty: nothing). */
  readonly error = signal('');
  readonly table = signal<readonly FreeRollVm[]>([]);
  readonly mine = signal<readonly FreeRollVm[]>([]);
  readonly listState = signal<ListState>('loading');
  /** The clock the lists' "há 3 min" count from: moves with each read and each roll, not by itself. */
  readonly now = signal(new Date());

  private readonly key: ActionKey;
  private readonly clock: () => Date;
  private waiting: FreeRollVm[] = [];
  /** Moves with each read and with `clear()`: an answer that comes for an older one is dropped. */
  private generation = 0;
  /** The roll in the air, so a second tap joins it instead of starting another. */
  private inFlight: Promise<void> | null = null;

  constructor(
    private readonly api: Pick<FreeDiceClient, 'list' | 'roll'>,
    private readonly options: FreeDiceOptions,
  ) {
    this.key = new ActionKey(options.makeKey ?? newKey);
    this.clock = options.now ?? (() => new Date());
    this.now.set(this.clock());
  }

  /** Reads the table's log and the caller's own rolls (on every `ready`, and "Tentar de novo"). */
  async refresh(): Promise<void> {
    const generation = this.generation;
    if (this.table().length === 0 && this.mine().length === 0) {
      this.listState.set('loading');
    }
    try {
      const lists = await this.api.list(this.options.campaignId());
      if (generation !== this.generation) {
        return;
      }
      this.table.set(mergeRolls(lists.table, this.table(), TABLE_LIMIT));
      this.mine.set(mergeRolls(lists.mine, this.mine(), MINE_LIMIT));
      this.listState.set('ready');
      this.now.set(this.clock());
    } catch {
      if (generation === this.generation) {
        // What was on screen stays; the form still rolls.
        this.listState.set('failed');
      }
    }
  }

  /** Another session, or none: both lists start empty. */
  clear(): void {
    this.generation++;
    this.table.set([]);
    this.mine.set([]);
    this.waiting = [];
    this.result.set(null);
    this.error.set('');
    this.listState.set('loading');
    this.form.set(DEFAULT_FORM);
  }

  /** The person changed a field of the form. */
  change(patch: Partial<FreeForm>): void {
    this.form.update((f) => normalizeForm({ ...f, ...patch }));
  }

  /** A shortcut chip fills the dice and keeps the modifier. */
  shortcut(shortcut: FreeShortcut): void {
    this.change({ sides: shortcut.sides, count: shortcut.count, mode: 'normal' });
  }

  /** "Mudar dados": back to the form, with the dice as they were. */
  backToForm(): void {
    this.result.set(null);
    this.error.set('');
  }

  /** The big button. */
  roll(): Promise<void> {
    this.inFlight ??= this.rollNow().finally(() => (this.inFlight = null));
    return this.inFlight;
  }

  /** "Rolar … de novo": the same dice, a new roll. */
  again(): Promise<void> {
    return this.roll();
  }

  /** "Repetir": the form takes that roll's dice and rolls at once. */
  repeat(roll: FreeRollVm): Promise<void> {
    if (this.rolling()) {
      return this.roll();
    }
    this.form.set(formFromRoll(roll));
    return this.roll();
  }

  /** The stream says someone rolled: the table's log gets it, and the dice tumble unless it is already known. */
  heard(roll: FreeRollVm): void {
    if (this.rolling()) {
      this.waiting.push(roll);
      return;
    }
    if (this.table().some((r) => r.id === roll.id)) {
      return;
    }
    this.table.update((list) => [roll, ...list].slice(0, TABLE_LIMIT));
    this.options.show(roll);
  }

  private async rollNow(): Promise<void> {
    const f = this.form();
    // A player has no "Só eu": the server would refuse it, and the form never asks.
    const request = {
      campaignId: this.options.campaignId(),
      sides: f.sides,
      count: f.count,
      modifier: f.modifier,
      mode: f.mode,
      secret: f.secret && this.options.isMaster(),
    };
    this.rolling.set(true);
    this.error.set('');
    try {
      const roll = await this.api.roll({ ...request, idempotencyKey: this.key.keyFor(request) });
      this.key.renew();
      this.result.set(roll);
      this.table.update((list) => mergeKnown(roll, list, TABLE_LIMIT));
      this.mine.update((list) => mergeKnown(roll, list, MINE_LIMIT));
      this.now.set(this.clock());
    } catch (err) {
      this.error.set(freeRollError(err));
    } finally {
      this.rolling.set(false);
      this.flush();
    }
  }

  /** The rolls heard while this person's own was on its way, now that its answer is in. */
  private flush(): void {
    const heard = this.waiting;
    this.waiting = [];
    heard.forEach((roll) => this.heard(roll));
  }
}

/** The roll first, once, and the list cut to `limit`. */
function mergeKnown(roll: FreeRollVm, list: readonly FreeRollVm[], limit: number): FreeRollVm[] {
  return [roll, ...list.filter((r) => r.id !== roll.id)].slice(0, limit);
}
