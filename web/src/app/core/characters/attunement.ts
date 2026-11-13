import { Injectable, inject } from '@angular/core';
import { Code, ConnectError, createClient } from '@connectrpc/connect';

import {
  AttunementRestriction,
  CharacterBlockedReason,
  CharacterBlockedSchema,
  CharacterService,
  type FullSheet,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { ContentService, type MagicItemOption } from '../../../gen/meurpg/rules/v1/rules_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** The most items a character is attuned to at once (SRD 5.1); the server refuses a fourth. */
export const MAX_ATTUNED = 3;

/** Whether the character meets what the item asks, as the server decides it (the app never matches a restriction). */
export type RestrictionState = 'none' | 'met' | 'notMet' | 'unknown';

const STATES: Record<number, RestrictionState> = {
  [AttunementRestriction.UNSPECIFIED]: 'none',
  [AttunementRestriction.MET]: 'met',
  [AttunementRestriction.NOT_MET]: 'notMet',
  [AttunementRestriction.UNKNOWN]: 'unknown',
};

/** One thing of the sheet a character can be attuned to: a magic item or a +N piece of gear that names a catalog item. */
export interface AttunementEntryVm {
  /** The entry id `SetAttunement` names; two copies of one item are two entries. */
  readonly id: string;
  readonly key: string;
  /** The catalog name ("Manto élfico", "Defensora"); a piece without a catalog item has no entry here. */
  readonly namePt: string;
  readonly kind: 'magic' | 'piece';
  readonly attuned: boolean;
  /** The item asks for attunement; an item that does not still shows, and derives nothing. */
  readonly asks: boolean;
  /** What the item asks of the character, "Só para um conjurador"; empty without a restriction. */
  readonly restrictionPt: string;
  readonly restriction: RestrictionState;
  /** Another copy is attuned, so this one cannot be. */
  readonly sameItemAttuned: boolean;
}

/** The attunement of a sheet: the entries in the order the sheet lists them (pieces, then magic items). */
export interface AttunementVm {
  readonly entries: readonly AttunementEntryVm[];
  /** How many entries that ask for attunement are attuned: "Sintonizados: 2 de 3". */
  readonly attuned: number;
}

/** The attunement of a full sheet, as the server says it on every read. */
export function attunementOf(
  full: FullSheet,
  gearName: (key: string) => string = () => '',
): AttunementVm {
  const entries: AttunementEntryVm[] = [];
  for (const b of full.itemBonuses) {
    if (b.itemKey === '' && !b.requiresAttunement) {
      continue;
    }
    entries.push({
      id: b.id,
      key: b.itemKey || b.key,
      namePt: b.itemNamePt || gearName(b.key),
      kind: 'piece',
      attuned: b.attuned,
      asks: b.requiresAttunement,
      restrictionPt: b.restrictionPt,
      restriction: STATES[b.restriction] ?? 'none',
      sameItemAttuned: b.sameItemAttuned,
    });
  }
  for (const m of full.magicItems) {
    entries.push({
      id: m.id,
      key: m.key,
      namePt: m.namePt,
      kind: 'magic',
      attuned: m.attuned,
      asks: m.requiresAttunement,
      restrictionPt: m.restrictionPt,
      restriction: STATES[m.restriction] ?? 'none',
      sameItemAttuned: m.sameItemAttuned,
    });
  }
  return { entries, attuned: entries.filter((e) => e.asks && e.attuned).length };
}

/** A row of the picker "Adicionar item mágico": an item, or a family whose variants are what is added. */
export interface MagicItemOptionVm {
  readonly key: string;
  readonly namePt: string;
  readonly category: string;
  readonly rarity: string;
  readonly asks: boolean;
  readonly restrictionPt: string;
  readonly variants: readonly MagicItemVariantVm[];
  /** A family that can also be added as itself. */
  readonly standalone: boolean;
}

export interface MagicItemVariantVm {
  readonly key: string;
  readonly namePt: string;
  readonly rarity: string;
  readonly asks: boolean;
  readonly restrictionPt: string;
}

export interface MagicItemsPage {
  readonly items: readonly MagicItemOptionVm[];
  readonly total: number;
  readonly nextPageToken: string;
}

export function toMagicItemOptionVm(o: MagicItemOption): MagicItemOptionVm {
  return {
    key: o.key,
    namePt: o.namePt,
    category: o.category,
    rarity: o.rarity,
    asks: o.requiresAttunement,
    restrictionPt: o.restrictionPt,
    standalone: o.standalone,
    variants: o.variants.map((v) => ({
      key: v.key,
      namePt: v.namePt,
      rarity: v.rarity,
      asks: v.requiresAttunement,
      restrictionPt: v.restrictionPt,
    })),
  };
}

/** `SetAttunement` and the picker's `ListMagicItems`. `providedIn: 'root'`, imported by the sheet page and its dialogs;
 * tests replace it with `{ provide: AttunementClient, useValue }`. */
@Injectable({ providedIn: 'root' })
export class AttunementClient {
  private readonly characters = createClient(CharacterService, inject(CONNECT_TRANSPORT));
  private readonly content = createClient(ContentService, inject(CONNECT_TRANSPORT));

  /** Attunes to the entry, or ends it, at the revision the caller holds. The refusals are `ConnectError`s with
   * `CharacterBlocked` details (see `characterBlockedMessage`). */
  async set(
    campaignId: string,
    characterId: string,
    entryId: string,
    attuned: boolean,
    revision: number,
  ): Promise<void> {
    await this.characters.setAttunement({ campaignId, characterId, entryId, attuned, revision });
  }

  async listMagicItems(campaignId: string, query: string, pageToken = ''): Promise<MagicItemsPage> {
    const res = await this.content.listMagicItems({
      campaignId,
      query,
      pageSize: 400,
      pageToken,
    });
    return {
      items: res.items.map(toMagicItemOptionVm),
      total: res.total,
      nextPageToken: res.nextPageToken,
    };
  }
}

/** The server refused the attunement because the character does not meet what the item asks (or the sheet cannot say):
 * the row already says it in the app's words. */
export function isRestrictedRefusal(err: unknown): boolean {
  const e = ConnectError.from(err, Code.Unavailable);
  return (
    e.code === Code.FailedPrecondition &&
    e
      .findDetails(CharacterBlockedSchema)
      .some((d) => d.reason === CharacterBlockedReason.ATTUNEMENT_RESTRICTED)
  );
}
