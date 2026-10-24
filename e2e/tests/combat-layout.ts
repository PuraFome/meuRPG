import { type Page } from '@playwright/test';

// The master's running combat is three columns from 1280 px (the order, the map with its tabs, the rail "Turno e conversa") and the
// tabs "Turno · Ordem · Mapa" on a phone, with the panels of the map's tab set behind "Mais" at the end of "Turno". These helpers
// take the master where a spec's control is, on whichever screen it runs.

/** A tab of the set under the map ("Registro" is open by default; the others are tabs that show only when they apply). */
export type CombatTab = 'Registro' | 'Efeitos' | 'Névoa e luz' | 'Armadilhas' | 'Tesouro' | 'Quebra-cabeças';

/** Opens a tab under the map; on a phone it opens "Turno" and its "Mais" fold first. */
export async function openCombatTab(page: Page, name: CombatTab): Promise<void> {
  const tab = page.getByRole('tab', { name: new RegExp(`^${name}`) });
  if (!(await tab.isVisible())) {
    const turn = page.getByRole('tab', { name: /^Turno/ });
    if (await turn.isVisible()) {
      await turn.click();
    }
    const more = page.getByRole('button', { name: /^Mais/, expanded: false });
    if ((await more.count()) > 0) {
      await more.click();
    }
  }
  await tab.click();
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
