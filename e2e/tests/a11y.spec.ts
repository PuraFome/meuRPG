import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Browser, type Page } from '@playwright/test';

import { canvasJpeg, newCampaign, uploadThroughPicker } from './gallery-support';
import { saveDocumentRPC, tableWithDocumentParts } from './document-support';
import { expectAligned } from './layout';
import { endOpenSessionRPC, endSessionRPC, openSessionPage, startSessionRPC, tableWithPensantus } from './live-session-support';
import { canvasPng, createMapRPC, createPointRPC, placeTokenRPC, revealMapRPC, setCurrentMapRPC, tableForMaps, uploadImageRPC } from './maps-support';
import { adjustVitalsRPC, beginAttackCombatRPC, combatRPC, getEncounterRPC, passTurnsTo, pensantusCasting, tableForCombat, toren, torenSheet } from './combat-support';
import { addActionRPC, cartActions, getOpenSceneRPC, openSceneRPC, rollSceneRPC, sceneActionIdsRPC, setAttemptsRPC, setShowDcRPC, tableForScenes } from './scene-support';
import { addClueRPC, cartClues, cartHooks, createNoteRPC } from './notes-support';
import { createCapitaoRPC, createMiraRPC, playedCombatRPC, putOnStageRPC, uploadPortrait } from './stage-support';
import { printRoute, tableForPrinting } from './print-support';
import { tableForLevelUp } from './levelup-support';
import { authStatePath, callRPC, characterRpcBody, createCharacterRPC, newSignedInContext, pensantus } from './support';
import { beginJointCombat, endPartRPC, jointTable } from './joint-turn-support';
import { tableForCreatures } from './creatures-support';
import { awardXpRPC, createEnemyRPC, tableForXp, tableForXpCombat, winCombatRPC } from './xp-support';

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

/** Printing a map to scale (MR-033, E8-12): the map page's entry with and
 * without a grid, and the print view in its states: the defaults, a size and
 * a paper that need 3 sheets, one that needs 36 (the amber notice, the labels
 * shrunk), 136 (the labels gone), an invalid size, and a map without a grid.
 * On a phone the setup stacks above the preview. */
async function scanPrintScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  test.setTimeout(180_000);
  const options = { colorScheme, viewport: { width, height: 900 } };
  const master = await browser.newContext({ ...options, storageState: authStatePath('Mestre Teste') });
  const player = await browser.newContext({ ...options, storageState: authStatePath('Jogador Teste') });
  const masterPage = await master.newPage();
  const playerPage = await player.newPage();
  const suffix = `(${colorScheme}, ${width}px)`;
  try {
    await masterPage.goto('/');
    const table = await tableForPrinting(masterPage, playerPage, `Acessibilidade impressão ${Date.now()}`);

    await open(masterPage, `/campanhas/${table.campaignId}/mapas/${table.gridMapId}`);
    await expectScreenPasses(masterPage, `Mapa com grade, entrada de impressão ${suffix}`);
    await open(masterPage, `/campanhas/${table.campaignId}/mapas/${table.plainMapId}`);
    await expect(masterPage.getByText('Defina a grade do mapa para imprimir em escala')).toBeVisible();
    await expectScreenPasses(masterPage, `Mapa sem grade, entrada de impressão ${suffix}`);

    await open(masterPage, printRoute(table.campaignId, table.gridMapId));
    await expect(masterPage.getByLabel('Tamanho do quadrado')).toHaveValue('2,54');
    await expectScreenPasses(masterPage, `Imprimir o mapa, A4 e 2,54 cm ${suffix}`);

    const square = masterPage.getByLabel('Tamanho do quadrado');
    await square.fill('2');
    await masterPage.getByRole('radio', { name: /A3/ }).check();
    await expect(masterPage.getByRole('button', { name: 'Voltar a 2,54 cm' })).toBeVisible();
    await expectScreenPasses(masterPage, `Imprimir o mapa, 2 cm em A3 ${suffix}`);

    await masterPage.getByRole('radio', { name: /A4/ }).check();
    await square.fill('5');
    await expect(masterPage.getByText('São 36 folhas.')).toBeVisible();
    await expectScreenPasses(masterPage, `Imprimir o mapa, 36 folhas e o aviso ${suffix}`);

    await square.fill('10');
    await expect(masterPage.getByText('São 136 folhas.')).toBeVisible();
    await expectScreenPasses(masterPage, `Imprimir o mapa, 136 folhas sem rótulos ${suffix}`);

    await square.fill('0,5');
    await expect(masterPage.getByRole('alert').filter({ hasText: 'Use um tamanho de 1 a 10 cm.' })).toBeVisible();
    await expectScreenPasses(masterPage, `Imprimir o mapa, tamanho inválido ${suffix}`);

    await open(masterPage, printRoute(table.campaignId, table.plainMapId));
    await expect(masterPage.getByText('Este mapa ainda não tem grade.')).toBeVisible();
    await expectScreenPasses(masterPage, `Imprimir o mapa, sem grade ${suffix}`);

    await open(playerPage, printRoute(table.campaignId, table.gridMapId));
    await expectScreenPasses(playerPage, `Imprimir o mapa, jogador ${suffix}`);
  } finally {
    await master.close();
    await player.close();
  }
}

test('a impressão do mapa passa no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-033'] }, async ({ browser }) => {
  await scanPrintScreens(browser, 'light', 1280);
});

test('a impressão do mapa passa no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-033'] }, async ({ browser }) => {
  await scanPrintScreens(browser, 'dark', 390);
});

test('a impressão do mapa passa no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-033'] }, async ({ browser }) => {
  await scanPrintScreens(browser, 'dark', 1024);
});

