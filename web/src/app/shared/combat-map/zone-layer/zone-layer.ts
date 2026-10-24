import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { cellsPath, outlinePath } from '../../../core/combat/spell-area';
import { ZONE_PATTERNS, ZONE_TILE, zonePattern } from '../../../core/combat/zone-pattern';
import type { ZoneView } from '../../../core/combat/zones';

let nextId = 0;

interface Drawn {
  readonly id: string;
  readonly fill: string;
  readonly edge: string;
  readonly pattern: number;
  readonly tint: 'ink' | 'accent';
  readonly dashed: boolean;
  readonly label: string;
  readonly concentration: boolean;
  /** Where the label sits, in grid units: over the top edge, or under the bottom one at the top of the map. */
  readonly x: number;
  readonly y: number;
  readonly below: boolean;
}

/**
 * The spell zones on the battle map (RN-37): each footprint drawn from the squares the server sent (already cut by walls),
 * with a line pattern per spell, an outline with a white halo and a label "Teia · Bartolo". A zone of a hidden caster has a
 * dashed outline. It sits among the map's own marks, under the tokens, and takes no pointer. The drawing is decorative
 * (`aria-hidden`): the zones list in the combat panel is the accessible version. It draws what the server sent, nothing more.
 */
@Component({
  selector: 'app-zone-layer',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { 'aria-hidden': 'true' },
  templateUrl: './zone-layer.html',
  styleUrl: './zone-layer.scss',
})
export class ZoneLayer {
  readonly columns = input.required<number>();
  readonly rows = input.required<number>();
  readonly zones = input.required<readonly ZoneView[]>();

  protected readonly uid = `zl-${nextId++}`;
  protected readonly patterns = ZONE_PATTERNS;
  protected readonly tile = ZONE_TILE;
  protected readonly drawn = computed<Drawn[]>(() =>
    this.zones().flatMap((z) => {
      if (z.squares.length === 0) {
        return [];
      }
      const pattern = zonePattern(z.spellKey);
      const cols = z.squares.map((s) => s.col);
      const rows = z.squares.map((s) => s.row);
      const top = Math.min(...rows);
      const below = top < 1;
      return [
        {
          id: z.id,
          fill: cellsPath(z.squares),
          edge: outlinePath(z.squares),
          pattern,
          tint: ZONE_PATTERNS[pattern].tint,
          dashed: z.dashed,
          label: z.label,
          concentration: z.concentration !== '',
          x: (Math.min(...cols) + Math.max(...cols) + 1) / 2,
          y: below ? Math.max(...rows) + 1 : top,
          below,
        },
      ];
    }),
  );
}
