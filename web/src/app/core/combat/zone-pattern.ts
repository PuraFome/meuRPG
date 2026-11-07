import type { Square } from './combat-grid';
import type { ZoneView } from './zones';

/**
 * How each spell's zone looks on the map: a drawing of its own (never colour alone), a tint of the map's own tokens and a veil
 * that says what the zone does to sight. Each pattern is one tile of 0.2 x 0.2 grid squares drawn as a stroked path, so the map
 * and the legend's swatch draw the same thing from the same data. The map's tokens do not follow the theme, so a look reads the
 * same in the light and the dark theme.
 */
export interface ZoneVeil {
  /** Which token tints the veil: the white surface, the dark ink or the garnet accent. */
  readonly tone: 'surface' | 'ink' | 'accent';
  /** How much of the footprint it covers, in percent. */
  readonly pct: number;
}

export interface ZonePatternTile {
  readonly name: string;
  /** The drawing class of the layer: no two layers share one. */
  readonly cls: string;
  /** Stroked with round caps: a dot is a zero-length line. */
  readonly d: string;
  /** Stroke width, in grid squares. */
  readonly stroke: number;
  /** What draws the pattern: the dark ink, the garnet accent, or white (on a dark or garnet veil). */
  readonly tint: 'ink' | 'accent' | 'light';
  /** What draws the outline. */
  readonly edge: 'ink' | 'accent';
  readonly veil: ZoneVeil;
  /** The outline is dashed whatever the caster. */
  readonly dashed: boolean;
}

export const ZONE_TILE = 0.2;

const THIN = 0.045;
const LIGHT: ZoneVeil = { tone: 'surface', pct: 38 };
const CLOUD: ZoneVeil = { tone: 'surface', pct: 72 };

function tile(
  name: string,
  cls: string,
  d: string,
  tint: ZonePatternTile['tint'],
  veil: ZoneVeil = LIGHT,
  more: Partial<Pick<ZonePatternTile, 'edge' | 'dashed' | 'stroke'>> = {},
): ZonePatternTile {
  const edge = more.edge ?? (tint === 'accent' ? 'accent' : 'ink');
  return {
    name,
    cls,
    d,
    tint,
    veil,
    edge,
    stroke: more.stroke ?? THIN,
    dashed: more.dashed ?? false,
  };
}

/** The look of every layer the map draws: the six the app began with, then one for each of the other spells, then the wall's burning side. */
export const ZONE_PATTERNS: readonly ZonePatternTile[] = [
  tile('linhas verticais', 'vlines', 'M0.1 0V0.2', 'ink'),
  tile('pontos', 'dots', 'M0.1 0.1h0.001', 'ink'),
  tile(
    'linhas diagonais',
    'diagonals',
    'M0 0.2L0.2 0M-0.05 0.05L0.05 -0.05M0.15 0.25L0.25 0.15',
    'accent',
  ),
  tile('ondas', 'waves', 'M0 0.1Q0.05 0 0.1 0.1T0.2 0.1', 'ink'),
  tile('triângulos', 'triangles', 'M0.04 0.15L0.1 0.04L0.16 0.15Z', 'accent'),
  tile('linhas horizontais', 'hlines', 'M0 0.1H0.2', 'accent'),
  tile('anéis', 'rings', 'M0.05 0.1a0.05 0.05 0 1 0 0.1 0a0.05 0.05 0 1 0 -0.1 0', 'ink'),
  tile(
    'asteriscos',
    'asterisks',
    'M0.1 0.04V0.16M0.048 0.07L0.152 0.13M0.048 0.13L0.152 0.07',
    'ink',
  ),
  tile('quadradinhos', 'squares', 'M0.05 0.05h0.1v0.1h-0.1Z', 'ink', CLOUD),
  tile('xis', 'crosses', 'M0.05 0.05L0.15 0.15M0.15 0.05L0.05 0.15', 'accent', CLOUD),
  tile(
    'pontos claros',
    'light-dots',
    'M0.05 0.05h0.001M0.15 0.15h0.001',
    'light',
    { tone: 'ink', pct: 80 },
    { edge: 'ink', stroke: 0.05 },
  ),
  tile(
    'tijolos',
    'bricks',
    'M0 0H0.2M0 0.1H0.2M0.1 0V0.1M0 0.1V0.2M0.2 0.1V0.2',
    'light',
    {
      tone: 'accent',
      pct: 72,
    },
    { edge: 'accent', stroke: 0.03 },
  ),
  tile('setas para o lado', 'arrows', 'M0.06 0.04L0.14 0.1L0.06 0.16', 'accent', LIGHT, {
    dashed: true,
  }),
];

