import { expect, type Page } from '@playwright/test';

import { adjustVitalsRPC, combatRPC, getEncounterRPC, startEncounterRPC, type CombatTable, type Encounter } from './combat-support';
import { createCharacterRPC, callRPC, type CharacterBuild } from './support';

// Setup for the rest and class resource specs (Etapa 8, MR-012, MR-014, RN-02), through the API: the master's
// rests, the hit dice sheet, Lay on Hands, Flexible Casting, Metamagic and Bardic Inspiration. These tests prove
// the screens, not the campaign, character and combat setup other specs cover. Every test makes its own campaign.

/** Aurora, a human Paladin 3 of the Oath of Devotion: a pool of 15 points of Lay on Hands. */
export const aurora: CharacterBuild = {
  name: 'Aurora',
  raceKey: 'race:human',
  race: 'Humano',
  classKey: 'class:paladin',
  class: 'Paladino',
  subclassKey: 'subclass:devotion',
  subclass: 'Juramento de Devoção',
  level: 3,
  background: 'Soldado',
  backgroundSkillKeys: ['skill:athletics', 'skill:intimidation'],
  backgroundSkills: ['Atletismo', 'Intimidação'],
  extraSkillKeys: ['skill:insight', 'skill:religion'],
  extraSkills: ['Intuição', 'Religião'],
  scores: { for: 16, des: 10, con: 14, int: 8, sab: 12, car: 14 },
};
export const auroraSheet = { weaponKeys: ['equipment:longsword'], armorKey: 'equipment:chain-mail' };

/** Orla, a human Bard 1: three uses of a d6 of Bardic Inspiration (Charisma +3). */
export const orla: CharacterBuild = {
  name: 'Orla',
  raceKey: 'race:human',
  race: 'Humano',
  classKey: 'class:bard',
  class: 'Bardo',
  level: 1,
  background: 'Artista',
  backgroundSkillKeys: ['skill:acrobatics', 'skill:performance'],
  backgroundSkills: ['Acrobacia', 'Atuação'],
  extraSkillKeys: ['skill:persuasion', 'skill:insight', 'skill:deception'],
  extraSkills: ['Persuasão', 'Intuição', 'Enganação'],
  scores: { for: 10, des: 14, con: 12, int: 12, sab: 10, car: 16 },
};
export const orlaSheet = {
  weaponKeys: ['equipment:dagger'],
  cantripKeys: ['spell:vicious-mockery', 'spell:minor-illusion'],
  knownSpellKeys: ['spell:healing-word', 'spell:thunderwave', 'spell:faerie-fire', 'spell:sleep', 'spell:charm-person'],
  preparedSpellKeys: ['spell:healing-word', 'spell:thunderwave', 'spell:faerie-fire', 'spell:sleep', 'spell:charm-person'],
};

/** Nael, a human Sorcerer 3 of the Draconic Bloodline who knows Careful and Twinned Spell: three points of sorcery. */
export const nael: CharacterBuild = {
  name: 'Nael',
  raceKey: 'race:human',
  race: 'Humano',
  classKey: 'class:sorcerer',
  class: 'Feiticeiro',
  subclassKey: 'subclass:draconic',
  subclass: 'Linhagem Dracônica',
  level: 3,
  background: 'Eremita',
  backgroundSkillKeys: ['skill:insight', 'skill:religion'],
  backgroundSkills: ['Intuição', 'Religião'],
  extraSkillKeys: ['skill:persuasion', 'skill:arcana'],
  extraSkills: ['Persuasão', 'Arcanismo'],
  scores: { for: 8, des: 14, con: 14, int: 10, sab: 12, car: 16 },
};
const naelSpells = ['spell:burning-hands', 'spell:charm-person', 'spell:magic-missile', 'spell:sleep'];
export const naelSheet = {
  cantripKeys: ['spell:fire-bolt', 'spell:ray-of-frost', 'spell:light', 'spell:mage-hand'],
  knownSpellKeys: naelSpells,
  preparedSpellKeys: naelSpells,
  featureChoiceKeys: ['trait:draconic-ancestry-red', 'feature:metamagic-careful-spell', 'feature:metamagic-twinned-spell'],
};

/** A story NPC ally with `hitPointsMax` hit points. */
export async function createAllyRPC(master: Page, campaignId: string, name: string, hitPointsMax = 20): Promise<string> {
  const res = await createCharacterRPC(master, campaignId, {
    kind: 'CHARACTER_KIND_STORY',
    name,
    sheet: { basic: { hitPointsMax, armorClass: 14, speedFt: 30, attackBonus: 0, damage: '1d4', description: '' } },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).character.id as string;
}

/** An NPC made from an SRD creature (a Skeleton is undead), kind story. */
export async function createCreatureNpcRPC(master: Page, campaignId: string, creatureKey: string, name: string): Promise<string> {
  const res = await callRPC(master, 'meurpg.characters.v1.CharacterService/CreateNpcFromCreature', {
    campaignId,
    creatureKey,
    name,
    kind: 'CHARACTER_KIND_STORY',
    idempotencyKey: crypto.randomUUID(),
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).character.id as string;
}

/** The master takes damage off a combatant by hand (`AdjustCombatantHitPoints`). */
export async function damageCombatantRPC(master: Page, campaignId: string, label: string, damage: number): Promise<Encounter> {
  const enc = await getEncounterRPC(master, campaignId);
  return combatRPC(master, 'AdjustCombatantHitPoints', {
    campaignId,
    encounterId: enc.id,
    combatantId: enc.combatants.find((c) => c.label === label)!.id,
    damage,
  });
}

/**
 * Starts a combat for the party and the given NPCs (one copy each, in plain sight), types every initiative so the
 * order is the same on every run (`faces` by label), puts everyone on the squares given (`at`, by label) and begins.
 */
export async function beginPlacedCombatRPC(
  master: Page,
  table: CombatTable,
  npcs: string[],
  faces: Record<string, number>,
  at: Record<string, [number, number]>,
): Promise<Encounter> {
  let enc = await startEncounterRPC(master, table, npcs.map((characterId) => ({ characterId, count: 1, hidden: false })));
  for (const c of enc.combatants) {
    enc = await combatRPC(master, 'SubmitInitiative', { campaignId: table.campaignId, encounterId: enc.id, combatantId: c.id, d20Face: faces[c.label] ?? 1 });
  }
  for (const [label, [col, row]] of Object.entries(at)) {
    const combatant = enc.combatants.find((c) => c.label === label);
    expect(combatant, `${label} is in the combat`).toBeTruthy();
    enc = await combatRPC(master, 'MoveCombatant', { campaignId: table.campaignId, encounterId: enc.id, combatantId: combatant!.id, col, row });
  }
  return combatRPC(master, 'BeginCombat', { campaignId: table.campaignId, encounterId: enc.id });
}

/** The master's correction of a rest-ready character: hit points, dice and uses spent (Toren: Second Wind and Action Surge). */
export async function spendForRestRPC(master: Page, campaignId: string, characterId: string): Promise<void> {
  await adjustVitalsRPC(master, campaignId, characterId, {
    hitPointsCurrent: 10,
    hitDiceUsedByDie: { '10': 3 },
    resourcesUsed: [
      { key: 'second_wind', used: 1 },
      { key: 'action_surge', used: 1 },
    ],
  });
}
