import { expect, test } from '@playwright/test';

import { callRPC, idpOrigin, signIn } from './support';

// "Sair dos outros dispositivos" (ASVS 5.0 V7.4.3). The account "Sessões
// Teste" exists only for this: ending every other session of an account
// would break every other spec sharing it, so no saved state of it exists
// and nothing else signs in as it.
test.describe('sessões', () => {
  test('sair dos outros dispositivos termina a outra sessão e mantém esta', async ({ browser }) => {
    const laptop = await browser.newContext();
    const phone = await browser.newContext();
    try {
      const laptopPage = await laptop.newPage();
      const phonePage = await phone.newPage();
      await signIn(laptopPage, 'Sessões Teste', '/');
      await signIn(phonePage, 'Sessões Teste', '/');

      // The laptop sees the phone.
      await laptopPage.goto('/perfil');
      await expect(laptopPage.getByText('conectado em 1 outro dispositivo.')).toBeVisible();

      // Asking is not enough: the confirmation comes first, on the page.
      await laptopPage.getByRole('button', { name: 'Sair dos outros dispositivos' }).click();
      await expect(laptopPage.getByText('Não dá para desfazer.')).toBeVisible();
      await laptopPage.getByRole('button', { name: 'Confirmar saída' }).click();
      await expect(laptopPage.getByRole('status').filter({ hasText: 'Pronto: 1 dispositivo foi desconectado.' })).toBeVisible();

      // The laptop is still signed in; the phone is signed out for real.
      expect((await callRPC(laptopPage, 'meurpg.identity.v1.IdentityService/GetMe')).ok()).toBeTruthy();
      expect((await callRPC(phonePage, 'meurpg.identity.v1.IdentityService/GetMe')).status()).toBe(401);
      await phonePage.goto('/perfil');
      await expect(phonePage).toHaveURL((url) => url.origin === idpOrigin && url.pathname === '/authorize');

      // And the laptop now has nobody to sign out.
      await laptopPage.goto('/perfil');
      await expect(laptopPage.getByText('Você não está conectado em outros dispositivos.')).toBeVisible();
    } finally {
      await laptop.close();
      await phone.close();
    }
  });
});