test('a impressão do mapa passa no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-033'] }, async ({ browser }) => {
  await scanPrintScreens(browser, 'light', 320);
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
    await expect(p.getByText('Mover 3,4 m')).toBeVisible();
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

/** The XP screens (Etapa 7, MR-016, RN-12): the end of a combat with "Dar XP" in each of its states, the
 * "Dar XP" dialog or sheet with its errors, the campaign's "Experiência" with its history and the question
 * of "Desfazer", the milestone's dialog and panel, the sheet with the XP block and the tag, and the NPC's
 * ND and XP (the list open, "Usar 50 XP"). Built in one function so the sweep is one place. */
async function scanXpScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: 900 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  const campaigns: string[] = [];
  /** A page that keeps its stream open never goes idle: wait for its h1 instead. */
  const openLive = async (page: Page, route: string, heading?: string) => {
    await page.goto(route);
    await expect(heading ? page.getByRole('heading', { name: heading }) : page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 30_000 });
  };
  try {
    await m.goto('/');
    await p.goto('/');

    // The end of a combat, before the XP is given.
    const combat = await tableForXpCombat(m, p, `Acessibilidade XP ${Date.now()}`, { experiencePoints: 2600 });
    const campaignId = combat.table.campaignId;
    campaigns.push(campaignId);
    await winCombatRPC(m, combat);
    await openLive(m, `/campanhas/${campaignId}/sessao`, 'Combate encerrado');
    const block = m.getByRole('region', { name: 'Experiência do combate' });
    await expect(block.getByRole('button', { name: /^Dar 350 XP/ })).toBeVisible();
    await expectScreenPasses(m, `Fim do combate com Dar XP ${where}`);
    await block.getByRole('checkbox', { name: 'Marcar Pensantus' }).uncheck();
    await expect(block.getByText('Marque pelo menos um personagem')).toBeVisible();
    await expectScreenPasses(m, `Fim do combate, ninguém marcado ${where}`);
    await block.getByRole('checkbox', { name: 'Marcar Pensantus' }).check();

    // "Agora não": the quiet line, and "Dar XP" opened from it, with an error.
    await block.getByRole('button', { name: 'Agora não' }).click();
    await expect(block.getByRole('button', { name: /XP do combate ainda não dado/ })).toBeFocused();
    await expectScreenPasses(m, `Fim do combate, XP para depois ${where}`);
    await block.getByRole('button', { name: /XP do combate ainda não dado/ }).click();
    const dialog = m.getByRole('dialog', { name: 'Dar XP' });
    await expect(dialog.getByLabel('Motivo')).toHaveValue('Combate: Emboscada na estrada');
    await expectScreenPasses(m, `Dar XP, aberto pelo resumo ${where}`);
    await dialog.getByLabel('Motivo').fill('');
    await dialog.getByLabel('XP para o grupo').focus();
    await dialog.getByLabel('Motivo').focus();
    await dialog.getByLabel('XP para o grupo').focus();
    await expect(dialog.getByText('Escreva o motivo do XP.')).toBeVisible();
    await expectScreenPasses(m, `Dar XP, com erro ${where}`);
    await dialog.getByLabel('Motivo').fill('Combate: Emboscada na estrada');
    await dialog.getByRole('button', { name: /^Dar 350 XP/ }).click();
    await expect(block).toContainText('350 XP dados');
    await expectScreenPasses(m, `Fim do combate, XP dado ${where}`);

    // The campaign page: the panel with its history, the question, the player's view and the sheet.
    await awardXpRPC(m, campaignId, { mode: 'MANUAL', reason: 'Pela ajuda ao ferreiro', characterIds: [combat.table.characterId], amount: 40 });
    // The campaign page of a master with a session open keeps the session's stream (MR-040): not `open`.
    await openLive(m, `/campanhas/${campaignId}`);
    await expect(m.getByRole('region', { name: 'Experiência', exact: true })).toContainText('Pela ajuda ao ferreiro');
    await expectScreenPasses(m, `Campanha com Experiência, mestre ${where}`);
    await m.getByRole('button', { name: /^Desfazer/ }).click();
    await expect(m.getByRole('alertdialog')).toBeVisible();
    await expectScreenPasses(m, `Experiência, Desfazer a pergunta ${where}`);
    await m.getByRole('button', { name: 'Voltar' }).click();
    await m.getByRole('button', { name: 'Dar XP' }).click();
    await expect(m.getByRole('dialog', { name: 'Dar XP' })).toBeVisible();
    await m.getByRole('dialog').getByLabel('Motivo').fill('Pelo resgate do mercador');
    await m.getByRole('dialog').getByLabel('XP para o grupo').fill('150');
    await expectScreenPasses(m, `Dar XP, a qualquer hora ${where}`);
    await m.getByRole('dialog').getByRole('button', { name: 'Cancelar' }).click();
    await open(p, `/campanhas/${campaignId}`);
    await expect(p.getByRole('region', { name: 'Experiência', exact: true })).toContainText('Todos da campanha veem este histórico.');
    await expectScreenPasses(p, `Campanha com Experiência, jogador ${where}`);
    await openLive(p, `/campanhas/${campaignId}/personagens/${combat.table.characterId}`);
    // The level-up block says it (MR-040), not the XP block's tag.
    await expect(p.getByRole('heading', { name: 'Pensantus pode subir de nível' })).toBeVisible();
    await expectScreenPasses(p, `Ficha com XP e Pode subir de nível ${where}`);

    // Milestones: the dialog, the panel after it and the sheet with only the tag.
    const marks = await tableForXp(m, p, `Acessibilidade marcos ${Date.now()}`, 'XP_MODE_MILESTONES');
    campaigns.push(marks.campaignId);
    await startSessionRPC(m, marks.campaignId);
    await openLive(m, `/campanhas/${marks.campaignId}`);
    await expectScreenPasses(m, `Experiência por marcos, sem marcos ${where}`);
    await m.getByRole('button', { name: 'Registrar um marco fora da lista' }).click();
    const markDialog = m.getByRole('dialog', { name: 'Registrar marco' });
    await markDialog.getByLabel('O que aconteceu').fill('Marco: a ponte do rio foi salva');
    await expectScreenPasses(m, `Registrar marco ${where}`);
    await markDialog.getByRole('button', { name: 'Registrar marco' }).click();
    await expect(m.getByRole('status').filter({ hasText: 'Marco registrado' })).toBeVisible();
    await expectScreenPasses(m, `Experiência por marcos, depois do marco ${where}`);
    await openLive(p, `/campanhas/${marks.campaignId}/personagens/${marks.characterId}`);
    await expect(p.getByRole('heading', { name: /pode subir de nível/ })).toBeVisible();
    await expectScreenPasses(p, `Ficha por marcos, o bloco de subir de nível ${where}`);

    // The NPC: the minion's section with the list open, "Usar 50 XP", and the enemy's header fields.
    const npc = await createEnemyRPC(m, campaignId, 'Capitão Goblin', '1', 200);
    await open(m, `/campanhas/${campaignId}/npcs/novo/minion`);
    await expectScreenPasses(m, `NPC curto com Ao ser derrotado ${where}`);
    const nd = m.getByRole('combobox', { name: 'Nível de desafio (ND)' });
    await nd.click();
    await expect(m.getByRole('option', { name: /^ND 1\/4/ })).toBeVisible();
    await expectScreenPasses(m, `NPC curto, lista de ND ${where}`);
    await m.getByRole('option', { name: /^ND 1\/4/ }).click();
    await m.getByLabel('XP ao derrotar').fill('0');
    await expect(m.getByRole('button', { name: 'Usar 50 XP' })).toBeVisible();
    await expectScreenPasses(m, `NPC curto, XP zero com Usar ${where}`);
    await open(m, `/campanhas/${campaignId}/personagens/${npc}/editar`);
    await expect(m.getByLabel('XP ao derrotar')).toHaveValue('200');
    await expectScreenPasses(m, `Inimigo, ND e XP no passo Básico ${where}`);
    await openLive(m, `/campanhas/${campaignId}/personagens/${npc}`);
    await expectScreenPasses(m, `Ficha do inimigo com ND e XP ${where}`);
  } finally {
    for (const id of campaigns) {
      await endOpenSessionRPC(m, id);
    }
    await master.close();
    await player.close();
  }
}

test('as telas de XP passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-016'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanXpScreens(browser, 'light', 1280);
});

test('as telas de XP passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-016'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanXpScreens(browser, 'dark', 390);
});

/**
 * The RP scenes (Etapa 7, MR-015; E7-01 to E7-05): the point panel with its
 * actions (the list, the add form, the DC error, the empty state, the full
 * list; on a computer), the master's "Cena de RP" and the picker, the open
 * scene with its rolls, and the player's scene block, roll sheet in each of
 * its states (how, typed, the number out of 1 to 20, the result), the rolled
 * row and the page without a scene. The rolls of the master's screen are made
 * through the API so the screens are the same on every run.
 */
