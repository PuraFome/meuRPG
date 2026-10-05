import { expect, test, type Browser, type BrowserContext, type Page } from '@playwright/test';

import { combatRPC, getEncounterRPC, startEncounterRPC, type CombatTable } from './combat-support';
import { moveTo, sessionRoute, tableForFog, visionOf, type FogTable } from './fog-support';
import { callRPC, newSignedInContext } from './support';

// The fog of war on screen (Etapa 9, MR-036, RN-10, RN-20; E9-03 and E9-04). The tests read the shading
// (`data-shaded` on the picture: how many squares of each state), the tokens the map draws and the words on
// the screen, never the pixels of the image.

interface Shading {
  dim: number;
  grey: number;
  remembered: number;
  unseen: number;
}

/** The squares of each state on the map the page draws. */
async function shading(page: Page): Promise<Shading> {
  await expect(page.locator('app-fog-base')).toHaveAttribute('data-shaded', /./);
  const raw = (await page.locator('app-fog-base').first().getAttribute('data-shaded')) ?? '';
  const read = (key: string) => Number(new RegExp(`${key}:(\\d+)`).exec(raw)?.[1] ?? NaN);
  return { dim: read('dim'), grey: read('grey'), remembered: read('remembered'), unseen: read('unseen') };
}

/** The names of the tokens the map draws (its list for assistive tech). */
async function tokens(page: Page): Promise<string[]> {
  return page.locator('app-fog-map ul[aria-label="No mapa"] li').allTextContents();
}

/** Waits for the last tile: the "Carregando o mapa" notice goes. */
async function loaded(page: Page): Promise<void> {
  await expect(page.locator('app-fog-base')).toBeVisible();
  await expect(page.getByTestId('fog-loading')).toHaveCount(0);
}

/** How many squares the caption says the viewer sees ("Você vê 38 de 384 quadrados à vista."). */
async function seen(page: Page): Promise<number> {
  const text = (await page.getByTestId('fog-caption').textContent()) ?? '';
  return Number(/Você vê\s*(\d+)\s+de/.exec(text.replace(/\s+/g, ' '))?.[1] ?? NaN);
}

async function atTable(browser: Browser, options: Parameters<typeof tableForFog>[4] = {}, name = 'Mirathel névoa') {
  const master = await newSignedInContext(browser, 'Mestre Teste');
  const pensantus = await newSignedInContext(browser, 'Jogador Teste');
  const toren = await newSignedInContext(browser, 'E-mail Não Verificado');
  const [mp, ap, bp] = await Promise.all([master.newPage(), pensantus.newPage(), toren.newPage()]);
  await Promise.all([mp.goto('/'), ap.goto('/'), bp.goto('/')]);
  const table = await tableForFog(mp, ap, bp, name, options);
  return { table, master, pensantus, toren, mp, ap, bp };
}

async function closeAll(...contexts: BrowserContext[]): Promise<void> {
  await Promise.all(contexts.map((c) => c.close()));
}

test('Pensantus e Toren veem mapas diferentes no mesmo instante, e nenhum baixa a imagem inteira @MR-036 @RN-10', async ({ browser }) => {
  const { table, master, pensantus, toren, ap, bp } = await atTable(browser);
  const fetched: string[] = [];
  for (const page of [ap, bp]) {
    page.on('request', (r) => fetched.push(new URL(r.url()).pathname));
  }
  await Promise.all([ap.goto(sessionRoute(table.campaignId)), bp.goto(sessionRoute(table.campaignId))]);
  await Promise.all([loaded(ap), loaded(bp)]);

  // Pensantus has darkvision (18 m): the cave near him is seen in grey; Toren has none and sees only the lit room.
  const a = await shading(ap);
  const b = await shading(bp);
  expect(a.grey).toBeGreaterThan(0);
  expect(a.grey).toBeGreaterThan(30);
  expect(b.grey).toBeLessThan(3);
  expect(a.unseen).toBeLessThan(b.unseen);
  expect(await seen(ap)).toBeGreaterThan(await seen(bp));

  // The enemies in the dark room are not on either map; the one in the torch's light is, for both.
  for (const page of [ap, bp]) {
    const names = (await tokens(page)).map((n) => n.trim());
    expect(names).toContain('Goblin 2');
    expect(names).not.toContain('Goblin 1');
    expect(names).not.toContain('Capitão Goblin');
  }
  // The party is on every map, even on a black square.
  expect((await tokens(bp)).map((n) => n.trim())).toEqual(expect.arrayContaining(['Pensantus', 'Toren (você)']));

  // The server's reads agree with what is drawn: the packed states of each player differ.
  const va = await visionOf(ap, table);
  const vb = await visionOf(bp, table);
  expect(va.states.filter((s) => s === 2).length).toBe(a.grey);
  expect(vb.states.filter((s) => s === 2).length).toBe(b.grey);

  // No request of a player fetched the whole image (`/images/<id>`): only the tiles of their own view.
  expect(fetched.filter((p) => /^\/images\/(?!maps\/)/.test(p))).toEqual([]);
  expect(fetched.some((p) => /^\/images\/maps\/[^/]+\/tiles\/\d+\/\d+$/.test(p))).toBe(true);
  await closeAll(master, pensantus, toren);
});

