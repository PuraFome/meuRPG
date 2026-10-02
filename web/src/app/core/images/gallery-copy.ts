import { Code, ConnectError } from '@connectrpc/connect';

import { ImageInUseSchema } from '../../../gen/meurpg/maps/v1/gallery_pb';
import { describeConnectError } from '../connect/connect-errors';
import { joinNames } from './image-format';

/** The platform's name rule for an image (gallery.proto,
 * `RenameGalleryImageRequest.name`): 1 to 80 characters, one line. */
export const IMAGE_NAME_MAX = 80;

/**
 * Checks a new image name the way the server will, so the field can say
 * what is wrong before the call. `null` when it is fine. The server trims
 * the name too, and stays the authority.
 */
export function imageNameError(name: string): string | null {
  const trimmed = name.trim();
  if (trimmed === '') {
    return 'Dê um nome à imagem.';
  }
  if ([...trimmed].length > IMAGE_NAME_MAX) {
    return `Use até ${IMAGE_NAME_MAX} caracteres.`;
  }
  // \p{Cc}: a line break, a tab or another control character.
  if (/\p{Cc}/u.test(trimmed)) {
    return 'Use um nome numa linha só.';
  }
  return null;
}

/** RenameGalleryImage's errors, in Portuguese. */
export function renameErrorMessage(err: unknown): string {
  return describeConnectError(err, {
    [Code.InvalidArgument]: `Use um nome de 1 a ${IMAGE_NAME_MAX} caracteres, numa linha só.`,
    [Code.NotFound]: 'Essa imagem não está mais na galeria. Recarregue a página.',
    [Code.PermissionDenied]: 'Só o mestre da campanha pode renomear imagens.',
  });
}

/** Why a delete was refused, for the card. `gone` means the image was
 * already deleted (another tab): the card just leaves the grid. */
export type DeleteRefusal =
  { readonly gone: true } | { readonly gone: false; readonly message: string };

/**
 * DeleteGalleryImage's errors. `failed_precondition` means a map uses the
 * image; its `ImageInUse` detail names the maps, so the sentence says which
 * ("Essa imagem é o fundo de Mirathel e arredores. Troque a imagem do mapa
 * antes de apagá-la."). Without the detail, the sentence stays general.
 */
export function deleteRefusal(err: unknown): DeleteRefusal {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code === Code.NotFound) {
    return { gone: true };
  }
  if (connectErr.code === Code.FailedPrecondition) {
    const [detail] = connectErr.findDetails(ImageInUseSchema);
    if (detail && detail.maps.length > 0) {
      const names = joinNames(detail.maps.map((m) => m.name));
      const plural = detail.maps.length > 1;
      return {
        gone: false,
        message: `Essa imagem é o fundo ${plural ? 'dos mapas' : 'de'} ${names}. Troque a imagem ${plural ? 'deles' : 'do mapa'} antes de apagá-la.`,
      };
    }
  }
  return {
    gone: false,
    message: describeConnectError(connectErr, {
      [Code.FailedPrecondition]:
        'Essa imagem está num mapa. Troque a imagem do mapa antes de apagá-la.',
      [Code.PermissionDenied]: 'Só o mestre da campanha pode apagar imagens.',
    }),
  };
}
