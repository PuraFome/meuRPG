import { expect, test } from '@playwright/test';

import { endOpenSessionRPC, openSessionPage } from './live-session-support';
import { addActionRPC, openSceneRPC, rollTyped, sceneActionIdsRPC, setAttemptsRPC, setShowDcRPC, tableForScenes } from './scene-support';
import { newSignedInContext } from './support';

// MR-032 (the session summary on screen), RN-20 and question 64: when the
// master ends the session the master lands on "Sessão encerrada" with the
// numbers, the session's highlights and the table of each player's checks; a
// player gets the card "A sessão acabou" with their own result and no table.
// Only the checks rolled while the scene showed its DC count: a roll made
// with the DC hidden is in neither number, for anyone.

test(
  'ao encerrar a sessão o mestre vê o resumo com a tabela por jogador e o jogador vê o cartão "A sessão acabou"; um teste rolado com a CD escondida não conta',
  { tag: ['@MR-032', '@RN-20'] },
  async ({ browser }) => {
    test.setTimeout(180_000);
    const masterContext = await newSignedInContext(browser, 'Mestre Teste');
    const playerContext = await newSignedInContext(browser, 'Jogador Teste');
    const master = await masterContext.newPage();
    const player = await playerContext.newPage();
    let campaignId = '';
    try {
      await master.goto('/');
      const table = await tableForScenes(master, player, `Resumo ${Date.now()}`, false);
      campaignId = table.campaignId;
      await addActionRPC(master, table, table.cartId, { key: 'skill:investigation', name: 'Procurar pistas na carroça', dc: 12 });
      await addActionRPC(master, table, table.cartId, { key: 'save:con', name: 'Resistir ao cheiro de fumaça', dc: 10 });
      const ids = await sceneActionIdsRPC(master, table, table.cartId);
      await setAttemptsRPC(master, table, table.cartId, ids['Procurar pistas na carroça'], 3);
      await openSceneRPC(master, campaignId, table.cartId);

      await openSessionPage(player, campaignId);
      const scene = player.getByRole('region', { name: 'Cena: A carroça tombada' });
      const clues = scene.getByRole('listitem').filter({ hasText: 'Procurar pistas na carroça' });

      // 15 + 6 = 21 with the DC hidden: it would pass, and it enters no count.
      await rollTyped(player, scene, 'Procurar pistas na carroça', 15);
      await expect(clues).toContainText('Restam 2 de 3 tentativas');
      await expect(scene).not.toContainText(/\bCD\b|passou/i);

      // The master shows the DC: the open scene changes for the player without a reload.
      await setShowDcRPC(master, table, table.cartId, true);
      await expect(clues).toContainText('CD 12');
      // 14 + 6 = 20 passes (counted); Constituição 2 + 3 = 5 fails (tried).
      await rollTyped(player, scene, 'Procurar pistas na carroça', 14);
      await expect(clues).toContainText('Passou · CD 12');
      await rollTyped(player, scene, 'Resistir ao cheiro de fumaça', 2);
      await expect(scene.getByRole('listitem').filter({ hasText: 'Resistir ao cheiro de fumaça' })).toContainText('Não passou · CD 10');

      // The master ends the session (asking first, as before) and lands on the summary.
      await openSessionPage(master, campaignId);
      await master.getByRole('button', { name: 'Encerrar sessão' }).click();
      await master.getByRole('button', { name: 'Confirmar encerramento' }).click();
      await expect(master.getByRole('heading', { name: 'Sessão encerrada' })).toBeVisible();
      const stat = (label: string) => master.locator('.stat').filter({ hasText: label });
      await expect(stat('Combates')).toContainText('0');
      await expect(stat('Cenas abertas')).toContainText('1');
      // 1 passed of 2 tried: the roll with the DC hidden is in neither number.
      await expect(stat('Testes passados fora do combate')).toContainText('1 de 2');
      await expect(master.getByRole('heading', { name: 'Resumo da sessão' })).toBeVisible();
      const tile = master.locator('.tile').filter({ hasText: 'Mais testes passados fora do combate' });
      await expect(tile).toContainText('1 teste');
      await expect(tile).toContainText('Pensantus');
      await expect(tile).toContainText('de 2 tentados');
      const tableRegion = master.getByRole('table', { name: 'Testes passados fora do combate' });
      await expect(tableRegion.getByRole('row').nth(1)).toContainText('Pensantus');
      await expect(tableRegion.getByRole('row').nth(1)).toContainText('1 de 2');
      await expect(master.getByText('Só contam testes de cenas que mostraram a CD aos jogadores;')).toBeVisible();
      // "Voltar à campanha" is the only button; the treasure line is not drawn yet.
      await expect(master.locator('app-session-ended').getByRole('button')).toHaveCount(0);
      await expect(master.getByRole('link', { name: 'Voltar à campanha', exact: true })).toBeVisible();
      await expect(master.getByText('tesouro')).toHaveCount(0);

      // The player gets the card from the stream's `session_ended`, with their own result and no table.
      const card = player.getByRole('region', { name: 'Resumo da sessão' });
      await expect(card.getByRole('heading', { name: 'A sessão acabou' })).toBeVisible();
      await expect(card.getByRole('heading', { name: 'Seu resultado, Pensantus' })).toBeVisible();
      const own = card.locator('.own__tile').filter({ hasText: 'Testes passados fora do combate' });
      await expect(own).toContainText('1 de 2');
      const winner = card.locator('.row').filter({ hasText: 'Mais testes passados fora do combate' });
      await expect(winner).toContainText('1 teste');
      await expect(winner).not.toContainText(/tentados|\bde 2\b/);
      await expect(player.getByRole('table')).toHaveCount(0);
      await expect(player.getByText('Cenas abertas')).toHaveCount(0);
      // The ✕ and an outlined "Fechar" leave the plain notice.
      await expect(card.getByRole('button', { name: 'Fechar' })).toHaveCount(2);
      await card.getByRole('button', { name: 'Fechar' }).last().click();
      await expect(card).toHaveCount(0);
      await expect(player.getByText('A sessão acabou.')).toBeVisible();
    } finally {
      if (campaignId) {
        await endOpenSessionRPC(master, campaignId);
      }
      await masterContext.close();
      await playerContext.close();
    }
  },
);
