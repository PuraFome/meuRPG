import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { expect, type Browser, type BrowserContext, type Locator, type Page } from '@playwright/test';

import { beginAttackCombatRPC, tableForCombat, type CombatTable } from './combat-support';
import { grog } from './combat-rolls-support';
import { endOpenSessionRPC, openSessionPage } from './live-session-support';
import { createPointRPC } from './maps-support';
import { openSceneRPC } from './scene-support';
import { setTableRulesRPC } from './table-rules-support';
import { thirdPlayer, type ThirdPlayer } from './trap-support';
import { callRPC, characterRpcBody, createCharacterRPC, newSignedInContext, pensantus, type CharacterBuild } from './support';

// The table of the NPC check outside a combat (#591): a master, Brisa (a College of Lore bard of level 5, a d8 of Bardic Inspiration,
// three uses, asked on every check) and, as the spec asks, bilbo (a Lore bard of level 3, a d6) or Toren (a fighter, who is never
// asked). The stage has Bartolo (an NPC typed by hand: no stat block), the Capitão bandido and the Espião, whom the master hid.
// Everything is made through the API: what is under test is what the master reads and the bards are asked.

export const brisaBard: CharacterBuild = {
  name: 'Brisa',
  raceKey: 'race:human',
  race: 'Humano',
  classKey: 'class:bard',
  class: 'Bardo',
  subclassKey: 'subclass:lore',
  subclass: 'Colégio do Conhecimento',
  level: 5,
  background: 'Artista',
  backgroundSkillKeys: ['skill:acrobatics', 'skill:performance'],
  backgroundSkills: ['Acrobacia', 'Atuação'],
  extraSkillKeys: ['skill:persuasion', 'skill:insight', 'skill:deception'],
  extraSkills: ['Persuasão', 'Intuição', 'Enganação'],
  scores: { for: 10, des: 14, con: 12, int: 12, sab: 10, car: 16 },
};

export const bilboBard: CharacterBuild = { ...brisaBard, name: 'bilbo', level: 3 };

/** The sheet's setting "Perguntar": on every check of an enemy the bard sees. */
export const askAll = { cuttingWordsAsk: 'CUTTING_WORDS_ASK_ALL' };

export interface NpcTable {
  m: Page;
  /** Brisa's page. */
  p: Page;
  table: CombatTable;
  campaignId: string;
  sceneId: string;
  /** The second player, when the table was asked for one: bilbo (a bard, asked after Brisa) or Toren (a fighter, never asked). */
  other?: ThirdPlayer;
  npc: { bartolo: string; capitao: string; espiao: string };
  done: () => Promise<void>;
}

export interface NpcLook {
  colorScheme?: 'light' | 'dark';
  masterWidth?: number;
  playerWidth?: number;
  deviceScaleFactor?: number;
}

export interface NpcOptions {
  /** The second player: another bard (bilbo) or a fighter (Toren). */
  other?: 'bard' | 'fighter';
  /** Brisa rolls her die by hand (the table's dice mode is the real dice). */
  physical?: boolean;
  /** No scene is open: the master's entry is the one beside "Pedir teste em grupo". */
  noScene?: boolean;
}

const shotsDir = process.env.E2E_SHOTS_DIR;

/** The device scale of the shots (2) when `E2E_SHOTS_DIR` is set. */
export const npcShotScale = shotsDir ? { deviceScaleFactor: 2 } : {};

export async function npcTable(browser: Browser, look: NpcLook = {}, options: NpcOptions = {}): Promise<NpcTable> {
  const master: BrowserContext = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: look.masterWidth ?? 1280, height: 900 }, colorScheme: look.colorScheme, deviceScaleFactor: look.deviceScaleFactor });
  const player: BrowserContext = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: look.playerWidth ?? 390, height: 844 }, colorScheme: look.colorScheme, deviceScaleFactor: look.deviceScaleFactor });
  const m = await master.newPage();
  const p = await player.newPage();
  await m.goto('/');
  await p.goto('/');
  const table = await tableForCombat(m, p, `Palavras de Interrupção ${Date.now()}`, true, false, { build: brisaBard, sheet: askAll });
  const { campaignId } = table;
  if (options.physical) {
    await setTableRulesRPC(m, campaignId, { diceMode: 'DICE_MODE_PHYSICAL' });
  }
  // The tokens are born hidden: the Capitão Goblin's is shown, so the map can measure it against Brisa's.
  const shown = await callRPC(m, 'meurpg.maps.v1.MapService/SetMapTokenHidden', { campaignId, mapId: table.mapId, characterId: table.captainId, hidden: false });
  expect(shown.ok(), await shown.text()).toBeTruthy();
  const sceneId = await createPointRPC(m, campaignId, table.mapId, { kind: 'SCENE', name: 'A taverna do Javali Cego', description: 'Uma taverna cheia.', xBp: 3000, yBp: 3000, revealed: true });
  if (!options.noScene) {
    await openSceneRPC(m, campaignId, sceneId);
  }
  const npc = {
    bartolo: await npcRPC(m, campaignId, 'Bartolo', 'STORY'),
    capitao: await npcRPC(m, campaignId, 'Capitão bandido', 'ENEMY'),
    espiao: await npcRPC(m, campaignId, 'Espião', 'ENEMY'),
  };
  if (!options.noScene) {
    await onStage(m, campaignId, npc.bartolo);
    await onStage(m, campaignId, npc.capitao);
    await onStage(m, campaignId, npc.espiao, true);
  }
  const base = { m, p, table, campaignId, done: async () => undefined };
  const other = options.other
    ? await thirdPlayer(browser, base, options.other === 'bard' ? bilboBard : grog, options.other === 'bard' ? askAll : {}, { colorScheme: look.colorScheme, width: look.playerWidth, deviceScaleFactor: look.deviceScaleFactor })
    : undefined;
  return {
    ...base,
    sceneId,
    other,
    npc,
    done: async () => {
      await endOpenSessionRPC(m, campaignId);
      await other?.close();
      await master.close();
      await player.close();
    },
  };
}

