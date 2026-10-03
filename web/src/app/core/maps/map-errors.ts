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
