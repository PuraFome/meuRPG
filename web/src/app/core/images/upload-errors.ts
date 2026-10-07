import type { GalleryUsage } from '../../../gen/meurpg/maps/v1/gallery_pb';
import { formatBytes } from './image-format';

/** What the file picker offers, and the only types the server accepts
 * (gallery.proto, `POST /uploads/images`). */
export const ACCEPTED_IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/webp'] as const;

/** The `accept` attribute for `<input type="file">`. */
export const ACCEPT_ATTRIBUTE = ACCEPTED_IMAGE_TYPES.join(',');

/** The limits the app assumes before `ListGalleryImages` answers; the
 * server's `GalleryUsage` replaces them as soon as it does. */
export const DEFAULT_LIMITS = {
  maxImages: 300,
  maxBytes: 500 * 1024 * 1024,
  maxImageBytes: 10 * 1024 * 1024,
} as const;

/**
 * Why an upload did not end with a new image. The first seven are the
 * `reason`s of the upload route's JSON errors (docs/arquitetura.md, "Os
 * erros do envio"); the rest come from the HTTP status or the connection.
 */
export type UploadFailureKind =
  | 'UNSUPPORTED_TYPE'
  | 'TOO_LARGE'
  | 'DIMENSIONS'
  | 'CORRUPT'
  | 'MALFORMED_REQUEST'
  | 'QUOTA'
  | 'RATE_LIMITED'
  | 'UNAUTHENTICATED'
  | 'PERMISSION_DENIED'
  | 'NOT_FOUND'
  | 'UNAVAILABLE'
  | 'NETWORK'
  | 'CANCELED'
  | 'UNKNOWN';

const REASONS: ReadonlySet<string> = new Set([
  'UNSUPPORTED_TYPE',
  'TOO_LARGE',
  'DIMENSIONS',
  'CORRUPT',
  'MALFORMED_REQUEST',
  'QUOTA',
  'RATE_LIMITED',
]);

const CODES: Readonly<Record<string, UploadFailureKind>> = {
  unauthenticated: 'UNAUTHENTICATED',
  permission_denied: 'PERMISSION_DENIED',
  not_found: 'NOT_FOUND',
  unavailable: 'UNAVAILABLE',
  resource_exhausted: 'QUOTA',
};

const STATUSES: Readonly<Record<number, UploadFailureKind>> = {
  401: 'UNAUTHENTICATED',
  403: 'PERMISSION_DENIED',
  404: 'NOT_FOUND',
  413: 'TOO_LARGE',
  429: 'QUOTA',
  503: 'UNAVAILABLE',
};

/** The error an upload rejects with. */
export class UploadFailed extends Error {
  constructor(readonly kind: UploadFailureKind) {
    super(`upload failed: ${kind}`);
    this.name = 'UploadFailed';
  }
}

/**
 * Maps the upload route's error answer to a kind, by the JSON `reason` first,
 * then its Connect `code`, then the HTTP status. Never reads `message`: it
 * is English, for developers, and may change (docs/arquitetura.md).
 * A body that is not our JSON (the CSRF guard's plain-text 403, a proxy's
 * HTML page) falls back to the status alone.
 */
export function uploadFailureFromResponse(status: number, body: string): UploadFailureKind {
  let parsed: { code?: unknown; reason?: unknown } | null = null;
  try {
    parsed = JSON.parse(body) as { code?: unknown; reason?: unknown };
  } catch {
    parsed = null;
  }
  if (parsed && typeof parsed === 'object') {
    if (typeof parsed.reason === 'string' && REASONS.has(parsed.reason)) {
      return parsed.reason as UploadFailureKind;
    }
    if (typeof parsed.code === 'string' && CODES[parsed.code]) {
      return CODES[parsed.code];
    }
  }
  return STATUSES[status] ?? 'UNKNOWN';
}

type Limits = Pick<GalleryUsage, 'maxImages' | 'maxBytes' | 'maxImageBytes'>;

/**
 * The Portuguese sentence for a failure, shown after "Não deu para enviar
 * <arquivo>." (`uploadFailureNotice`). The limits come from the server's
 * `GalleryUsage`, so the numbers on screen follow the server's.
 */
export function uploadFailureMessage(
  kind: UploadFailureKind,
  limits: Limits = DEFAULT_LIMITS,
): string {
  switch (kind) {
    case 'UNSUPPORTED_TYPE':
      return 'Esse arquivo não é uma imagem JPEG, PNG ou WebP.';
    case 'TOO_LARGE':
      return `A imagem passa de ${formatBytes(limits.maxImageBytes)}.`;
    case 'DIMENSIONS':
      return 'A imagem é grande demais (mais de 8192 pixels de lado ou 40 megapixels).';
    case 'CORRUPT':
      return 'Não deu para ler essa imagem. Ela pode estar corrompida.';
    case 'QUOTA':
      return `A galeria está cheia: ${limits.maxImages} imagens ou ${formatBytes(limits.maxBytes)}.`;
    case 'RATE_LIMITED':
      return 'Muitos envios em pouco tempo. Espere alguns segundos e tente de novo.';
    case 'NETWORK':
      return 'A conexão caiu durante o envio. Tente de novo.';
    case 'UNAUTHENTICATED':
      return 'Sua sessão terminou. Entre de novo e envie outra vez.';
    case 'PERMISSION_DENIED':
      return 'Só o mestre da campanha pode enviar imagens.';
    case 'NOT_FOUND':
      return 'Essa campanha não existe, ou você não é membro dela.';
    case 'UNAVAILABLE':
      return 'O servidor não está recebendo imagens agora. Tente de novo em instantes.';
    case 'CANCELED':
      return 'Envio cancelado.';
    case 'MALFORMED_REQUEST':
    case 'UNKNOWN':
      return 'Algo deu errado no envio. Tente de novo em instantes.';
  }
}

/**
 * The fast, client-side half of the checks (README-B: "The client checks
 * type and size first, to fail fast; the server stays the authority").
 *
 * - Type: by the browser's `file.type`. An empty type (some systems don't
 *   know `.webp`) goes to the server, which sniffs the bytes anyway.
 * - Size: the file itself over `maxImageBytes`.
 * - Quota: only the image count, which is exact (`pendingCount` counts the
 *   files already waiting in line). Bytes only when the gallery is already
 *   at its byte limit: the server counts the image once encoded again,
 *   which can be much smaller than the file, so a byte check here would
 *   refuse files the server accepts.
 */
export function precheckImageFile(
  file: Pick<File, 'type' | 'size'>,
  usage: Pick<GalleryUsage, 'imageCount' | 'byteCount'> & Limits,
  pendingCount = 0,
): UploadFailureKind | null {
  if (file.type !== '' && !(ACCEPTED_IMAGE_TYPES as readonly string[]).includes(file.type)) {
    return 'UNSUPPORTED_TYPE';
  }
  if (file.size > usage.maxImageBytes) {
    return 'TOO_LARGE';
  }
  if (usage.imageCount + pendingCount >= usage.maxImages || usage.byteCount >= usage.maxBytes) {
    return 'QUOTA';
  }
  return null;
}
