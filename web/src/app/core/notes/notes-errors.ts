import { Code } from '@connectrpc/connect';

import { describeConnectError } from '../connect/connect-errors';

export const NOTE_MAX = 2000;

/** The Portuguese message for a failed notes call, by code (notes.proto lists
 * what each returns). `what` finishes "Não deu para …". `resource_exhausted`
 * is the 300-note limit; `invalid_argument` is a text out of 1 to 2.000 or a
 * scene the group has not discovered (the server answers both the same). */
export function noteErrorMessage(err: unknown, what = 'salvar a anotação'): string {
  return describeConnectError(err, {
    [Code.InvalidArgument]: `Não deu para ${what}: escreva de 1 a 2.000 caracteres e escolha uma cena que o grupo já descobriu.`,
    [Code.NotFound]: 'Essa anotação não existe mais. A lista foi atualizada.',
    [Code.ResourceExhausted]: 'Limite de 300 anotações. Apague uma para escrever outra.',
    [Code.Unavailable]: `Não deu para ${what}: o servidor não respondeu. Tente de novo.`,
  });
}

/** What is wrong with a note's text, in words, or `''` when it is fine. */
export function noteTextError(text: string): string {
  const length = [...text.trim()].length;
  if (length === 0) {
    return 'Escreva a anotação antes de salvar.';
  }
  if (length > NOTE_MAX) {
    return `A anotação passa de 2.000 caracteres: tem ${length}, tire ${length - NOTE_MAX}.`;
  }
  return '';
}
