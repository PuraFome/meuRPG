import { expect, test } from '@playwright/test';

import { combatRPC, getEncounterRPC, setGridRPC, tableForCombat, type Encounter } from './combat-support';
import { endOpenSessionRPC, openSessionPage } from './live-session-support';
import { newSignedInContext } from './support';

// The combat on screen (Etapa 6, slice 6.5a, MR-013, RN-18 to RN-22): the
// master sets the grid and starts a combat, everybody rolls initiative, the
// turns go round, a player moves inside the reach, and the master ends it.
// The table and the NPCs come through the API; what is under test is the
// screens and what each audience sees (RN-10, RN-20).

test(
  'o mestre define a grade e inicia o combate; a iniciativa, os turnos e o fim aparecem para cada um',
  { tag: ['@MR-013', '@RN-19', '@RN-20', '@RN-21'] },
  async ({ browser }) => {
    test.setTimeout(180_000);
    const master = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: 1280, height: 900 } });
    const player = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: 390, height: 844 } });
    const m = await master.newPage();
    const p = await player.newPage();
    let campaignId = '';
    try {
      await m.goto('/');
      await p.goto('/');
      const table = await tableForCombat(m, p, `Combate ${Date.now()}`, false);
      campaignId = table.campaignId;

      // A map without a grid: "Iniciar combate" sends the master to the grid page.
      await m.goto(`/campanhas/${campaignId}/sessao`);
      await m.getByRole('button', { name: 'Iniciar combate' }).click();
      await expect(m.getByText('Esse mapa ainda não tem grade.')).toBeVisible();
      await expect(m.getByRole('button', { name: 'Iniciar combate' }).last()).toHaveAttribute('aria-disabled', 'true');
      await m.getByRole('link', { name: 'Definir a grade' }).click();
      await expect(m.getByRole('heading', { name: 'Grade do mapa' })).toBeVisible();
      // Columns 4 is below the screen's 5 to 60; 30 gives 21 rows for 2000 x 1400.
      await m.getByLabel('Quadrados de 1,5 m na largura').fill('4');
      await expect(m.getByText('Use um número inteiro de 5 a 60.')).toBeVisible();
      await expect(m.getByRole('button', { name: 'Salvar grade' })).toHaveAttribute('aria-disabled', 'true');
      await m.getByLabel('Quadrados de 1,5 m na largura').fill('20');
      await expect(m.getByText('20 × 14')).toBeVisible();
      await expect(m.getByText('30 m × 21 m')).toBeVisible();
      await m.getByRole('button', { name: 'Salvar grade' }).click();
      await expect(m).toHaveURL(/\/sessao$/);

      // Start with the party and three hidden goblins (and the captain).
      await m.getByRole('button', { name: 'Iniciar combate' }).click();
      await expect(m.getByRole('dialog', { name: 'Iniciar combate' })).toBeVisible();
      await expect(m.getByText('20 × 14 quadrados de 1,5 m')).toBeVisible();
      await expect(m.getByText('30 m × 21 m').first()).toBeVisible();
      for (let i = 0; i < 3; i++) {
        await m.getByRole('button', { name: 'Mais um Goblin', exact: true }).click();
      }
      await m.getByRole('button', { name: 'Mais um Capitão Goblin' }).click();
      await expect(m.getByText('Minion · vira Goblin 1, 2 e 3')).toBeVisible();
      await expect(m.getByText('Escondido no início').first()).toBeVisible();
      await m.getByRole('dialog').getByRole('button', { name: 'Iniciar combate' }).click();
      await expect(m.getByRole('heading', { level: 1, name: 'Sessão 1' })).toBeVisible();
      await expect(m.getByText('Os NPCs rolaram sozinhos.')).toBeVisible();
      await expect(m.getByRole('button', { name: 'Começar o combate' })).toHaveAttribute('aria-disabled', 'true');
      await expect(m.getByText('Falta a iniciativa de Pensantus.')).toBeVisible();

      // The player rolls in the app; the NPCs never show on their screen.
      await openSessionPage(p, campaignId);
      await expect(p.getByRole('heading', { name: 'Role a iniciativa' })).toBeVisible();
      await expect(p.getByText('Goblin')).toHaveCount(0);
      await p.getByRole('button', { name: 'Rolar no app' }).click();
      await expect(p.getByText('Esperando o mestre começar o combate')).toBeVisible();

      // Force a tie between two goblins, put the captain first, and break the tie on the screen.
      let enc = await getEncounterRPC(m, campaignId);
      const id = (label: string) => enc.combatants.find((c) => c.label === label)!.id;
      const face = async (label: string, d20Face: number) => {
        enc = await combatRPC(m, 'SubmitInitiative', { campaignId, encounterId: enc.id, combatantId: id(label), d20Face });
      };
      await face('Capitão Goblin', 20);
      // The player's roll stays what the app rolled, except that the master
      // may correct it: here, to make the order the same on every run.
      await face('Pensantus', 19);
      await face('Goblin 1', 7);
      await face('Goblin 2', 7);
      await face('Goblin 3', 3);
      const pen = enc.combatants.find((c) => c.label === 'Pensantus')!;
      const cap = enc.combatants.find((c) => c.label === 'Capitão Goblin')!;
      if (pen.initiative === cap.initiative) {
        await combatRPC(m, 'SetInitiativeOrder', { campaignId, encounterId: enc.id, combatantIds: [cap.id, pen.id] });
      }
      await expect(m.getByText(/Empate em 7: Goblin \d e Goblin \d\. Escolha a ordem com as setas\./)).toBeVisible();
      // The first of the group can't go up: the arrow is a dashed outline that says so and does nothing.
      const firstUp = m.locator('[aria-label^="Subir Goblin"][aria-disabled="true"]');
      await expect(firstUp).toHaveCount(1);
      const first = ((await firstUp.getAttribute('aria-label')) ?? '').replace(/^Subir | na ordem$/g, '');
      await m.getByRole('button', { name: `Descer ${first} na ordem` }).click();
      await expect(m.getByText('Empate em 7: Goblin')).toHaveCount(0);
      await expect(m.getByText('Todos rolaram.')).toBeVisible();

      // Put the goblins on squares, then begin.
      for (const [label, col, row] of [['Goblin 1', 9, 9], ['Goblin 2', 14, 10], ['Goblin 3', 15, 4], ['Capitão Goblin', 11, 5]] as const) {
        enc = await combatRPC(m, 'MoveCombatant', { campaignId, encounterId: enc.id, combatantId: id(label), col, row });
      }
      await m.getByRole('button', { name: 'Começar o combate' }).click();
      await expect(m.getByRole('button', { name: 'Próximo turno' })).toBeVisible();
      await expect(m.getByText('Vez do Capitão Goblin')).toBeVisible();
      await expect(m.getByText('Rodada 1', { exact: true })).toBeVisible();

      // The captain is hidden: the player sees "Vez do mestre", never a goblin.
      await expect(p.getByRole('heading', { name: 'Vez do mestre' })).toBeVisible();
      await expect(p.getByText('Goblin')).toHaveCount(0);
      await expect(p.getByRole('listitem').filter({ hasText: 'Pensantus' }).first()).toBeVisible();

      // "Próximo turno": Pensantus, and the player is told.
      await m.getByRole('button', { name: 'Próximo turno' }).click();
      await expect(m.getByText('Vez do Pensantus')).toBeVisible();
      await expect(p.getByRole('heading', { name: 'Sua vez, Pensantus' })).toBeVisible();

      // The player moves 3 m (2 right, 1 down) inside the reach, and is refused beyond it.
      await p.getByRole('button', { name: 'Mover' }).click();
      await expect(p.getByRole('heading', { name: 'Mover Pensantus' })).toBeVisible();
      const map = p.getByRole('group', { name: /Mapa de batalha/ });
      const box = (await map.boundingBox())!;
      const w = box.width / 20;
      const h = box.height / 14;
      const start = enc.combatants.find((c) => c.label === 'Pensantus')!;
      const own = (await getEncounterRPC(p, campaignId)).combatants.find((c) => c.mine)!;
      const at = (dc: number, dr: number) => ({ x: ((own.col ?? 0) + dc + 0.5) * w, y: ((own.row ?? 0) + dr + 0.5) * h });
      expect(start).toBeTruthy();
      await map.click({ position: at(8, 1) });
      await expect(p.getByText('Longe demais: faltam')).toBeVisible();
      await expect(p.getByRole('button', { name: 'Mover para cá' })).toHaveAttribute('aria-disabled', 'true');
      await map.click({ position: at(2, 1) });
      await expect(p.getByRole('status').filter({ hasText: 'Mover 3 m' })).toContainText('2 quadrados para a direita e 1 quadrado para baixo. Depois restam 4,5 m.');
      await p.getByRole('button', { name: 'Mover para cá' }).click();
      await expect(p.getByRole('heading', { name: 'Sua vez, Pensantus' })).toBeVisible();
      await expect.poll(async () => (await getEncounterRPC(m, campaignId)).combatants.find((c) => c.label === 'Pensantus')?.col).toBe((own.col ?? 0) + 2);

      // The master asks before ending, and the summary shows what happened.
      await m.getByRole('button', { name: 'Encerrar combate' }).click();
      await expect(m.getByRole('alertdialog', { name: 'Encerrar o combate?' })).toBeVisible();
      await expect(m.getByRole('button', { name: 'Cancelar' })).toBeFocused();
      await m.getByRole('button', { name: 'Encerrar combate' }).last().click();
      await expect(m.getByRole('heading', { name: 'Combate encerrado' })).toBeVisible();
      await expect(p.getByRole('heading', { name: 'Combate encerrado' })).toBeVisible();
      await m.getByRole('button', { name: 'Voltar à sessão' }).click();
      await expect(m.getByRole('button', { name: 'Iniciar combate' })).toBeVisible();
    } finally {
      if (campaignId) {
        await endOpenSessionRPC(m, campaignId);
      }
      await master.close();
      await player.close();
    }
  },
);

