import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import type { GearFlow } from '../../../../core/combat/gear-flow';
import { legendOfRows } from '../../../../core/combat/chip-legend';
import { ChipLegend } from '../../../../shared/chip-legend/chip-legend';

type Rows = Parameters<typeof legendOfRows>[0];

/** The line under the "Ataques" tab that says what the chips on its rows mean: only those the attack rows and the fold draw. */
@Component({
  selector: 'app-attack-legend',
  imports: [ChipLegend],
  template: `<app-chip-legend [marks]="legend().marks" [bonus]="legend().bonus" [decorative]="true" />`,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AttackLegend {
  readonly parts = input.required<{ open: Rows; folded: readonly { rows: Rows }[] }>();
  /** The player's gear, when the tab has the fold. */
  readonly flow = input<GearFlow | null>(null);

  protected readonly legend = computed(() => {
    const parts = this.parts();
    const flow = this.flow();
    return legendOfRows(
      [...parts.open, ...parts.folded.flatMap((f) => f.rows)],
      flow ? flow.gear() : [],
      flow?.shieldIdle() ?? false,
    );
  });
}
