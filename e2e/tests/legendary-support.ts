import { expect, type Locator, type Page } from '@playwright/test';
import { mkdirSync } from 'node:fs';
import { join } from 'node:path';

import { type Encounter, attackCombatSquares, beginAttackCombatRPC, combatRPC, getEncounterRPC, passTurnsToAndWait, tableForCombat, pensantusCasting } from './combat-support';
import { openCombatPanel, openOrderRow, openTurn } from './combat-layout';
import { expectLoaded } from './loaded';
import { waitForEncounterRevision } from './revision-support';
import { callRPC, openCombatLog } from './support';
import { setTableRulesRPC } from './table-rules-support';

// Setup and checks for the master's legendary offers and the Parry switch (#557), through the API: the master's turn column
// with the adult red dragon's Legendary Resistance window and its legendary-actions card, the Parry window of a Capitão
// bandido the master asked to be asked about, and what the player reads while any of it waits. The table is Pensantus
// (player, first in the order, with a dagger and Mãos Flamejantes), the Capitão Goblin and two Goblins, the dragon and the
// Capitão bandido, all in plain sight. Every test makes its own campaign. The pages must be signed in as each person.

export const DRAGON = 'Dragão vermelho adulto';
export const BANDIT = 'Capitão bandido';

/** The widths of the screens: a phone, a tablet and a computer. */
export const WIDTHS = [390, 768, 1280] as const;

export interface LegendaryFight {
  campaignId: string;
  enc: Encounter;
  /** The id of a combatant by its label. */
  id: (label: string) => string;
}

/** The faces of the initiative: Pensantus first, the dragon and the Capitão bandido after him, no total repeated. */
const faces: Record<string, number> = { Pensantus: 20, [DRAGON]: 16, [BANDIT]: 11, 'Capitão Goblin': 6, 'Goblin 1': 4, 'Goblin 2': 3 };

/** Where everyone stands: the Capitão bandido and a Goblin are next to Pensantus (5, 7); the dragon is a cone away to the east. */
const squares: Record<string, [number, number]> = { ...attackCombatSquares, [BANDIT]: [6, 7], 'Goblin 1': [5, 8], [DRAGON]: [8, 7] };

/**
 * The table and the combat on Pensantus's turn: the dragon (Legendary Resistance 3 a day, 3 legendary actions: Detectar,
 * Ataque de cauda, Ataque de asas), the Capitão bandido (Parry) and the Goblins, with the master's and the player's pages
 * on the session.
 */
export async function beginLegendaryFight(master: Page, player: Page, name: string): Promise<LegendaryFight> {
  const table = await tableForCombat(master, player, name, true, true, { sheet: pensantusCasting });
  const enc = await beginAttackCombatRPC(master, table, faces, squares, [], ['monster:adult-red-dragon', 'monster:bandit-captain']);
  return fightOf(table.campaignId, enc);
}

function fightOf(campaignId: string, enc: Encounter): LegendaryFight {
  return { campaignId, enc, id: (label) => enc.combatants.find((c) => c.label === label)!.id };
}

/** The fight as the server has it now. */
export async function refreshed(master: Page, fight: LegendaryFight): Promise<LegendaryFight> {
  return fightOf(fight.campaignId, await getEncounterRPC(master, fight.campaignId));
}

/** The master sets a combatant's conditions (replacing the old ones; none clears them) and waits for both pages to show it. */
export async function setConditions(master: Page, pages: Page[], fight: LegendaryFight, label: string, keys: string[]): Promise<LegendaryFight> {
  const enc = await combatRPC(master, 'SetCombatantConditions', { campaignId: fight.campaignId, encounterId: fight.enc.id, combatantId: fight.id(label), conditions: { keys } });
  await waitForEncounterRevision(pages, enc);
  return fightOf(fight.campaignId, enc);
}

/** Ends Pensantus's turn and passes the others' until his comes again (the next round), with the pages showing it. */
export async function nextTurnOfPensantus(master: Page, pages: Page[], fight: LegendaryFight): Promise<LegendaryFight> {
  const now = await getEncounterRPC(master, fight.campaignId);
  await combatRPC(master, 'EndTurn', { campaignId: fight.campaignId, encounterId: now.id, expectedCombatantId: now.currentCombatantId, discardPendingDamage: true });
  return fightOf(fight.campaignId, await passTurnsToAndWait(master, fight.campaignId, 'Pensantus', pages));
}

