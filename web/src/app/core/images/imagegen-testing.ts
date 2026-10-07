import { type MessageInitShape, create } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import { Code, ConnectError } from '@connectrpc/connect';

import type { GalleryImage } from '../../../gen/meurpg/maps/v1/gallery_pb';
import {
  type GetImageGenerationResponse,
  GetImageGenerationResponseSchema,
  type GetMapImageReferenceResponse,
  GetMapImageReferenceResponseSchema,
  type ImageEdit,
  ImageEditSchema,
  type ImageGeneration,
  ImageGenerationBlockedReason,
  ImageGenerationBlockedSchema,
  ImageGenerationInvalidFieldSchema,
  ImageGenerationKind,
  ImageGenerationSchema,
  ImageGenerationState,
  type ImageGenerationStatus,
  ImageGenerationStatusSchema,
  MapImageCreatureSchema,
} from '../../../gen/meurpg/maps/v1/imagegen_pb';
import type { Map as MapMessage } from '../../../gen/meurpg/maps/v1/maps_pb';
import { galleryImage } from './gallery-testing';
import type { MapRequest, SceneRequest, Started } from './imagegen-client';

/**
 * Builders and a stand-in for the generated-image specs (never imported by the app itself, so never bundled): the month's status, a request in
 * each state, a map's reference with its NPCs, the typed refusals as the server sends them, and an `ImageGenClient` that remembers its calls
 * and answers what the test sets.
 */
export function imageStatus(
  partial: MessageInitShape<typeof ImageGenerationStatusSchema> = {},
): ImageGenerationStatus {
  return create(ImageGenerationStatusSchema, {
    enabled: true,
    monthlyLimit: 20,
    usedThisMonth: 3,
    remaining: 17,
    month: '2026-10',
    // 1 November 2026, 00:00 in Brazil's time (UTC-3).
    resetsAt: timestampFromMs(Date.UTC(2026, 10, 1, 3)),
    maxPromptCharacters: 500,
    maxObjectReferences: 10,
    maxCharacterReferences: 4,
    ...partial,
  });
}

export function generation(
  partial: MessageInitShape<typeof ImageGenerationSchema> = {},
): ImageGeneration {
  return create(ImageGenerationSchema, {
    id: 'gen-1',
    campaignId: 'camp-1',
    kind: ImageGenerationKind.SCENE,
    state: ImageGenerationState.PENDING,
    number: 1,
    slotSpent: true,
    ...partial,
  });
}

export function reference(
  partial: MessageInitShape<typeof GetMapImageReferenceResponseSchema> = {},
): GetMapImageReferenceResponse {
  return create(GetMapImageReferenceResponseSchema, {
    // A one-pixel PNG: the app only turns the bytes into a data URL.
    preview: Uint8Array.from([137, 80, 78, 71, 13, 10, 26, 10]),
    previewContentType: 'image/png',
    playersSeeSomething: true,
    seenSquares: 120,
    totalSquares: 651,
    gridColumns: 31,
    gridRows: 21,
    ...partial,
  });
}

export const NPCS = [
  create(MapImageCreatureSchema, {
    characterId: 'npc-capitao',
    name: 'Capitão Goblin',
    portraitImageId: 'img-capitao',
  }),
  create(MapImageCreatureSchema, { characterId: 'npc-goblin1', name: 'Goblin 1' }),
  create(MapImageCreatureSchema, {
    characterId: 'npc-vesna',
    name: 'Vesna, a capitã',
    portraitImageId: 'img-vesna',
  }),
];

export function edit(image: GalleryImage, number: number, prompt = ''): ImageEdit {
  return create(ImageEditSchema, { image, number, prompt });
}

/** The error the server sends for a refusal with a typed `ImageGenerationBlocked`. */
export function blocked(
  reason: ImageGenerationBlockedReason,
  status: ImageGenerationStatus = imageStatus(),
): ConnectError {
  return new ConnectError('blocked', Code.FailedPrecondition, undefined, [
    {
      desc: ImageGenerationBlockedSchema,
      value: create(ImageGenerationBlockedSchema, { reason, status }),
    },
  ]);
}

/** The error for a field that breaks a rule (`invalid_argument` with `ImageGenerationInvalidField`). */
export function invalid(field: string): ConnectError {
  return new ConnectError('invalid', Code.InvalidArgument, undefined, [
    {
      desc: ImageGenerationInvalidFieldSchema,
      value: create(ImageGenerationInvalidFieldSchema, { field }),
    },
  ]);
}

export type FakeAsk =
  | { readonly via: 'scene'; readonly request: SceneRequest }
  | { readonly via: 'map'; readonly request: MapRequest }
  | { readonly via: 'edit'; readonly imageId: string; readonly instruction: string };

