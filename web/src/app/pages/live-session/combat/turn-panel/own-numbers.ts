import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { freeText } from '../../../../core/combat/cast-flow';
import type { PactSlotsVm, SlotUsageVm } from '../../../../core/play/resource-usage.types';
import { SlotDots } from '../../../../shared/slot-dots/slot-dots';

/** The player's slots as the turn card reads them. */
export interface OwnSlots {
  readonly usage: readonly SlotUsageVm[];
  readonly pact: PactSlotsVm | null;
}

/** The player's hit points as the vitals hold them. */
export interface OwnHitPoints {
  readonly current: number;
  readonly max: number;
}

/**
 * "PV 23 de 23 · CA 12 · 1º ○○○● 2º ○○" (C4-03): the character's own numbers under the turn's title, so the hit points, the armor
 * class and the slots are on the first screen. Only the player's own numbers are here (RN-20); an open ring is a free slot, a
 * crossed disc a used one, and each circle's words ("3 livres de 4") are its accessible name.
 */
@Component({
  selector: 'app-own-numbers',
  imports: [MatIconModule, SlotDots],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (pvText(); as pv) {
      <span class="num"><mat-icon aria-hidden="true">favorite_border</mat-icon><b>PV {{ pv }}</b></span>
    }
    @if (armorClass() !== null) {
      <span class="num"><mat-icon aria-hidden="true">shield</mat-icon>CA {{ armorClass() }}</span>
    }
    @if (rows().length) {
      <span class="num" role="group" aria-label="Espaços de magia">
        <mat-icon aria-hidden="true">auto_fix_high</mat-icon>
        @for (r of rows(); track r.name) {
          <span class="slot" role="img" [attr.aria-label]="r.name + ': ' + r.text">
            <span aria-hidden="true">{{ r.short }}</span>
            <app-slot-dots size="small" [total]="r.total" [used]="r.used" />
          </span>
        }
      </span>
    }
  `,
  styleUrl: './own-numbers.scss',
})
export class OwnNumbers {
  readonly own = input.required<Combatant>();
  readonly armorClass = input<number | null>(null);
  readonly slots = input<OwnSlots>({ usage: [], pact: null });
  /** Where the numbers come from when the combatant has none: the player's vitals. */
  readonly hitPoints = input<OwnHitPoints | null>(null);

  protected readonly pvText = computed(() => {
    const c = this.own();
    if (c.hitPointsCurrent !== undefined && c.hitPointsMax !== undefined) {
      return `${c.hitPointsCurrent} de ${c.hitPointsMax}`;
    }
    const v = this.hitPoints();
    return v ? `${v.current} de ${v.max}` : '';
  });

  protected readonly rows = computed(() => {
    const { usage, pact } = this.slots();
    const rows = usage
      .filter((u) => u.total > 0)
      .map((u) => ({
        name: `${u.level}º nível`,
        short: `${u.level}º`,
        total: u.total,
        used: u.used,
        text: freeText(u.total - u.used, u.total),
      }));
    if (pact && pact.total > 0) {
      rows.push({
        name: `${pact.slotLevel}º nível (pacto)`,
        short: `${pact.slotLevel}º pacto`,
        total: pact.total,
        used: pact.used,
        text: freeText(pact.total - pact.used, pact.total),
      });
    }
    return rows;
  });
}
