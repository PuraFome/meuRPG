import { expect, type Locator, type Page } from '@playwright/test';

import { startSessionRPC } from './live-session-support';
import { tableForMaps, type MapsTable } from './maps-support';
import { pensantus } from './support';

// Setup for the casting specs (MR-048), through the API: these tests prove the casting screens, not the
// campaign and character forms other specs cover. Every test makes its own campaign.

export interface CastingTable extends MapsTable {
  sessionId: string;
}

/**
 * Pensantus at level 5 with Armadura Arcana and Detectar Magia prepared and Alarme in the spellbook only (a ritual
 * the wizard may cast without preparing it), and an open session: the casts happen during one.
 */
export async function tableForCasting(master: Page, player: Page, name: string): Promise<CastingTable> {
  const base = await tableForMaps(
    master,
    player,
    name,
    false,
    {
      cantripKeys: ['spell:fire-bolt'],
      knownSpellKeys: ['spell:mage-armor', 'spell:alarm', 'spell:detect-magic', 'spell:magic-missile'],
      preparedSpellKeys: ['spell:mage-armor', 'spell:detect-magic', 'spell:magic-missile'],
    },
    { ...pensantus, level: 5 },
  );
  return { ...base, sessionId: await startSessionRPC(master, base.campaignId) };
}

/** The cast sheet, whichever container it is in (a dialog from a tablet up, a bottom sheet on a phone). */
export function castSheet(page: Page): Locator {
  return page.locator('app-cast-out-sheet');
}

/** Opens "Conjurar" on the session page, picks the spell and waits for its step. */
export async function openCastOf(page: Page, spell: string): Promise<Locator> {
  await page.getByRole('button', { name: 'Conjurar', exact: true }).click();
  const sheet = castSheet(page);
  await expect(sheet.getByText('Magia', { exact: true }).first()).toBeVisible();
  await sheet.locator('label.row', { hasText: spell }).click();
  await expect(sheet.locator('.frame__title')).toHaveText(spell);
  return sheet;
}
