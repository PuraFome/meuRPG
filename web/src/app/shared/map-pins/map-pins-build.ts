import { type MapPoint, MapPointKind, TrapState } from '../../../gen/meurpg/maps/v1/maps_pb';
import { type PinArea, trapArea } from '../../core/traps/trap-area';
import { pointHidden } from '../map-view/map-labels';
import { bpToPercent } from '../map-view/map-geometry';

export interface Pin {
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

/** Marks within a quarter of a percent of each other share a square (the number of squares per percent). */
const SQUARE_PER_PERCENT = 4;

/** What the pins need to know besides the points: who is looking and the grid. */
export interface PinsView {
  readonly isMaster: boolean;
  readonly columns: number;
  readonly rows: number;
}

/** The mark's own state, for its look: a trap is `fired`, `disarmed` or `armed`. */
function trapStateName(p: MapPoint): string {
  switch (p.trap?.state) {
    case TrapState.TRIGGERED:
      return 'fired';
    case TrapState.DISARMED:
      return 'disarmed';
    default:
      return 'armed';
  }
}

type PinBase = Pick<Pin, 'id' | 'x' | 'y' | 'n' | 'remembered' | 'secret' | 'area'>;

/** The pin of one point, or `null` for a kind that has none (or a light a player does not see). */
function pinOf(p: MapPoint, base: PinBase, view: PinsView): Pin | null {
  switch (p.kind) {
    case MapPointKind.TRAP: {
      const state = trapStateName(p);
      const area = trapArea(p.xBp, p.yBp, p.trap?.areaSize ?? 1, view.columns, view.rows);
      const secret = view.isMaster && state === 'armed' && pointHidden(p);
      return { ...base, kind: 'trap', state, area, secret, icon: 'warning' };
    }
    case MapPointKind.TREASURE:
      return {
        ...base,
        kind: 'treasure',
        state: p.treasureFoundAt !== undefined ? 'found' : 'hidden',
        icon: 'inventory_2',
      };
    case MapPointKind.LIGHT:
      return view.isMaster ? { ...base, kind: 'light', state: '', icon: 'lightbulb' } : null;
    default:
      return null;
  }
}

/** Whether the kind is one the pins draw at all: the others do not take a place in their square. */
const DRAWN_KINDS: readonly MapPointKind[] = [
  MapPointKind.TRAP,
  MapPointKind.TREASURE,
  MapPointKind.LIGHT,
];

/** The pins of a map's points, each numbered within its square so marks sharing it sit side by side. */
export function buildPins(points: readonly MapPoint[], view: PinsView): Pin[] {
  const seen = new Map<string, number>();
  const out: Pin[] = [];
  for (const p of points) {
    // The second lock: a player never gets the pin of what is still hidden, even if it slipped in.
    if (!view.isMaster && pointHidden(p)) {
      continue;
    }
    if (!DRAWN_KINDS.includes(p.kind)) {
      continue;
    }
    const x = bpToPercent(p.xBp);
    const y = bpToPercent(p.yBp);
    const square = `${Math.round(x * SQUARE_PER_PERCENT)}/${Math.round(y * SQUARE_PER_PERCENT)}`;
    const n = seen.get(square) ?? 0;
    const pin = pinOf(
      p,
      { id: p.id, x, y, n, remembered: p.remembered, secret: false, area: null },
      view,
    );
    if (pin) {
      out.push(pin);
    }
    // A light a player does not see still takes its place in the square, as before.
    seen.set(square, n + 1);
  }
  return out;
}
