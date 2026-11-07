import { expect, type BrowserContext, type Locator, type Page } from '@playwright/test';

import { openSessionMenu, startSessionRPC, tableWithPensantus } from './live-session-support';
import { canvasPng, createMapRPC, createPointRPC, revealMapRPC, setCurrentMapRPC, uploadImageRPC } from './maps-support';
import { openSceneRPC } from './scene-support';
import { callRPC } from './support';

// Setup and helpers for the Tela da mesa specs (#648): the master's control in the live session, the TV tab it opens and what that
// tab asks of the server. Data goes through the API; the control and the TV tab are what is under test.

/** The route of the TV tab, the one address the "Copiar o link" button puts on the clipboard. */
export const tvRoute = (campaignId: string): string => `/campaigns/${campaignId}/table-screen`;

export interface TvTable {
  campaignId: string;
  campaignName: string;
}

/** A campaign of Mestre Teste's with Jogador Teste (Pensantus) in it and its session open: no map, no scene, no image, so the TV is idle. */
export async function tableForTv(master: Page, player: Page, name: string): Promise<TvTable> {
  const table = await tableWithPensantus(master, player, name);
  await startSessionRPC(master, table.campaignId);
  return { campaignId: table.campaignId, campaignName: name };
}

export interface TvContent {
  mapId: string;
  /** "A carroça tombada": a revealed scene point of the map. */
  sceneId: string;
  /** The gallery image "Capitão Goblin", not shown yet. */
  imageId: string;
}

/** A revealed map "Estrada do Vale" with a scene point, and a gallery image; none of them is on the table until a test puts it there. */
export async function addTvContent(master: Page, campaignId: string): Promise<TvContent> {
  const mapImage = await uploadImageRPC(master, campaignId, 'Estrada do Vale', await canvasPng(master, 1600, 1000, 'Estrada do Vale'));
  const imageId = await uploadImageRPC(master, campaignId, 'Capitão Goblin', await canvasPng(master, 600, 800, 'Capitão Goblin', '#3a3a2a'));
  const mapId = await createMapRPC(master, campaignId, 'Estrada do Vale', mapImage);
  await revealMapRPC(master, campaignId, mapId);
  const sceneId = await createPointRPC(master, campaignId, mapId, {
    kind: 'SCENE',
    name: 'A carroça tombada',
    description: 'Uma carroça de mercador tombada na estrada. Há caixas espalhadas e rastros de botas na lama.',
    xBp: 4500,
    yBp: 4200,
    revealed: true,
  });
  return { mapId, sceneId, imageId };
}

/** The map the session has now, the way the master's "Mapa" tab sets it. */
export const putMapOnTable = setCurrentMapRPC;

/** The scene open to the whole table. */
export const putSceneOnTable = openSceneRPC;

/** The image shown to the whole table (`''` takes it off). */
export async function showImageRPC(master: Page, campaignId: string, imageId: string): Promise<void> {
  const res = await callRPC(master, 'meurpg.play.v1.PlayService/SetShownImage', { campaignId, imageId });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** A d20 rolled for the whole table, by the master. */
export async function rollForTableRPC(master: Page, campaignId: string): Promise<void> {
  const res = await callRPC(master, 'meurpg.play.v1.PlayService/RollFreeDice', { campaignId, sides: 20, count: 1, idempotencyKey: crypto.randomUUID() });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** The session bar's "Tela da mesa" button (from 1024 px; under it the "Sessão" menu keeps the line). */
export const tvButton = (page: Page): Locator => page.getByTestId('tv-button');

/** Opens the master's panel from the session bar (its button from 1024 px, the "Sessão" menu under it) and returns it. */
export async function openTvPanel(page: Page): Promise<Locator> {
  if ((page.viewportSize()?.width ?? Number.POSITIVE_INFINITY) < 1024) {
    await openSessionMenu(page);
    await page.getByTestId('tv-menu-item').click();
  } else {
    await tvButton(page).click();
  }
  const panel = page.getByRole('dialog', { name: 'Tela da mesa' });
  await expect(panel).toBeVisible();
  return panel;
}

/** Closes the panel with Escape, the way a keyboard does. */
export async function closeTvPanel(page: Page): Promise<void> {
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog', { name: 'Tela da mesa' })).toHaveCount(0);
}

/** Chooses a row of the panel (`tv-mirror`, `tv-pin-map`...) the way a person does, by its label: the radio under it is visually hidden. */
export async function chooseRow(panel: Locator, testId: string): Promise<void> {
  await panel.getByTestId(testId).locator('label').click();
  await expect(panel.getByTestId(testId).getByRole('radio')).toBeChecked();
}

/** "Abrir a Tela da mesa" of the panel: the link opens the TV in a new tab of the master's own browser, returned once it is on its route. */
export async function openTvTab(context: BrowserContext, panel: Locator, campaignId: string): Promise<Page> {
  const opened = context.waitForEvent('page');
  await panel.locator('.js-open').click();
  const tv = await opened;
  await expect(tv).toHaveURL(new RegExp(`${tvRoute(campaignId)}$`));
  return tv;
}

/** The title of the TV tab's header: the h1 names the item on the stage (the idle screen names the session). */
export const tvTitle = (tv: Page): Locator => tv.getByRole('heading', { level: 1 });

/** What a page asked of the server, in order. */
export interface Traffic {
  /** Every `/meurpg.` procedure path asked, such as `/meurpg.play.v1.TableScreenService/GetTableScreen`. */
  procedures: () => string[];
  /** The images the page asked for, as path and query. */
  images: () => string[];
}

/** Starts recording every request `page` makes; call it before the first `goto`. */
export function recordTraffic(page: Page): Traffic {
  const procedures: string[] = [];
  const images: string[] = [];
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (url.pathname.startsWith('/meurpg.')) {
      procedures.push(url.pathname);
    }
    if (request.resourceType() === 'image') {
      images.push(url.pathname + url.search);
    }
  });
  return { procedures: () => [...procedures], images: () => [...images] };
}

/** Everything the page keeps in `localStorage` and `sessionStorage`, as one comparable value. */
export function storageOf(page: Page): Promise<string> {
  return page.evaluate(() => {
    const dump = (store: Storage): [string, string | null][] =>
      Object.keys(store)
        .sort()
        .map((key) => [key, store.getItem(key)]);
    return JSON.stringify({ local: dump(localStorage), session: dump(sessionStorage) });
  });
}
