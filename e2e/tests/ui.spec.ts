import { expect, test } from '@playwright/test';

import { callRPC, idpOrigin } from './support';

// The sign-in flow as a person does it: through the app's own buttons.
// The other login tests drive /auth/login directly and check the server's
// side; these check what the user sees.
test.describe('entrar e sair pela interface', () => {
  test('o mestre entra pelo botão "Entrar" e sai pelo "Sair" @MR-001', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'Entrar' }).click();

    await expect(page).toHaveURL((url) => url.origin === idpOrigin && url.pathname === '/authorize');
    await page.getByRole('button', { name: 'Mestre Teste', exact: true }).click();

    await expect(page).toHaveURL('/');
    await expect(page.getByText('Minha conta')).toBeVisible();

    await page.getByRole('button', { name: 'Sair' }).click();
    await expect(page.getByRole('button', { name: 'Entrar' })).toBeVisible();
    expect((await callRPC(page, 'meurpg.identity.v1.IdentityService/GetMe')).status()).toBe(401);
  });

  test('uma página protegida manda para o login e volta para ela depois', async ({ page }) => {
    await page.goto('/campanhas');

    await expect(page).toHaveURL((url) => url.origin === idpOrigin && url.pathname === '/authorize');
    await page.getByRole('button', { name: 'Jogador Teste', exact: true }).click();

    await expect(page).toHaveURL('/campanhas');
    await expect(page.getByRole('heading', { name: 'Minhas campanhas' })).toBeVisible();
  });
});