test('depois de andar, o que já foi visto continua, escurecido, sem inimigos @MR-036 @RN-10', async ({ browser }) => {
  const { table, master, pensantus, toren, mp, ap } = await atTable(browser);
  await ap.goto(sessionRoute(table.campaignId));
  await loaded(ap);
  expect((await tokens(ap)).map((n) => n.trim())).toContain('Goblin 2');
  expect((await shading(ap)).remembered).toBe(0);

  // He walks to the chest chamber: the corridor goes out of sight, and the goblin with it.
  await moveTo(mp, table, table.pensantusId, 10, 13);
  await expect.poll(async () => (await shading(ap)).remembered).toBeGreaterThan(0);
  await loaded(ap);
  expect((await tokens(ap)).map((n) => n.trim())).not.toContain('Goblin 2');
  expect((await tokens(ap)).map((n) => n.trim())).toContain('Pensantus (você)');
  // "Já visto" is in the legend, in words.
  await expect(ap.locator('.mr-legend').getByText('Já visto', { exact: true })).toBeVisible();

  // The master clears the memory: the remembered part goes back to black.
  const forgot = await callRPC(mp, 'meurpg.maps.v1.MapService/ForgetMapVision', { campaignId: table.campaignId, mapId: table.mapId });
  expect(forgot.ok()).toBeTruthy();
  await expect.poll(async () => (await shading(ap)).remembered).toBe(0);
  await closeAll(master, pensantus, toren);
});

test('quem tem o personagem fora do mapa lê o aviso e vê só o que já tinha visto @MR-036', async ({ browser }) => {
  const { table, master, pensantus, toren, bp } = await atTable(browser, { torenOffMap: true });
  await bp.goto(sessionRoute(table.campaignId));
  const notice = bp.getByTestId('fog-off-map');
  await expect(notice).toBeVisible();
  await expect(notice).toContainText('Toren fora do mapa');
  await expect(notice).toContainText('Seu personagem não está neste mapa.');
  await expect(notice).toHaveAttribute('role', 'status');
  // Nothing is seen now: no square is lit for him, and no enemy is on the map.
  expect((await tokens(bp)).map((n) => n.trim())).not.toContain('Goblin 2');
  expect((await shading(bp)).grey).toBeLessThan(3);
  await closeAll(master, pensantus, toren);
});

test('Toren acende a tocha e passa a ver mais @MR-036 @RN-10', async ({ browser }) => {
  const { table, master, pensantus, toren, bp } = await atTable(browser);
  await bp.goto(sessionRoute(table.campaignId));
  await loaded(bp);
  const before = await seen(bp);
  const row = bp.getByRole('button', { name: /Luz que você carrega/ });
  await expect(row).toContainText('Nenhuma');

  await row.click();
  const sheet = bp.getByRole('dialog', { name: 'Luz que você carrega' });
  await expect(sheet).toBeVisible();
  await expect(sheet.getByRole('radio')).toHaveCount(4);
  await sheet.getByText('Tocha', { exact: true }).click();
  await sheet.getByRole('button', { name: 'Pronto' }).click();
  await expect(sheet).toHaveCount(0);
  // The page says what it did, for six seconds, and the sheet's work is done: no "Salvar".
  await expect(bp.getByRole('status').filter({ hasText: 'Você acendeu a tocha: 6 m claro + 6 m de penumbra.' })).toBeVisible();

  await expect(row).toContainText('Tocha');
  await expect.poll(() => seen(bp)).toBeGreaterThan(before);
  await closeAll(master, pensantus, toren);
});

