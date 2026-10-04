import { expect, type Page } from '@playwright/test';

import { combatRPC, getEncounterRPC, startEncounterRPC, tableForCombat, type CombatTable, type Encounter } from './combat-support';
import { createCharacterRPC } from './support';

// Setup for the joint-turn specs (MR-013, RN-19, RN-20), through the API. The
// suite has only two signed-in people, so the joint turn of the table is
// Pensantus (the player's) and Brisa, an NPC ally the master put on the same
// total: a group that holds a player's character. The goblins are an
// NPC-only group on another total.

export interface JointTable {
  table: CombatTable;
  brisaId: string;
}

/** A table with Brisa (a story NPC) next to Pensantus, a map and an open session. */
export async function jointTable(master: Page, player: Page, name: string): Promise<JointTable> {
  const table = await tableForCombat(master, player, name);
  const res = await createCharacterRPC(master, table.campaignId, {
    kind: 'CHARACTER_KIND_STORY',
    name: 'Brisa',
    sheet: { basic: { hitPointsMax: 20, armorClass: 14, speedFt: 30, attackBonus: 0, damage: '1d4', description: '' } },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return { table, brisaId: (await res.json()).character.id as string };
}

/**
 * Starts the combat with Brisa and two Goblins (all in plain sight), types
 * every initiative so the totals are the same on every run (physical dice:
 * a random roll could tie, and a tie is a joint turn) and begins:
 * Pensantus and Brisa share one total (11 or more), the goblins share 5.
 */
export async function beginJointCombat(master: Page, joint: JointTable): Promise<Encounter> {
  const { table, brisaId } = joint;
  let enc = await startEncounterRPC(master, table, [
    { characterId: brisaId, count: 1, hidden: false },
    { characterId: table.goblinId, count: 2, hidden: false },
  ]);
  const submit = async (label: string, d20Face: number) => {
    enc = await combatRPC(master, 'SubmitInitiative', {
      campaignId: table.campaignId, encounterId: enc.id, combatantId: enc.combatants.find((c) => c.label === label)!.id, d20Face,
    });
  };
  await submit('Pensantus', 12);
  const total = enc.combatants.find((c) => c.label === 'Pensantus')!.initiative!;
  const bonus = (label: string) => (enc.combatants.find((c) => c.label === label) as { initiativeBonus?: number }).initiativeBonus ?? 0;
  await submit('Brisa', total - bonus('Brisa'));
  await submit('Goblin 1', 5 - bonus('Goblin 1'));
  await submit('Goblin 2', 5 - bonus('Goblin 2'));
  for (const [label, col, row] of [['Brisa', 7, 3], ['Goblin 1', 9, 9], ['Goblin 2', 14, 10]] as const) {
    enc = await combatRPC(master, 'MoveCombatant', { campaignId: table.campaignId, encounterId: enc.id, combatantId: enc.combatants.find((c) => c.label === label)!.id, col, row });
  }
  return combatRPC(master, 'BeginCombat', { campaignId: table.campaignId, encounterId: enc.id });
}

/** The master ends one member's part, as the master's screen does. */
export async function endPartRPC(master: Page, campaignId: string, label: string): Promise<Encounter> {
  const enc = await getEncounterRPC(master, campaignId);
  return combatRPC(master, 'EndTurn', {
    campaignId, encounterId: enc.id, expectedCombatantId: enc.combatants.find((c) => c.label === label)!.id,
  });
}
