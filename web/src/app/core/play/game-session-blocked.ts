import { GameSessionBlockedReason } from '../../../gen/meurpg/play/v1/play_pb';

/**
 * What each `GameSessionBlocked` reason says in Portuguese: what happened and
 * what to do. The card of the campaign's session uses the first two (start and
 * end), the summary the third.
 */
export const GAME_SESSION_BLOCKED_TEXT: ReadonlyMap<GameSessionBlockedReason, string> = new Map([
  [GameSessionBlockedReason.NO_OPEN_SESSION, 'Não há nenhuma sessão em andamento nesta campanha.'],
  [
    GameSessionBlockedReason.SESSION_ALREADY_OPEN,
    'Já existe uma sessão em andamento nesta campanha.',
  ],
  [
    GameSessionBlockedReason.SESSION_NOT_ENDED,
    'Esta sessão ainda não terminou. O resumo aparece quando o mestre a encerrar.',
  ],
]);
