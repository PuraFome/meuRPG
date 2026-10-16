import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { MapPoint } from '../../../gen/meurpg/maps/v1/maps_pb';
import { ChestIcon } from '../chest-icon/chest-icon';
import { type Pin, buildPins } from './map-pins-build';

/**
 * The trap, treasure and light marks of a map (docs/design/README.md), drawn over the image
 * inside the map's stage so they pan and zoom with it:
 * - **Armadilha:** a red dashed border around its area and the warning glyph; for the master, an
 *   unrevealed one adds the crossed eye ("Só você vê"). A fired one has a solid border, a disarmed
 *   one a grey one. A player gets only the traps they know, so this draws what it is given.
 * - **Tesouro:** the chest; dashed while hidden (the master's), solid once found (everyone's).
 * - **Luz:** the bulb, the master's.
 * A remembered point (fog, 9.13) is darkened like the squares around it.
 *
 * Presentational and `aria-hidden`: the lists beside the map say the same in words, and the legend
 * under it names every mark. The fog drawing (9.13) hosts it the same way: it only needs the points
 * the server sent.
 */
@Component({
  selector: 'app-map-pins',
  imports: [ChestIcon, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @for (p of pins(); track p.id) {
      @if (p.kind === 'trap') {
        <!-- One drawing in every state (docs/design/README.md): a red dashed border around the area and the warning glyph, smaller than the
             square; what only the master sees adds the crossed eye, at the corner and never on the glyph. -->
        <span
          class="area"
          [class.area--selected]="p.id === selectedId()"
          [attr.data-pin-of]="p.id"
          [class.pin--remembered]="p.remembered"
          [style.left.%]="p.area?.left ?? p.x"
          [style.top.%]="p.area?.top ?? p.y"
          [style.width.%]="p.area?.width ?? 0"
          [style.height.%]="p.area?.height ?? 0"
          [class.area--none]="!p.area"
        >
          <mat-icon class="area__glyph">warning</mat-icon>
          @if (p.secret) {
            <span class="area__eye"><mat-icon>visibility_off</mat-icon></span>
          }
        </span>
      } @else {
        <span
          class="pin"
          [attr.data-pin-of]="p.id"
          [class]="'pin pin--' + p.kind + ' pin--' + p.state"
          [class.pin--remembered]="p.remembered"
          [class.pin--selected]="p.id === selectedId()"
          [style.left.%]="p.x"
          [style.top.%]="p.y"
          [style.--n]="p.n"
        >
          @if (p.kind === 'treasure') {
            <app-chest-icon class="pin__icon" />
          } @else {
            <mat-icon class="pin__icon">{{ p.icon }}</mat-icon>
          }
        </span>
      }
    }
  `,
  styleUrl: './map-pins.scss',
  host: { 'aria-hidden': 'true', '[class.faded]': 'faded()' },
})
export class MapPins {
  readonly points = input.required<readonly MapPoint[]>();
  /** The grid: how many squares across and down; 0 when the map has none. */
  readonly columns = input(0);
  readonly rows = input(0);
  readonly isMaster = input(false);
  /** Drawn at 40 %: the master is painting, and the layers show through. */
  readonly faded = input(false);
  /** The point the master has open: its mark gets the selection ring. */
  readonly selectedId = input<string | null>(null);

  protected readonly pins = computed<readonly Pin[]>(() =>
    buildPins(this.points(), {
      isMaster: this.isMaster(),
      columns: this.columns(),
      rows: this.rows(),
    }),
  );
}
