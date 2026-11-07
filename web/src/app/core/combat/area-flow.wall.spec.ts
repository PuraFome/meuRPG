import { create } from '@bufbuild/protobuf';

import {
  AreaPlacement,
  PreviewSpellAreaResponseSchema,
  WallSide,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { AreaFlow } from './area-flow';
import type { AreaChoice } from './combat-client-types';
import { type WallChoice, newWall, stepLength, switchShape } from './spell-wall';
import { nextTurn } from '../../../test-settle';

const answer = () =>
  create(PreviewSpellAreaResponseSchema, {
    origin: { col: 4, row: 3 },
    squares: [{ col: 4, row: 3 }],
    burningSquares: [{ col: 4, row: 4 }],
    lengthFt: 60,
  });

function wallFlow() {
  const asked: { area: AreaChoice | null; wall: WallChoice | null }[] = [];
  const flow = new AreaFlow(
    AreaPlacement.POINT,
    (area, wall) => {
      asked.push({ area, wall });
      return Promise.resolve(answer());
    },
    true,
  );
  return { flow, asked };
}

describe('AreaFlow for a wall', () => {
  it('has no wall for any other area, and asks with none', async () => {
    const asked: (WallChoice | null)[] = [];
    const flow = new AreaFlow(AreaPlacement.POINT, (_area, wall) => {
      asked.push(wall);
      return Promise.resolve(answer());
    });
    expect(flow.wall()).toBeNull();
    flow.place({ kind: 'point', square: { col: 1, row: 1 } });
    await nextTurn();
    expect(asked).toEqual([null]);
  });

  it('starts as a line to the east and sends the wall with the point it asks about', async () => {
    const { flow, asked } = wallFlow();
    expect(flow.wall()).toEqual(newWall(false));
    flow.place({ kind: 'point', square: { col: 4, row: 3 } });
    await nextTurn();
    expect(asked).toEqual([{ area: { origin: { col: 4, row: 3 } }, wall: newWall(false) }]);
    expect(flow.preview()?.burningSquares).toHaveLength(1);
  });

  it('asks again on every change of the wall once a point is placed, and not before', async () => {
    const { flow, asked } = wallFlow();
    flow.setWall(stepLength(newWall(false), -1));
    await nextTurn();
    expect(asked).toEqual([]);
    flow.place({ kind: 'point', square: { col: 4, row: 3 } });
    await nextTurn();
    flow.setWall({ ...flow.wall()!, direction: { dx: 0, dy: 1 } });
    await nextTurn();
    flow.setWall({ ...flow.wall()!, side: WallSide.LEFT });
    await nextTurn();
    expect(asked.map((a) => a.wall)).toEqual([
      { ...newWall(false), lengthFt: 55 },
      { ...newWall(false), lengthFt: 55, direction: { dx: 0, dy: 1 } },
      { ...newWall(false), lengthFt: 55, direction: { dx: 0, dy: 1 }, side: WallSide.LEFT },
    ]);
  });

  it('a drag places the start and sets the direction with one question', async () => {
    const { flow, asked } = wallFlow();
    flow.placeWall({ col: 2, row: 2 }, { dx: 1, dy: 1 });
    await nextTurn();
    expect(asked).toHaveLength(1);
    expect(asked[0].area).toEqual({ origin: { col: 2, row: 2 } });
    expect(asked[0].wall?.direction).toEqual({ dx: 1, dy: 1 });
    expect(flow.placed()).toEqual({ kind: 'point', square: { col: 2, row: 2 } });
  });

  it('a ring has no direction in what it asks', async () => {
    const { flow, asked } = wallFlow();
    flow.setWall(switchShape(flow.wall()!, true));
    flow.place({ kind: 'point', square: { col: 5, row: 5 } });
    await nextTurn();
    expect(asked[0].wall).toEqual({
      ring: true,
      direction: null,
      lengthFt: 20,
      side: WallSide.INNER,
    });
  });

  it('keeps the wall when it goes back to the map, and cannot change it on the list', async () => {
    const { flow, asked } = wallFlow();
    flow.place({ kind: 'point', square: { col: 4, row: 3 } });
    await nextTurn();
    await flow.confirm();
    flow.setWall({ ...flow.wall()!, side: WallSide.LEFT });
    await nextTurn();
    expect(flow.wall()?.side).toBe(WallSide.RIGHT);
    expect(asked).toHaveLength(1);
    flow.back();
    expect(flow.wall()?.side).toBe(WallSide.RIGHT);
  });
});
