import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import type { Square } from '../../core/combat/combat-grid';
import { cellsPath, outlinePath } from '../../core/combat/spell-area';

/**
 * "Área da última magia" (PM-02c state 9): the squares a player's area spell reached, drawn on the master's map while its
 * question waits, with the point of origin as a diamond. It only draws: no taps, no keys, and the map's own layer takes
 * every pointer under it. It uses the map's colours, the same as the picker's area, so the two read as one thing.
 */
@Component({
  selector: 'app-area-overlay',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg class="ao__cells" aria-hidden="true" preserveAspectRatio="none" [attr.viewBox]="'0 0 ' + columns() + ' ' + rows()">
      <path class="ao__fill" [attr.d]="fillPath()" />
      <path class="ao__edge-halo" [attr.d]="edgePath()" />
      <path class="ao__edge" [attr.d]="edgePath()" />
    </svg>
    @if (origin(); as o) {
      <span class="ao__diamond" aria-hidden="true" [style.left.%]="((o.col + 0.5) / columns()) * 100" [style.top.%]="((o.row + 0.5) / rows()) * 100"></span>
    }
  `,
  styleUrl: './area-overlay.scss',
})
export class AreaOverlay {
  readonly columns = input.required<number>();
  readonly rows = input.required<number>();
  readonly squares = input.required<readonly Square[]>();
  readonly origin = input<Square | null>(null);

  protected readonly fillPath = computed(() => cellsPath(this.squares()));
  protected readonly edgePath = computed(() => outlinePath(this.squares()));
}
