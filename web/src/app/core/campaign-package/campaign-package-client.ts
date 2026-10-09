import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { CampaignPackageService } from '../../../gen/meurpg/campaignpackage/v1/campaignpackage_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/**
 * Thin wrapper around the generated `CampaignPackageService` client (MR-050), in the same shape as
 * `CampaignsService`: the export calls are the campaign's master's, the import calls make a NEW campaign
 * whose master is the caller (campaignpackage.proto). Callers map the Connect codes and details to
 * Portuguese themselves (`package-errors.ts`).
 *
 * `providedIn: 'root'` is safe for the bundle: only the two lazy pages import it, so the generated
 * package code lands in their chunks. Tests replace it with `{ provide: CampaignPackageClient, useValue: fake }`.
 */
@Injectable({ providedIn: 'root' })
export class CampaignPackageClient {
  private readonly client = createClient(CampaignPackageService, inject(CONNECT_TRANSPORT));

  /** `idempotencyKey`: one per export action, sent again on a retry (see `ActionKey`). */
  startExport(campaignId: string, idempotencyKey: string) {
    return this.client.startCampaignExport({ campaignId, idempotencyKey });
  }

  getExport(campaignId: string) {
    return this.client.getCampaignExport({ campaignId });
  }

  cancelExport(campaignId: string, exportId: string) {
    return this.client.cancelCampaignExport({ campaignId, exportId });
  }

  beginImport(fileName: string, totalBytes: number, fingerprint: string) {
    return this.client.beginCampaignImport({
      fileName,
      totalBytes: BigInt(totalBytes),
      fingerprint,
    });
  }

  getImport() {
    return this.client.getCampaignImport({});
  }

  cancelImport(importId: string) {
    return this.client.cancelCampaignImport({ importId });
  }

  previewImport(importId: string) {
    return this.client.previewCampaignImport({ importId });
  }

  /** `idempotencyKey`: one per import, sent again on a retry (see `ActionKey`). */
  createFromImport(importId: string, idempotencyKey: string) {
    return this.client.createCampaignFromImport({ importId, idempotencyKey });
  }
}
