import { expect, type Page } from '@playwright/test';

import { setGridRPC, toren } from './combat-support';
import { startSessionRPC } from './live-session-support';
import { createMapRPC, placeTokenRPC, revealMapRPC, setCurrentMapRPC, uploadImageRPC } from './maps-support';
import { callRPC, characterRpcBody, createCharacterRPC, pensantus } from './support';

// Setup for the fog-of-war specs (Etapa 9, MR-036, RN-10, RN-20), through the API:
// the cave "A caverna do Vale Seco" of the design (design/etapa9/cave.py), 24 x 16
// squares, with Pensantus (a Rock Gnome: darkvision 18 m) and Toren (a human: none)
// at the table and the three goblins in the guard room. Every test makes its own
// campaign. The tests read the shading and the tokens, never the pixels.

/** The cave, row by row: # a wall, . floor, : rubble (terreno difícil), h the crates (meia cobertura), q the column (três quartos). */
export const CAVE: readonly string[] = [
  '########################',
  '########################',
  '################.......#',
  '################.......#',
  '################....q..#',
  '################.......#',
  '#......#########.......#',
  '...................h...#',
  '...................h...#',
  '#...::.#..######.......#',
  '#...::.#..##############',
  '######........##########',
  '######........##########',
  '######........##########',
  '######........##########',
  '########################',
];
export const CAVE_COLUMNS = 24;
export const CAVE_ROWS = 16;

/** The middle of a square, in basis points of the image. */
export function squareBp(col: number, row: number): { xBp: number; yBp: number } {
  return { xBp: Math.round(((col + 0.5) / CAVE_COLUMNS) * 10000), yBp: Math.round(((row + 0.5) / CAVE_ROWS) * 10000) };
}

/** The cave's picture, drawn by the browser on a canvas (960 x 640: 40 px a square), so no binary fixture lives in the repository. */
export async function caveImage(page: Page): Promise<Buffer> {
  const base64 = await page.evaluate((rows) => {
    const canvas = document.createElement('canvas');
    canvas.width = 960;
    canvas.height = 640;
    const ctx = canvas.getContext('2d')!;
    ctx.fillStyle = '#2e2a26';
    ctx.fillRect(0, 0, 960, 640);
    rows.forEach((row, r) => {
      [...row].forEach((ch, c) => {
        if (ch === '#') {
          return;
        }
        ctx.fillStyle = ch === ':' ? '#7d6e55' : '#a8977a';
        ctx.fillRect(c * 40, r * 40, 40, 40);
        if (ch === 'h' || ch === 'q') {
          ctx.fillStyle = '#5b4834';
          ctx.fillRect(c * 40 + 8, r * 40 + 8, 24, 24);
        }
      });
    });
    return canvas.toDataURL('image/png').split(',')[1];
  }, [...CAVE]);
  return Buffer.from(base64, 'base64');
}

export interface FogTable {
  campaignId: string;
  campaignName: string;
  sessionId: string;
  mapId: string;
  pensantusId: string;
  torenId: string;
  goblin1Id: string;
  goblin2Id: string;
  captainId: string;
  /** Nanquim, Pensantus's raven (a familiar), when asked for. */
  familiarId?: string;
}

async function join(master: Page, player: Page, campaignId: string): Promise<void> {
  const invite = await callRPC(master, 'meurpg.campaigns.v1.CampaignService/CreateInvite', { campaignId, maxUses: 1, expiresIn: '3600s' });
  expect(invite.ok()).toBeTruthy();
  const accepted = await callRPC(player, 'meurpg.campaigns.v1.CampaignService/AcceptInvite', { token: (await invite.json()).token });
  expect(accepted.ok()).toBeTruthy();
}

async function npc(master: Page, campaignId: string, name: string, kind: 'MINION' | 'ENEMY'): Promise<string> {
  // An enemy has a full sheet (as the combat specs make the Capitão); a minion, a basic one.
  const body =
    kind === 'ENEMY'
      ? characterRpcBody('ENEMY', { ...pensantus, name })
      : {
          kind: 'CHARACTER_KIND_MINION',
          name,
          sheet: { basic: { hitPointsMax: 7, armorClass: 15, speedFt: 30, attackBonus: 4, damage: '1d6+2', description: '' } },
        };
  const res = await createCharacterRPC(master, campaignId, body);
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).character.id as string;
}

