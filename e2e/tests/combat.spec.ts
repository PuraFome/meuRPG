import { expect, test, type Browser, type BrowserContext, type Page } from '@playwright/test';

import { beginAttackCombatRPC, combatRPC, getEncounterRPC, setGridRPC, tableForCombat, type CombatTable, type Encounter } from './combat-support';
import { endOpenSessionRPC, openSessionPage } from './live-session-support';
import { newSignedInContext } from './support';

// The combat on screen (Etapa 6, slice 6.5a, MR-013, RN-18 to RN-22): the
// master sets the grid and starts a combat, everybody rolls initiative, the
// turns go round, a player moves inside the reach, and the master ends it.
// The table and the NPCs come through the API; what is under test is the
// screens and what each audience sees (RN-10, RN-20).

test(
  'o mestre define a grade e inicia o combate; a iniciativa, os turnos e o fim aparecem para cada um',
  { tag: ['@MR-013', '@RN-19', '@RN-20', '@RN-21'] },
  async ({ browser }) => {
    test.setTimeout(180_000);
    const master = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: 1280, height: 900 } });
    const player = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: 390, height: 844 } });
    const m = await master.newPage();
    const p = await player.newPage();
    let campaignId = '';
    try {
      await m.goto('/');
      await p.goto('/');
      const table = await tableForCombat(m, p, `Combate ${Date.now()}`, false);
      campaignId = table.campaignId;

      // A map without a grid: "Iniciar combate" sends the master to the grid page.
      await m.goto(`/campanhas/${campaignId}/sessao`);
      await m.getByRole('button', { name: 'Iniciar combate' }).click();
      await expect(m.getByText('Esse mapa ainda não tem grade.')).toBeVisible();
      await expect(m.getByRole('button', { name: 'Iniciar combate' }).last()).toHaveAttribute('aria-disabled', 'true');
      await m.getByRole('link', { name: 'Definir a grade' }).click();
      await expect(m.getByRole('heading', { name: 'Grade do mapa' })).toBeVisible();
      // Columns 4 is below the screen's 5 to 60; 30 gives 21 rows for 2000 x 1400.
      await m.getByLabel('Quadrados de 1,5 m na largura').fill('4');
      await expect(m.getByText('Use um número inteiro de 5 a 60.')).toBeVisible();
      await expect(m.getByRole('button', { name: 'Salvar grade' })).toHaveAttribute('aria-disabled', 'true');
      await m.getByLabel('Quadrados de 1,5 m na largura').fill('20');
      await expect(m.getByText('20 × 14')).toBeVisible();
      await expect(m.getByText('30 m × 21 m')).toBeVisible();
      await m.getByRole('button', { name: 'Salvar grade' }).click();
      await expect(m).toHaveURL(/\/sessao$/);

      // Start with the party and three hidden goblins (and the captain).
      await m.getByRole('button', { name: 'Iniciar combate' }).click();
      await expect(m.getByRole('dialog', { name: 'Iniciar combate' })).toBeVisible();
      await expect(m.getByText('20 × 14 quadrados de 1,5 m')).toBeVisible();
      await expect(m.getByText('30 m × 21 m').first()).toBeVisible();
      for (let i = 0; i < 3; i++) {
        await m.getByRole('button', { name: 'Mais um Goblin', exact: true }).click();
      }
      await m.getByRole('button', { name: 'Mais um Capitão Goblin' }).click();
      await expect(m.getByText('Minion · vira Goblin 1, 2 e 3')).toBeVisible();
      await expect(m.getByText('Escondido no início').first()).toBeVisible();
      await m.getByRole('dialog').getByRole('button', { name: 'Iniciar combate' }).click();
      await expect(m.getByRole('heading', { level: 1, name: 'Sessão 1' })).toBeVisible();
      await expect(m.getByText('Os NPCs rolaram sozinhos.')).toBeVisible();
      await expect(m.getByRole('button', { name: 'Começar o combate' })).toHaveAttribute('aria-disabled', 'true');
      await expect(m.getByText('Falta a iniciativa de Pensantus.')).toBeVisible();

      // The player rolls in the app; the NPCs never show on their screen.
      await openSessionPage(p, campaignId);
      await expect(p.getByRole('heading', { name: 'Role a iniciativa' })).toBeVisible();
      await expect(p.getByText('Goblin')).toHaveCount(0);
      await p.getByRole('button', { name: 'Rolar no app' }).click();
      await expect(p.getByText('Esperando o mestre começar o combate')).toBeVisible();

      // Force a tie between two goblins, put the captain first, and break the tie on the screen.
      let enc = await getEncounterRPC(m, campaignId);
      const id = (label: string) => enc.combatants.find((c) => c.label === label)!.id;
      const face = async (label: string, d20Face: number) => {
        enc = await combatRPC(m, 'SubmitInitiative', { campaignId, encounterId: enc.id, combatantId: id(label), d20Face });
      };
      await face('Capitão Goblin', 20);
      // The player's roll stays what the app rolled, except that the master
      // may correct it: here, to make the order the same on every run.
      await face('Pensantus', 19);
      await face('Goblin 1', 7);
      await face('Goblin 2', 7);
      await face('Goblin 3', 3);
      const pen = enc.combatants.find((c) => c.label === 'Pensantus')!;
      const cap = enc.combatants.find((c) => c.label === 'Capitão Goblin')!;
      if (pen.initiative === cap.initiative) {
        await combatRPC(m, 'SetInitiativeOrder', { campaignId, encounterId: enc.id, combatantIds: [cap.id, pen.id] });
      }
      await expect(m.getByText(/Empate em 7: Goblin \d e Goblin \d\. Escolha a ordem com as setas\./)).toBeVisible();
      // The first of the group can't go up: the arrow is a dashed outline that says so and does nothing.
      const firstUp = m.locator('[aria-label^="Subir Goblin"][aria-disabled="true"]');
      await expect(firstUp).toHaveCount(1);
      const first = ((await firstUp.getAttribute('aria-label')) ?? '').replace(/^Subir | na ordem$/g, '');
      await m.getByRole('button', { name: `Descer ${first} na ordem` }).click();
      await expect(m.getByText('Empate em 7: Goblin')).toHaveCount(0);
      await expect(m.getByText('Todos rolaram.')).toBeVisible();

      // Put the goblins on squares, then begin.
      for (const [label, col, row] of [['Goblin 1', 9, 9], ['Goblin 2', 14, 10], ['Goblin 3', 15, 4], ['Capitão Goblin', 11, 5]] as const) {
        enc = await combatRPC(m, 'MoveCombatant', { campaignId, encounterId: enc.id, combatantId: id(label), col, row });
      }
      await m.getByRole('button', { name: 'Começar o combate' }).click();
      await expect(m.getByRole('button', { name: 'Próximo turno' })).toBeVisible();
      await expect(m.getByText('Vez do Capitão Goblin')).toBeVisible();
      await expect(m.getByRole('region', { name: 'Combate', exact: true }).getByText('Rodada 1', { exact: true })).toBeVisible();

      // The captain is hidden: the player sees "Vez do mestre", never a goblin.
      await expect(p.getByRole('heading', { name: 'Vez do mestre' })).toBeVisible();
      await expect(p.getByText('Goblin')).toHaveCount(0);
      await expect(p.getByRole('listitem').filter({ hasText: 'Pensantus' }).first()).toBeVisible();

      // "Próximo turno": Pensantus, and the player is told.
      await m.getByRole('button', { name: 'Próximo turno' }).click();
      await expect(m.getByText('Vez do Pensantus')).toBeVisible();
      await expect(p.getByRole('heading', { name: 'Sua vez, Pensantus' })).toBeVisible();

      // The player moves 3 m (2 right, 1 down) inside the reach, and is refused beyond it.
      await p.getByRole('button', { name: 'Mover' }).click();
      await expect(p.getByRole('heading', { name: 'Mover Pensantus' })).toBeVisible();
      const map = p.getByRole('group', { name: /Mapa de batalha/ });
      const box = (await map.boundingBox())!;
      const w = box.width / 20;
      const h = box.height / 14;
      const start = enc.combatants.find((c) => c.label === 'Pensantus')!;
      const own = (await getEncounterRPC(p, campaignId)).combatants.find((c) => c.mine)!;
      const at = (dc: number, dr: number) => ({ x: ((own.col ?? 0) + dc + 0.5) * w, y: ((own.row ?? 0) + dr + 0.5) * h });
      expect(start).toBeTruthy();
      await map.click({ position: at(8, 1) });
      await expect(p.getByText('Longe demais: faltam')).toBeVisible();
      await expect(p.getByRole('button', { name: 'Mover para cá' })).toHaveAttribute('aria-disabled', 'true');
      await map.click({ position: at(2, 1) });
      await expect(p.getByRole('status').filter({ hasText: 'Mover 3 m' })).toContainText('2 quadrados para a direita e 1 quadrado para baixo. Depois restam 4,5 m.');
      await p.getByRole('button', { name: 'Mover para cá' }).click();
      await expect(p.getByRole('heading', { name: 'Sua vez, Pensantus' })).toBeVisible();
      await expect.poll(async () => (await getEncounterRPC(m, campaignId)).combatants.find((c) => c.label === 'Pensantus')?.col).toBe((own.col ?? 0) + 2);

      // The master asks before ending, and the summary shows what happened.
      await m.getByRole('button', { name: 'Encerrar combate' }).click();
      await expect(m.getByRole('alertdialog', { name: 'Encerrar o combate?' })).toBeVisible();
      await expect(m.getByRole('button', { name: 'Cancelar' })).toBeFocused();
      await m.getByRole('button', { name: 'Encerrar combate' }).last().click();
      await expect(m.getByRole('heading', { name: 'Combate encerrado' })).toBeVisible();
      await expect(p.getByRole('heading', { name: 'Combate encerrado' })).toBeVisible();
      await m.getByRole('button', { name: 'Voltar à sessão' }).click();
      await expect(m.getByRole('button', { name: 'Iniciar combate' })).toBeVisible();
    } finally {
      if (campaignId) {
        await endOpenSessionRPC(m, campaignId);
      }
      await master.close();
      await player.close();
    }
  },
);

