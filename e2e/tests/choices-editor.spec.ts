import { expect, test, type Page } from '@playwright/test';

import { campaignWithEmptyPlayer, method } from './table-rules-support';
import { callRPC, newSignedInContext } from './support';

test.describe.configure({ timeout: 120_000 });

// RN-33 (class and race choices): the "Escolhas" step of the editor. A player does not create a sheet with a choice
// open; the step shows what the race and the class ask, with the rule of each option, and the options that cannot be
// taken yet stay in the list, dotted, with the reason. Every test makes its own campaign through the API.

/** Opens a `mat-select` by its label and picks an option (keyboard, as `support.ts` does). */
async function pick(page: Page, label: string, option: string): Promise<void> {
  const control = page.getByRole('combobox', { name: label, exact: true });
  await control.focus();
  await control.press('Enter');
  await page.getByRole('option', { name: option, exact: true }).click();
  await expect(control).toHaveAttribute('aria-expanded', 'false');
}

/** The sheet's stored picks, as the server holds them. */
async function storedPicks(page: Page, campaignId: string): Promise<string[]> {
  const list = await callRPC(page, 'meurpg.characters.v1.CharacterService/GetCharacter', {
    campaignId,
    characterId: page.url().split('/').pop()!,
  });
  expect(list.ok(), await list.text()).toBeTruthy();
  return (await list.json()).character.sheet.full.featureChoiceKeys as string[];
}

test(
  'um draconato guerreiro escolhe o ancestral e o estilo de luta, e só então cria o personagem @RN-33',
  { tag: '@RN-33' },
  async ({ browser }) => {
    const master = await newSignedInContext(browser, 'Mestre Teste');
    const player = await newSignedInContext(browser, 'Jogador Teste');
    try {
      const m = await master.newPage();
      const p = await player.newPage();
      await Promise.all([m.goto('/'), p.goto('/')]);
      const campaignId = await campaignWithEmptyPlayer(m, p, `Draconato ${Date.now()}`);

      await p.goto(`/campaigns/${campaignId}/characters/new`);
      await p.getByLabel('Nome do personagem', { exact: true }).fill('Tharn');
      await pick(p, 'Raça', 'Draconato');
      await pick(p, 'Classe', 'Guerreiro');
      await pick(p, 'Antecedente', 'Acólito');
      await p.getByRole('tab', { name: 'Habilidades' }).click();
      await method(p, 'Digitar');

      // The step appears once the race and the class are known, marked as pending in words.
      const tab = p.getByRole('tab', { name: /Escolhas/ });
      await expect(tab).toBeVisible();
      await expect(tab).toContainText('Escolha pendente');

      // Two choices are open: "Criar personagem" says so, stays reachable, and leads to the first one.
      const create = p.getByRole('button', { name: 'Criar personagem' });
      await expect(create).toHaveAttribute('aria-disabled', 'true');
      await expect(create).toHaveAccessibleDescription(/Faltam escolhas: .*Estilo de Luta/);
      await create.focus();
      await p.keyboard.press('Enter');
      await expect(p).toHaveURL(/characters\/new$/);
      await expect(p.getByRole('heading', { level: 3, name: /Ancestral Dracônico/ })).toBeFocused();
      await expect(p.getByText(/Escolhas feitas: 0 de 2/)).toBeVisible();

      const ancestry = p.getByRole('region', { name: /Ancestral Dracônico/ });
      await ancestry.getByRole('radio', { name: /Vermelho/ }).click();
      await expect(ancestry.getByRole('radio', { name: /Vermelho/ })).toBeChecked();
      const style = p.getByRole('region', { name: /Estilo de Luta/ });
      await style.getByRole('radio', { name: /Defesa/ }).click();
      await expect(p.getByText(/Escolhas feitas: 2 de 2/)).toBeVisible();

      // Nothing left open: the marker is gone, and the button creates.
      await expect(tab).not.toContainText('Escolha pendente');
      await expect(create).toHaveAttribute('aria-disabled', 'false');
      await create.click();

      await expect(p).toHaveURL(/\/campaigns\/[^/]+\/characters\/(?!new$)[^/]+$/);
      const picks = JSON.stringify(await storedPicks(p, campaignId));
      expect(picks).toContain('red');
      expect(picks).toContain('defense');
    } finally {
      await master.close();
      await player.close();
    }
  },
);

test(
  'um bruxo de nível 5 escolhe a dádiva e três invocações, e a que ainda não pode ser escolhida fica na lista com o motivo @RN-33',
  { tag: '@RN-33' },
  async ({ browser }) => {
    const master = await newSignedInContext(browser, 'Mestre Teste');
    const player = await newSignedInContext(browser, 'Jogador Teste');
    try {
      const m = await master.newPage();
      const p = await player.newPage();
      await Promise.all([m.goto('/'), p.goto('/')]);
      const campaignId = await campaignWithEmptyPlayer(m, p, `Bruxo ${Date.now()}`);

      await p.goto(`/campaigns/${campaignId}/characters/new`);
      await p.getByLabel('Nome do personagem', { exact: true }).fill('Mirela');
      await pick(p, 'Raça', 'Humano');
      await pick(p, 'Classe', 'Bruxo');
      await p.getByLabel('Nível', { exact: true }).fill('5');
      await pick(p, 'Subclasse', 'O Corruptor');
      await pick(p, 'Antecedente', 'Acólito');
      await p.getByRole('tab', { name: 'Habilidades' }).click();
      await method(p, 'Digitar');

      await p.getByRole('tab', { name: /Escolhas/ }).click();
      const pact = p.getByRole('region', { name: /Dádiva do Pacto/ });
      await pact.getByRole('radio', { name: /Pacto da Lâmina/ }).click();

      // Three invocations at level 5. One of them asks for a level the warlock has not reached: it stays in the
      // list, dotted (aria-disabled, still focusable), and its card says why.
      const invocations = p.getByRole('region', { name: /Invocações Místicas/ });
      const blocked = invocations.getByRole('checkbox', { name: /Passo Ascendente/ });
      await expect(blocked).toHaveAttribute('aria-disabled', 'true');
      await blocked.focus();
      await expect(blocked).toBeFocused();
      await expect(blocked).toContainText('9');
      await p.keyboard.press('Space');
      await expect(blocked).toHaveAttribute('aria-checked', 'false');

      for (const name of [/Armadura de Sombras/, /Idioma Bestial/, /Visão Diabólica/]) {
        await invocations.getByRole('checkbox', { name }).click();
      }
      await expect(p.getByRole('status').filter({ hasText: 'Já escolheu 3.' })).toBeVisible();
      await expect(p.getByRole('tab', { name: /Escolhas/ })).not.toContainText('Escolha pendente');

      await p.getByRole('button', { name: 'Criar personagem' }).click();
      await expect(p).toHaveURL(/\/campaigns\/[^/]+\/characters\/(?!new$)[^/]+$/);
      const picks = JSON.stringify(await storedPicks(p, campaignId));
      for (const key of ['armor-of-shadows', 'beast-speech', 'devils-sight', 'blade']) {
        expect(picks).toContain(key);
      }
      expect(picks).not.toContain('ascendant-step');
    } finally {
      await master.close();
      await player.close();
    }
  },
);
