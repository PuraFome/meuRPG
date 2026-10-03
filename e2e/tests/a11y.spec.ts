import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Browser, type Page } from '@playwright/test';

import { canvasJpeg, newCampaign, uploadThroughPicker } from './gallery-support';
import { saveDocumentRPC, tableWithDocumentParts } from './document-support';
import { expectAligned } from './layout';
import { endOpenSessionRPC, endSessionRPC, openSessionPage, startSessionRPC, tableWithPensantus } from './live-session-support';
import { canvasPng, createMapRPC, createPointRPC, placeTokenRPC, revealMapRPC, setCurrentMapRPC, tableForMaps, uploadImageRPC } from './maps-support';
import { adjustVitalsRPC, beginAttackCombatRPC, combatRPC, getEncounterRPC, passTurnsTo, pensantusCasting, tableForCombat, toren, torenSheet } from './combat-support';
import { authStatePath, callRPC, characterRpcBody, createCharacterRPC, newSignedInContext, pensantus } from './support';

// docs/design.md#como-uma-tela-é-feita: every screen passes axe with no
// serious or critical violation of WCAG 2.1 A and AA, in the light and the
// dark theme (contrast is checked per theme), at desktop and phone widths.
//
// Each test builds its own campaign and NPC through the API, so the screens
// have real content, and reuses the saved sign-in (no /auth/login hit).

const wcag = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'];

/** Scans the page and fails with one readable line per serious or critical
 * violation (the rule, what it means and the first few elements), then runs
 * the layout checks of layout.ts on the same screen: icons in line with
 * their words, nothing over an icon, tiles centred. */
