import { expect, test, type Page } from '@playwright/test';

import { adjustVitalsRPC, combatRPC, getEncounterRPC, passTurnsTo, type Encounter } from './combat-support';
import { rollsTable, vex } from './combat-rolls-support';
import { openSessionPage } from './live-session-support';

// The master's combat against a player's character, as the rehearsal of the table found it (G3, G5, G7): the NPC's hit applied from its
// card reads as the attack on the player's sheet, the NPC's "Alvo" starts on an enemy of its own side, and a downed character's turn
// is the death save and nothing else. The table, the armor and the combat come through the API; what is under test is what the
// master and the player read. Every test makes its own campaign.

const FACES = { 'Goblin 1': 20, Vex: 10, 'Capitão Goblin': 5, 'Goblin 2': 4 };

/** The player's polite status lines (the sheet's announcements are visually hidden). */
async function statusLines(p: Page): Promise<string[]> {
  return p.locator('[role=status]').evaluateAll((es) => es.map((e) => (e.textContent ?? '').trim()).filter(Boolean));
}

/** A table with the armor added to the combat (hidden or not), standing at (9, 10), next to Vex (8, 9) and Goblin 1 (9, 9), and its turn open. */
async function armorTurn(browser: import('@playwright/test').Browser, name: string, hidden: boolean) {
  const t = await rollsTable(browser, name, vex, { weaponKeys: ['equipment:rapier'] }, FACES);
  let enc = await getEncounterRPC(t.m, t.campaignId);
  enc = await combatRPC(t.m, 'AddMonsters', { campaignId: t.campaignId, encounterId: enc.id, creatureKey: 'monster:animated-armor', count: 1, hidden });
  const armor = enc.combatants.find((c) => c.label === 'Armadura animada')!;
  enc = await combatRPC(t.m, 'MoveCombatant', { campaignId: t.campaignId, encounterId: enc.id, combatantId: armor.id, col: 9, row: 10 });
  enc = await passTurnsTo(t.m, t.campaignId, 'Armadura animada');
  await openSessionPage(t.p, t.campaignId);
  await openSessionPage(t.m, t.campaignId);
  const card = t.m.getByRole('region', { name: /^(Ações da|Vez da) Armadura animada$/ });
  return { ...t, enc, armorId: armor.id, card };
}

/** The master types a hit (a d20 of 15 against Vex), rolls the damage and applies it from the NPC card. */
async function hitAndApplyFromCard(m: Page, card: ReturnType<Page['getByRole']>): Promise<void> {
  await card.getByRole('button', { name: 'Digitar o resultado' }).click();
  await card.getByLabel(/Role 1d20/).fill('15');
  await card.getByRole('button', { name: 'Confirmar 15' }).click();
  await card.getByRole('button', { name: 'Rolar dano' }).click();
  await card.getByRole('button', { name: /Aplicar \d+ de dano/ }).click();
}