test('o mestre vê o mapa como o Toren vê, e volta com "Todos" @MR-036 @RN-10', async ({ browser }) => {
  const { table, master, pensantus, toren, mp, bp } = await atTable(browser);
  const asked: string[] = [];
  mp.on('request', (r) => asked.push(r.url()));
  await mp.goto(sessionRoute(table.campaignId));
  // His own map has no fog: the whole image, and nothing of the players' view.
  await expect(mp.locator('app-fog-map')).toHaveCount(0);
  const list = mp.getByRole('radiogroup', { name: 'Ver como' });
  await expect(list.getByRole('radio')).toHaveCount(3);
  await expect(list.getByRole('radio', { name: /Todos/ })).toHaveAttribute('aria-checked', 'true');
  // The counts come from the packed states.
  await expect(list.getByRole('radio', { name: /Pensantus/ })).toContainText(/\d+\s+quadrados vistos/);

  // Keyboard: the arrows move, Enter chooses.
  await list.getByRole('radio', { name: /Todos/ }).focus();
  await mp.keyboard.press('ArrowDown');
  await mp.keyboard.press('ArrowDown');
  await expect(list.getByRole('radio', { name: /Toren/ })).toBeFocused();
  await expect(list.getByRole('radio', { name: /Toren/ })).toHaveAttribute('aria-checked', 'false');
  await mp.keyboard.press('Enter');
  await expect(list.getByRole('radio', { name: /Toren/ })).toHaveAttribute('aria-checked', 'true');

  await expect(mp.getByText('Você está vendo o mapa como Toren.', { exact: false })).toBeVisible();
  await expect(mp.getByText('Vendo como Toren')).toBeVisible();
  await loaded(mp);
  // The same map Toren has in front of him, tile for tile.
  await bp.goto(sessionRoute(table.campaignId));
  await loaded(bp);
  expect(await shading(mp)).toEqual(await shading(bp));
  expect((await tokens(mp)).map((n) => n.replace(' (você)', '').trim()).sort()).toEqual((await tokens(bp)).map((n) => n.replace(' (você)', '').trim()).sort());
  expect(asked.some((u) => /\/tiles\/\d+\/\d+\?r=\d+&as=/.test(u))).toBe(true);

  await list.getByRole('radio', { name: /Todos/ }).click();
  await expect(mp.locator('app-fog-map')).toHaveCount(0);
  await closeAll(master, pensantus, toren);
});

test('Pensantus vê pelos olhos do Nanquim fora do combate e volta aos seus olhos @MR-036', async ({ browser }) => {
  const { table, master, pensantus, toren, ap } = await atTable(browser, { familiar: { col: 15, row: 8 } });
  await ap.goto(sessionRoute(table.campaignId));
  await loaded(ap);
  const own = await seen(ap);
  expect(await shading(ap)).toMatchObject({ remembered: 0 });

  const start = await callRPC(ap, 'meurpg.play.v1.PlayService/StartFamiliarSight', { campaignId: table.campaignId, characterId: table.pensantusId, idempotencyKey: crypto.randomUUID() });
  expect(start.ok(), await start.text()).toBeTruthy();
  const band = ap.getByTestId('familiar-band');
  await expect(band).toBeVisible();
  await expect(band).toContainText('Você está vendo pelos olhos do Nanquim.');
  await expect(band).toContainText('Pensantus está cego e surdo.');
  await expect(band).toHaveAttribute('role', 'status');
  // He sees what the raven sees now: the guard room, not his own corner.
  await expect.poll(() => seen(ap)).not.toBe(own);

  await ap.getByRole('button', { name: 'Voltar aos seus olhos' }).click();
  await expect(band).toHaveCount(0);
  await expect.poll(() => seen(ap)).toBe(own);
  await closeAll(master, pensantus, toren);
});

