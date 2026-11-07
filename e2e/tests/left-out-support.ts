import { mkdirSync } from 'node:fs';
import { join } from 'node:path';

import { type Locator, type Page } from '@playwright/test';

import { listEntriesJSON } from './content-support';

// Setup for the "left out" specs (issue #736): the engine refuses an entry only when the data changes under it (a newer
// rule set), and the API refuses to save one it would refuse, so no call can make a refused entry. The test says it the
// way the server does: the answer of ListTableEntries gets the `left_out` it would have (field 4 of the response, a
// TableContentViolation each), for the entries the master has not yet archived, which is what the server does too.
// Protobuf allows a repeated field to follow the rest of the message, so the real answer is kept and the fields are added.

export interface FakeViolation {
  key: string;
  field: string;
  reason: string;
  message?: string;
}

/** From now on this page's ListTableEntries says these entries are refused, until the entry is archived. */
export async function refuseEntries(
  page: Page,
  campaignId: string,
  violations: readonly FakeViolation[],
  /** What the stored entry says, by key: the data that the engine refuses (a target that left the menu), as the API gives it. */
  stored: Readonly<Record<string, (entry: Record<string, any>) => void>> = {},
): Promise<void> {
  await page.route('**/meurpg.rules.v1.TableContentService/ListTableEntries', async (route) => {
    const real = await route.fetch();
    const archived = new Set((await listEntriesJSON(page, campaignId)).entries?.filter((e) => e.archived).map((e) => e.key as string));
    const live = violations.filter((v) => !archived.has(v.key));
    const headers = { ...real.headers() };
    delete headers['content-length'];
    delete headers['content-encoding'];
    const body = (await real.json()) as Record<string, unknown>;
    for (const entry of (body.entries ?? []) as Record<string, any>[]) {
      stored[entry.key]?.(entry);
    }
    await route.fulfill({ status: real.status(), headers, body: JSON.stringify({ ...body, leftOut: live.map((v) => ({ ...v, message: v.message ?? '' })) }) });
  });
}

/** The folder the screenshots go to when `E2E_SHOTS_DIR` names one; otherwise nothing is written. */
const shotsDir = process.env.E2E_SHOTS_DIR;

/** The device scale of a context that takes screenshots: 2 when they are asked for, else the browser's own. */
export const leftOutShotScale = shotsDir ? { deviceScaleFactor: 2 } : {};

/** The viewport of each width: a phone, a tablet and a laptop. The screens are judged at the fold, never full page. */
export const LEFT_OUT_VIEWPORTS: Record<number, number> = { 390: 844, 768: 1024, 1280: 800 };

/** Saves the viewport as `736-<screen>-<width>-<claro|escuro>.png` when `E2E_SHOTS_DIR` is set; a normal run writes nothing. */
export async function shootLeftOut(page: Page, screen: string, width: number, scheme: 'light' | 'dark', at?: Locator): Promise<void> {
  if (!shotsDir) {
    return;
  }
  mkdirSync(shotsDir, { recursive: true });
  await page.setViewportSize({ width, height: LEFT_OUT_VIEWPORTS[width] ?? 800 });
  if (at) {
    await at.evaluate((el) => {
      el.scrollIntoView({ block: el.getBoundingClientRect().height < innerHeight * 0.8 ? 'center' : 'start' });
    });
  } else {
    await page.evaluate(() => window.scrollTo(0, 0));
  }
  await page.screenshot({ path: join(shotsDir, `736-${screen}-${width}-${scheme === 'light' ? 'claro' : 'escuro'}.png`) });
}
