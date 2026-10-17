import type { Combatant } from '../../../gen/meurpg/play/v1/combat_pb';
import type { MapLayers } from '../maps/layers';
import { Sight, type Vision } from '../maps/vision';
import { metersFixed } from '../units';
import type { Square } from './combat-grid';
import { rangeFt } from './spell-area';

/** What a player is told of a square the teleport cannot use: the same words as the server's refusal. */
export const TOO_FAR_TEXT = 'Longe demais';
export const UNAVAILABLE_TEXT = 'Esse espaço não está disponível';

/** Tenths of a foot in a foot: the ring of the map takes the reach in tenths. */
export const DFT_PER_FOOT = 10;

/** Whether a square can be picked: yes, beyond the spell's reach, or taken/seen-not (the server says no for either). */
export type Verdict = 'ok' | 'far' | 'unavailable';

/** What the picker knows of the map and of who is on it. */
export interface TeleportScene {
  readonly caster: Square | null;
  readonly reachFt: number;
  /** The master picks anywhere: the server says if the square cannot be used. */
  readonly master: boolean;
  readonly casterId: string;
  readonly combatants: readonly Combatant[];
  readonly layers: MapLayers | null;
  /** A player's fog: only a square they see can be picked. */
  readonly fog: Vision | null;
  readonly columns: number;
  readonly rows: number;
}

const SEEN_NOW: ReadonlySet<number> = new Set([Sight.Grey, Sight.Dim, Sight.Bright]);

/** The square's key in a set: row-major, like the map's layers and the fog. */
function keyOf(s: Square, columns: number): number {
  return s.row * columns + s.col;
}

/** The judge of a square, with the walls and the creatures turned into sets once. */
export function teleportJudge(scene: TeleportScene): (square: Square) => Verdict {
  const { columns, fog } = scene;
  const walls = new Set((scene.layers?.walls ?? []).map((w) => keyOf(w, columns)));
  const taken = new Set(
    scene.combatants
      .filter((c) => c.placed && !c.defeated && c.id !== scene.casterId)
      .map((c) => keyOf({ col: c.col, row: c.row }, columns)),
  );
  const unseen = (s: Square) =>
    !!fog?.fogEnabled && !SEEN_NOW.has(fog.states[keyOf(s, columns)] ?? Sight.Unseen);
  return (square) => {
    if (scene.master) {
      return 'ok';
    }
    if (scene.caster && rangeFt(scene.caster, square) > scene.reachFt) {
      return 'far';
    }
    const key = keyOf(square, columns);
    return walls.has(key) || taken.has(key) || unseen(square) ? 'unavailable' : 'ok';
  };
}

/** Every square of the grid the player can pick, for the tint. Empty for the master, who is not held to any. */
export function pickableSquares(scene: TeleportScene): Square[] {
  if (scene.master) {
    return [];
  }
  const judge = teleportJudge(scene);
  const out: Square[] = [];
  for (let row = 0; row < scene.rows; row++) {
    for (let col = 0; col < scene.columns; col++) {
      if (judge({ col, row }) === 'ok') {
        out.push({ col, row });
      }
    }
  }
  return out;
}

/** The refusal's words, or empty for a square that can be picked. */
export function verdictText(verdict: Verdict): string {
  if (verdict === 'far') {
    return TOO_FAR_TEXT;
  }
  return verdict === 'unavailable' ? UNAVAILABLE_TEXT : '';
}

/** What the live region says of a square: where it is, how far, and whether it can be picked. */
export function squareSpoken(square: Square, from: Square | null, verdict: Verdict): string {
  const place = `Coluna ${square.col + 1}, linha ${square.row + 1}`;
  const far = from ? `, a ${metersFixed(rangeFt(from, square))}` : '';
  return verdict === 'ok'
    ? `${place}${far}. Escolhido.`
    : `${place}${far}. ${verdictText(verdict)}.`;
}
