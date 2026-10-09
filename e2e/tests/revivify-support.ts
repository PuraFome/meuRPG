import { expect, type Page } from '@playwright/test';

import { beginAttackCombatRPC, brisa, brisaSheet, combatRPC, tableForCombat, toren, torenSheet, type CombatTable } from './combat-support';
import { startSessionRPC } from './live-session-support';
import { tableForMaps } from './maps-support';
import { callRPC, characterRpcBody, createCharacterRPC, type CharacterBuild } from './support';

// Setup for the Revivificar specs (RN-03, RN-10; PM-08d), through the API: these tests prove the sheet and the notices, not the
// campaign, character and combat setup other specs already cover. Every test makes its own campaign.

/** Ilaria, a human Cleric 5 of the Life domain: Revivificar is always prepared at that level. */
export const ilaria: CharacterBuild = { ...brisa, name: 'Ilaria', level: 5 };
export const ilariaSheet = brisaSheet;

export interface RevivifyCombat {
  table: CombatTable;
  encounterId: string;
}

/**
 * A combat on Ilaria's turn where two enemies are down: Goblin 1 beside her and Goblin 2 far away (the list must hold only the
 * first). Master and player pages must be signed in as each; neither navigates.
 */
export async function combatWithTwoDown(master: Page, player: Page, name: string): Promise<RevivifyCombat> {
  const table = await tableForCombat(master, player, `${name} ${Date.now()}`, true, true, { build: ilaria, sheet: ilariaSheet });
  let enc = await beginAttackCombatRPC(master, table, { Ilaria: 20, 'Capitão Goblin': 15, 'Goblin 1': 5, 'Goblin 2': 4 });
  const me = enc.combatants.find((c) => c.label === 'Ilaria')!;
  const near = enc.combatants.find((c) => c.label === 'Goblin 1')!;
  const far = enc.combatants.find((c) => c.label === 'Goblin 2')!;
  enc = await combatRPC(master, 'MoveCombatant', {
    campaignId: table.campaignId,
    encounterId: enc.id,
    combatantId: near.id,
    col: (me.col ?? 0) + 1,
    row: me.row ?? 0,
  });
  for (const down of [near, far]) {
    enc = await combatRPC(master, 'AdjustCombatantHitPoints', { campaignId: table.campaignId, encounterId: enc.id, combatantId: down.id, damage: 999 });
  }
  return { table, encounterId: enc.id };
}

export interface DeadTable {
  campaignId: string;
  /** Toren, the character that died. */
  deadId: string;
  /** Ilaria, the cleric; empty when the table has only the dead one. */
  casterId: string;
}

/**
 * A table whose player's Toren is dead, with an open session. With `withCaster` the player has made Ilaria too (a new living
 * character after the death), who has Revivificar; without it Toren is the player's only character. Master and player pages
 * must be signed in as each; neither navigates.
 */
export async function tableWithDead(master: Page, player: Page, name: string, withCaster: boolean): Promise<DeadTable> {
  const base = await tableForMaps(master, player, `${name} ${Date.now()}`, false, torenSheet, toren);
  const dead = await callRPC(master, 'meurpg.characters.v1.CharacterService/MarkCharacterDead', {
    campaignId: base.campaignId,
    characterId: base.characterId,
  });
  expect(dead.status()).toBe(200);
  let casterId = '';
  if (withCaster) {
    const body = characterRpcBody('PLAYER', ilaria) as { sheet: { full: object } };
    body.sheet.full = { ...body.sheet.full, ...ilariaSheet };
    const made = await createCharacterRPC(player, base.campaignId, body);
    expect(made.status()).toBe(200);
    casterId = (await made.json()).character.id as string;
  }
  await startSessionRPC(master, base.campaignId);
  return { campaignId: base.campaignId, deadId: base.characterId, casterId };
}

/** The master brings the dead character back ("Reviver"). */
export async function reviveRPC(master: Page, campaignId: string, characterId: string): Promise<void> {
  const res = await callRPC(master, 'meurpg.characters.v1.CharacterService/ReviveCharacter', {
    campaignId,
    characterId,
    idempotencyKey: crypto.randomUUID(),
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}
