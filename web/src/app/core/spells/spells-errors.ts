import { Code, ConnectError } from '@connectrpc/connect';

import { describeConnectError } from '../connect/connect-errors';

/** `failed_precondition` of `ListSpells`: the character is a basic sheet (an NPC), which has no casting classes. */
export function isBasicSheet(err: unknown): boolean {
  return ConnectError.from(err, Code.Unavailable).code === Code.FailedPrecondition;
}

/**
 * The Portuguese sentence for a failed "Magias" call, by code and never by the server's message: what
 * happened and how to go on. A basic sheet is not an error of the page: it gets a sentence of its own
 * (`BASIC_SHEET_SENTENCE`) and a way out.
 */
export function spellsErrorMessage(err: unknown, action: 'list' | 'read' = 'list'): string {
  const what = action === 'read' ? 'abrir a descrição' : 'abrir as magias';
  return describeConnectError(err, {
    [Code.NotFound]:
      'Essa campanha não existe, ou você não é mais membro dela. Volte para Minhas campanhas.',
    [Code.InvalidArgument]: `Não deu para ${what}: confira a busca e os filtros e tente de novo.`,
    [Code.FailedPrecondition]: `Não deu para ${what} com esses filtros. Tire um filtro e tente de novo.`,
    [Code.Unavailable]: `Não deu para ${what}: o servidor não respondeu. Tente de novo.`,
  });
}

/** What the page says when "Só as que posso aprender" meets a sheet that has no classes. */
export const BASIC_SHEET_SENTENCE =
  'Essa ficha é básica e não tem classes que conjuram, então não há uma lista para aprender. Desligue “Só as que posso aprender” para ler todas as magias.';
