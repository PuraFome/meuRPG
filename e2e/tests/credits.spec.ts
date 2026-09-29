import fs from 'node:fs';
import path from 'node:path';

import { expect, test } from '@playwright/test';

// CC-BY-4.0 compliance (plan §1): the repo root's NOTICE carries the exact
// SRD 5.1 attribution text (ADR-0008, private), and the public /creditos
// page shows the same text. This test reads NOTICE at runtime instead of
// hardcoding the attribution, so it can never drift from what NOTICE
// actually says, and self-`fixme`s if NOTICE does not exist yet — WP-B
// writes it, in a parallel branch (see /scratchpad/etapa4-plan.md §3). The
// /creditos page itself is WP-D's; right now (Phase 1) NOTICE is also
// missing, so this test is fixme regardless.
const noticePath = path.resolve(__dirname, '..', '..', 'NOTICE');

/**
 * NOTICE's attribution paragraph: the exact CC-BY block ADR-0008 fixes, one
 * paragraph starting with "This work includes material...". NOTICE's
 * paragraphs are separated by a blank line (the licence text itself, the
 * CC-BY §3(a)(1)(B) "what we changed" line, the 5e-bits MIT notice); this
 * picks the one that names the System Reference Document.
 */
function readAttributionParagraph(): string {
  const text = fs.readFileSync(noticePath, 'utf-8');
  const paragraph = text.split(/\n\s*\n/).find((p) => p.includes('System Reference Document'));
  if (!paragraph) {
    throw new Error(`NOTICE exists but no paragraph mentions the System Reference Document: ${noticePath}`);
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
