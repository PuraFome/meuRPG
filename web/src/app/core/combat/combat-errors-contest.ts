import { ContestBlockedReason } from '../../../gen/meurpg/play/v1/contest_types_pb';

/** Why a contest or a special action was refused, in words (W7-X). Nothing here names a total, a DC or a creature the player does not see. */
export const CONTEST_BLOCKED_TEXT: Partial<Record<ContestBlockedReason, string>> = {
  [ContestBlockedReason.TARGET_TOO_BIG]: 'Grande demais: no máximo um tamanho acima do seu.',
  [ContestBlockedReason.NOT_AWAITING]:
    'Essa disputa não espera mais por isso. A tela foi atualizada.',
  [ContestBlockedReason.CONTEST_OPEN]: 'Uma disputa sua ainda espera a resposta.',
  [ContestBlockedReason.NOT_GRAPPLED]: 'Não há agarrão para soltar. A tela foi atualizada.',
  [ContestBlockedReason.PUSH_BLOCKED]: 'Há algo na casa de trás: o empurrão não sai do lugar.',
  [ContestBlockedReason.NO_ROOM_TO_DRAG]:
    'Não dá para arrastar por aí: não há casa livre atrás de você para quem você segura.',
  [ContestBlockedReason.SURPRISED]:
    'Surpresa: não se move, não age e não reage até o fim do turno.',
  [ContestBlockedReason.NOT_AVAILABLE]: 'Essa ação não está na sua ficha.',
  [ContestBlockedReason.NOT_AN_ALLY]: 'Só dá para ajudar um aliado que ainda está de pé.',
  [ContestBlockedReason.TASK_NOT_AVAILABLE]: 'Não dá para ajudar nessa tarefa.',
  [ContestBlockedReason.HIDE_NOT_PENDING]:
    'Esse esconderijo já foi decidido. A tela foi atualizada.',
  [ContestBlockedReason.GROUP_CHECK_OPEN]: 'Já há um teste aberto. Encerre-o antes de pedir outro.',
  [ContestBlockedReason.GROUP_CHECK_CLOSED]: 'O mestre já encerrou esse teste.',
  [ContestBlockedReason.NOT_IN_GROUP_CHECK]: 'O mestre não pediu esse teste ao seu personagem.',
  [ContestBlockedReason.ALREADY_ANSWERED]: 'Você já rolou esse teste.',
  [ContestBlockedReason.COMBAT_BEGUN]: 'O combate já começou.',
  [ContestBlockedReason.NOT_HIDDEN]: 'Ninguém está escondido aqui. A tela foi atualizada.',
  [ContestBlockedReason.NO_FREE_HAND]:
    'Na forma de fera não há mão livre para agarrar. Empurrar continua valendo. Nada foi gasto.',
};
