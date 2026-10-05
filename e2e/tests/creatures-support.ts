import { expect, type Page } from '@playwright/test';

import { startSessionRPC } from './live-session-support';
import { tableForMaps, type MapsTable } from './maps-support';
import { callRPC, pensantus } from './support';

// Setup for the creatures specs (Etapa 9, MR-037), through the API: these tests prove the
// creature screens, not the campaign and character forms other specs cover. Every test
// makes its own campaign.

export interface CreaturesTable extends MapsTable {
  /** Set when a session was started. */
  sessionId: string;
}

/** Pensantus at level 4 with Encontrar Familiar in the spellbook and prepared. */
const wizardWithFamiliar = {
  cantripKeys: ['spell:fire-bolt'],
  knownSpellKeys: ['spell:find-familiar', 'spell:magic-missile', 'spell:shield', 'spell:sleep'],
  preparedSpellKeys: ['spell:find-familiar', 'spell:magic-missile', 'spell:shield', 'spell:sleep'],
};

/**
 * A table for the creatures: Pensantus (a level 4 wizard who has Encontrar Familiar), and,
 * when `session` is true, an open session (a creature is cast during one). Master and
 * player pages must be signed in as each; neither navigates.
 */
export async function tableForCreatures(master: Page, player: Page, name: string, session = true): Promise<CreaturesTable> {
  const base = await tableForMaps(master, player, name, false, wizardWithFamiliar, { ...pensantus, level: 4 });
  const sessionId = session ? await startSessionRPC(master, base.campaignId) : '';
  return { ...base, sessionId };
}

/** The character's creatures as the app reads them (the owner's player and the master). */
export async function listCreaturesRPC(page: Page, campaignId: string, characterId: string): Promise<{ id: string; name: string; hitPointsCurrent?: number; hitPointsMax?: number; monsterKey: string; source: string }[]> {
  const res = await callRPC(page, 'meurpg.characters.v1.CharacterService/ListCharacterCreatures', { campaignId, characterId });
  expect(res.ok()).toBeTruthy();
  return ((await res.json()).creatures ?? []) as never;
}

/** The master gives the character a creature by its SRD key. */
export async function giveCreatureRPC(master: Page, campaignId: string, characterId: string, monsterKey: string, name = ''): Promise<string> {
  const res = await callRPC(master, 'meurpg.characters.v1.CharacterService/GiveCreature', { campaignId, characterId, monsterKey, name });
  expect(res.ok()).toBeTruthy();
  return (await res.json()).creature.id as string;
}
