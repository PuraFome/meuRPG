import { IMAGES_OFF_TEXT } from '../campaign-package/package-errors';
import { uploadFailureMessage } from '../images/upload-errors';

/**
 * The Portuguese sentence of every `reason` in the JSON body of an HTTP route's
 * error (`{"code", "reason", "message"}`): the uploads, the downloads and the
 * voice of a line, which cannot carry a typed detail. The keys are exactly the
 * `body` reasons of proto/error-reasons.json and the constants of the Go
 * package `platform/rpcerr` (reasons.go); `error-reasons.spec.ts` and the Go test
 * in `internal/archtest` fail when one of the three differs.
 *
 * A screen reads the body's `reason`, never its `message` (English, for logs).
 * Each sentence says what happened and what to do.
 */
export const BODY_REASON_TEXT = {
  RATE_LIMITED: 'Muitas ações em pouco tempo. Espere alguns segundos e tente de novo.',
  BUSY: 'O servidor está desenhando outras partes do mapa agora. Espere alguns segundos e tente de novo.',
  UNSUPPORTED_TYPE: uploadFailureMessage('UNSUPPORTED_TYPE'),
  TOO_LARGE: uploadFailureMessage('TOO_LARGE'),
  DIMENSIONS: uploadFailureMessage('DIMENSIONS'),
  CORRUPT: uploadFailureMessage('CORRUPT'),
  MALFORMED_REQUEST: uploadFailureMessage('MALFORMED_REQUEST'),
  QUOTA: uploadFailureMessage('QUOTA'),
  IMAGES_OFF: IMAGES_OFF_TEXT,
  WRONG_LENGTH:
    'Uma parte do arquivo chegou com o tamanho errado. Escolha o arquivo de novo para continuar de onde parou.',
  OFF: 'O áudio das falas está desligado neste servidor.',
  DAILY_LIMIT_REACHED: 'O servidor já fez todos os áudios de hoje. Tente de novo amanhã.',
  UNAVAILABLE: 'Não deu para fazer o áudio desta fala agora. Tente de novo em instantes.',
  REFUSED: 'O serviço de áudio não aceitou o texto desta fala. Mude o texto e tente de novo.',
} as const;

export type BodyReason = keyof typeof BODY_REASON_TEXT;

/** The sentence of a body `reason`, or `null` for a word that is not in the list. */
export function bodyReasonText(reason: string): string | null {
  return Object.hasOwn(BODY_REASON_TEXT, reason) ? BODY_REASON_TEXT[reason as BodyReason] : null;
}
