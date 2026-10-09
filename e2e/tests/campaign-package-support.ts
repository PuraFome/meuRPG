import { expect, type Page } from '@playwright/test';

import { createMapRPC, uploadImageRPC } from './document-support';
import { addClueRPC } from './notes-support';
import { createPointRPC } from './maps-support';
import { callRPC, characterRpcBody, createCharacterRPC, pensantus } from './support';

// Helpers for the campaign package's tests (MR-050): a small campaign built
// through the API, the export done through the screen, and package files
// crafted inside the test, so no binary fixture lives in the repository.

export interface PackageTable {
  campaignId: string;
  campaignName: string;
  mapName: string;
}

/** A campaign of the master's with one gallery image, one map, one scene
 * point with a clue and one NPC. `page` must be signed in as the master and
 * on an app page (the canvas JPEG needs one). */
export async function tableForPackage(page: Page, campaignName: string): Promise<PackageTable> {
  const created = await callRPC(page, 'meurpg.campaigns.v1.CampaignService/CreateCampaign', {
    name: campaignName,
    xpMode: 'XP_MODE_ENEMIES',
  });
  expect(created.ok()).toBeTruthy();
  const campaignId = (await created.json()).campaign.id as string;
  const imageId = await uploadImageRPC(page, campaignId, 'Taverna do Javali');
  const mapName = 'Mirathel e arredores';
  const mapId = await createMapRPC(page, campaignId, mapName, imageId);
  const pointId = await createPointRPC(page, campaignId, mapId, { kind: 'SCENE', name: 'A estalagem', xBp: 3000, yBp: 4000 });
  await addClueRPC(page, { campaignId, mapId }, pointId, 'Pegadas de botas pesadas levam à adega.');
  const npc = await createCharacterRPC(page, campaignId, characterRpcBody('ENEMY', { ...pensantus, name: 'Capitão Goblin' }));
  expect(npc.ok()).toBeTruthy();
  return { campaignId, campaignName, mapName };
}

/** The player's side: an invite from the master, accepted by the page of the
 * player's own context. */
export async function joinAsPlayer(master: Page, player: Page, campaignId: string): Promise<void> {
  const invite = await callRPC(master, 'meurpg.campaigns.v1.CampaignService/CreateInvite', {
    campaignId,
    maxUses: 1,
    expiresIn: '3600s',
  });
  expect(invite.ok()).toBeTruthy();
  const accepted = await callRPC(player, 'meurpg.campaigns.v1.CampaignService/AcceptInvite', {
    token: (await invite.json()).token,
  });
  expect(accepted.ok()).toBeTruthy();
}

/** Opens the export page and exports: waits for "Último pacote" with its
 * "Baixar de novo" link. */
export async function exportThroughScreen(page: Page, campaignId: string): Promise<void> {
  await page.goto(`/campaigns/${campaignId}/export`);
  await expect(page.getByRole('heading', { level: 2, name: 'Exportar campanha' })).toBeVisible();
  await page.getByRole('button', { name: 'Exportar campanha' }).click();
  const last = page.getByRole('region', { name: 'Último pacote' });
  await expect(last).toBeVisible({ timeout: 60_000 });
  await expect(last.getByRole('link', { name: 'Baixar de novo' })).toBeVisible();
}

const crcTable = Array.from({ length: 256 }, (_, n) => {
  let c = n;
  for (let k = 0; k < 8; k++) {
    c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
  }
  return c >>> 0;
});

function crc32(data: Buffer): number {
  let c = 0xffffffff;
  for (const byte of data) {
    c = crcTable[(c ^ byte) & 0xff] ^ (c >>> 8);
  }
  return (c ^ 0xffffffff) >>> 0;
}

/** A zip with one stored entry, `manifest.json`, whose body is `manifest`.
 * Enough for the server to read the manifest and judge its version. */
export function zipWithManifest(manifest: object): Buffer {
  const name = Buffer.from('manifest.json');
  const data = Buffer.from(JSON.stringify(manifest));
  const crc = crc32(data);

  const local = Buffer.alloc(30);
  local.writeUInt32LE(0x04034b50, 0);
  local.writeUInt16LE(20, 4); // version needed
  local.writeUInt32LE(crc, 14);
  local.writeUInt32LE(data.length, 18); // compressed size (stored)
  local.writeUInt32LE(data.length, 22);
  local.writeUInt16LE(name.length, 26);
  const localPart = Buffer.concat([local, name, data]);

  const central = Buffer.alloc(46);
  central.writeUInt32LE(0x02014b50, 0);
  central.writeUInt16LE(20, 4); // version made by
  central.writeUInt16LE(20, 6); // version needed
  central.writeUInt32LE(crc, 16);
  central.writeUInt32LE(data.length, 20);
  central.writeUInt32LE(data.length, 24);
  central.writeUInt16LE(name.length, 28);
  central.writeUInt32LE(0, 42); // offset of the local header
  const centralPart = Buffer.concat([central, name]);

  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(1, 8);
  end.writeUInt16LE(1, 10);
  end.writeUInt32LE(centralPart.length, 12);
  end.writeUInt32LE(localPart.length, 16);
  return Buffer.concat([localPart, centralPart, end]);
}
