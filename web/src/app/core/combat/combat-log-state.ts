import { computed, signal } from '@angular/core';
import { clone } from '@bufbuild/protobuf';

import {
  CombatLogEntrySchema,
  type CombatLogEntry,
  type CombatLogRound,
  type ListCombatLogResponse,
} from '../../../gen/meurpg/play/v1/combat_pb';
import type { CombatClient } from './combat-client';
import { undoableEntry } from './combat-log';

/** `since_seq` that asks for the whole log. */
const WHOLE_LOG = 0n;

/**
 * The combat log on this screen (`ListCombatLog`): read again after every
 * `combat_log_changed` and every reconnection (the page's `logTick`). It
 * asks only for what is new (`since_seq` = the `last_seq` it holds) and puts
 * the entries that come in place by id; an answer that is `full` replaces
 * everything. It keeps the rounds as the server sent them (the undo needs
 * the structured entry); the panel draws the groups from them.
 */
export class CombatLogState {
  readonly rounds = signal<readonly CombatLogRound[]>([]);
  /** The event `UndoLastAction` would take back; only the master gets it. */
  readonly undoableId = signal('');
  readonly loaded = signal(false);
  readonly undoable = computed(() => undoableEntry(this.rounds()));
  private asked = 0;
  /** The `last_seq` of what the page holds, and the combat it belongs to. */
  private lastSeq = WHOLE_LOG;
  private heldFor = '';
  /** A whole read was asked for and none has been applied yet (a later read may have replaced the one that carried it). */
  private needFull = false;

  async load(
    api: CombatClient,
    campaignId: string,
    encounterId: string,
    full = false,
  ): Promise<void> {
    const mine = ++this.asked;
    this.needFull ||= full;
    if (encounterId !== this.heldFor) {
      this.needFull = true;
    }
    const wholeLog = this.needFull || !this.loaded();
    try {
      const res = await api.turn.log(campaignId, encounterId, wholeLog ? WHOLE_LOG : this.lastSeq);
      if (mine === this.asked) {
        this.apply(res, wholeLog, encounterId);
      }
    } catch {
      // Best effort: the next `combat_log_changed` reads it again.
    }
  }

  private apply(res: ListCombatLogResponse, asked: boolean, encounterId: string): void {
    const whole = res.full || asked;
    this.rounds.set(whole ? res.rounds : mergeRounds(this.rounds(), res.rounds));
    // The flag of a held entry may be old: it is right only on the entry this answer sends.
    this.rounds.update((rounds) => withUndoable(rounds, res));
    this.undoableId.set(res.undoableEventId);
    this.lastSeq = res.lastSeq;
    this.heldFor = encounterId;
    this.needFull = false;
    this.loaded.set(true);
  }

  clear(): void {
    this.asked++;
    this.rounds.set([]);
    this.undoableId.set('');
    this.loaded.set(false);
    this.lastSeq = WHOLE_LOG;
    this.heldFor = '';
    this.needFull = false;
  }
}

/** The held rounds with the entries of a delta put in place: an entry with a known id replaces that one where it stands, a new one
 * goes before the first held entry with a lower `seq` (the rounds and their entries are latest first). */
function mergeRounds(
  held: readonly CombatLogRound[],
  delta: readonly CombatLogRound[],
): CombatLogRound[] {
  const rounds = held.map((r) => ({ ...r, entries: [...r.entries] }));
  for (const d of delta) {
    for (const entry of d.entries) {
      for (const r of rounds) {
        const at = r.entries.findIndex((e) => e.id === entry.id);
        if (at >= 0) {
          r.entries.splice(at, 1);
        }
      }
    }
  }
  for (const d of delta) {
    let round = rounds.find((r) => r.round === d.round);
    if (!round) {
      round = { ...d, entries: [] };
      const after = rounds.findIndex((r) => r.round < d.round);
      rounds.splice(after < 0 ? rounds.length : after, 0, round);
    }
    for (const entry of d.entries) {
      const at = round.entries.findIndex((e) => e.seq < entry.seq);
      round.entries.splice(at < 0 ? round.entries.length : at, 0, entry);
    }
  }
  return rounds.filter((r) => r.entries.length > 0) as CombatLogRound[];
}

/** The rounds with `undoable` set on the entries this answer sent as undoable, and only if the answer names an undoable event. */
function withUndoable(
  rounds: readonly CombatLogRound[],
  res: ListCombatLogResponse,
): CombatLogRound[] {
  const sent = new Set<string>();
  if (res.undoableEventId !== '') {
    for (const r of res.rounds) {
      for (const e of r.entries) {
        if (e.undoable) {
          sent.add(e.id);
        }
      }
    }
  }
  return rounds.map((r) => ({
    ...r,
    entries: r.entries.map((e) => withFlag(e, sent.has(e.id))),
  })) as CombatLogRound[];
}

function withFlag(e: CombatLogEntry, undoable: boolean): CombatLogEntry {
  if (e.undoable === undoable) {
    return e;
  }
  const copy = clone(CombatLogEntrySchema, e);
  copy.undoable = undoable;
  return copy;
}
