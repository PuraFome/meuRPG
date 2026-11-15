import { expect, type Locator } from '@playwright/test';
import { test } from './diagnostics';

import { combatRPC, passTurnsToAndWait } from './combat-support';
import { openCombatPanel } from './combat-layout';
import { endOpenSessionRPC, openSessionPage } from './live-session-support';
import { pickRadio } from './move-support';
import { beginPowerCombatRPC, powerAttackTable } from './power-attack-support';
import { newSignedInContext, openActionTab, openCombatLog, pensantus, viaTargetList } from './support';
import { secondPlayer } from './theatre-support';

// #586, MR-025: the power attack of a feat ("Golpe Pesado": 5 off the attack, 10 on the damage if it hits) from the roll to the combat log.
// Toren ticks the trade before the roll and reads the attack and the damage as they were and as they become. A hit shows the +10 as an
// automatic part of the damage and the roll counts it; the log says the numbers to Toren and to the master and only that a trade was made to
// another player. A miss never brings the +10 in. The feat is put on Toren's sheet through the API; the specs of the editor are elsewhere.

test.describe.configure({ timeout: 300_000 });

test(
  'o jogador usa o Golpe Pesado: acerta e o +10 entra no dano, erra e ele não entra, e o registro mostra os números só a quem pode',
  { tag: ['@MR-025', '@RN-10'] },
  async ({ browser }) => {
    const master = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: 1280, height: 1000 } });
    const player = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: 1280, height: 1000 } });
    const m = await master.newPage();
    const p = await player.newPage();
    await Promise.all([m.goto('/'), p.goto('/')]);
    const table = await powerAttackTable(m, p, `Ataque poderoso ${Date.now()}`);
    const campaignId = table.campaignId;
    const other = await secondPlayer(browser, m, campaignId, pensantus, {}, { viewport: { width: 1280, height: 1000 } });
    try {
      const enc = await beginPowerCombatRPC(m, table);
      const toren = enc.combatants.find((c) => c.label === 'Toren')!.id;
      await openSessionPage(p, campaignId);

      // Opens the greatsword's sheet on Goblin 1 and ticks the trade.
      const openSheet = async () => {
        await openActionTab(p, 'Ataques');
        await p.getByRole('button', { name: /^Atacar com Espada grande/ }).click();
        const sheet = p.getByRole('dialog');
        await viaTargetList(p, sheet);
        await sheet.locator('label', { hasText: 'Goblin 1' }).click();
        const check = sheet.getByRole('checkbox', { name: /Usar Golpe Pesado/ });
        await expect(check).not.toBeChecked();
        await pickRadio(sheet, 'Usar Golpe Pesado');
        await expect(check).toBeChecked();
        return sheet;
      };
      const typeD20 = async (sheet: Locator, face: string) => {
        await sheet.getByRole('button', { name: 'Digitar o resultado' }).click();
        await sheet.getByLabel(/Role 1d20/).fill(face);
        await sheet.getByRole('button', { name: `Confirmar ${face}` }).click();
      };

      // A miss with the trade on (a 1 on the d20): no +10 comes in, and the sheet says so. The goblin stays alive for the hit.
      let sheet = await openSheet();
      await typeD20(sheet, '1');
      await expect(sheet.getByText('sem dano, o +10 não entra')).toBeVisible();
      await expect(p.getByRole('region', { name: 'O que entra no dano' })).toHaveCount(0);
      await sheet.getByRole('button', { name: 'Fechar' }).last().click();
      const missLog = await openCombatLog(p);
      const miss = missLog.getByRole('listitem').filter({ hasText: 'Espada grande: errou' });
      await expect(miss).toContainText('com ataque poderoso (−5 no ataque, +10 no dano)');
      await expect(miss).not.toContainText('de dano');
      await openSessionPage(other.page, campaignId);
      const missOthers = await openCombatLog(other.page);
      const missLine = missOthers.getByRole('listitem').filter({ hasText: 'Espada grande: errou' });
      await expect(missLine).toContainText('usou um ataque poderoso');
      await expect(missLine).not.toContainText(/−5|\+\s?10/);

      // The next turn, a hit with the trade on: the box shows the attack and the damage as they were and as they become.
      await combatRPC(m, 'EndTurn', { campaignId, encounterId: enc.id, expectedCombatantId: toren, discardPendingDamage: true });
      await passTurnsToAndWait(m, campaignId, 'Toren', p);
      sheet = await openSheet();
      const box = sheet.locator('app-power-trade-offer');
      await expect(box).toContainText('−5 no ataque, +10 no dano se acertar');
      const attackRow = box.locator('.box__row', { hasText: 'Ataque' });
      const damageRow = box.locator('.box__row', { hasText: 'Dano' });
      await expect(attackRow.locator('s')).toHaveText('1d20 + 6');
      await expect(attackRow.locator('b')).toHaveText('1d20 + 1');
      await expect(damageRow.locator('s')).toHaveText('2d6 + 3');
      await expect(damageRow.locator('b')).toHaveText('2d6 + 13 cortante, se acertar');
      await typeD20(sheet, '18');
      await expect(sheet).toContainText('Acertou: role o dano.');
      await expect(sheet.getByText('Golpe Pesado: −5 no ataque')).toBeVisible();
      // The +10 is a part of the damage the player does not roll: the damage step lists it as automatic and the roll is the weapon's dice.
      const parts = p.getByRole('region', { name: 'O que entra no dano' });
      await expect(parts).toContainText('Golpe Pesado');
      await expect(parts).toContainText('automático');
      await expect(parts).toContainText('+10');
      const roll = p.getByRole('button', { name: 'Rolar 2d6 + 13 no app', exact: true });
      await expect(roll).toBeVisible();
      await roll.click();
      await expect(p.getByRole('button', { name: 'Voltar à sua vez' })).toBeFocused();
      await p.getByRole('button', { name: 'Voltar à sua vez' }).click();

      // The player's log (the owner) reads the numbers, with the damage rolled.
      const own = await openCombatLog(p);
      const hit = own.getByRole('listitem').filter({ hasText: 'Espada grande: acertou' });
      await expect(hit).toContainText('com ataque poderoso (−5 no ataque, +10 no dano)');
      await expect(hit).toContainText('de dano');
      // The master reads the same.
      await openSessionPage(m, campaignId);
      await openCombatPanel(m, 'Registro');
      const seen = m.getByRole('log', { name: 'Registro do combate' }).getByRole('listitem').filter({ hasText: 'Espada grande: acertou' });
      await expect(seen).toContainText('com ataque poderoso (−5 no ataque, +10 no dano)');
      // Another player reads only that a trade was made: no name of the feat, no number of the trade.
      await openSessionPage(other.page, campaignId);
      const others = await openCombatLog(other.page);
      const line = others.getByRole('listitem').filter({ hasText: 'Espada grande: acertou' });
      await expect(line).toContainText('usou um ataque poderoso');
      await expect(line).not.toContainText('Golpe Pesado');
      await expect(line).not.toContainText(/−5|\+\s?10/);
    } finally {
      await endOpenSessionRPC(m, campaignId);
      await other.close();
      await master.close();
      await player.close();
    }
  },
);
