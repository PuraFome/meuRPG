import { expect, test, type Browser, type Page } from '@playwright/test';
import { mkdirSync } from 'node:fs';

import { beginAttackCombatRPC, pensantusCasting, tableForCombat, toren, torenSheet } from './combat-support';
import { endOpenSessionRPC, openSessionPage } from './live-session-support';
import { paintRPC, pickRadio } from './move-support';
import { authStatePath } from './support';

const OUT = '/Users/viniciusf/personal/rpg-computer-use/scratch-ms/ui-shots/out/move-web-r3';

async function run(browser: Browser, scheme: 'light' | 'dark', width: number, height: number): Promise<void> {
  mkdirSync(OUT, { recursive: true });
  const opts = { colorScheme: scheme, viewport: { width, height }, deviceScaleFactor: 2 } as const;
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), ...opts });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), ...opts });
  const m = await master.newPage();
  const p = await player.newPage();
  const tag = `${width}x${height}-${scheme}`;
  const shot = async (page: Page, name: string) => {
    await page.waitForTimeout(500);
    await page.screenshot({ path: `${OUT}/${name}-${tag}.png` });
  };
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForCombat(m, p, `Telas r3 ${Date.now()}`, true, true, { build: toren, sheet: { ...torenSheet, ...pensantusCasting, weaponKeys: ['equipment:longsword', 'equipment:dagger'] } });
    campaignId = table.campaignId;
    await paintRPC(m, table, 'MAP_LAYER_COVER', 1, [[7, 8]]);
    await beginAttackCombatRPC(m, table, { Toren: 20, 'Goblin 1': 15, 'Capitão Goblin': 10, 'Goblin 2': 4 }, { 'Capitão Goblin': [9, 9], 'Goblin 1': [6, 7], 'Goblin 2': [15, 11] });
    await openSessionPage(p, campaignId);
    await openSessionPage(m, campaignId);
    await expect(p.getByRole('heading', { name: 'Sua vez, Toren' })).toBeVisible();
    await p.getByRole('button', { name: 'Mover', exact: true }).click();
    await p.getByRole('button', { name: 'Um quadrado para a esquerda' }).click();
    await expect(p.getByText('Mover 1,5 m', { exact: true })).toBeVisible();
    await p.getByRole('button', { name: 'Mover para cá' }).click();
    const card = m.getByRole('group', { name: 'Ataque de oportunidade de Goblin 1' });
    await expect(card.getByRole('button', { name: 'Não atacar' })).toBeVisible();
    await card.scrollIntoViewIfNeeded();
    await shot(m, '08-mestre-pergunta');
    await card.getByRole('button', { name: 'Não atacar' }).click();
    const order = m.getByRole('region', { name: 'Ordem de iniciativa' });
    await order.getByRole('button', { name: 'Marcar cobertura de Capitão Goblin' }).click();
    await order.getByRole('radiogroup').scrollIntoViewIfNeeded();
    await shot(m, '10-mestre-marcar-cobertura');
    await pickRadio(order, 'Três quartos');
    await order.getByRole('button', { name: 'Fechar' }).click();
    await order.getByRole('button', { name: 'Mais ações para Goblin 2' }).click();
    await m.getByRole('menuitem', { name: 'Marcar como aliado' }).click();
    await order.scrollIntoViewIfNeeded();
    await shot(m, '11-mestre-ordem');
    await m.getByRole('region', { name: 'Mapa', exact: true }).locator('.legend').scrollIntoViewIfNeeded().catch(() => undefined);
    await shot(m, '11b-legenda');
  } finally {
    if (campaignId) await endOpenSessionRPC(m, campaignId);
    await master.close();
    await player.close();
  }
}

for (const [scheme, w, h] of [['dark', 390, 844], ['dark', 320, 568], ['light', 1280, 800]] as const) {
  test(`r3 ${w}x${h}`, async ({ browser }) => {
    test.setTimeout(300_000);
    await run(browser, scheme, w, h);
  });
}