async function expectScreenPasses(page: Page, screen: string): Promise<void> {
  // A dialog still fading in has colours between two states: axe would judge
  // the contrast of a frame nobody stops on (it failed that way once, in the
  // spell dialog). Wait for the transitions that end; a looping one never
  // does.
  await page.waitForFunction(
    () =>
      document
        .getAnimations()
        .every((a) => a.playState !== 'running' || a.effect?.getComputedTiming().iterations === Infinity),
    undefined,
    { timeout: 5_000 },
  );
  const results = await new AxeBuilder({ page }).withTags(wcag).analyze();
  const serious = results.violations
    .filter((v) => v.impact === 'serious' || v.impact === 'critical')
    .map((v) => `${screen}: ${v.id} (${v.impact}) ${v.help}: ${v.nodes.slice(0, 3).map((n) => n.target.join(' ')).join(' | ')}`);
  expect(serious).toEqual([]);
  await expectAligned(page, screen);
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
      await expectScreenPasses(page, `${screen} (${colorScheme}, ${width}px)`);
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
    await expectScreenPasses(page, `Galeria vazia ${where}`);

    await uploadThroughPicker(page, [
      { name: 'Taverna do Javali.jpg', mimeType: 'image/jpeg', buffer: await canvasJpeg(page) },
      { name: 'Covil dos goblins.jpg', mimeType: 'image/jpeg', buffer: await canvasJpeg(page, '#5b4834') },
      { name: 'mapa-antigo.gif', mimeType: 'image/gif', buffer: Buffer.from('GIF89a') },
    ]);
    await expect(page.getByRole('article', { name: 'Covil dos goblins', exact: true })).toBeVisible();
    await expect(page.getByRole('alert')).toBeVisible();
    await expectScreenPasses(page, `Galeria com imagens e um envio recusado ${where}`);

    const card = page.getByRole('article', { name: 'Taverna do Javali', exact: true });
    await card.getByRole('button', { name: 'Apagar Taverna do Javali' }).click();
    await expect(card.getByRole('button', { name: 'Apagar imagem' })).toBeFocused();
    await expectScreenPasses(page, `Galeria, confirmar exclusão ${where}`);
    await card.getByRole('button', { name: 'Cancelar' }).click();

    await card.getByRole('button', { name: 'Ver Taverna do Javali' }).first().click();
    await expect(page.getByRole('dialog', { name: 'Taverna do Javali' })).toBeVisible();
    await expectScreenPasses(page, `Galeria, imagem aberta ${where}`);
    await page.keyboard.press('Escape');

    await open(page, `/campanhas/${campaignId}`);
    await expect(page.getByRole('link', { name: 'Abrir galeria' })).toBeVisible();
    await expectScreenPasses(page, `Campanha com o painel Galeria ${where}`);
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
    // The map dialog draws the points: one revealed, one hidden (E5-29).
    await createPointRPC(page, t.campaignId, t.mapId, { kind: 'BATTLE', name: 'Emboscada na estrada', xBp: 5000, yBp: 6500, revealed: true });
    await createPointRPC(page, t.campaignId, t.mapId, { kind: 'SUBMAP', name: 'Covil dos goblins', xBp: 2500, yBp: 3000 });
    await saveDocumentRPC(
      page,
      t.campaignId,
      `## Arco 1\n\nVeja [Mirathel e arredores](mapa:${t.mapId}) e [Capitão Goblin](ficha:${t.npcId}), com **negrito** e *itálico*.\n\n![Taverna do Javali](imagem:${t.imageId})\n\n## Segredos\n\n- um\n- dois`,
      0,
    );
    await open(page, `/campanhas/${t.campaignId}/documento`);
    await expect(page.getByRole('button', { name: 'Mirathel e arredores' })).toBeVisible();
    await expectScreenPasses(page, `Documento, leitura ${where}`);

    await page.getByRole('button', { name: 'Mirathel e arredores' }).click();
    const dialog = page.getByRole('dialog', { name: 'Mirathel e arredores' });
    await expect(dialog.getByRole('img', { name: 'Prévia do mapa Mirathel e arredores' })).toBeVisible();
    await expect(dialog.locator('.lbl__pill', { hasText: 'Covil dos goblins' })).toBeVisible();
    await expectScreenPasses(page, `Documento, janela do mapa ${where}`);
    await page.keyboard.press('Escape');

    await page.getByRole('button', { name: 'Editar documento' }).click();
    await expect(page.getByRole('textbox', { name: 'Texto' })).toBeVisible();
    await expectScreenPasses(page, `Documento, edição ${where}`);

    await page.getByRole('button', { name: 'Imagem da galeria' }).click();
    await expect(page.getByRole('dialog', { name: 'Imagem da galeria' }).getByRole('radio').first()).toBeVisible();
    await expectScreenPasses(page, `Documento, escolher imagem ${where}`);
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
        await expectScreenPasses(page, `${screen} (${colorScheme}, sem login)`);
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
    await expectScreenPasses(masterPage, `Sessão, mestre ${suffix}`);

    await masterPage.getByRole('button', { name: 'Ajustar Pensantus' }).click();
    await expect(masterPage.getByRole('dialog', { name: 'Ajustar Pensantus' })).toBeVisible();
    // Scan the sheet once it's in place: mid-animation, its text is still
    // fading in, and axe would measure the contrast of a half-drawn frame.
    await masterPage.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(masterPage, `Ajustar PV ${suffix}`);
    await masterPage.getByRole('button', { name: 'Cancelar' }).click();

    await openSessionPage(playerPage, campaignId);
    await expect(playerPage.getByRole('region', { name: 'Pensantus' })).toBeVisible();
    await expectScreenPasses(playerPage, `Sessão, jogador ${suffix}`);

    // A campaign Jogador Teste isn't in: the link says to ask for an invite.
    const closed = await callRPC(masterPage, 'meurpg.campaigns.v1.CampaignService/CreateCampaign', {
      name: `Mesa fechada ${Date.now()}`,
      xpMode: 'XP_MODE_ENEMIES',
    });
    await playerPage.goto(`/campanhas/${(await closed.json()).campaign.id}/sessao`);
    await expect(playerPage.getByRole('heading', { level: 1, name: 'Peça um convite ao mestre' })).toBeVisible();
    await expectScreenPasses(playerPage, `Sessão sem acesso ${suffix}`);

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
    await expectScreenPasses(masterPage, `Novo mapa ${suffix}`);
    await masterPage.getByRole('button', { name: 'Criar mapa' }).click();
    await expect(masterPage.getByText('Dê um nome ao mapa.')).toBeVisible();
    await masterPage.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(masterPage, `Novo mapa com erros ${suffix}`);

    await open(masterPage, `/campanhas/${campaignId}/mapas/${world}`);
    await expectScreenPasses(masterPage, `Mapa, mestre ${suffix}`);
    // E6-27: the header renaming, and asking before deleting.
    await masterPage.getByRole('button', { name: 'Renomear' }).click();
    await expect(masterPage.getByLabel('Nome do mapa')).toBeFocused();
    await expectScreenPasses(masterPage, `Mapa, renomear ${suffix}`);
    await masterPage.getByRole('button', { name: 'Cancelar' }).click();
    await masterPage.getByRole('button', { name: 'Apagar mapa' }).click();
    await expect(masterPage.getByRole('group', { name: /^Apagar / }).getByRole('button', { name: 'Cancelar' })).toBeFocused();
    await expectScreenPasses(masterPage, `Mapa, apagar ${suffix}`);
    await masterPage.getByRole('group', { name: /^Apagar / }).getByRole('button', { name: 'Cancelar' }).click();
    if (width >= 768) {
      await masterPage.getByRole('button', { name: 'Ruínas élficas, Cena de RP, escondido' }).click();
      await expect(masterPage.getByRole('heading', { name: 'Ruínas élficas' })).toBeVisible();
      await expectScreenPasses(masterPage, `Editor com um ponto escolhido ${suffix}`);
    }

    await open(playerPage, `/campanhas/${campaignId}/mapas/${world}`);
    await expectScreenPasses(playerPage, `Mapa, jogador ${suffix}`);
    await playerPage.getByRole('button', { name: 'Torre de Mirathel, Submapa' }).first().click();
    await expect(playerPage.getByRole('button', { name: 'Abrir Torre de Mirathel' })).toBeVisible();
    await expectScreenPasses(playerPage, `Mapa, jogador, com a ficha de um ponto ${suffix}`);

    await startSessionRPC(masterPage, campaignId);
    await setCurrentMapRPC(masterPage, campaignId, world);
    await openSessionPage(masterPage, campaignId);
    await expect(masterPage.getByRole('heading', { name: 'Pontos do mapa' })).toBeVisible();
    await expectScreenPasses(masterPage, `Sessão com mapa, mestre ${suffix}`);
    await openSessionPage(playerPage, campaignId);
    await expect(playerPage.getByRole('img', { name: 'Prévia do mapa Mirathel e arredores' })).toBeVisible();
    await expectScreenPasses(playerPage, `Sessão com mapa, jogador ${suffix}`);

    // The picker, then an image on show.
    await masterPage.getByRole('button', { name: 'Mostrar imagem' }).click();
    const dialog = masterPage.getByRole('dialog', { name: 'Mostrar uma imagem aos jogadores' });
    await dialog.getByRole('radio', { name: /Capitão Goblin/ }).click();
    await masterPage.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(masterPage, `Mostrar imagem, seletor ${suffix}`);
    await dialog.getByRole('button', { name: 'Mostrar aos jogadores' }).click();
    await expect(masterPage.getByRole('button', { name: 'Parar de mostrar' })).toBeVisible();
    await expectScreenPasses(masterPage, `Sessão com imagem à mostra, mestre ${suffix}`);
    await expect(playerPage.getByRole('region', { name: 'O mestre está mostrando' })).toBeVisible();
    await playerPage.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(playerPage, `Sessão com imagem à mostra, jogador ${suffix}`);

    // "Deixar com os jogadores" on, then the image left with the players.
    const keep = masterPage.getByRole('switch', { name: 'Deixar com os jogadores' });
    await keep.click();
    await expect(keep).toHaveAttribute('aria-checked', 'true');
    await expectScreenPasses(masterPage, `Sessão com "Deixar com os jogadores" ligado, mestre ${suffix}`);
    await masterPage.getByRole('button', { name: 'Parar de mostrar' }).click();
    await expect(masterPage.getByRole('button', { name: 'Tirar Capitão Goblin dos jogadores' })).toBeVisible();
    await expectScreenPasses(masterPage, `Sessão com uma imagem deixada, mestre ${suffix}`);
    await expect(playerPage.getByRole('region', { name: 'Imagens que o mestre deixou' })).toBeVisible();
    await playerPage.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(playerPage, `Sessão com uma imagem deixada, jogador ${suffix}`);
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

/** The dice settings (RN-18): the master's "Dados" panel and the player's
 * "Como você rola os dados", as a choice and locked (the master decided). */
async function scanDiceScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: 900 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const masterPage = await master.newPage();
  const playerPage = await player.newPage();
  try {
    await masterPage.goto('/');
    await playerPage.goto('/');
    const { campaignId } = await tableWithPensantus(masterPage, playerPage, `Acessibilidade dados ${Date.now()}`);
    const suffix = `(${colorScheme}, ${width}px)`;
    await open(masterPage, `/campanhas/${campaignId}`);
    await expectScreenPasses(masterPage, `Campanha com Dados, mestre ${suffix}`);
    await open(playerPage, `/campanhas/${campaignId}`);
    await expectScreenPasses(playerPage, `Campanha com Como você rola os dados, jogador ${suffix}`);

    const set = await callRPC(masterPage, 'meurpg.campaigns.v1.CampaignService/SetCampaignDiceMode', { campaignId, mode: 'DICE_MODE_APP' });
    expect(set.ok()).toBeTruthy();
    await open(masterPage, `/campanhas/${campaignId}`);
    await expectScreenPasses(masterPage, `Campanha com Dados, todos no app, mestre ${suffix}`);
    await open(playerPage, `/campanhas/${campaignId}`);
    await expectScreenPasses(playerPage, `Como você rola os dados, decidido pelo mestre ${suffix}`);
  } finally {
    await master.close();
    await player.close();
  }
}

