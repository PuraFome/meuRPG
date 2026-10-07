import { expect, test, type Page } from '@playwright/test';

import { endOpenSessionRPC } from './live-session-support';
import { newSignedInContext } from './support';
import { markMilestoneRPC, tableForLevelUp, toren } from './levelup-support';

// MR-040 (the guided level-up: Habilidades, Vida, Magias, Resumo; the master sees "O que mudou"), RN-01 (the
// sheet stays locked: only the level's choices change), RN-12 (who can level up). The data comes through
// the API; the screens are what is under test. Every test makes its own campaign.

const sheetOf = (campaignId: string, characterId: string) => `/campaigns/${campaignId}/characters/${characterId}`;

/** Clicks the row of a pick list by the name on it. */
const row = (page: Page, name: string) => page.locator('.row__main, .row').filter({ hasText: name }).first();

test(
  'Pensantus sobe do Mago 3 para o 4: habilidade, vida média, um truque, duas magias e duas para preparar; o mestre vê "O que mudou"',
  { tag: ['@MR-040', '@RN-01', '@RN-12'] },
  async ({ browser }) => {
    test.setTimeout(150_000);
    const master = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: 1280, height: 800 } });
    const player = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: 1280, height: 800 } });
    const m = await master.newPage();
    const p = await player.newPage();
    let campaignId = '';
    try {
      await m.goto('/');
      await p.goto('/');
      const table = await tableForLevelUp(m, p, `Subida de Pensantus ${Date.now()}`);
      campaignId = table.campaignId;

      // The locked sheet says why, and has the one filled button.
      await p.goto(sheetOf(campaignId, table.characterId));
      await expect(p.getByRole('heading', { name: 'Pensantus pode subir de nível' })).toBeVisible();
      await expect(p.getByText('O mestre marcou “Chegar ao Vale Seco”.')).toBeVisible();
      await p.getByRole('link', { name: 'Subir para o nível 4' }).click();

      // Habilidades: +2 in Inteligência, and what it changes, from the server.
      await expect(p.getByRole('heading', { name: 'Subir para o nível 4' })).toBeVisible();
      await expect(p.getByText('Passo 1 de 4 · Habilidades')).toBeVisible();
      await expect(p.getByText('Falta escolher 1 habilidade.')).toBeVisible();
      await row(p, 'Inteligência').click();
      await expect(p.getByText('18 → 20')).toBeVisible();
      await expect(p.getByRole('heading', { name: 'O que muda com Inteligência 20' })).toBeVisible();
      await p.getByRole('button', { name: 'Próximo' }).click();

      // Vida: the average is already chosen.
      await expect(p.getByText('Passo 2 de 4 · Vida')).toBeVisible();
      await expect(p.getByText('Média: 4')).toBeVisible();
      await expect(p.getByText('4 + Constituição +3 · de 23 para 30')).toBeVisible();
      await p.getByRole('button', { name: 'Próximo' }).click();

      // Magias: a cantrip, two spells for the book, two to prepare. A missing choice blocks "Próximo".
      await expect(p.getByText('Passo 3 de 4 · Magias')).toBeVisible();
      await expect(p.getByText('Falta escolher 1 truque.')).toBeVisible();
      // "Próximo" is aria-disabled but still answers a tap: the focus goes to the first missing choice.
      await p.getByRole('button', { name: 'Próximo' }).click({ force: true });
      await expect(p.locator('#pick-cantrips').getByRole('radio').first()).toBeFocused();
      await expect(p.getByText('Passo 3 de 4 · Magias')).toBeVisible();
      await p.getByRole('button', { name: /Ver os outros \d+ truques/ }).click();
      await row(p, 'Prestidigitação').click();
      await p.getByLabel('Buscar magia').fill('nebuloso');
      await row(p, 'Passo Nebuloso').first().click();
      await p.getByLabel('Buscar magia').fill('reflexos');
      await row(p, 'Reflexos').first().click();
      await p.getByLabel('Buscar magia').fill('');
      await expect(p.getByText('Faltam preparar 2 magias.')).toBeVisible();
      const prepare = p.locator('#pick-prepared');
      await prepare.locator('.row__main').filter({ hasText: 'Passo Nebuloso' }).click();
      await prepare.locator('.row__main').filter({ hasText: 'Detectar Magia' }).click();
      await expect(prepare).toContainText('9 de 9');
      await p.getByRole('button', { name: 'Próximo' }).click();

      // Resumo: everything before → after, and the rest stays locked.
      await expect(p.getByText('Passo 4 de 4 · Resumo')).toBeVisible();
      const summary = p.getByRole('region', { name: 'O que muda', exact: true });
      await expect(summary).toContainText('Mago 3');
      await expect(summary).toContainText('Mago 4');
      await expect(summary).toContainText('23');
      await expect(summary).toContainText('30');
      await expect(summary).toContainText('CD das magias');
      await expect(p.getByText('O resto da ficha não muda e continua travado.')).toBeVisible();
      await p.getByRole('button', { name: 'Confirmar o nível 4' }).click();

      // The sheet: level 4, the new numbers, a status the player dismisses, and no tag any more.
      await expect(p).toHaveURL(new RegExp(`/characters/${table.characterId}$`));
      await expect(p.getByRole('status').filter({ hasText: 'Pensantus subiu para o nível 4. O mestre foi avisado.' })).toBeVisible();
      await expect(p.getByRole('definition').filter({ hasText: 'Mago 4' })).toBeVisible();
      await expect(p.locator('app-ability-medallions')).toContainText('20');
      await expect(p.getByText('Pode subir de nível')).toHaveCount(0);
      await expect(p.getByRole('link', { name: 'Subir para o nível 4' })).toHaveCount(0);
      await expect(p.getByLabel('Estado do personagem')).toContainText('Travada');
      await p.getByRole('button', { name: 'Dispensar o aviso' }).click();
      await expect(p.getByText('Pensantus subiu para o nível 4.')).toHaveCount(0);

      // The master is told, and sees what was chosen.
      await m.goto(`/campaigns/${campaignId}`);
      await expect(m.getByRole('status').filter({ hasText: 'Pensantus subiu para o nível 4.' })).toBeVisible();
      await m.getByRole('button', { name: 'O que mudou: Pensantus' }).click();
      const changes = m.getByRole('region', { name: 'O que Pensantus escolheu no nível 4' });
      await expect(changes).toContainText('+2 em Inteligência');
      await expect(changes).toContainText('Prestidigitação');
      await expect(changes).toContainText('Passo Nebuloso e Reflexos');
      await expect(changes).toContainText('Confirmado em');
      await expect(changes).toContainText('Nada fica à espera do seu OK');
      await expect(changes.getByRole('link', { name: 'Abrir a ficha' })).toBeVisible();
    } finally {
      await endOpenSessionRPC(m, campaignId);
      await master.close();
      await player.close();
    }
  },
);

