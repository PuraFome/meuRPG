import { Code } from '@connectrpc/connect';

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