test('as configurações de dados passam no axe no tema claro, no desktop', { tag: ['@a11y', '@RN-18'] }, async ({ browser }) => {
  await scanDiceScreens(browser, 'light', 1280);
});

test('as configurações de dados passam no axe no tema escuro, no celular', { tag: ['@a11y', '@RN-18'] }, async ({ browser }) => {
  await scanDiceScreens(browser, 'dark', 390);
});

/** The campaign page with someone waiting to create a character (MR-024):
 * the "Membros" row with its tag, and the removal confirmation open. */
async function scanPendingMembers(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const context = await browser.newContext({
    storageState: authStatePath('Mestre Teste'),
    colorScheme,
    viewport: { width, height: 900 },
  });
  const playerContext = await newSignedInContext(browser, 'Jogador Teste');
  const page = await context.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  try {
    await page.goto('/');
    const created = await callRPC(page, 'meurpg.campaigns.v1.CampaignService/CreateCampaign', {
      name: `Acessibilidade esperando ${Date.now()}`,
      xpMode: 'XP_MODE_ENEMIES',
    });
    const campaignId = (await created.json()).campaign.id as string;
    const invite = await callRPC(page, 'meurpg.campaigns.v1.CampaignService/CreateInvite', {
      campaignId,
      maxUses: 1,
      expiresIn: '86400s',
      requiresApproval: true,
    });
    const { token } = await invite.json();
    const playerPage = await playerContext.newPage();
    await playerPage.goto('/');
    const accepted = await callRPC(playerPage, 'meurpg.campaigns.v1.CampaignService/AcceptInvite', { token });
    expect(accepted.ok()).toBeTruthy();

    await open(page, `/campanhas/${campaignId}`);
    await expect(page.getByRole('list', { name: 'Esperando para criar o personagem' })).toBeVisible();
    await expectScreenPasses(page, `Campanha com alguém sem personagem ${where}`);
    await page.getByRole('button', { name: /^Remover .* da campanha$/ }).click();
    await expect(page.getByRole('alertdialog')).toBeVisible();
    await expectScreenPasses(page, `Campanha, confirmar a remoção ${where}`);
  } finally {
    await playerContext.close();
    await context.close();
  }
}

test('quem está sem personagem passa no axe no tema claro, no desktop', { tag: ['@a11y', '@MR-024'] }, async ({ browser }) => {
  await scanPendingMembers(browser, 'light', 1280);
});

test('quem está sem personagem passa no axe no tema escuro, no celular', { tag: ['@a11y', '@MR-024'] }, async ({ browser }) => {
  await scanPendingMembers(browser, 'dark', 390);
});

/**
 * The character editor's rolls and the spell "?" (MR-004, E6-20 to E6-23):
 * the "Atributos" step with "Rolar 4d6" (half placed, and on the phone with a
 * result chosen), the rolled hit points, the "Magias" step with its search
 * fields, and the spell dialog (a bottom sheet on the phone).
 */
async function scanEditorRolls(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  test.setTimeout(90_000);
  const context = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport: { width, height: 900 } });
  const page = await context.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  try {
    await page.goto('/');
    const created = await callRPC(page, 'meurpg.campaigns.v1.CampaignService/CreateCampaign', { name: `Acessibilidade rolagens ${Date.now()}`, xpMode: 'XP_MODE_ENEMIES' });
    expect(created.ok()).toBeTruthy();
    const campaignId = (await created.json()).campaign.id as string;
    await open(page, `/campanhas/${campaignId}/personagens/novo`);
    await page.getByLabel('Nome do personagem', { exact: true }).fill('Zézinho');
    const classSelect = page.getByRole('combobox', { name: 'Classe', exact: true });
    await classSelect.focus();
    await classSelect.press('Enter');
    await page.getByRole('option', { name: 'Mago', exact: true }).click();
    await page.getByLabel('Nível', { exact: true }).fill('3');

    await page.getByRole('tab', { name: 'Atributos' }).click();
    await page.getByRole('radio', { name: /Rolar 4d6/ }).check();
    if (width < 768) {
      await page.getByRole('button', { name: /^\d+: dados .* Livre\.$/ }).first().click();
      await page.getByRole('button', { name: /^Força: colocar o/ }).click();
      await page.getByRole('button', { name: /^\d+: dados .* Livre\.$/ }).first().click();
    } else {
      await page.getByLabel('Força', { exact: true }).selectOption({ index: 1 });
    }
    await expectScreenPasses(page, `Atributos, Rolar 4d6 ${where}`);

    await page.getByRole('radio', { name: /Rolado/ }).check();
    await page.getByRole('button', { name: 'Rolar os níveis que faltam' }).click();
    await expectScreenPasses(page, `Pontos de vida rolados ${where}`);

    await page.getByRole('tab', { name: 'Magias' }).click();
    await expectScreenPasses(page, `Magias ${where}`);
    await page.getByRole('group', { name: 'Magias conhecidas', exact: true }).getByRole('button', { name: 'Descrição de Mísseis Mágicos' }).click();
    await expect(page.getByText('Texto do SRD 5.1 (em inglês)')).toBeVisible();
    await expectScreenPasses(page, `Descrição da magia ${where}`);
  } finally {
    await context.close();
  }
}

