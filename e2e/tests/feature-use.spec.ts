import { expect } from '@playwright/test';
import { test } from './diagnostics';
import { createEntryRPC, giveFeatRPC } from './content-support';
import { attackCombatSquares, beginAttackCombatRPC, getEncounterRPC, pensantusCasting, tableForCombat } from './combat-support';
import { elementFeatBody, endSessionOf, markFeatBody } from './feature-use-support';
import { openSessionPage } from './live-session-support';
import { newSignedInContext, openActionTab, openCombatLog, openSpentGroup } from './support';

/**
 * "Usar característica" (#743) through the real stack: a feature whose action declares a use is taken with `FeatureService.UseFeature`.
 * The player picks the creature, the row shows what is left and the combat log says it; the master uses another one in the name of the
 * player's character after the confirmation, and the server's count of the turn stops a second use.
 */
test('um jogador usa uma característica com alvo e o mestre usa uma com opção por ele', { tag: ['@MR-014', '@RN-19'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  const master = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: 1280, height: 900 } });
  const player = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: 1280, height: 900 } });
  const m = await master.newPage();
  const p = await player.newPage();
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForCombat(m, p, `Usar característica ${Date.now()}`, true, true, { sheet: pensantusCasting });
    campaignId = table.campaignId;
    for (const body of [markFeatBody(), elementFeatBody()]) {
      await giveFeatRPC(m, campaignId, table.characterId, await createEntryRPC(m, campaignId, 'tableFeat', body));
    }
    await beginAttackCombatRPC(m, table, { Pensantus: 20, 'Capitão Goblin': 15, 'Goblin 1': 5, 'Goblin 2': 4 }, attackCombatSquares);

    await openSessionPage(p, campaignId);
    await expect(p.getByRole('heading', { name: 'Sua vez, Pensantus' })).toBeVisible();
    await openActionTab(p, 'Habilidades');
    const groups = p.getByRole('region', { name: 'O que você pode fazer' });
    await expect(groups.getByText(/1\spor\scombate\s·\srestam\s1\sde\s1/)).toBeVisible();

    // The player picks the goblin; the sheet asks nothing else and the button names who.
    await groups.getByRole('button', { name: 'Usar Marca de Teste' }).click();
    const mark = p.getByRole('dialog', { name: /Marca de Teste/ });
    await expect(mark.getByRole('button', { name: 'Usar', exact: true })).toBeDisabled();
    await mark.locator('label', { hasText: 'Goblin 1' }).click();
    await mark.getByRole('button', { name: 'Usar no Goblin 1' }).click();
    await expect(mark).toHaveCount(0);
    await expect(groups.getByText(/Uso de Marca de Teste: 2 PV temporários/)).toBeVisible();

    // The server's count: the bonus action is spent, so the row waits in the spent group, off, with the limit of the combat at 0.
    const spent = await openSpentGroup(p, 'Ação bônus');
    await expect(spent.getByText(/restam\s0\sde\s1/)).toBeVisible();
    await expect(spent.getByRole('button', { name: 'Usar Marca de Teste' })).toBeDisabled();
    const log = await openCombatLog(p);
    await expect(log.getByText(/Pensantus usou Marca de Teste no Goblin 1/)).toBeVisible();

    // The master acts for Pensantus: the sheet confirms who, and the choice goes with the call.
    await openSessionPage(m, campaignId);
    await m.getByRole('button', { name: 'Agir por Pensantus' }).click();
    await openActionTab(m, 'Habilidades');
    await m.getByRole('button', { name: 'Usar Elemento de Teste' }).click();
    const element = m.getByRole('dialog', { name: /Elemento de Teste/ });
    await expect(element.getByText('Em quem: Pensantus')).toBeVisible();
    await expect(element.getByRole('button', { name: 'Usar', exact: true })).toBeDisabled();
    await element.getByRole('button', { name: 'Gelo', exact: true }).click();
    await element.getByRole('button', { name: 'Usar: Gelo' }).click();
    await expect(element).toHaveCount(0);
    // The master's own log (the fold "Registro" of the rail) says it, with the numbers; the player's carries it too.
    await m.getByRole('heading', { name: 'Registro' }).getByRole('button').click();
    await expect(m.getByRole('log', { name: 'Registro do combate' }).getByText(/Pensantus usou Elemento de Teste: Gelo, 3\sPV\stemporários\s\(pelo mestre\)/)).toBeVisible();
    await expect(log.getByText(/Pensantus usou Elemento de Teste/)).toBeVisible();
    await expect(m.getByRole('button', { name: 'Usar Elemento de Teste' })).toBeDisabled();

    // The server holds both uses: the player's page shows the turn's limit spent too.
    await expect(p.getByRole('region', { name: 'O que você pode fazer' }).getByRole('button', { name: 'Usar Elemento de Teste' })).toBeDisabled();
    expect((await getEncounterRPC(m, campaignId)).currentCombatantId).toBeTruthy();
  } finally {
    await endSessionOf(m, campaignId);
    await master.close();
    await player.close();
  }
});