test('no combate, ver pelos olhos do Nanquim gasta a ação e deixa o personagem cego @MR-036 @RN-20', async ({ browser }) => {
  const { table, master, pensantus, toren, mp, ap, bp } = await atTable(browser, { familiar: { col: 15, row: 8 } });
  const combatTable = { campaignId: table.campaignId, captainId: table.captainId, goblinId: table.goblin1Id } as unknown as CombatTable;
  let enc = await startEncounterRPC(mp, combatTable, [
    { characterId: table.captainId, count: 1, hidden: false },
    { characterId: table.goblin1Id, count: 1, hidden: false },
    { characterId: table.goblin2Id, count: 1, hidden: false },
  ]);
  const faces: Record<string, number> = { Pensantus: 20, Toren: 15 };
  for (const c of enc.combatants) {
    enc = await combatRPC(mp, 'SubmitInitiative', { campaignId: table.campaignId, encounterId: enc.id, combatantId: c.id, d20Face: faces[c.label] ?? 2 });
  }
  const at: Record<string, [number, number]> = { 'Capitão Goblin': [21, 3], 'Goblin 1': [18, 5], 'Goblin 2': [20, 7] };
  for (const [label, [col, row]] of Object.entries(at)) {
    const id = enc.combatants.find((c) => c.label === label)!.id;
    enc = await combatRPC(mp, 'MoveCombatant', { campaignId: table.campaignId, encounterId: enc.id, combatantId: id, col, row });
  }
  await combatRPC(mp, 'BeginCombat', { campaignId: table.campaignId, encounterId: enc.id });

  await ap.goto(sessionRoute(table.campaignId));
  await expect(ap.getByRole('heading', { name: /Sua vez, Pensantus/ })).toBeVisible();
  // The combat map is the same fog: the tiles, the shading, the legend; no enemy that he does not see.
  await loaded(ap);
  expect((await shading(ap)).grey).toBeGreaterThan(0);
  await expect(ap.locator('.mr-legend').getByText('No escuro, em cinza', { exact: true })).toBeVisible();
  await expect(ap.getByTestId('familiar-blind')).toHaveCount(0);

  const start = await callRPC(ap, 'meurpg.play.v1.PlayService/StartFamiliarSight', { campaignId: table.campaignId, characterId: table.pensantusId, idempotencyKey: crypto.randomUUID() });
  expect(start.ok(), await start.text()).toBeTruthy();
  const band = ap.getByTestId('familiar-band');
  await expect(band).toContainText('Até o começo da sua próxima vez, a rodada 2.');
  await expect(ap.getByTestId('familiar-blind')).toHaveText(/Cego: para atacar, fale com o mestre\./);
  // The action was spent: the tile says so; and the others see only the conditions, in words.
  const now = await getEncounterRPC(mp, table.campaignId);
  const me = now.combatants.find((c) => c.label === 'Pensantus')!;
  expect(me.conditions ?? []).toEqual(expect.arrayContaining(['condition:blinded', 'condition:deafened']));
  await expect(bp.getByTestId('familiar-band')).toHaveCount(0);

  await ap.getByRole('button', { name: 'Voltar aos seus olhos' }).click();
  await expect(band).toHaveCount(0);
  await expect(ap.getByTestId('familiar-blind')).toHaveCount(0);
  await closeAll(master, pensantus, toren);
});

test('um mapa sem névoa continua como era: a prévia, com a imagem inteira @MR-036', async ({ browser }) => {
  const { table, master, pensantus, toren, ap } = await atTable(browser, { noFog: true });
  const fetched: string[] = [];
  ap.on('request', (r) => fetched.push(new URL(r.url()).pathname));
  await ap.goto(sessionRoute(table.campaignId));
  await expect(ap.getByRole('img', { name: /Prévia do mapa A caverna do Vale Seco/ })).toBeVisible();
  await expect(ap.locator('app-fog-map')).toHaveCount(0);
  await expect(ap.getByRole('link', { name: 'Ver mapa' })).toBeVisible();
  expect(fetched.some((p) => /^\/images\/(?!maps\/)/.test(p))).toBe(true);
  await closeAll(master, pensantus, toren);
});

export type { FogTable };