test('as rolagens e a descrição da magia passam no axe no tema claro, no desktop', { tag: ['@a11y', '@MR-004'] }, async ({ browser }) => {
  await scanEditorRolls(browser, 'light', 1280);
});

test('as rolagens e a descrição da magia passam no axe no tema escuro, no celular', { tag: ['@a11y', '@MR-004'] }, async ({ browser }) => {
  await scanEditorRolls(browser, 'dark', 390);
});

/** The combat (MR-013, E6-01 to E6-16): the grid page, the start dialog, the
 * initiative of the master and of the player, the running combat for each, the
 * "Mover" page in each of its states, the end confirmation and the summary.
 * The master and the player each have a page at `width`; the NPCs' initiative is
 * set through the API so the screens are the same on every run. */
async function scanCombatScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: 900 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForCombat(m, p, `Acessibilidade combate ${Date.now()}`, false);
    campaignId = table.campaignId;

    await open(m, `/campanhas/${campaignId}/mapas/${table.mapId}/grade?de=sessao`);
    await expectScreenPasses(m, `Grade do mapa sem grade ${where}`);
    await m.getByRole('button', { name: 'Salvar grade' }).click();
    await expect(m).toHaveURL(/\/sessao$/);

    await expect(m.getByRole('button', { name: 'Iniciar combate' })).toBeVisible();
    await expectScreenPasses(m, `Sessão com o convite ao combate ${where}`);
    await m.getByRole('button', { name: 'Iniciar combate' }).click();
    await expect(m.getByRole('dialog', { name: 'Iniciar combate' })).toBeVisible();
    for (let i = 0; i < 3; i++) {
      await m.getByRole('button', { name: 'Mais um Goblin', exact: true }).click();
    }
    await m.getByRole('button', { name: 'Mais um Capitão Goblin' }).click();
    await expectScreenPasses(m, `Iniciar combate ${where}`);
    await m.getByRole('dialog').getByRole('button', { name: 'Iniciar combate' }).click();
    await expect(m.getByText('Os NPCs rolaram sozinhos.')).toBeVisible();

    // The player before rolling, then the master's initiative with a tie and a missing roll.
    await openSessionPage(p, campaignId);
    await expect(p.getByRole('heading', { name: 'Role a iniciativa' })).toBeVisible();
    await expectScreenPasses(p, `Iniciativa do jogador, antes de rolar ${where}`);
    let enc = await getEncounterRPC(m, campaignId);
    const id = (label: string) => enc.combatants.find((c) => c.label === label)!.id;
    const face = async (label: string, d20Face: number) => {
      enc = await combatRPC(m, 'SubmitInitiative', { campaignId, encounterId: enc.id, combatantId: id(label), d20Face });
    };
    await face('Capitão Goblin', 20);
    await face('Goblin 1', 7);
    await face('Goblin 2', 7);
    await face('Goblin 3', 3);
    await expect(m.getByText('Falta a iniciativa de Pensantus.').first()).toBeVisible();
    await expectScreenPasses(m, `Iniciativa do mestre, com empate e rolagem faltando ${where}`);
    await m.getByRole('button', { name: 'Digitar pelo jogador' }).click();
    await m.getByLabel(/Resultado do d20 de Pensantus/).fill('19');
    await expectScreenPasses(m, `Iniciativa do mestre, editando ${where}`);
    await m.getByRole('button', { name: 'Salvar', exact: true }).click();
    await expect(p.getByText('Esperando o mestre começar o combate')).toBeVisible();
    await expectScreenPasses(p, `Iniciativa do jogador, depois de rolar ${where}`);
    for (const [label, col, row] of [['Goblin 1', 9, 9], ['Goblin 2', 14, 10], ['Goblin 3', 15, 4], ['Capitão Goblin', 11, 5]] as const) {
      enc = await combatRPC(m, 'MoveCombatant', { campaignId, encounterId: enc.id, combatantId: id(label), col, row });
    }

    // The combat runs: the master's screen, the player out of turn (the captain is hidden: "Vez do mestre").
    await m.getByRole('button', { name: 'Começar o combate' }).click();
    await expect(m.getByRole('button', { name: 'Próximo turno' })).toBeVisible();
    await expect(m.getByText('Vez do Capitão Goblin')).toBeVisible();
    await expectScreenPasses(m, `Combate do mestre ${where}`);
    await expect(p.getByRole('heading', { name: 'Vez do mestre' })).toBeVisible();
    await expectScreenPasses(p, `Combate do jogador, vez do mestre ${where}`);
    enc = await getEncounterRPC(m, campaignId);
    await combatRPC(m, 'SetCombatantHidden', { campaignId, encounterId: enc.id, combatantId: id('Capitão Goblin'), hidden: false });
    await expect(p.getByRole('heading', { name: 'Vez do Capitão Goblin' })).toBeVisible();
    await expectScreenPasses(p, `Combate do jogador, fora da vez ${where}`);

    // The player's turn and the "Mover" page.
    await m.getByRole('button', { name: 'Próximo turno' }).click();
    await expect(p.getByRole('heading', { name: 'Sua vez, Pensantus' })).toBeVisible();
    await expectScreenPasses(p, `Combate do jogador, sua vez ${where}`);
    await p.getByRole('button', { name: 'Mover' }).click();
    await expect(p.getByRole('heading', { name: 'Mover Pensantus' })).toBeVisible();
    await expectScreenPasses(p, `Mover, nada escolhido ${where}`);
    const map = p.getByRole('group', { name: /Mapa de batalha/ });
    const box = (await map.boundingBox())!;
    const own = (await getEncounterRPC(p, campaignId)).combatants.find((c) => c.mine)!;
    const at = (dc: number, dr: number) => ({ x: ((own.col ?? 0) + dc + 0.5) * (box.width / 20), y: ((own.row ?? 0) + dr + 0.5) * (box.height / 14) });
    await map.click({ position: at(2, 1) });
    await expect(p.getByText('Mover 3 m')).toBeVisible();
    await expectScreenPasses(p, `Mover, quadrado escolhido ${where}`);
    await map.click({ position: at(8, 1) });
    await expect(p.getByText('Longe demais: faltam')).toBeVisible();
    await expectScreenPasses(p, `Mover, longe demais ${where}`);
    await p.getByRole('button', { name: 'Cancelar' }).click();

    // The end: the master's confirmation in place, then the summary for both.
    await m.getByRole('button', { name: 'Encerrar combate' }).click();
    await expect(m.getByRole('alertdialog', { name: 'Encerrar o combate?' })).toBeVisible();
    await expectScreenPasses(m, `Encerrar o combate, confirmação ${where}`);
    await m.getByRole('button', { name: 'Encerrar combate' }).last().click();
    await expect(m.getByRole('heading', { name: 'Combate encerrado' })).toBeVisible();
    await expect(p.getByRole('heading', { name: 'Combate encerrado' })).toBeVisible();
    await expectScreenPasses(m, `Combate encerrado, mestre ${where}`);
    await expectScreenPasses(p, `Combate encerrado, jogador ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
}

test('o combate passa no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-013'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanCombatScreens(browser, 'light', 1280);
});

test('o combate passa no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-013'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanCombatScreens(browser, 'dark', 390);
});

/** Acting in a combat (Etapa 6, slice 6.5b; E6-06, E6-07, E6-08, E6-15, E6-11):
 * the player's "Sua vez" groups, the end-turn question, every step of the
 * attack sheet (typed roll empty, wrong, valid; the result), the log sheet on a
 * phone, and the master's card with the armor class, "Aplicar" and its
 * questions, "Desfazer última ação" and "Dano/Cura". */
async function scanActionScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: 900 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForCombat(m, p, `Acessibilidade ações ${Date.now()}`, true, true);
    campaignId = table.campaignId;
    await beginAttackCombatRPC(m, table, { Pensantus: 20, 'Capitão Goblin': 15, 'Goblin 1': 5, 'Goblin 2': 4 });

    // The player's turn: the groups, the question when the action is free, and the attack.
    await openSessionPage(p, campaignId);
    await expect(p.getByRole('button', { name: 'Atacar com Raio de Fogo' })).toBeVisible();
    await expectScreenPasses(p, `Sua vez, com os grupos de ações ${where}`);
    await p.getByRole('button', { name: 'Encerrar turno' }).last().click();
    await expect(p.getByText('Ainda tem ação disponível. Encerrar mesmo?')).toBeVisible();
    await expectScreenPasses(p, `Encerrar turno com a ação livre ${where}`);
    await p.getByRole('button', { name: 'Voltar' }).click();

    await p.getByRole('button', { name: 'Atacar com Raio de Fogo' }).click();
    await expect(p.getByRole('heading', { name: 'Atacar com Raio de Fogo' })).toBeVisible();
    await expectScreenPasses(p, `Atacar, escolher o alvo ${where}`);
    await p.locator('label', { hasText: 'Goblin 1' }).click();
    await expect(p.getByRole('button', { name: 'Digitar o resultado' })).toBeVisible();
    await expectScreenPasses(p, `Atacar, rolar ${where}`);
    await p.getByRole('button', { name: 'Digitar o resultado' }).click();
    await expectScreenPasses(p, `Rolagem física, vazia ${where}`);
    await p.getByLabel(/Role 1d20/).fill('27');
    await expect(p.getByRole('alert').filter({ hasText: 'Digite um número de 1 a 20' })).toBeVisible();
    await expectScreenPasses(p, `Rolagem física, número fora de 1 a 20 ${where}`);
    await p.getByLabel(/Role 1d20/).fill('16');
    await expect(p.getByText('22 · dado físico').or(p.getByText('16 + 6 = 22 · dado físico'))).toBeVisible();
    await expectScreenPasses(p, `Rolagem física, número válido ${where}`);
    await p.getByRole('button', { name: 'Confirmar 16' }).click();
    await expect(p.locator('.pill', { hasText: 'Acertou' })).toBeVisible();
    await expectScreenPasses(p, `Atacar, o d20 e o dano a rolar ${where}`);
    await p.getByRole('button', { name: 'Digitar o resultado' }).click();
    await p.getByLabel(/Role 1d10/).fill('9');
    await p.getByRole('button', { name: 'Confirmar 9' }).click();
    await expect(p.getByRole('button', { name: 'Voltar à sua vez' })).toBeFocused();
    await expectScreenPasses(p, `Atacar, resultado ${where}`);
    await p.getByRole('button', { name: 'Voltar à sua vez' }).click();
    await expectScreenPasses(p, `Sua vez, depois de atacar ${where}`);
    if (width < 768) {
      await p.getByRole('button', { name: 'Abrir o registro do combate' }).click();
      await expect(p.getByRole('log', { name: 'Registro do combate' })).toBeVisible();
      await expectScreenPasses(p, `Registro do combate, folha ${where}`);
      await p.keyboard.press('Escape');
    }

    // The master: the captain's card, the roll with the armor class, the damage and the questions.
    await p.getByRole('button', { name: 'Encerrar turno' }).last().click();
    await openSessionPage(m, campaignId);
    await expect(m.getByRole('heading', { name: /Ações do Capitão Goblin|Vez do Capitão Goblin/ })).toBeVisible();
    await expectScreenPasses(m, `Cartão do mestre, antes de rolar ${where}`);
    await m.getByRole('button', { name: 'Digitar o resultado' }).click();
    await m.getByLabel(/Role 1d20/).fill('18');
    await m.getByRole('button', { name: 'Confirmar 18' }).click();
    await expect(m.getByText(/contra CA \d+ da Pensantus|contra CA \d+ do Pensantus/)).toBeVisible();
    // Pensantus can cast Escudo: a hit that is not critical waits for his reaction (E6-28b).
    await expect(m.getByText('Esperando a reação do Pensantus.')).toBeVisible();
    await expect(m.getByRole('button', { name: 'Rolar dano' })).toHaveAttribute('aria-disabled', 'true');
    await expectScreenPasses(m, `Cartão do mestre, esperando a reação (Escudo) ${where}`);
    await m.getByRole('button', { name: 'Seguir sem Escudo' }).click();
    await m.getByRole('button', { name: 'Rolar dano' }).click();
    await expect(m.getByRole('button', { name: /Aplicar \d+ de dano/ })).toBeVisible();
    await expectScreenPasses(m, `Cartão do mestre, dano para aplicar ${where}`);
    await m.getByRole('button', { name: 'Não aplicar' }).click();
    await expect(m.getByText(/Descartar o dano de \d+\?/)).toBeVisible();
    await expect(m.getByRole('button', { name: 'Voltar' })).toBeFocused();
    await expectScreenPasses(m, `Cartão do mestre, descartar o dano ${where}`);
    await m.getByRole('button', { name: 'Voltar' }).click();
    await m.getByRole('button', { name: 'Próximo turno' }).click();
    await expect(m.getByText('Há dano sem aplicar. Passar o turno mesmo assim?')).toBeVisible();
    await expectScreenPasses(m, `Próximo turno com dano sem aplicar ${where}`);
    await m.getByRole('button', { name: 'Voltar' }).click();
    await m.getByRole('button', { name: /Aplicar \d+ de dano/ }).click();
    await expect(m.getByText(/Dano de \d+ aplicado\./).first()).toBeVisible();
    await m.getByRole('button', { name: 'Desfazer última ação' }).first().click();
    await expect(m.getByText(/Desfazer o ataque do Capitão Goblin/)).toBeVisible();
    await expectScreenPasses(m, `Desfazer a última ação ${where}`);
    await m.getByRole('button', { name: 'Voltar' }).click();
    await m.getByRole('button', { name: 'Dano ou cura em Goblin 1' }).click();
    await expect(m.getByRole('heading', { name: 'Dano ou cura em Goblin 1' })).toBeVisible();
    await expectScreenPasses(m, `Dano/Cura de um NPC ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
}

