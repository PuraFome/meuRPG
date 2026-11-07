import { expect } from '@playwright/test';
import { test } from './diagnostics';
import { tableWithPensantus } from './live-session-support';
import { authStatePath } from './support';

const DIR = '/tmp/claude-0/prs/572-carry';
const SIZES: [number, number][] = [[390, 844], [768, 1024], [1280, 800]];

for (const scheme of ['light', 'dark'] as const) {
  for (const [w, h] of SIZES) {
    test(`shots ${w} ${scheme}`, async ({ browser }) => {
      test.setTimeout(300_000);
      const opts = { colorScheme: scheme, viewport: { width: w, height: h }, deviceScaleFactor: 2 } as const;
      const master = await browser.newContext({ storageState: authStatePath('Mestre Teste'), ...opts });
      const player = await browser.newContext({ storageState: authStatePath('Jogador Teste'), ...opts });
      const m = await master.newPage();
      const p = await player.newPage();
      await m.goto('/'); await p.goto('/');
      const { campaignId, characterId } = await tableWithPensantus(m, p, `Carga ${w}${scheme} ${Date.now()}`);
      await p.goto(`/campaigns/${campaignId}/characters/${characterId}`);
      const col = p.locator('app-proficiency-column');
      await expect(col.getByText('Capacidade de carga')).toBeVisible();
      const line = col.locator('p.senses', { hasText: 'Capacidade de carga' });
      console.log(`LINE ${w} ${scheme}: ${(await line.innerText()).replace(/\s+/g, ' ')}`);
      await line.evaluate((e) => e.scrollIntoView({ block: 'center' }));
      await p.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
      await p.waitForTimeout(400);
      await p.screenshot({ path: `${DIR}/572-carry-${w}-${scheme}.png` });
      await master.close(); await player.close();
    });
  }
}
