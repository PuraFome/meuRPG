import fs from 'node:fs';
import path from 'node:path';

import { expect, test } from '@playwright/test';

// CC-BY-4.0 compliance: the repo root's NOTICE carries the exact SRD 5.1
// attribution text (ADR-0008, private), and the public /creditos page
// (web/src/app/pages/creditos) shows the same text, hardcoded from the same
// source. This test reads NOTICE at runtime instead of hardcoding the
// attribution itself, so it can never drift from what NOTICE actually says,
// and self-`fixme`s if NOTICE is ever missing (it exists as of phase 2).
const noticePath = path.resolve(__dirname, '..', '..', 'NOTICE');

/**
 * NOTICE's attribution paragraph: the exact CC-BY block ADR-0008 fixes, one
 * paragraph starting with "This work includes material...". NOTICE's own
 * section heading ("System Reference Document 5.1\n----...") also mentions
 * the SRD by name, so this matches on the attribution's own opening words
 * instead, which only the real paragraph has.
 */
function readAttributionParagraph(): string {
  const text = fs.readFileSync(noticePath, 'utf-8');
  const paragraph = text.split(/\n\s*\n/).find((p) => p.includes('This work includes material'));
  if (!paragraph) {
    throw new Error(`NOTICE exists but no paragraph starts with the CC-BY attribution: ${noticePath}`);
  }
  return paragraph.replace(/\s+/g, ' ').trim();
}

test('a página de créditos mostra a atribuição do SRD 5.1', { tag: '@licenca' }, async ({ page }) => {
  test.fixme(!fs.existsSync(noticePath), 'NOTICE not written yet (WP-B, backend/internal/rules) — see plan §3');

  const attribution = readAttributionParagraph();

  await page.goto('/creditos');
  const bodyText = (await page.textContent('body'))?.replace(/\s+/g, ' ').trim() ?? '';
  expect(bodyText).toContain(attribution);
});
