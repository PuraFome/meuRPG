import { expect, type Page } from '@playwright/test';

import { canvasPng, createMapRPC, uploadImageRPC } from './maps-support';
import { callRPC } from './support';
import { tableForXp, type XpModeName, type XpTable } from './xp-support';

// Setup for the gold specs (Etapa 9, MR-041, MR-032, RN-09, RN-10), through the
// API: marking a treasure found has no screen yet (slice 9.14), so the tests
// mark them here and prove "Voltar à cidade", the history and the summary.

/** A table with Pensantus (Jogador Teste's, the only living player character the two test accounts
 * can have in one campaign, RN-03) and a map to put treasures on. */
export interface GoldTable extends XpTable {
  /** Pensantus, then whoever `addParty` added (the screenshots seed Toren and Brisa in the database). */
  characterIds: string[];
  mapId: string;
}

export async function tableForGold(
  masterPage: Page,
  playerPage: Page,
  name: string,
  xpMode: XpModeName = 'XP_MODE_GOLD',
  addParty?: (table: XpTable) => Promise<string[]>,
): Promise<GoldTable> {
  const table = await tableForXp(masterPage, playerPage, name, xpMode);
  const characterIds = [table.characterId, ...((await addParty?.(table)) ?? [])];
  const png = await canvasPng(masterPage, 640, 400, 'A caverna do Vale Seco');
  const imageId = await uploadImageRPC(masterPage, table.campaignId, 'A caverna do Vale Seco', png);
  const mapId = await createMapRPC(masterPage, table.campaignId, 'A caverna do Vale Seco', imageId);
  return { ...table, characterIds, mapId };
}

/** A treasure on the map, marked found by `finders` (character IDs); returns the point's ID. */
export async function treasureFoundRPC(
  master: Page,
  table: GoldTable,
  spec: { name: string; valuePo: number; finders: string[]; xBp?: number; yBp?: number },
): Promise<string> {
  const created = await callRPC(master, 'meurpg.maps.v1.MapService/CreateMapPoint', {
    campaignId: table.campaignId,
    mapId: table.mapId,
    kind: 'MAP_POINT_KIND_TREASURE',
    name: spec.name,
    description: 'Moedas e uma adaga de prata.',
    xBp: spec.xBp ?? 3000,
    yBp: spec.yBp ?? 4000,
    treasureValuePo: spec.valuePo,
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const pointId = (await created.json()).point.id as string;
  const found = await callRPC(master, 'meurpg.maps.v1.MapService/MarkTreasureFound', {
    campaignId: table.campaignId,
    mapId: table.mapId,
    pointId,
    characterIds: spec.finders,
  });
  expect(found.ok(), await found.text()).toBeTruthy();
  return pointId;
}

/** The artboards' three finds: 250 + 120 + 50 = 420 PO. With one character they all found them. */
export async function threeTreasuresRPC(master: Page, table: GoldTable): Promise<string[]> {
  const [pensantusId, torenId = pensantusId, brisaId = pensantusId] = table.characterIds;
  return [
    await treasureFoundRPC(master, table, { name: 'Baú de moedas', valuePo: 250, finders: [brisaId], xBp: 3000 }),
    await treasureFoundRPC(master, table, { name: 'Bolsa do capitão', valuePo: 120, finders: [torenId], xBp: 5000 }),
    await treasureFoundRPC(master, table, { name: 'Ídolo de prata', valuePo: 50, finders: [...new Set([pensantusId, torenId])], xBp: 7000 }),
  ];
}