async function scanSceneScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: width >= 768 ? 900 : 844 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForScenes(m, p, `Acessibilidade cenas ${Date.now()}`);
    campaignId = table.campaignId;

    // The editor is for a computer: a phone has the lists of points instead.
    if (width >= 768) {
      await open(m, `/campanhas/${campaignId}/mapas/${table.mapId}`);
      await m.getByRole('button', { name: /^A carroça tombada, Cena de RP/ }).click();
      await expect(m.getByRole('heading', { name: 'Ações da cena' })).toBeVisible();
      await expectScreenPasses(m, `Ações da cena no ponto ${where}`);
      await m.getByRole('button', { name: 'Adicionar ação' }).click();
      await expect(m.getByRole('form', { name: 'Nova ação' })).toBeVisible();
      await expectScreenPasses(m, `Nova ação ${where}`);
      await m.getByLabel('CD (opcional)').fill('31');
      await m.getByRole('form', { name: 'Nova ação' }).getByRole('button', { name: 'Adicionar ação' }).click();
      await expect(m.getByText('A CD vai de 1 a 30.')).toBeVisible();
      await expectScreenPasses(m, `Nova ação com a CD fora de 1 a 30 ${where}`);
      await m.getByRole('button', { name: 'Cancelar' }).click();
      await m.getByRole('button', { name: /^Vau do riacho, Cena de RP/ }).click();
      await expect(m.getByText('Nenhuma ação ainda')).toBeVisible();
      await expectScreenPasses(m, `Ações da cena, vazia ${where}`);
      for (let i = 0; i < 20; i++) {
        await addActionRPC(m, table, table.fordId, { key: 'skill:arcana' });
      }
      await open(m, `/campanhas/${campaignId}/mapas/${table.mapId}`);
      await m.getByRole('button', { name: /^Vau do riacho, Cena de RP/ }).click();
      await expect(m.getByText('Limite de 20 ações. Remova uma para adicionar outra.')).toBeVisible();
      await expectScreenPasses(m, `Ações da cena, lista cheia ${where}`);
    }

    // The session: "Cena de RP", the picker, the open scene with its rolls.
    await openSessionPage(m, campaignId);
    await expect(m.getByRole('heading', { name: 'Cena de RP' })).toBeVisible();
    await expectScreenPasses(m, `Sessão com "Cena de RP" ${where}`);
    await m.getByRole('button', { name: 'Abrir cena', exact: true }).click();
    await expect(m.getByRole('dialog', { name: 'Abrir uma cena' })).toBeVisible();
    await m.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(m, `Abrir uma cena ${where}`);
    await m.getByRole('dialog').getByText('A carroça tombada', { exact: true }).click();
    await m.getByRole('dialog').getByRole('button', { name: 'Abrir cena', exact: true }).click();
    await expect(m.getByRole('heading', { name: 'Cena: A carroça tombada' })).toBeFocused();

    // The player: the block, the roll sheet in each state, the rolled row.
    await openSessionPage(p, campaignId);
    const scene = p.getByRole('region', { name: 'Cena: A carroça tombada' });
    await expect(scene).toBeVisible();
    await expectScreenPasses(p, `Cena do jogador ${where}`);
    await scene.getByRole('button', { name: 'Rolar Procurar pistas na carroça' }).click();
    const sheet = p.getByRole('dialog', { name: 'Rolar Procurar pistas na carroça' });
    await expect(sheet.getByRole('button', { name: 'Rolar no app' })).toBeVisible();
    await p.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(p, `Rolar a ação, no app ${where}`);
    await sheet.getByRole('button', { name: 'Digitar o resultado' }).click();
    await expect(sheet.getByRole('heading', { name: 'Digite o resultado do dado' })).toBeVisible();
    await expectScreenPasses(p, `Rolar a ação, digitando ${where}`);
    await sheet.getByLabel(/Role 1d20 para Investigação/).fill('27');
    await expect(sheet.getByRole('alert')).toBeVisible();
    await expectScreenPasses(p, `Rolar a ação, número fora de 1 a 20 ${where}`);
    await sheet.getByLabel(/Role 1d20 para Investigação/).fill('11');
    await expect(sheet.getByRole('status')).toContainText('11 + 6 = 17');
    await expectScreenPasses(p, `Rolar a ação, número valido ${where}`);
    await sheet.getByRole('button', { name: 'Confirmar 11' }).click();
    await expect(sheet.getByText('Seu total em Investigação')).toBeVisible();
    await expectScreenPasses(p, `Rolar a ação, resultado ${where}`);
    await sheet.getByRole('button', { name: 'Voltar à cena' }).click();
    await expect(scene.getByText('Rolada')).toBeVisible();
    await expectScreenPasses(p, `Cena do jogador, uma ação rolada ${where}`);

    // The master with the roll and one more, with and without a DC.
    const open1 = await getOpenSceneRPC(p, campaignId);
    const ids = open1.scene!.actions.map((a) => a.id);
    await rollSceneRPC(p, campaignId, ids[3], 14);
    await rollSceneRPC(p, campaignId, ids[4], 3);
    await expect(m.getByText('Não passou · CD 10')).toBeVisible();
    await expect(m.getByText('Passou · CD 12')).toBeVisible();
    await expectScreenPasses(m, `Cena aberta com três rolagens ${where}`);
    if (width < 768) {
      await m.getByRole('button', { name: 'Ações da cena' }).click();
      await expect(m.getByRole('button', { name: 'Ações da cena' })).toHaveAttribute('aria-expanded', 'false');
      await expectScreenPasses(m, `Cena aberta, ações recolhidas ${where}`);
      await m.getByRole('button', { name: 'Ações da cena' }).click();
    }
    // With a scene open, the points of the map say what they do with it.
    await expect(m.getByText('Cena aberta agora')).toBeVisible();
    await expect(m.getByRole('button', { name: 'Trocar para a cena Posto da guarda' })).toBeVisible();
    await expectScreenPasses(m, `Pontos do mapa com a cena aberta ${where}`);
    await m.getByRole('button', { name: 'Trocar cena' }).click();
    await expect(m.getByRole('dialog', { name: 'Abrir uma cena' })).toBeVisible();
    await m.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(m, `Trocar cena ${where}`);
    await m.getByRole('button', { name: 'Cancelar' }).click();

    // No scene: nothing extra for the player.
    await m.getByRole('button', { name: 'Fechar cena' }).click();
    await expect(p.getByRole('region', { name: 'Cena: A carroça tombada' })).toHaveCount(0);
    await expectScreenPasses(p, `Sessão do jogador sem cena ${where}`);
    await expectScreenPasses(m, `Sessão do mestre depois de fechar a cena ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
}

test('as cenas de RP passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-015'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanSceneScreens(browser, 'light', 1280);
});

test('as cenas de RP passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-015'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanSceneScreens(browser, 'dark', 390);
});

test('as cenas de RP passam no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-015'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanSceneScreens(browser, 'dark', 1024);
});

test('as cenas de RP passam no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-015'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanSceneScreens(browser, 'light', 320);
});

/**
 * The planned milestones (Etapa 8, MR-016, RN-12, RN-20; E8-14): the empty panel, "Adicionar marco" open with its
 * error, the list, the edit field and the removal question in place, "Marcar como alcançado" (a dialog on a
 * computer, a sheet on a phone), the reached milestone with "Dar a mais alguém"'s absence (one character only),
 * "Desfazer"'s question, and the player's panel before and after the first milestone. Built in one function so
 * the sweep is one place.
 */
async function scanMilestoneScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: width <= 390 ? 700 : 900 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  const experience = (page: Page) => page.getByRole('region', { name: 'Experiência', exact: true });
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForXp(m, p, `Acessibilidade marcos planejados ${Date.now()}`, 'XP_MODE_MILESTONES');
    const route = `/campanhas/${table.campaignId}`;

    // Nothing planned yet, and the player's empty state.
    await open(m, route);
    await expect(experience(m)).toContainText('Nenhum marco planejado');
    await expectScreenPasses(m, `Marcos, nenhum planejado ${where}`);
    await open(p, route);
    await expect(experience(p)).toContainText('Nenhum marco alcançado ainda');
    await expectScreenPasses(p, `Marcos, jogador antes do primeiro ${where}`);

    // "Adicionar marco" open, with its error.
    await experience(m).getByRole('button', { name: 'Adicionar marco', exact: true }).click();
    await experience(m).getByLabel('Nome do marco').press('Enter');
    await expect(experience(m).getByRole('alert')).toContainText('Escreva o nome do marco');
    await expectScreenPasses(m, `Marcos, Adicionar marco com erro ${where}`);
    for (const text of ['Salvar o mercador', 'Chegar ao Vale Seco', 'Derrotar o Barão Ivo']) {
      await experience(m).getByLabel('Nome do marco').fill(text);
      await experience(m).getByLabel('Nome do marco').press('Enter');
      await expect(experience(m).getByRole('button', { name: `Marcar “${text}” como alcançado` })).toBeVisible();
      if (text !== 'Derrotar o Barão Ivo') {
        await experience(m).getByRole('button', { name: 'Adicionar marco', exact: true }).click();
      }
    }
    await expectScreenPasses(m, `Marcos, a lista planejada ${where}`);

    // Edit in place, and the removal question.
    await experience(m).getByRole('button', { name: 'Editar Salvar o mercador' }).click();
    await expect(experience(m).getByLabel('Nome do marco')).toHaveValue('Salvar o mercador');
    await expectScreenPasses(m, `Marcos, editar no lugar ${where}`);
    await experience(m).getByRole('button', { name: 'Cancelar' }).click();
    await experience(m).getByRole('button', { name: 'Remover Salvar o mercador' }).click();
    await expect(m.getByRole('alertdialog')).toBeVisible();
    await expectScreenPasses(m, `Marcos, remover a pergunta ${where}`);
    await m.getByRole('button', { name: 'Voltar' }).click();

    // "Marcar como alcançado".
    await experience(m).getByRole('button', { name: 'Marcar “Chegar ao Vale Seco” como alcançado' }).click();
    await expect(m.getByRole('dialog', { name: 'Marcar “Chegar ao Vale Seco” como alcançado' })).toBeVisible();
    await expectScreenPasses(m, `Marcar como alcançado ${where}`);
    await m.getByRole('dialog').getByRole('button', { name: 'Marcar como alcançado' }).click();
    await expect(experience(m).getByRole('status').filter({ hasText: 'Marco alcançado' })).toBeVisible();
    await expectScreenPasses(m, `Marcos, depois de alcançar ${where}`);

    // "Desfazer" asks in place.
    await experience(m).getByRole('button', { name: 'Desfazer Chegar ao Vale Seco' }).click();
    await expect(m.getByRole('alertdialog')).toBeVisible();
    await expectScreenPasses(m, `Marcos, desfazer a pergunta ${where}`);
    await m.getByRole('button', { name: 'Voltar' }).click();

    // The player, after the first milestone: only the reached one, and the own character.
    await open(p, route);
    await expect(experience(p)).toContainText('Chegar ao Vale Seco');
    await expectScreenPasses(p, `Marcos, jogador depois do marco ${where}`);
  } finally {
    await master.close();
    await player.close();
  }
}

test('os marcos planejados passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-016'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanMilestoneScreens(browser, 'light', 1280);
});

test('os marcos planejados passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-016'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanMilestoneScreens(browser, 'dark', 390);
});

test('os marcos planejados passam no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-016'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanMilestoneScreens(browser, 'light', 320);
});

/**
 * Clues, hooks and the players' notes (Etapa 8, MR-029, MR-030, E8-04 to
 * E8-07): the point panel's "Pistas" (empty, the list with who has each, the
 * add form with its error, the remove question, the full list) and "Ganchos e
 * anotações"; the open scene's clues and hooks (open and, on a phone, folded);
 * "Revelar pista" with nobody checked, one checked and after revealing; the
 * player's notice and the bar's button with a new clue; the notes sheet (empty,
 * the list with a clue, the filter open, a scene with nothing, a new note, the
 * limit of 2.000, editing with its two questions); and the panel on the
 * character sheet (list, form, empty).
 */
async function scanNotesScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: width >= 768 ? 900 : 844 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForScenes(m, p, `Acessibilidade pistas ${Date.now()}`, false);
    campaignId = table.campaignId;

    // The editor is for a computer: "Pistas" and "Ganchos e anotações" in the point panel.
    if (width >= 768) {
      await open(m, `/campanhas/${campaignId}/mapas/${table.mapId}`);
      await m.getByRole('button', { name: /^A carroça tombada, Cena de RP/ }).click();
      await expect(m.getByText('Nenhuma pista ainda')).toBeVisible();
      await expectScreenPasses(m, `Pistas e ganchos, vazios ${where}`);
      for (const text of cartClues) {
        await addClueRPC(m, table, table.cartId, text);
      }
      await open(m, `/campanhas/${campaignId}/mapas/${table.mapId}`);
      await m.getByRole('button', { name: /^A carroça tombada, Cena de RP/ }).click();
      await expect(m.getByText('3 de 30', { exact: true })).toBeVisible();
      await m.getByRole('textbox', { name: 'Ganchos e anotações' }).fill(cartHooks);
      await expectScreenPasses(m, `Pistas e ganchos, com três pistas ${where}`);
      await m.getByRole('button', { name: 'Adicionar pista' }).click();
      await expect(m.getByRole('form', { name: 'Nova pista' })).toBeVisible();
      await expectScreenPasses(m, `Nova pista ${where}`);
      await m.getByRole('form', { name: 'Nova pista' }).getByRole('button', { name: 'Adicionar pista' }).click();
      await expect(m.getByText('Escreva a pista antes de salvar.')).toBeVisible();
      await expectScreenPasses(m, `Nova pista, com o erro ${where}`);
      await m.getByRole('form', { name: 'Nova pista' }).getByLabel('Texto da pista').fill('x'.repeat(512));
      await expect(m.getByText('512 de 500')).toBeVisible();
      await expectScreenPasses(m, `Nova pista, passou de 500 caracteres ${where}`);
      await m.getByRole('button', { name: 'Cancelar' }).click();
      await m.getByRole('button', { name: 'Remover a pista 2' }).click();
      await expect(m.getByRole('alertdialog', { name: 'Remover a pista 2?' })).toBeVisible();
      await expectScreenPasses(m, `Remover a pista, a pergunta ${where}`);
      await m.getByRole('button', { name: 'Voltar' }).click();
      await m.getByRole('button', { name: /^Vau do riacho, Cena de RP/ }).click();
      for (let i = 1; i <= 30; i++) {
        await addClueRPC(m, table, table.fordId, `Pista número ${i}`);
      }
      await open(m, `/campanhas/${campaignId}/mapas/${table.mapId}`);
      await m.getByRole('button', { name: /^Vau do riacho, Cena de RP/ }).click();
      await expect(m.getByText('Limite de 30 pistas. Remova uma para adicionar outra.')).toBeVisible();
      await expectScreenPasses(m, `Pistas, lista cheia ${where}`);
    } else {
      for (const text of cartClues) {
        await addClueRPC(m, table, table.cartId, text);
      }
      const hooks = await callRPC(m, 'meurpg.maps.v1.MapService/UpdateMapPoint', { campaignId, mapId: table.mapId, pointId: table.cartId, hooks: cartHooks });
      expect(hooks.ok()).toBeTruthy();
    }
    await callRPC(m, 'meurpg.maps.v1.MapService/UpdateMapPoint', { campaignId, mapId: table.mapId, pointId: table.cartId, hooks: cartHooks });

    // The player's notes before anything arrived: the empty sheet.
    await openSessionPage(m, campaignId);
    await openSessionPage(p, campaignId);
    await p.getByRole('button', { name: 'Anotações', exact: true }).click();
    await expect(p.getByText('Nenhuma anotação ainda')).toBeVisible();
    await p.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(p, `Anotações, vazias ${where}`);
    await p.getByRole('button', { name: 'Fechar' }).click();

    // The open scene with its clues and hooks, and "Revelar pista".
    await m.getByRole('button', { name: 'Abrir cena', exact: true }).click();
    await m.getByRole('dialog').getByText('A carroça tombada', { exact: true }).click();
    await m.getByRole('dialog').getByRole('button', { name: 'Abrir cena', exact: true }).click();
    await expect(m.getByRole('heading', { name: 'Cena: A carroça tombada' })).toBeFocused();
    await expectScreenPasses(m, `Cena aberta com pistas e ganchos ${where}`);
    if (width < 768) {
      await m.getByRole('button', { name: 'Abrir os ganchos e anotações' }).click();
      await expect(m.getByText(cartHooks)).toBeVisible();
      await expectScreenPasses(m, `Cena aberta, ganchos abertos ${where}`);
    }
    await m.getByRole('button', { name: 'Revelar a pista 2' }).click();
    await expect(m.getByRole('dialog', { name: 'Revelar pista' })).toBeVisible();
    await m.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(m, `Revelar pista, ninguém marcado ${where}`);
    await m.getByRole('button', { name: 'Marcar todos' }).click();
    await expect(m.getByRole('button', { name: 'Revelar para Pensantus' })).toBeVisible();
    await expectScreenPasses(m, `Revelar pista, uma pessoa marcada ${where}`);
    await m.getByRole('button', { name: 'Revelar para Pensantus' }).click();
    await expect(m.getByText(/Pista revelada para todos às/)).toBeVisible();
    await expectScreenPasses(m, `Cena aberta, depois de revelar ${where}`);

    // The player: the notice, the bar's button with a new clue, the sheet.
    await expect(p.getByText('O mestre revelou uma pista para você.')).toBeVisible();
    await expect(p.getByRole('button', { name: 'Anotações, 1 nova' })).toBeVisible();
    await expectScreenPasses(p, `Sessão do jogador com a pista nova ${where}`);
    await p.getByRole('button', { name: 'Abrir anotações' }).click();
    const sheet = p.getByRole('dialog');
    await expect(sheet.getByText('Pista do mestre')).toBeVisible();
    await p.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(p, `Anotações, só a pista ${where}`);
    await sheet.getByRole('button', { name: 'Nova anotação' }).click();
    await expect(sheet.getByRole('heading', { name: 'Nova anotação' })).toBeVisible();
    await expectScreenPasses(p, `Nova anotação ${where}`);
    await sheet.getByLabel('Anotação', { exact: true }).fill('x'.repeat(2000));
    await expect(sheet.getByText('Chegou ao limite de 2.000 caracteres.')).toBeVisible();
    await expectScreenPasses(p, `Nova anotação, no limite de 2.000 ${where}`);
    await sheet.getByLabel('Anotação', { exact: true }).fill('Perguntar ao ferreiro sobre o brasão de lobo');
    await sheet.getByRole('combobox', { name: /Cena \(opcional\)/ }).click();
    await expect(p.getByRole('option', { name: 'A carroça tombada' })).toBeVisible();
    await expectScreenPasses(p, `Nova anotação, escolhendo a cena ${where}`);
    await p.getByRole('option', { name: 'A carroça tombada' }).click();
    await sheet.getByRole('button', { name: 'Cancelar' }).click();
    await expect(sheet.getByRole('alertdialog', { name: 'Descartar o que você escreveu?' })).toBeVisible();
    await expectScreenPasses(p, `Nova anotação, descartar ${where}`);
    await sheet.getByRole('button', { name: 'Continuar' }).click();
    await sheet.getByRole('button', { name: 'Salvar anotação' }).click();
    await expect(sheet.getByRole('heading', { name: 'Anotações' })).toBeVisible();
    await expectScreenPasses(p, `Anotações, uma nota e uma pista ${where}`);
    await sheet.getByRole('combobox', { name: /^Cena/ }).click();
    await expect(p.getByRole('option', { name: /Todas as anotações/ })).toBeVisible();
    await expectScreenPasses(p, `Anotações, o filtro por cena aberto ${where}`);
    await p.getByRole('option', { name: /Sem cena/ }).click();
    await expect(sheet.getByText('Nada sem cena')).toBeVisible();
    await expectScreenPasses(p, `Anotações, filtro sem resultado ${where}`);
    // Reopened only once the list that faded out is gone (see notes.spec.ts).
    await expect(p.getByRole('listbox')).toHaveCount(0);
    await sheet.getByRole('combobox', { name: /^Cena/ }).click();
    await p.getByRole('option', { name: /Todas as anotações/ }).click();
    await expect(p.getByRole('listbox')).toHaveCount(0);
    await sheet.getByRole('button', { name: /^Editar a anotação/ }).click();
    await sheet.getByRole('button', { name: 'Apagar anotação' }).click();
    await expect(sheet.getByRole('alertdialog', { name: 'Apagar esta anotação?' })).toBeVisible();
    await expectScreenPasses(p, `Editar anotação, apagar ${where}`);
    await sheet.getByRole('button', { name: 'Voltar' }).click();
    await sheet.getByRole('button', { name: 'Cancelar' }).click();
    await sheet.getByRole('button', { name: 'Fechar' }).click();

    // The panel on the player's sheet; the master's own sheet page has none.
    await createNoteRPC(p, campaignId, 'Brisa me deve 5 PO');
    // Not `open()`: with a session open the sheet follows its stream, so the network is never idle.
    await p.goto(`/campanhas/${campaignId}/personagens/${table.characterId}`);
    const panel = p.getByRole('region', { name: 'Anotações' });
    await expect(panel.getByText('Brisa me deve 5 PO')).toBeVisible();
    await expectScreenPasses(p, `Ficha com as anotações ${where}`);
    await panel.getByRole('button', { name: 'Nova anotação' }).click();
    await expect(panel.getByRole('heading', { name: 'Nova anotação' })).toBeVisible();
    await expectScreenPasses(p, `Ficha, nova anotação ${where}`);
    await panel.getByRole('button', { name: 'Cancelar' }).click();
    await m.goto(`/campanhas/${campaignId}/personagens/${table.characterId}`);
    await expect(m.getByRole('heading', { name: 'Pensantus' }).first()).toBeVisible();
    await expect(m.getByRole('heading', { name: 'Anotações' })).toHaveCount(0);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
}

test('pistas, ganchos e anotações passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-029', '@MR-030'] }, async ({ browser }) => {
  test.setTimeout(600_000);
  await scanNotesScreens(browser, 'light', 1280);
});

test('pistas, ganchos e anotações passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-029', '@MR-030'] }, async ({ browser }) => {
  test.setTimeout(600_000);
  await scanNotesScreens(browser, 'dark', 390);
});

test('pistas, ganchos e anotações passam no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-029', '@MR-030'] }, async ({ browser }) => {
  test.setTimeout(600_000);
  await scanNotesScreens(browser, 'dark', 1024);
});

test('pistas, ganchos e anotações passam no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-029', '@MR-030'] }, async ({ browser }) => {
  test.setTimeout(600_000);
  await scanNotesScreens(browser, 'light', 320);
});

/** The joint turn (MR-013, E8-01): the master's card and boxes, the player's
 * pill, the other members' card, "Encerrar a minha parte" and its question, the
 * state after the part ended, and the goblins' turn (a group of NPCs alone). */
async function scanJointTurnScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
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
    const joint = await jointTable(m, p, `Acessibilidade turno conjunto ${Date.now()}`);
    campaignId = joint.table.campaignId;
    await beginJointCombat(m, joint);

    await m.goto(`/campanhas/${campaignId}/sessao`);
    await expect(m.getByRole('heading', { name: /^Turno conjunto: / })).toBeVisible();
    await expectScreenPasses(m, `Turno conjunto, visto pelo mestre ${where}`);
    await openSessionPage(p, campaignId);
    await expect(p.getByRole('heading', { name: 'Sua vez, Pensantus' })).toBeVisible();
    await expectScreenPasses(p, `Turno conjunto, a vez do jogador ${where}`);
    await p.getByRole('button', { name: 'Encerrar a minha parte' }).click();
    await expect(p.getByRole('alertdialog', { name: 'Encerrar a sua parte?' })).toBeVisible();
    await expectScreenPasses(p, `Encerrar a minha parte, com pergunta ${where}`);
    await p.getByRole('alertdialog').getByRole('button', { name: 'Encerrar a minha parte' }).click();
    await expect(p.getByRole('heading', { name: 'Você encerrou a sua parte' })).toBeVisible();
    await expectScreenPasses(p, `Turno conjunto, depois de encerrar ${where}`);
    await expectScreenPasses(m, `Turno conjunto, uma parte encerrada ${where}`);

    await endPartRPC(m, campaignId, 'Brisa');
    await expect(p.getByRole('heading', { name: 'Vez dos Goblins' })).toBeVisible();
    await expectScreenPasses(p, `Vez de um grupo de NPCs, jogador ${where}`);
    await expectScreenPasses(m, `Vez de um grupo de NPCs, mestre ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
}

