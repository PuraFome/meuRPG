import { create } from '@bufbuild/protobuf';

import {
  GetMapVisionResponseSchema,
  type GetMapVisionResponse,
} from '../../../gen/meurpg/maps/v1/maps_pb';
import { Sight } from './vision';

/** Packs one state a square into the bytes `GetMapVision` sends (four bits a square, the even square in the low bits). */
export function packStates(states: readonly number[]): Uint8Array {
  const out = new Uint8Array(Math.ceil(states.length / 2));
  states.forEach((s, n) => {
    out[n >> 1] |= n & 1 ? s << 4 : s;
  });
  return out;
}

/** Builds the states of a grid from rows of letters. `.` (or a space) unseen, `w` (or `#`, as the Go oracle draws it) a wall seen,
 * `g` grey, `d` dim, `B` bright, `r` remembered. */
export function statesOf(rows: readonly string[]): number[] {
  const value: Record<string, number> = {
    '.': Sight.Unseen,
    ' ': Sight.Unseen,
    w: Sight.Wall,
    '#': Sight.Wall,
    g: Sight.Grey,
    d: Sight.Dim,
    B: Sight.Bright,
    r: Sight.Remembered,
  };
  return rows.flatMap((row) => [...row].map((ch) => value[ch] ?? Sight.Unseen));
}

/** A `GetMapVisionResponse` for a grid given as rows of letters (see `statesOf`), for the specs. */
export function visionResponse(
  rows: readonly string[],
  partial: Partial<Omit<GetMapVisionResponse, '$typeName'>> = {},
): GetMapVisionResponse {
  return create(GetMapVisionResponseSchema, {
    gridColumns: rows[0]?.length ?? 0,
    gridRows: rows.length,
    states: packStates(statesOf(rows)),
    revision: 1,
    characterOnMap: true,
    fogEnabled: true,
    ...partial,
  });
}
