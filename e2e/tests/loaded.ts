import { expect, type Page } from '@playwright/test';

/**
 * Waits until the screen is done loading, by what a person would see: no
 * spinner and nothing marked `aria-busy`, and the web fonts in place (a late
 * font moves text and changes the contrast an axe scan reads).
 *
 * This replaces `waitForLoadState('networkidle')`, which Playwright
 * discourages: the live stream or an image still loading keeps the page from
 * ever going idle, and idle never meant "rendered".
 */
export async function expectLoaded(page: Page): Promise<void> {
  await expect(page.locator('mat-spinner, [aria-busy="true"]')).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => document.fonts.status)).toBe('loaded');
}
