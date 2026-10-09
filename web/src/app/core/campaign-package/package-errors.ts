import { Code, ConnectError } from '@connectrpc/connect';

import {
  CampaignPackageBlockedReason,
  CampaignPackageBlockedSchema,
  type PreviewCampaignImportResponse,
} from '../../../gen/meurpg/campaignpackage/v1/campaignpackage_pb';
import {
  CampaignCreationRefusedReason,
  CampaignCreationRefusedSchema,
} from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { describeConnectError } from '../connect/connect-errors';

const IMAGES_OFF_TEXT =
  'Este servidor ainda não guarda pacotes de campanha: o armazenamento de arquivos está desligado. Peça a quem cuida do servidor para ligá-lo.';

/** Why a step of an import did not go through, by the Connect code and its typed detail (never the message). */
export type ImportIssue =
  /** The account has the most campaigns the server allows (`CampaignCreationRefused`, LIMIT_REACHED). */
  | { readonly kind: 'cap'; readonly max: number }
  /** The server only lets some people create campaigns (NOT_ALLOWED). */
  | { readonly kind: 'not-allowed' }
  /** `CreateCampaignFromImport` found problems in the package: the same preview the call before gave. */
  | { readonly kind: 'problems'; readonly preview: PreviewCampaignImportResponse }
  /** The upload is not there (`not_found`): it expired (1 hour) or was cancelled. */
  | { readonly kind: 'lost' }
  /**
   * Anything else, in words. `unknown`: the server could not say whether the change was saved.
   * `retryable`: the same call may work later (a server that was down); not a refusal of the request itself.
   */
  | {
      readonly kind: 'text';
      readonly text: string;
      readonly unknown: boolean;
      readonly retryable: boolean;
    };

/** Codes that say the request itself is refused: asking again changes nothing until the person does something. */
const REFUSALS: ReadonlySet<Code> = new Set([
  Code.InvalidArgument,
  Code.FailedPrecondition,
  Code.PermissionDenied,
  Code.Unauthenticated,
]);

export function importIssue(err: unknown): ImportIssue {
  const e = ConnectError.from(err, Code.Unavailable);
  const refusal = e.findDetails(CampaignCreationRefusedSchema)[0];
  if (refusal?.reason === CampaignCreationRefusedReason.LIMIT_REACHED) {
    return { kind: 'cap', max: refusal.maxCampaigns };
  }
  if (refusal?.reason === CampaignCreationRefusedReason.NOT_ALLOWED) {
    return { kind: 'not-allowed' };
  }
  if (e.code === Code.NotFound) {
    return { kind: 'lost' };
  }
  const blocked = e.findDetails(CampaignPackageBlockedSchema)[0];
  if (blocked?.reason === CampaignPackageBlockedReason.HAS_PROBLEMS && blocked.preview) {
    return { kind: 'problems', preview: blocked.preview };
  }
  const text = describeConnectError(err, {
    [Code.InvalidArgument]:
      'Esse arquivo não pôde ser enviado: o nome está vazio, ou ele passa de 200 MB.',
    [Code.FailedPrecondition]:
      blocked?.reason === CampaignPackageBlockedReason.INCOMPLETE
        ? 'O envio não terminou: faltam partes do arquivo. Escolha o arquivo de novo para continuar de onde parou.'
        : IMAGES_OFF_TEXT,
  });
  return { kind: 'text', text, unknown: e.code === Code.Unknown, retryable: !REFUSALS.has(e.code) };
}

/** The words of a failed export call (start, read or cancel). */
export function exportErrorText(err: unknown): string {
  return describeConnectError(err, { [Code.FailedPrecondition]: IMAGES_OFF_TEXT });
}
