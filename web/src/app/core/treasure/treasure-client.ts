import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import {
  type GenerateTreasureResponse,
  type GetMagicItemResponse,
  type GetTreasurePartyResponse,
  type PlaceTreasureResponse,
  TreasureMode,
  TreasureService,
} from '../../../gen/meurpg/maps/v1/treasure_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** What "Pôr no mapa" sends: the treasure as the page generated it (the server rolls it again from these), and where. */
export interface PlaceTreasureArgs {
  readonly campaignId: string;
  readonly mapId: string;
  readonly mode: TreasureMode;
  readonly partyLevel: number;
  readonly seed: bigint;
  readonly contentVersion: string;
  /** The square, in the rules' squares counted from 0 at the top left. */
  readonly column: number;
  readonly row: number;
  /** `''`: the server names the point ("Tesouro de covil" or "Tesouro individual"). */
  readonly name: string;
  /** One key per request: the same key and the same request give back the same point. */
  readonly idempotencyKey: string;
}

/**
 * Thin wrapper around the generated `TreasureService` client (MR-044, RN-09, RN-10): the master's
 * "Gerar tesouro" page, an item's description and "Pôr no mapa". Every call is the master's; a
 * player gets `not_found` from all of them, so the app never calls this as a player. The page maps
 * the errors to Portuguese (`treasure-errors.ts`). Tests replace it with `{ provide: TreasureClient, useValue }`.
 */
@Injectable({ providedIn: 'root' })
export class TreasureClient {
  private readonly client = createClient(TreasureService, inject(CONNECT_TRANSPORT));

  /** `GetTreasureParty`: "O grupo está nos níveis 4 e 5". */
  party(campaignId: string): Promise<GetTreasurePartyResponse> {
    return this.client.getTreasureParty({ campaignId });
  }

  /** `GenerateTreasure`: stores nothing. No seed asks the server to draw one ("Gerar outro"). */
  generate(campaignId: string, mode: TreasureMode, partyLevel: number, seed?: bigint): Promise<GenerateTreasureResponse> {
    return this.client.generateTreasure({ campaignId, mode, partyLevel, ...(seed === undefined ? {} : { seed }) });
  }

  /** `GetMagicItem`: "Ver descrição". */
  item(campaignId: string, key: string): Promise<GetMagicItemResponse> {
    return this.client.getMagicItem({ campaignId, key });
  }

  /** `PlaceTreasure`: a hidden treasure point on the map. */
  place(args: PlaceTreasureArgs): Promise<PlaceTreasureResponse> {
    return this.client.placeTreasure({
      campaignId: args.campaignId,
      mapId: args.mapId,
      mode: args.mode,
      partyLevel: args.partyLevel,
      seed: args.seed,
      column: args.column,
      row: args.row,
      ...(args.name === '' ? {} : { name: args.name }),
      idempotencyKey: args.idempotencyKey,
      contentVersion: args.contentVersion,
    });
  }
}
