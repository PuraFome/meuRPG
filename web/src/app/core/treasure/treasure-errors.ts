import { Code, ConnectError } from '@connectrpc/connect';

import { MapBlockedReason } from '../../../gen/meurpg/maps/v1/maps_pb';
import {
  TreasureBlockedReason,
  TreasureBlockedSchema,
  TreasureInvalidFieldSchema,
} from '../../../gen/meurpg/maps/v1/treasure_pb';
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

/** The request field a refused `invalid_argument` names (`TreasureInvalidField`), `''` when it names none, `null` for another error. */
export function treasureInvalidField(err: unknown): string | null {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code !== Code.InvalidArgument) {
    return null;
  }
  return connectErr.findDetails(TreasureInvalidFieldSchema)[0]?.field ?? '';
}

/** Said when "Gerar tesouro" is refused for lack of a party level. */
export const NO_PARTY_TEXT =
  'A campanha não tem personagem de jogador vivo para dar o nível. Escolha o nível do grupo e gere de novo.';

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
  /** The square is outside the grid the server has now: the map changed under the dialog, so it is read again. */
  readonly rereadMap: boolean;
}

/** The refusal of `PlaceTreasure` as an `invalid_argument` names the request field that broke the rule: what to do
 * depends on it. `null` for anything else. */
function invalidPlacement(field: string | null): PlaceFailure | null {
  switch (field) {
    case null:
      return null;
    case 'column':
    case 'row':
      return {
        generateAgain: false,
        rereadMap: true,
        text: 'Não deu para pôr o tesouro: o quadrado fica fora da grade do mapa. O mapa pode ter mudado; ele foi aberto de novo, escolha o quadrado outra vez.',
      };
    case 'name':
      return {
        generateAgain: false,
        rereadMap: false,
        text: 'O nome do ponto precisa ter de 1 a 80 caracteres. Corrija o nome e tente de novo.',
      };
    case 'idempotency_key':
      return {
        generateAgain: false,
        rereadMap: false,
        text: 'Esse pedido já foi usado para outro tesouro ou outro quadrado. Gere o tesouro de novo e ponha no mapa.',
      };
    case 'mode':
    case 'party_level':
    case 'seed':
    case 'content_version':
      return {
        generateAgain: true,
        rereadMap: false,
        text: 'Este tesouro não está completo ou o nível do grupo está fora de 1 a 20. Gere de novo e ponha o novo no mapa.',
      };
    default:
      return {
        generateAgain: false,
        rereadMap: false,
        text: 'Não deu para pôr o tesouro: confira o que foi escolhido e tente de novo.',
      };
  }
}

/** The words of a failed "Pôr no mapa", by the typed reason first and then by the code, never by the server's message. */
export function placeFailure(err: unknown): PlaceFailure {
  if (treasureBlockedReason(err) === TreasureBlockedReason.CONTENT_CHANGED) {
    return {
      text: 'As tabelas do jogo mudaram desde que este tesouro saiu, então a mesma semente daria outro tesouro. Gere de novo e ponha o novo no mapa.',
      generateAgain: true,
      rereadMap: false,
    };
  }
  if (mapBlockedReason(err) === MapBlockedReason.NO_GRID) {
    return {
      text: 'Escolha um mapa com grade. Um tesouro gerado precisa de um quadrado, e este mapa não tem grade.',
      generateAgain: false,
      rereadMap: false,
    };
  }
  const invalid = invalidPlacement(treasureInvalidField(err));
  if (invalid) {
    return invalid;
  }
  return {
    generateAgain: false,
    rereadMap: false,
    text: describeConnectError(err, {
      [Code.NotFound]:
        'Esse mapa não existe mais, ou você não é o mestre da campanha. Escolha outro mapa.',
      [Code.ResourceExhausted]:
        'O mapa chegou ao limite de 200 pontos. Apague um ponto ou escolha outro mapa.',
      [Code.Aborted]: 'O mapa mudou enquanto o tesouro era posto. Tente de novo.',
      [Code.Unavailable]: 'Não deu para pôr o tesouro: o servidor não respondeu. Tente de novo.',
    }),
  };
}

/** "Escolha um mapa com grade", the dialog's own words for a map it knows has none. */
export const NO_GRID_TEXT =
  'Escolha um mapa com grade. Um tesouro gerado precisa de um quadrado, e este mapa não tem grade.';
