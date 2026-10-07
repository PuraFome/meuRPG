import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { MapBlockedReason, MapBlockedSchema } from '../../../gen/meurpg/maps/v1/maps_pb';
import {
  TreasureBlockedReason,
  TreasureBlockedSchema,
} from '../../../gen/meurpg/maps/v1/treasure_pb';
import {
  NO_GRID_TEXT,
  NO_PARTY_TEXT,
  generateFailure,
  itemFailure,
  placeFailure,
  treasureBlockedReason,
} from './treasure-errors';

function blocked(reason: TreasureBlockedReason): ConnectError {
  return new ConnectError('refused', Code.FailedPrecondition, undefined, [
    { desc: TreasureBlockedSchema, value: create(TreasureBlockedSchema, { reason }) },
  ]);
}

describe('treasure-errors: every refusal by its code and its typed detail (MR-044)', () => {
  it('reads the reason of a failed precondition, never the message', () => {
    expect(treasureBlockedReason(blocked(TreasureBlockedReason.CONTENT_CHANGED))).toBe(
      TreasureBlockedReason.CONTENT_CHANGED,
    );
    expect(treasureBlockedReason(new ConnectError('x', Code.InvalidArgument))).toBeNull();
  });

  it('a generate without a party level asks for one', () => {
    expect(generateFailure(blocked(TreasureBlockedReason.NO_PARTY))).toBe(NO_PARTY_TEXT);
    expect(generateFailure(new ConnectError('x', Code.InvalidArgument))).toContain('1 a 20');
    expect(generateFailure(new ConnectError('x', Code.NotFound))).toContain('não é o mestre');
    expect(generateFailure(new Error('network'))).toContain('o servidor não respondeu');
  });

  it('an item that is gone says so', () => {
    expect(itemFailure(new ConnectError('x', Code.NotFound))).toContain('não existe mais');
  });

  it('a changed content version says "gere de novo" and offers it', () => {
    const f = placeFailure(blocked(TreasureBlockedReason.CONTENT_CHANGED));
    expect(f.generateAgain).toBe(true);
    expect(f.text).toContain('Gere de novo');
  });

  it('a map without a grid says "Escolha um mapa com grade"', () => {
    const err = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: MapBlockedSchema,
        value: create(MapBlockedSchema, { reason: MapBlockedReason.NO_GRID }),
      },
    ]);
    expect(placeFailure(err)).toEqual({ text: NO_GRID_TEXT, generateAgain: false });
    expect(NO_GRID_TEXT.startsWith('Escolha um mapa com grade')).toBe(true);
  });

  it('a square out of the grid, the 200-point cap, a map that is gone and a lost server each have their words', () => {
    expect(placeFailure(new ConnectError('x', Code.InvalidArgument)).text).toContain(
      'fora da grade',
    );
    expect(placeFailure(new ConnectError('x', Code.ResourceExhausted)).text).toContain(
      'limite de 200 pontos',
    );
    expect(placeFailure(new ConnectError('x', Code.NotFound)).text).toContain(
      'Esse mapa não existe mais',
    );
    expect(placeFailure(new ConnectError('x', Code.Aborted)).text).toContain('O mapa mudou');
    expect(placeFailure(new Error('net')).text).toContain('o servidor não respondeu');
    expect(placeFailure(new ConnectError('x', Code.NotFound)).generateAgain).toBe(false);
  });
});
