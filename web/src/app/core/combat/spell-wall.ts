import { WallSide } from '../../../gen/meurpg/play/v1/combat_pb';
import { SQUARE_FT, metersText, reachSquares } from '../units';
import type { Square } from './combat-grid';
import type { Direction } from './spell-area';
import type { ZoneView } from './zones';

/**
 * The spell that lays a wall (Wall of Fire, SRD 5.1) on the map: a straight wall or a ring, placed by a point, a direction,
 * a length and the side that burns (RN-37). The turn options mark it as a point spell (`AreaPlacement.POINT`) like Moonbeam
 * and carry nothing else that tells a wall apart, so the web names it by its catalog key, here and nowhere else.
 * Follow-up: a flag from the server on the spell's targets (`lays_wall`) would replace this constant.
 */
export const WALL_SPELL_KEY = 'spell:wall-of-fire';

/** Whether the spell lays a wall: its cast sheet asks "Onde fica" (shape, length, side) and sends `SpellWall`. */
export function laysWall(spellKey: string): boolean {
  return spellKey === WALL_SPELL_KEY;
}

/** The largest a straight wall is, and a ring's diameter, in feet: what the server takes (`wallLength`). */
export const WALL_LINE_FT = 60;
export const WALL_RING_FT = 20;
/** A ring is offered at two sizes only: the server rounds the radius up to 5 ft, so 5 and 10 ft are one ring (8 squares) and
 * 15 and 20 ft are another (16 squares). */
const RING_MIN_FT = 10;

/** The wall as the caster is laying it. */
export interface WallChoice {
  readonly ring: boolean;
  /** The way a line runs from its first square; `null` for a ring. */
  readonly direction: Direction | null;
  /** A line's length or a ring's diameter, in feet: always explicit, a multiple of 5. */
  readonly lengthFt: number;
  readonly side: WallSide;
}

const EAST: Direction = { dx: 1, dy: 0 };

export const maxLengthFt = (ring: boolean) => (ring ? WALL_RING_FT : WALL_LINE_FT);
export const minLengthFt = (ring: boolean) => (ring ? RING_MIN_FT : SQUARE_FT);

/** A wall at its full size: a line to the east with its right side burning, or a ring with its inside burning. */
export function newWall(ring: boolean): WallChoice {
  return {
    ring,
    direction: ring ? null : EAST,
    lengthFt: maxLengthFt(ring),
    side: ring ? WallSide.INNER : WallSide.RIGHT,
  };
}

/** The other shape: its own side and direction, the length kept as far as the new size allows. */
export function switchShape(wall: WallChoice, ring: boolean): WallChoice {
  if (wall.ring === ring) {
    return wall;
  }
  const fresh = newWall(ring);
  const lengthFt = ring
    ? wall.lengthFt > RING_MIN_FT + SQUARE_FT
      ? WALL_RING_FT
      : RING_MIN_FT
    : Math.max(minLengthFt(ring), wall.lengthFt);
  return { ...fresh, lengthFt };
}

/** How much one step of the stepper changes the length, in feet: a square for a line, 10 ft (the next ring) for a ring. */
export const stepFt = (ring: boolean) => (ring ? RING_MIN_FT : SQUARE_FT);

/** The length a step more or less (a square, 1,5 m, for a line; the other ring size for a ring), kept inside the shape's sizes. */
export function stepLength(wall: WallChoice, steps: number): WallChoice {
  const next = wall.lengthFt + steps * stepFt(wall.ring);
  return {
    ...wall,
    lengthFt: Math.min(maxLengthFt(wall.ring), Math.max(minLengthFt(wall.ring), next)),
  };
}

/** "12 m": the length as the table says it. */
export function lengthText(wall: WallChoice): string {
  return metersText(wall.lengthFt);
}

/** "De 1,5 m a 18 m: 8 quadrados." for a line, "3 m ou 6 m de diâmetro." for a ring. */
export function lengthNote(wall: WallChoice): string {
  if (wall.ring) {
    return `${metersText(minLengthFt(true))} ou ${metersText(maxLengthFt(true))} de diâmetro.`;
  }
  const range = `De ${metersText(minLengthFt(false))} a ${metersText(maxLengthFt(false))}`;
  const n = reachSquares(wall.lengthFt);
  return `${range}: ${n} ${n === 1 ? 'quadrado' : 'quadrados'}.`;
}

export interface SideChoice {
  readonly side: WallSide;
  readonly label: string;
  readonly icon: string;
}

/** The sides the shape offers: left or right of a line, inside or outside a ring. */
export function sideChoices(ring: boolean): readonly SideChoice[] {
  return ring
    ? [
        { side: WallSide.INNER, label: 'Por dentro', icon: 'center_focus_weak' },
        { side: WallSide.OUTER, label: 'Por fora', icon: 'fullscreen' },
      ]
    : [
        { side: WallSide.LEFT, label: 'À esquerda', icon: 'arrow_back' },
        { side: WallSide.RIGHT, label: 'À direita', icon: 'arrow_forward' },
      ];
}

/** What the burning side does, with the dice of the slot ("5d8"; empty says only "dano de fogo"). */
export function sideNote(wall: WallChoice, dice: string): string {
  const fire = dice ? `${dice} de fogo` : 'dano de fogo';
  if (!wall.ring) {
    return `A seta no mapa mostra o lado: ${fire} para quem termina o turno ali, até ${metersText(10)}.`;
  }
  const where = wall.side === WallSide.OUTER ? 'fora' : 'dentro';
  return `Quem termina o turno ${where} do anel, ou entra na muralha, sofre ${dice ? dice : 'dano'} de fogo.`;
}

/** The wall as `CastSpell.wall` and `PreviewSpellArea.wall` take it: a ring sends no direction. */
export function wallBody(wall: WallChoice) {
  return {
    ring: wall.ring,
    ...(!wall.ring && wall.direction
      ? { direction: { dx: wall.direction.dx, dy: wall.direction.dy } }
      : {}),
    lengthFt: wall.lengthFt,
    side: wall.side,
  };
}

/** The hint under the step's title: how to place the wall, then how to change it. */
export function wallHint(wall: WallChoice, placed: boolean): string {
  if (wall.ring) {
    return placed
      ? 'Centro marcado. Toque de novo no mesmo ponto para confirmar, ou mude o diâmetro e o lado abaixo.'
      : 'Toque no centro do anel. Setas movem o centro; Enter confirma.';
  }
  return placed
    ? 'Começo marcado. Arraste no mapa de novo ou use os botões de direção abaixo; toque no mesmo ponto para confirmar.'
    : 'Toque no começo e arraste para onde vai. Ou use as setas abaixo.';
}

/** What the server laid for a wall, as the zone the map draws: the wall's squares, its burning side and the side that burns. */
export function wallZoneView(
  answer: {
    readonly origin?: Square | undefined;
    readonly squares: readonly Square[];
    readonly burningSquares: readonly Square[];
  },
  side: WallSide,
): ZoneView {
  const squares = (list: readonly Square[]) => list.map((q) => ({ col: q.col, row: q.row }));
  return {
    id: 'wall-preview',
    letter: '',
    spellKey: WALL_SPELL_KEY,
    spellName: '',
    label: '',
    pieces: [],
    detail: '',
    roundsLeft: '',
    concentration: '',
    occupants: '',
    masterOnly: '',
    reminder: '',
    dashed: false,
    detailsHidden: false,
    canMove: false,
    canEnd: false,
    origin: answer.origin ? { col: answer.origin.col, row: answer.origin.row } : null,
    squares: squares(answer.squares),
    burningSquares: squares(answer.burningSquares),
    wallSide: side,
  };
}
