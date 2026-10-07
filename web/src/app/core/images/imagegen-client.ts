import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import {
  type GenerateMapImageRequest,
  type GenerateSceneImageRequest,
  type GetImageGenerationResponse,
  type GetMapImageReferenceResponse,
  type ImageEdit,
  type ImageGeneration,
  type ImageGenerationKind,
  ImageGenerationService,
  type ImageGenerationStatus,
} from '../../../gen/meurpg/maps/v1/imagegen_pb';
import type { Map as MapMessage } from '../../../gen/meurpg/maps/v1/maps_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** What a request carries besides the campaign: the form's fields, as `imagegen-form.ts` builds them. */
export type SceneRequest = Omit<GenerateSceneImageRequest, '$typeName' | '$unknown' | 'campaignId'>;
export type MapRequest = Omit<GenerateMapImageRequest, '$typeName' | '$unknown' | 'campaignId'>;

/** A request that went out: the PENDING request and the month after reserving its slot. */
export interface Started {
  readonly generation: ImageGeneration;
  readonly status: ImageGenerationStatus | undefined;
}

/**
 * Thin wrapper around the generated `ImageGenerationService` client (MR-039, RN-28, slices 10.8a and 10.8b): the master's "Gerar imagem"
 * dialog and the gallery's chain of edits. Every call is the master's (a player gets `not_found`), so the app never calls it as a player.
 * The browser generates nothing itself: it asks, waits with a long poll and shows what the server stored in the gallery.
 *
 * `providedIn: 'root'`, but only lazy code imports it, so the generated client stays out of the initial bundle. Callers map the errors by
 * their typed details (`imagegen-errors.ts`). Tests replace it with `{ provide: ImageGenClient, useValue: fake }`.
 */
@Injectable({ providedIn: 'root' })
export class ImageGenClient {
  private readonly client = createClient(ImageGenerationService, inject(CONNECT_TRANSPORT));

  /** `GetImageGenerationStatus`: works with generation off (`enabled` false). */
  async status(campaignId: string): Promise<ImageGenerationStatus> {
    const res = await this.client.getImageGenerationStatus({ campaignId });
    if (!res.status) {
      throw new Error('GetImageGenerationStatus answered without the status');
    }
    return res.status;
  }

  /** `GenerateSceneImage`: the scene art from the text alone. Answers at once with a PENDING request. */
  async generateScene(
    campaignId: string,
    request: SceneRequest,
    signal?: AbortSignal,
  ): Promise<Started> {
    const res = await this.client.generateSceneImage({ campaignId, ...request }, { signal });
    return started(res.generation, res.status);
  }

  /** `GenerateMapImage`: the scene art, the isometric view or the textured map of a map. */
  async generateMap(
    campaignId: string,
    request: MapRequest,
    signal?: AbortSignal,
  ): Promise<Started> {
    const res = await this.client.generateMapImage({ campaignId, ...request }, { signal });
    return started(res.generation, res.status);
  }

  /** `EditGeneratedImage`: "Pedir o ajuste". A new request that costs one slot. */
  async edit(
    campaignId: string,
    imageId: string,
    instruction: string,
    idempotencyKey: string,
    signal?: AbortSignal,
  ): Promise<Started> {
    const res = await this.client.editGeneratedImage(
      { campaignId, imageId, instruction, idempotencyKey },
      { signal },
    );
    return started(res.generation, res.status);
  }

  /** `GetImageGeneration` with `wait_seconds`: a long poll that answers when the state changes (0 to 25 seconds). */
  poll(
    campaignId: string,
    generationId: string,
    waitSeconds: number,
    signal?: AbortSignal,
  ): Promise<GetImageGenerationResponse> {
    return this.client.getImageGeneration({ campaignId, generationId, waitSeconds }, { signal });
  }

  /** `CancelImageGeneration`: before the request left the server the slot goes back (`slotSpent` false); after, only the wait stops. */
  async cancel(campaignId: string, generationId: string): Promise<Started> {
    const res = await this.client.cancelImageGeneration({ campaignId, generationId });
    return started(res.generation, res.status);
  }

  /** `ListImageEdits`: the chain of an image, oldest first. */
  async edits(campaignId: string, imageId: string): Promise<ImageEdit[]> {
    return (await this.client.listImageEdits({ campaignId, imageId })).edits;
  }

  /** `GetMapImageReference`: what a request made from a map would send, and the NPCs the players see. Uses no slot. */
  reference(
    campaignId: string,
    mapId: string,
    kind: ImageGenerationKind,
    signal?: AbortSignal,
  ): Promise<GetMapImageReferenceResponse> {
    return this.client.getMapImageReference({ campaignId, mapId, kind }, { signal });
  }

  /** `UseGeneratedImageAsMapImage`: the textured map becomes the map's image, with its layers kept. Returns the map as the master reads it. */
  async useAsMapImage(campaignId: string, imageId: string): Promise<MapMessage> {
    const res = await this.client.useGeneratedImageAsMapImage({ campaignId, imageId });
    if (!res.map) {
      throw new Error('UseGeneratedImageAsMapImage answered without the map');
    }
    return res.map;
  }
}

function started(
  generation: ImageGeneration | undefined,
  status: ImageGenerationStatus | undefined,
): Started {
  if (!generation) {
    throw new Error('The image service answered without the request');
  }
  return { generation, status };
}
