import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  type BeginCampaignImportResponse,
  type CampaignExport,
  CampaignExportSchema,
  CampaignExportState,
  CampaignPackageBlockedReason,
  CampaignPackageBlockedSchema,
  type CampaignImport,
  CampaignImportSchema,
  type CancelCampaignExportResponse,
  type CancelCampaignImportResponse,
  type CreateCampaignFromImportResponse,
  type GetCampaignExportResponse,
  type GetCampaignImportResponse,
  type PackageCounts,
  PackageCountsSchema,
  type PreviewCampaignImportResponse,
  PreviewCampaignImportResponseSchema,
  type StartCampaignExportResponse,
} from '../../../gen/meurpg/campaignpackage/v1/campaignpackage_pb';
import {
  CampaignCreationRefusedReason,
  CampaignCreationRefusedSchema,
} from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { PartUploader } from './import-upload';

const KIB = 1024;
const MIB = KIB * KIB;
/** The size of the finished export of Mirathel, and of its images in the preview, in MiB. */
const EXPORT_MIB = 84.2;
const IMAGES_MIB = 82;
/** A fake part reports this much progress before it ends. */
const HALF = 0.5;

/**
 * Builders and stand-ins for the campaign package specs (never imported by the app itself, so never bundled): the
 * messages as the server sends them, and fakes for `CampaignPackageClient` and `ImportPartUploader`.
 */
export function exportOf(partial: Partial<Omit<CampaignExport, '$typeName'>> = {}): CampaignExport {
  return create(CampaignExportSchema, { id: 'exp-1', ...partial });
}

/** A finished export of Mirathel: 84,2 MB, 312 items, kept until `expiresAt`. */
export function doneExport(finishedAt: Date, expiresAt: Date): CampaignExport {
  return exportOf({
    state: CampaignExportState.DONE,
    fileName: 'Mirathel.meurpg.zip',
    byteSize: BigInt(Math.round(EXPORT_MIB * MIB)),
    entryCount: 312,
    finishedAt: timestampFromDate(finishedAt),
    expiresAt: timestampFromDate(expiresAt),
    downloadPath: '/downloads/campaign-exports/abc123',
  });
}

export function importUpload(
  partial: Partial<Omit<CampaignImport, '$typeName'>> = {},
): CampaignImport {
  return create(CampaignImportSchema, {
    id: 'imp-1',
    fileName: 'Mirathel.meurpg.zip',
    totalBytes: 40n,
    partSize: 10,
    partCount: 4,
    receivedParts: [],
    ...partial,
  });
}

export function counts(partial: Partial<Omit<PackageCounts, '$typeName'>> = {}): PackageCounts {
  return create(PackageCountsSchema, {
    maps: 6,
    npcs: 24,
    scenes: 9,
    puzzles: 3,
    battlePoints: 7,
    treasurePoints: 5,
    images: 58,
    imageBytes: BigInt(IMAGES_MIB * MIB),
    contentEntries: 6,
    characters: 4,
    ...partial,
  });
}

export function previewOf(
  partial: Partial<Omit<PreviewCampaignImportResponse, '$typeName'>> = {},
): PreviewCampaignImportResponse {
  return create(PreviewCampaignImportResponseSchema, {
    campaignName: 'Mirathel',
    // Noon, so the day is the same in every time zone.
    exportedAt: timestampFromDate(new Date('2026-10-08T12:00:00')),
    formatVersion: 1,
    counts: counts(),
    problems: [],
    ...partial,
  });
}

/** A `CampaignPackageClient` stand-in: set the results (functions, so a rejection is only made when called), read the calls. */
export class FakePackageClient {
  readonly calls: unknown[][] = [];
  startResult: () => Promise<StartCampaignExportResponse> = () =>
    Promise.resolve({ export: exportOf({ state: CampaignExportState.RUNNING }) } as never);
  getExportResult: () => Promise<GetCampaignExportResponse> = () =>
    Promise.resolve({ export: undefined, estimatedBytes: 88_000_000n, limitBytes: 0n } as never);
  cancelExportResult: () => Promise<CancelCampaignExportResponse> = () =>
    Promise.resolve({
      export: exportOf({ state: CampaignExportState.CANCELED }),
    } as never);
  beginResult: () => Promise<BeginCampaignImportResponse> = () =>
    Promise.resolve({ upload: importUpload() } as never);
  getImportResult: () => Promise<GetCampaignImportResponse> = () =>
    Promise.resolve({ upload: undefined, fingerprint: '' } as never);
  cancelImportResult: () => Promise<CancelCampaignImportResponse> = () =>
    Promise.resolve({} as never);
  previewResult: () => Promise<PreviewCampaignImportResponse> = () => Promise.resolve(previewOf());
  createResult: () => Promise<CreateCampaignFromImportResponse> = () =>
    Promise.resolve({
      campaign: { id: 'camp-new', name: 'Mirathel' },
      counts: counts(),
    } as never);

