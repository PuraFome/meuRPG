import { expect } from '@playwright/test';
import { test } from './diagnostics';

import { endOpenSessionRPC } from './live-session-support';
import { passClassStep, tableForLevelUp } from './levelup-support';
import { doran, postureSheetRPC } from './option-swap-support';
import { newSignedInContext } from './support';

// #715: a table feature's options may be swapped at the level-up where its `swap_levels` say so, whatever the table's swap
// rule is and at a level with no ability step. The panel "Trocar uma escolha" is then one more panel of the "Escolhas" step.

test(
  'Doran, Guerreiro 4 do Mestre das Posturas, troca a Postura da Garça pela do Lobo no nível 5, que não tem habilidades: o painel está em Escolhas e a ficha mostra a postura nova',
  { tag: ['@MR-040', '@RN-12'] },
  async ({ browser }) => {
    test.setTimeout(150_000);
    const master = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: 1280, height: 800 } });
    const player = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: 1280, height: 800 } });
    const m = await master.newPage();
    const p = await player.newPage();
    let campaignId = '';
    try {
      await m.goto('/');
      await p.goto('/');
      const table = await tableForLevelUp(m, p, `Troca de postura ${Date.now()}`, {
        build: doran,
        prepare: (id) => postureSheetRPC(m, id, 4, [5]),
      });
      campaignId = table.campaignId;
      const sheet = `/campaigns/${campaignId}/characters/${table.characterId}`;
      await p.goto(`${sheet}/level-up`);

      // Classe, Vida, Escolhas, Resumo: the level has no ability step, and the swap is the one panel of Escolhas.
      await expect(p.getByRole('heading', { name: 'Subir para o nível 5' })).toBeVisible();
      await expect(p.getByText(/Passo 1 de 4 · Classe/)).toBeVisible();
      await passClassStep(p);
      await expect(p.getByText('Passo 2 de 4 · Vida')).toBeVisible();
      await p.getByRole('button', { name: 'Próximo' }).click();
      await expect(p.getByText('Passo 3 de 4 · Escolhas')).toBeVisible();
      const panel = p.getByRole('region', { name: 'Trocar uma escolha' });
      await expect(panel).toBeVisible();
      await expect(p.getByRole('heading', { name: 'Trocar uma escolha' })).toHaveCount(1);
      await expect(panel.getByRole('radio', { name: 'Não trocar' })).toBeChecked();
      await panel.getByText('Posturas', { exact: true }).click();
      await expect(panel.getByRole('radio', { name: 'Posturas' })).toBeChecked();
      await panel.getByRole('radiogroup', { name: 'Abrir mão de' }).getByText('Postura da Garça').click();
      await expect(panel).toContainText('Escolha o que deixar e o que pegar.');
      await panel.getByRole('radiogroup', { name: 'Pegar no lugar' }).getByText('Postura do Lobo').click();
      await expect(panel.getByRole('status')).toHaveText('Trocar Postura da Garça por Postura do Lobo');
      await p.getByRole('button', { name: 'Próximo' }).click();

      await expect(p.getByText('Passo 4 de 4 · Resumo')).toBeVisible();
      await expect(p.getByText('Trocar Postura da Garça por Postura do Lobo')).toBeVisible();
      await p.getByRole('button', { name: 'Confirmar o nível 5' }).click();

      // The sheet: the Lobo is picked and the Garça is not.
      await expect(p).toHaveURL(sheet);
      await expect(p.getByText('Doran subiu para o nível 5.').first()).toBeVisible();
      await expect(p.getByText('Postura do Lobo').first()).toBeVisible();
      await expect(p.getByText('Postura do Touro').first()).toBeVisible();
      await expect(p.getByText('Postura da Garça')).toHaveCount(0);
    } finally {
      await endOpenSessionRPC(m, campaignId);
      await master.close();
      await player.close();
    }
  },
);
