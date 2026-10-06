import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Browser, type Page } from '@playwright/test';

import { canvasJpeg, newCampaign, uploadThroughPicker } from './gallery-support';
import { saveDocumentRPC, tableWithDocumentParts } from './document-support';
import { expectAligned } from './layout';
import { endOpenSessionRPC, endSessionRPC, openSessionPage, startSessionRPC, tableWithPensantus } from './live-session-support';
import { canvasPng, createMapRPC, createPointRPC, placeTokenRPC, revealMapRPC, setCurrentMapRPC, tableForMaps, uploadImageRPC } from './maps-support';
import { adjustVitalsRPC, beginAttackCombatRPC, combatRPC, getEncounterRPC, endTurnOf, passTurnsTo, pensantusCasting, waitTurnLeaves, tableForCombat, toren, torenSheet } from './combat-support';
import { addActionRPC, cartActions, getOpenSceneRPC, openSceneRPC, rollSceneRPC, sceneActionIdsRPC, setAttemptsRPC, setShowDcRPC, tableForScenes } from './scene-support';
import { addClueRPC, cartClues, cartHooks, createNoteRPC } from './notes-support';
import { createCapitaoRPC, createMiraRPC, playedCombatRPC, putOnStageRPC, uploadPortrait } from './stage-support';
import { printRoute, tableForPrinting } from './print-support';
import { tableForLevelUp } from './levelup-support';
import { paintRPC, pickRadio, tapSquare } from './move-support';
import { beginFogCombat, moveTo, sessionRoute, tableForFog } from './fog-support';
import { beginCreatureCombat, hitAndApply, tableForCreatureCombat } from './creatures-combat-support';
import { authStatePath, callRPC, characterRpcBody, createCharacterRPC, newSignedInContext, pensantus } from './support';
import { beginJointCombat, endPartRPC, jointTable } from './joint-turn-support';
import { tableForCaster, tableForCreatures } from './creatures-support';
import { awardXpRPC, createEnemyRPC, tableForXp, tableForXpCombat, winCombatRPC } from './xp-support';
import { tableForGold, threeTreasuresRPC, treasureFoundRPC } from './gold-support';
import { movePensantus, pensantusFirst, sq20, trapRPC, treasureRPC } from './trap-support';
import { cavePoints, clickSquare, dragSquares, editorRoute, mapToPaint } from './editor-support';
import { campaignWithEmptyPlayer, factor, masterCampaign, method, setTableRulesRPC, wallSquares } from './table-rules-support';
import { archiveEntryRPC, createEntryRPC, entryRoute, raceBody, spellBody, updateEntryRPC } from './content-support';

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
    // The names show on the picture from 520 px of map (a phone's smaller one leaves them to this list).
    await expect(dialog.getByRole('list', { name: 'Pontos deste mapa' })).toContainText('Covil dos goblins');
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
    await expect(p.getByRole('alert').filter({ hasText: 'Longe demais' })).toBeVisible();
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

/** Moving by the circle, jumping, cover and the opportunity attacks (Etapa 9,
 * slice 9.15; E9-05, E9-06, E9-07, E9-13): the "Mover" page with nothing chosen,
 * with a cost and a warning, with a wall refused, "Saltar" (distance, then
 * height), the target list with its cover, the master's order with the mark open,
 * the master's prompt, the waiting mover, and the player's `alertdialog`. The
 * squares are chosen with the arrows under the map, which every width has. */
async function scanMoveScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: 900 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  const nudge = async (name: string, times = 1) => {
    for (let i = 0; i < times; i++) {
      await p.getByRole('button', { name }).click();
    }
  };
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForCombat(m, p, `Acessibilidade movimento ${Date.now()}`, true, true, { sheet: pensantusCasting });
    campaignId = table.campaignId;
    await paintRPC(m, table, 'MAP_LAYER_DIFFICULT_TERRAIN', 1, [[4, 7]]);
    await paintRPC(m, table, 'MAP_LAYER_WALL', 1, [[5, 5]]);
    await paintRPC(m, table, 'MAP_LAYER_COVER', 1, [[7, 8]]);
    await beginAttackCombatRPC(
      m,
      table,
      { Pensantus: 20, 'Goblin 1': 15, 'Capitão Goblin': 10, 'Goblin 2': 4 },
      { 'Capitão Goblin': [9, 9], 'Goblin 1': [6, 7], 'Goblin 2': [15, 11] },
    );

    // The player's turn: "Mover" with nothing chosen, then a square with a cost and the warning.
    await openSessionPage(p, campaignId);
    await openSessionPage(m, campaignId);
    await expect(p.getByRole('heading', { name: 'Sua vez, Pensantus' })).toBeVisible();
    await p.getByRole('button', { name: 'Mover', exact: true }).click();
    await expect(p.getByRole('heading', { name: 'Mover Pensantus' })).toBeVisible();
    await expectScreenPasses(p, `Mover, nada escolhido ${where}`);
    await nudge('Um quadrado para a esquerda');
    await expect(p.getByText('Mover 3,0 m', { exact: true })).toBeVisible();
    await expect(p.getByText('Sair do alcance do Goblin 1 pode provocar um ataque de oportunidade.')).toBeVisible();
    await expectScreenPasses(p, `Mover, custo e aviso de ataque de oportunidade ${where}`);
    await nudge('Um quadrado para a direita');
    await nudge('Um quadrado para cima', 2);
    await expect(p.getByRole('alert').filter({ hasText: 'Sem caminho reto' })).toBeVisible();
    await expectScreenPasses(p, `Mover, parede recusada ${where}`);

    // Saltar: the limits and the circle, then the height.
    await pickRadio(p, 'Saltar');
    await expect(p.getByRole('heading', { name: 'Saltar Pensantus' })).toBeVisible();
    await expectScreenPasses(p, `Saltar, distância ${where}`);
    await pickRadio(p, 'Altura');
    await expect(p.getByRole('button', { name: 'Aumentar a altura em 0,3 m' })).toBeVisible();
    await expectScreenPasses(p, `Saltar, altura ${where}`);
    await pickRadio(p, 'Andar');

    // The move that provokes: the turn waits for the master, who has the prompt.
    await nudge('Um quadrado para a esquerda');
    await p.getByRole('button', { name: 'Mover para cá' }).click();
    await expect(p.getByRole('status').filter({ hasText: 'Esperando a reação do mestre.' })).toBeVisible();
    await expectScreenPasses(p, `Sua vez, esperando a reação do mestre ${where}`);
    const card = m.getByRole('group', { name: 'Ataque de oportunidade de Goblin 1' });
    await expect(card.getByRole('button', { name: 'Não atacar' })).toBeFocused();
    await expectScreenPasses(m, `Ataque de oportunidade, a pergunta do mestre ${where}`);
    await card.getByRole('button', { name: 'Não atacar' }).click();
    await expect(card).toHaveCount(0);

    // Cover: the target list, and the master's order with the mark in place.
    await p.getByRole('button', { name: 'Atacar com Raio de Fogo' }).click();
    await expect(p.locator('label', { hasText: 'Capitão Goblin' })).toContainText('Meia cobertura (do mapa)');
    await expectScreenPasses(p, `Atacar, alvos com cobertura ${where}`);
    await p.getByRole('button', { name: 'Fechar' }).click();
    const order = m.getByRole('region', { name: 'Ordem de iniciativa' });
    await expect(order.getByText('Meia cobertura (do mapa) contra o Pensantus').first()).toBeVisible();
    await expectScreenPasses(m, `Ordem do mestre, com a cobertura contra quem tem a vez ${where}`);
    await order.getByRole('button', { name: 'Marcar cobertura de Capitão Goblin' }).click();
    await expect(order.getByRole('radiogroup', { name: 'Cobertura marcada de Capitão Goblin' })).toBeVisible();
    await expectScreenPasses(m, `Marcar cobertura, no lugar ${where}`);
    await pickRadio(order, 'Três quartos');
    await expect(order.getByText('Três quartos (marcada pelo mestre) contra o Pensantus')).toBeVisible();
    await order.getByRole('button', { name: 'Fechar' }).click();
    await order.getByRole('button', { name: 'Mais ações para Goblin 2' }).click();
    await m.getByRole('menuitem', { name: 'Marcar como aliado' }).click();
    await expect(order.getByText('Aliado')).toBeVisible();
    await expectScreenPasses(m, `Ordem do mestre, com "Aliado" e a marca ${where}`);

    // The player's `alertdialog`: Goblin 1 has the turn and leaves Pensantus's reach.
    await p.getByRole('button', { name: 'Encerrar turno' }).last().click();
    await p.getByRole('button', { name: 'Encerrar turno' }).last().click();
    await expect(m.getByText('Vez do Goblin 1')).toBeVisible();
    // It steps next to Pensantus (who moved away), then out of his reach again.
    const now = await getEncounterRPC(m, campaignId);
    const goblin = now.combatants.find((c) => c.label === 'Goblin 1')!.id;
    for (const col of [5, 9]) {
      await combatRPC(m, 'MoveCombatant', { campaignId, encounterId: now.id, combatantId: goblin, col, row: 7 });
    }
    const prompt = p.getByRole('alertdialog', { name: 'Ataque de oportunidade' });
    await expect(prompt.getByRole('button', { name: 'Não atacar' })).toBeFocused();
    await expectScreenPasses(p, `Ataque de oportunidade, o aviso do jogador ${where}`);
    await expectScreenPasses(m, `Ataque de oportunidade, esperando um jogador ${where}`);
    await prompt.getByRole('button', { name: 'Não atacar' }).click();
    await expect(prompt).toHaveCount(0);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
}

test('mover, saltar, a cobertura e o ataque de oportunidade passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-034'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanMoveScreens(browser, 'light', 1280);
});

test('mover, saltar, a cobertura e o ataque de oportunidade passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-034'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanMoveScreens(browser, 'dark', 390);
});

