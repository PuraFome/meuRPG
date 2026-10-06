import { type MessageInitShape, create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { type MapPoint, MapPointKind } from '../../../gen/meurpg/maps/v1/maps_pb';
import {
  type GenerateTreasureResponse,
  type GetMagicItemResponse,
  GetMagicItemResponseSchema,
  type GetTreasurePartyResponse,
  GetTreasurePartyResponseSchema,
  MagicItemRarity,
  type PlaceTreasureResponse,
  TreasureCoinKind,
  TreasureMode,
  TreasureSchema,
  type Treasure,
  TreasureItemSchema,
  type TreasureItem,
} from '../../../gen/meurpg/maps/v1/treasure_pb';
import { mapPoint } from '../maps/maps-testing';
import type { PlaceTreasureArgs } from './treasure-client';

/**
 * Builders and a stand-in for the treasure specs (never imported by the app itself, so never bundled): a treasure as the server
 * returns it (the artboard E10-10's hoard for level 4, with its sums) and a `TreasureClient` that remembers its calls.
 */
export const CONTENT_VERSION = 'srd51@a8abc93b235c+fx.17';

export function treasureItem(partial: MessageInitShape<typeof TreasureItemSchema> = {}): TreasureItem {
  return create(TreasureItemSchema, {
    key: 'item:ring-of-protection',
    name: 'Ring of Protection',
    namePt: 'Anel de Proteção',
    category: 'ring',
    rarity: MagicItemRarity.RARE,
    valuePo: 4000,
    attunement: true,
    ...partial,
  });
}

/** The four items of the artboard: a Potion of Healing (50 PO, half of 100), a Cloak of Elvenkind, a Wand of Magic Missiles and a Ring of Protection. */
export function hoardItems(): TreasureItem[] {
  return [
    treasureItem({ key: 'item:potion-of-healing-1', name: 'Potion of Healing', namePt: 'Poção de Cura', category: 'potion', rarity: MagicItemRarity.COMMON, valuePo: 50, consumable: true, halved: true, attunement: false }),
    treasureItem({ key: 'item:cloak-of-elvenkind', name: 'Cloak of Elvenkind', namePt: 'Capa Élfica', category: 'wondrous-item', rarity: MagicItemRarity.UNCOMMON, valuePo: 400 }),
    treasureItem({ key: 'item:wand-of-magic-missiles', name: 'Wand of Magic Missiles', namePt: 'Varinha de Mísseis Mágicos', category: 'wand', rarity: MagicItemRarity.UNCOMMON, valuePo: 400, attunement: false }),
    treasureItem(),
  ];
}

/** The artboard's hoard for level 4: 340 PO and 1.200 PP of coins (460), gems (30), art (25): 515 PO of gold; the items 4.850 PO. */
export function sampleHoard(partial: MessageInitShape<typeof TreasureSchema> = {}): Treasure {
  return create(TreasureSchema, {
    mode: TreasureMode.HOARD,
    partyLevel: 4,
    seed: 2209n,
    coins: [
      { coin: TreasureCoinKind.SILVER, count: 1200, valuePo: 120 },
      { coin: TreasureCoinKind.GOLD, count: 340, valuePo: 340 },
    ],
    gems: [
      { namePt: 'Ágata', valuePo: 10, count: 2 },
      { namePt: 'Quartzo azul', valuePo: 10, count: 1 },
    ],
    art: [{ namePt: 'Cálice de prata gravado', valuePo: 25, count: 1 }],
    items: hoardItems(),
    coinsPo: 460,
    gemsPo: 30,
    artPo: 25,
    goldPo: 515,
    itemsPo: 4850,
    contentVersion: CONTENT_VERSION,
    ...partial,
  });
}

/** An individual treasure: coins only (24 PO and 90 PP are 33 PO). */
export function sampleIndividual(partial: MessageInitShape<typeof TreasureSchema> = {}): Treasure {
  return create(TreasureSchema, {
    mode: TreasureMode.INDIVIDUAL,
    partyLevel: 4,
    seed: 5148n,
    coins: [
      { coin: TreasureCoinKind.SILVER, count: 90, valuePo: 9 },
      { coin: TreasureCoinKind.GOLD, count: 24, valuePo: 24 },
    ],
    coinsPo: 33,
    goldPo: 33,
    contentVersion: CONTENT_VERSION,
    ...partial,
  });
}

export function partyResponse(partial: MessageInitShape<typeof GetTreasurePartyResponseSchema> = {}): GetTreasurePartyResponse {
  return create(GetTreasurePartyResponseSchema, { livingCount: 2, lowestLevel: 4, highestLevel: 5, ...partial });
}

export function magicItemResponse(partial: MessageInitShape<typeof GetMagicItemResponseSchema> = {}): GetMagicItemResponse {
  return create(GetMagicItemResponseSchema, {
    key: 'item:ring-of-protection',
    name: 'Ring of Protection',
    namePt: 'Anel de Proteção',
    category: 'ring',
    rarity: MagicItemRarity.RARE,
    attunement: true,
    valuePo: 4000,
    valueLabel: 'Valores do SRD 5.2.1 (regras de 2024)',
    description: ['Ring, rare (requires attunement)', 'You gain a +1 bonus to AC and saving throws while wearing this ring.'],
    ...partial,
  });
}

/** What the fake answers and remembers (`calls` has one line per call). */
export class FakeTreasureClient {
  calls: string[] = [];
  partyAnswer: GetTreasurePartyResponse = partyResponse();
  /** The treasure the next `generate` answers; a request with a seed answers the same treasure with that seed. */
  next: Treasure = sampleHoard();
  /** The seed the "server" draws for a request with none. */
  drawSeed = 2209n;
  items = new Map<string, GetMagicItemResponse>([['item:ring-of-protection', magicItemResponse()]]);
  /** The requests `generate` and `place` got. */
  generated: { mode: TreasureMode; level: number; seed: bigint | undefined }[] = [];
  placed: PlaceTreasureArgs[] = [];
  /** What each method rejects with, by name, once set. */
  failWith = new Map<string, unknown>();
  /** What a call waits for before it answers (to hold the busy state). */
  gate: Promise<void> | null = null;
  /** The point `place` answers with. */
  point: MapPoint = mapPoint('treasure-1', 'Tesouro de covil', { kind: MapPointKind.TREASURE, treasureValuePo: 515, revealed: false });

  private fail(method: string): void {
    const err = this.failWith.get(method);
    if (err) {
      throw err;
    }
  }

  async party(_campaignId: string): Promise<GetTreasurePartyResponse> {
    this.calls.push('party');
    this.fail('party');
    return this.partyAnswer;
  }

  async generate(_campaignId: string, mode: TreasureMode, level: number, seed?: bigint): Promise<GenerateTreasureResponse> {
    this.calls.push(`generate ${mode} ${level} ${seed === undefined ? 'no seed' : seed}`);
    this.generated.push({ mode, level, seed });
    await this.gate;
    this.fail('generate');
    const base = mode === this.next.mode ? this.next : mode === TreasureMode.HOARD ? sampleHoard() : sampleIndividual();
    return { $typeName: 'meurpg.maps.v1.GenerateTreasureResponse', treasure: create(TreasureSchema, { ...base, mode, partyLevel: level, seed: seed ?? this.drawSeed }) };
  }

  async item(_campaignId: string, key: string): Promise<GetMagicItemResponse> {
    this.calls.push(`item ${key}`);
    await this.gate;
    this.fail('item');
    const item = this.items.get(key);
    if (!item) {
      throw new ConnectError('not found', Code.NotFound);
    }
    return item;
  }

  async place(args: PlaceTreasureArgs): Promise<PlaceTreasureResponse> {
    this.calls.push(`place ${args.mapId} ${args.column},${args.row}`);
    this.placed.push(args);
    await this.gate;
    this.fail('place');
    return {
      $typeName: 'meurpg.maps.v1.PlaceTreasureResponse',
      point: this.point,
      treasure: create(TreasureSchema, { ...this.next, mode: args.mode, partyLevel: args.partyLevel, seed: args.seed }),
    };
  }
}
