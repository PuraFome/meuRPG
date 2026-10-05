import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import { describe, expect, it } from 'vitest';

import { MapBlockedReason, MapBlockedSchema } from '../../../gen/meurpg/maps/v1/maps_pb';
import { editorErrorMessage, mapBlockedReason } from './map-errors';

function blocked(reason: MapBlockedReason): ConnectError {
  return new ConnectError('x', Code.FailedPrecondition, undefined, [{ desc: MapBlockedSchema, value: create(MapBlockedSchema, { reason }) }]);
}

describe('mapBlockedReason', () => {
  it('reads the typed reason of a failed precondition, never the message', () => {
    expect(mapBlockedReason(blocked(MapBlockedReason.NO_GRID))).toBe(MapBlockedReason.NO_GRID);
    expect(mapBlockedReason(new ConnectError('NO_GRID', Code.FailedPrecondition))).toBeNull();
    expect(mapBlockedReason(new ConnectError('x', Code.NotFound))).toBeNull();
    expect(mapBlockedReason(new Error('offline'))).toBeNull();
  });
});

describe('editorErrorMessage', () => {
  it('says why by the reason', () => {
    expect(editorErrorMessage(blocked(MapBlockedReason.NO_GRID), 'pintar')).toBe('Defina a grade para pintar e ligar a névoa.');
    expect(editorErrorMessage(blocked(MapBlockedReason.COMBAT_RUNNING), 'mudar a grade')).toBe('Há um combate neste mapa: a grade e a imagem só mudam depois dele.');
    expect(editorErrorMessage(blocked(MapBlockedReason.TREASURE_CONVERTED), 'salvar')).toContain('já virou XP');
    expect(editorErrorMessage(blocked(MapBlockedReason.TREASURE_FOUND), 'apagar')).toContain('Desmarque antes de apagar');
  });

  it('says a full gallery when the fog needs a copy of the image', () => {
    expect(editorErrorMessage(new ConnectError('x', Code.ResourceExhausted), 'ligar a névoa')).toContain('galeria da campanha está cheia');
  });

  it('falls back to the map messages by code', () => {
    expect(editorErrorMessage(new ConnectError('x', Code.PermissionDenied), 'pintar')).toBe('Só o mestre da campanha muda os mapas.');
    expect(editorErrorMessage(new ConnectError('x', Code.InvalidArgument), 'salvar o ponto')).toContain('Não deu para salvar o ponto');
  });
});
