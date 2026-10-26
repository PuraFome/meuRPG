import { type WritableSignal, untracked } from '@angular/core';

import {
  CombatLogKind,
  type CombatLogEntry,
  type CombatLogRound,
  PendingDamageStatus,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { article } from '../format/article';

/** The latest attack that hurt the player's own character, as the log tells it to them. */
export interface OwnHit {
  /** The log entry's id, so a hit is announced once. */
  readonly id: string;
  /** "A Espada voadora 2 acertou você: 9 de dano cortante." */
  readonly text: string;
}

const MS = 1000;
const NS_PER_MS = 1_000_000;

/** Whether the entry is an attack whose damage was applied to `ownId` (a miss, a roll still to come, a heal and the master's
 * `HIT_POINTS_ADJUSTED` are not). */
function hurts(e: CombatLogEntry, ownId: string): boolean {
  const d = e.damage;
  return (
    e.kind === CombatLogKind.ATTACK &&
    e.targetId === ownId &&
    !!d &&
    d.status === PendingDamageStatus.APPLIED &&
    !d.healing &&
    d.amount > 0
  );
}

const atMs = (e: CombatLogEntry) =>
  Number(e.at?.seconds ?? 0) * MS + (e.at?.nanos ?? 0) / NS_PER_MS;

/** "dano cortante", "dano de fogo". */
function damageWords(e: CombatLogEntry): string {
  const name = e.damage?.damageTypePt.toLowerCase() ?? '';
  if (!name) {
    return 'dano';
  }
  return /^(cortante|perfurante|contundente)$/.test(name) ? `dano ${name}` : `dano de ${name}`;
}

/**
 * The newest attack of the log whose damage was applied to the combatant `ownId`: what the player's sheet says when their hit
 * points drop, so the damage reads as the attack's and not as the master's own adjustment. `null` when no attack hurt them.
 */
export function ownHitOf(rounds: readonly CombatLogRound[], ownId: string): OwnHit | null {
  const hits = ownId ? rounds.flatMap((r) => r.entries).filter((e) => hurts(e, ownId)) : [];
  if (!hits.length) {
    return null;
  }
  const e = hits.reduce((a, b) => (atMs(b) > atMs(a) ? b : a));
  const who = e.actorLabel || 'Alguém';
  return {
    id: e.id,
    text: `${article(who) === 'a' ? 'A' : 'O'} ${who} acertou você: ${e.damage?.amount} de ${damageWords(e)}.`,
  };
}

/** The page's sheet says the damage of an attack as the attack's: puts the newest hit of the log on the combat state (none for
 * the master, who has no own character here). */
export function publishOwnHit(
  state: { readonly ownHit: WritableSignal<OwnHit | null> },
  rounds: readonly CombatLogRound[],
  ownId: string,
): void {
  untracked(() => state.ownHit.set(ownHitOf(rounds, ownId)));
}
