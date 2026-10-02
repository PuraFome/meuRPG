import { expect, test } from '@playwright/test';

import { authStatePath, createCampaign } from './support';

// MR-004: the character editor offers what the character can have at its
// level (web/src/app/pages/character-editor). Not a sign-in test, so it
// reuses the master's saved session (auth.setup.ts).
test.use({ storageState: authStatePath('Mestre Teste') });

test(
  'o editor da ficha mostra só as magias do nível e deixa tirar a subclasse',
  { tag: '@MR-004' },
  async ({ page }) => {
    const campaignId = await createCampaign(page, `Editor ${Date.now()}`);
    await page.goto(`/campanhas/${campaignId}/personagens/novo`);

    // A level-1 bard (the default level is 1).
    await page.getByLabel('Nome do personagem', { exact: true }).fill('Lira');
    // Same keyboard route as support.ts's selectMatOption, which is not
    // exported: the panel's options only exist while it is open.
    const classSelect = page.getByRole('combobox', { name: 'Classe', exact: true });
    await classSelect.focus();
    await classSelect.press('Enter');
    await page.getByRole('option', { name: 'Bardo', exact: true }).click();
    await expect(page.getByLabel('Nível', { exact: true })).toHaveValue('1');

    // "Subclasse" can go back to none, and says when the class chooses one.
    const subclassSelect = page.getByRole('combobox', { name: 'Subclasse', exact: true });
    await subclassSelect.focus();
    await subclassSelect.press('Enter');
    await expect(page.getByRole('option').first()).toHaveText('Nenhuma');
    await page.keyboard.press('Escape');
    await expect(page.getByText('O Bardo escolhe a subclasse no nível 3.')).toBeVisible();

    // "Magias conhecidas": only 1st-circle spells at level 1, circle then name.
    await page.getByRole('tab', { name: 'Magias' }).click();
    const known = page.getByRole('group', { name: 'Magias conhecidas', exact: true });
    await expect(known.getByRole('checkbox', { name: /^Amizade Animal/ })).toBeVisible();
    await expect(known.getByRole('checkbox', { name: /^Animar Objetos/ })).toHaveCount(0);

    const labels = await known.locator('mat-checkbox').allInnerTexts();
    const parsed = labels.map((text) => {
      const m = text.trim().match(/^(.*?)\s*\((\d+)º círculo/);
      return { name: m![1], level: Number(m![2]) };
    });
    expect(parsed.length).toBeGreaterThan(5);
    expect(parsed.every((s) => s.level === 1)).toBe(true);
    const names = parsed.map((s) => s.name);
    expect(names).toEqual([...names].sort((a, b) => a.localeCompare(b, 'pt-BR')));
  },
);
