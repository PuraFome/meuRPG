import { Code, ConnectError } from '@connectrpc/connect';

import { describeConnectError } from '../connect/connect-errors';

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
 * image (gallery.proto). Today it carries no detail naming the maps; when
 * the maps slice adds one, this is where its names go into the sentence.
 * Until then, and whenever the detail is missing, the sentence stays
 * general, which is still true and still says how to fix it.
 */
export function deleteRefusal(err: unknown): DeleteRefusal {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code === Code.NotFound) {
    return { gone: true };
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
