import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { cellsPath, outlinePath } from '../../../core/combat/spell-area';
import { badgeSpots } from '../../../core/combat/zone-badge';
import {
  arrowTurn,
  BURNING_PATTERN,
  ZONE_PATTERNS,
  ZONE_TILE,
  zonePattern,
  type ZonePatternTile,
} from '../../../core/combat/zone-pattern';
import type { ZoneView } from '../../../core/combat/zones';

let nextId = 0;

interface Burning {
  readonly fill: string;
  readonly edge: string;
  /** Degrees the arrows turn so they point from the wall to this side. */
  readonly turn: number;
}

interface Drawn {
  readonly id: string;
  readonly fill: string;
  readonly edge: string;
  readonly pattern: number;
  readonly look: ZonePatternTile;
  readonly dashed: boolean;
  readonly burning: Burning | null;
  readonly letter: string;
  /** The badge's corner of the footprint, in percent of the map. */
  readonly x: number;
  readonly y: number;
}

/**
 * The spell zones on the battle map (RN-37): each footprint drawn from the squares the server sent (already cut by walls),
 * with a veil (what the zone does to sight), a pattern of its own per spell, an outline with a white halo and, for a wall, the
 * burning side as its own layer drawn from `burningSquares`. No name floats on the map: a letter in a white corner diamond ties
 * each zone to the legend and the list, which carry the name, the caster and the concentration. The badge is kept inside the map.
 * A zone laid over another is drawn over it with both outlines. A zone of a hidden caster has a dashed outline. It sits among the map's
 * own marks, under the tokens, and takes no pointer. The drawing is decorative (`aria-hidden`): the zones list in the combat panel
 * is the accessible version. It draws what the server sent, nothing more.
 */
@Component({
  selector: 'app-zone-layer',
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
  protected readonly burningLook = ZONE_PATTERNS[BURNING_PATTERN];
  protected readonly burningIndex = BURNING_PATTERN;
  protected readonly drawn = computed<Drawn[]>(() => {
    const shown = this.zones().filter((z) => z.squares.length > 0);
    const spots = badgeSpots(
      shown.map((z) => z.squares),
      this.columns(),
      this.rows(),
    );
    return shown.map((z, i) => {
      const pattern = zonePattern(z.spellKey);
      return {
        id: z.id,
        fill: cellsPath(z.squares),
        edge: outlinePath(z.squares),
        pattern,
        look: ZONE_PATTERNS[pattern],
        dashed: z.dashed,
        burning:
          z.burningSquares.length > 0
            ? {
                fill: cellsPath(z.burningSquares),
                edge: outlinePath(z.burningSquares),
                turn: arrowTurn(z.squares, z.burningSquares),
              }
            : null,
        letter: z.letter,
        x: (spots[i].col / this.columns()) * 100,
        y: (spots[i].row / this.rows()) * 100,
      };
    });
  });
}