test('mover e a pergunta do ataque de oportunidade passam no axe no tema claro, no celular de 320', { tag: ['@a11y', '@MR-034'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanMoveScreens(browser, 'light', 320);
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
    await waitTurnLeaves(m, campaignId, 'Pensantus');
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
      await endTurnOf(p, m, campaignId, 'Pensantus');
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
      await endTurnOf(p, m, campaignId, 'Pensantus');
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

/**
 * "Voltar à cidade" and "Mais tesouro encontrado" (Etapa 9, MR-041, MR-032; E9-09): the Experiência panel of a
 * campaign by gold with its strip and the two buttons, "Dar XP" with the strip, the conversion dialog (everything
 * checked, a change, nothing checked, a refusal, nothing to convert), the history line and the question to undo,
 * the player's view, the campaign by enemies, and the session summary with the treasure block and the card.
 */
async function scanGoldScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: width >= 768 ? 900 : width <= 320 ? 568 : 844 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const m = await master.newPage();
  const p = await player.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  const campaigns: string[] = [];
  const panel = (page: Page) => page.getByRole('region', { name: 'Experiência', exact: true });
  try {
    await m.goto('/');
    await p.goto('/');

    // Nothing found yet: the strip invites, and the dialog explains.
    const gold = await tableForGold(m, p, `Acessibilidade ouro ${Date.now()}`);
    campaigns.push(gold.campaignId);
    await open(m, `/campanhas/${gold.campaignId}`);
    await expect(panel(m)).toContainText('Nenhum tesouro esperando.');
    await expectScreenPasses(m, `Experiência por ouro, nada esperando ${where}`);
    await panel(m).getByRole('button', { name: 'Voltar à cidade', exact: true }).click();
    const empty = m.getByRole('dialog', { name: 'Voltar à cidade' });
    await expect(empty).toContainText('Nenhum tesouro encontrado para converter.');
    await expectScreenPasses(m, `Voltar à cidade, nada para converter ${where}`);
    await empty.getByRole('button', { name: 'Fechar', exact: true }).last().click();

    // Three finds: the strip, the dialog and its calculation in each state.
    const ids = await threeTreasuresRPC(m, gold);
    await open(m, `/campanhas/${gold.campaignId}`);
    await expect(panel(m).getByText('3 tesouros · 420 PO')).toBeVisible();
    await expectScreenPasses(m, `Experiência por ouro, três tesouros esperando ${where}`);
    await panel(m).getByRole('button', { name: 'Voltar à cidade', exact: true }).click();
    const town = m.getByRole('dialog', { name: 'Voltar à cidade' });
    await expect(town).toContainText('420 XP ÷ 1 = 420 XP para cada');
    await expectScreenPasses(m, `Voltar à cidade, tudo marcado ${where}`);
    await town.getByRole('checkbox', { name: 'Converter Ídolo de prata' }).uncheck({ force: true });
    await expect(town).toContainText('370 XP ÷ 1 = 370 XP para cada');
    await expectScreenPasses(m, `Voltar à cidade, sem um tesouro ${where}`);
    await town.getByRole('checkbox', { name: 'Marcar Pensantus' }).uncheck({ force: true });
    await expect(town).toContainText('Marque pelo menos um tesouro e um personagem.');
    await expectScreenPasses(m, `Voltar à cidade, ninguém marcado ${where}`);
    await town.getByRole('button', { name: 'Cancelar' }).click();

    // Converted meanwhile: the refusal stays in the dialog.
    await panel(m).getByRole('button', { name: 'Voltar à cidade', exact: true }).click();
    await expect(m.getByRole('dialog', { name: 'Voltar à cidade' })).toContainText('420 XP ÷ 1 = 420 XP para cada');
    const other = await callRPC(m, 'meurpg.progression.v1.ProgressionService/AwardXP', {
      campaignId: gold.campaignId,
      mode: 'XP_AWARD_MODE_GOLD',
      reason: 'Voltar à cidade',
      characterIds: gold.characterIds,
      treasurePointIds: [ids[0]],
      idempotencyKey: crypto.randomUUID(),
    });
    expect(other.ok(), await other.text()).toBeTruthy();
    await m.getByRole('dialog', { name: 'Voltar à cidade' }).getByRole('button', { name: /^Dar 420 XP/ }).click();
    await expect(m.getByRole('dialog', { name: 'Voltar à cidade' }).getByRole('alert')).toContainText('já virou XP em outro prêmio');
    await expectScreenPasses(m, `Voltar à cidade, o erro no lugar ${where}`);
    await m.getByRole('dialog', { name: 'Voltar à cidade' }).getByRole('button', { name: /^Dar 170 XP/ }).click();
    await expect(panel(m).getByRole('status').filter({ hasText: 'foram convertidos' })).toContainText('Os 2 tesouros foram convertidos.');
    await expect(panel(m).getByText('Voltar à cidade · 170 PO em 2 tesouros')).toBeVisible();
    await expectScreenPasses(m, `Experiência, depois de converter ${where}`);
    await panel(m).getByRole('button', { name: /^Desfazer/ }).click();
    await expect(panel(m).getByRole('alertdialog')).toContainText('voltam a “encontrado, não convertido”');
    await expectScreenPasses(m, `Experiência, desfazer a conversão ${where}`);
    await panel(m).getByRole('alertdialog').getByRole('button', { name: 'Desfazer XP' }).click();
    await expect(panel(m).getByRole('status').filter({ hasText: 'XP desfeito' })).toBeVisible();
    await expectScreenPasses(m, `Experiência, conversão desfeita ${where}`);

    // "Dar XP" has the strip too, and its button.
    await panel(m).getByRole('button', { name: 'Dar XP' }).click();
    const give = m.getByRole('dialog', { name: 'Dar XP' });
    await expect(give).toContainText('ou digite o ouro');
    await expectScreenPasses(m, `Dar XP por ouro com os tesouros ${where}`);
    await give.getByRole('button', { name: 'Cancelar' }).click();

    // The player: the history line and the strip's absence.
    await panel(m).getByRole('button', { name: 'Voltar à cidade', exact: true }).click();
    await m.getByRole('dialog', { name: 'Voltar à cidade' }).getByRole('button', { name: /^Dar 170 XP/ }).click();
    await expect(panel(m).getByRole('status').filter({ hasText: 'foram convertidos' })).toContainText('Os 2 tesouros foram convertidos.');
    await open(p, `/campanhas/${gold.campaignId}`);
    await expect(panel(p)).toContainText('Voltar à cidade · 170 PO em 2 tesouros');
    await expectScreenPasses(p, `Experiência por ouro, jogador ${where}`);

    // A campaign by enemies: no button, the line why.
    const enemies = await tableForGold(m, p, `Acessibilidade inimigos ${Date.now()}`, 'XP_MODE_ENEMIES');
    campaigns.push(enemies.campaignId);
    await treasureFoundRPC(m, enemies, { name: 'Baú de moedas', valuePo: 250, finders: [enemies.characterIds[0]] });
    await open(m, `/campanhas/${enemies.campaignId}`);
    await expect(panel(m)).toContainText('Esta campanha dá XP por inimigos, então o tesouro não vira XP.');
    await expectScreenPasses(m, `Experiência por inimigos, com tesouro ${where}`);

    // "Dar XP" of the live session (state 6c): the strip with the way in, then the conversion from it.
    const live = await tableForGold(m, p, `Acessibilidade ouro sessão ${Date.now()}`);
    campaigns.push(live.campaignId);
    await threeTreasuresRPC(m, live);
    await startSessionRPC(m, live.campaignId);
    await openSessionPage(m, live.campaignId);
    await m.getByRole('button', { name: 'Dar XP', exact: true }).click();
    const liveGive = m.getByRole('dialog', { name: 'Dar XP' });
    await expect(liveGive).toContainText('3 tesouros · 420 PO');
    await expectScreenPasses(m, `Dar XP da sessão com a faixa de tesouros ${where}`);
    await liveGive.getByRole('button', { name: 'Voltar à cidade' }).click();
    await expect(m.getByRole('dialog', { name: 'Voltar à cidade' })).toContainText('420 XP ÷ 1 = 420 XP para cada');
    await expectScreenPasses(m, `Voltar à cidade aberto do Dar XP da sessão ${where}`);
    await m.getByRole('dialog', { name: 'Voltar à cidade' }).getByRole('button', { name: 'Cancelar' }).click();
    await endOpenSessionRPC(m, live.campaignId);

    // The session summary: the master's block and the player's card.
    const summary = await tableForGold(m, p, `Acessibilidade resumo ouro ${Date.now()}`, 'XP_MODE_ENEMIES');
    campaigns.push(summary.campaignId);
    await startSessionRPC(m, summary.campaignId);
    await threeTreasuresRPC(m, summary);
    await openSessionPage(p, summary.campaignId);
    await openSessionPage(m, summary.campaignId);
    await m.getByRole('button', { name: 'Encerrar sessão' }).click();
    await m.getByRole('button', { name: 'Confirmar encerramento' }).click();
    await expect(m.getByRole('heading', { name: 'Sessão encerrada' })).toBeVisible();
    await expect(m.getByRole('table', { name: 'Mais tesouro encontrado' })).toBeVisible();
    await expectScreenPasses(m, `Resumo da sessão com Mais tesouro encontrado, mestre ${where}`);
    const card = p.getByRole('region', { name: 'Resumo da sessão' });
    await expect(card.getByRole('heading', { name: 'A sessão acabou' })).toBeVisible();
    await expect(card).toContainText('Mais tesouro encontrado');
    await expectScreenPasses(p, `Cartão com Mais tesouro encontrado, jogador ${where}`);
  } finally {
    for (const id of campaigns) {
      await endOpenSessionRPC(m, id);
    }
    await master.close();
    await player.close();
  }
}

test('"Voltar à cidade" e o tesouro no resumo passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-041', '@MR-032'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanGoldScreens(browser, 'light', 1280);
});

test('"Voltar à cidade" e o tesouro no resumo passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-041', '@MR-032'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanGoldScreens(browser, 'dark', 390);
});

test('"Voltar à cidade" e o tesouro no resumo passam no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-041', '@MR-032'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanGoldScreens(browser, 'dark', 1024);
});

