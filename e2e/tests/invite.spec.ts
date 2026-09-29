import { expect, test } from '@playwright/test';

import { createCampaign, idpOrigin, signIn } from './support';

// MR-003 (entrar pelo convite): a second person, already signed in, opens
// the invite link and joins; the master sees them in the members list.

test('jogador já logado abre o link do convite, entra na campanha e o mestre o vê nos membros', { tag: '@MR-003' }, async ({ page, browser }) => {
  await signIn(page, 'Mestre Teste', '/');
  const campaignId = await createCampaign(page, `Mesa conjunta ${Date.now()}`);
  await page.getByRole('button', { name: 'Gerar convite' }).click();
  await expect(page.getByText('não será mostrado de novo')).toBeVisible();
  const link = (await page.locator('.invite-reveal__link').textContent())?.trim();
  expect(link).toBeTruthy();

  const playerContext = await browser.newContext();
  try {
    const playerPage = await playerContext.newPage();
    await signIn(playerPage, 'Jogador Teste', '/');

    await playerPage.goto(link!);
    // AcceptInvite runs as soon as the page loads (already signed in), then
    // the app navigates to the campaign.
    await expect(playerPage).toHaveURL(new RegExp(`/campanhas/${campaignId}$`));
    await expect(playerPage.getByRole('heading', { level: 1 })).toContainText('Mesa conjunta');
  } finally {
    await playerContext.close();
  }

  // Back on the master's page: reload to see the new member.
  await page.reload();
  await expect(page.getByText('jogador')).toBeVisible();
});

// The sign-in-through-invite path: the person opens the invite link signed
// out, clicks "Entrar para aceitar o convite", picks a devidp user, and lands
// on the campaign, joined. The page posts the token in the body of POST
// /auth/login (intent=campaign_invite); the server keeps only its hash in the
// login state and accepts the invite after the callback. The token must never
// appear in any URL the browser requests.
test(
  'visitante sem sessão entra pelo convite, faz login e é adicionado à campanha automaticamente',
  { tag: '@MR-003' },
  async ({ page, browser }) => {
    await signIn(page, 'Mestre Teste', '/');
    const campaignId = await createCampaign(page, `Convite sem sessão ${Date.now()}`);
    await page.getByRole('button', { name: 'Gerar convite' }).click();
    await expect(page.getByText('não será mostrado de novo')).toBeVisible();
    const link = (await page.locator('.invite-reveal__link').textContent())?.trim();

    const guestContext = await browser.newContext();
    try {
      const guestPage = await guestContext.newPage();
      const token = new URL(link!).hash.replace(/^#t=/, '');
      expect(token.length).toBeGreaterThan(0);
      const requestedURLs: string[] = [];
      guestPage.on('request', (request) => requestedURLs.push(request.url()));

      await guestPage.goto(link!);
      await guestPage.getByRole('button', { name: 'Entrar para aceitar o convite' }).click();

      await expect(guestPage).toHaveURL((url) => url.origin === idpOrigin && url.pathname === '/authorize');
      await guestPage.getByRole('button', { name: 'Jogador Teste', exact: true }).click();

      await expect(guestPage).toHaveURL(new RegExp(`/campanhas/${campaignId}$`));
      // Playwright reports the URL without the fragment, so the invite page's
      // own URL does not count; any other appearance would be a leak.
      expect(requestedURLs.filter((url) => url.includes(token))).toEqual([]);
    } finally {
      await guestContext.close();
    }

    // Back on the master's page: the guest is now a member.
    await page.reload();
    await expect(page.getByText('jogador')).toBeVisible();
  },
);
