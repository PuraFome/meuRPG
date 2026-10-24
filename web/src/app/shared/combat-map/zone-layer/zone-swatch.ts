import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { ZONE_PATTERNS, ZONE_TILE } from '../../../core/combat/zone-pattern';

let nextId = 0;

/**
 * The sample of a zone's pattern, for the legend and the zones list: the same tile the map draws, in a 22 px square,
 * dashed outline for a zone of a hidden caster. Decorative: the words beside it carry the meaning.
 */
@Component({
  selector: 'app-zone-swatch',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg viewBox="0 0 1 1" aria-hidden="true" width="22" height="22" focusable="false">
      <defs>
        <pattern [attr.id]="uid" patternUnits="userSpaceOnUse" [attr.width]="tile" [attr.height]="tile">
          <path [attr.d]="tileOf().d" [class]="'zs__' + tileOf().tint" />
        </pattern>
      </defs>
      <rect class="zs__bg" x="0.04" y="0.04" width="0.92" height="0.92" />
      <rect x="0.04" y="0.04" width="0.92" height="0.92" [attr.fill]="'url(#' + uid + ')'" />
      <rect class="zs__edge" x="0.04" y="0.04" width="0.92" height="0.92" [class.zs__edge--dashed]="dashed()" />
    </svg>
  `,
  styles: `
    :host {
      display: inline-flex;
      flex: none;
    }
    .zs__bg {
      fill: var(--mr-map-token-surface);
    }
    .zs__ink,
    .zs__accent {
      fill: none;
      stroke-linecap: round;
      stroke-width: 0.05;
    }
    .zs__ink {
      stroke: var(--mr-map-token-ink);
    }
    .zs__accent {
      stroke: var(--mr-map-accent);
    }
    .zs__edge {
      fill: none;
      stroke: var(--mr-map-token-ink);
      stroke-width: 0.07;
    }
    .zs__edge--dashed {
      stroke-dasharray: 0.14 0.1;
    }
  `,
})
export class ZoneSwatch {
  /** Index into `ZONE_PATTERNS`. */
  readonly pattern = input.required<number>();
  readonly dashed = input(false);
  protected readonly uid = `zsw-${nextId++}`;
  protected readonly tile = ZONE_TILE;
  protected readonly tileOf = computed(() => ZONE_PATTERNS[this.pattern()] ?? ZONE_PATTERNS[0]);
}
