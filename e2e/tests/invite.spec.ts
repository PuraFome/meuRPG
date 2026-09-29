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

// The sign-in-through-invite path (person opens the invite link signed out,
// clicks "Entrar para aceitar o convite", picks a devidp user, and lands on
// the campaign, joined) depends on the backend contract described in
// docs/arquitetura.md#frontend-web: a POST /auth/login with
// intent=campaign_invite and intent_payload=<token>, being built on branch
// feat/invite-signin (../meuRPG-invite as of this writing, not yet merged
// into this branch). The frontend side (InviteAccept.signInToAccept) is
// already built and unit-tested (invite-accept.spec.ts). Enable this test
// once the orchestrator integrates that branch.
test.fixme(
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
      await guestPage.goto(link!);
      await guestPage.getByRole('button', { name: 'Entrar para aceitar o convite' }).click();

      await expect(guestPage).toHaveURL((url) => url.origin === idpOrigin && url.pathname === '/authorize');
      await guestPage.getByRole('button', { name: 'Jogador Teste', exact: true }).click();

      await expect(guestPage).toHaveURL(new RegExp(`/campanhas/${campaignId}$`));
    } finally {
      await guestContext.close();
    }
  },
);