test(
  'Toren sobe do Guerreiro 4 para o 5: só Vida e Resumo, e rolar o dado no app guarda o resultado',
  { tag: ['@MR-040', '@RN-12', '@RN-18'] },
  async ({ browser }) => {
    test.setTimeout(120_000);
    const master = await newSignedInContext(browser, 'Mestre Teste');
    const player = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: 390, height: 844 } });
    const m = await master.newPage();
    const p = await player.newPage();
    let campaignId = '';
    try {
      await m.goto('/');
      await p.goto('/');
      const table = await tableForLevelUp(m, p, `Subida de Toren ${Date.now()}`, { build: toren });
      campaignId = table.campaignId;
      await p.goto(sheetOf(campaignId, table.characterId));
      await p.getByRole('link', { name: 'Subir para o nível 5' }).click();

      await expect(p.getByText('Passo 1 de 2 · Vida')).toBeVisible();
      await expect(p.getByText('Neste nível não há mais nada para escolher')).toBeVisible();
      await expect(p.getByText('Ataque Extra')).toBeVisible();
      await expect(p.getByText('Média: 6')).toBeVisible();

      await p.locator('.dice-choice__card').filter({ hasText: 'Rolar 1d10' }).click();
      await expect(p.getByText('Falta rolar o dado de vida.')).toBeVisible();
      await p.getByRole('button', { name: 'Rolar no app' }).click();
      await expect(p.getByText(/Rolado no app: \d+ no d10/)).toBeVisible();
      await p.getByRole('button', { name: 'Próximo' }).click();

      await expect(p.getByText('Passo 2 de 2 · Resumo')).toBeVisible();
      await expect(p.getByRole('region', { name: 'O que muda', exact: true })).toContainText('Dados de vida');
      await p.getByRole('button', { name: 'Confirmar o nível 5' }).click();
      await expect(p.getByRole('status').filter({ hasText: 'Toren subiu para o nível 5.' })).toBeVisible();
      await expect(p.getByRole('definition').filter({ hasText: 'Guerreiro 5' })).toBeVisible();

      // The master's list keeps the roll on record.
      await m.goto(`/campaigns/${campaignId}`);
      await m.getByRole('button', { name: 'O que mudou: Toren' }).click();
      await expect(m.getByRole('region', { name: 'O que Toren escolheu no nível 5' })).toContainText('Rolado no app');
    } finally {
      await endOpenSessionRPC(m, campaignId);
      await master.close();
      await player.close();
    }
  },
);

test(
  'quem não pode subir de nível não tem o botão, e a rota responde como a de uma ficha travada',
  { tag: ['@MR-040', '@RN-01', '@RN-12'] },
  async ({ browser }) => {
    test.setTimeout(90_000);
    const master = await newSignedInContext(browser, 'Mestre Teste');
    const player = await newSignedInContext(browser, 'Jogador Teste');
    const m = await master.newPage();
    const p = await player.newPage();
    let campaignId = '';
    try {
      await m.goto('/');
      await p.goto('/');
      const table = await tableForLevelUp(m, p, `Sem marco ${Date.now()}`, { milestone: false });
      campaignId = table.campaignId;

      await p.goto(sheetOf(campaignId, table.characterId));
      await expect(p.getByLabel('Estado do personagem')).toContainText('Travada');
      await expect(p.getByRole('link', { name: /Subir para o nível/ })).toHaveCount(0);
      await expect(p.getByText('Pode subir de nível')).toHaveCount(0);

      await p.goto(`${sheetOf(campaignId, table.characterId)}/level-up`);
      await expect(p.getByRole('heading', { name: 'Ainda não dá para subir de nível' })).toBeVisible();
      await expect(p.getByText('Falta o mestre marcar um marco para este personagem')).toBeVisible();
      await expect(p.getByRole('link', { name: 'Voltar para a ficha' }).last()).toBeVisible();
      await expect(p.getByText(/Passo \d de \d/)).toHaveCount(0);

      // The master has no button either: the player levels up, the master edits the sheet.
      await m.goto(`${sheetOf(campaignId, table.characterId)}/level-up`);
      await expect(m.getByText('Quem sobe o nível é o jogador')).toBeVisible();

      // Once the master marks a milestone, the same route opens.
      await markMilestoneRPC(m, campaignId, 'Chegar ao Vale Seco', [table.characterId]);
      await p.goto(`${sheetOf(campaignId, table.characterId)}/level-up`);
      await expect(p.getByText('Passo 1 de 4 · Habilidades')).toBeVisible();
    } finally {
      await endOpenSessionRPC(m, campaignId);
      await master.close();
      await player.close();
    }
  },
);