test('um jogador que digita os dados rola a iniciativa com o número do dado físico', { tag: ['@MR-013', '@RN-18'] }, async ({ browser }) => {
  test.setTimeout(120_000);
  const master = await newSignedInContext(browser, 'Mestre Teste');
  const player = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: 390, height: 844 } });
  const m = await master.newPage();
  const p = await player.newPage();
  let campaignId = '';
  try {
    await m.goto('/');
    await p.goto('/');
    const table = await tableForCombat(m, p, `Dados físicos ${Date.now()}`);
    campaignId = table.campaignId;
    const pref = await p.request.post('/meurpg.campaigns.v1.CampaignService/SetMyDicePreference', {
      data: { campaignId, preference: 'DICE_PREFERENCE_PHYSICAL' },
      headers: { 'Connect-Protocol-Version': '1' },
    });
    expect(pref.ok()).toBeTruthy();
    await combatRPC(m, 'StartEncounter', {
      campaignId,
      name: 'Dados',
      participants: [{ characterId: table.goblinId, count: 1 }],
    });
    await openSessionPage(p, campaignId);
    // While the campaign lets each player choose, both ways are offered (RN-18); the saved choice only
    // decides which one is the filled button.
    await expect(p.getByRole('button', { name: 'Rolar no app' })).toBeVisible();
    await p.getByRole('button', { name: 'Digitar o resultado' }).click();
    await p.getByLabel(/Role 1d20 para a iniciativa/).fill('99');
    await expect(p.getByText('Digite um número de 1 a 20')).toBeVisible();
    await p.getByLabel(/Role 1d20 para a iniciativa/).fill('12');
    await expect(p.getByText('12 + 3 = 15 · dado físico')).toBeVisible();
    await p.getByRole('button', { name: 'Confirmar 12' }).click();
    await expect(p.getByText('Esperando o mestre começar o combate')).toBeVisible();
    await expect(p.getByText('1d20 (12) + 3 = 15')).toBeVisible();
    const enc = await getEncounterRPC(m, campaignId);
    expect(enc.combatants.find((c: Encounter['combatants'][number]) => c.label === 'Pensantus')?.initiative).toBe(15);
    await setGridRPC(m, campaignId, table.mapId, 20);
  } finally {
    if (campaignId) {
      await endOpenSessionRPC(m, campaignId);
    }
    await master.close();
    await player.close();
  }
});

