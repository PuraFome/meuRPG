import type { Locator, Page } from '@playwright/test';
import { mkdirSync } from 'node:fs';
import { join } from 'node:path';

// Screenshots of the attunement screens (#561) when `E2E_SHOTS_DIR` names a folder; otherwise nothing is written.

const shotsDir = process.env.E2E_SHOTS_DIR;

/** A context that takes screenshots draws at 2x, so the crops are sharp. */
export const attunementScale = shotsDir ? { deviceScaleFactor: 2 } : {};

export const ATT_WIDTHS = [390, 768, 1280] as const;
const HEIGHTS: Record<number, number> = { 390: 844, 768: 1024, 1280: 800 };

export function attViewport(width: number): { width: number; height: number } {
  return { width, height: HEIGHTS[width] ?? 800 };
}

const themeWord = (scheme: 'light' | 'dark') => (scheme === 'light' ? 'claro' : 'escuro');

/** Saves the viewport as `561-<screen>-<width>-<claro|escuro>.png`, scrolled to `at` first (or the top). */
export async function shootAtt(page: Page, screen: string, width: number, scheme: 'light' | 'dark', at?: Locator): Promise<void> {
  if (!shotsDir) {
    return;
  }
  mkdirSync(shotsDir, { recursive: true });
  await page.setViewportSize(attViewport(width));
  if (at) {
    await at.evaluate((el) => {
      el.style.scrollMarginTop = '140px';
      el.scrollIntoView({ block: el.getBoundingClientRect().height < innerHeight * 0.8 ? 'center' : 'start' });
    });
  } else {
    await page.evaluate(() => window.scrollTo(0, 0));
  }
  await page.screenshot({ path: join(shotsDir, `561-${screen}-${width}-${themeWord(scheme)}.png`) });
}

/** Saves a crop of one element, `561-<name>-recorte-<claro|escuro>.png`, at the context's 2x scale. */
export async function cropAtt(at: Locator, name: string, scheme: 'light' | 'dark'): Promise<void> {
  if (!shotsDir) {
    return;
  }
  mkdirSync(shotsDir, { recursive: true });
  await at.scrollIntoViewIfNeeded();
  await at.screenshot({ path: join(shotsDir, `561-${name}-recorte-${themeWord(scheme)}.png`) });
}