test('"Voltar à cidade" e o tesouro no resumo passam no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-041', '@MR-032'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanGoldScreens(browser, 'light', 320);
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
    await expect(sheet.getByText('Escolha a forma e dê um nome ao familiar.')).toBeVisible();
    await expectScreenPasses(p, `Encontrar Familiar, faltando o nome ${where}`);
    await sheet.getByLabel('Nome do familiar').fill('Nanquim');
    await sheet.locator('label', { hasText: /Corvo/ }).click();
    await expect(sheet.getByText('Conjurar como ritual · 1 hora · sem gastar espaço')).toBeVisible();
    await expectScreenPasses(p, `Encontrar Familiar, pronto ${where}`);
    await sheet.getByRole('button', { name: 'Convocar o familiar' }).click();
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

    await panel.getByRole('link', { name: 'Ver a ficha de Nanquim' }).click();
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

    // A creature that is not there (or that the viewer may not read): the page's not-found state.
    await p.goto(`/campanhas/${campaignId}/personagens/${table.characterId}/criaturas/6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001`);
    await expect(p.getByRole('heading', { name: 'Criatura não encontrada', level: 1 })).toBeVisible();
    await expectScreenPasses(p, `A ficha da criatura, não encontrada ${where}`);

    // A druid: the slot picker, "Quantas criaturas", the list with "−" and "+", the warning of what a new
    // concentration ends, and a refusal of the server inside the sheet.
    const druid = await tableForCaster(m, p, `Criaturas druida ${Date.now()}`, 'druid');
    const druidCampaign = druid.campaignId;
    try {
      await p.goto(`/campanhas/${druidCampaign}/personagens/${druid.characterId}`);
      const dpanel = p.locator('app-creatures-panel');
      await dpanel.getByRole('button', { name: 'Conjurar Animais' }).click();
      const cast = p.getByRole('dialog', { name: 'Conjurar Animais' }).or(p.locator('mat-bottom-sheet-container'));
      await expect(cast.getByText('Quantas criaturas')).toBeVisible();
      await expectScreenPasses(p, `Conjurar Animais, as opções e o espaço ${where}`);
      await cast.locator('label', { hasText: /2\s+feras\s+de\s+ND\s+1\s/ }).click();
      // A row not chosen yet has only "Escolher"; once chosen it has "Menos" and "Mais".
      await cast.getByRole('button', { name: 'Escolher Lobo', exact: true }).click();
      await cast.getByRole('button', { name: 'Mais Lobo', exact: true }).click();
      await expect(cast.locator('.line')).toContainText('2 criaturas · 1 ação · gasta um espaço de 3º círculo');
      await expectScreenPasses(p, `Conjurar Animais, a mistura pronta ${where}`);
      await cast.getByRole('button', { name: 'Conjurar Animais', exact: true }).click();
      await expect(dpanel.getByText('2 criaturas chegaram.')).toBeVisible();

      await dpanel.getByRole('button', { name: 'Conjurar Animais' }).click();
      await expect(cast.getByText('Isso encerra Conjurar Animais e dispensa 2 criaturas')).toBeVisible();
      await expectScreenPasses(p, `Conjurar Animais, o aviso do que a concentração encerra ${where}`);
      // The session ends while the sheet is open: the server refuses, and the sheet says so, still open.
      await cast.locator('label', { hasText: /1\s+fera\s+de\s+ND\s+2\s/ }).click();
      await cast.locator('app-creature-choice-list label.row').first().click();
      await endOpenSessionRPC(m, druidCampaign);
      await cast.getByRole('button', { name: 'Conjurar Animais', exact: true }).click();
      await expect(cast.getByRole('alert')).toContainText('A sessão acabou');
      await expectScreenPasses(p, `Conjurar Animais, a recusa do servidor na folha ${where}`);
    } finally {
      await endOpenSessionRPC(m, druidCampaign);
    }
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

// The fog of war (Etapa 9, MR-036, E9-03 and E9-04): the player's map with its legend and caption, the tiles on their way,
// what was seen before, the character who is not on the map, the carried light (the row, the sheet, the toast), the
// master's "Ver como" and "Luz dos personagens", and the familiar's eyes (the band, in and out of a combat).
async function scanFogScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: width < 700 ? 800 : 900 };
  const contexts = await Promise.all(
    (['Mestre Teste', 'Jogador Teste', 'E-mail Não Verificado'] as const).map((user) =>
      browser.newContext({ storageState: authStatePath(user), colorScheme, viewport }),
    ),
  );
  const [m, p, t] = await Promise.all(contexts.map((c) => c.newPage()));
  const where = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  const loaded = async (page: Page) => {
    await expect(page.locator('app-fog-base')).toBeVisible();
    await expect(page.getByTestId('fog-loading')).toHaveCount(0);
  };
  try {
    await Promise.all([m.goto('/'), p.goto('/'), t.goto('/')]);
    const table = await tableForFog(m, p, t, `Acessibilidade névoa ${Date.now()}`, { familiar: { col: 15, row: 8 }, torenOffMap: true });
    campaignId = table.campaignId;

    // The player: the map, the legend, the caption, the row of the light; the party's chips on a phone.
    await p.goto(sessionRoute(campaignId));
    await loaded(p);
    await expectScreenPasses(p, `A névoa, a vista do jogador ${where}`);

    // The tiles on their way: still stripes with the dashed border, and the notice.
    await p.route('**/tiles/**', async (route) => {
      await new Promise((r) => setTimeout(r, 4000));
      await route.continue().catch(() => undefined);
    });
    await p.reload();
    await expect(p.getByTestId('fog-loading')).toBeVisible();
    await expectScreenPasses(p, `A névoa, carregando o mapa ${where}`);
    await p.unrouteAll({ behavior: 'ignoreErrors' });
    await loaded(p);

    // What was seen stays, darkened.
    await moveTo(m, table, table.pensantusId, 10, 13);
    await expect(p.locator('.mr-legend').getByText('Já visto', { exact: true })).toBeVisible();
    await expectScreenPasses(p, `A névoa, o que já foi visto ${where}`);

    // A character who is not on the map (Toren): the fixed notice with its icon.
    await t.goto(sessionRoute(campaignId));
    await expect(t.getByTestId('fog-off-map')).toBeVisible();
    await expectScreenPasses(t, `A névoa, o personagem fora do mapa ${where}`);

    // The carried light: the sheet, then the toast.
    await p.getByRole('button', { name: /Luz que você carrega/ }).click();
    const sheet = p.getByRole('dialog', { name: 'Luz que você carrega' });
    await expect(sheet).toBeVisible();
    await expectScreenPasses(p, `Luz que você carrega, a folha ${where}`);
    await sheet.getByText('Tocha', { exact: true }).click();
    await expect(sheet.getByRole('radio', { name: /Tocha/ })).toBeChecked();
    await sheet.getByRole('button', { name: 'Pronto' }).click();
    await expect(p.getByRole('status').filter({ hasText: 'Você acendeu a tocha' })).toBeVisible();
    await expectScreenPasses(p, `Luz que você carrega, a tocha acesa ${where}`);

    // The master: "Ver como" and "Luz dos personagens", then the map as Pensantus sees it.
    await m.goto(sessionRoute(campaignId));
    const list = m.getByRole('radiogroup', { name: 'Ver como' });
    await expect(list.getByRole('radio', { name: /Pensantus/ })).toContainText(/\d+\s+quadrados vistos/);
    await expectScreenPasses(m, `Ver como, a lista e a luz dos personagens ${where}`);
    // Nothing sticks out of the page, whatever its width (the "Ver como" and light cards at 320 px did).
    expect(await m.evaluate(() => document.documentElement.scrollWidth - window.innerWidth), `rolagem lateral do mestre ${where}`).toBeLessThanOrEqual(0);
    expect(await p.evaluate(() => document.documentElement.scrollWidth - window.innerWidth), `rolagem lateral do jogador ${where}`).toBeLessThanOrEqual(0);
    await list.getByRole('radio', { name: /Pensantus/ }).click();
    await expect(m.getByText('Você está vendo o mapa como Pensantus.', { exact: false })).toBeVisible();
    await loaded(m);
    await expectScreenPasses(m, `Ver como, o mapa de Pensantus ${where}`);

    // The familiar's eyes, out of a combat: the row under the legend, the question, then the band with the one filled button.
    const familiar = p.getByRole('region', { name: 'Seu familiar Nanquim' });
    await expect(familiar).toBeVisible();
    await expectScreenPasses(p, `Pelos olhos do Nanquim, a linha do familiar ${where}`);
    await familiar.getByRole('button', { name: 'Ver pelos olhos' }).click();
    const question = p.getByRole('dialog', { name: 'Ver pelos olhos do Nanquim?' });
    await expect(question).toBeVisible();
    await expectScreenPasses(p, `Pelos olhos do Nanquim, a pergunta ${where}`);
    await question.getByRole('button', { name: 'Ver pelos olhos' }).click();
    await expect(p.getByTestId('familiar-band')).toBeVisible();
    await expectScreenPasses(p, `Pelos olhos do Nanquim, a faixa ${where}`);
    await p.getByRole('button', { name: 'Voltar aos seus olhos' }).click();
    await expect(p.getByTestId('familiar-band')).toHaveCount(0);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await Promise.all(contexts.map((c) => c.close()));
  }
}

test('a névoa de guerra passa no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-036'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanFogScreens(browser, 'light', 1280);
});

test('a névoa de guerra passa no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-036'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanFogScreens(browser, 'dark', 390);
});

test('a névoa de guerra passa no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-036'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanFogScreens(browser, 'dark', 1024);
});

test('a névoa de guerra passa no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-036'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanFogScreens(browser, 'light', 320);
});


