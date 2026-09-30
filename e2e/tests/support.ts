import { expect, type APIResponse, type Page } from '@playwright/test';

// devidp's issuer (deploy/local/compose.yaml). The browser resolves any
// *.localhost name to 127.0.0.1 by itself.
export const idpOrigin = process.env.E2E_IDP_ORIGIN ?? 'http://idp.localhost:9090';

export const sessionCookie = '__Host-meurpg_session';
export const loginCookie = '__Host-meurpg_login';

/** The test users devidp lists on its login page (oidctest.TestUsers). */
export type TestUser = 'Mestre Teste' | 'Jogador Teste';

/**
 * Signs in through devidp, the way a person would: open the sign-in URL,
 * pick a user on the provider's page, and land back on returnTo.
 *
 * It opens /auth/login directly, which is faster and keeps these tests about
 * the server. ui.spec.ts covers the same flow through the app's buttons.
 */
export async function signIn(page: Page, user: TestUser = 'Mestre Teste', returnTo = '/'): Promise<void> {
  await page.goto(`/auth/login?return_to=${encodeURIComponent(returnTo)}`);
  await expect(page).toHaveURL((url) => url.origin === idpOrigin && url.pathname === '/authorize');
  await page.getByRole('button', { name: user, exact: true }).click();
  // A relative URL is resolved against baseURL: back on the app, at returnTo.
  await expect(page).toHaveURL(returnTo);
}

/**
 * Calls a unary Connect RPC with the JSON codec, through the page's own
 * cookies. Every Connect call must carry Connect-Protocol-Version: 1 (the
 * server's CSRF protection, docs/arquitetura.md#csrf).
 *
 * Use it for what the page does not show; ui.spec.ts asserts on the page.
 */
export function callRPC(page: Page, method: string, body: object = {}): Promise<APIResponse> {
  return page.request.post(`/${method}`, {
    data: body,
    headers: { 'Connect-Protocol-Version': '1' },
  });
}

export const getMe = (page: Page) => callRPC(page, 'meurpg.identity.v1.IdentityService/GetMe');
export const signOut = (page: Page) => callRPC(page, 'meurpg.identity.v1.IdentityService/SignOut');

export const day = 24 * 60 * 60 * 1000;

/**
 * Creates a campaign through the "Nova campanha" form on `/campanhas`, the
 * way MR-001 asks for, and returns its id from the `/campanhas/<id>` URL
 * the app navigates to afterwards.
 *
 * Tests whose own story is not campaign creation (MR-002, MR-003) use this
 * as setup, so their assertions stay about invites and membership, not
 * about the form they don't need to prove again.
 */
export async function createCampaign(page: Page, name: string): Promise<string> {
  await page.goto('/campanhas');
  await page.getByLabel('Nome da campanha').fill(name);
  await page.getByLabel('Modo de XP').click();
  await page.getByRole('option', { name: 'Por inimigos derrotados' }).click();
  await page.getByRole('button', { name: 'Criar campanha' }).click();

  await expect(page).toHaveURL(/\/campanhas\/[^/]+$/);
  return page.url().split('/').pop()!;
}
