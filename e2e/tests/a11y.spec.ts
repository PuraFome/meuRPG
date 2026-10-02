import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Browser, type Page } from '@playwright/test';

import { canvasJpeg, newCampaign, uploadThroughPicker } from './gallery-support';
import { saveDocumentRPC, tableWithDocumentParts } from './document-support';
import { endOpenSessionRPC, endSessionRPC, openSessionPage, startSessionRPC, tableWithPensantus } from './live-session-support';
import { canvasPng, createMapRPC, createPointRPC, placeTokenRPC, revealMapRPC, setCurrentMapRPC, tableForMaps, uploadImageRPC } from './maps-support';
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

/** The campaign document (MR-018): read mode, edit mode (with its toolbar
 * and the preview), the map dialog and the image picker dialog. */
async function scanDocument(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const context = await browser.newContext({
    storageState: authStatePath('Mestre Teste'),
    colorScheme,
    viewport: { width, height: 900 },
  });
  const page = await context.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  try {
    await page.goto('/');
    const t = await tableWithDocumentParts(page, `Acessibilidade documento ${Date.now()}`);
    await saveDocumentRPC(
      page,
      t.campaignId,
      `## Arco 1\n\nVeja [Mirathel e arredores](mapa:${t.mapId}) e [Capitão Goblin](ficha:${t.npcId}), com **negrito** e *itálico*.\n\n![Taverna do Javali](imagem:${t.imageId})\n\n## Segredos\n\n- um\n- dois`,
      0,
    );
    await open(page, `/campanhas/${t.campaignId}/documento`);
    await expect(page.getByRole('button', { name: 'Mirathel e arredores' })).toBeVisible();
    await expectNoSeriousViolations(page, `Documento, leitura ${where}`);

    await page.getByRole('button', { name: 'Mirathel e arredores' }).click();
    const dialog = page.getByRole('dialog', { name: 'Mirathel e arredores' });
    await expect(dialog.getByRole('img')).toBeVisible();
    await expectNoSeriousViolations(page, `Documento, janela do mapa ${where}`);
    await page.keyboard.press('Escape');

    await page.getByRole('button', { name: 'Editar documento' }).click();
    await expect(page.getByRole('textbox', { name: 'Texto' })).toBeVisible();
    await expectNoSeriousViolations(page, `Documento, edição ${where}`);

    await page.getByRole('button', { name: 'Imagem da galeria' }).click();
    await expect(page.getByRole('dialog', { name: 'Imagem da galeria' }).getByRole('radio').first()).toBeVisible();
    await expectNoSeriousViolations(page, `Documento, escolher imagem ${where}`);
  } finally {
    await context.close();
  }
}

test('o documento passa no axe no tema claro, no desktop', { tag: ['@a11y', '@MR-018'] }, async ({ browser }) => {
  test.slow();
  await scanDocument(browser, 'light', 1280);
});

