import { create } from '@bufbuild/protobuf';
import { type WritableSignal, computed, signal } from '@angular/core';

import {
  type AttackRoll,
  AttackRollSchema,
  type PendingDamage,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { AttackStage } from '../../../../core/combat/attack-flow';
import type { ExtraPick } from '../../../../core/combat/damage-parts';
import { asksLucky } from '../../../../core/held-rolls/held-roll-offer';
import type { AttackSheetData } from './attack-sheet-types';

/** Where the sheet opens: at the held roll's question, at the damage never rolled, at the roll of an opportunity attack
 * (the target is the mover), or at the target. */
function initialStage(data: AttackSheetData): AttackStage {
  if (data.held) {
    return asksLucky(data.held.offer) ? 'lucky' : 'inspire';
  }
  if (data.resume) {
    return 'damage';
  }
  return data.opportunity ? 'roll' : 'target';
}

/** What the attack sheet is showing, shared by the parts that read and write it. */
export class AttackState {
  /** The held roll the sheet opens at (the screen was read again): its question, Lucky first. */
  readonly reopened: AttackSheetData['held'];
  readonly stage: WritableSignal<AttackStage>;
  readonly targetId: WritableSignal<string | null>;
  readonly typing = signal(false);
  readonly busy = signal(false);
  readonly error = signal('');
  readonly roll: WritableSignal<AttackRoll | null>;
  /** The roll waits for an answer: neither a result nor the damage is told. */
  readonly asking = computed(() => this.stage() === 'lucky' || this.stage() === 'inspire');
  readonly pending: WritableSignal<PendingDamage | null>;
  /** The damage rolled, once it is. */
  readonly damage = signal<PendingDamage | null>(null);
  /** The extras marked in the damage step. */
  readonly picks = signal<readonly ExtraPick[]>([]);

  constructor(readonly data: AttackSheetData) {
    this.reopened = data.held;
    this.stage = signal(initialStage(data));
    this.targetId = signal(data.opportunity?.targetId ?? this.reopened?.offer.targetId ?? null);
    this.roll = signal(
      this.reopened
        ? create(AttackRollSchema, {
            attackerId: data.attackerId,
            targetId: this.reopened.offer.targetId,
            attackKey: this.reopened.offer.attackKey,
            d20: this.reopened.offer.d20,
          })
        : null,
    );
    this.pending = signal(data.resume?.pending ?? null);
  }
}