// Acting in a combat (Etapa 6, slice 6.5b, MR-012, MR-014, RN-18 to RN-20): the
// player's "Sua vez" groups and attack sheet, the master's card with the armor
// class and the damage to apply, undo, "Dano/Cura" and the log. Pensantus
// carries a dagger and Raio de Fogo; the Capitão a scimitar and chain mail. The
// d20 is typed where the test needs a sure hit; the app's own roll is checked
// for what it shows either way.

interface ActingTable {
  m: Page;
  p: Page;
  table: CombatTable;
  campaignId: string;
  done: () => Promise<void>;
}

/** A combat with Pensantus, the Capitão and two Goblins, begun with the given d20 faces. */
async function actingTable(browser: Browser, name: string, faces: Record<string, number>, hidden: string[] = []): Promise<ActingTable> {
  const master: BrowserContext = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: 1280, height: 900 } });
  const player: BrowserContext = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: 390, height: 844 } });
  const m = await master.newPage();
  const p = await player.newPage();
  await m.goto('/');
  await p.goto('/');
  const table = await tableForCombat(m, p, `${name} ${Date.now()}`, true, true);
  await beginAttackCombatRPC(m, table, faces, undefined, hidden);
  return {
    m,
    p,
    table,
    campaignId: table.campaignId,
    done: async () => {
      await endOpenSessionRPC(m, table.campaignId);
      await master.close();
      await player.close();
    },
  };
}

