import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { type MapPoint, MapPointKind, TrapState } from '../../../gen/meurpg/maps/v1/maps_pb';
import { type PinArea, trapArea } from '../../core/traps/trap-area';
import { bpToPercent } from '../map-view/map-geometry';

interface Pin {
  readonly id: string;
  readonly kind: 'trap' | 'treasure' | 'light';
  /** The mark's own state, for its look: `armed`, `fired`, `disarmed`; `hidden`, `found`; `''`. */
  readonly state: string;
  readonly area: PinArea | null;
  readonly x: number;
  readonly y: number;
  /** The master sees it and the players do not: the crossed eye. */
  readonly secret: boolean;
  readonly remembered: boolean;
  readonly icon: string;
  /** How many marks share its square before it: side by side, not stacked (a trapped chest). */
  readonly n: number;
}

/**
 * The trap, treasure and light marks of a map (MAP-LANGUAGE.md "Points"), drawn over the image
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
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @for (p of pins(); track p.id) {
      @if (p.area; as a) {
        <span
          class="area"
          [class]="'area area--' + p.state"
          [class.pin--remembered]="p.remembered"
          [style.left.%]="a.left"
          [style.top.%]="a.top"
          [style.width.%]="a.width"
          [style.height.%]="a.height"
        ></span>
      }
      <span
        class="pin"
        [class]="'pin pin--' + p.kind + ' pin--' + p.state"
        [class.pin--remembered]="p.remembered"
        [style.left.%]="p.x"
        [style.top.%]="p.y"
        [style.--n]="p.n"
      >
        <mat-icon class="pin__icon">{{ p.icon }}</mat-icon>
        @if (p.secret) {
          <span class="pin__eye"><mat-icon>visibility_off</mat-icon></span>
        }
      </span>
    }
  `,
  styleUrl: './map-pins.scss',
  host: { 'aria-hidden': 'true' },
})
export class MapPins {
  readonly points = input.required<readonly MapPoint[]>();
  /** The grid: how many squares across and down; 0 when the map has none. */
  readonly columns = input(0);
  readonly rows = input(0);
  readonly isMaster = input(false);

  protected readonly pins = computed<readonly Pin[]>(() => {
    const seen = new Map<string, number>();
    const out: Pin[] = [];
    for (const p of this.points()) {
      const x = bpToPercent(p.xBp);
      const y = bpToPercent(p.yBp);
      const square = `${Math.round(x * 4)}/${Math.round(y * 4)}`;
      const n = seen.get(square) ?? 0;
      const base = { id: p.id, x, y, n, remembered: p.remembered, secret: false, area: null };
      switch (p.kind) {
        case MapPointKind.TRAP: {
          const state = p.trap?.state === TrapState.TRIGGERED ? 'fired' : p.trap?.state === TrapState.DISARMED ? 'disarmed' : 'armed';
          const area = trapArea(p.xBp, p.yBp, p.trap?.areaSize ?? 1, this.columns(), this.rows());
          const secret = this.isMaster() && state === 'armed' && !p.revealed && p.trapRevealedTo.length === 0;
          out.push({ ...base, kind: 'trap', state, area, secret, icon: state === 'disarmed' ? 'check' : 'warning' });
          break;
        }
        case MapPointKind.TREASURE:
          out.push({ ...base, kind: 'treasure', state: p.treasureFoundAt !== undefined ? 'found' : 'hidden', icon: 'inventory_2' });
          break;
        case MapPointKind.LIGHT:
          if (this.isMaster()) {
            out.push({ ...base, kind: 'light', state: '', icon: 'lightbulb' });
          }
          break;
        default:
          continue;
      }
      seen.set(square, n + 1);
    }
    return out;
  });
}
