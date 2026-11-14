import { computed, signal } from '@angular/core';

import type { RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import type { Attack, PowerAttackOption } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import {
  type TradeOutcome,
  tradeBox,
  tradeRollLabel,
  tradeRollLine,
  tradedBonus,
} from '../../../../core/combat/power-attack-trade';
import { d20Formula } from '../../../../core/combat/roll-mode';
import { article } from '../../../../core/format/article';

/** The trade of attack roll for damage a power attack feature offers on this roll ("Troca do talento"): the trades the
 * weapon line lists, the one ticked (always off at the start: it is never remembered from an attack to the next), and
 * what ticking it changes in the numbers. One at a time: a roll makes one trade. */
export class PowerTrade {
  /** The trades the line lists, with the uses left as the last read says; a read after a refusal replaces them. */
  readonly options = signal<readonly PowerAttackOption[]>([]);
  /** The key ticked; empty for none. */
  readonly pickedKey = signal('');
  /** The trade the roll makes: ticked, and not one that is off. */
  readonly picked = computed(() => {
    const o = this.options().find((x) => x.key === this.pickedKey());
    return o && !o.disabled ? o : undefined;
  });
  /** The key the request sends: only a trade that is ticked and on offer. */
  readonly requestKey = computed(() => this.picked()?.key ?? '');
  /** The attack bonus the roll is made with, the penalty taken off. */
  readonly bonus = computed(() => tradedBonus(this.attack.attackBonus, this.picked()));
  /** What the last roll made of the trade, for the line under its formula. */
  readonly made = signal<PowerAttackOption | null>(null);

  constructor(
    private readonly attack: Attack,
    options: readonly PowerAttackOption[],
  ) {
    this.options.set(options);
  }

  /** Ticks the trade, or unticks it when it was the one ticked; one that is off cannot be ticked. */
  choose(key: string): void {
    const o = this.options().find((x) => x.key === key);
    if (!o || o.disabled) {
      return;
    }
    this.pickedKey.set(this.pickedKey() === key ? '' : key);
  }

  /** The box before the roll: the attack and the damage as they were and as they are with the trade. */
  box(mode: RollMode) {
    const p = this.picked();
    return p ? tradeBox(this.attack, p, mode) : null;
  }

  /** The app's roll button with the trade ticked; `null` with none, so the button says what it always said. */
  rollLabel(mode: RollMode): string | null {
    return this.picked() ? tradeRollLabel(mode, this.bonus()) : null;
  }

  /** The d20 as a formula; for a roll that made a trade the penalty is written apart ("+ 6 − 5") and the line names the target. */
  formula(roll: Parameters<typeof d20Formula>[0], target: string): string {
    const m = this.made();
    return m
      ? `${d20Formula(roll, true, m.penalty)} contra ${article(target)} ${target}`
      : d20Formula(roll, true);
  }

  /** The line under the roll the trade was made on. */
  rollLine(outcome: TradeOutcome): string {
    const m = this.made();
    return m ? tradeRollLine(m, outcome) : '';
  }
}