test('o dano do ataque de um NPC aplicado no cartão aparece na ficha do jogador como o ataque, e não como ajuste do mestre', { tag: ['@RN-02', '@MR-012'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  const { m, p, card, done } = await armorTurn(browser, 'Pancada', false);
  try {
    await hitAndApplyFromCard(m, card);
    await expect(p.locator('[role=status]').filter({ hasText: /^A Armadura animada acertou você: \d+ de dano de concussão\./ }).first()).toBeAttached();
    expect((await statusLines(p)).join(' ')).not.toContain('O mestre ajustou');
  } finally {
    await done().catch(() => undefined);
  }
});

test('o atacante escondido não dá nome na ficha: "Alguém acertou você", e o registro do jogador continua sem a linha', { tag: ['@RN-02', '@RN-20'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  const { m, p, card, done } = await armorTurn(browser, 'Pancada escondida', true);
  try {
    await hitAndApplyFromCard(m, card);
    await expect(p.locator('[role=status]').filter({ hasText: /^Alguém acertou você: \d+ de dano de concussão\./ }).first()).toBeAttached();
    const lines = (await statusLines(p)).join(' ');
    expect(lines).not.toContain('O mestre ajustou');
    expect(lines).not.toContain('Armadura animada');
  } finally {
    await done().catch(() => undefined);
  }
});

test('um ajuste de verdade do mestre continua dizendo "O mestre ajustou"', { tag: ['@RN-02'] }, async ({ browser }) => {
  test.setTimeout(120_000);
  const t = await rollsTable(browser, 'Ajuste', vex, { weaponKeys: ['equipment:rapier'] }, FACES);
  try {
    await openSessionPage(t.p, t.campaignId);
    await expect(t.p.getByText(/24 de 24/).first()).toBeVisible();
    await adjustVitalsRPC(t.m, t.campaignId, t.table.characterId, { hitPointsCurrent: 17 });
    await expect(t.p.locator('[role=status]').filter({ hasText: 'O mestre ajustou: 17 de 24 pontos de vida' }).first()).toBeAttached();
  } finally {
    await t.done().catch(() => undefined);
  }
});

test('o alvo do ataque do NPC começa no inimigo dele mais perto, nunca no aliado, e acompanha o NPC quando ele se move', { tag: ['@MR-012'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  const { m, campaignId, armorId, card, done } = await armorTurn(browser, 'Alvo', false);
  try {
    const target = card.getByRole('combobox', { name: /Alvo/ });
    // Goblin 1 (the armor's own side) stands at 1,5 m, first in the list; Vex is as near: the armor starts on Vex.
    await expect(target).toContainText('Vex');
    await expect(target).not.toContainText('Goblin 1');
    // The master moves the armor far away: nobody is in reach, and the default is still the player's character, not the ally.
    let enc: Encounter = await getEncounterRPC(m, campaignId);
    enc = await combatRPC(m, 'MoveCombatant', { campaignId, encounterId: enc.id, combatantId: armorId, col: 2, row: 2, forced: true });
    await expect(target).toContainText('Vex');
    await expect(target).toContainText('fora do alcance');
    // And back next to the party: in reach again.
    await combatRPC(m, 'MoveCombatant', { campaignId, encounterId: enc.id, combatantId: armorId, col: 9, row: 10, forced: true });
    await expect(target).toContainText('Vex');
    await expect(target).not.toContainText('fora do alcance');
  } finally {
    await done().catch(() => undefined);
  }
});

test('no turno de um personagem a 0 PV o teste contra a morte é o turno inteiro: depois dele "Próximo turno" passa sem perguntar', { tag: ['@RN-03', '@MR-053'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  const t = await rollsTable(browser, 'Caído passa', vex, { weaponKeys: ['equipment:rapier'] }, FACES);
  const { m, campaignId } = t;
  try {
    await adjustVitalsRPC(m, campaignId, t.table.characterId, { hitPointsCurrent: 0 });
    await passTurnsTo(m, campaignId, 'Vex');
    await openSessionPage(m, campaignId);
    await expect(m.getByRole('heading', { name: 'Vex está caído: teste contra a morte' })).toBeVisible();
    await m.getByRole('button', { name: 'Digitar o resultado' }).click();
    await m.getByLabel(/Role 1d20 para o teste contra a morte/).fill('15');
    await m.getByRole('button', { name: 'Confirmar 15' }).click();
    await expect(m.getByRole('status').filter({ hasText: 'Teste contra a morte' }).first()).toBeVisible();
    // The save was the whole turn: nothing is asked, and the turn goes to the next one.
    await m.getByRole('button', { name: 'Próximo turno' }).click();
    await expect(m.getByRole('alertdialog', { name: /Passar a vez de Vex/ })).toHaveCount(0);
    await expect(m.getByText('Vez do Capitão Goblin').first()).toBeVisible();
  } finally {
    await t.done().catch(() => undefined);
  }
});

test('um personagem consciente que ainda não agiu continua perguntando: "Passar a vez de Vex, que ainda não agiu?"', { tag: ['@MR-053'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  const t = await rollsTable(browser, 'Consciente passa', vex, { weaponKeys: ['equipment:rapier'] }, FACES);
  const { m, campaignId } = t;
  try {
    await passTurnsTo(m, campaignId, 'Vex');
    await openSessionPage(m, campaignId);
    await m.getByRole('button', { name: 'Próximo turno' }).click();
    await expect(m.getByText(/Passar a vez de Vex\?/)).toBeVisible();
    await m.getByRole('button', { name: 'Cancelar' }).click();
    await expect(m.getByText(/Passar a vez de Vex\?/)).toHaveCount(0);
  } finally {
    await t.done().catch(() => undefined);
  }
});
