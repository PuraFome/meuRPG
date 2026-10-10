import { expect, type Locator, type Page } from '@playwright/test';

import { startSessionRPC } from './live-session-support';
import {
  canvasPng,
  createMapRPC,
  createPointRPC,
  placeTokenRPC,
  revealMapRPC,
  setCurrentMapRPC,
  tableForMaps,
  uploadImageRPC,
  type MapsTable,
} from './maps-support';
import { callRPC } from './support';

// Setup for the RP scene specs (Etapa 7, MR-015), through the API: these
// tests prove the scene screens, not the campaign, character and map forms
// other specs already cover. Every test makes its own campaign. The data is
// the artboards' table: "Estrada do Vale", the point "A carroça tombada" and
// Pensantus, whose Investigação is +6.

export interface SceneTable extends MapsTable {
  sessionId: string;
  mapId: string;
  /** "A carroça tombada": a revealed scene point. */
  cartId: string;
  /** "Posto da guarda": a hidden scene point. */
  guardId: string;
  /** "Vau do riacho": a revealed scene point with no actions. */
  fordId: string;
}

export interface ActionSpec {
  key: string;
  name?: string;
  dc?: number;
}

/** The five actions of the artboards, in order. */
export const cartActions: ActionSpec[] = [
  { key: 'skill:investigation', name: 'Procurar pistas na carroça', dc: 12 },
  { key: 'skill:survival', name: 'Seguir os rastros dos goblins', dc: 13 },
  { key: 'skill:animal-handling', name: 'Acalmar os cavalos' },
  { key: 'skill:perception' },
  { key: 'save:con', name: 'Resistir ao cheiro de fumaça', dc: 10 },
];

export async function addActionRPC(page: Page, table: { campaignId: string; mapId: string }, pointId: string, action: ActionSpec): Promise<void> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/AddSceneAction', {
    campaignId: table.campaignId,
    mapId: table.mapId,
    pointId,
    key: action.key,
    name: action.name ?? '',
    dc: action.dc ?? 0,
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/**
 * A table ready for a scene: Pensantus (the player's, with a token), an open
 * session and "Estrada do Vale" as its current map, with three scene points.
 * With `seed` the cart has the five actions of the artboards and the guard post
 * three of them; without it no point has an action (the editor test adds its own).
 * Master and player pages must be signed in as each; neither navigates.
 */
export async function tableForScenes(masterPage: Page, playerPage: Page, name: string, seed = true): Promise<SceneTable> {
  const base = await tableForMaps(masterPage, playerPage, name);
  const sessionId = await startSessionRPC(masterPage, base.campaignId);
  await masterPage.goto('/');
  const image = await uploadImageRPC(masterPage, base.campaignId, 'Estrada do Vale', await canvasPng(masterPage, 2000, 1400, 'Estrada do Vale'));
  const mapId = await createMapRPC(masterPage, base.campaignId, 'Estrada do Vale', image);
  await revealMapRPC(masterPage, base.campaignId, mapId);
  await setCurrentMapRPC(masterPage, base.campaignId, mapId);
  await placeTokenRPC(masterPage, base.campaignId, mapId, base.characterId, 2500, 7000);
  const cartId = await createPointRPC(masterPage, base.campaignId, mapId, {
    kind: 'SCENE',
    name: 'A carroça tombada',
    description: 'Uma carroça de mercador tombada na estrada. Há caixas espalhadas e rastros de botas na lama.',
    xBp: 4500,
    yBp: 4200,
    revealed: true,
  });
  const guardId = await createPointRPC(masterPage, base.campaignId, mapId, { kind: 'SCENE', name: 'Posto da guarda', description: 'Um posto abandonado.', xBp: 7200, yBp: 5600 });
  const fordId = await createPointRPC(masterPage, base.campaignId, mapId, { kind: 'SCENE', name: 'Vau do riacho', xBp: 6500, yBp: 1500, revealed: true });
  const table: SceneTable = { ...base, sessionId, mapId, cartId, guardId, fordId };
  if (seed) {
    for (const action of cartActions) {
      await addActionRPC(masterPage, table, cartId, action);
    }
    for (const action of cartActions.slice(0, 3)) {
      await addActionRPC(masterPage, table, guardId, action);
    }
  }
  return table;
}

