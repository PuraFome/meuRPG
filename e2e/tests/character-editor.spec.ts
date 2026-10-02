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

// MR-004 (E6-20 to E6-23): creation rolls happen in the browser and the "?"
// reads a spell. Each test opens its own campaign and the player's editor.
async function openEditorAs(page: import('@playwright/test').Page, className: string) {
  const campaignId = await createCampaign(page, `Rolagens ${Date.now()}`);
  await page.goto(`/campanhas/${campaignId}/personagens/novo`);
  await page.getByLabel('Nome do personagem', { exact: true }).fill('Zézinho');
  const classSelect = page.getByRole('combobox', { name: 'Classe', exact: true });
  await classSelect.focus();
  await classSelect.press('Enter');
  await page.getByRole('option', { name: className, exact: true }).click();
}

const ABILITIES = ['Força', 'Destreza', 'Constituição', 'Inteligência', 'Sabedoria', 'Carisma'];

test(
  'rolar 4d6 dá seis resultados e todos podem ser colocados nos atributos',
  { tag: '@MR-004' },
  async ({ page }) => {
    await openEditorAs(page, 'Bárbaro');
    await page.getByRole('tab', { name: 'Atributos' }).click();
    await page.getByRole('radio', { name: /Rolar 4d6/ }).check();

    // Six results, each with its four dice and the discarded one named.
    const chips = page.getByRole('group', {
      name: /^\d+: dados \d, \d, \d e \d; o \d foi descartado/,
    });
    await expect(chips).toHaveCount(6);
    const totals: number[] = [];
    for (const label of await chips.evaluateAll((els) =>
      els.map((e) => e.getAttribute('aria-label')!),
    )) {
      totals.push(Number(label.split(':')[0]));
    }
    expect(totals).toEqual([...totals].sort((a, b) => b - a));
    totals.forEach((t) => {
      expect(t).toBeGreaterThanOrEqual(3);
      expect(t).toBeLessThanOrEqual(18);
    });

    // Saving waits until every result has an ability.
    await expect(page.getByText('Faltam 6 atributos')).toBeVisible();
    await page.getByRole('button', { name: 'Criar personagem' }).click();
    await expect(page.getByText(/coloque cada resultado num atributo/)).toBeVisible();

    // The failed save took us to the first step with a problem: back to this one.
    await page.getByRole('tab', { name: 'Atributos' }).click();
    // Result i goes to ability i (option 0 is "Escolher").
    for (const [i, ability] of ABILITIES.entries()) {
      await page.getByLabel(ability, { exact: true }).selectOption({ index: i + 1 });
    }
    await expect(page.getByText('Os seis resultados estão colocados.')).toBeVisible();
    const placed: number[] = [];
    for (const ability of ABILITIES) {
      const text = await page
        .getByLabel(ability, { exact: true })
        .evaluate((s: HTMLSelectElement) => s.selectedOptions[0].textContent!.trim());
      placed.push(Number(text));
    }
    expect(placed).toEqual(totals);
    expect(placed.reduce((a, b) => a + b, 0)).toBe(totals.reduce((a, b) => a + b, 0));
    await expect(page.getByRole('group', { name: /\. Em Força\.$/ })).toHaveCount(1);
  },
);

test(
  'o conjunto padrão pode ser colocado e trocado entre atributos',
  { tag: '@MR-004' },
  async ({ page }) => {
    await openEditorAs(page, 'Bárbaro');
    await page.getByRole('tab', { name: 'Atributos' }).click();
    await page.getByRole('radio', { name: /Conjunto padrão/ }).check();

    const chips = page.getByRole('group', { name: /^\d+\. (Livre|Em )/ });
    await expect(chips).toHaveCount(6);
    await expect(page.getByRole('button', { name: 'Rolar de novo' })).toHaveCount(0);

    for (const [i, ability] of ABILITIES.entries()) {
      await page.getByLabel(ability, { exact: true }).selectOption({ index: i + 1 });
    }
    await expect(page.getByLabel('Força', { exact: true })).toHaveValue('0');
    // Força takes Destreza's 14: the two swap, so the six stay the standard array.
    await page.getByLabel('Força', { exact: true }).selectOption({ index: 2 });
    const shown = await Promise.all(
      ABILITIES.map((a) =>
        page
          .getByLabel(a, { exact: true })
          .evaluate((s: HTMLSelectElement) => Number(s.selectedOptions[0].textContent!.trim())),
      ),
    );
    expect(shown[0]).toBe(14);
    expect(shown[1]).toBe(15);
    expect([...shown].sort((a, b) => b - a)).toEqual([15, 14, 13, 12, 10, 8]);
  },
);

test(
  'os pontos de vida rolados preenchem todos os níveis que faltam',
  { tag: '@MR-004' },
  async ({ page }) => {
    await openEditorAs(page, 'Bárbaro');
    await page.getByLabel('Nível', { exact: true }).fill('3');
    await page.getByRole('tab', { name: 'Atributos' }).click();
    await page.getByRole('radio', { name: /Rolado/ }).check();

    const level2 = page.getByLabel('Nível 2 (1d12)', { exact: true });
    const level3 = page.getByLabel('Nível 3 (1d12)', { exact: true });
    await expect(level2).toHaveValue('');
    await page.getByRole('button', { name: 'Rolar os níveis que faltam' }).click();
    for (const field of [level2, level3]) {
      const value = Number(await field.inputValue());
      expect(value).toBeGreaterThanOrEqual(1);
      expect(value).toBeLessThanOrEqual(12);
    }
    await expect(page.getByRole('button', { name: 'Rolar os níveis que faltam' })).toBeDisabled();
    await expect(page.getByText(/PV máximos até agora/)).toBeVisible();
    await expect(page.getByText(/É uma prévia/)).toBeVisible();
  },
);

test(
  'o "?" ao lado da magia abre a descrição, com o texto do SRD em inglês',
  { tag: '@MR-004' },
  async ({ page }) => {
    await openEditorAs(page, 'Bardo');
    await page.getByRole('tab', { name: 'Magias' }).click();
    const help = page.getByRole('button', { name: 'Descrição de Amizade Animal' });
    await help.click();

    const dialog = page.getByRole('dialog');
    await expect(dialog.getByRole('heading', { name: 'Amizade Animal' })).toBeVisible();
    await expect(dialog.getByText('1º círculo · Encantamento')).toBeVisible();
    for (const label of ['Tempo de conjuração', 'Alcance', 'Componentes', 'Duração']) {
      await expect(dialog.getByText(label, { exact: true })).toBeVisible();
    }
    await expect(dialog.getByText('1 ação')).toBeVisible();
    await expect(dialog.getByText('Texto do SRD 5.1 (em inglês)')).toBeVisible();
    await expect(dialog.locator('.spell__prose[lang="en"]')).toContainText('beast');

    // The X and the button at the bottom are both "Fechar".
    await dialog.getByRole('button', { name: 'Fechar', exact: true }).last().click();
    await expect(dialog).toHaveCount(0);
    await expect(help).toBeFocused();
  },
);
