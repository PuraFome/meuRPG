import { expect, type Page } from '@playwright/test';

import { callRPC } from './support';

// Setup for the "Conteúdo da mesa" specs (Etapa 10, slice 10.11, MR-025, RN-23, RN-10): the entries come through the
// API, so a test proves its own screen and not the editors another test already proves. Every test makes its own campaign.

const service = 'meurpg.rules.v1.TableContentService';

/** A table spell as the API's JSON, with the fields a test changes. */
export function spellBody(namePt: string, over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    namePt,
    level: 1,
    schoolKey: 'school:evocation',
    castingTime: { unit: 'CASTING_TIME_UNIT_ACTION', amount: 1 },
    range: { kind: 'SPELL_RANGE_KIND_RANGED', distanceFt: 60 },
    duration: { kind: 'SPELL_DURATION_KIND_INSTANTANEOUS' },
    components: { verbal: true, somatic: true },
    classKeys: ['class:wizard'],
    descPt: ['Um risco de tinta negra corta o ar e rasga o alvo.'],
    target: { kind: 'TABLE_SPELL_TARGET_KIND_CREATURE' },
    attack: 'ranged',
    damage: [{ damageTypeKey: 'damage-type:necrotic', dice: '2d8', perSlotLevel: '1d8' }],
    ...over,
  };
}

/** The race Corujeiro of the artboard: darkvision, Sabedoria +2 and Destreza +1, one trait with an effect and one with a note. */
export function raceBody(namePt = 'Corujeiro'): Record<string, unknown> {
  return {
    namePt,
    size: 'Medium',
    speedFt: 30,
    darkvisionFt: 60,
    abilityBonuses: { wisdom: 2, dexterity: 1 },
    languages: ['language:common'],
    traits: [
      { namePt: 'Olhos de caçador', descPt: ['Enxergam longe e no escuro.'], effects: [{ type: 'proficiency', proficiency: 'skill:perception' }] },
      { namePt: 'Planar', descPt: ['Cai devagar: sem dano de queda até 6 m.'], effects: [{ type: 'note', textPt: 'Cai devagar.' }] },
    ],
  };
}

export async function createEntryRPC(master: Page, campaignId: string, kind: 'tableSpell' | 'tableRace' | 'tableBackground', body: Record<string, unknown>): Promise<string> {
  const res = await callRPC(master, `${service}/CreateTableEntry`, { campaignId, [kind]: body });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).entry.key as string;
}

export async function archiveEntryRPC(master: Page, campaignId: string, key: string): Promise<void> {
  const res = await callRPC(master, `${service}/ArchiveTableEntry`, { campaignId, key });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** `ListTableEntries` as the JSON the app receives. */
export async function listEntriesJSON(page: Page, campaignId: string): Promise<{ entries?: Record<string, any>[]; tableRevision?: number }> {
  const res = await callRPC(page, `${service}/ListTableEntries`, { campaignId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/** The route of an entry's page: the key is one path segment. */
export function entryRoute(campaignId: string, key: string): string {
  return `/campaigns/${campaignId}/content/entries/${encodeURIComponent(key)}`;
}

/** `UpdateTableEntry` at the entry's current revision, as the other master's tab would. */
export async function updateEntryRPC(master: Page, campaignId: string, key: string, kind: 'tableSpell' | 'tableRace' | 'tableBackground', body: Record<string, unknown>): Promise<void> {
  const current = (await listEntriesJSON(master, campaignId)).entries!.find((e) => e.key === key)!;
  const res = await callRPC(master, `${service}/UpdateTableEntry`, { campaignId, key, expectedRevision: current.revision, [kind]: body });
  expect(res.ok(), await res.text()).toBeTruthy();
}
