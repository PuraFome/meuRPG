import { computed, signal } from '@angular/core';

import { RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import type { TargetInReach } from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { ModeChoice } from '../../../../core/combat/combat-client-types';
import { d20Count, modeStatus, orNormal } from '../../../../core/combat/roll-mode';

/**
 * The master's choice of how an NPC's d20 is rolled, as the player's attack dialog has it: the mode the server suggests
 * for the target, the one the master picked (any mode, with a reason when it differs) and what the roll sends.
 */
export class NpcRollMode {
  /** The mode the master picked; null follows the server's suggestion. */
  readonly picked = signal<RollMode | null>(null);
  readonly reason = signal('');
  readonly suggested = computed(() => orNormal(this.target()?.rollMode ?? RollMode.NORMAL));
  readonly sources = computed(() => this.target()?.sources ?? []);
  readonly criticalOnHit = computed(() => this.target()?.criticalOnHit ?? false);
  readonly chosen = computed<RollMode>(() => this.picked() ?? this.suggested());
  /** The target is known and the mode is settled (a changed mode needs its reason). */
  readonly ready = computed(
    () =>
      !this.target() ||
      modeStatus('free', this.suggested(), this.chosen(), this.reason(), null) === 'ready',
  );
  readonly faceCount = computed(() => d20Count(this.chosen()));
  readonly combine = computed(() => (this.chosen() === RollMode.DISADVANTAGE ? 'lower' : 'higher'));

  constructor(private readonly target: () => TargetInReach | null) {}

  /** A new attack or target starts from its own suggestion. */
  reset(): void {
    this.picked.set(null);
    this.reason.set('');
  }

  /** What the roll sends about the mode: nothing when the suggestion stands. */
  choice(): ModeChoice | undefined {
    return this.chosen() === this.suggested()
      ? undefined
      : { mode: this.chosen(), reason: this.reason().trim() };
  }
}
