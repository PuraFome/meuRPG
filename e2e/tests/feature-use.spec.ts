import { expect } from '@playwright/test';
import { test } from './diagnostics';
import { createEntryRPC, giveFeatRPC } from './content-support';
import { attackCombatSquares, beginAttackCombatRPC, combatRPC, getEncounterRPC, pensantusCasting, tableForCombat } from './combat-support';
import { callFeatBody, chamadosFeatBody, elementFeatBody, endSessionOf, haloFeatBody, haloSquares, markFeatBody } from './feature-use-support';
import { openSessionPage } from './live-session-support';
import { waitForEncounterRevision } from './revision-support';
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
    await expect(m.getByRole('log', { name: 'Registro do combate' }).getByText(/Pensantus usou Elemento de Teste: Gelo, 3\sPV\stemporários\s·\sgastou\s1\suso\s\(restam\s0\sde\s1\)\s\(pelo mestre\)/)).toBeVisible();
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

/**
 * The server's refusal of a target (FEATURE_TARGET): the chosen goblin leaves the reach before the call, the sheet reads the sentence that names it,
 * keeps the choice, and a second pick goes through with the same sheet. Nothing is spent by the refused call.
 */
test('o servidor recusa um alvo que saiu do alcance e a folha deixa escolher outro', { tag: ['@MR-014', '@RN-19'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  const master = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: 1280, height: 900 } });
  const player = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: 1280, height: 900 } });
  const m = await master.newPage();
  const p = await player.newPage();
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForCombat(m, p, `Usar característica recusa ${Date.now()}`, true, true, { sheet: pensantusCasting });
    campaignId = table.campaignId;
    await giveFeatRPC(m, campaignId, table.characterId, await createEntryRPC(m, campaignId, 'tableFeat', markFeatBody()));
    const begun = await beginAttackCombatRPC(m, table, { Pensantus: 20, 'Capitão Goblin': 15, 'Goblin 1': 5, 'Goblin 2': 4 }, attackCombatSquares);
    const captain = begun.combatants.find((c) => c.label === 'Capitão Goblin')!.id;

    await openSessionPage(p, campaignId);
    await expect(p.getByRole('heading', { name: 'Sua vez, Pensantus' })).toBeVisible();
    await openActionTab(p, 'Habilidades');
    const groups = p.getByRole('region', { name: 'O que você pode fazer' });
    await groups.getByRole('button', { name: 'Usar Marca de Teste' }).click();
    const mark = p.getByRole('dialog', { name: /Marca de Teste/ });
    await mark.locator('label', { hasText: 'Capitão Goblin' }).click();
    await expect(mark.getByRole('radio', { name: /Capitão Goblin/ })).toBeChecked();

    // The captain leaves the 9 m of the use after it was picked.
    await combatRPC(m, 'MoveCombatant', { campaignId, encounterId: begun.id, combatantId: captain, col: 19, row: 1 });
    await mark.getByRole('button', { name: 'Usar no Capitão Goblin' }).click();
    const band = mark.locator('.mr-notice--danger');
    await expect(band).toContainText('O Capitão Goblin saiu do alcance. Escolha outro alvo.');
    await expect(mark.getByRole('radio', { name: /Capitão Goblin/ })).toBeChecked();
    await expect(groups.getByText(/Uso de Marca de Teste/)).toHaveCount(0);

    // A second pick goes through, and the use is spent once.
    await mark.locator('label', { hasText: 'Goblin 1' }).click();
    await mark.getByRole('button', { name: 'Usar no Goblin 1' }).click();
    await expect(mark).toHaveCount(0);
    await expect(groups.getByText(/Uso de Marca de Teste: 2 PV temporários/)).toBeVisible();
    const log = await openCombatLog(p);
    await expect(log.getByText(/Pensantus usou Marca de Teste no Goblin 1/)).toBeVisible();
    await expect(log.getByText(/Marca de Teste no Capitão Goblin/)).toHaveCount(0);
  } finally {
    await endSessionOf(m, campaignId);
    await master.close();
    await player.close();
  }
});

/**
 * The reaction window of a feature with a trigger (#743, board 743-02): the Capitão starts its turn 10 feet from Pensantus, her player reads
 * the prompt with the creature chosen, "Não usar" first and the clock, and answers. "Usar no Capitão Goblin" runs the feature as a reaction
 * (the master and the owner read "gastou a reação" and what is left of the uses in the log); "Não usar" spends nothing.
 */
