import { create } from '@bufbuild/protobuf';

import { GetTrapNoticersResponseSchema } from '../../../gen/meurpg/maps/v1/maps_pb';
import { TrapBoard } from './trap-board';

const res = (dc: number) => create(GetTrapNoticersResponseSchema, { noticeDc: dc });

function setup(read: (n: number) => Promise<unknown>) {
  let n = 0;
  const maps = { getTrapNoticers: () => read(++n) };
  const traps = { activity: async () => ({ activity: [] }), damages: async () => ({ damages: [] }) };
  return new TrapBoard(traps as never, maps as never, () => 'c', () => 'm', () => true);
}

describe('TrapBoard "Quem notaria"', () => {
  it('says the read failed, and a retry clears it', async () => {
    let fail = true;
    const board = setup(async () => {
      if (fail) {
        throw new Error('down');
      }
      return res(15);
    });
    await board.watchNoticers('p1');
    expect(board.noticersFailed().has('p1')).toBe(true);
    expect(board.noticers().has('p1')).toBe(false);
    fail = false;
    await board.retryNoticers('p1');
    expect(board.noticersFailed().has('p1')).toBe(false);
    expect(board.noticers().get('p1')?.noticeDc).toBe(15);
  });

  it('never lets a stale answer overwrite a newer one', async () => {
    const slow: ((v: unknown) => void)[] = [];
    const board = setup((n) => (n === 1 ? new Promise((r) => slow.push(r)) : Promise.resolve(res(20))));
    const first = board.watchNoticers('p1');
    await board.retryNoticers('p1'); // the newer read answers 20
    slow[0](res(5)); // the older one arrives late
    await first;
    expect(board.noticers().get('p1')?.noticeDc).toBe(20);
  });
});