test('o documento passa no axe no tema escuro, no celular', { tag: ['@a11y', '@MR-018'] }, async ({ browser }) => {
  test.slow();
  await scanDocument(browser, 'dark', 390);
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

// docs/design.md#cor: every control that takes focus shows the same 2px
// ring. Material's buttons remove their outline in their own styles, so
// this checks them explicitly (axe does not check that a focus ring shows).
test('os botões do Material mostram o anel de foco @a11y', async ({ page }) => {
  await page.goto('/uma-pagina-que-nao-existe');
  for (const name of ['Voltar para o início', 'Minhas campanhas']) {
    const button = page.getByRole('main').getByRole('link', { name });
    await button.focus();
    const ring = await button.evaluate((el) => {
      const style = getComputedStyle(el);
      return { style: style.outlineStyle, width: style.outlineWidth };
    });
    expect(ring, name).toEqual({ style: 'solid', width: '2px' });
  }
});

/**
 * The live session's screens (Etapa 5): the session page for the master and
 * for the player, the adjust sheet open (a dialog on the desktop, a bottom
 * sheet on the phone), and the link opened by someone who isn't in the
 * campaign. The session page keeps a stream open, so these wait for the
 * page's own "Ao vivo" instead of `networkidle`.
 */
async function scanLiveSessionScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  test.setTimeout(90_000);
  const options = { colorScheme, viewport: { width, height: 900 } };
  const master = await browser.newContext({ ...options, storageState: authStatePath('Mestre Teste') });
  const player = await browser.newContext({ ...options, storageState: authStatePath('Jogador Teste') });
  const masterPage = await master.newPage();
  const playerPage = await player.newPage();
  const suffix = `(${colorScheme}, ${width}px)`;
  try {
    await masterPage.goto('/');
    await playerPage.goto('/');
    const { campaignId } = await tableWithPensantus(masterPage, playerPage, `Acessibilidade ao vivo ${Date.now()}`);
    const sessionId = await startSessionRPC(masterPage, campaignId);

    await openSessionPage(masterPage, campaignId);
    await expectNoSeriousViolations(masterPage, `Sessão, mestre ${suffix}`);

    await masterPage.getByRole('button', { name: 'Ajustar Pensantus' }).click();
    await expect(masterPage.getByRole('dialog', { name: 'Ajustar Pensantus' })).toBeVisible();
    // Scan the sheet once it's in place: mid-animation, its text is still
    // fading in, and axe would measure the contrast of a half-drawn frame.
    await masterPage.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectNoSeriousViolations(masterPage, `Ajustar PV ${suffix}`);
    await masterPage.getByRole('button', { name: 'Cancelar' }).click();

    await openSessionPage(playerPage, campaignId);
    await expect(playerPage.getByRole('region', { name: 'Pensantus' })).toBeVisible();
    await expectNoSeriousViolations(playerPage, `Sessão, jogador ${suffix}`);

    // A campaign Jogador Teste isn't in: the link says to ask for an invite.
    const closed = await callRPC(masterPage, 'meurpg.campaigns.v1.CampaignService/CreateCampaign', {
      name: `Mesa fechada ${Date.now()}`,
      xpMode: 'XP_MODE_ENEMIES',
    });
    await playerPage.goto(`/campanhas/${(await closed.json()).campaign.id}/sessao`);
    await expect(playerPage.getByRole('heading', { level: 1, name: 'Peça um convite ao mestre' })).toBeVisible();
    await expectNoSeriousViolations(playerPage, `Sessão sem acesso ${suffix}`);

    await endSessionRPC(masterPage, campaignId, sessionId);
  } finally {
    await master.close();
    await player.close();
  }
}

test('as telas da sessão ao vivo passam no axe no tema claro, no desktop', { tag: ['@a11y', '@MR-012'] }, async ({ browser }) => {
  await scanLiveSessionScreens(browser, 'light', 1280);
});

test('as telas da sessão ao vivo passam no axe no tema escuro, no celular', { tag: ['@a11y', '@MR-012'] }, async ({ browser }) => {
  await scanLiveSessionScreens(browser, 'dark', 390);
});

/**
 * The maps' screens (Etapa 5, MR-008, MR-009, MR-012, MR-028): "Novo mapa",
 * the editor with a point selected, the player's map with a point's sheet
 * open, the picker dialog, and the session page with the current map and
 * with an image on show, for the master and for the player.
 */
async function scanMapScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  test.setTimeout(120_000);
  const options = { colorScheme, viewport: { width, height: 900 } };
  const master = await browser.newContext({ ...options, storageState: authStatePath('Mestre Teste') });
  const player = await browser.newContext({ ...options, storageState: authStatePath('Jogador Teste') });
  const masterPage = await master.newPage();
  const playerPage = await player.newPage();
  const suffix = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  try {
    await masterPage.goto('/');
    await playerPage.goto('/');
    const table = await tableForMaps(masterPage, playerPage, `Acessibilidade mapas ${Date.now()}`, true);
    campaignId = table.campaignId;
    const worldImage = await uploadImageRPC(masterPage, campaignId, 'Mapa de Mirathel', await canvasPng(masterPage, 1200, 800, 'Mirathel'));
    const towerImage = await uploadImageRPC(masterPage, campaignId, 'Planta da torre', await canvasPng(masterPage, 800, 800, 'Torre', '#5b4834'));
    await uploadImageRPC(masterPage, campaignId, 'Capitão Goblin', await canvasPng(masterPage, 400, 500, 'Capitão Goblin', '#3a3a2a'));
    const tower = await createMapRPC(masterPage, campaignId, 'Torre de Mirathel', towerImage);
    const world = await createMapRPC(masterPage, campaignId, 'Mirathel e arredores', worldImage);
    await revealMapRPC(masterPage, campaignId, tower);
    await revealMapRPC(masterPage, campaignId, world);
    await createPointRPC(masterPage, campaignId, world, { kind: 'BATTLE', name: 'Emboscada na estrada', xBp: 3800, yBp: 6200, revealed: true });
    await createPointRPC(masterPage, campaignId, world, { kind: 'SUBMAP', name: 'Torre de Mirathel', description: 'Uma torre antiga na colina.', xBp: 7100, yBp: 2800, targetMapId: tower, revealed: true });
    await createPointRPC(masterPage, campaignId, world, { kind: 'SCENE', name: 'Ruínas élficas', xBp: 8300, yBp: 7600 });
    await placeTokenRPC(masterPage, campaignId, world, table.characterId, 5200, 5400);
    await placeTokenRPC(masterPage, campaignId, world, table.npcId!, 3700, 6000);

    await open(masterPage, `/campanhas/${campaignId}/mapas/novo`);
    await expectNoSeriousViolations(masterPage, `Novo mapa ${suffix}`);
    await masterPage.getByRole('button', { name: 'Criar mapa' }).click();
    await expect(masterPage.getByText('Dê um nome ao mapa.')).toBeVisible();
    await masterPage.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectNoSeriousViolations(masterPage, `Novo mapa com erros ${suffix}`);

    await open(masterPage, `/campanhas/${campaignId}/mapas/${world}`);
    await expectNoSeriousViolations(masterPage, `Mapa, mestre ${suffix}`);
    if (width >= 768) {
      await masterPage.getByRole('button', { name: 'Ruínas élficas, Cena de RP, escondido' }).click();
      await expect(masterPage.getByRole('heading', { name: 'Ruínas élficas' })).toBeVisible();
      await expectNoSeriousViolations(masterPage, `Editor com um ponto escolhido ${suffix}`);
    }

    await open(playerPage, `/campanhas/${campaignId}/mapas/${world}`);
    await expectNoSeriousViolations(playerPage, `Mapa, jogador ${suffix}`);
    await playerPage.getByRole('button', { name: 'Torre de Mirathel, Submapa' }).first().click();
    await expect(playerPage.getByRole('button', { name: 'Abrir Torre de Mirathel' })).toBeVisible();
    await expectNoSeriousViolations(playerPage, `Mapa, jogador, com a ficha de um ponto ${suffix}`);

    await startSessionRPC(masterPage, campaignId);
    await setCurrentMapRPC(masterPage, campaignId, world);
    await openSessionPage(masterPage, campaignId);
    await expect(masterPage.getByRole('heading', { name: 'Pontos do mapa' })).toBeVisible();
    await expectNoSeriousViolations(masterPage, `Sessão com mapa, mestre ${suffix}`);
    await openSessionPage(playerPage, campaignId);
    await expect(playerPage.getByRole('img', { name: 'Prévia do mapa Mirathel e arredores' })).toBeVisible();
    await expectNoSeriousViolations(playerPage, `Sessão com mapa, jogador ${suffix}`);

    // The picker, then an image on show.
    await masterPage.getByRole('button', { name: 'Mostrar imagem' }).click();
    const dialog = masterPage.getByRole('dialog', { name: 'Mostrar uma imagem aos jogadores' });
    await dialog.getByRole('radio', { name: /Capitão Goblin/ }).click();
    await masterPage.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectNoSeriousViolations(masterPage, `Mostrar imagem, seletor ${suffix}`);
    await dialog.getByRole('button', { name: 'Mostrar aos jogadores' }).click();
    await expect(masterPage.getByRole('button', { name: 'Parar de mostrar' })).toBeVisible();
    await expectNoSeriousViolations(masterPage, `Sessão com imagem à mostra, mestre ${suffix}`);
    await expect(playerPage.getByRole('region', { name: 'O mestre está mostrando' })).toBeVisible();
    await playerPage.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectNoSeriousViolations(playerPage, `Sessão com imagem à mostra, jogador ${suffix}`);
  } finally {
    await endOpenSessionRPC(masterPage, campaignId);
    await master.close();
    await player.close();
  }
}

test('as telas de mapa e da imagem mostrada passam no axe no tema claro, no desktop', { tag: ['@a11y', '@MR-008', '@MR-028'] }, async ({ browser }) => {
  await scanMapScreens(browser, 'light', 1280);
});

test('as telas de mapa e da imagem mostrada passam no axe no tema escuro, no celular', { tag: ['@a11y', '@MR-009', '@MR-028'] }, async ({ browser }) => {
  await scanMapScreens(browser, 'dark', 390);
});
