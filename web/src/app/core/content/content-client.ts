import { Injectable, inject } from '@angular/core';
import type { MessageInitShape } from '@bufbuild/protobuf';
import { Code, ConnectError, createClient } from '@connectrpc/connect';

import { ContentService, type Content } from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  type AffectedCharacter,
  type CreateTableEntryRequestSchema,
  type GetClassTableDefaultsResponse,
  type GetEffectMenuResponse,
  type TableContentViolation,
  TableContentBlockedReason,
  TableContentBlockedSchema,
  TableContentRefusalSchema,
  TableContentService,
  type TableEntry,
  type UpdateTableEntryResponse,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** What an editor sends: exactly one body, which gives the kind (`CreateTableEntryRequest.body`), as the request takes it. */
export type EntryBody = NonNullable<MessageInitShape<typeof CreateTableEntryRequestSchema>['body']>;

export interface EntryList {
  readonly entries: readonly TableEntry[];
  readonly tableRevision: number;
}

/**
 * The table's content calls (MR-025, RN-23, ADR-0018): `TableContentService` for the entries in full and the master's
 * writes, `ContentService.ListContent` for the catalog the editors' pickers read (the schools, the classes, the skills).
 * `providedIn: 'root'`, imported only by lazy code. Nothing here knows a rule: the server checks every entry with the
 * rules engine and says what it refused, field by field.
 */
@Injectable({ providedIn: 'root' })
export class TableContentClient {
  private readonly client = createClient(TableContentService, inject(CONNECT_TRANSPORT));
  private readonly content = createClient(ContentService, inject(CONNECT_TRANSPORT));

  /** Every entry the caller may see: the master gets the archived ones and the counts, a player neither. */
  async list(campaignId: string): Promise<EntryList> {
    const res = await this.client.listTableEntries({ campaignId });
    return { entries: res.entries, tableRevision: res.tableRevision };
  }

  async create(campaignId: string, body: EntryBody): Promise<TableEntry> {
    const res = await this.client.createTableEntry({ campaignId, body });
    return res.entry as TableEntry;
  }

  async update(campaignId: string, key: string, expectedRevision: number, body: EntryBody): Promise<UpdateTableEntryResponse> {
    return this.client.updateTableEntry({ campaignId, key, expectedRevision, body });
  }

  /** Creates the entry, or replaces the body of the one the editor read (with the revision it read). */
  async save(
    campaignId: string,
    entry: TableEntry | null,
    body: EntryBody,
  ): Promise<{ entry: TableEntry; affected: readonly AffectedCharacter[] }> {
    if (!entry) {
      return { entry: await this.create(campaignId, body), affected: [] };
    }
    const res = await this.update(campaignId, entry.key, entry.revision, body);
    return { entry: res.entry as TableEntry, affected: res.affectedCharacters };
  }

  async archive(campaignId: string, key: string): Promise<TableEntry> {
    const res = await this.client.archiveTableEntry({ campaignId, key });
    return res.entry as TableEntry;
  }

  async unarchive(campaignId: string, key: string): Promise<TableEntry> {
    const res = await this.client.unarchiveTableEntry({ campaignId, key });
    return res.entry as TableEntry;
  }

  /** The closed menu of effects, as data (master only). */
  effectMenu(campaignId: string): Promise<GetEffectMenuResponse> {
    return this.client.getEffectMenu({ campaignId });
  }

  /** The numbers the class editor starts from: the SRD's proficiency bonus, the ASI levels and the table of each way of casting (master only). */
  classDefaults(campaignId: string): Promise<GetClassTableDefaultsResponse> {
    return this.client.getClassTableDefaults({ campaignId });
  }

  /** The campaign's catalog: the SRD's content and the table's, for the pickers. */
  async catalog(campaignId: string): Promise<Content> {
    const res = await this.content.listContent({ campaignId });
    return res.content as Content;
  }
}

/** The violations of a refused write (`invalid_argument` with a `TableContentRefusal`), or `null` for any other error. */
export function refusalOf(err: unknown): readonly TableContentViolation[] | null {
  const e = ConnectError.from(err, Code.Unavailable);
  if (e.code !== Code.InvalidArgument) {
    return null;
  }
  const detail = e.findDetails(TableContentRefusalSchema)[0];
  return detail ? detail.violations : null;
}

/** Why a write cannot be done now (`TableContentBlocked`), from `aborted` and `failed_precondition`; `null` for any other error. */
export function blockedReason(err: unknown): TableContentBlockedReason | null {
  const e = ConnectError.from(err, Code.Unavailable);
  if (e.code !== Code.Aborted && e.code !== Code.FailedPrecondition) {
    return null;
  }
  return e.findDetails(TableContentBlockedSchema)[0]?.reason ?? null;
}

/** The Portuguese words of an error that is not a refusal: stale, archived, no permission, offline. */
export function contentErrorText(err: unknown, what: string): string {
  const e = ConnectError.from(err, Code.Unavailable);
  const blocked = blockedReason(err);
  if (blocked === TableContentBlockedReason.STALE) {
    return 'Esta entrada mudou enquanto você editava. Recarregue para ver a versão nova.';
  }
  if (blocked === TableContentBlockedReason.ARCHIVED) {
    return 'Esta entrada já está arquivada.';
  }
  if (blocked === TableContentBlockedReason.NOT_ARCHIVED) {
    return 'Esta entrada não está arquivada.';
  }
  switch (e.code) {
    case Code.PermissionDenied:
      return 'Só o mestre da campanha muda o conteúdo da mesa.';
    case Code.NotFound:
      return 'Essa entrada, ou a campanha, não existe mais.';
    case Code.Unauthenticated:
      return 'Sua sessão acabou. Entre de novo para continuar.';
    default:
      return `Não foi possível ${what}. Confira a conexão e tente de novo.`;
  }
}

/** The save lost to another edit: the page offers to reload. */
export function isStale(err: unknown): boolean {
  return blockedReason(err) === TableContentBlockedReason.STALE;
}