test('o turno conjunto passa no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-013'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanJointTurnScreens(browser, 'light', 1280);
});

test('o turno conjunto passa no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-013'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanJointTurnScreens(browser, 'dark', 390);
});

test('o turno conjunto passa no axe e nas conferências de layout no celular de 320', { tag: ['@a11y', '@MR-013'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanJointTurnScreens(browser, 'light', 320);
});


// Etapa 8.6 (MR-031, MR-032): the NPC's portrait field, the master's and the
// players' stage with its larger view, and the combat highlights.
async function scanStageScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: width >= 768 ? 900 : 844 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForScenes(m, p, `Acessibilidade palco ${Date.now()}`);
    campaignId = table.campaignId;
    const miraImage = await uploadPortrait(m, campaignId, 'Retrato da Mira');
    const capitaoImage = await uploadPortrait(m, campaignId, 'Retrato do Capitão', '#6e8a52');
    const miraId = await createMiraRPC(m, campaignId, miraImage);
    const capitaoId = await createCapitaoRPC(m, campaignId, capitaoImage);
    const aldoId = await createMiraRPC(m, campaignId, '', 'Aldo');
    const ids = [miraId, capitaoId, aldoId];

    // The portrait field: with an image, the gallery picker, the question, and without.
    await open(m, `/campanhas/${campaignId}/personagens/${miraId}/editar`);
    await expectScreenPasses(m, `Retrato do NPC ${where}`);
    await m.getByRole('button', { name: 'Trocar retrato' }).click();
    await expect(m.getByRole('dialog', { name: 'Escolher o retrato de Mira' })).toBeVisible();
    await m.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(m, `Escolher o retrato ${where}`);
    await m.getByRole('dialog').getByRole('radio', { name: /Retrato do Capitão/ }).click();
    await expectScreenPasses(m, `Escolher o retrato, uma imagem escolhida ${where}`);
    await m.getByRole('dialog').getByRole('button', { name: 'Cancelar' }).click();
    await m.getByRole('button', { name: 'Remover retrato' }).click();
    await expect(m.getByText('A imagem continua na galeria.')).toBeVisible();
    await expectScreenPasses(m, `Remover o retrato, a pergunta ${where}`);
    await m.getByRole('button', { name: 'Voltar' }).click();
    await open(m, `/campanhas/${campaignId}/personagens/${capitaoId}/editar`);
    await expectScreenPasses(m, `Retrato do inimigo ${where}`);
    await open(m, `/campanhas/${campaignId}/personagens/${capitaoId}`);
    await expectScreenPasses(m, `Ficha do inimigo com o retrato ${where}`);

    // The stage: the master's cards and list, then the players' stage.
    await openSceneRPC(m, campaignId, table.cartId);
    await openSessionPage(m, campaignId);
    await openSessionPage(p, campaignId);
    await expect(m.getByRole('heading', { name: 'Cena: A carroça tombada' })).toBeVisible();
    await expectScreenPasses(m, `Em cena, vazio ${where}`);
    await m.getByRole('button', { name: 'Pôr em cena', exact: true }).click();
    await expect(m.getByRole('button', { name: 'Pôr Mira em cena' })).toBeVisible();
    await m.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(m, `Pôr em cena ${where}`);
    for (const name of ['Mira', 'Capitão Goblin']) {
      await m.getByRole('button', { name: `Pôr ${name} em cena` }).click();
      await expect(m.getByRole('button', { name: `Tirar ${name} de cena` }).or(m.getByText(`${name} entrou na cena.`).first()).first()).toBeVisible();
    }
    await m.getByRole('button', { name: width >= 768 ? 'Fechar' : 'Fechar', exact: true }).last().click();
    await m.getByRole('button', { name: 'Dar a fala a Capitão Goblin' }).click();
    await expect(m.getByRole('button', { name: 'Capitão Goblin está com a fala. Tirar a fala' })).toBeVisible();
    await expectScreenPasses(m, `Em cena, dois NPCs, um falando ${where}`);

    const stage = p.getByRole('group', { name: 'Em cena: Mira e Capitão Goblin' });
    await expect(stage).toBeVisible();
    await p.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(p, `Palco do jogador, dois NPCs ${where}`);
    await stage.getByRole('button', { name: 'Ver Mira maior' }).click();
    await expect(p.getByRole('heading', { name: 'Mira' })).toBeFocused();
    await p.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(p, `Mira maior ${where}`);
    await p.keyboard.press('Escape');
    await expect(stage.getByRole('button', { name: 'Ver Mira maior' })).toBeFocused();

    // Four on the stage: the grid on a phone, the row on a desktop.
    await putOnStageRPC(m, campaignId, aldoId);
    const extra = await createMiraRPC(m, campaignId, '', 'Barão Ivo');
    await putOnStageRPC(m, campaignId, extra);
    await expect(p.getByRole('group', { name: /^Em cena: .*, .* e / })).toBeVisible();
    await p.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(p, `Palco do jogador, quatro NPCs ${where}`);
    await expect(m.getByText('A cena comporta 4 NPCs. Tire um para pôr outro.')).toBeVisible();
    await expectScreenPasses(m, `Em cena, quatro de quatro ${where}`);
    expect(ids).toHaveLength(3);
    await m.getByRole('button', { name: 'Fechar cena' }).click();
    await endOpenSessionRPC(m, campaignId);
    campaignId = '';
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }

  // The combat highlights, in a table of their own.
  const master2 = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player2 = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m2 = await master2.newPage();
  const p2 = await player2.newPage();
  let id2 = '';
  try {
    await m2.goto('/');
    await p2.goto('/');
    const table = await tableForCombat(m2, p2, `Acessibilidade destaques ${Date.now()}`, true, true);
    id2 = table.campaignId;
    const enc = await playedCombatRPC(m2, p2, table);
    await openSessionPage(m2, id2);
    await openSessionPage(p2, id2);
    await combatRPC(m2, 'EndEncounter', { campaignId: id2, encounterId: enc.id });
    await expect(m2.getByRole('region', { name: 'Destaques do combate' })).toBeVisible();
    await expect(p2.getByRole('region', { name: 'Destaques do combate' })).toBeVisible();
    await m2.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await p2.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
    await expectScreenPasses(m2, `Destaques do combate, o mestre ${where}`);
    await expectScreenPasses(p2, `Destaques do combate, o cartão do jogador ${where}`);
  } finally {
    if (id2) {
      await endOpenSessionRPC(m2, id2);
    }
    await master2.close();
    await player2.close();
  }
}

