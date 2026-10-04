import { expect, type Page } from '@playwright/test';

import { callRPC } from './support';

// Setup for the clue and notes specs (Etapa 8, MR-029, MR-030), through the
// API: the specs prove the screens, so the data they need (clues on a point,
// notes) is also made here when the screens are not what is being proved.

/** The artboards' clues of "A carroça tombada", in order. */
export const cartClues = [
  'Um brasão de lobo queimado na lona da carroça.',
  'Rastros de três goblins e de botas pesadas, rumo ao norte.',
  'Uma carta rasgada: “…entregar no Vale Seco antes da lua cheia.”',
];

/** The master's private hooks of that scene. It must never reach a player. */
export const cartHooks = 'O mercador Aldo foi levado para a caverna do Vale Seco. Mira, a filha do Aldo, está escondida debaixo da carroça.';

export async function addClueRPC(page: Page, table: { campaignId: string; mapId: string }, pointId: string, text: string): Promise<string> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/AddSceneClue', { campaignId: table.campaignId, mapId: table.mapId, pointId, text });
  expect(res.ok(), await res.text()).toBeTruthy();
  const clues = (await res.json()).clues as { id: string }[];
  return clues[clues.length - 1].id;
}

export async function setHooksRPC(page: Page, table: { campaignId: string; mapId: string }, pointId: string, hooks: string): Promise<void> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/UpdateMapPoint', { campaignId: table.campaignId, mapId: table.mapId, pointId, hooks });
  expect(res.ok(), await res.text()).toBeTruthy();
}

export async function revealClueRPC(page: Page, campaignId: string, clueId: string, characterIds: string[]): Promise<void> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/RevealSceneClue', { campaignId, clueId, characterIds });
  expect(res.ok(), await res.text()).toBeTruthy();
}

export async function createNoteRPC(page: Page, campaignId: string, text: string, scenePointId = ''): Promise<string> {
  const res = await callRPC(page, 'meurpg.notes.v1.NotesService/CreateNote', { campaignId, text, scenePointId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).note.id as string;
}
