import { expect } from '@playwright/test';
import { test } from './diagnostics';

// A tab left open across a deploy cannot load the old hashed chunks: the server answers 404 or index.html.
// The app reloads the page once for it, and a second failure on the same entry does not reload again.

// What the server answers for a chunk of a past build: a 404, or index.html (the app shell) with a 200.
const goneAnswers = {
  '404': { status: 404, contentType: 'text/plain', body: 'gone' },
  'index.html': {
    status: 200,
    contentType: 'text/html',
    body: '<!doctype html><html></html>',
  },
};

for (const [answer, gone] of Object.entries(goneAnswers)) {
  test(`uma parte preguiçosa que falha ao carregar (${answer}) recarrega a página uma única vez`, async ({ page }) => {
    const loadedScripts = new Set<string>();
    let breakNewScripts = false;
    await page.route('**/*.js', (route) => {
      const url = route.request().url();
      if (!breakNewScripts || loadedScripts.has(url)) {
        loadedScripts.add(url);
        return route.continue();
      }
      return route.fulfill(gone);
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
    // The app's error handler logs the failed chunk: that is the signal that the second failure was handled.
    const handled = page.waitForEvent('console', (message) => /^ERROR .*(dynamically imported module|module script)/i.test(message.text()));
    await footer.getByRole('link', { name: 'Termos de uso' }).click();
    await handled;
    await expect(page.getByRole('heading', { level: 1, name: 'Termos de uso' })).toHaveCount(0);
    await expect(page.getByText('Servidor conectado')).toBeVisible();
    expect(documentLoads).toBe(2);
  });
}
