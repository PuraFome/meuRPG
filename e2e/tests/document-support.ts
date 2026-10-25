import { expect, type Page } from '@playwright/test';

import { canvasJpeg } from './gallery-support';
import { callRPC, characterRpcBody, createCharacterRPC, pensantus } from './support';

// Setup for the campaign document's specs (MR-018), through the API: these
// tests prove the document screens, not the gallery, the maps or the
// character forms, which have their own specs.

export interface DocumentTable {
  campaignId: string;
  campaignName: string;
  imageId: string;
  mapId: string;
  npcId: string;
}

/** Uploads a JPEG to the campaign's gallery, the way the page does
 * (`POST /uploads/images`), and returns the image's ID. */
export async function uploadImageRPC(page: Page, campaignId: string, fileName: string, color?: string): Promise<string> {
  const res = await page.request.post('/uploads/images', {
    multipart: {
      campaign_id: campaignId,
      file: { name: `${fileName}.jpg`, mimeType: 'image/jpeg', buffer: await canvasJpeg(page, color) },
    },
  });
  expect(res.status()).toBe(201);
  return (await res.json()).id as string;
}

export async function createMapRPC(page: Page, campaignId: string, name: string, imageId: string): Promise<string> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/CreateMap', { campaignId, name, imageId });
  expect(res.ok()).toBeTruthy();
  return (await res.json()).map.id as string;
}

/** A campaign of Mestre Teste's with a gallery image ("Taverna do Javali"),
 * a map made from it ("Mirathel e arredores") and an enemy NPC ("Capitão
 * Goblin"). `page` must be signed in as the master and on an app page (the
 * canvas JPEG needs one). */
export async function tableWithDocumentParts(page: Page, name: string): Promise<DocumentTable> {
  const created = await callRPC(page, 'meurpg.campaigns.v1.CampaignService/CreateCampaign', {
    name,
    xpMode: 'XP_MODE_ENEMIES',
  });
  expect(created.ok()).toBeTruthy();
  const campaignId = (await created.json()).campaign.id as string;
  const imageId = await uploadImageRPC(page, campaignId, 'Taverna do Javali');
  const mapId = await createMapRPC(page, campaignId, 'Mirathel e arredores', imageId);
  const npc = await createCharacterRPC(page, campaignId, characterRpcBody('ENEMY', { ...pensantus, name: 'Capitão Goblin' }));
  expect(npc.ok()).toBeTruthy();
  return { campaignId, campaignName: name, imageId, mapId, npcId: (await npc.json()).character.id as string };
}

/** Saves a document body through the API; returns the new revision. */
export async function saveDocumentRPC(page: Page, campaignId: string, body: string, expectedRevision: number): Promise<number> {
  const res = await callRPC(page, 'meurpg.campaigns.v1.CampaignDocumentService/UpdateCampaignDocument', {
    campaignId,
    body,
    expectedRevision,
  });
  expect(res.ok()).toBeTruthy();
  return (await res.json()).document.revision as number;
}
