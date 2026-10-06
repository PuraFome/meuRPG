import { expect, type Page } from '@playwright/test';

import { callRPC } from './support';

// Setup for the dungeon generator specs (Etapa 10, slice 10.14b, MR-010, RN-26, RN-10; E10-05 1 to 6). The campaign comes through the
// API; what is under test is the "Gerar masmorra" page, the rooms list and "Redesenhar", and what the server keeps and tells.

/** The "Gerar masmorra" page of a campaign. */
export function dungeonRoute(campaignId: string): string {
  return `/campanhas/${campaignId}/mapas/masmorra`;
}

/** Previews a dungeon through the API (the same call the page makes), as the master. */
export async function previewRPC(master: Page, campaignId: string, body: Record<string, unknown> = {}): Promise<Record<string, any>> {
  const res = await callRPC(master, 'meurpg.maps.v1.DungeonService/PreviewDungeon', { campaignId, ...body });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/** Creates a generated dungeon's map through the API; returns the map's ID. */
export async function createDungeonRPC(master: Page, campaignId: string, name: string, body: Record<string, unknown> = {}): Promise<string> {
  const res = await callRPC(master, 'meurpg.maps.v1.DungeonService/CreateDungeonMap', { campaignId, name, ...body });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).map.id as string;
}

/** `GetDungeonRooms`, as the master. */
export async function roomsRPC(master: Page, campaignId: string, mapId: string): Promise<Record<string, any>> {
  const res = await callRPC(master, 'meurpg.maps.v1.DungeonService/GetDungeonRooms', { campaignId, mapId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/** The squares set in a packed one-bit layer. */
export function bitAt(packed: string | undefined, columns: number, col: number, row: number): number {
  const bytes = Buffer.from(packed ?? '', 'base64');
  const n = row * columns + col;
  return ((bytes[n >> 3] ?? 0) >> (n & 7)) & 1;
}

/** A generated dungeon's walls layer and door count, as `GetMapLayers` gives the caller. */
export async function layersOfDungeon(page: Page, campaignId: string, mapId: string): Promise<{ columns: number; rows: number; wall: string; doors: number }> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/GetMapLayers', { campaignId, mapId });
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  const columns = (body.gridColumns ?? 0) as number;
  const rows = (body.gridRows ?? 0) as number;
  const doors = Buffer.from(body.doors ?? '', 'base64');
  let count = 0;
  for (let n = 0; n < columns * rows; n++) {
    const byte = doors[n >> 1] ?? 0;
    if (((n & 1 ? byte >> 4 : byte) & 15) !== 0) {
      count++;
    }
  }
  return { columns, rows, wall: body.wall ?? '', doors: count };
}
