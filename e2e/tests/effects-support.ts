import { expect, type Browser, type BrowserContext, type Locator, type Page } from '@playwright/test';

import { beginAttackCombatRPC, getEncounterRPC, tableForCombat, type CombatTable, type Encounter } from './combat-support';
import { endOpenSessionRPC, openSessionPage } from './live-session-support';
import { authStatePath, callRPC, newSignedInContext } from './support';

// Setup for the master's side of the effects that last (W7-E, RN-22): a combat with a few effects, put through the
// API, so the specs prove the panel, its dialogs and the exhaustion controls, not the spells that make an effect.

export interface EffectsTable {
  m: Page;
  campaignId: string;
  table: CombatTable;
  /** The combat as it began: Pensantus first, then the Capitão and the two Goblins. */
  encounter: Encounter;
  /** The id of the combatant with this label. */
  id: (label: string) => string;
  done: () => Promise<void>;
}

export const pensantusFirst = { Pensantus: 20, 'Capitão Goblin': 15, 'Goblin 1': 5, 'Goblin 2': 4 };

/** A write of `LastingEffectService`, with a fresh idempotency key; answers the JSON body. */
export async function effectsRPC(page: Page, method: string, body: object): Promise<Record<string, unknown>> {
  const res = await callRPC(page, `meurpg.play.v1.LastingEffectService/${method}`, { idempotencyKey: crypto.randomUUID(), ...body });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()) as Record<string, unknown>;
}

/** `AddLastingEffect` from the catalog. `duration` is the JSON of an `EffectDurationChoice`. */
export async function addEffectRPC(
  page: Page,
  campaignId: string,
  encounterId: string,
  catalogKey: string,
  targetIds: string[],
  extra: { casterId?: string; duration?: object; playerVisible?: boolean } = {},
): Promise<void> {
  await effectsRPC(page, 'AddLastingEffect', {
    campaignId,
    encounterId,
    catalogKey,
    targetIds,
    casterId: extra.casterId ?? '',
    duration: extra.duration ?? { kind: 'EFFECT_DURATION_KIND_UNTIL_DISMISSED' },
    ...(extra.playerVisible === undefined ? {} : { playerVisible: extra.playerVisible }),
  });
}

/** `SetExhaustion` of a character outside a combat (the level the screen shows is `expectedLevel`). */
export async function setExhaustionRPC(page: Page, campaignId: string, characterId: string, level: number, expectedLevel: number): Promise<void> {
  await effectsRPC(page, 'SetExhaustion', { campaignId, characterId, level, expectedLevel });
}

/**
 * A combat of Pensantus (the player's), the Capitão and two Goblins, begun with Pensantus first, with the master's page
 * at `width` and the player's at the phone's width. Effects: a Bênção (a concentration of Pensantus) on Pensantus and
 * Goblin 1, and a Derrubado the master put on Goblin 2.
 */
export async function effectsTable(
  browser: Browser,
  name: string,
  width = 1280,
  scheme: 'light' | 'dark' = 'light',
): Promise<EffectsTable> {
  const viewport = { width, height: width >= 768 ? 900 : 844 };
  const master: BrowserContext = await newSignedInContext(browser, 'Mestre Teste', { viewport, colorScheme: scheme });
  const player: BrowserContext = await browser.newContext({ storageState: authStatePath('Jogador Teste'), colorScheme: scheme, viewport: { width: 390, height: 844 } });
  const m = await master.newPage();
  const p = await player.newPage();
  await m.goto('/');
  await p.goto('/');
  const table = await tableForCombat(m, p, `${name} ${Date.now()}`, true, true);
  const encounter = await beginAttackCombatRPC(m, table, pensantusFirst);
  const id = (label: string) => encounter.combatants.find((c) => c.label === label)!.id;
  await addEffectRPC(m, table.campaignId, encounter.id, 'spell:bless', [id('Pensantus'), id('Goblin 1')], {
    casterId: id('Pensantus'),
    duration: { kind: 'EFFECT_DURATION_KIND_ROUNDS', rounds: 10 },
  });
  await addEffectRPC(m, table.campaignId, encounter.id, 'condition:prone', [id('Goblin 2')]);
  return {
    m,
    campaignId: table.campaignId,
    table,
    encounter,
    id,
    done: async () => {
      await endOpenSessionRPC(m, table.campaignId);
      await master.close();
      await player.close();
    },
  };
}

/** The master's page of the table, open on the session, with the panel on screen. */
export async function openPanel(t: EffectsTable): Promise<Locator> {
  await openSessionPage(t.m, t.campaignId);
  const panel = t.m.getByRole('region', { name: 'Efeitos em jogo' }).first();
  await expect(panel).toBeVisible();
  await expect(panel.locator('li.fx__row').first()).toBeVisible();
  return panel;
}

/** The row of the panel for an effect, by its name. */
export function effectRow(panel: Locator, name: string): Locator {
  return panel.locator('li.fx__row', { has: panel.page().getByRole('heading', { level: 3, name, exact: true }) });
}

/** The current level of exhaustion of a combatant, from the encounter the master reads. */
export async function exhaustionOf(page: Page, campaignId: string, label: string): Promise<number> {
  const enc = (await getEncounterRPC(page, campaignId)) as unknown as { combatants: { label: string; exhaustionLevel?: number }[] };
  return enc.combatants.find((c) => c.label === label)?.exhaustionLevel ?? 0;
}
