import { expect, test } from '@playwright/test';

import { tableWithPensantus } from './live-session-support';
import { newSignedInContext } from './support';

// RN-18 (MR-013, MR-014): how the campaign's players roll dice. The master
// sets it in "Dados" (E6-17); a player sees "Como você rola os dados"
// (E6-18) on the campaign page, as a choice or, when the master decided for
// everyone, as a locked note.
test(
  'o mestre decide como rolam; o jogador escolhe só quando a campanha deixa, e o mestre vê a escolha',
  { tag: '@RN-18' },
  async ({ browser }) => {
    const master = await newSignedInContext(browser, 'Mestre Teste');
    const player = await newSignedInContext(browser, 'Jogador Teste');
    try {
      const masterPage = await master.newPage();
      const playerPage = await player.newPage();
      await masterPage.goto('/');
      await playerPage.goto('/');
      const { campaignId } = await tableWithPensantus(masterPage, playerPage, `Dados ${Date.now()}`);
      await masterPage.goto(`/campanhas/${campaignId}`);
      await playerPage.goto(`/campanhas/${campaignId}`);

      // The default: players choose, and the player may pick.
      const dados = masterPage.getByRole('region', { name: 'Dados' });
      await expect(dados.getByRole('radio', { name: /Cada jogador escolhe/ })).toBeChecked();
      const como = playerPage.getByRole('region', { name: 'Como você rola os dados' });
      await expect(como.getByRole('radio', { name: /No app/ })).toBeChecked();

      // The master makes everyone roll in the app: the player gets the note.
      await dados.getByText('Todos rolam no app', { exact: true }).click();
      await dados.getByRole('button', { name: 'Salvar dados' }).click();
      await expect(dados.getByText('Salvo.')).toBeVisible();
      await playerPage.reload();
      await expect(como.getByText('O mestre decidiu: todos rolam no app.')).toBeVisible();
      await expect(como.getByRole('radio', { name: /Meus próprios dados/ })).toBeDisabled();
      await expect(como.getByRole('button', { name: 'Salvar escolha' })).toHaveCount(0);

      // Back to "Cada jogador escolhe": the player picks their own dice and
      // the master sees it in the list.
      await dados.getByText('Cada jogador escolhe', { exact: true }).click();
      await dados.getByRole('button', { name: 'Salvar dados' }).click();
      await expect(dados.getByText('Salvo.')).toBeVisible();
      await playerPage.reload();
      await como.getByText('Meus próprios dados', { exact: true }).click();
      await como.getByRole('button', { name: 'Salvar escolha' }).click();
      await expect(como.getByText('Salvo.')).toBeVisible();

      await masterPage.reload();
      await expect(dados.getByRole('listitem').filter({ hasText: 'Jogador sem nome' })).toContainText('Meus próprios dados');
    } finally {
      await master.close();
      await player.close();
    }
  },
);
