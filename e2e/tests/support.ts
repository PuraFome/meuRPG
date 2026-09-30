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
 * TODO(web sign-in UI): the "Entrar" button of the Angular app is being
 * built. When it lands, start from the app and click it instead of opening
 * /auth/login directly.
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
 * TODO(web sign-in UI): once the app shows who is signed in, assert on the
 * page as well, and keep these calls for what the page does not show.
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
