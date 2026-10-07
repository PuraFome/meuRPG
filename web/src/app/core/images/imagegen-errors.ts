import { Code, ConnectError } from '@connectrpc/connect';

import {
  ImageGenerationBlockedReason,
  ImageGenerationBlockedSchema,
  ImageGenerationFailure,
  type ImageGeneration,
  ImageGenerationInvalidFieldSchema,
  type ImageGenerationStatus,
} from '../../../gen/meurpg/maps/v1/imagegen_pb';
import { describeConnectError } from '../connect/connect-errors';
import { monthName, resetText } from './imagegen-copy';

/** A refusal said for the master: what happened, and what to do (E10-07 8). */
export interface GenerateIssue {
  readonly text: string;
  /** The form field to point at, when the server named one (`npc_character_ids`, `character_image_ids`). */
  readonly field: string | null;
  /** The typed reason of a `failed_precondition`, so a screen can react (disable "Gerar imagem" with no month left). */
  readonly reason: ImageGenerationBlockedReason | null;
  /** The month, as the refusal carried it: it gives the count under the form. */
  readonly status: ImageGenerationStatus | null;
}

/** The typed reason of a refused call (`ImageGenerationBlocked`), or `null`. Never the message. */
export function blockedOf(err: unknown): { reason: ImageGenerationBlockedReason; status: ImageGenerationStatus | null } | null {
  const e = ConnectError.from(err, Code.Unavailable);
  if (e.code !== Code.FailedPrecondition) {
    return null;
  }
  const detail = e.findDetails(ImageGenerationBlockedSchema)[0];
  return detail ? { reason: detail.reason, status: detail.status ?? null } : null;
}

/** The words of a refusal that made no slot move, by the typed detail of the call. `monthly` fills in "Você usou as 20 imagens de outubro". */
export function blockedText(reason: ImageGenerationBlockedReason, status: ImageGenerationStatus | null): string {
  switch (reason) {
    case ImageGenerationBlockedReason.OFF:
      return 'A geração de imagens não está ligada neste servidor.';
    case ImageGenerationBlockedReason.LIMIT_REACHED:
      return status
        ? `Você usou as ${status.monthlyLimit} imagens de ${monthName(status.month)}. Volta em ${resetText(status)}.`
        : 'A campanha usou as imagens deste mês. O limite volta no mês que vem.';
    case ImageGenerationBlockedReason.DAILY_LIMIT_REACHED:
      return 'O servidor já fez todas as imagens de hoje. Tente de novo amanhã.';
    case ImageGenerationBlockedReason.GALLERY_FULL:
      return 'A galeria está cheia. Apague as imagens que você não usa e tente de novo.';
    case ImageGenerationBlockedReason.REQUEST_TOO_LARGE:
      return 'O pedido ficou grande demais para o serviço. Escolha menos imagens de referência.';
    case ImageGenerationBlockedReason.PLAYERS_SEE_NOTHING:
      return 'Os jogadores não veem nada deste mapa agora: nenhum personagem de jogador está nele, ou nenhum vê um quadrado. Ponha os personagens no mapa e tente de novo.';
    case ImageGenerationBlockedReason.MAP_HAS_NO_GRID:
      return 'Este mapa não tem grade. Defina a grade do mapa para gerar a imagem a partir dele.';
    case ImageGenerationBlockedReason.MAP_IMAGE_TOO_LARGE:
      return 'A imagem deste mapa tem mais de 16 megapixels (4.000 × 4.000 px). Troque por uma menor para usar o mapa com textura.';
    case ImageGenerationBlockedReason.MAP_CHANGED:
      return 'O mapa mudou desde que esta imagem foi feita: a imagem, a grade ou as paredes. Gere de novo.';
    default:
      return 'Não deu para gerar a imagem agora. Tente de novo.';
  }
}