const playerFirst = { Pensantus: 20, 'Capitão Goblin': 15, 'Goblin 1': 5, 'Goblin 2': 4 };
const captainFirst = { Pensantus: 15, 'Capitão Goblin': 20, 'Goblin 1': 5, 'Goblin 2': 4 };

test('o jogador ataca com dados físicos: digita o d20 e a soma do dano, e o goblin é derrotado', { tag: ['@MR-012', '@MR-014', '@RN-18', '@RN-20'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  const { m, p, campaignId, done } = await actingTable(browser, 'Ataque físico', playerFirst);
  try {
    await openSessionPage(p, campaignId);
    await expect(p.getByRole('heading', { name: 'Sua vez, Pensantus' })).toBeVisible();
    // The groups: the cantrip with its pill, the spells, the standard actions (not Atacar or Conjurar), the movement.
    const groups = p.getByRole('region', { name: 'O que você pode fazer' });
    await expect(groups.getByRole('heading', { name: 'Ação', exact: true })).toBeVisible();
    await expect(groups.getByText('+6 para acertar · 1d10 de fogo · alcance 36 m')).toBeVisible();
    await expect(groups.getByRole('button', { name: 'Disparada' })).toBeVisible();
    await expect(groups.getByRole('button', { name: 'Conjurar uma magia' })).toHaveCount(0);
    await expect(groups.getByRole('heading', { name: 'Movimento' })).toBeVisible();
    // No armor class on the player's screen.
    await expect(p.getByText(/contra CA/)).toHaveCount(0);

    await p.getByRole('button', { name: 'Atacar com Raio de Fogo' }).click();
    const sheet = p.getByRole('dialog', { name: 'Atacar com Raio de Fogo' });
    await expect(sheet).toBeVisible();
    await expect(sheet.getByRole('heading', { name: 'Atacar com Raio de Fogo' })).toBeFocused();
    await expect(sheet.getByText('Ferido')).toHaveCount(0);
    await sheet.locator('label', { hasText: 'Goblin 1' }).click();
    await sheet.getByRole('button', { name: 'Digitar o resultado' }).click();
    await expect(sheet.getByRole('heading', { name: 'Digite o resultado do dado' })).toBeVisible();
    await sheet.getByLabel(/Role 1d20 para Raio de Fogo/).fill('27');
    await expect(sheet.getByText('Digite um número de 1 a 20')).toBeVisible();
    await expect(sheet.getByRole('button', { name: 'Confirmar' })).toHaveAttribute('aria-disabled', 'true');
    await sheet.getByLabel(/Role 1d20 para Raio de Fogo/).fill('20');
    await expect(sheet.getByText('20 + 6 = 26 · dado físico')).toBeVisible();
    await sheet.getByRole('button', { name: 'Confirmar 20' }).click();
    // A natural 20 is a critical hit: the dice double.
    await expect(sheet.locator('.pill', { hasText: 'Crítico' })).toBeVisible();
    await expect(sheet.getByText('Acerto crítico: os dados do dano dobram (2d10).')).toBeVisible();
    await sheet.getByRole('button', { name: 'Digitar o resultado' }).click();
    await sheet.getByLabel(/Role 2d10/).fill('21');
    await expect(sheet.getByText('Digite um número de 2 a 20')).toBeVisible();
    await sheet.getByLabel(/Role 2d10/).fill('12');
    await sheet.getByRole('button', { name: 'Confirmar 12' }).click();
    await expect(sheet.getByText('Goblin 1 derrotado')).toBeVisible();
    await expect(sheet.getByText('12 de fogo')).toBeVisible();
    await expect(sheet.getByText('Sua ação foi usada. Truque: nenhum espaço de magia foi gasto.')).toBeVisible();
    await expect(sheet.getByRole('button', { name: 'Voltar à sua vez' })).toBeFocused();
    await sheet.getByRole('button', { name: 'Voltar à sua vez' }).click();

    // The goblin is defeated for everyone, the action is used, and "Encerrar turno" does not ask.
    const enc = await getEncounterRPC(m, campaignId);
    expect(enc.combatants.find((c) => c.label === 'Goblin 1')?.defeated).toBe(true);
    await expect(p.getByText('Ação já usada').first()).toBeVisible();
    // The log: his sentence, with the damage and the defeat.
    await p.getByRole('button', { name: 'Abrir o registro do combate' }).click();
    await expect(p.getByRole('log', { name: 'Registro do combate' })).toContainText('Pensantus atira no Goblin 1 com o Raio de Fogo: crítico, 12 de dano. Goblin 1 derrotado');
  } finally {
    await done();
  }
});

test('o jogador rola o ataque no app: o resultado mostra a conta e nunca a CA', { tag: ['@MR-012', '@MR-014', '@RN-20'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  const { p, campaignId, done } = await actingTable(browser, 'Ataque no app', playerFirst);
  try {
    await openSessionPage(p, campaignId);
    await p.getByRole('button', { name: 'Atacar com Raio de Fogo' }).click();
    const sheet = p.getByRole('dialog', { name: 'Atacar com Raio de Fogo' });
    await expect(sheet.getByRole('radio', { name: /Capitão Goblin/ })).toBeEnabled();
    await sheet.locator('label', { hasText: 'Capitão Goblin' }).click();
    await sheet.getByRole('button', { name: 'Rolar no app' }).click();
    await expect(sheet.getByText(/1d20 \(\d+\) \+ 6 = \d+/)).toBeVisible();
    await expect(sheet.getByText(/Acertou|Crítico|Errou/).first()).toBeVisible();
    if (await sheet.getByRole('button', { name: 'Rolar 1d10 no app' }).or(sheet.getByRole('button', { name: 'Rolar 2d10 no app' })).isVisible()) {
      await sheet.getByRole('button', { name: /Rolar \dd10 no app/ }).click();
      await expect(sheet.getByText(/\d+d10 \([\d, ]+\) = \d+ de fogo/)).toBeVisible();
    } else {
      await expect(sheet.getByText('Sem dano: o ataque errou.')).toBeVisible();
    }
    await expect(sheet.getByText(/CA/)).toHaveCount(0);
    await sheet.getByRole('button', { name: 'Voltar à sua vez' }).click();
  } finally {
    await done();
  }
});

test('o jogador encerra o turno: pergunta com a ação livre, e a Disparada dobra o movimento', { tag: ['@MR-014', '@RN-21'] }, async ({ browser }) => {
  test.setTimeout(120_000);
  const { m, p, campaignId, done } = await actingTable(browser, 'Turno do jogador', playerFirst);
  try {
    await openSessionPage(p, campaignId);
    await expect(p.getByRole('heading', { name: 'Sua vez, Pensantus' })).toBeVisible();
    // "Encerrar turno" is an outline while the action is free, and asks before ending.
    await p.getByRole('button', { name: 'Encerrar turno' }).last().click();
    await expect(p.getByRole('alertdialog', { name: /Ainda tem ação disponível/ })).toBeVisible();
    await expect(p.getByRole('button', { name: 'Voltar' })).toBeFocused();
    await p.getByRole('button', { name: 'Voltar' }).click();
    expect((await getEncounterRPC(m, campaignId)).currentCombatantId).toBeTruthy();
    const before = (await getEncounterRPC(m, campaignId)).combatants.find((c) => c.mine === undefined && c.label === 'Pensantus');
    expect(before?.movementLeftFt).toBe(25);

    // The Disparada spends the action and doubles the movement left.
    await p.getByRole('button', { name: 'Disparada' }).click();
    await expect(p.getByText('Ação já usada').first()).toBeVisible();
    await expect(p.getByText('Restam 15 m')).toBeVisible();
    const after = (await getEncounterRPC(m, campaignId)).combatants.find((c) => c.label === 'Pensantus');
    expect(after?.movementLeftFt).toBe(50);
    await expect(p.getByRole('log', { name: 'Registro do combate' })).toHaveCount(0);

    // The action is used and the bonus action still free: still no filled button, but no question either.
    await p.getByRole('button', { name: 'Encerrar turno' }).last().click();
    await expect(p.getByRole('heading', { name: 'Vez do Capitão Goblin' })).toBeVisible();
  } finally {
    await done();
  }
});

test('o goblin do mestre ataca: ele vê a CA, aplica o dano (os PV do jogador mudam), descarta outro e desfaz', { tag: ['@MR-012', '@MR-014', '@RN-02', '@RN-20'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  const { m, p, campaignId, done } = await actingTable(browser, 'Ataque do mestre', captainFirst);
  try {
    await openSessionPage(m, campaignId);
    await openSessionPage(p, campaignId);
    await expect(m.getByRole('heading', { name: 'Ações do Capitão Goblin' })).toBeVisible();
    const card = m.getByRole('region', { name: 'Ações do Capitão Goblin' });
    await expect(card.getByText('Pontos de vida')).toBeVisible();
    await expect(card.getByText('CA', { exact: true })).toBeVisible();
    await expect(m.getByText('CA 13').first()).toBeVisible(); // Pensantus, in the order
    await expect(p.getByText(/CA \d+/)).toHaveCount(0); // never on the player's screen

    // The first attack: the master types a 20 (a sure hit) and sees the armor class it met.
    await card.getByRole('button', { name: 'Digitar o resultado' }).click();
    await card.getByLabel(/Role 1d20 para Cimitarra/).fill('20');
    await card.getByRole('button', { name: 'Confirmar 20' }).click();
    await expect(card.getByText(/contra CA 13 d[ao] Pensantus/)).toBeVisible();
    await card.getByRole('button', { name: 'Rolar dano' }).click();
    await expect(card.getByRole('button', { name: /Aplicar \d+ de dano/ })).toBeVisible();
    await expect(card.getByText(/Pensantus: 23 de 23 PV, depois \d+/)).toBeVisible();
    await expect(m.getByText(/Falta aplicar \d+ de dano/).first()).toBeVisible();

    // Passing the turn with a damage waiting asks first; "Voltar" keeps the turn.
    await m.getByRole('button', { name: 'Próximo turno' }).click();
    await expect(m.getByRole('alertdialog', { name: /Há dano sem aplicar/ })).toBeVisible();
    await expect(m.getByRole('button', { name: 'Voltar' })).toBeFocused();
    await m.getByRole('button', { name: 'Voltar' }).click();
    await expect(m.getByRole('heading', { name: 'Ações do Capitão Goblin' })).toBeVisible();

    // Applying changes the player's hit points, and the log says so.
    await card.getByRole('button', { name: /Aplicar \d+ de dano/ }).click();
    await expect.poll(async () => (await getEncounterRPC(m, campaignId)).combatants.find((c) => c.label === 'Pensantus')?.hitPointsCurrent).toBeLessThan(23);
    await expect(m.getByRole('log', { name: 'Registro do combate' })).toContainText('Capitão Goblin ataca');

    // A second attack of the same turn (the master may attack again): the damage is discarded, after a question.
    await card.getByRole('button', { name: 'Digitar o resultado' }).click();
    await card.getByLabel(/Role 1d20 para Cimitarra/).fill('20');
    await card.getByRole('button', { name: 'Confirmar 20' }).click();
    await card.getByRole('button', { name: 'Rolar dano' }).click();
    await card.getByRole('button', { name: 'Não aplicar' }).click();
    await expect(card.getByText(/Descartar o dano de \d+\?/)).toBeVisible();
    await expect(card.getByRole('button', { name: 'Voltar' })).toBeFocused();
    await card.getByRole('button', { name: 'Descartar' }).click();
    const hp = (await getEncounterRPC(m, campaignId)).combatants.find((c) => c.label === 'Pensantus')?.hitPointsCurrent;
    expect(hp).toBeLessThan(23);

    // Undo takes back the last action only, named in the question; the damage discarded comes back to be decided.
    await m.getByRole('button', { name: 'Desfazer última ação' }).first().click();
    await expect(m.getByText(/Desfazer o ataque do Capitão Goblin ao Pensantus/).or(m.getByText(/Desfazer o ataque do Capitão Goblin à Pensantus/))).toBeVisible();
    await expect(m.getByRole('button', { name: 'Voltar' }).last()).toBeFocused();
    await m.getByRole('button', { name: 'Desfazer', exact: true }).click();
    await expect(m.getByRole('alertdialog', { name: 'Desfazer a última ação' })).toHaveCount(0);
  } finally {
    await done();
  }
});

test('o mestre ajusta os PV de um NPC em "Dano/Cura", e o jogador não vê um goblin escondido no registro', { tag: ['@MR-012', '@RN-02', '@RN-20'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  const { m, p, campaignId, table, done } = await actingTable(browser, 'Dano e cura', { Pensantus: 15, 'Capitão Goblin': 12, 'Goblin 1': 20, 'Goblin 2': 4 }, ['Goblin 1']);
  try {
    await openSessionPage(m, campaignId);
    await openSessionPage(p, campaignId);
    const enc = await getEncounterRPC(m, campaignId);
    const goblin2 = enc.combatants.find((c) => c.label === 'Goblin 2')!;

    // The hidden Goblin 1 is on turn: the player sees "Vez do mestre", and its attack never reaches their log.
    await expect(p.getByRole('heading', { name: 'Vez do mestre' })).toBeVisible();
    await combatRPC(m, 'RollAttack', {
      campaignId,
      encounterId: enc.id,
      attackerId: enc.combatants.find((c) => c.label === 'Goblin 1')!.id,
      attackKey: 'basic:0',
      targetId: enc.combatants.find((c) => c.label === 'Pensantus')!.id,
      d20Face: 3,
    });
    await expect(m.getByRole('log', { name: 'Registro do combate' })).toContainText('Goblin 1');
    await expect(m.getByRole('log', { name: 'Registro do combate' })).toContainText('Só o mestre vê');
    await p.getByRole('button', { name: 'Abrir o registro do combate' }).click();
    await expect(p.getByRole('log', { name: 'Registro do combate' })).toBeVisible();
    await expect(p.getByRole('log', { name: 'Registro do combate' })).not.toContainText('Goblin 1');
    await expect(p.getByRole('log', { name: 'Registro do combate' })).not.toContainText('Só o mestre vê');
    await p.keyboard.press('Escape');

    // "Dano/Cura" on Goblin 2: 3 of damage, then a heal back.
    await m.getByRole('button', { name: 'Dano ou cura em Goblin 2' }).click();
    const dialog = m.getByRole('dialog', { name: 'Dano ou cura em Goblin 2' });
    await expect(dialog.getByRole('heading', { name: 'Dano ou cura em Goblin 2' })).toBeFocused();
    await expect(dialog.getByText('Agora: 7 de 7 PV')).toBeVisible();
    await dialog.getByLabel('Dano sofrido').fill('3');
    await expect(dialog.getByText('Depois: 4 de 7 PV')).toBeVisible();
    await dialog.getByRole('button', { name: 'Salvar ajuste' }).click();
    await expect(dialog).toHaveCount(0);
    await expect.poll(async () => (await getEncounterRPC(m, campaignId)).combatants.find((c) => c.id === goblin2.id)?.hitPointsCurrent).toBe(4);
    await expect(m.getByRole('log', { name: 'Registro do combate' })).toContainText('Goblin 2 perdeu 3 PV, por ajuste do mestre, agora com 4 PV');
    // The player sees only the state word of the NPC, never the numbers.
    await expect(p.getByText('4 de 7')).toHaveCount(0);

    await m.getByRole('button', { name: 'Dano ou cura em Goblin 2' }).click();
    await m.getByRole('dialog').locator('label', { hasText: /^\s*Cura\s*$/ }).click();
    await m.getByLabel('PV curados').fill('9');
    await expect(m.getByText('Depois: 7 de 7 PV')).toBeVisible();
    await m.getByRole('button', { name: 'Salvar ajuste' }).click();
    await expect.poll(async () => (await getEncounterRPC(m, campaignId)).combatants.find((c) => c.id === goblin2.id)?.hitPointsCurrent).toBe(7);
    expect(table.campaignId).toBe(campaignId);
  } finally {
    await done();
  }
});