/** Burning Hands from Pensantus, a cone to the east (a Dexterity save): the server takes who stands inside. */
export async function castBurningHands(player: Page, fight: LegendaryFight): Promise<void> {
  const res = await callRPC(player, 'meurpg.play.v1.CombatService/CastSpell', {
    campaignId: fight.campaignId,
    encounterId: fight.enc.id,
    casterId: fight.id('Pensantus'),
    spellKey: 'spell:burning-hands',
    slot: { level: 1 },
    direction: { dx: 1, dy: 0 },
    idempotencyKey: crypto.randomUUID(),
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** Pensantus's dagger at `target`, with the face that makes the total exactly the target's armor class: a hit that a Parry (+2) turns into a miss. */
export async function daggerAtArmorClass(player: Page, fight: LegendaryFight, target: string, armorClass: number): Promise<void> {
  const options = await callRPC(player, 'meurpg.play.v1.CombatService/GetTurnOptions', { campaignId: fight.campaignId, encounterId: fight.enc.id, combatantId: fight.id('Pensantus') });
  expect(options.ok(), await options.text()).toBeTruthy();
  const attacks = ((await options.json()).options?.attacks ?? []) as { attack: { key: string; attackBonus?: number } }[];
  const dagger = attacks.find((a) => a.attack.key === 'equipment:dagger')?.attack;
  expect(dagger, 'Pensantus has a dagger').toBeTruthy();
  const res = await callRPC(player, 'meurpg.play.v1.CombatService/RollAttack', {
    campaignId: fight.campaignId,
    encounterId: fight.enc.id,
    attackerId: fight.id('Pensantus'),
    attackKey: 'equipment:dagger',
    targetId: fight.id(target),
    idempotencyKey: crypto.randomUUID(),
    d20Face: armorClass - (dagger?.attackBonus ?? 0),
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** The folder the screenshots go to when `E2E_SHOTS_DIR` names one; otherwise nothing is written. */
const shotsDir = process.env.E2E_SHOTS_DIR;

/** The device scale of a context that takes screenshots: 2 when they are asked for, else the browser's own. */
export const shotScale = shotsDir ? { deviceScaleFactor: 2 } : {};

/** Saves the page as `557-<screen>-<width>-<claro|escuro>.png` when `E2E_SHOTS_DIR` is set; a normal run writes nothing. */
export async function shoot(page: Page, screen: string, width: number, colorScheme: 'light' | 'dark'): Promise<void> {
  if (!shotsDir) {
    return;
  }
  mkdirSync(shotsDir, { recursive: true });
  await page.screenshot({ path: join(shotsDir, `557-${screen}-${width}-${colorScheme === 'light' ? 'claro' : 'escuro'}.png`), fullPage: true });
}

/** Runs `check` at each width of `WIDTHS` on the pages: the pages are resized first, so the same screen is judged at all three. */
export async function atEachWidth(pages: Page[], check: (width: number) => Promise<void>): Promise<void> {
  for (const width of WIDTHS) {
    for (const page of pages) {
      await page.setViewportSize({ width, height: 1100 });
    }
    await check(width);
  }
}

/** What a screen is judged with: axe and the layout checks of the spec, and the screenshots of the run. */
export type Scan = (page: Page, screen: string) => Promise<void>;

interface Look {
  m: Page;
  p: Page;
  scheme: 'light' | 'dark';
  scan: Scan;
}

/** Scans `page` (a screen named `screen` at `width`) and, when screenshots are asked for, saves `shot` (the page when not given). */
async function look(l: Look, page: Page, screen: string, width: number, shot?: Locator): Promise<void> {
  await expectLoaded(page);
  await l.scan(page, `${screen} (${l.scheme}, ${width}px)`);
  if (!shot) {
    await shoot(page, screen, width, l.scheme);
    return;
  }
  if (shotsDir) {
    mkdirSync(shotsDir, { recursive: true });
    await shot.screenshot({ path: join(shotsDir, `557-${screen}-${width}-${l.scheme === 'light' ? 'claro' : 'escuro'}.png`) });
  }
}

/** The text of the player's "Esperando o mestre" card: the same words whatever the master is deciding. */
async function waitText(player: Page): Promise<string> {
  const wait = player.getByTestId('master-wait').first();
  await expect(wait).toBeVisible();
  return ((await wait.innerText()) ?? '').replace(/\s+/g, ' ').trim();
}

/** Nothing of the master's decisions reaches a player: no legendary control, no Parry, no queue. */
async function expectNeutralPlayer(player: Page): Promise<void> {
  await expect(player.getByText(/Resistência Lendária|Aparar|ações? lendárias?|Esperando você/)).toHaveCount(0);
  await expect(player.getByTestId('legendary-card')).toHaveCount(0);
  await expect(player.getByTestId('parry-offer')).toHaveCount(0);
  await expect(player.getByTestId('resistance-offer')).toHaveCount(0);
}

/** `first` comes before `second` in the tab order: Tab from the first lands on the second. */
async function expectTabGoes(page: Page, first: Locator, second: Locator): Promise<void> {
  await first.focus();
  await page.keyboard.press('Tab');
  await expect(second).toBeFocused();
}

/** The legendary-actions card of the dragon, unfolded, with its three rows. */
async function legendaryCardScreens(l: Look, fight: LegendaryFight): Promise<void> {
  const { m, p } = l;
  const card = m.getByTestId('legendary-card');
  await expect(card).toBeVisible();
  await expect(card.getByTestId('legendary-badge')).toHaveText('Opcional');
  await expect(card.getByTestId('legendary-row')).toHaveCount(3);
  await expect(card.getByTestId('legendary-row').nth(0)).toContainText(/Detectar\s*custa 1/);
  await expect(card.getByRole('button', { name: 'Usar: Detectar' })).toBeEnabled();
  await expect(card.getByRole('button', { name: 'Atacar: Ataque de cauda' })).toBeEnabled();
  await expect(card.getByRole('button', { name: 'Usar: Ataque de asas' })).toBeEnabled();
  await expect(card.getByTestId('legendary-row').nth(2)).toContainText('custa 2');
  await expect(m.getByText(`Restam 3 ações lendárias: ${DRAGON}.`)).toBeVisible();
  await expectNeutralPlayer(p);
  await atEachWidth([m, p], async (width) => {
    await openTurn(m);
    await look(l, m, 'acoes-lendarias', width, card);
    // Folded: the same card is one line with "Abrir"; nothing is sent, so the player and the count stay as they were.
    await card.getByRole('button', { name: /^Agora não: dobrar as ações lendárias/ }).click();
    const folded = m.getByTestId('legendary-folded');
    await expect(folded).toContainText('Ações lendárias · 3 de 3');
    await look(l, m, 'acoes-lendarias-dobrada', width, folded);
    await folded.getByRole('button', { name: /^Abrir as ações lendárias/ }).click();
    await expect(card).toBeVisible();
  });
  expect((await refreshed(m, fight)).enc.revision, 'folding calls no server').toBe(fight.enc.revision);
}

/** Two spends of "Detectar": the card says what was spent and the third row cannot be afforded; the log is the master's alone. */
async function legendarySpendScreens(l: Look): Promise<void> {
  const { m, p } = l;
  const card = m.getByTestId('legendary-card');
  const detect = card.getByRole('button', { name: 'Usar: Detectar' });
  await detect.click();
  await expect(card.getByText('2 de 3')).toBeVisible();
  await detect.click();
  await expect(card.getByText('1 de 3')).toBeVisible();
  await expect(card.getByTestId('legendary-spent')).toContainText(`${DRAGON} gastou 1 ação lendária: Detectar`);
  await expect(card.getByTestId('legendary-spent')).toContainText('Só você vê');
  const wings = card.getByRole('button', { name: 'Usar: Ataque de asas' });
  await expect(wings).toBeDisabled();
  await expect(wings).toHaveAccessibleDescription('Custa 2; restam 1.');
  await expect(card.getByRole('button', { name: 'Usar: Detectar' })).toBeEnabled();
  await expect(m.getByText(`Restam 1 ação lendária: ${DRAGON}.`)).toBeVisible();
  await atEachWidth([m, p], async (width) => {
    await openTurn(m);
    await look(l, m, 'acoes-lendarias-gasto', width, card);
  });
  // The log: the master reads both spends with "Só o mestre vê"; the player reads no line of them.
  await atEachWidth([m, p], async (width) => {
    await openTurn(m);
    await openCombatPanel(m, 'Registro');
    const log = m.getByRole('log', { name: 'Registro do combate' });
    const lines = log.getByRole('listitem').filter({ hasText: 'gastou 1 ação lendária: Detectar' });
    await expect(lines).toHaveCount(2);
    await expect(lines.filter({ hasText: 'Só o mestre vê' })).toHaveCount(2);
    await look(l, m, 'registro-lendario-mestre', width);
    const own = await openCombatLog(p);
    await expect(own.getByText(/ação lendária|ações lendárias/)).toHaveCount(0);
    await expect(own.getByText('Só o mestre vê').filter({ hasText: /lendária/ })).toHaveCount(0);
    await look(l, p, 'registro-lendario-jogador', width);
    await p.keyboard.press('Escape');
  });
}

/** The failed save of the stunned dragon against Mãos Flamejantes: the master's Legendary Resistance window, with the legendary card behind it. */
async function resistanceScreens(l: Look, fight: LegendaryFight): Promise<LegendaryFight> {
  const { m, p } = l;
  const stunned = await setConditions(m, [m, p], fight, DRAGON, ['condition:stunned']);
  await castBurningHands(p, stunned);
  const offer = m.getByTestId('resistance-offer');
  await expect(offer).toBeVisible();
  await expect(offer.getByRole('heading', { name: 'Transformar a falha em sucesso?' })).toBeVisible();
  await expect(offer).toContainText(`${DRAGON} falhou:`);
  await expect(offer).toContainText('Resistência Lendária: restam 3 de 3 hoje · usar gasta 1');
  await expect(offer.getByRole('checkbox')).toHaveCount(0);
  const decline = offer.getByRole('button', { name: 'Deixar como rolou' });
  const use = offer.getByRole('button', { name: 'Usar Resistência Lendária' });
  await expect(use).toBeEnabled();
  await expectTabGoes(m, decline, use);
  const behind = m.getByTestId('legendary-card');
  await expect(behind.getByTestId('legendary-badge')).toHaveText('Depois');
  await expect(behind).toContainText('Depois da janela acima: enquanto uma reação espera, nada anda.');
  await expect(behind.getByTestId('legendary-row')).toHaveCount(0);
  await expect(m.getByTestId('queue-header')).toContainText('Esperando você');
  const spell = await waitText(p);
  expect(spell).toContain('Esperando o mestre');
  await expectNeutralPlayer(p);
  await atEachWidth([m, p], async (width) => {
    await openTurn(m);
    await look(l, m, 'resistencia-lendaria-mestre', width);
    await look(l, p, 'esperando-o-mestre-conjuracao-jogador', width);
  });
  await decline.click();
  await expect(offer).toHaveCount(0);
  await expect(p.getByTestId('master-wait')).toHaveCount(0);
  return setConditions(m, [m, p], await refreshed(m, stunned), DRAGON, []);
}

/** The entry "Aparar: …" of the order's "Mais" menu: only for an NPC with Parry, "decidir sozinho" until the master switches it. */
async function parryMenuScreens(l: Look): Promise<void> {
  const { m, p } = l;
  const menuOf = (label: string) => m.getByRole('button', { name: `Mais ações para ${label}` });
  await atEachWidth([m, p], async (width) => {
    await openOrderRow(m, BANDIT);
    await menuOf(BANDIT).click();
    await expect(m.getByRole('menuitem', { name: 'Aparar: decidir sozinho' })).toBeVisible();
    await look(l, m, 'menu-aparar', width);
    await m.keyboard.press('Escape');
    await expect(m.getByRole('menu')).toHaveCount(0);
  });
  await openOrderRow(m, 'Goblin 1');
  await menuOf('Goblin 1').click();
  await expect(m.getByRole('menuitem', { name: /^Aparar/ })).toHaveCount(0);
  await m.keyboard.press('Escape');
  await expectNeutralPlayer(p);
  await openOrderRow(m, BANDIT);
  await menuOf(BANDIT).click();
  await m.getByRole('menuitem', { name: 'Aparar: decidir sozinho' }).click();
  await menuOf(BANDIT).click();
  await expect(m.getByRole('menuitem', { name: 'Aparar: perguntar a cada golpe' })).toBeVisible();
  await m.keyboard.press('Escape');
  await openTurn(m);
}

/** A hit on the Capitão bandido that its Parry (+2) would turn into a miss: the master's window, and the player's neutral wait. */
async function parryWindowScreens(l: Look, fight: LegendaryFight): Promise<string> {
  const { m, p } = l;
  await daggerAtArmorClass(p, fight, BANDIT, 15);
  const offer = m.getByTestId('parry-offer');
  await expect(offer).toBeVisible();
  await expect(offer.getByRole('heading', { name: 'Aparar o golpe de Pensantus?' })).toBeVisible();
  await expect(offer).toContainText('15 contra CA 15. Aparar soma 2: contra CA 17, vira erro.');
  const pass = offer.getByRole('button', { name: 'Deixar passar' });
  const parry = offer.getByRole('button', { name: 'Aparar (+2 CA)' });
  await expectTabGoes(m, pass, parry);
  await expect(m.getByTestId('legendary-card').getByTestId('legendary-badge')).toHaveText('Depois');
  const text = await waitText(p);
  expect(text).toContain('Esperando o mestre');
  await expectNeutralPlayer(p);
  await atEachWidth([m, p], async (width) => {
    await openTurn(m);
    await look(l, m, 'aparar-mestre', width);
    await look(l, p, 'esperando-o-mestre-ataque-jogador', width);
  });
  await pass.click();
  await expect(offer).toHaveCount(0);
  await expect(p.getByTestId('master-wait')).toHaveCount(0);
  return text;
}

/** The table rule "Sempre": a hit on a Goblin, which has no reaction, waits for the master and the player reads the very same words. */
async function noTellScreens(l: Look, fight: LegendaryFight, parryText: string): Promise<void> {
  const { m, p } = l;
  await setTableRulesRPC(m, fight.campaignId, { enemyReactions: 'ENEMY_REACTIONS_RULE_ALWAYS' });
  await daggerAtArmorClass(p, fight, 'Goblin 1', 15);
  await expect(m.getByRole('button', { name: 'Sem reação' })).toBeVisible();
  expect(await waitText(p)).toBe(parryText);
  await expectNeutralPlayer(p);
  await atEachWidth([m, p], async (width) => {
    await look(l, p, 'esperando-o-mestre-sem-reacao-jogador', width);
  });
  const none = m.getByRole('button', { name: 'Sem reação' });
  await none.click();
  // The hit opens a second check, for the damage it would do: the player reads the same card again.
  await expect(m.getByText('Ataque de Pensantus contra Goblin 1 · acertou')).toBeVisible();
  expect(await waitText(p)).toBe(parryText);
  await none.click();
  await expect(none).toHaveCount(0);
  await expect(p.getByTestId('master-wait')).toHaveCount(0);
}

/**
 * The legendary screens of one theme, in the order the combat gives them: the card on its own (open, folded, spent), the
 * log, the Legendary Resistance window of a stunned dragon, the Parry switch, the Parry window and the same wait for a
 * creature with no reaction. `scan` judges each screen at the three widths; screenshots are saved only with E2E_SHOTS_DIR.
 */
export async function legendaryScreens(m: Page, p: Page, scheme: 'light' | 'dark', scan: Scan, fight: LegendaryFight): Promise<void> {
  const l: Look = { m, p, scheme, scan };
  await legendaryCardScreens(l, fight);
  await legendarySpendScreens(l);
  let now = await resistanceScreens(l, fight);
  now = await nextTurnOfPensantus(m, [m, p], now);
  await parryMenuScreens(l);
  const parryText = await parryWindowScreens(l, now);
  now = await nextTurnOfPensantus(m, [m, p], now);
  await noTellScreens(l, now, parryText);
}
