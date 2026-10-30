import { expect, test, type Browser, type Page } from '@playwright/test';

import { combatRPC, passTurnsTo, tableForCombat, type Encounter } from './combat-support';
import { vex } from './combat-rolls-support';
import { endOpenSessionRPC, openSessionPage } from './live-session-support';
import { masterLog } from './combat-layout';
import { callRPC, newSignedInContext, openCombatLog } from './support';

// The master's creature actions that are no attack roll (a breath weapon, a claw's rider), as the master and the player read them.
// The table and the combat come through the API, in the mind's theatre (the master names the targets, no map to pick on); the d20s
// are typed, so the order of the turns never depends on a roll. A saving throw's outcome is random, so the specs assert that the
// line exists and what the player must never read (the stat block's text, the DC), not whether the target saved.

/** A combat with one monster of the SRD against Vex, the monster first in the order, and its turn open. */
async function monsterTurn(browser: Browser, name: string, creatureKey: string) {
  const master = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: 1280, height: 1000 } });
  const player = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: 390, height: 844 } });
  const m = await master.newPage();
  const p = await player.newPage();
  await m.goto('/');
  await p.goto('/');
  const table = await tableForCombat(m, p, `${name} ${Date.now()}`, false, false, { build: vex, sheet: { weaponKeys: ['equipment:rapier'] } });
  const campaignId = table.campaignId;
  const started = await callRPC(m, 'meurpg.play.v1.CombatService/StartEncounter', {
    campaignId,
    idempotencyKey: crypto.randomUUID(),
    name: 'Emboscada na estrada',
    participants: [],
    monsters: [{ creatureKey, count: 1 }],
    mode: 'ENCOUNTER_MODE_THEATRE',
  });
  expect(started.ok(), await started.text()).toBeTruthy();
  let enc = (await started.json()).encounter as Encounter;
  const monsterIds = new Set(enc.combatants.filter((c) => (c as { bestiaryCreatureKey?: string }).bestiaryCreatureKey).map((c) => c.id));
  for (const c of enc.combatants) {
    enc = await combatRPC(m, 'SubmitInitiative', { campaignId, encounterId: enc.id, combatantId: c.id, d20Face: monsterIds.has(c.id) ? 20 : 1 });
  }
  enc = await combatRPC(m, 'BeginCombat', { campaignId, encounterId: enc.id });
  const monster = enc.combatants.find((c) => monsterIds.has(c.id))!;
  // A monster starts hidden, and the players read nothing of a hidden creature's turn: show it, as the master does at the ambush.
  await combatRPC(m, 'SetCombatantHidden', { campaignId, encounterId: enc.id, combatantId: monster.id, hidden: false });
  await passTurnsTo(m, campaignId, monster.label);
  await openSessionPage(p, campaignId);
  await openSessionPage(m, campaignId);
  const card = m.getByRole('region', { name: new RegExp(`^(Ações|Vez) d[ao] ${monster.label}$`) });
  return {
    m,
    p,
    card,
    done: async () => {
      await endOpenSessionRPC(m, campaignId);
      await master.close();
      await player.close();
    },
  };
}

/** The text of the combat log, for the audience of the page (the player's sheet or the master's fold, opened first). */
async function logText(page: Page, master: boolean): Promise<string> {
  const log = master ? await masterLog(page) : await openCombatLog(page);
  return (await log.textContent()) ?? '';
}

test('o sopro do cão infernal: o mestre usa a ação, o resultado aparece no cartão e o jogador lê a salvaguarda dele sem o texto do SRD nem a CD', { tag: ['@MR-012', '@RN-20'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  const { p, card, done } = await monsterTurn(browser, 'Sopro', 'monster:hell-hound');
  try {
    const use = card.getByRole('button', { name: /^Usar Sopro de fogo/ });
    await expect(use).toBeVisible();
    await expect(card.getByRole('group', { name: /Quem a ação atinge/ })).toBeVisible();
    await use.click();
    const result = card.getByTestId('creature-action-result');
    await expect(result).toBeVisible();
    // The save is random: the line says it passed or failed, and the master reads the roll against the DC.
    await expect(result).toContainText(/Passou|Falhou|Falha automática/);
    await expect(result).toContainText('Teste de resistência');
    // The action is spent.
    await expect(use).toBeDisabled();
    // The player's log carries the action and the save, never the stat block's words or the DC.
    const playerLog = await openCombatLog(p);
    await expect(playerLog).toContainText(/usou Sopro de fogo/);
    const log = (await playerLog.textContent()) ?? '';
    expect(log).toMatch(/Vex (falhou|resistiu)/);
    expect(log).not.toContain('Texto do SRD');
    expect(log).not.toMatch(/\bCD\b/);
  } finally {
    await done().catch(() => undefined);
  }
});

test('a mordida da carniçal acerta e oferece o efeito das garras: o mestre aplica e a oferta some', { tag: ['@MR-012'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  const { m, card, done } = await monsterTurn(browser, 'Garras', 'monster:ghoul');
  try {
    // A typed hit (a d20 of 15), the damage rolled and applied; the claw then offers its rider.
    await card.locator('label', { hasText: 'Garras' }).click();
    const claws = card.getByRole('button', { name: 'Digitar o resultado' });
    await claws.click();
    await card.getByLabel(/Role 1d20/).fill('15');
    await card.getByRole('button', { name: 'Confirmar 15' }).click();
    await card.getByRole('button', { name: 'Rolar dano' }).click();
    await card.getByRole('button', { name: /Aplicar \d+ de dano/ }).click();
    const offer = m.getByTestId('hit-rider').first();
    await expect(offer).toBeVisible();
    await offer.getByRole('button', { name: /^Aplicar / }).click();
    await expect(offer).toBeHidden();
    await expect.poll(() => logText(m, true)).toMatch(/usou Garras/);
  } finally {
    await done().catch(() => undefined);
  }
});

