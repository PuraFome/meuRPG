import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { MapBlockedReason, MapBlockedSchema } from '../../../gen/meurpg/maps/v1/maps_pb';
import {
  TreasureBlockedReason,
  TreasureBlockedSchema,
  TreasureInvalidFieldSchema,
} from '../../../gen/meurpg/maps/v1/treasure_pb';
import {
  NO_GRID_TEXT,
  NO_PARTY_TEXT,
  generateFailure,
  itemFailure,
  placeFailure,
  treasureBlockedReason,
  treasureInvalidField,
} from './treasure-errors';

function blocked(reason: TreasureBlockedReason): ConnectError {
  return new ConnectError('refused', Code.FailedPrecondition, undefined, [
    { desc: TreasureBlockedSchema, value: create(TreasureBlockedSchema, { reason }) },
  ]);
}

/** An `invalid_argument` as the server sends it: with the request field that broke the rule. */
function invalidField(field: string): ConnectError {
  return new ConnectError('refused', Code.InvalidArgument, undefined, [
    { desc: TreasureInvalidFieldSchema, value: create(TreasureInvalidFieldSchema, { field }) },
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
    expect(placeFailure(err)).toEqual({
      text: NO_GRID_TEXT,
      generateAgain: false,
      rereadMap: false,
    });
    expect(NO_GRID_TEXT.startsWith('Escolha um mapa com grade')).toBe(true);
  });

  it('a square out of the grid, the 200-point cap, a map that is gone and a lost server each have their words', () => {
    expect(placeFailure(invalidField('column')).text).toBe(
      'Não deu para pôr o tesouro: o quadrado fica fora da grade do mapa. O mapa pode ter mudado e foi aberto de novo: escolha o quadrado outra vez.',
    );
    expect(placeFailure(invalidField('row')).text).toContain('fora da grade');
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

  it('words an invalid argument by the field it names, and only a square outside the grid reads the map again', () => {
    expect(treasureInvalidField(invalidField('name'))).toBe('name');
    expect(treasureInvalidField(new ConnectError('x', Code.InvalidArgument))).toBe('');
    expect(treasureInvalidField(new ConnectError('x', Code.NotFound))).toBeNull();
    const square = placeFailure(invalidField('row'));
    expect(square).toMatchObject({ rereadMap: true, generateAgain: false });
    const name = placeFailure(invalidField('name'));
    expect(name.text).toContain('1 a 80 caracteres');
    expect(name.rereadMap).toBe(false);
    const key = placeFailure(invalidField('idempotency_key'));
    expect(key.text).toContain('já foi usado para outro');
    expect(key.rereadMap).toBe(false);
    for (const field of ['mode', 'party_level', 'seed', 'content_version']) {
      expect(placeFailure(invalidField(field))).toMatchObject({
        generateAgain: true,
        rereadMap: false,
      });
    }
    // An invalid argument that names nothing is never worded as a square outside the grid.
    const unnamed = placeFailure(new ConnectError('x', Code.InvalidArgument));
    expect(unnamed.text).not.toContain('fora da grade');
    expect(unnamed.rereadMap).toBe(false);
  });
});