/** An `ImageGenClient` stand-in. `started` answers the asking calls; `polls` are the answers of the long polls, in order (the last one repeats). */
export class FakeImageGenClient {
  statusResult: ImageGenerationStatus | Error = imageStatus();
  started: Started | Error = {
    generation: generation(),
    status: imageStatus({ usedThisMonth: 4, remaining: 16 }),
  };
  polls: (GetImageGenerationResponse | Error)[] = [];
  cancelResult: Started | Error = {
    generation: generation({ state: ImageGenerationState.CANCELED, slotSpent: false }),
    status: imageStatus(),
  };
  editsResult: ImageEdit[] = [];
  references = new Map<ImageGenerationKind, GetMapImageReferenceResponse | Error>();
  useResult: MapMessage | Error | null = null;

  readonly asks: FakeAsk[] = [];
  /** The idempotency key of every ask, in order, and how many slots the asks spent (a key the server knows spends none). */
  readonly keys: string[] = [];
  slotsSpent = 0;
  /** The next ask is made by the server but its answer is lost on the way (`unavailable`). */
  loseNextAnswer = false;
  private readonly byKey = new Map<string, Started>();
  readonly polled: { generationId: string; waitSeconds: number }[] = [];
  readonly canceled: string[] = [];
  readonly used: string[] = [];
  readonly referenced: ImageGenerationKind[] = [];
  /** A poll that waits until the test calls `release`. */
  hold: Promise<void> | null = null;
  private pollIndex = 0;

  async status(_campaignId: string): Promise<ImageGenerationStatus> {
    return unwrap(this.statusResult);
  }

  /** The server's rule for a key it knows: the request it made comes back, and no second slot is spent. */
  private answer(key: string): Started {
    this.keys.push(key);
    let started = this.byKey.get(key);
    if (!started) {
      started = unwrap(this.started);
      this.byKey.set(key, started);
      this.slotsSpent++;
    }
    if (this.loseNextAnswer) {
      // The server made the request, and the answer never got back.
      this.loseNextAnswer = false;
      throw new ConnectError('the answer was lost', Code.Unavailable);
    }
    return started;
  }

  async generateScene(
    _campaignId: string,
    request: SceneRequest,
    _signal?: AbortSignal,
  ): Promise<Started> {
    this.asks.push({ via: 'scene', request });
    return this.answer(request.idempotencyKey);
  }

  async generateMap(
    _campaignId: string,
    request: MapRequest,
    _signal?: AbortSignal,
  ): Promise<Started> {
    this.asks.push({ via: 'map', request });
    return this.answer(request.idempotencyKey);
  }

  async edit(
    _campaignId: string,
    imageId: string,
    instruction: string,
    key = '',
    _signal?: AbortSignal,
  ): Promise<Started> {
    this.asks.push({ via: 'edit', imageId, instruction });
    return this.answer(key);
  }

  async poll(
    _campaignId: string,
    generationId: string,
    waitSeconds: number,
    signal?: AbortSignal,
  ): Promise<GetImageGenerationResponse> {
    this.polled.push({ generationId, waitSeconds });
    if (this.hold && waitSeconds > 0) {
      await new Promise<void>((resolve, reject) => {
        signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')));
        void this.hold!.then(resolve);
      });
    }
    const next = this.polls[Math.min(this.pollIndex++, this.polls.length - 1)];
    if (!next) {
      // Still pending: a real long poll waits, so a test that never settles the request does not spin the event loop.
      await new Promise((r) => setTimeout(r, 5));
      return create(GetImageGenerationResponseSchema, { generation: generation() });
    }
    return unwrap(next);
  }

  async cancel(_campaignId: string, generationId: string): Promise<Started> {
    this.canceled.push(generationId);
    return unwrap(this.cancelResult);
  }

  async edits(_campaignId: string, _imageId: string): Promise<ImageEdit[]> {
    return this.editsResult;
  }

  async reference(
    _campaignId: string,
    _mapId: string,
    kind: ImageGenerationKind,
  ): Promise<GetMapImageReferenceResponse> {
    this.referenced.push(kind);
    const res = this.references.get(kind) ?? reference();
    return unwrap(res);
  }

  async useAsMapImage(_campaignId: string, imageId: string): Promise<MapMessage> {
    this.used.push(imageId);
    if (this.useResult === null) {
      throw new Error('useResult not set');
    }
    return unwrap(this.useResult);
  }
}

/** A finished request with its picture, as `GetImageGeneration` answers. */
export function done(
  imageId = 'img-gen-1',
  number = 1,
  partial: MessageInitShape<typeof ImageGenerationSchema> = {},
  imagePartial: Partial<Omit<GalleryImage, '$typeName'>> = {},
): GetImageGenerationResponse {
  const image = galleryImage(imageId, `Imagem ${number}`, { generated: true, ...imagePartial });
  return create(GetImageGenerationResponseSchema, {
    generation: generation({ state: ImageGenerationState.DONE, imageId, number, ...partial }),
    image,
    status: imageStatus({ usedThisMonth: 4, remaining: 16 }),
  });
}

function unwrap<T>(value: T | Error): T {
  if (value instanceof Error) {
    throw value;
  }
  return value;
}
