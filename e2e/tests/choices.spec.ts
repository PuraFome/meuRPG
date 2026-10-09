import { expect, test } from '@playwright/test';

import { tableWithOpenChoices } from './choices-support';
import { endOpenSessionRPC } from './live-session-support';
import { newSignedInContext } from './support';

// PM-05 (class and race choices), RN-01 (the sheet stays locked: only the picks change): a half-elf whose sheet
// locked without its two +1 picks completes them on the page "Completar escolhas pendentes", from the banner of the
// sheet, and the master sees the character tagged until then. The data comes through the API; the screens are what
// is under test. Every test makes its own campaign.

test(
  'um meio-elfo com a ficha travada sem os dois +1 completa as escolhas pela ficha, e o mestre vê a marca até lá',
  { tag: ['@RN-01', '@MR-006'] },
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
      const table = await tableWithOpenChoices(m, p, `Escolhas pendentes ${Date.now()}`);
      campaignId = table.campaignId;
      const sheet = `/campaigns/${campaignId}/characters/${table.characterId}`;

      // The master's list tags the character, and a player's never does.
      await m.goto(`/campaigns/${campaignId}`);
      await expect(m.getByText('2 escolhas em aberto')).toBeVisible();

      // The locked sheet says so, with the way to complete it.
      await p.goto(sheet);
      await expect(p.getByText('Esta ficha tem 2 escolhas pendentes.')).toBeVisible();
      await p.getByRole('link', { name: 'Completar escolhas pendentes' }).click();

      await expect(p).toHaveURL(new RegExp(`${sheet}/choices$`));
      await expect(p.getByRole('heading', { level: 1, name: 'Completar escolhas pendentes' })).toBeVisible();
      await expect(p.getByText(/Escolhas feitas: 0 de \d+/)).toBeVisible();
      const abilities = p.getByRole('region', { name: /\+1 em duas habilidades/ });
      await expect(abilities).toBeVisible();

      // Two picks, no more: the rest are off once the choice is full, and the server's result line comes with the picks.
      await abilities.getByRole('checkbox', { name: /Destreza/ }).click();
      await abilities.getByRole('checkbox', { name: /Constituição/ }).click();
      await expect(abilities.getByRole('checkbox', { name: /Força/ })).toHaveAttribute('aria-disabled', 'true');
      await expect(abilities.getByText('Destreza 16 → 17')).toBeVisible();
      await expect(abilities.getByText('Constituição 15 → 16')).toBeVisible();

      await p.getByRole('button', { name: 'Salvar escolhas' }).click();

      // Back on the sheet, still locked, with nothing pending.
      await expect(p).toHaveURL(new RegExp(`${sheet}$`));
      await expect(p.getByRole('heading', { level: 1 })).toBeVisible();
      await expect(p.getByText('escolhas pendentes')).toHaveCount(0);
      await expect(p.getByRole('link', { name: 'Editar ficha' })).toHaveCount(0);

      // The page of a sheet with nothing open says so.
      await p.goto(`${sheet}/choices`);
      await expect(p.getByText('Pensantus não tem escolhas pendentes.')).toBeVisible();

      await m.goto(`/campaigns/${campaignId}`);
      await expect(m.getByRole('link', { name: /Pensantus/ }).first()).toBeVisible();
      await expect(m.getByText('escolhas em aberto')).toHaveCount(0);
    } finally {
      await endOpenSessionRPC(m, campaignId);
      await master.close();
      await player.close();
    }
  },
);
