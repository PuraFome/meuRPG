import { expect, test } from '@playwright/test';

import { endOpenSessionRPC, openSessionPage, startSessionRPC } from './live-session-support';
import { canvasPng, tableForMaps, uploadImageRPC } from './maps-support';
import { newSignedInContext } from './support';

// MR-028: the master shows a gallery image to the players during a session.
// Setup goes through the API; the master's and the player's open session
// pages are what is under test.

test(
  'o mestre mostra uma imagem da galeria: a página do jogador mostra "O mestre está mostrando" com a imagem e o nome, e some quando o mestre para',
  { tag: '@MR-028' },
  async ({ browser }) => {
    const masterContext = await newSignedInContext(browser, 'Mestre Teste');
    const playerContext = await newSignedInContext(browser, 'Jogador Teste');
    let campaignId = '';
    const master = await masterContext.newPage();
    try {
      const player = await playerContext.newPage();
      await master.goto('/');
      const table = await tableForMaps(master, player, `Imagem mostrada ${Date.now()}`);
      campaignId = table.campaignId;
      await uploadImageRPC(master, campaignId, 'Capitão Goblin', await canvasPng(master, 400, 500, 'Capitão Goblin', '#3a3a2a'));
      await uploadImageRPC(master, campaignId, 'Taverna do Javali', await canvasPng(master, 800, 500, 'Taverna'));
      await startSessionRPC(master, campaignId);

      await openSessionPage(player, campaignId);
      await openSessionPage(master, campaignId);
      await expect(player.getByText('O mestre está mostrando')).toHaveCount(0);

      // The master's panel starts empty; the picker's filled button is "Mostrar aos jogadores".
      await expect(master.getByText('Nenhuma imagem à mostra.')).toBeVisible();
      await master.getByRole('button', { name: 'Mostrar imagem' }).click();
      const dialog = master.getByRole('dialog', { name: 'Mostrar uma imagem aos jogadores' });
      await dialog.getByRole('button', { name: 'Mostrar aos jogadores' }).click();
      await expect(dialog.getByText('Escolha uma imagem para mostrar.')).toBeVisible();
      await dialog.getByRole('radio', { name: /Capitão Goblin/ }).click();
      await dialog.getByRole('button', { name: 'Mostrar aos jogadores' }).click();
      await expect(dialog).toBeHidden();
      await expect(master.getByText('Capitão Goblin está na tela dos jogadores.')).toBeAttached();
      await expect(master.getByRole('button', { name: 'Parar de mostrar' })).toBeFocused();

      // The player's open page shows it, with the name as caption.
      const block = player.getByRole('region', { name: 'O mestre está mostrando' });
      await expect(block).toBeVisible();
      await expect(block.getByRole('img', { name: 'Capitão Goblin' })).toBeVisible();
      await expect(block.getByText('Capitão Goblin', { exact: true })).toBeVisible();
      await expect(player.getByText('O mestre está mostrando Capitão Goblin.')).toBeAttached();

      // Swapping changes it in place.
      await master.getByRole('button', { name: 'Trocar imagem' }).click();
      const swap = master.getByRole('dialog', { name: 'Mostrar uma imagem aos jogadores' });
      await expect(swap.getByText('À mostra agora')).toBeVisible();
      await swap.getByRole('radio', { name: /Taverna do Javali/ }).click();
      await swap.getByRole('button', { name: 'Mostrar aos jogadores' }).click();
      await expect(block.getByRole('img', { name: 'Taverna do Javali' })).toBeVisible();

      // Stopping takes it off the player's screen, at once for the master.
      await master.getByRole('button', { name: 'Parar de mostrar' }).click();
      await expect(master.getByText('Imagem retirada da tela dos jogadores.')).toBeAttached();
      await expect(master.getByRole('button', { name: 'Mostrar imagem' })).toBeFocused();
      await expect(player.getByRole('region', { name: 'O mestre está mostrando' })).toHaveCount(0);
      await expect(player.getByText('O mestre parou de mostrar a imagem.')).toBeAttached();
    } finally {
      await endOpenSessionRPC(master, campaignId);
      await masterContext.close();
      await playerContext.close();
    }
  },
);