  startExport(campaignId: string, key: string) {
    this.calls.push(['startExport', campaignId, key]);
    return this.startResult();
  }

  getExport(campaignId: string) {
    this.calls.push(['getExport', campaignId]);
    return this.getExportResult();
  }

  cancelExport(campaignId: string, exportId: string) {
    this.calls.push(['cancelExport', campaignId, exportId]);
    return this.cancelExportResult();
  }

  beginImport(fileName: string, totalBytes: number, fingerprint: string) {
    this.calls.push(['beginImport', fileName, totalBytes, fingerprint]);
    return this.beginResult();
  }

  getImport() {
    this.calls.push(['getImport']);
    return this.getImportResult();
  }

  cancelImport(importId: string) {
    this.calls.push(['cancelImport', importId]);
    return this.cancelImportResult();
  }

  previewImport(importId: string) {
    this.calls.push(['previewImport', importId]);
    return this.previewResult();
  }

  createFromImport(importId: string, key: string) {
    this.calls.push(['createFromImport', importId, key]);
    return this.createResult();
  }

  /** The calls of one method, without its name. */
  callsOf(name: string): unknown[][] {
    return this.calls.filter((c) => c[0] === name).map((c) => c.slice(1));
  }
}

/** One part sent, as the fake saw it. */
export interface SentPart {
  readonly importId: string;
  readonly part: number;
  readonly size: number;
}

/**
 * An `ImportPartUploader` stand-in. `behave` decides what each call does (default: it succeeds, after reporting
 * half and then all of its progress); it gets the call and its 0-based attempt count for that part, and may
 * return a rejected promise (an `UploadFailed`) or one it settles by hand.
 */
export class FakePartUploader implements PartUploader {
  readonly sent: SentPart[] = [];
  private readonly attempts = new Map<number, number>();
  behave: (call: SentPart, attempt: number) => Promise<void> = () => Promise.resolve();

  upload(
    importId: string,
    partNumber: number,
    bytes: Blob,
    options: { onProgress?: (fraction: number) => void; signal?: AbortSignal } = {},
  ): Promise<void> {
    const attempt = this.attempts.get(partNumber) ?? 0;
    this.attempts.set(partNumber, attempt + 1);
    const call = { importId, part: partNumber, size: bytes.size };
    this.sent.push(call);
    options.onProgress?.(HALF);
    return this.behave(call, attempt).then(() => options.onProgress?.(1));
  }

  /** The numbers of the parts, in the order they were sent (a retry shows twice). */
  get partNumbers(): number[] {
    return this.sent.map((s) => s.part);
  }
}

/** A Connect error with a typed detail, as the server sends it (the message is English, for developers). */
function withDetail(
  code: Code,
  detail: Parameters<ConnectError['details']['push']>[0],
): ConnectError {
  const err = new ConnectError('English message for developers', code);
  err.details.push(detail);
  return err;
}

/** `resource_exhausted` or `permission_denied` with a `CampaignCreationRefused` detail (the campaign cap and its kin). */
export function creationRefused(
  reason: CampaignCreationRefusedReason,
  maxCampaigns = 10,
): ConnectError {
  const code =
    reason === CampaignCreationRefusedReason.NOT_ALLOWED
      ? Code.PermissionDenied
      : Code.ResourceExhausted;
  return withDetail(code, {
    desc: CampaignCreationRefusedSchema,
    value: create(CampaignCreationRefusedSchema, { reason, maxCampaigns }),
  });
}

/** `failed_precondition` with a `CampaignPackageBlocked` detail; HAS_PROBLEMS carries the preview. */
export function packageBlocked(
  reason: CampaignPackageBlockedReason,
  preview?: PreviewCampaignImportResponse,
): ConnectError {
  return withDetail(Code.FailedPrecondition, {
    desc: CampaignPackageBlockedSchema,
    value: create(CampaignPackageBlockedSchema, { reason, preview }),
  });
}
