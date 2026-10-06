import { type MessageInitShape, create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  DungeonAxis,
  DungeonDoorKind,
  type DungeonRoom,
  DungeonRoomSchema,
  DungeonSide,
  type GetDungeonRoomsResponse,
  GetDungeonRoomsResponseSchema,
  type PreviewDungeonResponse,
  PreviewDungeonResponseSchema,
} from '../../../gen/meurpg/maps/v1/dungeons_pb';
import type { Map as MapMessage, MapPoint } from '../../../gen/meurpg/maps/v1/maps_pb';
import { mapMessage, mapPoint } from './maps-testing';
import type { DungeonOptionsInit } from './dungeons-client';

/**
 * Builders and a stand-in for the dungeon specs (never imported by the app itself, so never bundled): a preview and a rooms list as the
 * server sends them, for a small dungeon of 11 × 9 squares with two rooms and a corridor between them, and a `DungeonsClient` that
 * remembers its calls.
 *
 * ```
 *   ###########
 *   #...#.....#      room 1: x 1..3, y 1..3 (door at 4,2, an archway)
 *   #...D.....#      room 2: x 5..9, y 1..3 (also the secret door at 7,4 to the corridor)
 *   #...#.....#
 *   #######S###
 *   #.........#      corridor: y 5
 *   ###########
 * ```
 */
export const SAMPLE_WIDTH = 11;
export const SAMPLE_HEIGHT = 9;

/** One bit a square, set where a creature can stand, in the byte layout of the walls layer. */
export function openBits(width: number, height: number, floors: readonly (readonly [number, number, number, number])[]): Uint8Array {
  const bytes = new Uint8Array(Math.ceil((width * height) / 8));
  for (const [x, y, w, h] of floors) {
    for (let row = y; row < y + h; row++) {
      for (let col = x; col < x + w; col++) {
        const n = row * width + col;
        bytes[n >> 3] = (bytes[n >> 3] ?? 0) | (1 << (n & 7));
      }
    }
  }
  return bytes;
}

export function previewResponse(partial: MessageInitShape<typeof PreviewDungeonResponseSchema> = {}): PreviewDungeonResponse {
  return create(PreviewDungeonResponseSchema, {
    seed: 48213n,
    generatorVersion: 1,
    width: SAMPLE_WIDTH,
    height: SAMPLE_HEIGHT,
    open: openBits(SAMPLE_WIDTH, SAMPLE_HEIGHT, [
      [1, 1, 3, 3],
      [5, 1, 5, 3],
      [4, 2, 1, 1],
      [7, 4, 1, 1],
      [1, 5, 9, 1],
    ]),
    doors: [
      { x: 4, y: 2, kind: DungeonDoorKind.ARCHWAY, axis: DungeonAxis.VERTICAL_WALL },
      { x: 7, y: 4, kind: DungeonDoorKind.SECRET, axis: DungeonAxis.HORIZONTAL_WALL },
      { x: 2, y: 4, kind: DungeonDoorKind.LOCKED, axis: DungeonAxis.HORIZONTAL_WALL, trapped: true },
      { x: 9, y: 4, kind: DungeonDoorKind.BARRED, axis: DungeonAxis.HORIZONTAL_WALL },
      { x: 5, y: 4, kind: DungeonDoorKind.CLOSED, axis: DungeonAxis.HORIZONTAL_WALL },
    ],
    stairs: [
      { x: 1, y: 1, up: true, facing: DungeonSide.SOUTH },
      { x: 9, y: 3, up: false, facing: DungeonSide.NORTH },
    ],
    rooms: [
      { id: 1, floor: { x: 1, y: 1, width: 3, height: 3 } },
      { id: 2, floor: { x: 5, y: 1, width: 5, height: 3 } },
    ],
    ...partial,
  });
}

export function room(id: number, partial: MessageInitShape<typeof DungeonRoomSchema> = {}): DungeonRoom {
  return create(DungeonRoomSchema, {
    id,
    floor: { x: 1, y: 1, width: 3, height: 3 },
    centerCol: 2,
    centerRow: 2,
    ...partial,
  } as MessageInitShape<typeof DungeonRoomSchema>);
}

