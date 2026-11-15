import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { expect, type Locator, type Page } from '@playwright/test';

import { combatRPC, getEncounterRPC } from './combat-support';
import { rollsTable, vex, type RollsTable } from './combat-rolls-support';
import { openSessionPage } from './live-session-support';
import { callRPC, viaTargetList } from './support';

// The table of the take-out specs (#835): Vex, a rogue with a rapier and Sneak Attack, against a held goblin. The rapier and
// Sneak Attack are both piercing, so the player types ONE sum for the type: a typed group.

export const FACES = { Vex: 20, 'Goblin 1': 15, 'Goblin 1': 5, 'Goblin 2': 4 };

export interface TakeOutTable extends RollsTable {
  encounterId: string;
  /** Combatant ids by label. */
  id: (label: string) => string;
}

export async function takeOutTable(
  browser: Parameters<typeof rollsTable>[0],
  look: Parameters<typeof rollsTable>[5] = {},
): Promise<TakeOutTable> {
  const t = await rollsTable(browser, 'Tirar parte', vex, { weaponKeys: ['equipment:rapier'] }, FACES, look);
  const enc = await getEncounterRPC(t.m, t.campaignId);
  // The goblin is held: Vex attacks with advantage, which is what makes Sneak Attack hold.
  const goblin = enc.combatants.find((c) => c.label === 'Goblin 1')!;
  await combatRPC(t.m, 'SetCombatantConditions', { campaignId: t.campaignId, encounterId: enc.id, combatantId: goblin.id, conditions: { keys: ['condition:restrained'] } });
  return { ...t, encounterId: enc.id, id: (label) => enc.combatants.find((c) => c.label === label)!.id };
}

/** Vex hits Goblin 1 with advantage, marks Sneak Attack and types ONE sum (12) for the piercing; the damage is applied (15). */
export async function sneakHit(t: TakeOutTable, keepSheet = false): Promise<Locator> {
  const { p, campaignId } = t;
  await openSessionPage(p, campaignId);
  await expect(p.getByRole('heading', { name: 'Sua vez, Vex' })).toBeVisible();
  await p.getByRole('button', { name: /^Atacar com Rapieira/ }).click();
  const sheet = p.getByRole('dialog');
  await viaTargetList(p, sheet);
  await sheet.locator('label', { hasText: 'Goblin 1' }).click();
  await sheet.getByRole('button', { name: 'Digitar o resultado' }).click();
  await sheet.getByLabel(/d20 \(1º\)|Primeiro d20|1º d20/).first().fill('4');
  await sheet.getByLabel(/d20 \(2º\)|Segundo d20|2º d20/).first().fill('17');
  await sheet.getByRole('button', { name: /Confirmar/ }).click();
  await sheet.getByRole('checkbox', { name: /Ataque Furtivo/ }).check();
  await sheet.getByRole('button', { name: 'Digitar o resultado' }).click();
  await sheet.getByLabel('Soma dos dados: 1d8 + 2d6', { exact: true }).fill('12');
  await sheet.getByRole('button', { name: 'Aplicar 15 de dano' }).click();
  await expect(sheet.getByText(/Ataque Furtivo/).first()).toBeVisible();
  if (!keepSheet) {
    await sheet.getByRole('button', { name: /Voltar à sua vez/ }).click();
  }
  return sheet;
}

/** The master takes Sneak Attack out of the damage by the API (the screens are shown by the other specs). */
export async function takeOutRPC(t: TakeOutTable, reason: string): Promise<void> {
  const pending = await pendingIdOf(t);
  const res = await callRPC(t.m, 'meurpg.play.v1.CombatService/RemoveDamagePart', {
    campaignId: t.campaignId,
    encounterId: t.encounterId,
    pendingDamageId: pending,
    partKey: 'sneak-attack',
    reason,
    idempotencyKey: crypto.randomUUID(),
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** The id of the damage the master's options list for Vex's hit. */
export async function pendingIdOf(t: TakeOutTable): Promise<string> {
  const res = await callRPC(t.m, 'meurpg.play.v1.CombatService/GetTurnOptions', { campaignId: t.campaignId, encounterId: t.encounterId, combatantId: t.id('Vex') });
  expect(res.ok(), await res.text()).toBeTruthy();
  const list = (await res.json()).pendingDamages as { id: string }[];
  expect(list.length).toBeGreaterThan(0);
  return list[0].id;
}

/** The master's card of Vex's turn: the damage with "Tirar". */
export const damageCard = (m: Page) => m.getByRole('region', { name: /^Dano de Vex$/ });

const shotsDir = process.env.E2E_SHOTS_DIR;

/** The device scale of the shots (2) when `E2E_SHOTS_DIR` is set. */
export const takeOutShotScale = shotsDir ? { deviceScaleFactor: 2 } : {};

/** Saves the viewport as `835-web-<screen>-<width>-<light|dark>.png` when `E2E_SHOTS_DIR` is set, scrolled to `at` first. */
export async function shootTakeOut(page: Page, screen: string, width: number, scheme: 'light' | 'dark', at?: Locator): Promise<void> {
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
  await page.screenshot({ path: join(shotsDir, `835-web-${screen}-${width}-${scheme}.png`) });
}

/** What the screen must hold at every width: no sideways scroll and every button of `scope` at least 44px tall and wide enough to tap. */
export async function expectFits(page: Page, scope: Locator, screen: string): Promise<void> {
  const wide = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(wide, `${screen}: the page scrolls sideways by ${wide}px`).toBeLessThanOrEqual(0);
  const small: string[] = [];
  for (const b of await scope.getByRole('button').all()) {
    const box = await b.boundingBox();
    const name = (await b.innerText()).replace(/\s+/g, ' ').trim();
    if (box && (box.height < 44 || box.width < 44)) {
      small.push(`${name} ${Math.round(box.width)}x${Math.round(box.height)}`);
    }
  }
  expect(small, `${screen}: touch targets under 44px`).toEqual([]);
}

/** The button is dashed (the app's disabled pattern), not greyed out. */
export async function expectDashed(button: Locator): Promise<void> {
  await expect(button).toHaveCSS('border-top-style', 'dashed');
}
