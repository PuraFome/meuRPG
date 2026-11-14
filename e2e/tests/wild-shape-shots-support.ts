import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import type { Locator, Page } from '@playwright/test';

/** The folder the screenshots of the #584 screen go to when `E2E_SHOTS_DIR` names one; otherwise nothing is written. */
const shotsDir = process.env.E2E_SHOTS_DIR;

/** The device scale of a context that takes screenshots: 2 when they are asked for, else the browser's own. */
export const wildShotScale = shotsDir ? { deviceScaleFactor: 2 } : {};

/** The viewport of each width: a phone, a tablet and a laptop. The screens are judged at the fold, never full page. */
export const WILD_VIEWPORTS: Record<number, number> = { 390: 844, 768: 1024, 1280: 800 };

export const WILD_WIDTHS = shotsDir ? [390, 768, 1280] : [];

const fileOf = (screen: string, width: number | 'crop', scheme: 'light' | 'dark'): string => {
  mkdirSync(shotsDir!, { recursive: true });
  return join(shotsDir!, `584-${screen}-${width}-${scheme === 'light' ? 'claro' : 'escuro'}.png`);
};

/**
 * Saves the viewport at `width` as `584-<screen>-<width>-<claro|escuro>.png` when `E2E_SHOTS_DIR` is set (a normal run writes
 * nothing). `at` is brought into view first: centered, or at the top when it is taller than most of the screen.
 */
export async function shootWild(page: Page, screen: string, width: number, scheme: 'light' | 'dark', at?: Locator): Promise<void> {
  if (!shotsDir) {
    return;
  }
  await page.setViewportSize({ width, height: WILD_VIEWPORTS[width] ?? 800 });
  if (at) {
    await at.evaluate((el) => {
      el.scrollIntoView({ block: el.getBoundingClientRect().height < innerHeight * 0.8 ? 'center' : 'start' });
    });
  } else {
    await page.evaluate(() => window.scrollTo(0, 0));
  }
  await page.screenshot({ path: fileOf(screen, width, scheme) });
}

/** A close-up of one element, 2x (the context's scale), as `584-<name>-crop-<claro|escuro>.png`. */
export async function cropWild(at: Locator, name: string, scheme: 'light' | 'dark'): Promise<void> {
  if (!shotsDir) {
    return;
  }
  await at.scrollIntoViewIfNeeded();
  await at.screenshot({ path: fileOf(name, 'crop', scheme) });
}
