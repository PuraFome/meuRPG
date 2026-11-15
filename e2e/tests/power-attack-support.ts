import { type Page } from '@playwright/test';

import { adjustVitalsRPC, beginAttackCombatRPC, tableForCombat, toren, type CombatTable } from './combat-support';
import { createEntryRPC, giveFeatRPC, heavyBlowFeatBody, steadyShotFeatBody } from './content-support';

// Setup for the screens of the power attack (Golpe Pesado, #586), through the API: Toren, a fighter of level 5 with a greatsword
// and a longbow, has the two table feats of the content helpers. The specs prove the screens, not the table editor.

/** The pool "Disparo Firme" spends from (steadyShotFeatBody), three uses. */
const POOL = 'folego_do_atirador';

/** Toren's sheet: the greatsword (heavy, melee) takes "Golpe Pesado"; the longbow (ranged) takes "Disparo Firme". */
export const powerSheet = { weaponKeys: ['equipment:greatsword', 'equipment:longbow'], armorKey: 'equipment:chain-mail' };

/** A table ready for a combat (the grid, the Capitão and two Goblins) where Toren has both feats. Master and player are signed in. */
export async function powerAttackTable(master: Page, player: Page, name: string): Promise<CombatTable> {
  const table = await tableForCombat(master, player, name, true, true, { build: toren, sheet: powerSheet });
  for (const body of [heavyBlowFeatBody(), steadyShotFeatBody()]) {
    await giveFeatRPC(master, table.campaignId, table.characterId, await createEntryRPC(master, table.campaignId, 'tableFeat', body));
  }
  return table;
}

/** Begins the combat with Toren on turn: Goblin 1 stands at his side, the captain and Goblin 2 far. */
export function beginPowerCombatRPC(master: Page, table: CombatTable) {
  return beginAttackCombatRPC(
    master,
    table,
    { Toren: 20, 'Capitão Goblin': 15, 'Goblin 1': 5, 'Goblin 2': 4 },
    { Toren: [3, 7], 'Goblin 1': [4, 7], 'Capitão Goblin': [14, 4], 'Goblin 2': [15, 11] },
  );
}

/** The master's correction that leaves the pool of "Disparo Firme" without a use. */
export async function spendBreathRPC(master: Page, table: CombatTable): Promise<void> {
  await adjustVitalsRPC(master, table.campaignId, table.characterId, { resourcesUsed: [{ key: POOL, used: 3 }] });
}
