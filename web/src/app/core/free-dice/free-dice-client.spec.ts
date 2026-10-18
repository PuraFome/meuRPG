import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import {
  FreeRollDieSchema,
  FreeRollMode,
  FreeRollSchema,
  ListFreeRollsResponseSchema,
  RollFreeDiceResponseSchema,
} from '../../../gen/meurpg/play/v1/free_dice_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { FreeDiceClient } from './free-dice-client';

const wire = (id: string) =>
  create(FreeRollSchema, {
    id,
    sides: 6,
    count: 1,
    dice: [create(FreeRollDieSchema, { face: 3 })],
    total: 3,
  });

function clientWith(calls: Record<string, ReturnType<typeof vi.fn>>): FreeDiceClient {
  TestBed.configureTestingModule({
    providers: [FreeDiceClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
  });
  const client = TestBed.inject(FreeDiceClient);
  (client as unknown as { client: unknown }).client = calls;
  return client;
}

describe('FreeDiceClient', () => {
  it('reads the table and the own rolls', async () => {
    const listFreeRolls = vi
      .fn()
      .mockResolvedValue(
        create(ListFreeRollsResponseSchema, { table: [wire('a'), wire('b')], mine: [wire('b')] }),
      );
    const lists = await clientWith({ listFreeRolls }).list('c1');
    expect(lists.table.map((r) => r.id)).toEqual(['a', 'b']);
    expect(lists.mine.map((r) => r.id)).toEqual(['b']);
    expect(listFreeRolls).toHaveBeenCalledWith({ campaignId: 'c1' });
  });

  it('asks for the roll with the mode as the wire enum, and gives back the roll', async () => {
    const rollFreeDice = vi
      .fn()
      .mockResolvedValue(create(RollFreeDiceResponseSchema, { roll: wire('r1') }));
    const roll = await clientWith({ rollFreeDice }).roll({
      campaignId: 'c1',
      sides: 20,
      count: 1,
      modifier: 5,
      mode: 'advantage',
      secret: true,
      idempotencyKey: 'k1',
    });
    expect(roll.id).toBe('r1');
    expect(rollFreeDice).toHaveBeenCalledWith({
      campaignId: 'c1',
      sides: 20,
      count: 1,
      modifier: 5,
      mode: FreeRollMode.ADVANTAGE,
      secret: true,
      idempotencyKey: 'k1',
    });
  });

  it('refuses an answer with no roll', async () => {
    const rollFreeDice = vi.fn().mockResolvedValue(create(RollFreeDiceResponseSchema, {}));
    await expect(
      clientWith({ rollFreeDice }).roll({
        campaignId: 'c1',
        sides: 6,
        count: 1,
        modifier: 0,
        mode: 'normal',
        secret: false,
        idempotencyKey: 'k',
      }),
    ).rejects.toThrow();
  });
});
