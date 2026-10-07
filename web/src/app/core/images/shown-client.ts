import { Injectable, inject } from '@angular/core';
import { Code, ConnectError, createClient } from '@connectrpc/connect';

import { PlayService } from '../../../gen/meurpg/play/v1/play_pb';
import { describeConnectError } from '../connect/connect-errors';
import { CONNECT_TRANSPORT } from '../connect/transport';

/**
 * "Mostrar aos jogadores" for an image of the gallery, from outside the session page (MR-028, E8-05): `PlayService.SetShownImage`, the same
 * call the session's "Imagem para os jogadores" panel makes. It needs an open session; a generated image stays hidden in the gallery until the
 * master asks for this. `providedIn: 'root'`, imported only by lazy code.
 */
@Injectable({ providedIn: 'root' })
export class ShownImageClient {
  private readonly play = createClient(PlayService, inject(CONNECT_TRANSPORT));

  async show(campaignId: string, imageId: string): Promise<void> {
    await this.play.setShownImage({ campaignId, imageId });
  }
}

/** SetShownImage's refusals in Portuguese: no open session is the usual one (`failed_precondition`). */
export function showImageIssue(err: unknown): string {
  if (ConnectError.from(err).code === Code.FailedPrecondition) {
    return 'Não há uma sessão aberta. Abra a sessão para mostrar a imagem aos jogadores: as imagens só são mostradas durante a sessão.';
  }
  return describeConnectError(err, {
    [Code.NotFound]: 'Essa imagem não está mais na galeria.',
    [Code.PermissionDenied]: 'Só o mestre da campanha mostra imagens.',
    [Code.ResourceExhausted]:
      'A galeria está cheia, e mostrar esta imagem precisaria de uma cópia dela. Apague imagens que você não usa.',
  });
}
