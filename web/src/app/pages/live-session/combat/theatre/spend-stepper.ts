import { Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { type Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { clampSpend, spendPlan, stepSpend } from '../../../../core/combat/theatre';

let nextId = 0;

/**
 * The amount of "Gastar movimento" (RN-25, E10-04): a minus, the amount in meters with one decimal and a plus, in steps of 1,5 m (5 ft).
 * The plus stops at what is left and goes dashed there; the minus stops at one step. It does no rule: what is left is the server's
 * (`movement_left_*`), and the server refuses what is too far. Both the master's row and the player's sheet use it, with the amount held by
 * the caller (`ft`, two-way) so a refresh of the combat never changes what was chosen, only the limits.
 */
@Component({
  selector: 'app-spend-stepper',
  imports: [MatIconModule],
  template: `
    <div class="stepper" role="group" [attr.aria-label]="label()">
      <button type="button" class="step" aria-label="Menos 1,5 m" [attr.aria-disabled]="!plan().canLess || busy()" (click)="move(-1)">
        <mat-icon aria-hidden="true">remove</mat-icon>
      </button>
      <output class="amount" [id]="id" aria-live="polite" [attr.aria-label]="'Quanto gastar: ' + plan().amount">{{ plan().amount }}</output>
      <button type="button" class="step" aria-label="Mais 1,5 m" [attr.aria-disabled]="!plan().canMore || busy()" (click)="move(1)">
        <mat-icon aria-hidden="true">add</mat-icon>
      </button>
    </div>
  `,
  host: { '[class.fill]': 'fill()' },
  styles: `
    :host {
      display: inline-block;
    }

    :host(.fill) {
      display: block;
      flex: 1 0 100%;
    }

    :host(.fill) .amount {
      flex: 1;
    }

    .stepper {
      display: flex;
      align-items: center;
      gap: 12px;
    }

    .step {
      display: inline-flex;
      flex: none;
      align-items: center;
      justify-content: center;
      box-sizing: border-box;
      width: 44px;
      height: 44px;
      padding: 0;
      border: 1px solid var(--mr-control-line);
      border-radius: var(--mr-radius-sm);
      background: var(--mr-surface);
      color: var(--mr-ink);
      cursor: pointer;
    }

    // At its limit: dashed and quiet, still focusable (the reason is the line the caller writes near it).
    .step[aria-disabled='true'] {
      border-style: dashed;
      color: var(--mr-ink-muted);
      cursor: default;
    }

    .step:focus-visible {
      outline: 2px solid var(--mr-focus);
      outline-offset: 2px;
    }

    .amount {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      box-sizing: border-box;
      min-width: 84px;
      height: 44px;
      padding: 0 12px;
      border: 1px solid var(--mr-control-line);
      border-radius: var(--mr-radius-sm);
      font-size: 20px;
      font-weight: 700;
      font-variant-numeric: tabular-nums;
    }
  `,
})
export class SpendStepper {
  /** Who spends: its movement left and its speed. */
  readonly who = input.required<Combatant>();
  /** The amount now, in whole feet. */
  readonly ft = input.required<number>();
  readonly label = input('Quanto gastar');
  readonly busy = input(false);
  /** The stepper takes the whole line: the minus, the number and the plus spread to the width of the button under it. */
  readonly fill = input(false);
  readonly ftChange = output<number>();

  protected readonly id = `spend-amount-${nextId++}`;
  protected readonly plan = computed(() => spendPlan(this.who(), this.ft()));

  protected move(direction: -1 | 1): void {
    // At its limit the button stays focusable (aria-disabled), and a press does nothing.
    if (this.busy() || (direction === -1 ? !this.plan().canLess : !this.plan().canMore)) {
      return;
    }
    this.ftChange.emit(stepSpend(this.plan().ft, direction, this.who().movementLeftFt));
  }
}

/** The amount to start from: one step (1,5 m), or what is left when that is less. */
export function startAmount(c: Pick<Combatant, 'movementLeftFt'>): number {
  return clampSpend(5, c.movementLeftFt);
}
