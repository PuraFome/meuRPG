import { Code, ConnectError } from '@connectrpc/connect';

import { GameSessionBlockedReason, GameSessionBlockedSchema } from '../../../gen/meurpg/play/v1/play_pb';
import {
  type PuzzleBlocked,
  PuzzleBlockedReason,
  PuzzleBlockedSchema,
  type PuzzleInvalid,
  PuzzleInvalidReason,
  PuzzleInvalidSchema,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import { describeConnectError } from '../connect/connect-errors';

/** The typed detail of a `failed_precondition` from a puzzle call, or `null`. Never read from the message. */
export function puzzleBlocked(err: unknown): PuzzleBlocked | null {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code !== Code.FailedPrecondition) {
    return null;
  }
  return connectErr.findDetails(PuzzleBlockedSchema)[0] ?? null;
}

/** The typed detail of an `invalid_argument` from a puzzle call, or `null`. */
export function puzzleInvalid(err: unknown): PuzzleInvalid | null {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code !== Code.InvalidArgument) {
    return null;
  }
  return connectErr.findDetails(PuzzleInvalidSchema)[0] ?? null;
}

/** Whether a failed move may be sent again with the same key: the server did not answer (or could not finish). */
export function isTransient(err: unknown): boolean {
  const code = ConnectError.from(err, Code.Unavailable).code;
  return code === Code.Unavailable || code === Code.Aborted || code === Code.DeadlineExceeded || code === Code.Unknown;
}

/** What each refusal says, in words the master or the player can act on. */
export function puzzleBlockedMessage(reason: PuzzleBlockedReason): string {
  switch (reason) {
    case PuzzleBlockedReason.NO_OPEN_SESSION:
      return 'A sessão acabou: os quebra-cabeças só se jogam durante a sessão.';
    case PuzzleBlockedReason.SOLVED:
      return 'Este quebra-cabeça já foi resolvido.';
    case PuzzleBlockedReason.ALREADY_SHOWN:
      return 'Este quebra-cabeça já foi mostrado numa sessão e não pode mais ser editado.';
    case PuzzleBlockedReason.ARCHIVED:
      return 'Este quebra-cabeça está arquivado. Desarquive-o para mostrar.';
    case PuzzleBlockedReason.NOT_SHOWN:
      return 'Este quebra-cabeça não está sendo mostrado.';
    case PuzzleBlockedReason.NO_CHARACTER:
      return 'Você não tem um personagem vivo nesta campanha para jogar.';
    case PuzzleBlockedReason.NO_MORE_HINTS:
      return 'Não há mais dicas para soltar.';
    case PuzzleBlockedReason.NO_GENERATED_START:
      return 'O começo da fechadura é o que você escolheu: não há outro para gerar.';
    case PuzzleBlockedReason.STOPPED:
      return 'O quebra-cabeça parou. Ninguém joga mais até o mestre recomeçar ou fechar.';
    case PuzzleBlockedReason.NO_ATTEMPTS_LEFT:
      return 'Você não tem mais tentativas nesta rodada.';
    default:
      return 'O quebra-cabeça não está num estado que aceite isso. A tela foi atualizada.';
  }
}

/** The field a form's `PuzzleInvalid` points at, in the words a message under that field uses. */
export function puzzleInvalidMessage(invalid: PuzzleInvalid): string {
  switch (invalid.reason) {
    case PuzzleInvalidReason.NAME:
      return 'O nome precisa ter de 1 a 80 letras.';
    case PuzzleInvalidReason.TEXT:
      return 'Um dos textos passa do limite: a pista tem até 500 letras, cada dica até 300 e a mensagem até 200.';
    case PuzzleInvalidReason.SIZE:
      return 'O painel vai de 3 a 7 de lado, a fechadura de 2 a 6 rodas e os símbolos de 3 a 6 pilares e de 3 a 6 símbolos.';
    case PuzzleInvalidReason.SYMBOLS:
      return 'Uma roda ou um pilar mostra um símbolo que não existe. Confira o alfabeto e a quantidade.';
    case PuzzleInvalidReason.LINKS:
      return 'As ligações entre os pilares não valem: um pilar não gira junto consigo mesmo.';
    case PuzzleInvalidReason.START_SOLVED:
      return 'O começo é igual à solução. Gire uma roda para o começo ser outro.';
    case PuzzleInvalidReason.UNSOLVABLE:
      return 'Com essas ligações, os pilares não chegam ao mural. Mude o mural ou as ligações.';
    case PuzzleInvalidReason.TARGET:
      return 'O alvo do “Ao resolver” não é desta campanha, ou não existe mais. Escolha de novo.';
    case PuzzleInvalidReason.HINT_CHECK:
      return 'O teste de perícia da dica não vale.';
    default:
      return 'Confira os campos e tente de novo.';
  }
}

/**
 * The Portuguese message for a failed puzzle call, by code and typed detail (puzzles.proto lists what each call
 * returns). `what` finishes "Não deu para …".
 */
export function puzzleErrorMessage(err: unknown, what = 'fazer isso'): string {
  const blocked = puzzleBlocked(err);
  if (blocked) {
    return puzzleBlockedMessage(blocked.reason);
  }
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (
    connectErr.code === Code.FailedPrecondition &&
    connectErr.findDetails(GameSessionBlockedSchema)[0]?.reason === GameSessionBlockedReason.NO_OPEN_SESSION
  ) {
    return puzzleBlockedMessage(PuzzleBlockedReason.NO_OPEN_SESSION);
  }
  const invalid = puzzleInvalid(err);
  if (invalid) {
    return puzzleInvalidMessage(invalid);
  }
  return describeConnectError(err, {
    [Code.InvalidArgument]: `Não deu para ${what}: confira os campos e tente de novo.`,
    [Code.NotFound]: 'Este quebra-cabeça não existe mais, ou você não o vê. A tela foi atualizada.',
    [Code.PermissionDenied]: 'Você não pode fazer isso agora.',
    [Code.Unavailable]: `Não deu para ${what}: o servidor não respondeu. Tente de novo.`,
    [Code.Aborted]: `Não deu para ${what}: o servidor estava ocupado. Tente de novo.`,
  });
}
