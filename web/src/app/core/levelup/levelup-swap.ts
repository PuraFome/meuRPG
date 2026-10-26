import { computed, signal } from '@angular/core';

import type {
  LevelUpNamedKey,
  LevelUpOptions,
  LevelUpSwapGroup,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import type { LevelUpChoicesInit } from './levelup-client';

/**
 * "Trocar uma escolha" at an Ability Score Improvement level (MR-025, the table rule "Trocar uma escolha no aumento
 * de atributo"): one earlier pick of one group given up for another of the same pool. The server lists the groups
 * only where the table plays with the rule; with none, nothing here shows. The swap is optional and is sent only
 * once the group, the pick to give up and the pick to take are all chosen.
 */
export class LevelUpSwap {
  readonly groupKey = signal('');
  readonly fromKey = signal('');
  readonly toKey = signal('');

  readonly groups: () => readonly LevelUpSwapGroup[];
  readonly group = computed(() => this.groups().find((g) => g.group === this.groupKey()) ?? null);

  /** The swap to send, or `undefined` while it is not whole. */
  readonly pick = computed<LevelUpChoicesInit['swapPick']>(() => {
    const g = this.group();
    if (
      !g ||
      !g.picks.some((p) => p.key === this.fromKey()) ||
      !g.pool.some((p) => p.key === this.toKey())
    ) {
      return undefined;
    }
    return { group: g.group, fromKey: this.fromKey(), toKey: this.toKey() };
  });

  /** "Trocar Defesa por Duelismo", or '' while the swap is not whole. */
  readonly summary = computed(() => {
    const g = this.group();
    if (!this.pick() || !g) {
      return '';
    }
    return `Trocar ${nameIn(g.picks, this.fromKey())} por ${nameIn(g.pool, this.toKey())}`;
  });

  /** Some part of the swap was chosen: leaving the level then asks first. */
  readonly started = computed(() => this.groupKey() !== '');

  constructor(options: LevelUpOptions) {
    this.groups = () => options.swapGroups;
  }

  /** Opens a group (a group already open stays as it is); the picks of another group never carry over. */
  setGroup(key: string): void {
    if (key === this.groupKey()) {
      return;
    }
    this.groupKey.set(this.groups().some((g) => g.group === key) ? key : '');
    this.fromKey.set('');
    this.toKey.set('');
  }

  setFrom(key: string): void {
    this.fromKey.set(this.group()?.picks.some((p) => p.key === key) ? key : '');
  }

  setTo(key: string): void {
    this.toKey.set(this.group()?.pool.some((p) => p.key === key) ? key : '');
  }

  /** "Não trocar". */
  clear(): void {
    this.groupKey.set('');
    this.fromKey.set('');
    this.toKey.set('');
  }

  /** Takes over what `other` had chosen, if these options still offer it. */
  adopt(other: LevelUpSwap): void {
    this.setGroup(other.groupKey());
    this.setFrom(other.fromKey());
    this.setTo(other.toKey());
  }
}

function nameIn(list: readonly LevelUpNamedKey[], key: string): string {
  return list.find((p) => p.key === key)?.namePt ?? key;
}
