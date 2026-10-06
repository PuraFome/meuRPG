import { expect, type Page } from '@playwright/test';

import { callRPC } from './support';

// Setup for the treasure specs (Etapa 10, slice 10.17c, MR-044, MR-041, RN-09, RN-10; E10-10), through the API. What is under
// test is the "Tesouro" page, an item's description and "Pôr no mapa", and what the server keeps and tells.

/** The "Tesouro" page of a campaign. */
export function treasureRoute(campaignId: string): string {
  return `/campanhas/${campaignId}/tesouro`;
}

/** `GenerateTreasure` through the API, as the master (the same call the page makes). */
export async function generateTreasureRPC(
  master: Page,
  campaignId: string,
  body: { mode: 'TREASURE_MODE_HOARD' | 'TREASURE_MODE_INDIVIDUAL'; partyLevel?: number; seed?: string },
): Promise<Record<string, any>> {
  const res = await callRPC(master, 'meurpg.maps.v1.TreasureService/GenerateTreasure', { campaignId, ...body });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).treasure;
}

/** The points of a map, as `GetMap` gives the caller (the raw JSON text too, to prove what a player never receives). */
export async function mapPointsRPC(page: Page, campaignId: string, mapId: string): Promise<{ points: Record<string, any>[]; text: string }> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/GetMap', { campaignId, mapId });
  expect(res.ok(), await res.text()).toBeTruthy();
  const text = await res.text();
  return { points: (JSON.parse(text).points ?? []) as Record<string, any>[], text };
}

/** Puts a grid on a map made from a plain image (the "Pôr no mapa" square needs one). */
export async function gridRPC(page: Page, campaignId: string, mapId: string, columns: number): Promise<void> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/SetMapGrid', { campaignId, mapId, columns });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** The campaign's XP of a character, to prove what "Voltar à cidade" gave. */
export async function markFoundRPC(page: Page, campaignId: string, mapId: string, pointId: string, characterIds: string[]): Promise<void> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/MarkTreasureFound', { campaignId, mapId, pointId, characterIds });
  expect(res.ok(), await res.text()).toBeTruthy();
}
