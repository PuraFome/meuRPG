import { expect, type Page } from '@playwright/test';

import { callRPC } from './support';

// Setup for the class and subclass editor specs (Etapa 10, slice 10.12, MR-025, RN-23): the entries come through the API, so
// a test proves its own screen. Every test makes its own campaign.

const service = 'meurpg.rules.v1.TableContentService';

/** The 20 rows of a class that does not cast, with the SRD's proficiency bonus. */
export function plainRows(): Record<string, unknown>[] {
  return Array.from({ length: 20 }, (_, i) => ({ profBonus: 2 + Math.floor(i / 4), features: [] }));
}

/** A table class as the API's JSON: a fighter-like class with two saving throws and five skills. */
export function classBody(namePt = 'Guardião do Vale', over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    namePt,
    hitDie: 10,
    savingThrows: ['ABILITY_STRENGTH', 'ABILITY_WISDOM'],
    skillChoose: 2,
    skillFrom: ['skill:athletics', 'skill:insight', 'skill:nature', 'skill:perception', 'skill:survival'],
    proficiencies: ['proficiency:light-armor', 'proficiency:medium-armor', 'proficiency:shields', 'proficiency:simple-weapons', 'proficiency:martial-weapons'],
    subclassLevel: 3,
    levels: plainRows(),
    ...over,
  };
}

export async function createClassRPC(master: Page, campaignId: string, body: Record<string, unknown>): Promise<string> {
  const res = await callRPC(master, `${service}/CreateTableEntry`, { campaignId, tableClass: body });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).entry.key as string;
}

export async function createSubclassRPC(master: Page, campaignId: string, body: Record<string, unknown>): Promise<string> {
  const res = await callRPC(master, `${service}/CreateTableEntry`, { campaignId, tableSubclass: body });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).entry.key as string;
}

export async function listEntries(page: Page, campaignId: string): Promise<Record<string, any>[]> {
  const res = await callRPC(page, `${service}/ListTableEntries`, { campaignId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return ((await res.json()).entries ?? []) as Record<string, any>[];
}

/** A half-caster class as the API's JSON: slots of the 1st circle from level 2, the druid's list. */
export function halfCasterBody(namePt = 'Guardião do Vale'): Record<string, unknown> {
  return classBody(namePt, {
    casting: { kind: 'half', ability: 'ABILITY_WISDOM', preparation: 'prepared', listFrom: 'class:druid' },
    levels: plainRows().map((row, i) => (i === 0 ? row : { ...row, slots: [2, 0, 0, 0, 0, 0, 0, 0, 0] })),
  });
}
