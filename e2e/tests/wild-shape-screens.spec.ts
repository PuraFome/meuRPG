import { type Browser, expect } from '@playwright/test';
import { test } from './diagnostics';

import { endTurnOf, passTurnsTo } from './combat-support';
import { openCombatPanel, openOrderRow, openTurn } from './combat-layout';
import { beginCreatureCombat, sessionRoute, tableForCreatureCombat } from './creatures-combat-support';
import { endOpenSessionRPC } from './live-session-support';
import { authStatePath, openActionTab } from './support';
import { WILD_VIEWPORTS, WILD_WIDTHS, cropWild, shootWild, wildShotScale } from './wild-shape-shots-support';

// The #584 screen (boards 584-01 to 584-06): the time of the form in the band, the economy row, "Permanecer na forma", the
// sheet's "Criaturas" block and the master's time. The checks run in a normal run; with `E2E_SHOTS_DIR` set the same run also
// saves the screens at 390, 768 and 1280 and the close-ups (the shots are judged against the boards).

const FACES = { Toren: 20, Sálvia: 13, 'Capitão Goblin': 5, 'Goblin 1': 4, 'Goblin 2': 4 };

async function wildShapeScreens(browser: Browser, scheme: 'light' | 'dark'): Promise<void> {
  const viewport = { width: 1280, height: 800 };
  const contexts = await Promise.all(
    (['Mestre Teste', 'Jogador Teste', 'E-mail Não Verificado'] as const).map((user) =>
      browser.newContext({ storageState: authStatePath(user), colorScheme: scheme, viewport, ...wildShotScale }),
    ),
  );
  const [m, p, t] = await Promise.all(contexts.map((c) => c.newPage()));
  let campaignId = '';
  try {
    await Promise.all([m.goto('/'), p.goto('/'), t.goto('/')]);
    const table = await tableForCreatureCombat(m, p, t, `Telas da forma ${Date.now()}`);
    campaignId = table.campaignId;
    await beginCreatureCombat(m, table, { Sálvia: p, Toren: t }, FACES);
    await passTurnsTo(m, campaignId, 'Sálvia');
    await p.goto(sessionRoute(campaignId));
    await expect(p.getByRole('heading', { name: 'Sua vez, Sálvia' })).toBeVisible();

    // Her own shape: the band has no chip; the economy row is the first row of "Ações".
    await expect(p.locator('app-form-chip')).toHaveCount(0);
    await expect(p.getByRole('list', { name: 'O que resta da vez' })).toBeVisible();

    // She turns into a Wolf.
    await openActionTab(p, 'Druida');
    await p.getByRole('button', { name: 'Transformar: Forma Selvagem' }).click();
    const sheet = p.getByRole('dialog', { name: 'Forma Selvagem' });
    await sheet.getByLabel('Buscar fera').fill('wolf');
    await sheet.locator('label', { hasText: '(Wolf)' }).click();
    await sheet.getByRole('button', { name: 'Virar Lobo' }).click();
    const chip = p.locator('app-form-chip').first();
    await expect(chip).toContainText('Na forma de Lobo');
    await expect(chip).toContainText(/\d+ (h|min)|rodadas?/);
    await expect(chip).not.toContainText('restantes');

    // 584-01, 584-02: the owner's band with the chip, the economy row above the tabs with "Mover" as its only button.
    const economy = p.getByRole('list', { name: 'O que resta da vez' });
    await expect(p.locator('app-action-groups .econ').getByRole('button')).toHaveCount(1);
    await expect(p.getByRole('button', { name: 'Encerrar turno' })).toHaveCount(1);
    for (const w of WILD_WIDTHS) {
      await shootWild(p, 'banda-dona', w, scheme);
    }
    await cropWild(chip, 'recorte-chip', scheme);
    await cropWild(p.locator('app-action-groups .econ'), 'recorte-economia', scheme);
    await expect(economy).toBeVisible();

    // The class tab (Habilidades in a form): "Voltar" then "Permanecer na forma", the economy row still above.
    await openActionTab(p, 'Habilidades');
    const stay = p.locator('app-action-row', { hasText: 'Permanecer na forma' });
    await expect(stay).toContainText('Sem magias');
    await expect(stay).toContainText('o tempo recomeça');
    const rows = await p.locator('app-action-row:visible').allTextContents();
    expect(rows.findIndex((r) => r.includes('Voltar à forma normal'))).toBeLessThan(rows.findIndex((r) => r.includes('Permanecer na forma')));
    for (const w of WILD_WIDTHS) {
      await shootWild(p, 'acoes-forma', w, scheme);
    }
    await cropWild(stay, 'recorte-permanecer', scheme);

    // 584-02 card 2: another player reads the name of the beast, never the time.
    await t.goto(sessionRoute(campaignId));
    const others = t.locator('app-form-chip').first();
    await expect(others).toContainText('Na forma de Lobo');
    await expect(others).not.toContainText(/min|rodada|\dh/);
    for (const w of WILD_WIDTHS) {
      await shootWild(t, 'banda-outro', w, scheme);
    }
    await cropWild(others, 'recorte-chip-outro', scheme);

    // 584-03: the sheet's "Criaturas" block, with the time and the two buttons.
    await p.goto(`/campaigns/${campaignId}/characters/${table.heroId}`);
    const panel = p.locator('app-creatures-panel');
    await expect(panel.getByRole('heading', { name: 'Criaturas', level: 2 })).toBeVisible();
    const block = panel.locator('.form');
    await expect(block).toContainText('Na forma de Lobo');
    await expect(block).toContainText(/(h|min) restantes|rodadas?/);
    await expect(block.getByRole('button')).toHaveText([/Voltar à forma normal/, /Permanecer na forma/]);
    await expect(panel).not.toContainText('Nenhuma criatura');
    for (const w of WILD_WIDTHS) {
      await shootWild(p, 'ficha-criaturas', w, scheme, panel);
    }
    await cropWild(block, 'recorte-ficha-forma', scheme);

    // 584-04, 584-05: the master reads the time in the order row and in the act-as card.
    await m.goto(sessionRoute(campaignId));
    for (const w of WILD_WIDTHS.length ? WILD_WIDTHS : [1280]) {
      await m.setViewportSize({ width: w, height: WILD_VIEWPORTS[w] });
      await openOrderRow(m, 'Sálvia');
      const order = m.locator('app-order-list');
      await expect(order).toContainText(/Na forma de Lobo\s*·\s*hourglass_empty\s*(\d+ h|\d+ min|\d+ rodadas?)/);
      await shootWild(m, 'mestre-ordem', w, scheme, order.getByRole('button', { name: /^Sálvia/ }));
      if (w === 1280) {
        await cropWild(order.locator('app-form-tag').first(), 'recorte-mestre-ordem', scheme);
      }
      await openTurn(m);
      const offer = m.locator('app-act-as-card');
      await expect(offer).toContainText(/Na forma de Lobo/);
      await shootWild(m, 'mestre-agir', w, scheme, offer);
      if (w === 1280) {
        await cropWild(offer, 'recorte-mestre-agir', scheme);
      }
      await m.getByRole('button', { name: 'Agir por Sálvia' }).click();
      await expect(m.getByText('Você está agindo por Sálvia')).toBeVisible();
      await m.getByRole('button', { name: 'Voltar ao mestre' }).click();
    }
    await m.setViewportSize({ width: 1280, height: 800 });

    // 584-04 card 7: the master's effects row carries the time and "Encerrar".
    const effectsRow = () => m.getByRole('region', { name: 'Efeitos em jogo' }).first().locator('li.fx__row', { hasText: 'Forma Selvagem' });
    for (const w of WILD_WIDTHS.length ? WILD_WIDTHS : [1280]) {
      await m.setViewportSize({ width: w, height: WILD_VIEWPORTS[w] });
      await openCombatPanel(m, 'Efeitos');
      await expect(effectsRow()).toContainText(/Duração e fim: .*(h|min) restantes|rodadas?/);
      await shootWild(m, 'mestre-efeitos', w, scheme, effectsRow());
      if (w === 1280) {
        await cropWild(effectsRow(), 'recorte-mestre-tempo', scheme);
      }
    }
    await m.setViewportSize({ width: 1280, height: 800 });
    const row = effectsRow();

    // 584-06 card 8: the form ended between the reading and the tap: refused, nothing spent, never a new form. Her page loses the
    // table (offline) while the master ends the form, and taps "Permanecer" the moment the line is back, before its stream is.
    await p.goto(sessionRoute(campaignId));
    await expect(p.locator('app-form-chip').first()).toContainText('Na forma de Lobo');
    await openActionTab(p, 'Habilidades');
    const staleStay = p.getByRole('button', { name: 'Permanecer na forma' });
    await expect(staleStay).toBeVisible();
    await contexts[1].setOffline(true);
    await row.getByRole('button', { name: /Encerrar/ }).click();
    const confirm = m.getByRole('alertdialog');
    if (await confirm.isVisible().catch(() => false)) {
      await confirm.getByRole('button', { name: /Encerrar/ }).click();
    }
    await expect(row).toHaveCount(0);
    await contexts[1].setOffline(false);
    await staleStay.click();
    await expect(p.getByText('A forma já acabou').first()).toBeVisible();
    for (const w of WILD_WIDTHS) {
      await shootWild(p, 'forma-acabou', w, scheme);
    }
    await expect(p.getByText('Na forma de Lobo')).toHaveCount(0);
    await endTurnOf(p, m, campaignId, 'Sálvia');
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await Promise.all(contexts.map((c) => c.close()));
  }
}

test('a tela da forma selvagem: a faixa com o tempo, a linha da vez, Permanecer, a ficha e o mestre, no tema claro', { tag: ['@MR-037', '@RN-20'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await wildShapeScreens(browser, 'light');
});

test('a tela da forma selvagem: a faixa com o tempo, a linha da vez, Permanecer, a ficha e o mestre, no tema escuro', { tag: ['@MR-037', '@RN-20'] }, async ({ browser }) => {
  test.setTimeout(420_000);
  await wildShapeScreens(browser, 'dark');
});
