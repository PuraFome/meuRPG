import { type Locator, type Page } from '@playwright/test';

// The master's running combat is three columns from 1280 px (the order, the map, the rail "Turno · Conversa") and the tabs
// "Turno · Ordem · Mapa" on a phone. The panels that were tabs under the map are rows of "Mais do combate" under the content of "Turno"
// (on a phone, in the "Mais" fold at the end of "Turno"); each row is a fold that opens in place. These helpers take the master where a
// spec's control is, on whichever screen it runs.

/** A row of "Mais do combate" ("Registro" and "Efeitos" always; the others only when they apply). */
export type CombatPanel = 'Registro' | 'Efeitos' | 'Névoa e luz' | 'Armadilhas' | 'Tesouro' | 'Quebra-cabeças';

/** Opens a row of "Mais do combate" in place (nothing if it is open); on a phone it opens "Turno" and its "Mais" fold first. */
export async function openCombatPanel(page: Page, name: CombatPanel): Promise<void> {
  const row = page.locator('app-combat-view .more').getByRole('button', { name: new RegExp(`^${name}`) });
  // The phone keeps the rows inside the "Mais" fold at the end of "Turno" ("Mais 5 painéis").
  const fold = page.getByRole('button', { name: /^Mais \d+ painéis/ });
  await row.or(fold).first().waitFor();
  if (!(await row.isVisible())) {
    const turn = page.getByRole('tab', { name: /^Turno/ });
    if (await turn.isVisible()) {
      await turn.click();
    }
    if ((await fold.getAttribute('aria-expanded')) === 'false') {
      await fold.click();
    }
    await row.waitFor();
  }
  if ((await row.getAttribute('aria-expanded')) === 'false') {
    await row.click();
  }
}

/** The master's combat log, with its row open. */
export async function masterLog(page: Page): Promise<Locator> {
  await openCombatPanel(page, 'Registro');
  return page.getByRole('log', { name: 'Registro do combate' });
}

/** Opens the row of a combatant in the master's order, in place: its "Dano/Cura", "Condições" and "Mais" are in it. On a phone the order is its own tab. */
export async function openOrderRow(page: Page, label: string): Promise<void> {
  const orderTab = page.getByRole('tab', { name: /^Ordem/ });
  if ((await orderTab.count()) > 0 && (await orderTab.isVisible())) {
    await orderTab.click();
  }
  const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const head = page.locator('app-order-list').getByRole('button', { name: new RegExp(`^${escaped}(,|$)`) });
  if ((await head.getAttribute('aria-expanded')) === 'false') {
    await head.click();
  }
}

/** Back to "Turno" on a phone (where the order is its own tab, and an opened row leaves "Turno" behind); on a computer it is the rail's tab. */
export async function openTurn(page: Page): Promise<void> {
  const turn = page.getByRole('tab', { name: /^Turno/ });
  if (await turn.isVisible()) {
    await turn.click();
  }
}

/** The map on a phone (where it is the tab "Mapa"); on a computer it is already in the middle column. */
export async function openMap(page: Page): Promise<void> {
  const map = page.getByRole('tab', { name: /^Mapa/ });
  if (await map.isVisible()) {
    await map.click();
  }
}