test('o retrato, o palco e os destaques passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-031', '@MR-032'] }, async ({ browser }) => {
  test.setTimeout(480_000);
  await scanStageScreens(browser, 'light', 1280);
});

test('o retrato, o palco e os destaques passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-031', '@MR-032'] }, async ({ browser }) => {
  test.setTimeout(480_000);
  await scanStageScreens(browser, 'dark', 390);
});

test('o retrato, o palco e os destaques passam no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-031', '@MR-032'] }, async ({ browser }) => {
  test.setTimeout(480_000);
  await scanStageScreens(browser, 'dark', 1024);
});

test('o retrato, o palco e os destaques passam no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-031', '@MR-032'] }, async ({ browser }) => {
  test.setTimeout(480_000);
  await scanStageScreens(browser, 'light', 320);
});

/** The spells in the session (Etapa 8, slice 8.4; E8-02, E8-03): the list with its "?" and slot
 * rows, the details sheet, the cast sheet with its "?", the details over it, the result a player
 * reads, and the master's card under Sono in the log. */
async function scanCombatDetailsScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: 900 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  const phone = width < 768;
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForCombat(m, p, `Acessibilidade magias ${Date.now()}`, true, true);
    campaignId = table.campaignId;
    await beginAttackCombatRPC(m, table, { Pensantus: 20, 'Capitão Goblin': 15, 'Goblin 1': 5, 'Goblin 2': 4 });
    await openSessionPage(m, campaignId);
    await openSessionPage(p, campaignId);

    await expect(p.getByRole('button', { name: 'Detalhes de Sono' })).toBeVisible();
    await expectScreenPasses(p, `Magias com o "?" e os espaços ${where}`);
    await p.getByRole('button', { name: 'Detalhes de Sono' }).click();
    const details = p.getByRole('dialog', { name: phone ? 'Descrição de Sono' : 'Sono', exact: true });
    await expect(details.getByText('This spell sends creatures into a magical slumber.')).toBeVisible();
    await expectScreenPasses(p, `Detalhes de Sono na sessão ${where}`);
    await details.getByRole('button', { name: 'Fechar' }).last().click();

    await p.getByRole('button', { name: 'Conjurar Sono' }).click();
    const sheet = p.getByRole('dialog', { name: 'Conjurar Sono' });
    await sheet.locator('label', { hasText: 'Goblin 1' }).click();
    await sheet.locator('label', { hasText: 'Capitão Goblin' }).click();
    await expectScreenPasses(p, `Conjurar Sono, quem está na área ${where}`);
    await sheet.getByRole('button', { name: 'Detalhes de Sono' }).click();
    await expect(p.getByRole('dialog', { name: phone ? 'Descrição de Sono' : 'Sono', exact: true })).toBeVisible();
    await expectScreenPasses(p, `Detalhes de Sono por cima de Conjurar ${where}`);
    await p.getByRole('dialog', { name: phone ? 'Descrição de Sono' : 'Sono', exact: true }).getByRole('button', { name: 'Fechar' }).last().click();
    // A dialog on a desktop is named by its title, which changes with the step: ask the page.
    await sheet.getByRole('button', { name: 'Digitar o resultado' }).click();
    await p.getByLabel(/Role 5d8/).fill('20');
    await p.getByRole('button', { name: 'Confirmar 20' }).click();
    await expect(p.getByRole('heading', { name: 'Sono conjurado' })).toBeVisible();
    await expectScreenPasses(p, `Sono conjurado, o resultado do jogador ${where}`);
    await p.getByRole('button', { name: 'Voltar à sua vez' }).click();

    if (phone) {
      // The master's log is folded on a phone.
      await m.getByRole('button', { name: 'Abrir o registro' }).click();
    }
    await expect(m.locator('app-pool-card')).toBeVisible({ timeout: 20_000 });
    await expectScreenPasses(m, `O cartão do Sono no registro do mestre ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
}

test('as magias na sessão passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-014'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanCombatDetailsScreens(browser, 'light', 1280);
});

test('as magias na sessão passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-014'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanCombatDetailsScreens(browser, 'dark', 390);
});

/**
 * The options of the scene (Etapa 8, MR-015, RN-20; E8-13): the point panel with the DC switch off and on and the
 * attempts of each action (on a computer), the master's open scene with "Os jogadores veem a CD", the limits and
 * "Tentativa 1 de 3", "Dar mais uma tentativa" and its question in place and the status after it, and the
 * player's scene with the CD pill, "Passou"/"Não passou", the attempts that are left ("Restam 2 de 3 tentativas",
 * "Sem mais tentativas") and the page with the DC hidden. The rolls are made through the API, so the screens are
 * the same on every run.
 */
async function scanSceneOptionsScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: width >= 768 ? 900 : width <= 320 ? 568 : 844 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForScenes(m, p, `Acessibilidade opções ${Date.now()}`);
    campaignId = table.campaignId;
    const ids = await sceneActionIdsRPC(m, table, table.cartId);
    await setAttemptsRPC(m, table, table.cartId, ids['Percepção'], 3);
    await setAttemptsRPC(m, table, table.cartId, ids['Acalmar os cavalos'], 0);

    // The editor: the switch off and on, and every action with its attempts.
    await open(m, `/campanhas/${campaignId}/mapas/${table.mapId}`);
    const cart = m.getByRole('button', { name: /^A carroça tombada, Cena de RP/ });
    if (await cart.isVisible()) {
      await cart.click();
      await expect(m.getByRole('switch', { name: 'Mostrar a CD aos jogadores' })).toBeVisible();
      await expectScreenPasses(m, `Ações da cena com o interruptor da CD desligado ${where}`);
      await m.getByRole('switch', { name: 'Mostrar a CD aos jogadores' }).click();
      await expect(m.getByText('Como o jogador vê, antes e depois de rolar')).toBeVisible();
      await expectScreenPasses(m, `Ações da cena com o interruptor da CD ligado ${where}`);
      await expect(m.getByRole('status').filter({ hasText: 'Os jogadores agora veem a CD.' })).toHaveCount(1);
      await expect(m.getByText('Sem limite: o jogador rola quantas vezes quiser')).toBeVisible();
      await expectScreenPasses(m, `Ações da cena com "Sem limite" ${where}`);
    }

    // The session: the scene is open with the DC shown. The player rolls (Percepção 4 + 1 = 5, Investigação 11 + 6 = 17, Constituição 3 + 3 = 6).
    await setShowDcRPC(m, table, table.cartId, true);
    await openSceneRPC(m, campaignId, table.cartId);
    await openSessionPage(p, campaignId);
    const scene = p.getByRole('region', { name: 'Cena: A carroça tombada' });
    await expect(scene).toBeVisible();
    await expect(scene.getByText('CD 12')).toBeVisible();
    await expect(scene.getByText('Restam 3 de 3 tentativas')).toBeVisible();
    await expectScreenPasses(p, `Cena do jogador com a CD à mostra ${where}`);
    const open1 = await getOpenSceneRPC(p, campaignId);
    const actionIds = open1.scene!.actions.map((a) => a.id);
    await rollSceneRPC(p, campaignId, actionIds[3], 4);
    await rollSceneRPC(p, campaignId, actionIds[0], 11);
    await rollSceneRPC(p, campaignId, actionIds[4], 3);
    await expect(scene.getByText('Restam 2 de 3 tentativas')).toBeVisible();
    await expect(scene.getByText('Passou · CD 12')).toBeVisible();
    await expect(scene.getByText('Não passou · CD 10')).toBeVisible();
    await expect(scene.getByText('Sem mais tentativas').first()).toBeVisible();
    await expectScreenPasses(p, `Cena do jogador com Passou, Não passou e as tentativas ${where}`);

    await openSessionPage(m, campaignId);
    await expect(m.getByRole('heading', { name: 'Cena: A carroça tombada' })).toBeVisible();
    await expect(m.getByText('Os jogadores veem a CD')).toBeVisible();
    await expect(m.getByText('Tentativa 1 de 1').first()).toBeVisible();
    await expectScreenPasses(m, `Cena aberta do mestre com os limites e as tentativas ${where}`);
    const grant = m.getByRole('button', { name: 'Dar mais uma tentativa a Pensantus em Resistir ao cheiro de fumaça' });
    await grant.click();
    const ask = m.getByRole('alertdialog', { name: 'Dar mais uma tentativa a Pensantus?' });
    await expect(ask.getByRole('button', { name: 'Voltar' })).toBeFocused();
    await expectScreenPasses(m, `Dar mais uma tentativa, a pergunta no lugar ${where}`);
    await ask.getByRole('button', { name: 'Dar mais uma tentativa' }).click();
    await expect(m.getByRole('status').filter({ hasText: 'Mais uma tentativa dada a Pensantus' })).toBeVisible();
    await expectScreenPasses(m, `Dar mais uma tentativa, depois de dar ${where}`);
    await expect(scene.getByText('Restam 1 de 1 tentativas').or(scene.getByText('1 tentativa')).first()).toBeVisible();
    await expectScreenPasses(p, `Cena do jogador com uma tentativa a mais ${where}`);

    // The DC hidden (the default): no CD and no "Passou" for the player.
    await setShowDcRPC(m, table, table.cartId, false);
    await expect(scene.getByText('CD 12')).toHaveCount(0);
    await expectScreenPasses(p, `Cena do jogador com a CD escondida ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
}