async function paint(master: Page, campaignId: string, mapId: string, layer: 'DIFFICULT_TERRAIN' | 'WALL' | 'COVER', value: number, squares: { col: number; row: number }[]): Promise<void> {
  if (squares.length === 0) {
    return;
  }
  const res = await callRPC(master, 'meurpg.maps.v1.MapService/PaintMapCells', { campaignId, mapId, layer: `MAP_LAYER_${layer}`, value, squares });
  expect(res.ok(), await res.text()).toBeTruthy();
}

function squaresOf(chars: string): { col: number; row: number }[] {
  const out: { col: number; row: number }[] = [];
  CAVE.forEach((row, r) => [...row].forEach((ch, c) => chars.includes(ch) && out.push({ col: c, row: r })));
  return out;
}

export interface FogOptions {
  /** Pensantus's square (the corridor mouth by default); Toren's. */
  pensantusAt?: { col: number; row: number };
  torenAt?: { col: number; row: number };
  /** Toren carries a torch from the start. */
  torch?: boolean;
  /** Leave the map fog off (a plain map: the session must stay as it was). */
  noFog?: boolean;
  /** Toren has no token on the map (his player is "fora do mapa"). */
  torenOffMap?: boolean;
  /** Pensantus has Nanquim, a raven familiar (Encontrar Familiar, a ritual), on this square. */
  familiar?: { col: number; row: number };
}

/**
 * The cave at the table: a campaign with Pensantus (`pensantusPlayer`'s) and Toren
 * (`torenPlayer`'s), the Goblin 1, Goblin 2 and the Capitão Goblin, an open
 * session, the cave as the current map with its grid, walls, rubble, cover and a
 * torch in the guard room, the fog on with the base light dark, and everyone's
 * token placed. The master's and the two players' pages must be signed in as
 * each; none navigates.
 */