/** An NPC of the table: "Bartolo" is typed by hand (no stat block, so its checks roll +0), the others have a full sheet. */
async function npcRPC(page: Page, campaignId: string, name: string, kind: 'STORY' | 'ENEMY'): Promise<string> {
  const body =
    kind === 'STORY'
      ? { kind: 'CHARACTER_KIND_STORY', name, sheet: { basic: { hitPointsMax: 9, armorClass: 11, speedFt: 30, attackBonus: 0, damage: '', description: '' } } }
      : characterRpcBody('ENEMY', { ...pensantus, name });
  const res = await createCharacterRPC(page, campaignId, body);
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).character.id as string;
}

async function onStage(page: Page, campaignId: string, characterId: string, hidden = false): Promise<void> {
  const res = await callRPC(page, 'meurpg.play.v1.PlayService/PutOnStage', { campaignId, characterId, hidden });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** A combat begins by the API (what the master does in another tab): the next NPC check is refused. */
export async function beginCombatRPC(t: NpcTable): Promise<void> {
  await beginAttackCombatRPC(t.m, t.table, { 'Capitão Goblin': 12, 'Goblin 1': 9, 'Goblin 2': 8, Brisa: 15 }, { 'Capitão Goblin': [11, 5], 'Goblin 1': [9, 9], 'Goblin 2': [14, 10], Brisa: [8, 9] });
}

// What the master reads, by the words on the screen.

export const npcSheet = (m: Page): Locator => m.getByRole('dialog', { name: 'Teste de um NPC' });
export const npcCard = (m: Page): Locator => m.getByRole('region', { name: /^Teste de um NPC: / });
/** The bard's question: an alert dialog that must be answered. */
export const cutSheet = (p: Page): Locator => p.getByRole('alertdialog');

/** The master opens the sheet from the head of "Ações e rolagens" (or the page's own button when no scene is open). */
export async function openNpcSheet(m: Page, campaignId: string): Promise<Locator> {
  await openSessionPage(m, campaignId);
  await m.getByRole('button', { name: 'Teste de um NPC' }).first().click();
  const sheet = npcSheet(m);
  await expect(sheet.getByRole('heading', { name: 'Teste de um NPC' })).toBeVisible();
  await expect(sheet.getByText('Lendo quem está em cena e no mapa…')).toHaveCount(0);
  return sheet;
}

/** Chooses the creature in the sheet's list. */
export async function pickNpc(sheet: Locator, name: string): Promise<void> {
  await sheet.locator('label.row', { hasText: name }).first().click();
  await expect(sheet.getByRole('button', { name: new RegExp(`^Rolar .* d[oa] ${name}`) })).toBeVisible();
}

/** The sheet's d20: the app's, or the face of a real die typed in the field. */
export async function typeD20(sheet: Locator, face: string): Promise<void> {
  await sheet.locator('label', { hasText: 'Digitar o d20' }).click();
  await sheet.getByLabel('Número que saiu no d20 (1 a 20)').fill(face);
}

/** Rolls the chosen creature's check (the button names it) and waits for the sheet to go. */
export async function rollChosen(m: Page, sheet: Locator, name = /^Rolar .* d[oa] /): Promise<void> {
  await sheet.getByRole('button', { name }).click();
  await expect(sheet).toHaveCount(0);
}

/** The master's whole path to one check: the creature, how its d20 comes ("app", or the face typed) and whether the box stays ticked. */
export async function masterRolls(t: NpcTable, name: string, d20: 'app' | string, box?: boolean): Promise<void> {
  const sheet = await openNpcSheet(t.m, t.campaignId);
  await pickNpc(sheet, name);
  if (d20 !== 'app') {
    await typeD20(sheet, d20);
  }
  if (box !== undefined) {
    const check = sheet.getByRole('checkbox', { name: /Os bardos veem a criatura/ });
    if ((await check.isChecked()) !== box) {
      await sheet.locator('app-check-row label').click();
    }
  }
  await rollChosen(t.m, sheet);
}

/** The bard's page, on the session, ready to be asked. */
export async function bardReady(p: Page, campaignId: string): Promise<void> {
  await openSessionPage(p, campaignId);
}

/** Saves the viewport as `591-web-<screen>-<width>-<light|dark>.png` when `E2E_SHOTS_DIR` is set, scrolled to `at` first. */
export async function shootNpc(page: Page, screen: string, width: number, scheme: 'light' | 'dark', at?: Locator): Promise<void> {
  if (!shotsDir) {
    return;
  }
  mkdirSync(shotsDir, { recursive: true });
  if (at) {
    await at.evaluate((el) => {
      el.style.scrollMarginTop = '140px';
      el.scrollIntoView({ block: el.getBoundingClientRect().height < innerHeight * 0.8 ? 'center' : 'start' });
    });
  }
  await page.screenshot({ path: join(shotsDir, `591-web-${screen}-${width}-${scheme}.png`) });
}

/** What a flow may do at each screen: save a shot and scan it. Both are optional: the flow spec only reads. */
export interface NpcView {
  shoot?: (page: Page, screen: string, at?: Locator) => Promise<void>;
  scan?: (page: Page, screen: string) => Promise<void>;
}

/** Scans and shoots one screen. */
export async function see(v: NpcView, page: Page, screen: string, at?: Locator): Promise<void> {
  await v.scan?.(page, screen);
  await v.shoot?.(page, screen, at);
}
