import { expect, type Page } from '@playwright/test';

import { tableForMaps, type MapsTable } from './maps-support';
import { callRPC, pensantus } from './support';

// Setup for the "Magias" specs (MR-045, slice 10.11b), through the API: these tests prove the
// page, not the campaign and character forms. Every test makes its own campaign.

/** A level 1 Mago (no subclass yet): "Só as que posso aprender" gives truques and the 1st circle. */
export const mageLevel1 = { ...pensantus, name: 'Lia', level: 1, subclassKey: undefined, subclass: undefined };

export async function tableForSpells(master: Page, player: Page, name: string): Promise<MapsTable> {
  return tableForMaps(master, player, name, false, {}, mageLevel1);
}

/** The master's own spell, in the Mago's list: "Lâmina de Nanquim" (a creature, ranged attack, 2d8 necrotic). */
export async function createInkBladeRPC(master: Page, campaignId: string): Promise<string> {
  const res = await callRPC(master, 'meurpg.rules.v1.TableContentService/CreateTableEntry', {
    campaignId,
    tableSpell: {
      namePt: 'Lâmina de Nanquim',
      level: 1,
      schoolKey: 'school:evocation',
      castingTime: { unit: 'CASTING_TIME_UNIT_ACTION', amount: 1 },
      range: { kind: 'SPELL_RANGE_KIND_RANGED', distanceFt: 60 },
      duration: { kind: 'SPELL_DURATION_KIND_INSTANTANEOUS' },
      components: { verbal: true, somatic: true, material: true, materialPt: 'uma pena molhada em tinta' },
      classKeys: ['class:wizard'],
      descPt: ['Um risco de tinta negra corta o ar e rasga o alvo.'],
      higherLevelPt: ['+1d8 de dano por círculo acima do 1º.'],
      target: { kind: 'TABLE_SPELL_TARGET_KIND_CREATURE' },
      attack: 'ranged',
      damage: [{ damageTypeKey: 'damage-type:necrotic', dice: '2d8', perSlotLevel: '1d8' }],
    },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).entry.key as string;
}

export async function archiveEntryRPC(master: Page, campaignId: string, key: string): Promise<void> {
  const res = await callRPC(master, 'meurpg.rules.v1.TableContentService/ArchiveTableEntry', { campaignId, key });
  expect(res.ok(), await res.text()).toBeTruthy();
}

export interface SpellRow {
  key: string;
  namePt: string;
  level?: number;
  archived?: boolean;
  classKeys?: string[];
}

/** What `ListSpells` answers, as the app's JSON (the page's own read). */
export async function listSpellsRPC(
  page: Page,
  campaignId: string,
  filter: Record<string, unknown> = {},
): Promise<{ spells: SpellRow[]; total: number; nextPageToken?: string }> {
  const res = await callRPC(page, 'meurpg.rules.v1.ContentService/ListSpells', { campaignId, pageSize: 400, ...filter });
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  return { spells: body.spells ?? [], total: body.total ?? 0, nextPageToken: body.nextPageToken };
}
