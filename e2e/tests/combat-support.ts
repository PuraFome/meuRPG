import { expect, type Page } from '@playwright/test';

import { canvasPng, createMapRPC, placeTokenRPC, revealMapRPC, setCurrentMapRPC, tableForMaps, uploadImageRPC, type MapsTable } from './maps-support';
import { startSessionRPC } from './live-session-support';
import { callRPC, characterRpcBody, createCharacterRPC, pensantus } from './support';

// Setup for the combat specs (Etapa 6, MR-013), through the API: these tests
// prove the combat screens, not the campaign, character and map forms other
// specs already cover. Every test makes its own campaign.

export interface CombatTable extends MapsTable {
  sessionId: string;
  mapId: string;
  /** The Capitão Goblin (an enemy), the Goblin (a minion) and Velha Odra (story). */
  captainId: string;
  goblinId: string;
  odraId: string;
}

/**
 * A table ready for a combat: Pensantus (the player's, with a token), the
 * NPCs Capitão Goblin, Goblin and Velha Odra, an open session and a map as
 * its current map, with a 20-column grid when `grid` (the default). Master
 * and player pages must be signed in as each; neither navigates.
 */
export async function tableForCombat(masterPage: Page, playerPage: Page, name: string, grid = true, attacks = false): Promise<CombatTable> {
  const base = await tableForMaps(masterPage, playerPage, name, true, attacks ? pensantusAttacks : {});
  // With `attacks` the Capitão carries a scimitar and wears chain mail, so the master has something to roll.
  const captainBody = characterRpcBody('ENEMY', { ...pensantus, name: 'Capitão Goblin' }) as { sheet: { full: object } };
  if (attacks) {
    captainBody.sheet.full = { ...captainBody.sheet.full, weaponKeys: ['equipment:scimitar'], armorKey: 'equipment:chain-mail' };
  }
  const captain = await createCharacterRPC(masterPage, base.campaignId, captainBody);
  expect(captain.ok()).toBeTruthy();
  const odra = await createCharacterRPC(masterPage, base.campaignId, {
    kind: 'CHARACTER_KIND_STORY',
    name: 'Velha Odra',
    sheet: { basic: { hitPointsMax: 9, armorClass: 10, speedFt: 25, attackBonus: 0, damage: '1d4', description: '' } },
  });
  expect(odra.ok()).toBeTruthy();

  const sessionId = await startSessionRPC(masterPage, base.campaignId);
  await masterPage.goto('/');
  const image = await uploadImageRPC(masterPage, base.campaignId, 'Emboscada na estrada', await canvasPng(masterPage, 2000, 1400, 'Emboscada na estrada'));
  const mapId = await createMapRPC(masterPage, base.campaignId, 'Emboscada na estrada', image);
  await revealMapRPC(masterPage, base.campaignId, mapId);
  await setCurrentMapRPC(masterPage, base.campaignId, mapId);
  if (grid) {
    await setGridRPC(masterPage, base.campaignId, mapId, 20);
  }
  await placeTokenRPC(masterPage, base.campaignId, mapId, base.characterId, 2500, 5400);
  const captainId = (await captain.json()).character.id as string;
  await placeTokenRPC(masterPage, base.campaignId, mapId, captainId, 5750, 3900);
  return { ...base, sessionId, mapId, captainId, goblinId: base.npcId!, odraId: (await odra.json()).character.id as string };
}

/** What Pensantus carries in the attack specs: a dagger, Raio de Fogo and some
 * spells (the "Sua vez" groups need something to list). */
export const pensantusAttacks = {
  weaponKeys: ['equipment:dagger'],
  cantripKeys: ['spell:fire-bolt'],
  knownSpellKeys: ['spell:magic-missile', 'spell:sleep', 'spell:shield', 'spell:web', 'spell:misty-step'],
  preparedSpellKeys: ['spell:magic-missile', 'spell:sleep', 'spell:shield', 'spell:web', 'spell:misty-step'],
};

/**
 * Starts a combat with the Capitão and two Goblins in plain sight, rolls
 * everyone's initiative with the given d20 faces (by label; the master may
 * type them) and places them, then begins. Returns the combat on turn 1.
 */
export async function beginAttackCombatRPC(
  master: Page,
  table: CombatTable,
  faces: Record<string, number>,
  at: Record<string, [number, number]> = { 'Capitão Goblin': [11, 5], 'Goblin 1': [9, 9], 'Goblin 2': [14, 10] },
  hidden: string[] = [],
): Promise<Encounter> {
  let enc = await startEncounterRPC(master, table, [
    { characterId: table.captainId, count: 1, hidden: false },
    { characterId: table.goblinId, count: 2, hidden: false },
  ]);
  for (const c of enc.combatants) {
    enc = await combatRPC(master, 'SubmitInitiative', { campaignId: table.campaignId, encounterId: enc.id, combatantId: c.id, d20Face: faces[c.label] ?? 1 });
  }
  for (const [label, [col, row]] of Object.entries(at)) {
    const id = enc.combatants.find((c) => c.label === label)!.id;
    enc = await combatRPC(master, 'MoveCombatant', { campaignId: table.campaignId, encounterId: enc.id, combatantId: id, col, row });
  }
  for (const label of hidden) {
    const id = enc.combatants.find((c) => c.label === label)!.id;
    enc = await combatRPC(master, 'SetCombatantHidden', { campaignId: table.campaignId, encounterId: enc.id, combatantId: id, hidden: true });
  }
  return combatRPC(master, 'BeginCombat', { campaignId: table.campaignId, encounterId: enc.id });
}

export async function setGridRPC(page: Page, campaignId: string, mapId: string, columns: number): Promise<void> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/SetMapGrid', { campaignId, mapId, columns });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** `StartEncounter` for the whole party and the given NPC copies. */
export async function startEncounterRPC(
  page: Page,
  table: CombatTable,
  npcs: { characterId: string; count: number; hidden?: boolean }[],
  name = 'Emboscada na estrada',
): Promise<Encounter> {
  const res = await callRPC(page, 'meurpg.play.v1.CombatService/StartEncounter', {
    campaignId: table.campaignId,
    idempotencyKey: crypto.randomUUID(),
    name,
    participants: npcs,
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).encounter as Encounter;
}

export async function getEncounterRPC(page: Page, campaignId: string): Promise<Encounter> {
  const res = await callRPC(page, 'meurpg.play.v1.CombatService/GetEncounter', { campaignId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).encounter as Encounter;
}

/** Any write of `CombatService` that takes the encounter and an idempotency key. */
export async function combatRPC(page: Page, method: string, body: object): Promise<Encounter> {
  const res = await callRPC(page, `meurpg.play.v1.CombatService/${method}`, { idempotencyKey: crypto.randomUUID(), ...body });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).encounter as Encounter;
}

/** The JSON shape of an `Encounter` (the Connect JSON codec: camelCase, and
 * zero values left out) that the specs read. */
export interface Encounter {
  id: string;
  name: string;
  status: string;
  round?: number;
  currentCombatantId?: string;
  masterTurn?: boolean;
  gridColumns: number;
  gridRows: number;
  combatants: Combatant[];
  revision: number;
}

export interface Combatant {
  id: string;
  label: string;
  kind: string;
  mine?: boolean;
  hidden?: boolean;
  initiative?: number;
  placed?: boolean;
  col?: number;
  row?: number;
  movementLeftFt?: number;
  defeated?: boolean;
}