// The fog in a combat (Etapa 9, MR-036): the combat's map with the same fog and legend, the action "Ver pelos olhos do Nanquim", the band
// "Até o começo da sua próxima vez" with the "Cego" line, and the master's "Ver como" and "Luz dos personagens" under the combat.
async function scanFogCombatScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: width < 700 ? 800 : 900 };
  const contexts = await Promise.all(
    (['Mestre Teste', 'Jogador Teste', 'E-mail Não Verificado'] as const).map((user) =>
      browser.newContext({ storageState: authStatePath(user), colorScheme, viewport }),
    ),
  );
  const [m, p, t] = await Promise.all(contexts.map((c) => c.newPage()));
  const where = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  try {
    await Promise.all([m.goto('/'), p.goto('/'), t.goto('/')]);
    const table = await tableForFog(m, p, t, `Acessibilidade névoa no combate ${Date.now()}`, { familiar: { col: 15, row: 8 } });
    campaignId = table.campaignId;
    await beginFogCombat(m, table);

    await p.goto(sessionRoute(campaignId));
    await expect(p.getByRole('heading', { name: /Sua vez, Pensantus/ })).toBeVisible();
    await expect(p.locator('app-fog-base')).toBeVisible();
    await expect(p.getByTestId('fog-loading')).toHaveCount(0);
    await expectScreenPasses(p, `Combate na névoa, o mapa e a luz ${where}`);

    const action = p.locator('app-action-row', { hasText: 'Ver pelos olhos do Nanquim' });
    await expect(action).toBeVisible();
    await expectScreenPasses(p, `Combate na névoa, a ação "Ver pelos olhos do Nanquim" ${where}`);
    await action.getByRole('button', { name: 'Ver pelos olhos do Nanquim' }).click();
    const question = p.getByRole('dialog', { name: 'Ver pelos olhos do Nanquim?' });
    await expect(question).toBeVisible();
    await expectScreenPasses(p, `Combate na névoa, a pergunta com o custo da ação ${where}`);
    await question.getByRole('button', { name: 'Ver pelos olhos' }).click();
    await expect(p.getByTestId('familiar-band')).toBeVisible();
    await expect(p.getByTestId('familiar-blind')).toBeVisible();
    await expectScreenPasses(p, `Combate na névoa, a faixa e a linha "Cego" ${where}`);

    await m.goto(sessionRoute(campaignId));
    await expect(m.getByRole('radiogroup', { name: 'Ver como' })).toBeVisible();
    await m.getByRole('radio', { name: /Toren/ }).click();
    await expect(m.locator('app-view-as-map')).toBeVisible();
    await expect(m.getByTestId('fog-loading')).toHaveCount(0);
    await expectScreenPasses(m, `Combate na névoa, o mestre vendo como Toren ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await Promise.all(contexts.map((c) => c.close()));
  }
}

test('a névoa no combate passa no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-036'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanFogCombatScreens(browser, 'light', 1280);
});

test('a névoa no combate passa no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-036'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanFogCombatScreens(browser, 'dark', 390);
});

// Traps and treasure in the session (slice 9.14, MR-035, MR-041, E9-08, E9-09): the master's cards and
// their dialogs, the damage that waits, the player's search sheet in each step, the toast and the treasure's
// sheet. Every state goes through axe and the alignment checks.
async function scanTrapScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
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
    const table = await tableForCombat(m, p, `Acessibilidade armadilhas ${Date.now()}`, true, true);
    campaignId = table.campaignId;
    await trapRPC(m, table, 'Fosso escondido', 6, 7, { noticeDc: 30, damage: '3' });
    await trapRPC(m, table, 'Agulha envenenada', 15, 10, { manual: true, noticeDc: 0, findDc: 20 });
    await trapRPC(m, table, 'Fosso fundo', 8, 7, { noticeDc: 30, damage: '3' });
    await treasureRPC(m, table, 'Baú de moedas', 12, 7);
    await openSessionPage(m, campaignId);
    await openSessionPage(p, campaignId);

    // The master's cards, the reveal and the fire dialogs.
    const panel = m.getByRole('region', { name: 'Armadilhas do mapa' });
    const card = panel.getByRole('article', { name: 'Fosso escondido' });
    await expect(card).toContainText('Quem notaria');
    await expectScreenPasses(m, `Armadilhas do mapa, o cartão aberto ${where}`);
    await card.getByRole('button', { name: 'Revelar para…' }).click();
    await expect(m.getByRole('dialog', { name: 'Revelar armadilha' })).toBeVisible();
    await expectScreenPasses(m, `Revelar armadilha ${where}`);
    await m.getByRole('dialog', { name: 'Revelar armadilha' }).getByRole('button', { name: 'Cancelar' }).click();
    await card.getByRole('button', { name: 'Disparar…' }).click();
    await expect(m.getByRole('dialog', { name: /Disparar/ })).toBeVisible();
    await expectScreenPasses(m, `Disparar a armadilha ${where}`);
    await m.getByRole('dialog', { name: /Disparar/ }).getByRole('button', { name: 'Disparar para quem está na área' }).click();
    await expect(card).toContainText('Disparada');
    await expectScreenPasses(m, `Armadilha disparada e o registro ${where}`);

    // The treasure: hidden, the form in place, found, the question in place.
    const chest = m.getByRole('region', { name: 'Tesouros do mapa' }).getByRole('article', { name: 'Baú de moedas' });
    await expectScreenPasses(m, `Tesouros do mapa, escondido ${where}`);
    await chest.getByRole('button', { name: 'Marcar o Baú de moedas como encontrado' }).click();
    await expectScreenPasses(m, `Marcar como encontrado no lugar ${where}`);
    await chest.locator('label', { hasText: 'Pensantus' }).click();
    await chest.getByRole('button', { name: 'Marcar como encontrado' }).click();
    await expect(chest).toContainText('Encontrado por Pensantus');
    // The toast goes away on its own: scan the player's screen before the master's.
    await expect(p.getByRole('status').filter({ hasText: 'Pensantus encontrou: Baú de moedas' })).toBeVisible();
    await expectScreenPasses(p, `Aviso do tesouro achado, jogador ${where}`);
    await expectScreenPasses(m, `Tesouro achado ${where}`);
    await chest.getByRole('button', { name: 'Desmarcar' }).click();
    await expectScreenPasses(m, `Desmarcar no lugar ${where}`);
    await chest.getByRole('alertdialog').getByRole('button', { name: 'Voltar' }).click();

    // The player: the toast, the row and sheet of the treasure, and the search in each step.
    await p.getByRole('button', { name: /Baú de moedas, Tesouro/ }).click();
    await expect(p.getByRole('dialog', { name: 'Baú de moedas' })).toBeVisible();
    await expectScreenPasses(p, `Folha do tesouro, jogador ${where}`);
    await p.getByRole('dialog', { name: 'Baú de moedas' }).getByRole('button', { name: 'Fechar', exact: true }).last().click();
    await p.getByRole('button', { name: 'Procurar armadilhas' }).click();
    const sheet = p.getByRole('dialog', { name: 'Procurar armadilhas' });
    await expectScreenPasses(p, `Procurar armadilhas, como ${where}`);
    await sheet.getByRole('button', { name: 'Digitar o resultado' }).click();
    await sheet.getByLabel(/Role 1d20 para/).fill('2');
    await expectScreenPasses(p, `Procurar armadilhas, o dado ${where}`);
    await sheet.getByRole('button', { name: /Confirmar 2/ }).click();
    await expect(sheet).toContainText(/Você (achou|não encontrou)/);
    await expectScreenPasses(p, `Procurar armadilhas, o resultado ${where}`);
    await sheet.getByRole('button', { name: 'Fechar', exact: true }).last().click();

    // In a combat: the damage that waits for the master, and the player's note.
    await pensantusFirst(m, table);
    await movePensantus(p, table, 9, 7);
    await openSessionPage(m, campaignId);
    await expectScreenPasses(m, `Combate com o dano de armadilha esperando ${where}`);
    await expectScreenPasses(p, `Combate, a nota da queda, jogador ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
}

test('as armadilhas e os tesouros na sessão passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-035', '@MR-041'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanTrapScreens(browser, 'light', 1280);
});

test('as armadilhas e os tesouros na sessão passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-035', '@MR-041'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanTrapScreens(browser, 'dark', 390);
});

test('as armadilhas e os tesouros na sessão passam no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-035', '@MR-041'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanTrapScreens(browser, 'dark', 1024);
});

test('as armadilhas e os tesouros na sessão passam no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-035', '@MR-041'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await scanTrapScreens(browser, 'light', 320);
});

// ---- the creatures in combat and Wild Shape (MR-037, E9-11, E9-12) ----

/** Taps a button of the page on a phone, where the pinned turn bar can cover half the screen: it is brought up under the app bar first. */
async function tapAboveBar(button: import('@playwright/test').Locator): Promise<void> {
  await button.evaluate((el) => {
    el.scrollIntoView({ block: 'start' });
    window.scrollBy(0, -140);
  });
  await button.click();
}

