import { test as setup } from '@playwright/test';

import { authStatePath, signIn } from './support';

// Playwright's standard "setup project" pattern: signs in once per test
// user this suite needs, and saves each signed-in `storageState` (cookies)
// to a gitignored file (`authStatePath`, `e2e/.gitignore`) that every other
// test reuses instead of hitting the real, rate-limited `/auth/login`
// again (`backend/internal/identity/login.go`'s 20-per-client burst).
// `playwright.config.ts`'s "chrome" project has `dependencies: ['setup']`,
// so these two run first, in the "setup" project, before anything else.
//
// Only "Mestre Teste" and "Jogador Teste" are needed: no spec signs in as
// "E-mail Não Verificado" today. Add a third `setup(...)` here, with its
// own `authStatePath` case, if one ever does.

setup('autenticar como Mestre Teste', async ({ page }) => {
  await signIn(page, 'Mestre Teste', '/');
  await page.context().storageState({ path: authStatePath('Mestre Teste') });
});

setup('autenticar como Jogador Teste', async ({ page }) => {
  await signIn(page, 'Jogador Teste', '/');
  await page.context().storageState({ path: authStatePath('Jogador Teste') });
});
