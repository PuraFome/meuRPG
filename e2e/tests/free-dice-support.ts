import {
  expect,
  type APIResponse,
  type Browser,
  type Locator,
  type Page,
} from "@playwright/test";

import {
  endOpenSessionRPC,
  openSessionPage,
  startSessionRPC,
  tableWithPensantus,
} from "./live-session-support";
import { callRPC, newSignedInContext } from "./support";

// Setup and helpers for the free dice roller specs (MR-052): a master and a player on one open session, and the
// sheet of "Dados" as the screen offers it. Every test makes its own campaign.

export interface DiceTable {
  m: Page;
  p: Page;
  campaignId: string;
  done: () => Promise<void>;
}

/**
 * Mestre Teste and Jogador Teste (Pensantus) on an open session, both pages live. The widths pick the sheet of a phone
 * (under 768) or the dialog of a desktop for each.
 */
export async function diceTable(
  browser: Browser,
  name: string,
  widths: { master: number; player: number },
  colorScheme?: "light" | "dark",
): Promise<DiceTable> {
  const master = await newSignedInContext(browser, "Mestre Teste", {
    viewport: { width: widths.master, height: 900 },
    colorScheme,
  });
  const player = await newSignedInContext(browser, "Jogador Teste", {
    viewport: { width: widths.player, height: 900 },
    colorScheme,
  });
  const m = await master.newPage();
  const p = await player.newPage();
  await Promise.all([m.goto("/"), p.goto("/")]);
  const { campaignId } = await tableWithPensantus(
    m,
    p,
    `${name} ${Date.now()}`,
  );
  await startSessionRPC(m, campaignId);
  await Promise.all([
    openSessionPage(m, campaignId),
    openSessionPage(p, campaignId),
  ]);
  return {
    m,
    p,
    campaignId,
    done: async () => {
      await endOpenSessionRPC(m, campaignId);
      await master.close();
      await player.close();
    },
  };
}

/** The header's "Dados" button opens the sheet; returns it (a dialog on a desktop, a bottom sheet on a phone). */
export async function openDice(page: Page): Promise<Locator> {
  await page.getByRole("button", { name: "Dados", exact: true }).click();
  const sheet = page.getByRole("dialog", { name: "Rolar dados" });
  await expect(sheet).toBeVisible();
  await expect(
    sheet.getByText("Atalhos (preenchem o formulário)"),
  ).toBeVisible();
  return sheet;
}

/** Steps the modifier box from `from` to `to` with its − and + buttons, and waits for the box to show `to`. */
export async function stepModifier(
  sheet: Locator,
  from: number,
  to: number,
): Promise<void> {
  const by = to - from;
  const button = sheet.getByRole("button", {
    name: by > 0 ? "Aumentar o modificador" : "Diminuir o modificador",
  });
  for (let i = 0; i < Math.abs(by); i += 1) {
    await button.click();
  }
  await expect(sheet.locator("#free-mod")).toHaveValue(
    to === 0 ? "0" : to > 0 ? `+${to}` : `−${Math.abs(to)}`,
  );
}

/** The faces of the dice in a result card, and the total it shows. */
export async function cardNumbers(
  card: Locator,
): Promise<{ faces: number[]; total: number }> {
  const faces = (await card.locator(".die__box").allTextContents()).map((t) =>
    Number(t.trim()),
  );
  const total = Number(
    (await card.locator(".card__total").textContent())?.trim(),
  );
  return { faces, total };
}

/** The roll as the server keeps it, newest first: `table` is what the caller may read, `mine` the caller's own. */
export async function freeRollsRPC(
  page: Page,
  campaignId: string,
): Promise<{ table: FreeRollJson[]; mine: FreeRollJson[]; text: string }> {
  const res = await callRPC(page, "meurpg.play.v1.PlayService/ListFreeRolls", {
    campaignId,
  });
  expect(res.ok()).toBeTruthy();
  const text = await res.text();
  const body = JSON.parse(text) as {
    table?: FreeRollJson[];
    mine?: FreeRollJson[];
  };
  return { table: body.table ?? [], mine: body.mine ?? [], text };
}

export interface FreeRollJson {
  id: string;
  formula: string;
  logText: string;
  secret?: boolean;
  total?: number;
}

/**
 * Rolls 1d6 in parallel batches of 50 until the server refuses one for too many rolls (`429`), and returns how many
 * were granted before and the first refusal. The burst is 20 times the stack's rate multiplier, so the number of
 * batches is not fixed; 600 rolls is far beyond any setting.
 */
export async function rollUntilRefused(
  page: Page,
  campaignId: string,
): Promise<{ granted: number; refused: APIResponse }> {
  const batch = 50;
  let granted = 0;
  for (let round = 0; round < 12; round += 1) {
    const answers = await Promise.all(
      Array.from({ length: batch }, (_, i) =>
        callRPC(page, "meurpg.play.v1.PlayService/RollFreeDice", {
          campaignId,
          sides: 6,
          count: 1,
          idempotencyKey: `limit-${round}-${i}-${Date.now()}`,
        }),
      ),
    );
    granted += answers.filter((a) => a.ok()).length;
    const refused = answers.find((a) => a.status() === 429);
    if (refused) {
      return { granted, refused };
    }
  }
  throw new Error("no roll was refused for too many rolls");
}
