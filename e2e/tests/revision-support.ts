import { expect, type Page } from '@playwright/test';

import type { Encounter } from './combat-support';

/**
 * Waits until the page shows the combat at the revision a call returned, or a later one.
 *
 * A call that changes the server answers with the new `Encounter`, but the page learns of it from the stream or from a
 * read, a moment later. Acting on the page before that acts on the old state. The combat view exposes the combat it
 * shows as `data-encounter-id` and `data-encounter-revision` on `app-combat-view` (docs/testing.md, "Waiting for the
 * page to show a change"); this waits for that revision, and its failure says which one the page showed and which it
 * expected, so a stalled stream and a dropped push read differently from a slow page.
 *
 * Wait on the page the next action uses; if both the master and a player act next, pass both (`[m, p]`). A player's
 * copy on a fog-of-war map counts what the player saw, not the server's number, so its revision is not comparable
 * there: wait for that player's screen to show the change itself. With the fog off (or no map) a player's page works.
 */
export async function waitForEncounterRevision(pages: Page | Page[], expected: Encounter, what = 'the combat'): Promise<void> {
  await Promise.all([pages].flat().map((page) => waitOnePage(page, expected, what)));
}

async function waitOnePage(page: Page, expected: Encounter, what: string): Promise<void> {
  // A revision of 0 is left out of the protobuf JSON, so it arrives as undefined.
  const want = expected.revision ?? 0;
  // -1: no combat view on the page, or a view showing another combat (its number means nothing here). Both
  // attributes are read in one call, so a switch of combat between two reads cannot pair the wrong ones.
  const shown = async (): Promise<number> => {
    const attrs = await page
      .locator('app-combat-view')
      .first()
      .evaluate((el) => ({ id: el.getAttribute('data-encounter-id'), revision: el.getAttribute('data-encounter-revision') }), undefined, { timeout: 1_000 })
      .catch(() => null);
    return attrs?.id === expected.id ? Number(attrs.revision ?? '-1') : -1;
  };
  await expect
    .poll(shown, {
      message: `${what}: the page should show combat ${expected.id} at revision ${want} or later (-1 means no combat view, or another combat)`,
    })
    .toBeGreaterThanOrEqual(want);
}