/** The layer of a wall's burning side. */
export const BURNING_PATTERN = ZONE_PATTERNS.length - 1;

/** The spells the app keeps, each with a look of its own so no two share one. Any other spell takes the hash below. */
const KNOWN: Readonly<Record<string, number>> = {
  'spell:moonbeam': 0,
  'spell:web': 1,
  'spell:spirit-guardians': 2,
  'spell:cloudkill': 3,
  'spell:grease': 4,
  'spell:entangle': 5,
  'spell:silence': 6,
  'spell:spike-growth': 7,
  'spell:fog-cloud': 8,
  'spell:stinking-cloud': 9,
  'spell:darkness': 10,
  'spell:wall-of-fire': 11,
};

const HASH_BASE = 31;
/** A spell the table defined, or one the app does not know, takes one of the first six looks: none of them hides sight. */
const HASHED = 6;

/** The pattern of a spell's zone: fixed for each spell of the catalog, a hash of the key for the others. */
export function zonePattern(spellKey: string): number {
  const known = KNOWN[spellKey];
  if (known !== undefined) {
    return known;
  }
  let h = 0;
  for (const ch of spellKey) {
    h = (h * HASH_BASE + ch.charCodeAt(0)) >>> 0;
  }
  return h % HASHED;
}

const QUARTER_TURN = 90;
const HALF_TURN = 180;
const THREE_QUARTER_TURN = 270;

/** How far the arrows turn (0, 90, 180 or 270 degrees; 0 points right) so they run from the wall to its burning side. */
export function arrowTurn(wall: readonly Square[], burning: readonly Square[]): number {
  if (wall.length === 0 || burning.length === 0) {
    return 0;
  }
  const mean = (xs: readonly number[]) => xs.reduce((a, b) => a + b, 0) / xs.length;
  const dx = mean(burning.map((s) => s.col)) - mean(wall.map((s) => s.col));
  const dy = mean(burning.map((s) => s.row)) - mean(wall.map((s) => s.row));
  if (Math.abs(dx) >= Math.abs(dy)) {
    return dx >= 0 ? 0 : HALF_TURN;
  }
  return dy >= 0 ? QUARTER_TURN : THREE_QUARTER_TURN;
}

export interface ZoneLegendItem {
  readonly key: string;
  readonly name: string;
  readonly pattern: number;
  /** The letters of the zones of this spell on the map, in the order of the list. */
  readonly letters: readonly string[];
  /** Every zone of this spell on the map is the master's alone. */
  readonly masterOnly: boolean;
}

/** One legend entry per spell that has a zone on the map, in the order they were cast, then the burning side of a wall. */
export function zoneLegend(views: readonly ZoneView[]): ZoneLegendItem[] {
  const items = new Map<string, ZoneLegendItem>();
  const burning: string[] = [];
  for (const v of views) {
    const seen = items.get(v.spellKey);
    const hidden = v.masterOnly !== '';
    items.set(v.spellKey, {
      key: v.spellKey,
      name: v.spellName,
      pattern: zonePattern(v.spellKey),
      letters: [...(seen?.letters ?? []), v.letter],
      masterOnly: seen ? seen.masterOnly && hidden : hidden,
    });
    if (v.burningSquares.length > 0) {
      burning.push(v.letter);
    }
  }
  const list = [...items.values()];
  if (burning.length > 0) {
    list.push({
      key: 'burning',
      name: 'Lado que queima',
      pattern: BURNING_PATTERN,
      letters: burning,
      masterOnly: false,
    });
  }
  return list;
}
