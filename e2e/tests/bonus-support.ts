import { expect, type Locator, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { join } from "node:path";

import { callRPC } from "./support";

// Screenshots of the magic bonus screens (gear rows with the +N chip, the "Marcas" and "Bônus" sheets) when
// `E2E_SHOTS_DIR` names a folder; otherwise nothing is written. The context draws at 2x (`attunementScale`), so the crops are sharp.

const shotsDir = process.env.E2E_SHOTS_DIR;

const HEIGHTS: Record<number, number> = { 390: 844, 768: 1024, 1280: 800 };

const themeWord = (scheme: "light" | "dark") =>
  scheme === "light" ? "claro" : "escuro";

/** What the character of the bonus boards carries: a longsword with a mark, a dagger, a two-handed longbow carried, a shield and chain mail, each with its bonus. */
export const bonusSheet = {
  weaponKeys: ["equipment:longsword", "equipment:dagger", "equipment:longbow"],
  armorKey: "equipment:chain-mail",
  shield: true,
  carriedKeys: ["equipment:longbow"],
  weaponMarks: [{ weaponKey: "equipment:longsword", silvered: true }],
  itemBonuses: [
    { key: "equipment:longsword", bonus: 1 },
    { key: "equipment:dagger", bonus: 1 },
    { key: "equipment:longbow", bonus: 1 },
    { key: "equipment:shield", bonus: 1 },
    { key: "equipment:chain-mail", bonus: 2 },
  ],
};

/** The character of the sheet and combat boards: a marked longsword, the longbow carried, a shield and chain mail, each with its bonus. */
export const bonusCombatSheet = {
  ...bonusSheet,
  weaponKeys: ["equipment:longsword", "equipment:longbow"],
  itemBonuses: bonusSheet.itemBonuses.filter(
    (b) => b.key !== "equipment:dagger",
  ),
};

/** Saves the viewport as `564-<screen>-<width>-<claro|escuro>.png`, scrolled to `at` first (or the top). */
export async function shootBonus(
  page: Page,
  screen: string,
  width: number,
  scheme: "light" | "dark",
  at?: Locator,
): Promise<void> {
  if (!shotsDir) {
    return;
  }
  mkdirSync(shotsDir, { recursive: true });
  await page.setViewportSize({ width, height: HEIGHTS[width] ?? 800 });
  if (at) {
    await at.evaluate((el) => {
      el.style.scrollMarginTop = "140px";
      el.scrollIntoView({
        block:
          el.getBoundingClientRect().height < innerHeight * 0.8
            ? "center"
            : "start",
      });
    });
  } else {
    await page.evaluate(() => window.scrollTo(0, 0));
  }
  await page.screenshot({
    path: join(shotsDir, `564-${screen}-${width}-${themeWord(scheme)}.png`),
  });
}

/** Saves a crop of one element, `564-<name>-<width>-<claro|escuro>.png`, at the context's 2x scale. */
export async function cropBonus(
  at: Locator,
  name: string,
  width: number,
  scheme: "light" | "dark",
): Promise<void> {
  if (!shotsDir) {
    return;
  }
  mkdirSync(shotsDir, { recursive: true });
  await at.scrollIntoViewIfNeeded();
  await at.screenshot({
    path: join(shotsDir, `564-${name}-${width}-${themeWord(scheme)}.png`),
  });
}

/** The armor class the "Equipado" head shows. */
export async function armorClassOf(equipment: Locator): Promise<number> {
  const text = (await equipment.locator(".gear-group__ac").textContent()) ?? "";
  const n = /CA (\d+)/.exec(text);
  expect(n, `the head says "${text}"`).not.toBeNull();
  return Number(n![1]);
}

/** The master rewrites the bonuses on the stored sheet, behind the owner's open page: the owner's next save is stale. */
export async function masterSetsBonuses(
  master: Page,
  campaignId: string,
  characterId: string,
  change: (bonuses: { key: string; bonus: number }[]) => { key: string; bonus: number }[],
  weaponMarks?: { weaponKey: string; silvered?: boolean }[],
): Promise<void> {
  const got = await callRPC(
    master,
    "meurpg.characters.v1.CharacterService/GetCharacter",
    { campaignId, characterId },
  );
  expect(got.ok(), await got.text()).toBeTruthy();
  const character = (await got.json()).character as {
    name: string;
    revision: number;
    sheet: { full: { itemBonuses: { key: string; bonus: number }[] } };
  };
  const full = character.sheet.full;
  const put = await callRPC(
    master,
    "meurpg.characters.v1.CharacterService/UpdateCharacter",
    {
      campaignId,
      characterId,
      revision: character.revision,
      name: character.name,
      sheet: {
        full: {
          ...full,
          itemBonuses: change(full.itemBonuses),
          ...(weaponMarks ? { weaponMarks } : {}),
        },
      },
    },
  );
  expect(put.ok(), await put.text()).toBeTruthy();
}

/** The master changes the shield's bonus on the stored sheet. */
export const masterSetsShieldBonus = (
  master: Page,
  campaignId: string,
  characterId: string,
  bonus: number,
) =>
  masterSetsBonuses(master, campaignId, characterId, (bonuses) =>
    bonuses.map((b) => (b.key === "equipment:shield" ? { ...b, bonus } : b)),
  );

/**
 * Makes the page read the dagger twice: the server refuses a sheet that repeats a weapon key, so the "Adaga ×2" row comes from
 * the read of the character, with the count the server derives for a sheet that lists it twice.
 */
export async function readDaggerTwice(
  page: Page,
): Promise<() => Promise<void>> {
  const reads = "**/meurpg.characters.v1.CharacterService/GetCharacter";
  await page.route(reads, async (route) => {
    const response = await route.fetch();
    const body = (await response.json()) as {
      character: { derived: { gear: { key: string; count: number }[] } };
    };
    for (const g of body.character.derived.gear) {
      if (g.key === "equipment:dagger") {
        g.count = 2;
      }
    }
    await route.fulfill({ response, json: body });
  });
  return () => page.unroute(reads);
}
