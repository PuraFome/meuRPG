import { expect } from '@playwright/test';
import { test } from './diagnostics';

import { endOpenSessionRPC } from './live-session-support';
import { tableForScenes } from './scene-support';
import { authStatePath, newSignedInContext, waitForCampaignList } from './support';

// A request body over its procedure's read limit (docs/architecture/api.md#request-size-limits) is refused by the server
// while the browser may still be sending it. The screen must show the Portuguese refusal, not the sentence for a network
// failure: the person can fix the first and can only retry the second.
// These three guard that: nothing in the page turns a refusal into a network failure.
test.use({ storageState: authStatePath('Mestre Teste') });

const REFUSED_BY_A_LIMIT = 'O servidor recusou o pedido por um limite. Reduza o conteúdo ou tente de novo mais tarde.';
const NETWORK_FAILURE = 'Não foi possível falar com o servidor agora. Tente de novo em instantes.';

// The default read limit is 64 KiB: one size just over it, one many times over, as a full upload buffer would be.
const KIB = 1024;
for (const size of [80 * KIB, 3 * 1024 * KIB]) {
  test(`um pedido de ${size / KIB} KiB, acima do limite de leitura, mostra a recusa e não uma falha de rede`, { tag: '@MR-001' }, async ({ page }) => {
    await page.route('**/meurpg.campaigns.v1.CampaignService/CreateCampaign', (route) => {
      const body = JSON.parse(route.request().postData() ?? '{}') as Record<string, unknown>;
      return route.continue({ postData: JSON.stringify({ ...body, name: 'x'.repeat(size) }) });
    });
    await page.goto('/campaigns');
    await waitForCampaignList(page);
    await page.getByLabel('Nome da campanha').fill(`Limite ${Date.now()}`);
    await page.getByRole('combobox', { name: 'Modo de XP' }).click();
    await page.getByRole('option', { name: 'Por inimigos derrotados' }).click();
    const refused = page.waitForResponse((response) => response.url().endsWith('/CreateCampaign'));
    await page.getByRole('button', { name: 'Criar campanha' }).click();

    // The server answers 429 (resource_exhausted) before the body is read to its end.
    expect((await refused).status()).toBe(429);
    const alert = page.getByRole('alert').filter({ hasText: REFUSED_BY_A_LIMIT });
    await expect(alert).toBeVisible();
    await expect(page.getByText(NETWORK_FAILURE)).toHaveCount(0);
    await expect(page).toHaveURL(/\/campaigns$/);
  });
}

// A screen with its own wording for `resource_exhausted` (the scene's 30-clue limit) still shows the generic refusal when the
// refusal is the size of the request, and only its own limit when the limit is its own.
test('uma pista grande demais mostra a recusa por tamanho e não o limite de 30 pistas', { tag: '@MR-029' }, async ({ browser }) => {
  test.setTimeout(120_000);
  const masterContext = await newSignedInContext(browser, 'Mestre Teste');
  const playerContext = await newSignedInContext(browser, 'Jogador Teste');
  const master = await masterContext.newPage();
  const player = await playerContext.newPage();
  await master.goto('/');
  const table = await tableForScenes(master, player, `Limite ${Date.now()}`, false);
  try {
    await master.route('**/meurpg.maps.v1.MapService/AddSceneClue', (route) => {
      const body = JSON.parse(route.request().postData() ?? '{}') as Record<string, unknown>;
      return route.continue({ postData: JSON.stringify({ ...body, text: 'x'.repeat(80 * KIB) }) });
    });
    await master.goto(`/campaigns/${table.campaignId}/maps/${table.mapId}`);
    await master.getByRole('button', { name: /^A carroça tombada, Cena de RP/ }).click();
    await master.getByRole('button', { name: 'Adicionar pista' }).click();
    const form = master.getByRole('form', { name: 'Nova pista' });
    await form.getByLabel('Texto da pista').fill('Pegadas de botas na lama.');
    await form.getByRole('button', { name: 'Adicionar pista' }).click();

    await expect(form.getByText(REFUSED_BY_A_LIMIT)).toBeVisible();
    await expect(master.getByText('Limite de 30 pistas')).toHaveCount(0);
  } finally {
    await endOpenSessionRPC(master, table.campaignId);
    await masterContext.close();
    await playerContext.close();
  }
});
