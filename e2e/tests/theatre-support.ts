import { expect, type Browser, type BrowserContext, type Page } from '@playwright/test';

import { type CombatTable, combatRPC, getEncounterRPC, tableForCombat, toren, torenSheet, type Encounter } from './combat-support';
import { endOpenSessionRPC } from './live-session-support';
import { callRPC, characterRpcBody, createCharacterRPC, newSignedInContext, type CharacterBuild } from './support';

// Setup for the combat-without-a-map specs (Etapa 10, slice 10.13b, MR-025, RN-24, RN-25, RN-20, ADR-0017), through the API:
// a table with Toren (a human Fighter 5, 9,0 m of speed) as the player's character and the Capitão Goblin and the Goblin as
// NPCs. What is under test is the screens and what each audience reads.

export interface TheatreTable {
  m: Page;
  p: Page;
  table: CombatTable;
  campaignId: string;
  done: () => Promise<void>;
}

/** A table of Mestre Teste and Jogador Teste (Toren), at the 1280 px desktop and the 390 px phone. */
export async function theatreTable(browser: Browser, name: string, character: { build?: CharacterBuild; sheet?: Record<string, unknown> } = {}, grid = true): Promise<TheatreTable> {
  const master: BrowserContext = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: 1280, height: 1000 } });
  const player: BrowserContext = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: 390, height: 844 } });
  const m = await master.newPage();
  const p = await player.newPage();
  await m.goto('/');
  await p.goto('/');
  const table = await tableForCombat(m, p, `${name} ${Date.now()}`, grid, false, { build: character.build ?? toren, sheet: character.sheet ?? torenSheet });
  return {
    m,
    p,
    table,
    campaignId: table.campaignId,
    done: async () => {
      await endOpenSessionRPC(m, table.campaignId);
      await master.close();
      await player.close();
    },
  };
}

/** `StartEncounter` in the given mode, for the party and the NPC copies; `faces` are the typed d20s by label. Returns the combat in setup. */
export async function startTheatreRPC(
  page: Page,
  table: CombatTable,
  npcs: { characterId: string; count: number; hidden?: boolean }[],
  mode: 'ENCOUNTER_MODE_THEATRE' | 'ENCOUNTER_MODE_GRID' = 'ENCOUNTER_MODE_THEATRE',
): Promise<Encounter & { mode?: string }> {
  const res = await callRPC(page, 'meurpg.play.v1.CombatService/StartEncounter', {
    campaignId: table.campaignId,
    idempotencyKey: crypto.randomUUID(),
    name: 'Emboscada na estrada',
    participants: npcs,
    mode,
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).encounter;
}

/** Rolls everyone's initiative with the typed faces (by label) and begins the combat. */
export async function beginTheatreRPC(page: Page, table: CombatTable, enc: Encounter, faces: Record<string, number>): Promise<Encounter> {
  let e = enc;
  for (const c of e.combatants) {
    e = await combatRPC(page, 'SubmitInitiative', { campaignId: table.campaignId, encounterId: e.id, combatantId: c.id, d20Face: faces[c.label] ?? 1 });
  }
  return combatRPC(page, 'BeginCombat', { campaignId: table.campaignId, encounterId: e.id });
}

/** A second player (the third devidp account) with a character at the table. */
export async function secondPlayer(
  browser: Browser,
  master: Page,
  campaignId: string,
  build: CharacterBuild,
  sheet: Record<string, unknown> = {},
): Promise<{ page: Page; characterId: string; close: () => Promise<void> }> {
  const context = await newSignedInContext(browser, 'E-mail Não Verificado', { viewport: { width: 390, height: 844 } });
  const page = await context.newPage();
  await page.goto('/');
  const invite = await callRPC(master, 'meurpg.campaigns.v1.CampaignService/CreateInvite', { campaignId, maxUses: 1, expiresIn: '3600s' });
  expect(invite.ok(), await invite.text()).toBeTruthy();
  const joined = await callRPC(page, 'meurpg.campaigns.v1.CampaignService/AcceptInvite', { token: (await invite.json()).token });
  expect(joined.ok(), await joined.text()).toBeTruthy();
  const body = characterRpcBody('PLAYER', build) as { sheet: { full: object } };
  body.sheet.full = { ...body.sheet.full, ...sheet };
  const made = await createCharacterRPC(page, campaignId, body);
  expect(made.ok(), await made.text()).toBeTruthy();
  return { page, characterId: (await made.json()).character.id as string, close: () => context.close() };
}

export { getEncounterRPC };