/** The field a refused `invalid_argument` names (`ImageGenerationInvalidField`), `''` when it names none, `null` for another error. */
export function invalidField(err: unknown): string | null {
  const e = ConnectError.from(err, Code.Unavailable);
  if (e.code !== Code.InvalidArgument) {
    return null;
  }
  return e.findDetails(ImageGenerationInvalidFieldSchema)[0]?.field ?? '';
}

/** What went wrong in a call that asks for, edits or cancels a picture (and "Usar como imagem do mapa"), from its code and its typed detail. */
export function generateIssue(err: unknown, what: 'generate' | 'edit' = 'generate'): GenerateIssue {
  const blocked = blockedOf(err);
  if (blocked) {
    return { text: blockedText(blocked.reason, blocked.status), field: null, reason: blocked.reason, status: blocked.status };
  }
  const field = invalidField(err);
  if (field !== null) {
    return { text: invalidText(field), field: field === '' ? null : field, reason: null, status: null };
  }
  return {
    text: describeConnectError(err, {
      [Code.NotFound]:
        what === 'edit'
          ? 'A imagem que você quer ajustar não existe mais na galeria, ou não foi feita pelo app. Feche o diálogo e abra de novo.'
          : 'Esse mapa, ou uma das imagens escolhidas, não existe mais. Feche o diálogo e abra de novo.',
      [Code.Unauthenticated]: 'Sua sessão acabou. Entre de novo para gerar a imagem.',
    }),
    field: null,
    reason: null,
    status: null,
  };
}

function invalidText(field: string): string {
  switch (field) {
    case 'npc_character_ids':
      return 'Um dos NPCs marcados não aparece mais para os jogadores. Marque só os da lista.';
    case 'character_image_ids':
      return 'O retrato de um NPC que os jogadores não veem não pode ir como referência. Tire-o das referências.';
    default:
      return 'Confira o texto (de 1 a 500 caracteres), o estilo e as referências, e tente de novo.';
  }
}

/** How a request that ended without a picture is told (E10-07 8): the sentence and whether the slot came back. */
export function failureText(generation: Pick<ImageGeneration, 'failure' | 'slotSpent' | 'reasonPt'>): string {
  const back = generation.slotSpent ? 'Esta tentativa gastou uma imagem do mês.' : 'Esta tentativa não gastou nenhuma imagem do mês.';
  switch (generation.failure) {
    case ImageGenerationFailure.NO_IMAGE:
      return `O serviço não gerou esta imagem. Tente descrever a cena de outro jeito. ${back}`;
    case ImageGenerationFailure.REFUSED:
      return `O serviço recusou este texto. Tente descrever a cena de outro jeito. ${back}`;
    case ImageGenerationFailure.UNAVAILABLE:
      return `O serviço de imagens não respondeu. Tente de novo em instantes. ${back}`;
    case ImageGenerationFailure.GALLERY_FULL:
      return `A imagem foi feita, mas a galeria não tinha espaço para guardá-la. Apague imagens que você não usa e peça de novo. ${back}`;
    case ImageGenerationFailure.IMAGE_MISSING:
      return `Uma imagem de referência foi apagada no meio do pedido. Escolha outra e peça de novo. ${back}`;
    case ImageGenerationFailure.TIMEOUT:
      return `O pedido se perdeu pelo caminho. Tente de novo. ${back}`;
    case ImageGenerationFailure.SERVICE_OFF:
      return `O serviço de imagens não está disponível agora. ${back}`;
    default:
      return `${generation.reasonPt || 'Não deu para gerar a imagem.'} ${back}`;
  }
}

/** The words of a failed "Usar como imagem do mapa". */
export function useIssue(err: unknown): string {
  const blocked = blockedOf(err);
  if (blocked) {
    return blockedText(blocked.reason, blocked.status);
  }
  return describeConnectError(err, {
    [Code.NotFound]: 'Esta imagem não é um mapa com textura de um mapa da campanha. Gere de novo.',
    [Code.ResourceExhausted]: 'A galeria está cheia e o mapa precisaria de uma cópia da imagem. Apague imagens que você não usa e tente de novo.',
  });
}
