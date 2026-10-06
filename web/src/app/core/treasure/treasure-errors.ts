import { Code, ConnectError } from '@connectrpc/connect';

import { MapBlockedReason } from '../../../gen/meurpg/maps/v1/maps_pb';
import { TreasureBlockedReason, TreasureBlockedSchema } from '../../../gen/meurpg/maps/v1/treasure_pb';
import { describeConnectError } from '../connect/connect-errors';
import { mapBlockedReason } from '../maps/map-errors';

/** The reason a treasure call was refused with `failed_precondition` (`TreasureBlocked`), or `null`. By the typed detail, never the message. */
export function treasureBlockedReason(err: unknown): TreasureBlockedReason | null {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code !== Code.FailedPrecondition) {
    return null;
  }
  return connectErr.findDetails(TreasureBlockedSchema)[0]?.reason ?? null;
}

/** Said when "Gerar tesouro" is refused for lack of a party level. */
export const NO_PARTY_TEXT = 'A campanha não tem personagem de jogador vivo para dar o nível. Escolha o nível do grupo e gere de novo.';

/** The words of a failed "Gerar tesouro". */
export function generateFailure(err: unknown): string {
  if (treasureBlockedReason(err) === TreasureBlockedReason.NO_PARTY) {
    return NO_PARTY_TEXT;
  }
  return describeConnectError(err, {
    [Code.InvalidArgument]: 'O nível do grupo vai de 1 a 20. Confira e gere de novo.',
    [Code.NotFound]: 'Essa campanha não existe, ou você não é o mestre dela.',
    [Code.Unavailable]: 'Não deu para gerar o tesouro: o servidor não respondeu. Tente de novo.',
  });
}

/** The words of a failed "Ver descrição". */
export function itemFailure(err: unknown): string {
  return describeConnectError(err, {
    [Code.NotFound]: 'Esse item não existe mais no SRD, ou você não é o mestre da campanha.',
    [Code.Unavailable]: 'Não deu para abrir a descrição: o servidor não respondeu. Tente de novo.',
  });
}

/** What a failed "Pôr no mapa" looks like to the sheet: the words, and whether the treasure must be generated again. */
export interface PlaceFailure {
  readonly text: string;
  /** `CONTENT_CHANGED`: the tables or the items changed, so the same seed would now be another treasure. */
  readonly generateAgain: boolean;
}

/** The words of a failed "Pôr no mapa", by the typed reason first and then by the code, never by the server's message. */
export function placeFailure(err: unknown): PlaceFailure {
  if (treasureBlockedReason(err) === TreasureBlockedReason.CONTENT_CHANGED) {
    return {
      text: 'As tabelas do jogo mudaram desde que este tesouro saiu, então a mesma semente daria outro tesouro. Gere de novo e ponha o novo no mapa.',
      generateAgain: true,
    };
  }
  if (mapBlockedReason(err) === MapBlockedReason.NO_GRID) {
    return { text: 'Escolha um mapa com grade. Um tesouro gerado precisa de um quadrado, e este mapa não tem grade.', generateAgain: false };
  }
  return {
    generateAgain: false,
    text: describeConnectError(err, {
      [Code.InvalidArgument]: 'Não deu para pôr o tesouro: o quadrado fica fora da grade do mapa, ou o nome passa de 80 letras. O mapa pode ter mudado; escolha o quadrado de novo.',
      [Code.NotFound]: 'Esse mapa não existe mais, ou você não é o mestre da campanha. Escolha outro mapa.',
      [Code.ResourceExhausted]: 'O mapa chegou ao limite de 200 pontos. Apague um ponto ou escolha outro mapa.',
      [Code.Aborted]: 'O mapa mudou enquanto o tesouro era posto. Tente de novo.',
      [Code.Unavailable]: 'Não deu para pôr o tesouro: o servidor não respondeu. Tente de novo.',
    }),
  };
}

/** "Escolha um mapa com grade", the dialog's own words for a map it knows has none. */
export const NO_GRID_TEXT = 'Escolha um mapa com grade. Um tesouro gerado precisa de um quadrado, e este mapa não tem grade.';
