import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Browser, type Page } from '@playwright/test';

import { canvasJpeg, newCampaign, uploadThroughPicker } from './gallery-support';
import { authStatePath, callRPC, characterRpcBody, createCharacterRPC, pensantus } from './support';

// docs/design.md#como-uma-tela-é-feita: every screen passes axe with no
// serious or critical violation of WCAG 2.1 A and AA, in the light and the
// dark theme (contrast is checked per theme), at desktop and phone widths.
//
// Each test builds its own campaign and NPC through the API, so the screens
// have real content, and reuses the saved sign-in (no /auth/login hit).

const wcag = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'];

/** Scans the page and fails with one readable line per serious or critical
 * violation: the rule, what it means and the first few elements. */
async function expectNoSeriousViolations(page: Page, screen: string): Promise<void> {
  const results = await new AxeBuilder({ page }).withTags(wcag).analyze();
  const serious = results.violations
    .filter((v) => v.impact === 'serious' || v.impact === 'critical')
    .map((v) => `${screen}: ${v.id} (${v.impact}) ${v.help}: ${v.nodes.slice(0, 3).map((n) => n.target.join(' ')).join(' | ')}`);
  expect(serious).toEqual([]);
}

/** Opens a route and waits for its h1 and for its sections' calls to
 * finish, so axe scans the loaded screen. (A loading state must pass too:
 * a spinner needs an accessible name. This just keeps each scan about one
 * state.) */
async function open(page: Page, route: string): Promise<void> {
  await page.goto(route);
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  await page.waitForLoadState('networkidle');
}

/** A campaign with a full-sheet enemy, created through the API as the
 * master. */
async function campaignWithNpc(page: Page): Promise<{ campaignId: string; npcId: string }> {
  const created = await callRPC(page, 'meurpg.campaigns.v1.CampaignService/CreateCampaign', {
    name: `Acessibilidade ${Date.now()}`,
    xpMode: 'XP_MODE_ENEMIES',
  });
  expect(created.ok()).toBeTruthy();
  const campaignId = (await created.json()).campaign.id as string;
  const npc = await createCharacterRPC(page, campaignId, characterRpcBody('ENEMY', { ...pensantus, name: 'Capitão Goblin' }));
  expect(npc.ok()).toBeTruthy();
  return { campaignId, npcId: (await npc.json()).character.id as string };
}

async function scanMasterScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const context = await browser.newContext({
    storageState: authStatePath('Mestre Teste'),
    colorScheme,
    viewport: { width, height: 900 },
  });
  const page = await context.newPage();
  try {
    await page.goto('/');
    const { campaignId, npcId } = await campaignWithNpc(page);
    const screens: [string, string][] = [
      ['Início', '/'],
      ['Campanha', `/campanhas/${campaignId}`],
      ['Ficha do NPC', `/campanhas/${campaignId}/personagens/${npcId}`],
      ['Editar a ficha do NPC', `/campanhas/${campaignId}/personagens/${npcId}/editar`],
      ['Novo NPC básico', `/campanhas/${campaignId}/npcs/novo/minion`],
      ['Meu perfil', '/perfil'],
      ['Créditos', '/creditos'],
    ];
    for (const [screen, route] of screens) {
      await open(page, route);
      await expectNoSeriousViolations(page, `${screen} (${colorScheme}, ${width}px)`);
    }
  } finally {
    await context.close();
  }
}

test('as telas do mestre passam no axe no tema claro, no desktop', { tag: '@a11y' }, async ({ browser }) => {
  await scanMasterScreens(browser, 'light', 1280);
});

test('as telas do mestre passam no axe no tema escuro, no celular', { tag: '@a11y' }, async ({ browser }) => {
  await scanMasterScreens(browser, 'dark', 390);
});

/** The gallery (MR-019): empty, with images, a refused upload's notice,
 * a card's delete confirmation and the lightbox open. The gallery picker
 * (shared/gallery-picker) has no screen of its own until the map form
 * (5.3) uses it; its radio-group semantics are covered by its unit tests,
 * and it joins this scan with that screen. */
async function scanGallery(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const context = await browser.newContext({
    storageState: authStatePath('Mestre Teste'),
    colorScheme,
    viewport: { width, height: 900 },
  });
  const page = await context.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  try {
    await page.goto('/');
    const campaignId = await newCampaign(page, `Acessibilidade galeria ${Date.now()}`);
    await open(page, `/campanhas/${campaignId}/galeria`);
    await expect(page.getByRole('heading', { name: 'Nenhuma imagem ainda' })).toBeVisible();
    await expectNoSeriousViolations(page, `Galeria vazia ${where}`);

    await uploadThroughPicker(page, [
      { name: 'Taverna do Javali.jpg', mimeType: 'image/jpeg', buffer: await canvasJpeg(page) },
      { name: 'Covil dos goblins.jpg', mimeType: 'image/jpeg', buffer: await canvasJpeg(page, '#5b4834') },
      { name: 'mapa-antigo.gif', mimeType: 'image/gif', buffer: Buffer.from('GIF89a') },
    ]);
    await expect(page.getByRole('article', { name: 'Covil dos goblins', exact: true })).toBeVisible();
    await expect(page.getByRole('alert')).toBeVisible();
    await expectNoSeriousViolations(page, `Galeria com imagens e um envio recusado ${where}`);

    const card = page.getByRole('article', { name: 'Taverna do Javali', exact: true });
    await card.getByRole('button', { name: 'Apagar Taverna do Javali' }).click();
    await expect(card.getByRole('button', { name: 'Apagar imagem' })).toBeFocused();
    await expectNoSeriousViolations(page, `Galeria, confirmar exclusão ${where}`);
    await card.getByRole('button', { name: 'Cancelar' }).click();

    await card.getByRole('button', { name: 'Ver Taverna do Javali' }).first().click();
    await expect(page.getByRole('dialog', { name: 'Taverna do Javali' })).toBeVisible();
    await expectNoSeriousViolations(page, `Galeria, imagem aberta ${where}`);
    await page.keyboard.press('Escape');

    await open(page, `/campanhas/${campaignId}`);
    await expect(page.getByRole('link', { name: 'Abrir galeria' })).toBeVisible();
    await expectNoSeriousViolations(page, `Campanha com o painel Galeria ${where}`);
  } finally {
    await context.close();
  }
}

// Five scans and three uploads in one test: more room than the default.
test('a galeria passa no axe no tema claro, no desktop', { tag: ['@a11y', '@MR-019'] }, async ({ browser }) => {
  test.slow();
  await scanGallery(browser, 'light', 1280);
});

test('a galeria passa no axe no tema escuro, no celular', { tag: ['@a11y', '@MR-019'] }, async ({ browser }) => {
  test.slow();
  await scanGallery(browser, 'dark', 390);
});

test('as telas de quem não entrou passam no axe, nos dois temas', { tag: '@a11y' }, async ({ browser }) => {
  for (const colorScheme of ['light', 'dark'] as const) {
    const context = await browser.newContext({ colorScheme });
    const page = await context.newPage();
    try {
      for (const [screen, route] of [
        ['Início', '/'],
        ['Créditos', '/creditos'],
        ['Página não encontrada', '/nao-existe'],
      ]) {
        await open(page, route);
        await expectNoSeriousViolations(page, `${screen} (${colorScheme}, sem login)`);
      }
    } finally {
      await context.close();
    }
  }
});