/** The rooms list of the sample: room 1 with a trapped locked door, room 2 behind a secret door only, and a third room with a plain door. */
export function roomsResponse(partial: MessageInitShape<typeof GetDungeonRoomsResponseSchema> = {}): GetDungeonRoomsResponse {
  const p = previewResponse();
  return create(GetDungeonRoomsResponseSchema, {
    rooms: [
      room(1, {
        exits: [
          { side: DungeonSide.SOUTH, x: 2, y: 4, kind: DungeonDoorKind.LOCKED, trapped: true, otherRoomId: 0 },
          { side: DungeonSide.EAST, x: 4, y: 2, kind: DungeonDoorKind.ARCHWAY, trapped: false, otherRoomId: 2 },
        ],
      }),
      room(2, {
        floor: { x: 5, y: 1, width: 5, height: 3 },
        centerCol: 7,
        centerRow: 2,
        exits: [{ side: DungeonSide.SOUTH, x: 7, y: 4, kind: DungeonDoorKind.SECRET, trapped: false, otherRoomId: 0 }],
        scenePointIds: ['scene-1'],
      }),
      room(3, { floor: { x: 1, y: 6, width: 9, height: 1 }, centerCol: 5, centerRow: 6, exits: [{ side: DungeonSide.NORTH, x: 5, y: 4, kind: DungeonDoorKind.CLOSED, trapped: false, otherRoomId: 0 }] }),
    ],
    options: { width: SAMPLE_WIDTH, height: SAMPLE_HEIGHT },
    seed: 48213n,
    generatorVersion: 1,
    width: SAMPLE_WIDTH,
    height: SAMPLE_HEIGHT,
    imageIsGenerated: true,
    doors: p.doors,
    stairs: p.stairs,
    entrance: { x: 1, y: 1, onStairs: true },
    ...partial,
  } as MessageInitShape<typeof GetDungeonRoomsResponseSchema>);
}

/** What the fake answers and remembers (`calls` has one line per call). */
export class FakeDungeonsClient {
  calls: string[] = [];
  previews: PreviewDungeonResponse[] = [];
  /** The seed the "server" draws for a request with none. */
  drawSeed = 48213n;
  /** The requests the preview got: the options and the seed (`undefined` when none was sent). */
  previewRequests: { options: DungeonOptionsInit; seed: bigint | undefined }[] = [];
  /** What `rooms` answers; `null` is `not_found` (a map the generator did not make). */
  roomsAnswer: GetDungeonRoomsResponse | null = roomsResponse();
  /** What each method rejects with, by name, once set. */
  failWith = new Map<string, unknown>();
  /** What a call waits for before it answers (to hold the creating state). */
  gate: Promise<void> | null = null;
  createdMap: MapMessage = mapMessage('dungeon-1', 'Masmorra de Mirathel');
  redrawnMap: MapMessage = mapMessage('dungeon-1', 'Masmorra de Mirathel', { revision: 2 });
  placed: MapPoint | null = null;
  private nextPreview = 0;

  private fail(method: string): void {
    const err = this.failWith.get(method);
    if (err) {
      throw err;
    }
  }

  async preview(_campaignId: string, options: DungeonOptionsInit, seed?: bigint): Promise<PreviewDungeonResponse> {
    this.calls.push(`preview ${seed === undefined ? 'no seed' : seed}`);
    this.previewRequests.push({ options, seed });
    await this.gate;
    this.fail('preview');
    const next = this.previews[this.nextPreview++] ?? previewResponse();
    return create(PreviewDungeonResponseSchema, { ...next, seed: seed ?? this.drawSeed });
  }

  async create(_campaignId: string, name: string, options: DungeonOptionsInit, seed: bigint, signal?: AbortSignal): Promise<{ map: MapMessage; seed: bigint; roomCount: number }> {
    this.calls.push(`create ${name} ${seed}`);
    await this.gate;
    if (signal?.aborted) {
      throw new ConnectError('aborted', Code.Canceled);
    }
    this.fail('create');
    this.createdOptions = options;
    return { map: this.createdMap, seed, roomCount: 9 };
  }
  createdOptions: DungeonOptionsInit | null = null;

  async rooms(_campaignId: string, mapId: string): Promise<GetDungeonRoomsResponse> {
    this.calls.push(`rooms ${mapId}`);
    this.fail('rooms');
    if (this.roomsAnswer === null) {
      throw new ConnectError('not found', Code.NotFound);
    }
    return this.roomsAnswer;
  }

  async placeScene(_campaignId: string, mapId: string, roomId: number): Promise<MapPoint> {
    this.calls.push(`placeScene ${mapId} ${roomId}`);
    this.fail('placeScene');
    this.placed = mapPoint(`scene-room-${roomId}`, `Sala ${roomId}`, { mapId });
    return this.placed;
  }

  async redraw(_campaignId: string, mapId: string): Promise<MapMessage> {
    this.calls.push(`redraw ${mapId}`);
    this.fail('redraw');
    return this.redrawnMap;
  }
}