test('agir no combate passa no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-014'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanActionScreens(browser, 'light', 1280);
});

test('agir no combate passa no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-014'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanActionScreens(browser, 'dark', 390);
});

/** Casting, the fallen, Escudo, conditions and the master's other amount (Etapa 6,
 * slice 6.5c; E6-09, E6-13, E6-28 to E6-31): every step of the cast sheet, the
 * Escudo Arcano prompt and its answer, the conditions dialog with its tags, the
 * player's concentration line, the opportunity-attack sheet, "Aplicar outro
 * valor" with the concentration reminder, the death saves (stable, then three
 * failures) and the question that confirms a death. */
async function scanCastingScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: 900 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForCombat(m, p, `Acessibilidade conjurar ${Date.now()}`, true, true, { sheet: pensantusCasting });
    campaignId = table.campaignId;
    await beginAttackCombatRPC(m, table, { Pensantus: 20, 'Capitão Goblin': 15, 'Goblin 1': 5, 'Goblin 2': 4 });
    await adjustVitalsRPC(m, campaignId, table.characterId, { spellSlotsUsed: [{ level: 1, used: 3 }, { level: 2, used: 2 }] });
    // Pensantus stands next to Goblin 1, so the dagger reaches it for an opportunity attack.
    const begun = await getEncounterRPC(m, campaignId);
    await combatRPC(m, 'MoveCombatant', { campaignId, encounterId: begun.id, combatantId: begun.combatants.find((c) => c.label === 'Pensantus')!.id, col: 8, row: 9 });
    await openSessionPage(m, campaignId);
    await openSessionPage(p, campaignId);

    // The cast sheet: the slot and the darts, then the result and the damage to roll.
    await p.getByRole('button', { name: 'Conjurar Mísseis Mágicos' }).click();
    await expect(p.getByText('É o seu último espaço de 1º círculo: depois dele, o Escudo Arcano fica sem espaço.')).toBeVisible();
    await expectScreenPasses(p, `Conjurar, o espaço e os dardos ${where}`);
    const more = (who: string) => p.getByRole('button', { name: `Pôr um dardo em ${who}` });
    await more('Capitão Goblin').click();
    await more('Capitão Goblin').click();
    await more('Goblin 1').click();
    await expect(p.getByText('3 de 3 dardos distribuídos.')).toBeVisible();
    await expectScreenPasses(p, `Conjurar, os três dardos distribuídos ${where}`);
    await p.getByRole('dialog').getByRole('button', { name: 'Conjurar Mísseis Mágicos' }).click();
    await expect(p.getByText('Falta rolar o dano.').first()).toBeVisible();
    await expectScreenPasses(p, `Conjurar, resultado com dano a rolar ${where}`);
    await p.getByRole('button', { name: 'Digitar o resultado' }).click();
    await p.getByLabel(/Role 2d4/).fill('9');
    await expect(p.getByRole('alert').filter({ hasText: 'Digite um número de 2 a 8' })).toBeVisible();
    await expect(p.getByRole('button', { name: /Confirmar/ })).toBeInViewport({ ratio: 1 });
    await expect(p.getByLabel(/Role 2d4/)).toBeInViewport({ ratio: 1 });
    await expectScreenPasses(p, `Conjurar, rolagem do dano inválida ${where}`);
    await p.getByLabel(/Role 2d4/).fill('4');
    await p.getByRole('button', { name: 'Confirmar 4' }).click();
    await p.getByRole('button', { name: 'Digitar o resultado' }).click();
    await p.getByLabel(/Role 1d4/).fill('2');
    await p.getByRole('button', { name: 'Confirmar 2' }).click();
    await expect(p.getByRole('button', { name: 'Voltar à sua vez' })).toBeFocused();
    await expectScreenPasses(p, `Conjurar, resultado final ${where}`);
    await p.getByRole('button', { name: 'Voltar à sua vez' }).click();

    // The conditions: the dialog (the master) and the tags on both screens.
    await m.getByRole('button', { name: 'Mais ações para Goblin 1' }).click();
    await m.getByRole('menuitem', { name: 'Condições…' }).click();
    await expect(m.getByRole('dialog', { name: 'Condições de Goblin 1' })).toBeVisible();
    await expectScreenPasses(m, `Condições, a janela ${where}`);
    await m.getByRole('checkbox', { name: 'Envenenado' }).check();
    await m.getByRole('checkbox', { name: 'Derrubado' }).check();
    await m.getByRole('button', { name: 'Salvar condições' }).click();
    await expect(m.getByRole('list', { name: 'Condições de Goblin 1' })).toBeVisible();
    await expectScreenPasses(m, `Condições, as etiquetas na ordem do mestre ${where}`);

    // Escudo Arcano: the prompt, its answer, and the master's card meanwhile (the slots are given back: the cast took the last one).
    await adjustVitalsRPC(m, campaignId, table.characterId, { spellSlotsUsed: [{ level: 1, used: 2 }, { level: 2, used: 0 }] });
    await p.getByRole('button', { name: 'Encerrar turno' }).last().click();
    const card = m.getByRole('region', { name: /Ações do Capitão Goblin|Vez do Capitão Goblin/ });
    await card.getByRole('button', { name: 'Digitar o resultado' }).click();
    await card.getByLabel(/Role 1d20 para Cimitarra/).fill('11');
    await card.getByRole('button', { name: 'Confirmar 11' }).click();
    const prompt = p.getByRole('alertdialog', { name: 'Você foi atingido: usar Escudo Arcano?' });
    await expect(prompt).toBeVisible();
    await expect(prompt.getByText('Capitão Goblin · Cimitarra · Rodada 1')).toBeVisible();
    await expectScreenPasses(p, `Escudo, o aviso ${where}`);
    await expectScreenPasses(m, `Escudo, o cartão do mestre esperando ${where}`);
    await prompt.getByRole('button', { name: 'Conjurar Escudo Arcano' }).click();
    await expect(prompt.getByText('O Escudo Arcano segurou o ataque do Capitão Goblin.')).toBeVisible();
    await expectScreenPasses(p, `Escudo, o resultado ${where}`);
    await prompt.getByRole('button', { name: 'Fechar' }).click();
    // The captain's turn goes on: a second hit (a critical one) is damage to roll and discard; then Pensantus's turn.
    await card.getByRole('button', { name: 'Digitar o resultado' }).click();
    await card.getByLabel(/Role 1d20 para Cimitarra/).fill('20');
    await card.getByRole('button', { name: 'Confirmar 20' }).click();
    await card.getByRole('button', { name: 'Rolar dano' }).click();
    await card.getByRole('button', { name: 'Não aplicar' }).click();
    await card.getByRole('button', { name: 'Descartar' }).click();
    await passTurnsTo(m, campaignId, 'Pensantus');

    // Round 2: Pensantus casts Teia (through the API) and concentrates; the line and its action are his.
    const enc2 = await getEncounterRPC(m, campaignId);
    const cast = await callRPC(p, 'meurpg.play.v1.CombatService/CastSpell', {
      campaignId,
      encounterId: enc2.id,
      casterId: enc2.combatants.find((c) => c.label === 'Pensantus')!.id,
      spellKey: 'spell:web',
      slot: { level: 2 },
      targets: [],
      idempotencyKey: crypto.randomUUID(),
    });
    expect(cast.ok(), await cast.text()).toBeTruthy();
    await expect(p.getByText('Concentrado em Teia', { exact: true })).toBeVisible();
    await expect(p.getByRole('button', { name: 'Encerrar concentração' })).toBeVisible();
    await expectScreenPasses(p, `Concentrado, a linha da vez ${where}`);
    await expectScreenPasses(m, `Concentrado, a ordem do mestre ${where}`);
    // Teia used the action, so "Encerrar turno" ends the turn without asking.
    await p.getByRole('button', { name: 'Encerrar turno' }).last().click();
    await passTurnsTo(m, campaignId, 'Capitão Goblin');

    // Off turn: "Ataque de oportunidade" opens the attack sheet with the dagger (a melee weapon).
    await expect(p.getByRole('button', { name: /Ataque de oportunidade/ })).toBeVisible();
    await expectScreenPasses(p, `Sua reação, com o ataque de oportunidade ${where}`);
    await p.getByRole('button', { name: /Ataque de oportunidade/ }).click();
    await expect(p.getByRole('dialog', { name: /Adaga/ })).toBeVisible();
    await expectScreenPasses(p, `Ataque de oportunidade, escolher o alvo ${where}`);
    await p.getByRole('dialog').getByRole('button', { name: 'Fechar' }).click();

    // Another amount, with the concentration reminder (Pensantus concentrates on Teia).
    const card2 = m.getByRole('region', { name: /Ações do Capitão Goblin|Vez do Capitão Goblin/ });
    await card2.getByRole('button', { name: 'Digitar o resultado' }).click();
    await card2.getByLabel(/Role 1d20 para Cimitarra/).fill('20');
    await card2.getByRole('button', { name: 'Confirmar 20' }).click();
    await card2.getByRole('button', { name: 'Rolar dano' }).click();
    await card2.getByRole('button', { name: 'Aplicar outro valor' }).click();
    await expectScreenPasses(m, `Aplicar outro valor ${where}`);
    await card2.getByLabel('Dano a aplicar').fill('1');
    await card2.getByRole('button', { name: 'Aplicar 1 de dano' }).click();
    await expect(card2.getByText(/1 de dano aplicado/).first()).toBeVisible();
    await expect(card2.getByText('Pensantus está concentrado em Teia. Teste de Constituição, CD 10.')).toBeVisible();
    await expectScreenPasses(m, `Aplicar outro valor, o lembrete da concentração ${where}`);

    // The fallen, first stable: three successes.
    await adjustVitalsRPC(m, campaignId, table.characterId, { hitPointsCurrent: 0 });
    for (let i = 0; i < 3; i++) {
      await passTurnsTo(m, campaignId, 'Pensantus');
      await expect(p.getByRole('heading', { name: 'Pensantus está caído' })).toBeVisible();
      if (i === 0) {
        await expectScreenPasses(p, `Caído, antes de rolar ${where}`);
      }
      await p.getByRole('button', { name: 'Digitar o resultado' }).click();
      await p.getByLabel(/Role 1d20 para o teste contra a morte/).fill('15');
      if (i === 0) {
        await expectScreenPasses(p, `Caído, rolagem física ${where}`);
      }
      await p.getByRole('button', { name: 'Confirmar 15' }).click();
      await expect(p.getByRole('status').filter({ hasText: 'Teste contra a morte' })).toBeVisible();
      if (i === 0) {
        await expectScreenPasses(p, `Caído, depois de rolar ${where}`);
      }
      await p.getByRole('button', { name: 'Encerrar turno' }).click();
    }
    await expect(p.getByText('Pensantus está estável.')).toBeVisible();
    await expectScreenPasses(p, `Estável, fora da vez ${where}`);
    await passTurnsTo(m, campaignId, 'Pensantus');
    await expect(p.getByText(/Estável: não rola mais testes contra a morte/)).toBeVisible();
    await expectScreenPasses(p, `Estável, na vez ${where}`);

    // Healed and down again: now three failures, and the master's question.
    await adjustVitalsRPC(m, campaignId, table.characterId, { hitPointsCurrent: 5 });
    await adjustVitalsRPC(m, campaignId, table.characterId, { hitPointsCurrent: 0 });
    for (let i = 0; i < 2; i++) {
      await passTurnsTo(m, campaignId, 'Capitão Goblin');
      await passTurnsTo(m, campaignId, 'Pensantus');
      await p.getByRole('button', { name: 'Digitar o resultado' }).click();
      await p.getByLabel(/Role 1d20 para o teste contra a morte/).fill(i === 0 ? '1' : '2');
      await p.getByRole('button', { name: i === 0 ? 'Confirmar 1' : 'Confirmar 2' }).click();
      await p.getByRole('button', { name: 'Encerrar turno' }).click();
    }
    await expect(m.getByRole('alertdialog', { name: /Confirmar a morte/ })).toBeVisible();
    await expect(m.getByRole('alertdialog', { name: /Confirmar a morte/ })).toBeInViewport({ ratio: 1 });
    await expectScreenPasses(m, `Confirmar a morte ${where}`);
    await expectScreenPasses(p, `Caído, três falhas, para o jogador ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
}

test('conjurar, cair, o Escudo e as condições passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-014'] }, async ({ browser }) => {
  test.setTimeout(400_000);
  await scanCastingScreens(browser, 'light', 1280);
});

test('conjurar, cair, o Escudo e as condições passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-014'] }, async ({ browser }) => {
  test.setTimeout(400_000);
  await scanCastingScreens(browser, 'dark', 390);
});

/** The fighter's turn (Etapa 6, slice 6.5c): Extra Attack's "1 ataque restante", Retomar o
 * Fôlego's sheet and Surto de Ação's note. */
async function scanFighterScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: 900 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForCombat(m, p, `Acessibilidade guerreiro ${Date.now()}`, true, true, { build: toren, sheet: torenSheet });
    campaignId = table.campaignId;
    await beginAttackCombatRPC(m, table, { Toren: 20, 'Capitão Goblin': 15, 'Goblin 1': 5, 'Goblin 2': 4 });
    await adjustVitalsRPC(m, campaignId, table.characterId, { hitPointsCurrent: 20 });
    const start = await getEncounterRPC(m, campaignId);
    await combatRPC(m, 'MoveCombatant', { campaignId, encounterId: start.id, combatantId: start.combatants.find((c) => c.label === 'Toren')!.id, col: 8, row: 9 });
    await openSessionPage(p, campaignId);
    const groups = p.getByRole('region', { name: 'O que você pode fazer' });
    await expectScreenPasses(p, `Guerreiro, a vez com as habilidades ${where}`);
    await groups.getByRole('button', { name: /^Atacar com Espada/ }).click();
    const sheet = p.getByRole('dialog', { name: /Atacar com Espada/ });
    await sheet.locator('label', { hasText: 'Goblin 1' }).click();
    await p.getByRole('button', { name: 'Digitar o resultado' }).click();
    await p.getByLabel(/Role 1d20/).fill('1');
    await p.getByRole('button', { name: 'Confirmar 1' }).click();
    await expect(p.getByText('Você ainda tem 1 ataque desta ação.')).toBeVisible();
    await p.getByRole('button', { name: 'Voltar à sua vez' }).click();
    await expect(groups.getByText('1 ataque restante').first()).toBeVisible();
    await expectScreenPasses(p, `Ataque Extra, um ataque restante ${where}`);
    await groups.getByRole('button', { name: 'Usar Retomar o Fôlego' }).click();
    await expectScreenPasses(p, `Retomar o Fôlego, antes de rolar ${where}`);
    await p.getByRole('button', { name: 'Digitar o resultado' }).click();
    await p.getByLabel(/Role 1d10/).fill('7');
    await expectScreenPasses(p, `Retomar o Fôlego, rolagem física ${where}`);
    await p.getByRole('button', { name: 'Confirmar 7' }).click();
    await expect(p.getByText(/\d+ PV recuperados/)).toBeVisible();
    await expectScreenPasses(p, `Retomar o Fôlego, resultado ${where}`);
    await p.getByRole('button', { name: 'Voltar à sua vez' }).click();
    await groups.getByRole('button', { name: 'Usar Surto de Ação' }).click();
    await expect(groups.getByText('Surto de Ação: você tem outra ação.')).toBeVisible();
    await expectScreenPasses(p, `Surto de Ação, a nota ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
}

test('o guerreiro (Ataque Extra, Retomar o Fôlego, Surto de Ação) passa no axe e nas conferências de layout no tema escuro, no desktop', { tag: ['@a11y', '@MR-014'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanFighterScreens(browser, 'dark', 1280);
});

test('o guerreiro (Ataque Extra, Retomar o Fôlego, Surto de Ação) passa no axe e nas conferências de layout no tema claro, no celular', { tag: ['@a11y', '@MR-014'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanFighterScreens(browser, 'light', 390);
});
