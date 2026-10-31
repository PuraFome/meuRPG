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
 * Wait on the master's page. A player's copy on a fog-of-war map counts what the player saw, not the server's number,
 * so its revision is not comparable.
 */
export async function waitForEncounterRevision(page: Page, expected: Encounter, what = 'the combat'): Promise<void> {
  const view = page.locator('app-combat-view');
  // -1: no combat view on the page, or a view showing another combat (its number means nothing here).
  const shown = async (): Promise<number> => {
    const id = await view.first().getAttribute('data-encounter-id', { timeout: 1_000 }).catch(() => null);
    if (id !== expected.id) {
      return -1;
    }
    return Number(await view.first().getAttribute('data-encounter-revision', { timeout: 1_000 }).catch(() => '-1'));
  };
  await expect
    .poll(shown, {
      message: `${what}: the page should show combat ${expected.id} at revision ${expected.revision} or later (-1 means no combat view, or another combat)`,
    })
    .toBeGreaterThanOrEqual(expected.revision);
}