test('um jogador que digita os dados rola a iniciativa com o número do dado físico', { tag: ['@MR-013', '@RN-18'] }, async ({ browser }) => {
  test.setTimeout(120_000);
  const master = await newSignedInContext(browser, 'Mestre Teste');
  const player = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: 390, height: 844 } });
  const m = await master.newPage();
  const p = await player.newPage();
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForCombat(m, p, `Dados físicos ${Date.now()}`);
    campaignId = table.campaignId;
    const pref = await p.request.post('/meurpg.campaigns.v1.CampaignService/SetMyDicePreference', {
      data: { campaignId, preference: 'DICE_PREFERENCE_PHYSICAL' },
      headers: { 'Connect-Protocol-Version': '1' },
    });
    expect(pref.ok()).toBeTruthy();
    await combatRPC(m, 'StartEncounter', {
      campaignId,
      name: 'Dados',
      participants: [{ characterId: table.goblinId, count: 1 }],
    });
    await openSessionPage(p, campaignId);
    // While the campaign lets each player choose, both ways are offered (RN-18); the saved choice only
    // decides which one is the filled button.
    await expect(p.getByRole('button', { name: 'Rolar no app' })).toBeVisible();
    await p.getByRole('button', { name: 'Digitar o resultado' }).click();
    await p.getByLabel(/Role 1d20 para a iniciativa/).fill('99');
    await expect(p.getByText('Digite um número de 1 a 20')).toBeVisible();
    await p.getByLabel(/Role 1d20 para a iniciativa/).fill('12');
    await expect(p.getByText('12 + 3 = 15 · dado físico')).toBeVisible();
    await p.getByRole('button', { name: 'Confirmar 12' }).click();
    await expect(p.getByText('Esperando o mestre começar o combate')).toBeVisible();
    await expect(p.getByText('1d20 (12) + 3 = 15')).toBeVisible();
    const enc = await getEncounterRPC(m, campaignId);
    expect(enc.combatants.find((c: Encounter['combatants'][number]) => c.label === 'Pensantus')?.initiative).toBe(15);
    await setGridRPC(m, campaignId, table.mapId, 20);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
});
