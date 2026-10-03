import { expect, type Page } from '@playwright/test';

import { callRPC, characterRpcBody, createCharacterRPC, pensantus, type CharacterBuild } from './support';

// Setup for the maps and shown-image specs (Etapa 5, MR-008, MR-009, MR-012,
// MR-028), through the API: these tests prove the map screens and the
// session's map and image, not the campaign, invite, character and upload
// forms other specs already cover. Images are drawn inside the test (a
// canvas PNG), so no binary fixture lives in the repository.

/**
 * A PNG drawn by the browser on a canvas: a parchment field with a coloured
 * band and `label` written on it, so two images look different on screen.
 * Any page of the app works: the canvas never leaves the page.
 */
export async function canvasPng(page: Page, width: number, height: number, label: string, band = '#8a5a2b'): Promise<Buffer> {
  const base64 = await page.evaluate(
    ({ width, height, label, band }) => {
      const canvas = document.createElement('canvas');
      canvas.width = width;
      canvas.height = height;
      const ctx = canvas.getContext('2d')!;
      ctx.fillStyle = '#e8ddc2';
      ctx.fillRect(0, 0, width, height);
      ctx.fillStyle = band;
      ctx.fillRect(0, height * 0.4, width, height * 0.2);
      ctx.strokeStyle = '#5b4834';
      ctx.lineWidth = 6;
      ctx.strokeRect(12, 12, width - 24, height - 24);
      ctx.fillStyle = '#1b2230';
      ctx.font = `bold ${Math.round(height / 12)}px serif`;
      ctx.textAlign = 'center';
      ctx.fillText(label, width / 2, height * 0.5 + height / 36);
      return canvas.toDataURL('image/png').split(',')[1];
    },
    { width, height, label, band },
  );
  return Buffer.from(base64, 'base64');
}

/** Uploads an image to the campaign's gallery the way the browser does
 * (`POST /uploads/images`, `campaign_id` first); returns its ID. The name
 * the gallery shows is the file's name without the extension. */
export async function uploadImageRPC(page: Page, campaignId: string, name: string, png: Buffer): Promise<string> {
  const res = await page.request.post('/uploads/images', {
    multipart: {
      campaign_id: campaignId,
      file: { name: `${name}.png`, mimeType: 'image/png', buffer: png },
    },
  });
  expect(res.status(), await res.text()).toBe(201);
  return (await res.json()).id as string;
}

/** A campaign of Mestre Teste's with Jogador Teste in it, playing Pensantus,
 * and (with `npc`) a goblin the master can put on a map. */
export interface MapsTable {
  campaignId: string;
  campaignName: string;
  /** Pensantus, the player's character. */
  characterId: string;
  /** A goblin NPC (a minion), when asked for. */
  npcId?: string;
}

/** `sheet` adds to the player's full sheet (the combat specs give Pensantus a
 * dagger, Raio de Fogo and spells: `weaponKeys`, `cantripKeys`...); `build` is
 * another character than Pensantus (the slice 6.5c specs play a fighter and a cleric). */
export async function tableForMaps(
  masterPage: Page,
  playerPage: Page,
  name: string,
  npc = false,
  sheet: Record<string, unknown> = {},
  build: CharacterBuild = pensantus,
): Promise<MapsTable> {
  const created = await callRPC(masterPage, 'meurpg.campaigns.v1.CampaignService/CreateCampaign', {
    name,
    xpMode: 'XP_MODE_ENEMIES',
  });
  expect(created.ok()).toBeTruthy();
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

  const body = characterRpcBody('PLAYER', build) as { sheet: { full: object } };
  body.sheet.full = { ...body.sheet.full, ...sheet };
  const character = await createCharacterRPC(playerPage, campaignId, body);
  expect(character.ok()).toBeTruthy();
  const table: MapsTable = {
    campaignId,
    campaignName: name,
    characterId: (await character.json()).character.id,
  };

  if (npc) {
    const goblin = await createCharacterRPC(masterPage, campaignId, {
      kind: 'CHARACTER_KIND_MINION',
      name: 'Goblin',
      sheet: {
        basic: {
          hitPointsMax: 7,
          armorClass: 15,
          speedFt: 30,
          attackBonus: 4,
          damage: '1d6+2',
          description: '',
          // The combat specs need an attack to roll: "basic:0".
          attacks: [{ name: 'Cimitarra', attackBonus: 4, damageDiceCount: 1, damageDiceSides: 6, damageBonus: 2, damageType: 'DAMAGE_TYPE_SLASHING', rangeFt: 5 }],
        },
      },
    });
    expect(goblin.ok()).toBeTruthy();
    table.npcId = (await goblin.json()).character.id;
  }
  return table;
}

export async function createMapRPC(page: Page, campaignId: string, name: string, imageId: string): Promise<string> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/CreateMap', { campaignId, name, imageId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).map.id as string;
}

export async function revealMapRPC(page: Page, campaignId: string, mapId: string, revealed = true): Promise<void> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/SetMapRevealed', { campaignId, mapId, revealed });
  expect(res.ok(), await res.text()).toBeTruthy();
}

export interface PointSpec {
  kind: 'BATTLE' | 'SUBMAP' | 'SCENE';
  name: string;
  description?: string;
  xBp: number;
  yBp: number;
  targetMapId?: string;
  revealed?: boolean;
}

/** Creates a point (born hidden) and reveals it when `revealed`; returns its ID. */
export async function createPointRPC(page: Page, campaignId: string, mapId: string, point: PointSpec): Promise<string> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/CreateMapPoint', {
    campaignId,
    mapId,
    kind: `MAP_POINT_KIND_${point.kind}`,
    name: point.name,
    description: point.description ?? '',
    xBp: point.xBp,
    yBp: point.yBp,
    targetMapId: point.targetMapId ?? '',
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  const id = (await res.json()).point.id as string;
  if (point.revealed) {
    const shown = await callRPC(page, 'meurpg.maps.v1.MapService/SetMapPointRevealed', { campaignId, mapId, pointId: id, revealed: true });
    expect(shown.ok(), await shown.text()).toBeTruthy();
  }
  return id;
}

export async function placeTokenRPC(page: Page, campaignId: string, mapId: string, characterId: string, xBp: number, yBp: number): Promise<void> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/PlaceMapToken', { campaignId, mapId, characterId, xBp, yBp });
  expect(res.ok(), await res.text()).toBeTruthy();
}

export async function setCurrentMapRPC(page: Page, campaignId: string, mapId: string): Promise<void> {
  const res = await callRPC(page, 'meurpg.play.v1.PlayService/SetCurrentMap', { campaignId, mapId });
  expect(res.ok(), await res.text()).toBeTruthy();
}
