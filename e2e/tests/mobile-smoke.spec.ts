import { expect, type Browser, type BrowserContext, type Page } from '@playwright/test';
import { test } from './diagnostics';

import { openDice } from './free-dice-support';
import { endOpenSessionRPC, openSessionPage, openTableRolls, startSessionRPC, tableWithPensantus } from './live-session-support';
import { callRPC, signIn, type TestUser } from './support';

// A smoke of the screens a phone uses: on WebKit and an iPhone (playwright.mobile.config.ts, over https) and on Chrome (the default config).
// It runs in the viewport of its project: nothing here forces a width.
// The stream closes after 2 minutes hidden (web/src/app/core/play/live-stream.ts, HIDDEN_CLOSE_MS).
const hiddenCloseMs = 2 * 60_000;

/**
 * A fresh context signed in through the real devidp flow, and its page. No saved state: the mobile config (https) has no setup project,
 * and cookies saved over http are not valid on its https origin. The test closes the contexts and never signs out.
 */
async function signedIn(browser: Browser, user: TestUser, fakeClock = false): Promise<{ context: BrowserContext; page: Page }> {
  const context = await browser.newContext();
  // Closing the context on a failure is the caller's; a sign-in that fails leaves it to the test's teardown.
  if (fakeClock) {
    // Installed before the page loads, so the stream's timers run on it; it ticks with real time until the test jumps it.
    await context.clock.install();
  }
  const page = await context.newPage();
  await signIn(page, user, '/');
  // The page's own request, with the cookies the browser kept: the browser itself is signed in, not only the test's API client.
  const status = await page.evaluate(async () => {
    const res = await fetch('/meurpg.identity.v1.IdentityService/GetMe', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1' },
      body: '{}',
    });
    return res.status;
  });
  expect(status).toBe(200);
  return { context, page };
}

/** Makes the page report itself hidden, as a phone does when the browser goes to the background. */
async function hideTab(page: Page): Promise<void> {
  await page.evaluate(() => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'hidden' });
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => true });
    document.dispatchEvent(new Event('visibilitychange'));
  });
}

/** Removes what `hideTab` set, so the page reports the browser's own state (visible), and tells the page. */
async function showTab(page: Page): Promise<void> {
  await page.evaluate(() => {
    Reflect.deleteProperty(document, 'visibilityState');
    Reflect.deleteProperty(document, 'hidden');
    document.dispatchEvent(new Event('visibilitychange'));
  });
}

test(
  'entra pelo login real e o jogador rola 1d20 em Dados e, com a aba escondida e mostrada de novo, recebe a rolagem do mestre',
  { tag: ['@smoke', '@MR-001', '@MR-052'] },
  async ({ browser }) => {
    test.setTimeout(120_000);
    // Both sign in for real, in the project's own browser (a login allows 20 sign-ins at once, so the smoke makes only these two).
    const [{ context: master, page: m }, { context: player, page: p }] = await Promise.all([
      signedIn(browser, 'Mestre Teste'),
      signedIn(browser, 'Jogador Teste', true),
    ]);
    const { campaignId } = await tableWithPensantus(m, p, `Smoke móvel ${Date.now()}`);
    await startSessionRPC(m, campaignId);
    await Promise.all([openSessionPage(m, campaignId), openSessionPage(p, campaignId)]);
    try {
      // The player opens "Dados" (a bottom sheet on a phone, a dialog on a desktop) and rolls 1d20.
      const sheet = await openDice(p);
      await sheet.getByRole('button', { name: 'Rolar 1d20', exact: true }).click();
      const card = sheet.locator('app-free-roll-card');
      await expect(card.locator('.card__line')).toContainText('1d20 = ');
      await p.keyboard.press('Escape');
      await expect(sheet).toBeHidden();

      // The tab goes to the background and stays there past the stream's limit: the stream closes, which the header shows as
      // "Dados" described by the reconnecting notice.
      const dice = p.getByRole('button', { name: 'Dados', exact: true });
      await expect(dice).not.toHaveAttribute('aria-describedby', 'dice-off');
      await hideTab(p);
      await p.clock.fastForward(hiddenCloseMs + 5_000);
      await expect(dice).toHaveAttribute('aria-describedby', 'dice-off');

      // The master rolls while the player's stream is closed.
      const rolled = await callRPC(m, 'meurpg.play.v1.PlayService/RollFreeDice', {
        campaignId,
        sides: 20,
        count: 1,
        idempotencyKey: `smoke-${Date.now()}`,
      });
      expect(rolled.ok()).toBeTruthy();
      const logText = (await rolled.json()).roll.logText as string;
      expect(logText).toContain('1d20 = ');
      const masterRow = (await openTableRolls(m)).getByRole('listitem').filter({ hasText: logText });
      await expect(masterRow).toHaveCount(1);
      // The player's page has not heard of it: the stream is closed.
      const playerLog = await openTableRolls(p);
      await expect(playerLog.getByRole('listitem').filter({ hasText: logText })).toHaveCount(0);

      // The tab comes back: the stream reconnects, reads a new snapshot, and the roll shows in the table's log.
      await showTab(p);
      await expect(dice).not.toHaveAttribute('aria-describedby', 'dice-off');
      await expect(playerLog.getByRole('listitem').filter({ hasText: logText })).toHaveCount(1);
    } finally {
      await endOpenSessionRPC(m, campaignId);
      await master.close();
      await player.close();
    }
  },
);
