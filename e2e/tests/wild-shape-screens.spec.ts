import { type Browser, expect } from "@playwright/test";
import { test } from "./diagnostics";

import { endTurnOf, passTurnsTo } from "./combat-support";
import { hitAndApply } from "./creatures-combat-support";
import { openCombatPanel, openOrderRow, openTurn } from "./combat-layout";
import {
  beginCreatureCombat,
  sessionRoute,
  tableForCreatureCombat,
} from "./creatures-combat-support";
import { endOpenSessionRPC } from "./live-session-support";
import { authStatePath, openActionTab } from "./support";
import {
  WILD_VIEWPORTS,
  WILD_WIDTHS,
  cropWild,
  shootWild,
  wildShotScale,
} from "./wild-shape-shots-support";

// The #584 screen (boards 584-01 to 584-06): the time of the form in the band, the economy row, "Permanecer na forma", the
// sheet's "Criaturas" block and the master's time. The checks run in a normal run; with `E2E_SHOTS_DIR` set the same run also
// saves the screens at 390, 768 and 1280 and the close-ups (the shots are judged against the boards).

const FACES = {
  Toren: 20,
  Sálvia: 13,
  "Capitão Goblin": 5,
  "Goblin 1": 4,
  "Goblin 2": 4,
};

async function wildShapeScreens(
  browser: Browser,
  scheme: "light" | "dark",
): Promise<void> {
  const viewport = { width: 1280, height: 800 };
  const contexts = await Promise.all(
    (["Mestre Teste", "Jogador Teste", "E-mail Não Verificado"] as const).map(
      (user) =>
        browser.newContext({
          storageState: authStatePath(user),
          colorScheme: scheme,
          viewport,
          ...wildShotScale,
        }),
    ),
  );
  const [m, p, t] = await Promise.all(contexts.map((c) => c.newPage()));
  let campaignId = "";
  try {
    await Promise.all([m.goto("/"), p.goto("/"), t.goto("/")]);
    const table = await tableForCreatureCombat(
      m,
      p,
      t,
      `Telas da forma ${Date.now()}`,
    );
    campaignId = table.campaignId;
    await beginCreatureCombat(m, table, { Sálvia: p, Toren: t }, FACES);
    await passTurnsTo(m, campaignId, "Sálvia");
    await p.goto(sessionRoute(campaignId));
    await expect(
      p.getByRole("heading", { name: "Sua vez, Sálvia" }),
    ).toBeVisible();

    // Her own shape: the band has no chip; the economy row is the first row of "Ações".
    await expect(p.locator("app-form-chip")).toHaveCount(0);
    await expect(
      p.getByRole("list", { name: "O que resta da vez" }),
    ).toBeVisible();

    // She turns into a Wolf.
    await openActionTab(p, "Druida");
    await p
      .getByRole("button", { name: "Transformar: Forma Selvagem" })
      .click();
    const sheet = p.getByRole("dialog", { name: "Forma Selvagem" });
    await sheet.getByLabel("Buscar fera").fill("wolf");
    await sheet.locator("label", { hasText: "(Wolf)" }).click();
    await sheet.getByRole("button", { name: "Virar Lobo" }).click();
    const chip = p.locator("app-form-chip").first();
    await expect(chip).toContainText("Na forma de Lobo");
    await expect(chip).toContainText(/\d+ (h|min)|rodadas?/);
    await expect(chip).not.toContainText("restantes");

    // 584-01, 584-02: the owner's band with the chip, the economy row above the tabs with "Mover" as its only button.
    const economy = p.getByRole("list", { name: "O que resta da vez" });
    await expect(
      p.locator("app-action-groups .econ").getByRole("button"),
    ).toHaveCount(1);
    await expect(p.getByRole("button", { name: "Encerrar turno" })).toHaveCount(
      1,
    );
    for (const w of WILD_WIDTHS) {
      await shootWild(p, "banda-dona", w, scheme);
    }
    await cropWild(chip, "recorte-chip", scheme);
    await cropWild(
      p.locator("app-action-groups .econ"),
      "recorte-economia",
      scheme,
    );
    await expect(economy).toBeVisible();

    // 584-01, 584-02: the tabs in a form are Ataques (the beast's bite), Druida (Voltar, Permanecer) and Outras, no Magias tab,
    // and the notice at the foot. Each tab is opened and shot; the counts do not depend on when you look.
    const tabNames = async () =>
      (await p.getByRole("tab").allTextContents()).map((t) => t.replace(/\s+/g, " ").trim());
    await expect(p.getByRole("tab", { name: /^Magias/ })).toHaveCount(0);
    await expect(
      p.getByText("Sem magias na forma de fera. Volte à forma normal para conjurar."),
    ).toBeVisible();
    await openActionTab(p, "Ataques");
    // Turning into the beast took her Action, so the bite is under the folded "Ação · Usada" line; opened, it is the only row.
    await p.getByRole("button", { name: "Ação · Usada" }).click();
    await expect(p.locator("app-action-row:visible").first()).toContainText(/Mordida|Bite/);
    await expect(p.locator("app-action-row:visible")).toHaveCount(1);
    await expect(p.getByText("Agarrar")).toHaveCount(0);
    const counts = await tabNames();
    expect(counts.some((t) => /Ataques\s*1$/.test(t)), counts.join("|")).toBe(true);
    expect(counts.some((t) => /Druida\s*2$/.test(t)), counts.join("|")).toBe(true);
    for (const w of WILD_WIDTHS) {
      await shootWild(p, "abas-ataques", w, scheme);
    }
    await openActionTab(p, "Outras");
    for (const w of WILD_WIDTHS) {
      await shootWild(p, "abas-outras", w, scheme);
    }
    await openActionTab(p, "Druida");
    const stay = p.locator("app-action-row", {
      hasText: "Permanecer na forma",
    });
    await expect(stay).toContainText("Sem ação");
    await expect(stay).not.toContainText("Sem custo");
    await expect(stay).toContainText("Sem magias");
    await expect(stay).toContainText(/e o tempo volta a \d+ h/);
    await expect(
      p.locator("app-action-groups .econ").getByRole("button", { name: "Mover" }),
    ).toBeVisible();
    const rows = await p.locator("app-action-row:visible").allTextContents();
    expect(
      rows.findIndex((r) => r.includes("Voltar à forma normal")),
    ).toBeLessThan(rows.findIndex((r) => r.includes("Permanecer na forma")));
    for (const w of WILD_WIDTHS) {
      await shootWild(p, "acoes-forma", w, scheme);
    }
    await cropWild(stay, "recorte-permanecer", scheme);

    // 584-02, 584-04, 584-05: the wolf takes 4 damage; the form's pool is what the order and the cards show, never the druid's.
    await hitAndApply(m, campaignId, "Goblin 1", "Sálvia", 4);
    const mine = p.locator("app-order-column");
    await expect(mine).toContainText("Você · 7 de 11 PV (Lobo)");
    await expect(mine).not.toContainText("Na forma de Lobo");
    await expect(mine).not.toContainText("27");
    const sheetRegion = p.getByRole("region", { name: "Sálvia", exact: true });
    await expect(sheetRegion.getByRole("group", { name: "PV do Lobo: 7 de 11" })).toBeVisible();
    await expect(sheetRegion).toContainText(/CA do Lobo\s*13/);
    // The turn card of the phone: the form's PV and CA, never the druid's.
    await p.setViewportSize({ width: 390, height: 844 });
    const numbers = p.locator("app-own-numbers").first();
    await expect(numbers).toContainText("PV 7 de 11");
    await expect(numbers).toContainText("CA 13");
    await expect(numbers).not.toContainText("38");
    await cropWild(numbers, "recorte-turno-pv", scheme);
    for (const w of WILD_WIDTHS) {
      await shootWild(p, "turno-forma", w, scheme);
    }
    await p.setViewportSize({ width: 1280, height: 800 });
    await cropWild(mine, "recorte-ordem-dona", scheme);

    // 584-02 card 2: another player reads the name of the beast, never the time.
    await t.goto(sessionRoute(campaignId));
    const others = t.locator("app-form-chip").first();
    await expect(others).toContainText("Na forma de Lobo");
    await expect(others).not.toContainText(/min|rodada|\dh/);
    for (const w of WILD_WIDTHS) {
      await shootWild(t, "banda-outro", w, scheme);
    }
    await cropWild(others, "recorte-chip-outro", scheme);

    // 584-03: the sheet's "Criaturas" block, with the time and the two buttons.
    await p.goto(`/campaigns/${campaignId}/characters/${table.heroId}`);
    const panel = p.locator("app-creatures-panel");
    await expect(
      panel.getByRole("heading", { name: "Criaturas", level: 2 }),
    ).toBeVisible();
    const block = panel.locator(".form");
    await expect(block).toContainText("Na forma de Lobo");
    await expect(block).toContainText(/(h|min) restantes|rodadas?/);
    await expect(block.getByRole("button")).toHaveText([
      /Voltar à forma normal/,
      /Permanecer na forma/,
    ]);
    await expect(panel).not.toContainText("Nenhuma criatura");
    for (const w of WILD_WIDTHS) {
      await shootWild(p, "ficha-criaturas", w, scheme, panel);
    }
    await cropWild(block, "recorte-ficha-forma", scheme);

    // 584-04, 584-05: the master reads the time in the order row and in the act-as card.
    await m.goto(sessionRoute(campaignId));
    for (const w of WILD_WIDTHS.length ? WILD_WIDTHS : [1280]) {
      await m.setViewportSize({ width: w, height: WILD_VIEWPORTS[w] });
      const order = m.locator("app-order-list");
      await expect(order).toContainText("7/11");
      await shootWild(m, "mestre-ordem-fechada", w, scheme, order);
      await openOrderRow(m, "Sálvia");
      await expect(order).toContainText(/CA 13\s*·\s*7 de 11 PV do Lobo\s*·\s*a Sálvia tem \d+ de \d+/);
      await expect(order).toContainText(/Lobo\s*·\s*hourglass_empty\s*(\d+ h|\d+ min)\s*restantes/);
      await expect(order).not.toContainText("Na forma de Lobo");
      await shootWild(m, "mestre-ordem", w, scheme, order.getByRole("button", { name: /^Sálvia/ }));
      if (w === 1280) {
        await cropWild(order.locator("li", { hasText: "Sálvia" }).first(), "recorte-mestre-ordem", scheme);
      }
      await openTurn(m);
      const offer = m.locator("app-act-as-card");
      await expect(offer).toContainText(/Pontos de vida\s*7 de 11/);
      await shootWild(m, "mestre-agir", w, scheme, offer);
      if (w === 1280) {
        await cropWild(offer, "recorte-mestre-agir", scheme);
      }
      await m.getByRole("button", { name: "Agir por Sálvia" }).click();
      await expect(m.getByText("Você está agindo por Sálvia")).toBeVisible();
      await m.getByRole("button", { name: "Voltar ao mestre" }).click();
    }
    await m.setViewportSize({ width: 1280, height: 800 });

    // 584-04 card 7: the master's effects row carries the time and "Encerrar".
    const effectsRow = () =>
      m
        .getByRole("region", { name: "Efeitos em jogo" })
        .first()
        .locator("li.fx__row", { hasText: "Forma Selvagem" });
    for (const w of WILD_WIDTHS.length ? WILD_WIDTHS : [1280]) {
      await m.setViewportSize({ width: w, height: WILD_VIEWPORTS[w] });
      await openCombatPanel(m, "Efeitos");
      await expect(effectsRow()).toContainText(
        /Duração e fim: .*(h|min) restantes|rodadas?/,
      );
      await shootWild(m, "mestre-efeitos", w, scheme, effectsRow());
      if (w === 1280) {
        await cropWild(effectsRow(), "recorte-mestre-tempo", scheme);
      }
    }
    await m.setViewportSize({ width: 1280, height: 800 });
    const row = effectsRow();

    // 584-06 card 8: the form ended between the reading and the tap: refused, nothing spent, never a new form. Her page loses the
    // table (offline) while the master ends the form, and taps "Permanecer" the moment the line is back, before its stream is.
    await p.goto(sessionRoute(campaignId));
    await expect(p.locator("app-form-chip").first()).toContainText(
      "Na forma de Lobo",
    );
    await openActionTab(p, "Druida");
    const staleStay = p.getByRole("button", { name: "Permanecer na forma" });
    await expect(staleStay).toBeVisible();
    await contexts[1].setOffline(true);
    await row.getByRole("button", { name: /Encerrar/ }).click();
    const confirm = m.getByRole("alertdialog");
    if (await confirm.isVisible().catch(() => false)) {
      await confirm.getByRole("button", { name: /Encerrar/ }).click();
    }
    await expect(row).toHaveCount(0);
    await contexts[1].setOffline(false);
    await staleStay.click();
    await expect(p.getByText("A forma já acabou").first()).toBeVisible();
    for (const w of WILD_WIDTHS) {
      await shootWild(p, "forma-acabou", w, scheme);
    }
    await expect(p.getByText("Na forma de Lobo")).toHaveCount(0);
    await endTurnOf(p, m, campaignId, "Sálvia");
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await Promise.all(contexts.map((c) => c.close()));
  }
}