test('as opções da cena passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-015'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanSceneOptionsScreens(browser, 'light', 1280);
});

test('as opções da cena passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-015'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanSceneOptionsScreens(browser, 'dark', 390);
});

test('as opções da cena passam no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-015'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanSceneOptionsScreens(browser, 'dark', 1024);
});

test('as opções da cena passam no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-015'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanSceneOptionsScreens(browser, 'light', 320);
});

/**
 * The session summary (Etapa 8, MR-032, RN-20; E8-11 states 4 and 5): the master's "Sessão encerrada" with the
 * highlights and the table, the player's card "A sessão acabou" and the plain notice after "Fechar". The rolls are
 * made through the API, with the DC shown, so the numbers are the same on every run.
 */
async function scanSessionSummaryScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: width >= 768 ? 900 : width <= 320 ? 568 : 844 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForScenes(m, p, `Acessibilidade resumo ${Date.now()}`);
    campaignId = table.campaignId;
    await setShowDcRPC(m, table, table.cartId, true);
    await openSceneRPC(m, campaignId, table.cartId);
    await openSessionPage(p, campaignId);
    const open1 = await getOpenSceneRPC(p, campaignId);
    const actionIds = open1.scene!.actions.map((a) => a.id);
    await rollSceneRPC(p, campaignId, actionIds[0], 11);
    await rollSceneRPC(p, campaignId, actionIds[4], 3);
    await openSessionPage(m, campaignId);
    await expect(m.getByText('Passou · CD 12')).toBeVisible();

    await m.getByRole('button', { name: 'Encerrar sessão' }).click();
    await m.getByRole('button', { name: 'Confirmar encerramento' }).click();
    await expect(m.getByRole('heading', { name: 'Sessão encerrada' })).toBeVisible();
    await expect(m.getByRole('table', { name: 'Testes passados fora do combate' })).toBeVisible();
    await expectScreenPasses(m, `Resumo da sessão do mestre ${where}`);

    const card = p.getByRole('region', { name: 'Resumo da sessão' });
    await expect(card.getByRole('heading', { name: 'A sessão acabou' })).toBeVisible();
    await expect(card.getByRole('heading', { name: 'Seu resultado, Pensantus' })).toBeVisible();
    await expectScreenPasses(p, `Cartão "A sessão acabou" do jogador ${where}`);
    await card.getByRole('button', { name: 'Fechar' }).last().click();
    await expect(p.getByText('A sessão acabou.')).toBeVisible();
    await expectScreenPasses(p, `A sessão acabou, o aviso simples ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
}

test('o resumo da sessão passa no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-032'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanSessionSummaryScreens(browser, 'light', 1280);
});

test('o resumo da sessão passa no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-032'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanSessionSummaryScreens(browser, 'dark', 390);
});

test('o resumo da sessão passa no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-032'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await scanSessionSummaryScreens(browser, 'light', 320);
});

// The guided level-up (MR-040, E8-15): the sheet's block, every step and state of the page, the question
// before discarding, the blocked route, and the master's "O que mudou". Pensantus goes from Mago 3 to 4.
async function scanLevelUpScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const where = `${colorScheme === 'light' ? 'no tema claro' : 'no tema escuro'}, a ${width} px`;
  const height = width === 320 ? 568 : width === 1024 ? 768 : width < 768 ? 844 : 800;
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport: { width, height } });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport: { width, height } });
  const m = await master.newPage();
  const p = await player.newPage();
  const row = (name: string) => p.locator('.row__main, .row').filter({ hasText: name }).first();
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForLevelUp(m, p, `Acessibilidade subida ${Date.now()}`);
    campaignId = table.campaignId;
    const sheet = `/campanhas/${campaignId}/personagens/${table.characterId}`;

    // Not `open`: with a session open the page keeps its stream, so the network is never idle.
    await p.goto(sheet);
    await expect(p.getByRole('link', { name: 'Subir para o nível 4' })).toBeVisible();
    await expectScreenPasses(p, `A ficha que pode subir de nível ${where}`);

    await p.getByRole('link', { name: 'Subir para o nível 4' }).click();
    await expect(p.getByText('Passo 1 de 4 · Atributos')).toBeVisible();
    await expectScreenPasses(p, `Atributos, com a escolha faltando ${where}`);
    await row('Inteligência').click();
    await expect(p.getByText('18 → 20')).toBeVisible();
    await expectScreenPasses(p, `Atributos, Inteligência 20 ${where}`);
    await p.getByRole('button', { name: 'Cancelar' }).click();
    await expect(p.getByText('Descartar as escolhas?')).toBeVisible();
    await expectScreenPasses(p, `A pergunta de descartar ${where}`);
    await p.getByRole('button', { name: 'Continuar escolhendo' }).click();
    await p.getByRole('button', { name: 'Próximo' }).click();

    await expect(p.getByText('Passo 2 de 4 · Vida')).toBeVisible();
    await expectScreenPasses(p, `Vida, a média ${where}`);
    await p.locator('.dice-choice__card').filter({ hasText: 'Rolar 1d6' }).click();
    await expectScreenPasses(p, `Vida, rolar o dado ${where}`);
    await p.getByRole('button', { name: 'Digitar o resultado' }).click();
    await p.getByLabel(/Role 1d6/).fill('4');
    await expectScreenPasses(p, `Vida, o dado físico digitado ${where}`);
    await p.getByRole('button', { name: 'Confirmar 4' }).click();
    await expect(p.getByText('Dado físico: 4 no d6')).toBeVisible();
    await expectScreenPasses(p, `Vida, o resultado do dado ${where}`);
    await p.getByRole('button', { name: 'Próximo' }).click();

    await expect(p.getByText('Passo 3 de 4 · Magias')).toBeVisible();
    await expectScreenPasses(p, `Magias, com a escolha faltando ${where}`);
    await p.getByRole('button', { name: /Ver os outros \d+ truques/ }).click();
    await row('Prestidigitação').click();
    await p.getByLabel('Buscar magia').fill('nebuloso');
    await row('Passo Nebuloso').click();
    await p.getByLabel('Buscar magia').fill('espelhada');
    await row('Imagem Espelhada').click();
    await p.getByLabel('Buscar magia').fill('');
    await expectScreenPasses(p, `Magias, o livro completo e as preparadas faltando ${where}`);
    await p.getByRole('button', { name: 'Descrição de Passo Nebuloso' }).first().click();
    await expect(p.getByRole('dialog').first()).toBeVisible();
    await expectScreenPasses(p, `O "?" de uma magia do subir de nível ${where}`);
    await p.keyboard.press('Escape');
    const prepare = p.locator('#pick-prepared');
    await prepare.locator('.row__main').filter({ hasText: 'Passo Nebuloso' }).click();
    await prepare.locator('.row__main').filter({ hasText: 'Detectar Magia' }).click();
    await expectScreenPasses(p, `Magias, tudo escolhido ${where}`);
    await p.getByRole('button', { name: 'Próximo' }).click();

    await expect(p.getByText('Passo 4 de 4 · Resumo')).toBeVisible();
    await expectScreenPasses(p, `Resumo ${where}`);
    await p.getByRole('button', { name: 'Confirmar o nível 4' }).click();
    await expect(p.getByText('Pensantus subiu para o nível 4.').first()).toBeVisible();
    await expectScreenPasses(p, `A ficha depois de subir ${where}`);

    await p.goto(`${sheet}/subir-de-nivel`);
    await expect(p.getByRole('heading', { name: 'Ainda não dá para subir de nível' })).toBeVisible();
    await expectScreenPasses(p, `A rota sem a marca ${where}`);

    await m.goto(`/campanhas/${campaignId}`);
    await expect(m.getByRole('status').filter({ hasText: 'Pensantus subiu para o nível 4.' })).toBeVisible();
    await expectScreenPasses(m, `O mestre, o aviso da subida ${where}`);
    await m.getByRole('button', { name: 'O que mudou: Pensantus' }).click();
    await expect(m.getByRole('region', { name: 'O que Pensantus escolheu no nível 4' })).toBeVisible();
    await expectScreenPasses(m, `O mestre, "O que mudou" aberto ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
}