async function scanCreatureCombatScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: width < 700 ? 800 : 900 };
  const contexts = await Promise.all(
    (['Mestre Teste', 'Jogador Teste', 'E-mail Não Verificado'] as const).map((user) =>
      browser.newContext({ storageState: authStatePath(user), colorScheme, viewport }),
    ),
  );
  const [m, p, t] = await Promise.all(contexts.map((c) => c.newPage()));
  const where = `(${colorScheme}, ${width}px)`;
  let campaignId = '';
  try {
    await Promise.all([m.goto('/'), p.goto('/'), t.goto('/')]);
    const table = await tableForCreatureCombat(m, p, t, `Acessibilidade criaturas ${Date.now()}`);
    campaignId = table.campaignId;
    await beginCreatureCombat(m, table, { Sálvia: p, Toren: t }, { Toren: 20, Sálvia: 13, 'Capitão Goblin': 5, 'Goblin 1': 4, 'Goblin 2': 4 }, { 'Capitão Goblin': [21, 3], 'Goblin 1': [6, 6], 'Goblin 2': [20, 7] });
    await passTurnsTo(m, campaignId, 'Sálvia');
    await p.goto(sessionRoute(campaignId));
    await expect(p.getByRole('heading', { name: 'Sua vez, Sálvia' })).toBeVisible();
    await expectScreenPasses(p, `A vez da Sálvia, com Forma Selvagem ${where}`);

    // Wild Shape: the list of beasts, with one chosen.
    await tapAboveBar(p.getByRole('button', { name: 'Transformar: Forma Selvagem' }));
    const wild = p.getByRole('dialog', { name: 'Forma Selvagem' });
    await expect(wild.getByText('Escolha uma fera.')).toBeVisible();
    await expectScreenPasses(p, `Forma Selvagem, a lista ${where}`);
    await wild.getByLabel('Buscar fera').fill('lobo');
    await wild.locator('label', { hasText: '(Wolf)' }).click();
    await expect(wild.locator('.row__sub').first()).toContainText('CA 13');
    await expectScreenPasses(p, `Forma Selvagem, a fera escolhida ${where}`);
    await wild.getByRole('button', { name: 'Virar Lobo' }).click();
    await expect(p.getByText('Na forma de Lobo').first()).toBeVisible();
    await expectScreenPasses(p, `A vez como Lobo ${where}`);

    // The beast falls: the notice that stays.
    await hitAndApply(m, campaignId, 'Goblin 1', 'Sálvia', 30);
    await expect(p.getByTestId('form-ended')).toBeVisible();
    await expectScreenPasses(p, `O aviso da fera que caiu ${where}`);
    await p.getByTestId('form-ended').getByRole('button', { name: 'Entendi' }).click();
    // Next round: her action is free again.
    await endTurnOf(p, m, campaignId, 'Sálvia');
    await passTurnsTo(m, campaignId, 'Sálvia');
    await expect(p.getByRole('heading', { name: 'Sua vez, Sálvia' })).toBeVisible();

    // Conjurar Animais: the sheet, then the result.
    await tapAboveBar(p.getByRole('button', { name: 'Conjurar Conjurar Animais' }));
    // The dialog's name is its title, which changes to "Lobos atrozes conjurados" with the result.
    const sheet = p.getByRole('dialog');
    await sheet.getByText('2 feras de ND 1 ou menos').click();
    await sheet.getByRole('button', { name: 'Escolher Lobo atroz' }).click();
    await sheet.getByRole('button', { name: 'Mais Lobo atroz' }).click();
    await expectScreenPasses(p, `Conjurar Animais em combate ${where}`);
    await sheet.getByRole('button', { name: 'Digitar o d20 de um dado físico' }).click();
    await sheet.getByLabel('Role 1d20 para a iniciativa das criaturas').fill('8');
    await expectScreenPasses(p, `Conjurar Animais, o d20 digitado ${where}`);
    await sheet.getByRole('button', { name: 'Conjurar Animais', exact: true }).click();
    await expect(sheet.getByRole('heading', { name: 'Lobos atrozes conjurados' })).toBeVisible();
    await expectScreenPasses(p, `Os Lobos atrozes conjurados ${where}`);
    await sheet.getByRole('button', { name: 'Fechar' }).last().click();
    await expect(p.getByRole('tablist', { name: 'O que você joga' })).toBeVisible();
    await expectScreenPasses(p, `As abas, a vez da Sálvia ${where}`);

    // The wolves' turn, and what Toren sees.
    await endTurnOf(p, m, campaignId, 'Sálvia');
    await expect(p.getByRole('heading', { name: 'Vez dos seus Lobos atrozes' })).toBeVisible();
    await expectScreenPasses(p, `A vez dos Lobos atrozes ${where}`);
    await p.getByRole('button', { name: 'Encerrar a parte dos Lobos' }).click();
    await expect(p.getByRole('alertdialog')).toBeVisible();
    await expectScreenPasses(p, `Encerrar a parte dos Lobos, a pergunta ${where}`);
    await p.getByRole('alertdialog').getByRole('button', { name: 'Voltar' }).click();
    await t.goto(sessionRoute(campaignId));
    await expect(t.getByRole('heading', { name: 'Vez dos Lobos atrozes da Sálvia' })).toBeVisible();
    await expectScreenPasses(t, `A vez dos Lobos atrozes, vista por outro jogador ${where}`);

    // The master: the order with the group box, the legend and the concentration question.
    await m.goto(sessionRoute(campaignId));
    const order = m.getByRole('region', { name: 'Ordem de iniciativa' });
    await expect(order.getByText('Concentra em Conjurar Animais · 2 Lobos atrozes')).toBeVisible();
    await expectScreenPasses(m, `A ordem do mestre com as criaturas ${where}`);
    await order.getByRole('button', { name: 'Perdeu a concentração' }).click();
    await expect(order.getByRole('alertdialog')).toBeVisible();
    await expectScreenPasses(m, `Perdeu a concentração, a pergunta ${where}`);
    await order.getByRole('alertdialog').getByRole('button', { name: 'Dispensar os Lobos' }).click();
    await expect(p.getByTestId('concentration-lost')).toBeVisible();
    await expectScreenPasses(p, `O aviso da concentração perdida ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await Promise.all(contexts.map((c) => c.close()));
  }
}

test('as criaturas no combate e a Forma Selvagem passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-037'] }, async ({ browser }) => {
  test.setTimeout(360_000);
  await scanCreatureCombatScreens(browser, 'light', 1280);
});

test('as criaturas no combate e a Forma Selvagem passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-037'] }, async ({ browser }) => {
  test.setTimeout(360_000);
  await scanCreatureCombatScreens(browser, 'dark', 390);
});

test('as criaturas no combate e a Forma Selvagem passam no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-037'] }, async ({ browser }) => {
  test.setTimeout(360_000);
  await scanCreatureCombatScreens(browser, 'dark', 1024);
});

test('as criaturas no combate e a Forma Selvagem passam no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-037'] }, async ({ browser }) => {
  test.setTimeout(360_000);
  await scanCreatureCombatScreens(browser, 'light', 320);
});

// The map editor (slice 9.12, MR-034, MR-035, MR-036, MR-041, E9-01 and E9-02): painting with each tool, the in-place questions, the
// fog's settings, the Luz, Armadilha and Tesouro panels in each state, the map with no grid, with a combat on it, "Ver como", and,
// on a phone, the manage view with its fixed notice. Every state goes through axe and the alignment checks.
async function scanMapEditorScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: width >= 768 ? 900 : width <= 320 ? 568 : 844 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const pensantus = await newSignedInContext(browser, 'Jogador Teste');
  const toren = await newSignedInContext(browser, 'E-mail Não Verificado');
  const [m, ap, bp] = [await master.newPage(), await pensantus.newPage(), await toren.newPage()];
  const where = `(${colorScheme}, ${width}px)`;
  const phone = width < 768;
  let campaignId = '';
  try {
    await Promise.all([m.goto('/'), ap.goto('/'), bp.goto('/')]);
    const table = await tableForFog(m, ap, bp, `Acessibilidade editor ${Date.now()}`, {});
    campaignId = table.campaignId;
    await cavePoints(m, table);
    const image = await uploadImageRPC(m, campaignId, 'Sem grade', await canvasPng(m, 960, 640, 'Sem grade'));
    const noGrid = await createMapRPC(m, campaignId, 'Mapa sem grade', image);
    const map = { columns: 24, rows: 16 };
    const route = editorRoute(campaignId, table.mapId);
    // Back to the list with nothing chosen: the editor asks before it leaves a point with unsaved changes, so these scans start again.
    const reopen = async () => {
      await m.goto(route);
      await expect(m.getByRole('radio', { name: 'Pontos' })).toBeVisible();
    };
    const list = m.getByRole('region', { name: 'Pontos do mapa' });
    const tools = m.getByRole('group', { name: 'Ferramenta de pintura' });

    if (phone) {
      await m.goto(editorRoute(campaignId, table.mapId));
      await expect(m.getByText('Pintar só no computador')).toBeVisible();
      await m.waitForLoadState('networkidle');
      await expectScreenPasses(m, `Mapa no celular, sem pintura ${where}`);
      await m.getByRole('button', { name: 'Esquecer o que foi visto' }).click();
      await expect(m.getByRole('heading', { name: 'Esquecer o que foi visto?' })).toBeVisible();
      await expectScreenPasses(m, `Esquecer o que foi visto, no celular ${where}`);
      await m.goto(editorRoute(campaignId, noGrid));
      await expect(m.getByText('Pintar só no computador')).toBeVisible();
      await m.waitForLoadState('networkidle');
      await expectScreenPasses(m, `Mapa sem grade no celular ${where}`);
      return;
    }

    // Pontos: the list, and each kind's panel.
    await m.goto(editorRoute(campaignId, table.mapId));
    await expect(m.getByRole('radio', { name: 'Pontos' })).toBeVisible();
    await m.waitForLoadState('networkidle');
    await expectScreenPasses(m, `Editor, Pontos, a lista ${where}`);
    await list.getByRole('button', { name: /Fosso escondido/ }).click();
    await expect(m.getByRole('heading', { name: 'Predefinições do SRD' })).toBeVisible();
    await expect(m.getByText('Percepção passiva contra a CD 15')).toBeVisible();
    await expectScreenPasses(m, `Armadilha, o formulário e Quem notaria ${where}`);
    await m.getByLabel('CD para achar (Investigação)').fill('40');
    await m.getByRole('button', { name: 'Salvar ponto' }).click();
    await expect(m.getByText('Use uma CD de 1 a 30.')).toBeVisible();
    await expectScreenPasses(m, `Armadilha, um campo com erro ${where}`);
    await m.getByRole('radio', { name: /Agulha envenenada/ }).click();
    await expect(m.getByRole('heading', { name: 'Teste de resistência' })).toBeVisible();
    await expectScreenPasses(m, `Armadilha, a Agulha envenenada em partes ${where}`);
    // The form has unsaved changes (the preset): going to "Pintar" asks in place.
    await m.getByRole('radio', { name: 'Pintar' }).click();
    await expect(m.getByRole('heading', { name: /Salvar as mudanças em/ })).toBeFocused();
    await expectScreenPasses(m, `Salvar as mudanças? ${where}`);
    await m.getByRole('button', { name: 'Continuar editando' }).click();
    await reopen();
    await list.getByRole('button', { name: /Brasa do altar/ }).click();
    await expect(m.getByRole('radiogroup', { name: 'Tipo de luz' })).toBeVisible();
    await expectScreenPasses(m, `Luz personalizada ${where}`);
    await m.getByRole('radio', { name: /Tocha/ }).click();
    await expectScreenPasses(m, `Luz, uma predefinição ${where}`);
    await reopen();
    // The list is drawn a moment after the page: pick the row again until the panel is there.
    await expect(async () => {
      await list.getByRole('button', { name: /Baú de moedas/ }).click();
      await expect(m.getByRole('heading', { name: 'Baú de moedas', level: 2 })).toBeVisible({ timeout: 2_000 });
    }).toPass();
    await expect(m.locator('app-treasure-point-panel').getByText('Não encontrado')).toBeVisible();
    await expectScreenPasses(m, `Tesouro escondido ${where}`);
    await m.getByRole('button', { name: 'Marcar como encontrado' }).click();
    await expect(m.getByRole('group', { name: /Quem encontrou/ })).toBeVisible();
    await expectScreenPasses(m, `Tesouro, quem encontrou ${where}`);
    await m.getByRole('group', { name: /Quem encontrou/ }).locator('label', { hasText: 'Pensantus' }).click();
    await m.getByRole('button', { name: 'Marcar como encontrado' }).last().click();
    await expect(m.getByText(/Encontrado por Pensantus/)).toBeVisible();
    await expectScreenPasses(m, `Tesouro encontrado ${where}`);
    await m.getByRole('button', { name: 'Desmarcar' }).click();
    await expect(m.getByRole('group', { name: /^Desmarcar / })).toBeVisible();
    await expectScreenPasses(m, `Tesouro, desmarcar no lugar ${where}`);
    await m.getByRole('group', { name: /^Desmarcar / }).getByRole('button', { name: 'Desmarcar' }).click();
    await reopen();

    // Ver como
    await expect(m.getByRole('heading', { name: 'Ver como' })).toBeVisible();
    await m.getByRole('radio', { name: /Toren/ }).click();
    await expect(m.getByText('Você está vendo o mapa como Toren')).toBeVisible();
    await m.waitForLoadState('networkidle');
    await expectScreenPasses(m, `Ver como Toren, no editor ${where}`);
    await m.getByRole('button', { name: 'Voltar à sua vista' }).click();

    // Pintar: every tool, the brush, the layers.
    await reopen();
    await m.getByRole('radio', { name: 'Pintar' }).click();
    await expect(tools).toBeVisible();
    await tools.getByRole('button', { name: 'Terreno difícil' }).click();
    await dragSquares(m, map, [4, 9], [5, 10]);
    await expect(m.locator('app-layers-panel').getByText('Tudo salvo').first()).toBeVisible();
    await expectScreenPasses(m, `Pintar, Terreno difícil ${where}`);
    await tools.getByRole('button', { name: 'Cobertura' }).click();
    await m.getByRole('radio', { name: 'Três quartos' }).click();
    await clickSquare(m, map, 20, 4);
    await expectScreenPasses(m, `Pintar, Cobertura e o grau ${where}`);
    await tools.getByRole('button', { name: 'Luz' }).click();
    await m.getByRole('radio', { name: 'Claro' }).first().click();
    await m.getByRole('radio', { name: '3×3' }).click();
    await clickSquare(m, map, 8, 13);
    await expect(m.locator('app-layers-panel').getByText('Tudo salvo').first()).toBeVisible();
    await expectScreenPasses(m, `Pintar, Luz com os glifos ${where}`);
    await tools.getByRole('button', { name: 'Apagar' }).click();
    await expectScreenPasses(m, `Pintar, Apagar ${where}`);

    // The questions in place.
    await m.getByRole('button', { name: 'Mudar a grade' }).click();
    await expect(m.getByRole('heading', { name: 'Mudar a grade?' })).toBeFocused();
    await expectScreenPasses(m, `Mudar a grade? ${where}`);
    await m.getByRole('button', { name: 'Voltar' }).click();
    await m.getByRole('button', { name: 'Esquecer o que foi visto' }).click();
    await expect(m.getByRole('heading', { name: 'Esquecer o que foi visto?' })).toBeFocused();
    await expectScreenPasses(m, `Esquecer o que foi visto? ${where}`);
    await m.getByRole('button', { name: 'Voltar' }).click();
    await m.evaluate(() => window.scrollTo(0, 0));
    await m.getByRole('button', { name: 'Trocar imagem' }).click();
    await expect(m.getByRole('heading', { name: 'Trocar a imagem?' })).toBeFocused();
    await expectScreenPasses(m, `Trocar a imagem? ${where}`);
    await m.getByRole('button', { name: 'Voltar' }).first().click();

    // No grid, and a combat on the map.
    await m.goto(editorRoute(campaignId, noGrid));
    await m.getByRole('radio', { name: 'Pintar' }).click();
    await expect(m.getByText('Defina a grade para pintar e ligar a névoa.')).toBeVisible();
    await m.waitForLoadState('networkidle');
    await expectScreenPasses(m, `Mapa sem grade, Pintar ${where}`);
    await beginFogCombat(m, table);
    await m.goto(editorRoute(campaignId, table.mapId));
    await m.getByRole('radio', { name: 'Pintar' }).click();
    await expect(m.getByText('Combate em andamento')).toBeVisible();
    await m.waitForLoadState('networkidle');
    await expectScreenPasses(m, `Combate no mapa, Pintar ${where}`);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await Promise.all([master.close(), pensantus.close(), toren.close()]);
  }
}

test('o editor do mapa passa no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-034', '@MR-035', '@MR-036', '@MR-041'] }, async ({ browser }) => {
  test.setTimeout(600_000);
  await scanMapEditorScreens(browser, 'light', 1280);
});

test('o editor do mapa passa no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-034', '@MR-036'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanMapEditorScreens(browser, 'dark', 390);
});

test('o editor do mapa passa no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-034', '@MR-035', '@MR-036', '@MR-041'] }, async ({ browser }) => {
  test.setTimeout(600_000);
  await scanMapEditorScreens(browser, 'dark', 1024);
});

test('o editor do mapa passa no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-034', '@MR-036'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanMapEditorScreens(browser, 'light', 320);
});

// The doors (slice 10.14a, MR-010, RN-26, RN-10; E10-05 7 to 11): the "Porta" tool in "Pintar" with its panel, its refusal and its
// question; the master's door sheet in the session (a door, a secret door and the question before revealing it); what a player's
// map says (only "Porta fechada"); and the "Mover" page after a locked door stopped the move. Every state goes through axe and the
// alignment checks. On a phone the editor is "Pintar só no computador" (the existing scans), so only the session's states run there.
async function scanDoorScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const viewport = { width, height: width >= 768 ? 900 : width <= 320 ? 568 : 844 };
  const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const pensantus = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const toren = await newSignedInContext(browser, 'E-mail Não Verificado');
  const [m, ap, bp] = [await master.newPage(), await pensantus.newPage(), await toren.newPage()];
  const where = `(${colorScheme}, ${width}px)`;
  const phone = width < 768;
  let campaignId = '';
  const door = (state: string, col: number, row: number) => m.getByRole('button', { name: new RegExp(`^${state}, coluna ${col}, linha ${row}`) });
  // Opens a door's sheet by keyboard: a phone's map is zoomed on the party, so the door may be off the screen.
  const openDoor = async (state: string, col: number, row: number) => {
    await door(state, col, row).focus();
    await m.keyboard.press('Enter');
  };
  try {
    await Promise.all([m.goto('/'), ap.goto('/'), bp.goto('/')]);
    const table = await tableForFog(m, ap, bp, `Acessibilidade portas ${Date.now()}`, {});
    campaignId = table.campaignId;
    const target = { campaignId, mapId: table.mapId };
    const paint = async (layer: string, value: number, squares: [number, number][]) => {
      const res = await callRPC(m, 'meurpg.maps.v1.MapService/PaintMapCells', { ...target, layer, value, squares: squares.map(([col, row]) => ({ col, row })) });
      expect(res.ok(), await res.text()).toBeTruthy();
    };
    // The wall at (7, 9) and (7, 10) has floor on both sides: a closed and a locked door; a grade, an open door and a secret one stand on the floor.
    await paint('MAP_LAYER_WALL', 0, [[7, 9], [7, 10]]);
    await paint('MAP_LAYER_DOORS', 2, [[7, 9]]);
    await paint('MAP_LAYER_DOORS', 3, [[7, 10]]);
    await paint('MAP_LAYER_DOORS', 4, [[3, 7]]);
    await paint('MAP_LAYER_DOORS', 1, [[2, 7]]);
    await paint('MAP_LAYER_DOORS', 5, [[1, 7]]);

    if (!phone) {
      // The "Porta" tool: its panel, the kinds, a refused tap and the question before a door goes where someone stands.
      await m.goto(editorRoute(campaignId, table.mapId));
      await expect(m.getByRole('radio', { name: 'Pontos' })).toBeVisible();
      await m.getByRole('radio', { name: 'Pintar' }).click();
      const tools = m.getByRole('group', { name: 'Ferramenta de pintura' });
      await tools.getByRole('button', { name: 'Porta' }).click();
      await expect(m.getByText('Toque num quadrado para pôr a porta do tipo escolhido.')).toBeVisible();
      await expect(m.locator('app-layers-panel').getByText('Tudo salvo').first()).toBeVisible();
      await expectScreenPasses(m, `Editor, Pintar, a ferramenta Porta ${where}`);
      await m.getByRole('group', { name: 'Tipo de porta' }).getByRole('button', { name: 'Trancada' }).click();
      await expectScreenPasses(m, `Editor, Porta trancada escolhida ${where}`);
      const map = { columns: 24, rows: 16 };
      await clickSquare(m, map, 12, 12);
      await expect(m.getByRole('alert').filter({ hasText: 'uma porta precisa de chão dos dois lados' })).toBeVisible();
      await expectScreenPasses(m, `Editor, Porta: um toque que não serve ${where}`);
      // Toren stands on a wall square that has floor on both sides: a closed door there asks first.
      await moveTo(m, table, table.torenId, 7, 11);
      await paint('MAP_LAYER_WALL', 1, [[7, 11]]);
      await m.goto(editorRoute(campaignId, table.mapId));
      await m.getByRole('radio', { name: 'Pintar' }).click();
      await m.getByRole('group', { name: 'Ferramenta de pintura' }).getByRole('button', { name: 'Porta' }).click();
      await m.getByRole('group', { name: 'Tipo de porta' }).getByRole('button', { name: 'Fechada' }).click();
      await clickSquare(m, map, 7, 11);
      await expect(m.getByText('Pôr a porta onde há alguém?')).toBeVisible();
      await expectScreenPasses(m, `Editor, Porta: a pergunta de quem está no quadrado ${where}`);
      await m.getByRole('button', { name: 'Voltar' }).click();
    }

    // The session: the master's map names every door and each door has its sheet; the player's names only "Porta fechada".
    await Promise.all([m.goto(sessionRoute(campaignId)), ap.goto(sessionRoute(campaignId))]);
    await expect(m.getByRole('list', { name: 'Legenda do mapa' }).getByText('Porta secreta (só você vê)')).toBeVisible();
    await expect(m.getByRole('group', { name: /^Mapa A caverna/ })).toBeVisible();
    await expectScreenPasses(m, `Sessão, o mestre, o mapa com as portas e a legenda ${where}`);
    await expect(ap.getByRole('list', { name: 'Legenda do mapa' }).getByText('Porta fechada')).toBeVisible();
    await expect(ap.locator('app-fog-base').first()).toBeVisible();
    await expectScreenPasses(ap, `Sessão, o jogador, o mapa com "Porta fechada" ${where}`);
    await openDoor('Porta fechada', 8, 10);
    await expect(m.getByRole('dialog', { name: 'Porta' }).getByRole('radio', { name: /Fechada/ })).toHaveAttribute('aria-checked', 'true');
    await expectScreenPasses(m, `Sessão, a folha da porta fechada ${where}`);
    await m.keyboard.press('Escape');
    await expect(m.getByRole('dialog', { name: 'Porta' })).toHaveCount(0);
    await openDoor('Grade', 4, 8);
    await expect(m.getByRole('dialog', { name: 'Porta' }).getByRole('radio')).toHaveCount(2);
    await expectScreenPasses(m, `Sessão, a folha de uma grade ${where}`);
    await m.keyboard.press('Escape');
    await expect(m.getByRole('dialog', { name: 'Porta' })).toHaveCount(0);
    await openDoor('Porta secreta', 2, 8);
    const sheet = m.getByRole('dialog', { name: 'Porta' });
    await expect(sheet.getByRole('heading', { name: 'Porta secreta' })).toBeVisible();
    await expectScreenPasses(m, `Sessão, a folha da porta secreta ${where}`);
    await sheet.getByRole('button', { name: 'Revelar a porta secreta' }).click();
    await expect(sheet.getByText('Os jogadores vão ver a porta. Revelar?')).toBeVisible();
    await expectScreenPasses(m, `Sessão, a pergunta antes de revelar a porta secreta ${where}`);
    await sheet.getByRole('button', { name: 'Voltar' }).click();
    await m.keyboard.press('Escape');
    await expect(m.getByRole('dialog', { name: 'Porta' })).toHaveCount(0);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await Promise.all([master.close(), pensantus.close(), toren.close()]);
  }

  // The "Mover" page after a locked door stopped the move: the notice with the lock, and the map still "Porta fechada".
  const mover = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport });
  const moverPlayer = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport });
  const [mm, pp] = [await mover.newPage(), await moverPlayer.newPage()];
  let moveCampaign = '';
  try {
    await Promise.all([mm.goto('/'), pp.goto('/')]);
    const table = await tableForCombat(mm, pp, `Acessibilidade porta trancada ${Date.now()}`, true, true, { sheet: pensantusCasting });
    moveCampaign = table.campaignId;
    await paintRPC(mm, table, 'MAP_LAYER_WALL', 1, [[3, 5], [4, 5], [6, 5], [7, 5]]);
    const res = await callRPC(mm, 'meurpg.maps.v1.MapService/PaintMapCells', { campaignId: table.campaignId, mapId: table.mapId, layer: 'MAP_LAYER_DOORS', value: 3, squares: [{ col: 5, row: 5 }] });
    expect(res.ok(), await res.text()).toBeTruthy();
    await beginAttackCombatRPC(mm, table, { Pensantus: 20, 'Goblin 1': 15, 'Capitão Goblin': 10, 'Goblin 2': 4 }, { 'Capitão Goblin': [11, 9], 'Goblin 1': [14, 7], 'Goblin 2': [15, 11] });
    await openSessionPage(pp, moveCampaign);
    await expect(pp.getByRole('heading', { name: 'Sua vez, Pensantus' })).toBeVisible();
    await pp.getByRole('button', { name: 'Mover', exact: true }).click();
    await expect(pp.getByRole('heading', { name: 'Mover Pensantus' })).toBeVisible();
    await tapSquare(pp, 5, 4);
    await pp.getByRole('button', { name: 'Mover para cá' }).click();
    await expect(pp.getByRole('status').filter({ hasText: 'A porta está trancada.' })).toBeVisible();
    await expectScreenPasses(pp, `Mover, uma porta trancada parou o movimento ${where}`);
  } finally {
    if (moveCampaign) {
      await endOpenSessionRPC(mm, moveCampaign);
    }
    await Promise.all([mover.close(), moverPlayer.close()]);
  }
}

test('as portas passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-010', '@RN-26'] }, async ({ browser }) => {
  test.setTimeout(600_000);
  await scanDoorScreens(browser, 'light', 1280);
});

test('as portas passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-010', '@RN-26'] }, async ({ browser }) => {
  test.setTimeout(600_000);
  await scanDoorScreens(browser, 'dark', 390);
});

test('as portas passam no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-010', '@RN-26'] }, async ({ browser }) => {
  test.setTimeout(600_000);
  await scanDoorScreens(browser, 'dark', 1024);
});

test('as portas passam no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-010', '@RN-26'] }, async ({ browser }) => {
  test.setTimeout(600_000);
  await scanDoorScreens(browser, 'light', 320);
});

/**
 * The bestiary (MR-042, E10-08): the list, loading and failing, a search with results, one with none, a
 * filter set, the Ogre's stat block, the "Criar NPC" dialog (a sheet on a phone), its empty-name error, the
 * confirmation after the NPC is made, the player's notice and the campaign page with its "Bestiário" panel.
 */
async function scanBestiaryScreens(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  const context = await browser.newContext({
    storageState: authStatePath('Mestre Teste'),
    colorScheme,
    viewport: { width, height: 900 },
  });
  const playerContext = await browser.newContext({
    storageState: authStatePath('Jogador Teste'),
    colorScheme,
    viewport: { width, height: 900 },
  });
  const page = await context.newPage();
  const player = await playerContext.newPage();
  const where = `(${colorScheme}, ${width}px)`;
  const listed = (p: Page, count: string) => expect(p.locator('.list__n')).toHaveText(count);
  try {
    await page.goto('/');
    await player.goto('/');
    const { campaignId } = await tableForMaps(page, player, `Acessibilidade bestiário ${Date.now()}`);

    // Loading: the answer is held until the scan is done.
    let release!: () => void;
    const held = new Promise<void>((resolve) => (release = resolve));
    await page.route('**/meurpg.rules.v1.ContentService/ListCreatures', async (route) => {
      await held;
      await route.continue();
    });
    await page.goto(`/campanhas/${campaignId}/bestiario`);
    await expect(page.getByText('Buscando as criaturas...')).toBeVisible();
    await expectScreenPasses(page, `Bestiário, carregando ${where}`);
    release();
    await listed(page, '334 de 334 criaturas');
    await page.unroute('**/meurpg.rules.v1.ContentService/ListCreatures');
    await expectScreenPasses(page, `Bestiário, a lista ${where}`);

    // Failing: the server does not answer.
    await page.route('**/meurpg.rules.v1.ContentService/ListCreatures', (route) =>
      route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 'unavailable', message: 'down' }) }),
    );
    await page.getByRole('searchbox', { name: 'Nome' }).fill('lobo');
    await expect(page.getByRole('alert')).toContainText('o servidor não respondeu');
    await expectScreenPasses(page, `Bestiário, erro ${where}`);
    await page.unroute('**/meurpg.rules.v1.ContentService/ListCreatures');
    await page.getByRole('button', { name: 'Tentar de novo' }).click();
    await listed(page, '5 de 334 criaturas');
    await expectScreenPasses(page, `Bestiário, busca "lobo" ${where}`);

    await page.getByRole('searchbox', { name: 'Nome' }).fill('xyzzy');
    await expect(page.getByText('Nenhuma criatura com “xyzzy”.')).toBeVisible();
    await expectScreenPasses(page, `Bestiário, busca sem resultado ${where}`);

    await page.getByRole('button', { name: 'Limpar a busca' }).click();
    await page.locator('select[name=type]').selectOption('dragon');
    await page.locator('select[name=size]').selectOption('huge');
    await page.locator('select[name=cr]').selectOption('11-30');
    await expect(page.getByRole('button', { name: 'Limpar filtros' }).first()).toBeVisible();
    await expectScreenPasses(page, `Bestiário, filtros ligados ${where}`);

    await page.goto(`/campanhas/${campaignId}/bestiario/ogre`);
    await expect(page.getByText('Os textos abaixo são do livro de regras (SRD 5.1), em inglês.')).toBeVisible();
    await expectScreenPasses(page, `Bestiário, a ficha do Ogro ${where}`);

    await page.getByRole('button', { name: 'Criar NPC' }).click();
    const dialog = page.getByRole('dialog', { name: 'Criar NPC' });
    await expect(dialog.getByLabel('Nome do NPC')).toBeFocused();
    await expectScreenPasses(page, `Criar NPC ${where}`);

    await dialog.getByLabel('Nome do NPC').fill('');
    await dialog.getByRole('button', { name: 'Criar NPC' }).click();
    await expect(dialog.getByText('Dê um nome ao NPC.')).toBeVisible();
    await expect(dialog.getByLabel('Nome do NPC')).toBeFocused();
    await expectScreenPasses(page, `Criar NPC, nome vazio ${where}`);

    await dialog.getByLabel('Nome do NPC').fill('Capitão bandido');
    await dialog.getByRole('button', { name: 'Criar NPC' }).click();
    await expect(page.locator('.made')).toContainText('NPC criado: Capitão bandido.');
    await expectScreenPasses(page, `Criar NPC, a confirmação ${where}`);

    await page.goto(`/campanhas/${campaignId}`);
    const panel = page.getByRole('link', { name: 'Abrir o bestiário' });
    await panel.scrollIntoViewIfNeeded();
    await expect(panel).toBeVisible();
    await expectScreenPasses(page, `Campanha com o painel Bestiário ${where}`);

    // A player: no panel on the campaign page, and a calm notice on the page itself.
    await player.goto(`/campanhas/${campaignId}/bestiario`);
    await expect(player.getByText('Só o mestre usa o bestiário da campanha.')).toBeVisible();
    await expectScreenPasses(player, `Bestiário, o aviso do jogador ${where}`);
  } finally {
    await context.close();
    await playerContext.close();
  }
}

test('o bestiário passa no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-042'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanBestiaryScreens(browser, 'light', 1280);
});

test('o bestiário passa no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-042'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanBestiaryScreens(browser, 'dark', 390);
});

test('o bestiário passa no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-042'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanBestiaryScreens(browser, 'dark', 1024);
});

test('o bestiário passa no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-042'] }, async ({ browser }) => {
  test.setTimeout(300_000);
  await scanBestiaryScreens(browser, 'light', 320);
});

/**
 * "Regras da mesa" (MR-025, RN-24, RN-09; E10-03 states 1 to 3): the page as saved, a style chosen (the notice and
 * the "Mudou" tags, the save bar lit), the XP mode question open in place, and the rules for a table with a long
 * list of reminders. The XP question needs XP already given, so the campaign has Pensantus and an award.
 */
async function scanTableRules(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  test.setTimeout(120_000);
  const mContext = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport: { width, height: 900 } });
  const pContext = await newSignedInContext(browser, 'Jogador Teste');
  try {
    const m = await mContext.newPage();
    const p = await pContext.newPage();
    await Promise.all([m.goto('/'), p.goto('/')]);
    const table = await tableForXp(m, p, `Acessibilidade regras ${Date.now()}`);
    await awardXpRPC(m, table.campaignId, { mode: 'MANUAL', reason: 'A porta da torre', characterIds: [table.characterId], amount: 50 });
    await setTableRulesRPC(m, table.campaignId, { houseRules: ['Beber uma poção é uma ação bônus', 'Quem cai fica caído até o fim do turno'] });
    const where = `(${colorScheme}, ${width}px)`;

    await m.goto(`/campanhas/${table.campaignId}`);
    await expect(m.getByRole('heading', { level: 1 })).toBeVisible();
    await m.waitForLoadState('networkidle');
    await expectScreenPasses(m, `Campanha com o painel Regras da mesa ${where}`);

    await open(m, `/campanhas/${table.campaignId}/regras`);
    await expectScreenPasses(m, `Regras da mesa ${where}`);
    await pickRadio(m, /Mesa física/);
    await expect(m.getByText(/o estilo preencheu/)).toBeVisible();
    await expectScreenPasses(m, `Regras da mesa, um estilo escolhido ${where}`);
    await pickRadio(m, /Por marcos/);
    await m.getByRole('button', { name: 'Mudar para marcos' }).click();
    await expect(m.getByRole('heading', { name: 'Mudar para “por marcos”?' })).toBeVisible();
    await expectScreenPasses(m, `Regras da mesa, mudar o modo de XP ${where}`);

    // A player is told it is the master's page.
    await open(p, `/campanhas/${table.campaignId}/regras`);
    await expect(p.getByText('Só o mestre muda as regras da mesa.')).toBeVisible();
    await expectScreenPasses(p, `Regras da mesa, visto por um jogador ${where}`);
  } finally {
    await Promise.all([mContext.close(), pContext.close()]);
  }
}

test('as regras da mesa passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@RN-24', '@RN-09'] }, async ({ browser }) => {
  await scanTableRules(browser, 'light', 1280);
});

test('as regras da mesa passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@RN-24', '@RN-09'] }, async ({ browser }) => {
  await scanTableRules(browser, 'dark', 390);
});

test('as regras da mesa passam no axe e nas conferências de layout no tema escuro, no celular de 320', { tag: ['@a11y', '@RN-24'] }, async ({ browser }) => {
  await scanTableRules(browser, 'dark', 320);
});

/**
 * "Conteúdo da mesa" (MR-025, RN-23; E10-01): the master's list and editors (a spell, a race, a background), the question to
 * archive, a refusal on its field; the same list on a phone with the sheet that asks to archive; and what a player reads. The
 * entries come through the API, with one archived so its state shows.
 */
async function scanTableContent(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  test.setTimeout(180_000);
  const mContext = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport: { width, height: 900 } });
  const pContext = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport: { width, height: 900 } });
  try {
    const m = await mContext.newPage();
    const p = await pContext.newPage();
    await Promise.all([m.goto('/'), p.goto('/')]);
    const campaignId = await campaignWithEmptyPlayer(m, p, `Acessibilidade conteúdo ${Date.now()}`);
    const where = `(${colorScheme}, ${width}px)`;
    const spell = await createEntryRPC(m, campaignId, 'tableSpell', spellBody('Lâmina de Nanquim'));
    await createEntryRPC(m, campaignId, 'tableSpell', spellBody('Sopro de Nanquim', {
      range: { kind: 'SPELL_RANGE_KIND_SELF' },
      target: { kind: 'TABLE_SPELL_TARGET_KIND_AREA', shape: 'TABLE_AREA_SHAPE_CONE', sizeFt: 15 },
      attack: '',
      save: { ability: 'ABILITY_DEXTERITY', onSuccess: 'SPELL_SAVE_SUCCESS_HALF' },
      damage: [{ damageTypeKey: 'damage-type:necrotic', dice: '3d6', perSlotLevel: '1d6' }],
    }));
    const archived = await createEntryRPC(m, campaignId, 'tableSpell', spellBody('Rascunho de Tinta'));
    const race = await createEntryRPC(m, campaignId, 'tableRace', raceBody());
    const background = await createEntryRPC(m, campaignId, 'tableBackground', {
      namePt: 'Cartógrafo do Vale',
      skills: ['skill:investigation', 'skill:survival'],
      tools: ['proficiency:thieves-tools'],
      equipmentPt: 'Um estojo de mapas, tinta e 10 PO',
      feature: { namePt: 'Mapas na memória', descPt: ['Você lembra o desenho de qualquer lugar que já mapeou.'], effects: [{ type: 'note', textPt: 'Lembra qualquer lugar mapeado.' }] },
    });
    await archiveEntryRPC(m, campaignId, archived);

    await open(m, `/campanhas/${campaignId}/conteudo?tipo=magias`);
    await expectScreenPasses(m, `Conteúdo da mesa, as magias ${where}`);
    if (width >= 768) {
      await open(m, `/campanhas/${campaignId}/conteudo`);
      await expectScreenPasses(m, `Conteúdo da mesa, a lista inicial ${where}`);
      await open(m, entryRoute(campaignId, spell));
      await expect(m.getByLabel('Nome', { exact: true })).toHaveValue('Lâmina de Nanquim');
      await expectScreenPasses(m, `Editor de magia ${where}`);
      await open(m, entryRoute(campaignId, race));
      await expect(m.getByLabel('Nome', { exact: true })).toHaveValue('Corujeiro');
      await expectScreenPasses(m, `Editor de raça ${where}`);
      await m.getByRole('button', { name: 'Mais opções' }).first().click();
      await expect(m.getByRole('button', { name: 'Menos opções' }).first()).toBeVisible();
      await expectScreenPasses(m, `Editor de raça, "Mais opções" aberto ${where}`);
      await m.getByRole('button', { name: 'Arquivar', exact: true }).click();
      await expect(m.getByRole('region', { name: 'Arquivar Corujeiro?' })).toBeVisible();
      await expectScreenPasses(m, `Arquivar a raça, a pergunta no lugar ${where}`);
      await m.getByRole('button', { name: 'Arquivar Corujeiro' }).click();
      await expect(m.getByText('A raça Corujeiro está arquivada.')).toBeVisible();
      await expectScreenPasses(m, `A raça arquivada, com "Desarquivar" ${where}`);
      // It comes back at once, so the player's screens below still have it.
      await m.getByRole('button', { name: 'Desarquivar' }).click();
      await expect(m.getByText('A raça Corujeiro está arquivada.')).toHaveCount(0);
      await open(m, `/campanhas/${campaignId}/conteudo/novo/subraca`);
      await expectScreenPasses(m, `Editor de sub-raça, a raça a escolher ${where}`);
      await open(m, entryRoute(campaignId, background));
      await expect(m.getByLabel('Nome', { exact: true })).toHaveValue('Cartógrafo do Vale');
      await expectScreenPasses(m, `Editor de antecedente ${where}`);
      await open(m, `/campanhas/${campaignId}/conteudo/novo/magia`);
      await m.getByLabel('Nome', { exact: true }).fill('Lâmina de Nanquim');
      await m.getByLabel('Distância').fill('18');
      await m.getByRole('button', { name: 'Salvar magia' }).click();
      await expect(m.getByText('Já existe uma magia da mesa com este nome. Escolha outro.')).toBeVisible();
      await expectScreenPasses(m, `Editor de magia, a recusa no campo ${where}`);
      // Another tab saves first: the stale alert, with "Recarregar".
      await open(m, entryRoute(campaignId, spell));
      await expect(m.getByLabel('Nome', { exact: true })).toHaveValue('Lâmina de Nanquim');
      await updateEntryRPC(m, campaignId, spell, 'tableSpell', spellBody('Lâmina de Nanquim', { descPt: ['Outro texto.'] }));
      await m.getByRole('button', { name: 'Salvar magia' }).click();
      await expect(m.getByRole('alert').filter({ hasText: 'Esta entrada mudou enquanto você editava.' })).toBeVisible();
      await expectScreenPasses(m, `Editor de magia, a entrada mudou enquanto se editava ${where}`);
    } else {
      await m.getByRole('button', { name: 'Arquivar Lâmina de Nanquim' }).click();
      await expect(m.getByRole('heading', { name: 'Arquivar Lâmina de Nanquim?' })).toBeVisible();
      await expectScreenPasses(m, `Arquivar, a folha de baixo ${where}`);
      await m.getByRole('button', { name: 'Voltar', exact: true }).click();
      await open(m, entryRoute(campaignId, race));
      await expectScreenPasses(m, `Raça lida pelo mestre no celular ${where}`);
    }

    await open(p, `/campanhas/${campaignId}/conteudo`);
    await expect(p.getByText('Da mesa').first()).toBeVisible();
    await expectScreenPasses(p, `Conteúdo da mesa, visto por um jogador ${where}`);
    await open(p, entryRoute(campaignId, race));
    await expect(p.getByText('Olhos de caçador.')).toBeVisible();
    await expectScreenPasses(p, `Raça, vista por um jogador ${where}`);
    await open(p, entryRoute(campaignId, spell));
    await expectScreenPasses(p, `Magia, vista por um jogador ${where}`);
    await open(p, entryRoute(campaignId, background));
    await expect(p.getByText('Ferramentas de ladrão')).toBeVisible();
    await expectScreenPasses(p, `Antecedente, visto por um jogador ${where}`);
  } finally {
    await Promise.all([mContext.close(), pContext.close()]);
  }
}

test('o conteúdo da mesa passa no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@MR-025', '@RN-23'] }, async ({ browser }) => {
  await scanTableContent(browser, 'light', 1280);
});

test('o conteúdo da mesa passa no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@MR-025'] }, async ({ browser }) => {
  await scanTableContent(browser, 'dark', 1024);
});

test('o conteúdo da mesa passa no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@MR-025', '@RN-23'] }, async ({ browser }) => {
  await scanTableContent(browser, 'dark', 390);
});

test('o conteúdo da mesa passa no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@MR-025'] }, async ({ browser }) => {
  await scanTableContent(browser, 'light', 320);
});

/**
 * The "Atributos" step of a player who makes a new sheet by the table's rules (E10-03 state 4): the four ways, each
 * with what it shows (the placing of the standard array, the point buy with the points left, the 4d6 the server
 * rolled and the physical dice to type, and the typed values).
 */
async function scanTableAbilities(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  test.setTimeout(180_000);
  const mContext = await newSignedInContext(browser, 'Mestre Teste');
  const pContext = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme, viewport: { width, height: 900 } });
  try {
    const m = await mContext.newPage();
    const p = await pContext.newPage();
    await Promise.all([m.goto('/'), p.goto('/')]);
    const campaignId = await campaignWithEmptyPlayer(m, p, `Acessibilidade atributos ${Date.now()}`);
    const where = `(${colorScheme}, ${width}px)`;
    await open(p, `/campanhas/${campaignId}/personagens/novo`);
    await p.getByLabel('Nome do personagem', { exact: true }).fill('Ícaro');
    await p.getByRole('tab', { name: 'Atributos' }).click();
    await expect(p.getByRole('radio', { name: 'Padrão' })).toBeChecked();
    await expectScreenPasses(p, `Atributos, conjunto padrão ${where}`);

    await method(p, 'Pontos');
    for (let i = 0; i < 7; i++) {
      await p.getByRole('button', { name: 'Aumentar Sabedoria', exact: true }).click();
    }
    await expect(p.getByText('Restam 18 pontos')).toBeVisible();
    await expectScreenPasses(p, `Atributos, compra por pontos ${where}`);

    await method(p, '4d6');
    await expectScreenPasses(p, `Atributos, 4d6 ainda sem rolar ${where}`);
    await p.getByRole('button', { name: 'Rolar os atributos' }).click();
    await expect(p.getByText(/Rolados em/)).toBeVisible();
    await expectScreenPasses(p, `Atributos, 4d6 rolados pelo servidor ${where}`);

    await method(p, 'Digitar');
    await p.getByLabel('Força', { exact: true }).fill('19');
    await expectScreenPasses(p, `Atributos, digitar com um valor fora do limite ${where}`);

    // Physical dice: a second campaign where everybody rolls their own.
    const physical = await campaignWithEmptyPlayer(m, p, `Acessibilidade dados ${Date.now()}`);
    await setTableRulesRPC(m, physical, { diceMode: 'DICE_MODE_PHYSICAL' });
    await open(p, `/campanhas/${physical}/personagens/novo`);
    await p.getByRole('tab', { name: 'Atributos' }).click();
    await method(p, '4d6');
    await expect(p.getByText('Digite os quatro dados de cada rolagem.')).toBeVisible();
    await expectScreenPasses(p, `Atributos, dados físicos a digitar ${where}`);
  } finally {
    await Promise.all([mContext.close(), pContext.close()]);
  }
}

test('os atributos por jeito passam no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@RN-24'] }, async ({ browser }) => {
  await scanTableAbilities(browser, 'light', 1280);
});

test('os atributos por jeito passam no axe e nas conferências de layout no tema escuro, no celular', { tag: ['@a11y', '@RN-24'] }, async ({ browser }) => {
  await scanTableAbilities(browser, 'dark', 390);
});

test('os atributos por jeito passam no axe e nas conferências de layout no tema claro, no celular de 320', { tag: ['@a11y', '@RN-24'] }, async ({ browser }) => {
  await scanTableAbilities(browser, 'light', 320);
});

/** The grid calibration (E10-03 state 5): the panel of a calibrated map, the question with "Outro", and "Mudar a grade?". */
async function scanCalibration(browser: Browser, colorScheme: 'light' | 'dark', width: number): Promise<void> {
  test.setTimeout(180_000);
  const context = await browser.newContext({ storageState: authStatePath('Mestre Teste'), colorScheme, viewport: { width, height: 900 } });
  try {
    const page = await context.newPage();
    await page.goto('/');
    const campaignId = await masterCampaign(page, `Acessibilidade calibração ${Date.now()}`);
    const map = await mapToPaint(page, campaignId, 'A torre em ruínas', 12);
    const painted = await callRPC(page, 'meurpg.maps.v1.MapService/PaintMapCells', { campaignId, mapId: map.mapId, layer: 'MAP_LAYER_WALL', value: 1, squares: wallSquares() });
    expect(painted.ok()).toBeTruthy();
    const where = `(${colorScheme}, ${width}px)`;
    await page.goto(editorRoute(campaignId, map.mapId));
    await expect(page.getByRole('radio', { name: 'Pintar' })).toBeVisible();
    await page.getByRole('radio', { name: 'Pintar' }).click();
    const grid = page.getByRole('region', { name: 'Grade' });
    await grid.getByRole('button', { name: 'Calibrar o quadrado' }).click();
    await expect(page.getByRole('heading', { name: 'Cada quadrado deste desenho vale' })).toBeFocused();
    await expectScreenPasses(page, `Calibração da grade, a pergunta ${where}`);
    await pickRadio(page.locator('app-calibrate-ask'), /Outro/);
    await page.getByLabel('Quanto vale o quadrado', { exact: true }).fill('4,5');
    await expect(page.getByText('o mapa terá 36 × 24 quadrados.')).toBeVisible();
    await expectScreenPasses(page, `Calibração da grade, Outro ${where}`);
    await factor(page, '3 m');
    await page.getByRole('button', { name: 'Salvar a grade' }).click();
    await expect(grid.getByText('cada um vale 3 m')).toBeVisible();
    await expectScreenPasses(page, `Calibração da grade, o mapa calibrado ${where}`);
    await grid.getByRole('button', { name: 'Calibrar o quadrado' }).click();
    await factor(page, '4,5 m');
    await page.getByRole('button', { name: 'Salvar a grade' }).click();
    await expect(page.getByRole('heading', { name: 'Mudar a grade?' })).toBeVisible();
    await expectScreenPasses(page, `Calibração da grade, Mudar a grade? ${where}`);
  } finally {
    await context.close();
  }
}

test('a calibração da grade passa no axe e nas conferências de layout no tema claro, no desktop', { tag: ['@a11y', '@RN-25'] }, async ({ browser }) => {
  await scanCalibration(browser, 'light', 1280);
});

test('a calibração da grade passa no axe e nas conferências de layout no tema escuro, no desktop de 1024', { tag: ['@a11y', '@RN-25'] }, async ({ browser }) => {
  await scanCalibration(browser, 'dark', 1024);
});

test('a calibração da grade passa no axe e nas conferências de layout no tema claro, no tablet de 768', { tag: ['@a11y', '@RN-25'] }, async ({ browser }) => {
  await scanCalibration(browser, 'light', 768);
});

test('a página Créditos, com a atribuição do SRD 5.2.1, passa no axe nos dois temas', { tag: ['@a11y', '@licenca'] }, async ({ browser }) => {
  for (const [scheme, width] of [['light', 1280], ['dark', 390]] as const) {
    const context = await browser.newContext({ colorScheme: scheme, viewport: { width, height: 900 } });
    try {
      const page = await context.newPage();
      await open(page, '/creditos');
      await expect(page.getByText('System Reference Document 5.2.1', { exact: false }).first()).toBeVisible();
      await expectScreenPasses(page, `Créditos (${scheme}, ${width}px)`);
    } finally {
      await context.close();
    }
  }
});
