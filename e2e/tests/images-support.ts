import { expect, type Page } from '@playwright/test';

import { type FogTable, moveTo, tableForFog } from './fog-support';
import { callRPC } from './support';

// Setup for the generated-image specs (Etapa 10, slice 10.16, MR-039, RN-28, RN-10), through the API. The server runs with
// IMAGE_GENERATOR=fake (the native stack's default): a request answers in a moment with a small deterministic PNG, and nothing leaves the
// machine. The table is the fog cave with Pensantus beside the guard room, so the players see the goblins there, and the Goblin 2 hidden:
// the players' view lists Goblin 1 and the Capitão Goblin, never the hidden one.

const SERVICE = 'meurpg.maps.v1.ImageGenerationService';

export interface ImagesTable extends FogTable {
  /** The hidden NPC: the players do not see it, so the dialog never lists it. */
  hiddenId: string;
}

export async function tableForImages(master: Page, pensantusPlayer: Page, torenPlayer: Page, name: string, hideGoblin2 = true): Promise<ImagesTable> {
  const table = await tableForFog(master, pensantusPlayer, torenPlayer, name);
  // Beside the guard room, in the torch's light: Goblin 1, Goblin 2 and the Capitão are all in view...
  await moveTo(master, table, table.pensantusId, 17, 6);
  // ...until the master hides the Goblin 2.
  if (!hideGoblin2) {
    return { ...table, hiddenId: table.goblin2Id };
  }
  const hidden = await callRPC(master, 'meurpg.maps.v1.MapService/SetMapTokenHidden', {
    campaignId: table.campaignId,
    mapId: table.mapId,
    characterId: table.goblin2Id,
    hidden: true,
  });
  expect(hidden.ok(), await hidden.text()).toBeTruthy();
  return { ...table, hiddenId: table.goblin2Id };
}

export const mapRoute = (table: { campaignId: string; mapId: string }) => `/campanhas/${table.campaignId}/mapas/${table.mapId}`;

/** `GetMapImageReference` as the master calls it: the NPCs the players see now, and what the drawing says. */
export async function referenceRPC(page: Page, table: { campaignId: string; mapId: string }, kind: 'IMAGE_GENERATION_KIND_MAP_SCENE' | 'IMAGE_GENERATION_KIND_TEXTURED_MAP' | 'IMAGE_GENERATION_KIND_ISOMETRIC'): Promise<Record<string, any>> {
  const res = await callRPC(page, `${SERVICE}/GetMapImageReference`, { campaignId: table.campaignId, mapId: table.mapId, kind });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/** The month's status. */
export async function statusRPC(page: Page, campaignId: string): Promise<Record<string, any>> {
  const res = await callRPC(page, `${SERVICE}/GetImageGenerationStatus`, { campaignId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).status;
}

/** Spends one slot: a scene art made from a text alone, waited for until it is in the gallery (the fake answers at once). */
export async function generateSceneRPC(page: Page, campaignId: string, prompt: string): Promise<string> {
  const asked = await callRPC(page, `${SERVICE}/GenerateSceneImage`, { campaignId, idempotencyKey: crypto.randomUUID(), prompt, style: 'IMAGE_STYLE_INK' });
  expect(asked.ok(), await asked.text()).toBeTruthy();
  const generationId = (await asked.json()).generation.id as string;
  for (let i = 0; i < 40; i++) {
    const read = await callRPC(page, `${SERVICE}/GetImageGeneration`, { campaignId, generationId, waitSeconds: 5 });
    expect(read.ok(), await read.text()).toBeTruthy();
    const body = await read.json();
    if (body.generation.state !== 'IMAGE_GENERATION_STATE_PENDING') {
      expect(body.generation.state).toBe('IMAGE_GENERATION_STATE_DONE');
      return body.image.id as string;
    }
  }
  throw new Error('the image was never made');
}

/** What a player gets of the session: the image the master shows (none until the master shows one). */
export async function liveSessionAsPlayer(player: Page, campaignId: string): Promise<Record<string, any>> {
  const res = await callRPC(player, 'meurpg.play.v1.PlayService/GetLiveSession', { campaignId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/** Spends one slot: the textured map of the table's map, waited for until it is in the gallery. Its gallery image says it shows the whole map. */
export async function generateTextureRPC(page: Page, table: { campaignId: string; mapId: string }): Promise<string> {
  const asked = await callRPC(page, `${SERVICE}/GenerateMapImage`, {
    campaignId: table.campaignId,
    mapId: table.mapId,
    kind: 'IMAGE_GENERATION_KIND_TEXTURED_MAP',
    idempotencyKey: crypto.randomUUID(),
    prompt: 'Uma caverna de pedra clara',
  });
  expect(asked.ok(), await asked.text()).toBeTruthy();
  const generationId = (await asked.json()).generation.id as string;
  for (let i = 0; i < 40; i++) {
    const read = await callRPC(page, `${SERVICE}/GetImageGeneration`, { campaignId: table.campaignId, generationId, waitSeconds: 5 });
    expect(read.ok(), await read.text()).toBeTruthy();
    const body = await read.json();
    if (body.generation.state !== 'IMAGE_GENERATION_STATE_PENDING') {
      expect(body.generation.state).toBe('IMAGE_GENERATION_STATE_DONE');
      expect(body.image.showsWholeMap).toBe(true);
      return body.image.id as string;
    }
  }
  throw new Error('the image was never made');
}