// 584-06 card 9: with no session open the sheet still says which form it is, and the two buttons wait.
async function wildShapeOutsideASession(
  browser: Browser,
  scheme: "light" | "dark",
): Promise<void> {
  const viewport = { width: 1280, height: 800 };
  const contexts = await Promise.all(
    (["Mestre Teste", "Jogador Teste", "E-mail Não Verificado"] as const).map(
      (user) =>
        browser.newContext({
          storageState: authStatePath(user),
          colorScheme: scheme,
          viewport,
          ...wildShotScale,
        }),
    ),
  );
  const [m, p, t] = await Promise.all(contexts.map((c) => c.newPage()));
  let campaignId = "";
  try {
    await Promise.all([m.goto("/"), p.goto("/"), t.goto("/")]);
    const table = await tableForCreatureCombat(
      m,
      p,
      t,
      `Forma sem sessão ${Date.now()}`,
    );
    campaignId = table.campaignId;
    await beginCreatureCombat(m, table, { Sálvia: p, Toren: t }, FACES);
    await passTurnsTo(m, campaignId, "Sálvia");
    await p.goto(sessionRoute(campaignId));
    await openActionTab(p, "Druida");
    await p
      .getByRole("button", { name: "Transformar: Forma Selvagem" })
      .click();
    const sheet = p.getByRole("dialog", { name: "Forma Selvagem" });
    await sheet.getByLabel("Buscar fera").fill("wolf");
    await sheet.locator("label", { hasText: "(Wolf)" }).click();
    await sheet.getByRole("button", { name: "Virar Lobo" }).click();
    await expect(p.locator("app-form-chip").first()).toContainText(
      "Na forma de Lobo",
    );
    await endOpenSessionRPC(m, campaignId);

    await p.goto(`/campaigns/${campaignId}/characters/${table.heroId}`);
    const panel = p.locator("app-creatures-panel");
    const block = panel.locator(".form");
    await expect(block).toContainText("Na forma de Lobo");
    await expect(block).toContainText(/\d+ de \d+ PV/);
    await expect(block).not.toContainText(/restantes|rodadas?|\d+ h|\d+ min/);
    for (const name of [/Voltar à forma normal/, /Permanecer na forma/]) {
      await expect(block.getByRole("button", { name })).toHaveAttribute(
        "aria-disabled",
        "true",
      );
    }
    await expect(block).toContainText("Só durante um combate ou uma sessão.");
    for (const w of WILD_WIDTHS) {
      await shootWild(p, "ficha-sem-sessao", w, scheme, panel);
    }
    await cropWild(block, "recorte-ficha-sem-sessao", scheme);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId).catch(() => undefined);
    }
    await Promise.all(contexts.map((c) => c.close()));
  }
}