/** Opens a scene through the API (what the master's picker does). */
export async function openSceneRPC(page: Page, campaignId: string, pointId: string): Promise<void> {
  const res = await callRPC(page, 'meurpg.play.v1.PlayService/OpenScene', { campaignId, pointId });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** Rolls an action as the player, through the API, with a typed face. */
export async function rollSceneRPC(page: Page, campaignId: string, actionId: string, d20Face: number): Promise<void> {
  const res = await callRPC(page, 'meurpg.play.v1.PlayService/RollSceneCheck', {
    campaignId,
    actionId,
    d20Face,
    idempotencyKey: crypto.randomUUID(),
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** The open scene as the caller sees it (`GetOpenScene`). */
export async function getOpenSceneRPC(page: Page, campaignId: string): Promise<{ scene?: { actions: { id: string; name: string; checkName: string }[]; rolls: unknown[] } }> {
  const res = await callRPC(page, 'meurpg.play.v1.PlayService/GetOpenScene', { campaignId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/** The ids of a point's actions by their title (the name, or the check's name), through `GetMap`. */
export async function sceneActionIdsRPC(page: Page, table: { campaignId: string; mapId: string }, pointId: string): Promise<Record<string, string>> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/GetMap', { campaignId: table.campaignId, mapId: table.mapId });
  expect(res.ok(), await res.text()).toBeTruthy();
  const points = ((await res.json()).points ?? []) as { id: string; sceneActions?: { id: string; name?: string; checkName: string }[] }[];
  const point = points.find((p) => p.id === pointId);
  return Object.fromEntries((point?.sceneActions ?? []).map((a) => [a.name || a.checkName, a.id]));
}

/** "Tentativas por jogador" of one action (1 to 5, 0 unlimited), as the editor's select saves it. */
export async function setAttemptsRPC(page: Page, table: { campaignId: string; mapId: string }, pointId: string, actionId: string, maxAttempts: number): Promise<void> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/UpdateSceneAction', {
    campaignId: table.campaignId,
    mapId: table.mapId,
    pointId,
    actionId,
    maxAttempts,
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** The scene's "Mostrar a CD aos jogadores" switch, as "Salvar ponto" saves it. */
export async function setShowDcRPC(page: Page, table: { campaignId: string; mapId: string }, pointId: string, showDc: boolean): Promise<void> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/UpdateMapPoint', {
    campaignId: table.campaignId,
    mapId: table.mapId,
    pointId,
    showDc,
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** A player rolls an action with a physical die: the sheet's "Digitar o resultado", then back to the scene. */
export async function rollTyped(player: Page, scene: Locator, action: string, face: number): Promise<void> {
  await scene.getByRole('button', { name: `Rolar ${action}` }).click();
  const sheet = player.getByRole('dialog', { name: `Rolar ${action}` });
  await sheet.getByRole('button', { name: 'Digitar o resultado' }).click();
  await sheet.getByLabel(/Role 1d20 para/).fill(String(face));
  await sheet.getByRole('button', { name: `Confirmar ${face}` }).click();
  await sheet.getByRole('button', { name: 'Voltar à cena' }).waitFor();
  await sheet.getByRole('button', { name: 'Voltar à cena' }).click();
  await expect(sheet).toBeHidden();
}

/** Uploads two pictures to the gallery and attaches them, in this order, to a scene point (`SetSceneImages`, "Imagens da cena"). */
export async function attachTwoImagesRPC(
  page: Page,
  table: { campaignId: string; mapId: string },
  pointId: string,
  names: [string, string] = ['Vista da carroça', 'Rastros na lama'],
): Promise<[string, string]> {
  const first = await uploadImageRPC(page, table.campaignId, names[0], await canvasPng(page, 640, 400, names[0], '#4b6b8a'));
  const second = await uploadImageRPC(page, table.campaignId, names[1], await canvasPng(page, 640, 400, names[1], '#6b8a4b'));
  await setSceneImagesRPC(page, table, pointId, [first, second]);
  return [first, second];
}

/** Replaces the images of a scene point. */
export async function setSceneImagesRPC(page: Page, table: { campaignId: string; mapId: string }, pointId: string, imageIds: string[]): Promise<void> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/SetSceneImages', { campaignId: table.campaignId, mapId: table.mapId, pointId, imageIds });
  expect(res.ok(), await res.text()).toBeTruthy();
}
