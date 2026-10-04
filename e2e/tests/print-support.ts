import { expect, type Page } from '@playwright/test';

import { canvasPng, createMapRPC, tableForMaps, uploadImageRPC, type MapsTable } from './maps-support';
import { callRPC } from './support';

// Setup for the map printing specs (MR-033, Etapa 8): a campaign with a
// master and a player, a map drawn inside the test with a grid of 30 × 20
// squares (the artboard's "Estrada do Vale"), and one without a grid.

export interface PrintTable extends MapsTable {
  /** 30 × 20 squares on a 1.500 × 1.000 px image. */
  gridMapId: string;
  /** The same image, no grid. */
  plainMapId: string;
}

export async function tableForPrinting(masterPage: Page, playerPage: Page, name: string): Promise<PrintTable> {
  const table = await tableForMaps(masterPage, playerPage, name);
  const image = await uploadImageRPC(masterPage, table.campaignId, 'Estrada do Vale', await canvasPng(masterPage, 1500, 1000, 'Estrada do Vale'));
  const gridMapId = await createMapRPC(masterPage, table.campaignId, 'Estrada do Vale', image);
  const grid = await callRPC(masterPage, 'meurpg.maps.v1.MapService/SetMapGrid', { campaignId: table.campaignId, mapId: gridMapId, columns: 30 });
  expect(grid.ok(), await grid.text()).toBeTruthy();
  const plainMapId = await createMapRPC(masterPage, table.campaignId, 'Sem grade', image);
  return { ...table, gridMapId, plainMapId };
}

export const printRoute = (campaignId: string, mapId: string): string => `/campanhas/${campaignId}/mapas/${mapId}/imprimir`;

/** The summary box's text, e.g. "76,2 × 50,8 cm em 9 folhas A4 ...". */
export function summary(page: Page) {
  return page.getByRole('status').filter({ hasText: /×\s.*cm/ }).first();
}

/** The count of pages in a PDF the browser made (each is a `/Type /Page`). */
export function pdfPages(pdf: Buffer): number {
  return (pdf.toString('latin1').match(/\/Type\s*\/Page(?![s\w])/g) ?? []).length;
}