export async function tableForFog(master: Page, pensantusPlayer: Page, torenPlayer: Page, name: string, options: FogOptions = {}): Promise<FogTable> {
  const created = await callRPC(master, 'meurpg.campaigns.v1.CampaignService/CreateCampaign', { name, xpMode: 'XP_MODE_ENEMIES' });
  expect(created.ok()).toBeTruthy();
  const campaignId = (await created.json()).campaign.id as string;

  await join(master, pensantusPlayer, campaignId);
  const pensantusBody = characterRpcBody('PLAYER', pensantus) as { sheet: { full: object } };
  if (options.familiar) {
    // The spell is in the book and prepared: Encontrar Familiar is a ritual, so it costs no slot.
    pensantusBody.sheet.full = { ...pensantusBody.sheet.full, knownSpellKeys: ['spell:find-familiar'], preparedSpellKeys: ['spell:find-familiar'] };
  }
  const pensantusRes = await createCharacterRPC(pensantusPlayer, campaignId, pensantusBody);
  expect(pensantusRes.ok(), await pensantusRes.text()).toBeTruthy();
  await join(master, torenPlayer, campaignId);
  const torenRes = await createCharacterRPC(torenPlayer, campaignId, characterRpcBody('PLAYER', toren));
  expect(torenRes.ok(), await torenRes.text()).toBeTruthy();
  const pensantusId = (await pensantusRes.json()).character.id as string;
  const torenId = (await torenRes.json()).character.id as string;

  const goblin1Id = await npc(master, campaignId, 'Goblin 1', 'MINION');
  const goblin2Id = await npc(master, campaignId, 'Goblin 2', 'MINION');
  const captainId = await npc(master, campaignId, 'Capitão Goblin', 'ENEMY');

  const sessionId = await startSessionRPC(master, campaignId);
  await master.goto('/');
  const image = await uploadImageRPC(master, campaignId, 'A caverna do Vale Seco', await caveImage(master));
  const mapId = await createMapRPC(master, campaignId, 'A caverna do Vale Seco', image);
  await revealMapRPC(master, campaignId, mapId);
  await setCurrentMapRPC(master, campaignId, mapId);
  await setGridRPC(master, campaignId, mapId, CAVE_COLUMNS);
  await paint(master, campaignId, mapId, 'WALL', 1, squaresOf('#'));
  await paint(master, campaignId, mapId, 'DIFFICULT_TERRAIN', 1, squaresOf(':'));
  await paint(master, campaignId, mapId, 'COVER', 1, squaresOf('h'));
  await paint(master, campaignId, mapId, 'COVER', 2, squaresOf('q'));

  const torch = squareBp(19, 4);
  const light = await callRPC(master, 'meurpg.maps.v1.MapService/CreateMapPoint', {
    campaignId,
    mapId,
    kind: 'MAP_POINT_KIND_LIGHT',
    name: 'Tocha da guarita',
    ...torch,
    light: { presetKey: 'light:torch', brightFt: 20, dimFt: 20 },
  });
  expect(light.ok(), await light.text()).toBeTruthy();

  const place = (id: string, col: number, row: number) => {
    const { xBp, yBp } = squareBp(col, row);
    return placeTokenRPC(master, campaignId, mapId, id, xBp, yBp);
  };
  const pensantusAt = options.pensantusAt ?? { col: 5, row: 8 };
  await place(pensantusId, pensantusAt.col, pensantusAt.row);
  if (!options.torenOffMap) {
    const torenAt = options.torenAt ?? { col: 6, row: 7 };
    await place(torenId, torenAt.col, torenAt.row);
  }
  await place(goblin1Id, 18, 5);
  await place(goblin2Id, 20, 7);
  await place(captainId, 21, 3);
  // The master places an NPC hidden; the table shows them (the fog decides who sees them).
  for (const id of [goblin1Id, goblin2Id, captainId]) {
    const shown = await callRPC(master, 'meurpg.maps.v1.MapService/SetMapTokenHidden', { campaignId, mapId, characterId: id, hidden: false });
    expect(shown.ok(), await shown.text()).toBeTruthy();
  }

  if (!options.noFog) {
    const fog = await callRPC(master, 'meurpg.maps.v1.MapService/SetMapFog', { campaignId, mapId, fogEnabled: true, baseLight: 'LIGHT_LEVEL_DARK' });
    expect(fog.ok(), await fog.text()).toBeTruthy();
  }
  let familiarId: string | undefined;
  if (options.familiar) {
    const cast = await callRPC(pensantusPlayer, 'meurpg.play.v1.PlayService/CastSummon', {
      campaignId,
      characterId: pensantusId,
      spellKey: 'spell:find-familiar',
      ritual: true,
      summon: { option: 0, creatureKeys: ['monster:raven'], names: ['Nanquim'] },
      idempotencyKey: crypto.randomUUID(),
    });
    expect(cast.ok(), await cast.text()).toBeTruthy();
    familiarId = (await cast.json()).creatureIds[0] as string;
    const { xBp, yBp } = squareBp(options.familiar.col, options.familiar.row);
    const placed = await callRPC(master, 'meurpg.maps.v1.MapService/PlaceMapToken', { campaignId, mapId, creatureId: familiarId, xBp, yBp });
    expect(placed.ok(), await placed.text()).toBeTruthy();
  }
  if (options.torch) {
    await setCarriedLightRPC(master, campaignId, mapId, torenId, 'light:torch');
  }
  return { campaignId, campaignName: name, sessionId, mapId, pensantusId, torenId, goblin1Id, goblin2Id, captainId, familiarId };
}

export async function setCarriedLightRPC(page: Page, campaignId: string, mapId: string, characterId: string, lightKey: string): Promise<void> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/SetCarriedLight', { campaignId, mapId, characterId, lightKey });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** `GetMapVision` as the page's own session would call it: what the caller sees, unpacked to one state a square. */
export async function visionOf(page: Page, table: FogTable, asCharacterId = ''): Promise<{ states: number[]; revision: number; characterOnMap: boolean; tiles: number }> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/GetMapVision', { campaignId: table.campaignId, mapId: table.mapId, asCharacterId });
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  const bytes = Buffer.from(body.states ?? '', 'base64');
  const states: number[] = [];
  for (let n = 0; n < CAVE_COLUMNS * CAVE_ROWS; n++) {
    const byte = bytes[n >> 1] ?? 0;
    states.push(n & 1 ? byte >> 4 : byte & 0x0f);
  }
  return { states, revision: body.revision ?? 0, characterOnMap: body.characterOnMap ?? false, tiles: (body.tiles ?? []).length };
}

/** Moves a token (the master's call), by square. */
export async function moveTo(master: Page, table: FogTable, characterId: string, col: number, row: number): Promise<void> {
  const { xBp, yBp } = squareBp(col, row);
  await placeTokenRPC(master, table.campaignId, table.mapId, characterId, xBp, yBp);
}

/** The session page's route. */
export function sessionRoute(campaignId: string): string {
  return `/campanhas/${campaignId}/sessao`;
}
