import { expect } from '@playwright/test';
import { test } from './diagnostics';

// A tab left open across a deploy cannot load the old hashed chunks: the server answers 404 or index.html.
// The app reloads the page once for it, and a second failure on the same entry does not reload again.

test('uma parte preguiçosa que falha ao carregar recarrega a página uma única vez', async ({ page }) => {
  const loadedScripts = new Set<string>();
  let breakNewScripts = false;
  await page.route('**/*.js', (route) => {
    const url = route.request().url();
    if (!breakNewScripts || loadedScripts.has(url)) {
      loadedScripts.add(url);
      return route.continue();
    }
    return route.fulfill({ status: 404, contentType: 'text/plain', body: 'gone' });
  });

  let documentLoads = 0;
  page.on('request', (request) => {
    if (request.isNavigationRequest() && request.frame() === page.mainFrame()) {
      documentLoads++;
    }
  });

  await page.goto('/');
  await expect(page.getByText('Servidor conectado')).toBeVisible();
  expect(documentLoads).toBe(1);

  // From here on, every script the page did not load yet is "gone", as after a deploy.
  breakNewScripts = true;
  const footer = page.getByRole('navigation', { name: 'Rodapé' });
  await footer.getByRole('link', { name: 'Termos de uso' }).click();

  // The reload is the deterministic event: the document request count reaches 2.
  await expect.poll(() => documentLoads).toBe(2);
  await expect(page.getByText('Servidor conectado')).toBeVisible();

  // A second failure on this entry: the marker (kept by the page, not by the router) forbids another reload.
  const failedChunk = page.waitForResponse((response) => response.status() === 404);
  await footer.getByRole('link', { name: 'Termos de uso' }).click();
  await failedChunk;
  await expect(page.getByRole('heading', { level: 1, name: 'Termos de uso' })).toHaveCount(0);
  await expect(page.getByText('Servidor conectado')).toBeVisible();
  expect(documentLoads).toBe(2);
});
