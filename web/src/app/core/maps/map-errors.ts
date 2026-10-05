import { Code, ConnectError } from '@connectrpc/connect';

import { MapBlockedReason, MapBlockedSchema } from '../../../gen/meurpg/maps/v1/maps_pb';
import { describeConnectError } from '../connect/connect-errors';

/** The Portuguese message for a failed map call: what happened and how to
 * fix it (maps.proto lists which method returns what). */
export function mapErrorMessage(err: unknown, what = 'salvar'): string {
  return describeConnectError(err, {
    [Code.InvalidArgument]: `Não deu para ${what}: confira os campos e tente de novo.`,
    [Code.NotFound]: 'Esse mapa não existe mais, ou a imagem saiu da galeria. Recarregue a página.',
    [Code.PermissionDenied]: 'Só o mestre da campanha muda os mapas.',
    [Code.Aborted]: 'O mapa mudou em outra aba. Recarregue a página e tente de novo.',
    [Code.ResourceExhausted]: 'A campanha chegou ao limite de mapas ou de pontos.',
  });
}

/** The Portuguese message for a failed scene-action call (maps.proto,
 * `AddSceneAction` and the others): `resource_exhausted` is the 20-action limit. */
export function sceneActionErrorMessage(err: unknown, what = 'salvar a ação'): string {
  return describeConnectError(err, {
    [Code.InvalidArgument]: `Não deu para ${what}: o nome vai até 60 caracteres e a CD de 1 a 30.`,
    [Code.NotFound]: 'Esse ponto não existe mais. Recarregue a página.',
    [Code.PermissionDenied]: 'Só o mestre da campanha muda as ações da cena.',
    [Code.ResourceExhausted]: 'Limite de 20 ações. Remova uma para adicionar outra.',
  });
}

/** The Portuguese message for a failed clue call on a point (maps.proto,
 * `AddSceneClue` and the others): `resource_exhausted` is the 30-clue limit. */
export function sceneClueErrorMessage(err: unknown, what = 'salvar a pista'): string {
  return describeConnectError(err, {
    [Code.InvalidArgument]: `Não deu para ${what}: a pista vai de 1 a 500 caracteres, numa linha só.`,
    [Code.NotFound]: 'Essa pista ou esse ponto não existe mais. Recarregue a página.',
    [Code.PermissionDenied]: 'Só o mestre da campanha muda as pistas da cena.',
    [Code.ResourceExhausted]: 'Limite de 30 pistas. Remova uma para adicionar outra.',
  });
}

/** The Portuguese message for a failed `RevealSceneClue`. */
export function revealErrorMessage(err: unknown): string {
  return describeConnectError(err, {
    [Code.InvalidArgument]: 'Marque pelo menos um jogador para receber a pista.',
    [Code.NotFound]: 'A pista, ou um desses personagens, não existe mais. Feche e tente de novo.',
    [Code.PermissionDenied]: 'Só o mestre da campanha revela pistas.',
  });
}

/** The reason of a map call refused with `failed_precondition` (`MapBlocked`), or `null`. By the typed detail, never the message. */
export function mapBlockedReason(err: unknown): MapBlockedReason | null {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code !== Code.FailedPrecondition) {
    return null;
  }
  return connectErr.findDetails(MapBlockedSchema)[0]?.reason ?? null;
}

/** The Portuguese message for a failed call of the map editor's painting, grid and fog (maps.proto: `PaintMapCells`,
 * `SetMapGrid`, `SetMapFog`, `ForgetMapVision`, `UpdateMap` with a new image) and of the trap, light and treasure points. */
export function editorErrorMessage(err: unknown, what: string): string {
  switch (mapBlockedReason(err)) {
    case MapBlockedReason.NO_GRID:
      return 'Defina a grade para pintar e ligar a névoa.';
    case MapBlockedReason.COMBAT_RUNNING:
      return 'Há um combate neste mapa: a grade e a imagem só mudam depois dele.';
    case MapBlockedReason.TREASURE_CONVERTED:
      return 'Esse tesouro já virou XP. Para mexer nele, desfaça esse XP na página da campanha.';
    case MapBlockedReason.TREASURE_FOUND:
      return 'Esse tesouro foi encontrado. Desmarque antes de apagar.';
    default:
  }
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code === Code.ResourceExhausted) {
    return 'A galeria da campanha está cheia: apague uma imagem para ligar a névoa.';
  }
  return mapErrorMessage(err, what);
}
