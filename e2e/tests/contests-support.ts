import { expect, type Browser, type Locator, type Page } from '@playwright/test';

import { combatRPC, startEncounterRPC, tableForCombat, type CombatTable, type Encounter } from './combat-support';
import type { RollsTable } from './combat-rolls-support';
import { endOpenSessionRPC } from './live-session-support';
import { callRPC, createCharacterRPC, newSignedInContext, type CharacterBuild } from './support';

// Helpers of the contest specs (W7-X): the grapple, shove, escape, Hide, Help, surprise and group-check
// screens. The table, the NPCs and the combat come through the API (see combat-rolls-support.ts); what is
// under test is what the player and the master read and type.

/** A `ContestService` call; the answer is the parsed JSON. */
export async function contestRPC(page: Page, method: string, body: object): Promise<Record<string, unknown>> {
  const res = await callRPC(page, `meurpg.play.v1.ContestService/${method}`, { idempotencyKey: crypto.randomUUID(), ...body });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()) as Record<string, unknown>;
}

/** The id of the combatant called `label`. */
export function idOf(enc: Encounter, label: string): string {
  return enc.combatants.find((c) => c.label === label)!.id;
}

/** Types the physical d20 into the open roll form of `scope` and confirms it. */
export async function typeD20(scope: Locator, face: number): Promise<void> {
  await scope.getByRole('button', { name: 'Digitar o resultado' }).click();
  await scope.getByLabel(/Role 1d20/).fill(String(face));
  await scope.getByRole('button', { name: /^Confirmar/ }).click();
}

/** The foot button "Fechar" of a result sheet (the frame's own "x" has the same name and comes first). */
export function closeSheet(sheet: Locator): Promise<void> {
  return sheet.getByRole('button', { name: 'Fechar' }).last().click();
}

/**
 * The table of the Hide and Help specs: the player's character beside Goblin 1, a story NPC ally (Tavo) on the party's
 * side and the combat begun with the typed initiatives in `faces`. The characters come through the API.
 */
export async function alliedTable(
  browser: Browser,
  name: string,
  who: CharacterBuild,
  sheet: Record<string, unknown>,
  faces: Record<string, number>,
  look: { colorScheme?: 'light' | 'dark'; masterWidth?: number; playerWidth?: number } = {},
): Promise<RollsTable & { encounterId: string }> {
  const master = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: look.masterWidth ?? 1280, height: 900 }, colorScheme: look.colorScheme });
  const player = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: look.playerWidth ?? 390, height: 844 }, colorScheme: look.colorScheme });
  const m = await master.newPage();
  const p = await player.newPage();
  await m.goto('/');
  await p.goto('/');
  const table: CombatTable = await tableForCombat(m, p, `${name} ${Date.now()}`, true, true, { build: who, sheet });
  const tavo = await createCharacterRPC(m, table.campaignId, {
    kind: 'CHARACTER_KIND_STORY',
    name: 'Tavo',
    sheet: { basic: { hitPointsMax: 20, armorClass: 14, speedFt: 30, attackBonus: 0, damage: '1d4', description: '' } },
  });
  expect(tavo.ok(), await tavo.text()).toBeTruthy();
  const tavoId = (await tavo.json()).character.id as string;
  let enc = await startEncounterRPC(m, table, [
    { characterId: tavoId, count: 1, hidden: false },
    { characterId: table.goblinId, count: 2, hidden: false },
  ]);
  for (const c of enc.combatants) {
    enc = await combatRPC(m, 'SubmitInitiative', { campaignId: table.campaignId, encounterId: enc.id, combatantId: c.id, d20Face: faces[c.label] ?? 1 });
  }
  enc = await combatRPC(m, 'SetCombatantSide', { campaignId: table.campaignId, encounterId: enc.id, combatantId: idOf(enc, 'Tavo'), side: 'COMBATANT_SIDE_PARTY' });
  for (const [label, [col, row]] of Object.entries({ Tavo: [7, 9], 'Goblin 1': [9, 9], 'Goblin 2': [14, 10], [who.name]: [8, 9] })) {
    enc = await combatRPC(m, 'MoveCombatant', { campaignId: table.campaignId, encounterId: enc.id, combatantId: idOf(enc, label), col, row });
  }
  enc = await combatRPC(m, 'BeginCombat', { campaignId: table.campaignId, encounterId: enc.id });
  return {
    m,
    p,
    table,
    campaignId: table.campaignId,
    encounterId: enc.id,
    done: async () => {
      await endOpenSessionRPC(m, table.campaignId);
      await master.close();
      await player.close();
    },
  };
}