const haloReplies = [
  {
    answer: 'Usar',
    button: 'Usar no Capitão Goblin',
    logLine: /Pensantus usou Halo de esporos no Capitão Goblin · gastou a reação e 1 uso \(restam 1 de 2\)/,
    logged: 1,
  },
  { answer: 'Não usar', button: 'Não usar', logLine: /Pensantus usou Halo de esporos/, logged: 0 },
];
for (const reply of haloReplies) {
  test(`o jogador responde a janela do Halo de esporos: ${reply.answer}`, { tag: ['@MR-014', '@RN-19'] }, async ({ browser }) => {
    test.setTimeout(240_000);
    const master = await newSignedInContext(browser, 'Mestre Teste', {
      viewport: { width: 1280, height: 900 },
    });
    const player = await newSignedInContext(browser, 'Jogador Teste', {
      viewport: { width: 1280, height: 900 },
    });
    const m = await master.newPage();
    const p = await player.newPage();
    let campaignId = '';
    try {
      await m.goto('/');
      await p.goto('/');
      const table = await tableForCombat(m, p, `Janela do halo ${reply.answer} ${Date.now()}`, true, true, { sheet: pensantusCasting });
      campaignId = table.campaignId;
      await giveFeatRPC(m, campaignId, table.characterId, await createEntryRPC(m, campaignId, 'tableFeat', haloFeatBody()));
      const begun = await beginAttackCombatRPC(m, table, { Pensantus: 20, 'Capitão Goblin': 15, 'Goblin 1': 5, 'Goblin 2': 4 }, haloSquares);
      await openSessionPage(p, campaignId);
      await expect(p.getByRole('heading', { name: 'Sua vez, Pensantus' })).toBeVisible();
      const after = await combatRPC(m, 'EndTurn', {
        campaignId,
        encounterId: begun.id,
        expectedCombatantId: begun.currentCombatantId,
        discardPendingDamage: true,
      });
      await waitForEncounterRevision(p, after);

      const prompt = p.getByRole('alertdialog', { name: /Halo de esporos/ });
      await expect(prompt.getByRole('heading', { name: 'Halo de esporos' })).toBeVisible();
      await expect(prompt.getByText(/O\sCapitão\sGoblin\scomeçou\sa\svez\sa\s3\sm\sde\svocê\.\sUsar\so\sHalo\sde\sesporos\?/)).toBeVisible();
      await expect(prompt.getByRole('radio', { name: /Capitão Goblin/ })).toBeChecked();
      await expect(prompt.getByText(/Se você não responder, passa sozinho em \d+ s\./)).toBeVisible();
      await expect(prompt.getByRole('button', { name: 'Fechar' })).toHaveCount(0);
      await expect(prompt.locator('.pair button').first()).toHaveText('Não usar');

      await prompt.getByRole('button', { name: reply.button }).click();
      await expect(prompt).toHaveCount(0);
      const log = await openCombatLog(p);
      await expect(log.getByText(reply.logLine)).toHaveCount(reply.logged);
      // The turn goes on to the Capitão either way: the window held it until the answer.
      const now = await getEncounterRPC(m, campaignId);
      expect(now.reactionWindows ?? []).toHaveLength(0);
    } finally {
      await endSessionOf(m, campaignId);
      await master.close();
      await player.close();
    }
  });
}

/**
 * The master acts for Pensantus: the use of a rest resource says what it cost and what is left in the log ("gastou a ação bônus e 1 uso de
 * Chamados de Teste (restam 1 de 2)"); with the bonus action spent the next one is dashed with its reason, and the master may press it:
 * the sheet says why and asks "Usar mesmo assim". The options sheet heads a feature's choices with its own question.
 */
test('o mestre usa mesmo assim uma característica cuja ação bônus já foi gasta', { tag: ['@MR-014', '@RN-19'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  const master = await newSignedInContext(browser, 'Mestre Teste', {
    viewport: { width: 1280, height: 900 },
  });
  const player = await newSignedInContext(browser, 'Jogador Teste', {
    viewport: { width: 1280, height: 900 },
  });
  const m = await master.newPage();
  const p = await player.newPage();
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForCombat(m, p, `Usar mesmo assim ${Date.now()}`, true, true, {
      sheet: pensantusCasting,
    });
    campaignId = table.campaignId;
    for (const body of [chamadosFeatBody(), callFeatBody(), elementFeatBody('Elemento de Teste', 'Qual elemento')]) {
      await giveFeatRPC(m, campaignId, table.characterId, await createEntryRPC(m, campaignId, 'tableFeat', body));
    }
    await beginAttackCombatRPC(m, table, { Pensantus: 20, 'Capitão Goblin': 15, 'Goblin 1': 5, 'Goblin 2': 4 }, attackCombatSquares);

    await openSessionPage(m, campaignId);
    await m.getByRole('button', { name: 'Agir por Pensantus' }).click();
    await openActionTab(m, 'Habilidades');
    await m.getByRole('button', { name: 'Usar Elemento de Teste' }).click();
    const element = m.getByRole('dialog', { name: /Elemento de Teste/ });
    await expect(element.getByText('Qual elemento', { exact: true })).toBeVisible();
    await element.getByRole('button', { name: 'Cancelar' }).click();

    // The first bonus action: no override on an enabled row, "Usar" as always.
    await m.getByRole('button', { name: 'Usar Chamados de Teste' }).click();
    const first = m.getByRole('dialog', { name: /Chamados de Teste/ });
    await expect(first.getByText(/Usar mesmo assim/)).toHaveCount(0);
    await first.getByRole('button', { name: 'Usar', exact: true }).click();
    await expect(first).toHaveCount(0);

    // The bonus action is spent: the row is dashed with its reason, and the master may press it.
    const group = await openSpentGroup(m, 'Ação bônus');
    const spent = group.getByRole('button', {
      name: /Usar Chamado de Teste: o mestre pode usar mesmo assim/,
    });
    await expect(spent).toBeEnabled();
    await spent.click();
    const over = m.getByRole('dialog', { name: /Chamado de Teste/ });
    await expect(over.getByText(/O mestre pode usar mesmo assim\./)).toBeVisible();
    await over.locator('label', { hasText: 'Goblin 1' }).click();
    await over.getByRole('button', { name: 'Usar mesmo assim' }).click();
    await expect(over).toHaveCount(0);

    await m.getByRole('heading', { name: 'Registro' }).getByRole('button').click();
    const log = m.getByRole('log', { name: 'Registro do combate' });
    await expect(
      log.getByText(/Pensantus usou Chamados de Teste: 1 PV temporários · gastou a ação bônus e 1 uso de Chamados de Teste \(restam 1 de 2\)/),
    ).toBeVisible();
    await expect(log.getByText(/Pensantus usou Chamado de Teste no Goblin 1: 1 PV temporários · gastou a ação bônus/)).toBeVisible();
  } finally {
    await endSessionOf(m, campaignId);
    await master.close();
    await player.close();
  }
});
