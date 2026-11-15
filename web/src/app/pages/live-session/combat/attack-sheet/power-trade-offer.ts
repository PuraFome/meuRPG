import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

import type { PowerAttackOption } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { tradeOfferSub, tradeWhy } from '../../../../core/combat/power-attack-trade';
import { OptionCard } from '../../../../shared/option-card/option-card';

/** What the trade does to the roll and to the damage: each number as it was and as it is. */
export interface TradeBox {
  readonly attackBefore: string;
  readonly attackAfter: string;
  readonly damageBefore: string;
  readonly damageAfter: string;
}

/**
 * "Troca do talento" on the roll step of a weapon attack (a `power_attack` effect of the character's features): one
 * check for each trade, "Usar Golpe Pesado", off at the start of every attack. A trade with no use left stays on the list,
 * dashed, with the reason (the pool's name when the trade spends one, else the server's text). With a trade ticked a box shows the attack and the damage as they were
 * (struck) and as they are, in a live region so the new numbers are read when the check changes. What the sheet projects (the refusal of a pool the two checks share) sits between the checks and the box.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-power-trade-offer',
  imports: [OptionCard],
  template: `
    <fieldset class="offer">
      <legend class="offer__legend">Troca do talento</legend>
      @for (t of trades(); track t.key) {
        <app-option-card
          [title]="'Usar ' + t.name"
          [sub]="sub(t)"
          [why]="why(t)"
          [checked]="pickedKey() === t.key"
          (chosen)="chosen.emit(t.key)"
        />
      }
    </fieldset>
    <ng-content />
    <div aria-live="polite">
      @if (box(); as b) {
        <dl class="box">
          <div class="box__row">
            <dt>Ataque</dt>
            <dd>
              <span class="mr-visually-hidden">era </span><s>{{ b.attackBefore }}</s>
              <span class="box__arrow" aria-hidden="true">→</span>
              <span class="mr-visually-hidden">fica </span><b>{{ b.attackAfter }}</b>
            </dd>
          </div>
          <div class="box__row">
            <dt>Dano</dt>
            <dd>
              <span class="mr-visually-hidden">era </span><s>{{ b.damageBefore }}</s>
              <span class="box__arrow" aria-hidden="true">→</span>
              <span class="mr-visually-hidden">fica </span><b>{{ b.damageAfter }}</b>
            </dd>
          </div>
        </dl>
      }
    </div>
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
    }

    .offer {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);
      min-width: 0;
      margin: 0;
      padding: 0;
      border: 0;
    }

    .offer__legend {
      padding: 0 0 var(--mr-space-2);
      font-size: var(--mr-font-15);
      font-weight: 700;
      line-height: 20px;
    }

    .box {
      display: flex;
      flex-direction: column;
      gap: 6px;
      margin: 0;
      padding: 12px 16px;
      border: 1px solid var(--mr-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-ground);
      font-size: var(--mr-font-15);
      line-height: 20px;
    }

    .box__row {
      display: flex;
      flex-wrap: wrap;
      gap: 2px 12px;
    }

    dt {
      flex: none;
      min-width: 48px;
      color: var(--mr-ink-muted);
    }

    dd {
      display: flex;
      flex-wrap: wrap;
      align-items: baseline;
      gap: 2px 6px;
      min-width: 0;
      margin: 0;
    }

    s {
      color: var(--mr-ink-muted);
    }

    b {
      font-weight: 700;
    }
  `,
})
export class PowerTradeOffer {
  /** The trades the weapon line lists. */
  readonly trades = input<readonly PowerAttackOption[]>([]);
  /** The key ticked; empty for none. */
  readonly pickedKey = input('');
  /** The numbers as they were and as they are with the trade ticked; `null` with none. */
  readonly box = input<TradeBox | null>(null);
  /** The key of the trade whose check was pressed (ticks it, or takes it off). */
  readonly chosen = output<string>();

  protected sub(t: PowerAttackOption): string {
    return tradeOfferSub(t);
  }

  protected why(t: PowerAttackOption): string {
    return tradeWhy(t);
  }
}