test('o subir de nível passa no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-040'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanLevelUpScreens(browser, 'light', 1280);
});

test('o subir de nível passa no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-040'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanLevelUpScreens(browser, 'dark', 390);
});

test('o subir de nível passa no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-040'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanLevelUpScreens(browser, 'dark', 1024);
});

test('o subir de nível passa no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-040'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanLevelUpScreens(browser, 'light', 320);
});

// Etapa 9, MR-037: the character's creatures. The sheet's "Criaturas" panel (empty, outside a
// session, with a creature), the cast sheet, the questions in place, the stat block, and the
// master's list with "Dar uma criatura" (a computer's dialog: not on a phone).
async function scanCreatureScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const height = width < 768 ? (width < 360 ? 568 : 844) : width === 1024 ? 768 : 800;
  const options = { colorScheme, viewport: { width, height } };
  const master = await newSignedInContext(browser, 'Mestre Teste', options);
  const player = await newSignedInContext(browser, 'Jogador Teste', options);
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const closed = await tableForCreatures(m, p, `Criaturas fechada ${Date.now()}`, false);
    await p.goto(`/campanhas/${closed.campaignId}/personagens/${closed.characterId}`);
    await expect(p.locator('app-creatures-panel').getByRole('heading', { name: 'Criaturas' })).toBeVisible();
    await expectScreenPasses(p, `Criaturas, fora de uma sessão ${where}`);

    const table = await tableForCreatures(m, p, `Criaturas ${Date.now()}`);
    campaignId = table.campaignId;
    const panel = p.locator('app-creatures-panel');
    await p.goto(`/campanhas/${campaignId}/personagens/${table.characterId}`);
    await expect(panel.getByRole('heading', { name: 'Criaturas' })).toBeVisible();
    await expectScreenPasses(p, `Criaturas, vazio ${where}`);

    await panel.getByRole('button', { name: 'Encontrar Familiar' }).click();
    const sheet = p.getByRole('dialog', { name: 'Encontrar Familiar' }).or(p.locator('mat-bottom-sheet-container'));
    await expect(sheet.getByText('Falta dar um nome ao familiar.')).toBeVisible();
    await expectScreenPasses(p, `Encontrar Familiar, faltando o nome ${where}`);
    await sheet.getByLabel('Nome do familiar').fill('Nanquim');
    await sheet.locator('label', { hasText: /Corvo/ }).click();
    await expect(sheet.getByText('Conjurar como ritual · 1 hora · sem gastar espaço')).toBeVisible();
    await expectScreenPasses(p, `Encontrar Familiar, pronto ${where}`);
    await sheet.getByRole('button', { name: 'Convocar Nanquim' }).click();
    await expect(panel.getByText('Nanquim chegou.')).toBeVisible();
    await expectScreenPasses(p, `Criaturas, com o Nanquim e o aviso ${where}`);

    await panel.getByRole('button', { name: 'Renomear' }).click();
    await expect(panel.getByLabel('Nome da criatura')).toBeFocused();
    await expectScreenPasses(p, `Criaturas, renomear ${where}`);
    await panel.getByRole('button', { name: 'Cancelar' }).click();
    await panel.getByRole('button', { name: 'Dispensar' }).click();
    await expect(panel.getByRole('button', { name: 'Voltar' })).toBeFocused();
    await expectScreenPasses(p, `Criaturas, dispensar pergunta ${where}`);
    await panel.getByRole('button', { name: 'Voltar' }).click();

    await panel.getByRole('link', { name: 'Ver a ficha do Nanquim' }).click();
    await expect(p.getByRole('heading', { name: 'Nanquim', level: 1 })).toBeVisible();
    await expectScreenPasses(p, `A ficha da criatura ${where}`);
    await p.getByRole('button', { name: 'Dispensar' }).click();
    await expect(p.getByRole('alertdialog', { name: 'Dispensar Nanquim?' })).toBeVisible();
    await expectScreenPasses(p, `A ficha da criatura, dispensar pergunta ${where}`);

    await m.goto(`/campanhas/${campaignId}`);
    const row = m.locator('app-character-creatures');
    await expect(row.locator('.item__name', { hasText: 'Nanquim' })).toBeVisible();
    await expectScreenPasses(m, `O mestre, a lista de personagens com a criatura ${where}`);
    if (width >= 768) {
      await row.getByRole('button', { name: 'Dar uma criatura a Pensantus' }).click();
      const dialog = m.getByRole('dialog', { name: 'Dar uma criatura a Pensantus' });
      await expect(dialog.getByText(/de 334 · em ordem de nome/)).toBeVisible();
      await expectScreenPasses(m, `Dar uma criatura, aberta ${where}`);
      await dialog.getByLabel('Nome, em português ou inglês').fill('ma');
      await dialog.getByLabel('Tipo').selectOption('beast');
      await dialog.getByLabel('Nível de desafio').selectOption('1/8');
      await expect(dialog.getByText('2 de 334 · em ordem de nome')).toBeVisible();
      await dialog.locator('label', { hasText: /Mastim/ }).click();
      await expect(dialog.getByText(/com os PV do livro \(5\)/)).toBeVisible();
      await expectScreenPasses(m, `Dar uma criatura, o Mastim escolhido ${where}`);
      await dialog.getByRole('button', { name: 'Dar Mastim a Pensantus' }).click();
      await expect(m.getByText('Mastim dado a Pensantus.')).toBeVisible();
      await expectScreenPasses(m, `O mestre, depois de dar ${where}`);
    }
    await row.getByRole('button', { name: 'Dispensar Nanquim' }).click();
    await expect(row.getByRole('alertdialog')).toBeVisible();
    await expectScreenPasses(m, `O mestre, dispensar pergunta ${where}`);
    await row.getByRole('button', { name: 'Voltar' }).click();

    await m.goto(`/campanhas/${campaignId}/personagens/${table.characterId}`);
    const mc = m.locator('app-creatures-panel app-creature-card').first();
    await mc.getByRole('button', { name: 'Corrigir PV' }).click();
    await expect(mc.getByLabel(/PV de /)).toBeFocused();
    await expectScreenPasses(m, `O mestre, corrigir os PV da criatura ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
}

test('as criaturas passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-037'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanCreatureScreens(browser, 'light', 1280);
});

test('as criaturas passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-037'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanCreatureScreens(browser, 'dark', 390);
});

test('as criaturas passam no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-037'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanCreatureScreens(browser, 'dark', 1024);
});

test('as criaturas passam no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-037'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanCreatureScreens(browser, 'light', 320);
});