test(
  "a tela da forma selvagem: a faixa com o tempo, a linha da vez, Permanecer, a ficha e o mestre, no tema claro",
  { tag: ["@MR-037", "@RN-20"] },
  async ({ browser }) => {
    test.setTimeout(420_000);
    await wildShapeScreens(browser, "light");
  },
);

test(
  "a tela da forma selvagem: a faixa com o tempo, a linha da vez, Permanecer, a ficha e o mestre, no tema escuro",
  { tag: ["@MR-037", "@RN-20"] },
  async ({ browser }) => {
    test.setTimeout(420_000);
    await wildShapeScreens(browser, "dark");
  },
);

test(
  "a ficha da forma selvagem sem sessão aberta mostra a forma, sem tempo, e os dois botões esperando, no tema claro",
  { tag: ["@MR-037", "@RN-20"] },
  async ({ browser }) => {
    test.setTimeout(240_000);
    await wildShapeOutsideASession(browser, "light");
  },
);

test(
  "a ficha da forma selvagem sem sessão aberta mostra a forma, sem tempo, e os dois botões esperando, no tema escuro",
  { tag: ["@MR-037", "@RN-20"] },
  async ({ browser }) => {
    test.setTimeout(240_000);
    await wildShapeOutsideASession(browser, "dark");
  },
);
