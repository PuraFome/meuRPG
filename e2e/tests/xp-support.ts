import { expect, type Page } from '@playwright/test';

import { combatRPC, startEncounterRPC, tableForCombat, type CombatTable, type Encounter } from './combat-support';
import { startSessionRPC } from './live-session-support';
import { callRPC, characterRpcBody, createCharacterRPC, pensantus } from './support';

// Setup for the XP specs (Etapa 7, MR-016, RN-09, RN-12), through the API:
// these tests prove the XP screens, not the campaign, character and combat
// ones other specs already cover. Every test makes its own campaign.

export type XpModeName = 'XP_MODE_ENEMIES' | 'XP_MODE_GOLD' | 'XP_MODE_MILESTONES';

/** A campaign of Mestre Teste's with Jogador Teste in it, playing Pensantus. */
export interface XpTable {
  campaignId: string;
  campaignName: string;
  /** Pensantus, the player's character. */
  characterId: string;
}

/**
 * A campaign that levels the way `xpMode` says, with Jogador Teste in it as
 * Pensantus (Mago 3, so the next level asks for 2.700 XP); `sheet` adds to his
 * full sheet (`experiencePoints: 2600`). No session yet: `startSessionRPC` does
 * that, which locks the sheet (RN-01). Master and player pages must be signed in
 * as each; neither navigates.
 */
export async function tableForXp(
  masterPage: Page,
  playerPage: Page,
  name: string,
  xpMode: XpModeName = 'XP_MODE_ENEMIES',
  sheet: Record<string, unknown> = {},
): Promise<XpTable> {
  const created = await callRPC(masterPage, 'meurpg.campaigns.v1.CampaignService/CreateCampaign', { name, xpMode });
  expect(created.ok(), await created.text()).toBeTruthy();
  const campaignId = (await created.json()).campaign.id as string;

  const invite = await callRPC(masterPage, 'meurpg.campaigns.v1.CampaignService/CreateInvite', {
    campaignId,
    maxUses: 1,
    expiresIn: '3600s',
  });
  expect(invite.ok()).toBeTruthy();
  const accepted = await callRPC(playerPage, 'meurpg.campaigns.v1.CampaignService/AcceptInvite', {
    token: (await invite.json()).token,
  });
  expect(accepted.ok()).toBeTruthy();

  const body = characterRpcBody('PLAYER', pensantus) as { sheet: { full: object } };
  body.sheet.full = { ...body.sheet.full, ...sheet };
  const character = await createCharacterRPC(playerPage, campaignId, body);
  expect(character.ok(), await character.text()).toBeTruthy();
  return { campaignId, campaignName: name, characterId: (await character.json()).character.id };
}

/** A table with a session open (Pensantus' sheet locked) in an enemies campaign. */
export async function tableWithSession(masterPage: Page, playerPage: Page, name: string, sheet: Record<string, unknown> = {}): Promise<XpTable> {
  const table = await tableForXp(masterPage, playerPage, name, 'XP_MODE_ENEMIES', sheet);
  await startSessionRPC(masterPage, table.campaignId);
  return table;
}

/** A goblin (a minion) that gives `xp` when defeated: the E7-11 numbers (ND 1/4, 50 XP). */
export async function createMinionRPC(page: Page, campaignId: string, name: string, challengeRating: string, xpValue: number): Promise<string> {
  const res = await createCharacterRPC(page, campaignId, {
    kind: 'CHARACTER_KIND_MINION',
    name,
    sheet: {
      basic: {
        hitPointsMax: 7,
        armorClass: 15,
        speedFt: 30,
        attacks: [{ name: 'Cimitarra', attackBonus: 4, damageDiceCount: 1, damageDiceSides: 6, damageBonus: 2, damageType: 'DAMAGE_TYPE_SLASHING', rangeFt: 5 }],
        description: '',
        challengeRating,
        xpValue,
      },
    },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).character.id as string;
}

/** The Capitão Goblin (an enemy, full sheet) with his own ND and XP. */
export async function createEnemyRPC(page: Page, campaignId: string, name: string, challengeRating: string, xpValue: number): Promise<string> {
  const body = characterRpcBody('ENEMY', { ...pensantus, name }) as { sheet: { full: object } };
  body.sheet.full = { ...body.sheet.full, challengeRating, xpValue };
  const res = await createCharacterRPC(page, campaignId, body);
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).character.id as string;
}

/** The combat of the artboards: the Capitão (200 XP) and three Goblins (50 each) fight
 * Pensantus, who is on the table. Returns the table (with its map, an open session
 * and an enemies campaign) and the NPCs' ids. */
export interface XpCombat {
  table: CombatTable;
  captainId: string;
  goblinId: string;
}

export async function tableForXpCombat(masterPage: Page, playerPage: Page, name: string, sheet: Record<string, unknown> = {}): Promise<XpCombat> {
  const table = await tableForCombat(masterPage, playerPage, name, true, false, { sheet });
  const captainId = await createEnemyRPC(masterPage, table.campaignId, 'Capitão da Emboscada', '1', 200);
  const goblinId = await createMinionRPC(masterPage, table.campaignId, 'Goblin da Emboscada', '1/4', 50);
  return { table, captainId, goblinId };
}

/** Wins the combat through the API, the way the master would at the table: the
 * Capitão and `goblins` Goblins fall, and the master ends it. The encounter is ENDED. */
export async function winCombatRPC(master: Page, combat: XpCombat, goblins = 3): Promise<Encounter> {
  const { table } = combat;
  let enc = await startEncounterRPC(master, table, [
    { characterId: combat.captainId, count: 1, hidden: false },
    { characterId: combat.goblinId, count: goblins, hidden: false },
  ], 'Emboscada na estrada');
  for (const c of enc.combatants) {
    enc = await combatRPC(master, 'SubmitInitiative', { campaignId: table.campaignId, encounterId: enc.id, combatantId: c.id, d20Face: 10 });
  }
  enc = await combatRPC(master, 'BeginCombat', { campaignId: table.campaignId, encounterId: enc.id });
  for (const c of enc.combatants.filter((x) => x.kind !== 'COMBATANT_KIND_PLAYER')) {
    enc = await combatRPC(master, 'AdjustCombatantHitPoints', { campaignId: table.campaignId, encounterId: enc.id, combatantId: c.id, damage: 999 });
  }
  return combatRPC(master, 'EndEncounter', { campaignId: table.campaignId, encounterId: enc.id });
}

/** `AwardXP` through the API: `mode` is the enum's short name (ENEMIES, GOLD, MANUAL). */
export async function awardXpRPC(
  page: Page,
  campaignId: string,
  body: { mode: 'ENEMIES' | 'GOLD' | 'MANUAL'; reason: string; characterIds: string[]; encounterId?: string; amount?: number; gold?: number },
) {
  const res = await callRPC(page, 'meurpg.progression.v1.ProgressionService/AwardXP', {
    campaignId,
    idempotencyKey: crypto.randomUUID(),
    ...body,
    mode: `XP_AWARD_MODE_${body.mode}`,
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()) as { award: { id: string }; xpEach: number; lostXp?: number };
}

/** `GetCampaignExperience`: each living player character's XP and level-up flag. */
export async function getExperienceRPC(page: Page, campaignId: string) {
  const res = await callRPC(page, 'meurpg.progression.v1.ProgressionService/GetCampaignExperience', { campaignId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()) as { characters: { characterId: string; name: string; experiencePoints?: number; nextLevelXp?: number; canLevelUp?: boolean }[] };
}
