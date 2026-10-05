import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import { describe, expect, it } from 'vitest';

import { MapBlockedReason, MapBlockedSchema } from '../../../gen/meurpg/maps/v1/maps_pb';
import { GALLERY_FULL, IMAGE_GALLERY_FULL, POINTS_FULL, editorErrorMessage, mapBlockedReason } from './map-errors';

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

describe('editorErrorMessage: each call maps its own reasons and its own limit', () => {
  const exhausted = new ConnectError('x', Code.ResourceExhausted);

  it('the painting says a map with no grid cannot be painted', () => {
    expect(editorErrorMessage(blocked(MapBlockedReason.NO_GRID), 'paint', 'pintar')).toBe('Defina a grade para pintar e ligar a névoa.');
  });

  it('the grid and the image say a combat holds them, by the typed reason', () => {
    const combat = 'Há um combate neste mapa: a grade e a imagem só mudam depois dele.';
    expect(editorErrorMessage(blocked(MapBlockedReason.COMBAT_RUNNING), 'grid', 'mudar a grade')).toBe(combat);
    expect(editorErrorMessage(blocked(MapBlockedReason.COMBAT_RUNNING), 'image', 'trocar a imagem')).toBe(combat);
  });

  it('a reason the call cannot give falls back to the words of the code', () => {
    expect(editorErrorMessage(blocked(MapBlockedReason.COMBAT_RUNNING), 'paint', 'pintar')).toContain('Não foi possível falar com o servidor');
    expect(editorErrorMessage(blocked(MapBlockedReason.NO_GRID), 'point', 'salvar o ponto')).toContain('Não foi possível falar com o servidor');
  });

  it('a treasure found or turned into XP says so on a point call', () => {
    expect(editorErrorMessage(blocked(MapBlockedReason.TREASURE_CONVERTED), 'point', 'salvar')).toContain('já virou XP');
    expect(editorErrorMessage(blocked(MapBlockedReason.TREASURE_FOUND), 'point', 'apagar')).toContain('Desmarque antes de apagar');
  });

  it('resource_exhausted is the gallery for the fog and for a new image, and 200 points for a new point', () => {
    expect(editorErrorMessage(exhausted, 'fog', 'ligar a névoa')).toBe(GALLERY_FULL);
    expect(editorErrorMessage(exhausted, 'image', 'trocar a imagem')).toBe(IMAGE_GALLERY_FULL);
    expect(editorErrorMessage(exhausted, 'pointNew', 'criar o ponto')).toBe(POINTS_FULL);
    expect(POINTS_FULL).not.toContain('galeria');
  });

  it('resource_exhausted on a call that has no such limit is not explained as one', () => {
    expect(editorErrorMessage(exhausted, 'point', 'salvar o ponto')).not.toContain('galeria');
    expect(editorErrorMessage(exhausted, 'paint', 'pintar')).not.toContain('200 pontos');
  });

  it('falls back to the map messages by code', () => {
    expect(editorErrorMessage(new ConnectError('x', Code.PermissionDenied), 'paint', 'pintar')).toBe('Só o mestre da campanha muda os mapas.');
    expect(editorErrorMessage(new ConnectError('x', Code.InvalidArgument), 'point', 'salvar o ponto')).toContain('Não deu para salvar o ponto');
  });
});
