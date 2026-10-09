import type { Browser, BrowserContext, Page } from '@playwright/test';

import { beginAttackCombatRPC, tableForCombat, type CombatTable } from './combat-support';
import { endOpenSessionRPC } from './live-session-support';
import { newSignedInContext, type CharacterBuild } from './support';

// The table of the attack-roll specs: a player's character beside the goblins, adjacent to
// Goblin 1, with the combat begun. The characters come through the API.

export const vex: CharacterBuild = {
  name: 'Vex',
  raceKey: 'race:human',
  race: 'Humano',
  classKey: 'class:rogue',
  class: 'Ladino',
  subclassKey: 'subclass:thief',
  subclass: 'Ladrão',
  level: 3,
  background: 'Criminoso',
  backgroundSkillKeys: ['skill:stealth', 'skill:deception'],
  backgroundSkills: ['Furtividade', 'Enganação'],
  extraSkillKeys: ['skill:acrobatics', 'skill:perception', 'skill:investigation', 'skill:sleight-of-hand'],
  extraSkills: ['Acrobacia', 'Percepção', 'Investigação', 'Prestidigitação'],
  scores: { for: 10, des: 16, con: 14, int: 12, sab: 12, car: 10 },
};

export const grog: CharacterBuild = {
  name: 'Grog',
  raceKey: 'race:human',
  race: 'Humano',
  classKey: 'class:barbarian',
  class: 'Bárbaro',
  subclassKey: 'subclass:berserker',
  subclass: 'Caminho do Furioso',
  level: 3,
  background: 'Forasteiro',
  backgroundSkillKeys: ['skill:athletics', 'skill:survival'],
  backgroundSkills: ['Atletismo', 'Sobrevivência'],
  extraSkillKeys: ['skill:perception', 'skill:intimidation'],
  extraSkills: ['Percepção', 'Intimidação'],
  scores: { for: 16, des: 14, con: 15, int: 8, sab: 12, car: 10 },
};

export interface RollsTable {
  m: Page;
  p: Page;
  table: CombatTable;
  campaignId: string;
  done: () => Promise<void>;
}

export async function rollsTable(
  browser: Browser,
  name: string,
  who: CharacterBuild,
  sheet: Record<string, unknown>,
  faces: Record<string, number>,
  look: { colorScheme?: 'light' | 'dark'; masterWidth?: number; playerWidth?: number } = {},
): Promise<RollsTable> {
  const master: BrowserContext = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: look.masterWidth ?? 1280, height: 900 }, colorScheme: look.colorScheme });
  const player: BrowserContext = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: look.playerWidth ?? 390, height: 844 }, colorScheme: look.colorScheme });
  const m = await master.newPage();
  const p = await player.newPage();
  await m.goto('/');
  await p.goto('/');
  const table = await tableForCombat(m, p, `${name} ${Date.now()}`, true, true, { build: who, sheet });
  await beginAttackCombatRPC(m, table, faces, { 'Capitão Goblin': [11, 5], 'Goblin 1': [9, 9], 'Goblin 2': [14, 10], [who.name]: [8, 9] });
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
