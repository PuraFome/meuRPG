import { expect, type Page } from '@playwright/test';

import { endOpenSessionRPC, openSessionPage, startSessionRPC } from './live-session-support';
import { mapToPaint, type EditorMap } from './editor-support';
import { tableForMaps, type MapsTable } from './maps-support';
import { callRPC } from './support';

// Setup for the puzzle specs (Etapa 10, slice 10.15a: MR-038, RN-27, RN-10; E10-06). The puzzles, their runs and the doors come
// through the API; what is under test is the screens: the master's form, list and live view, and the player's boards. The tests also
// read what a player's response carries, as the app reads it (the JSON of `GetPuzzleRun`), to prove it never has the answer.

const service = 'meurpg.play.v1.PuzzleService';

/** A DoorState as the layer stores it. */
export const DOOR_CLOSED = 2;
export const DOOR_OPEN = 1;

/** The wheel of a lock the artboard uses: four runes, the start a turn away from the solution on two wheels. */
export const LOCK = {
  solution: [1, 0, 0, 0],
  start: [0, 0, 0, 1],
};

export interface PuzzleTable extends MapsTable {
  /** A map of the campaign with a grid of 20 columns and a wall down column 10, with a closed door at (10, 6). */
  map: EditorMap;
  door: { col: number; row: number };
}

/** The squares of the wall, with a gap at the door's row. */
const WALL_ROWS = [3, 4, 5, 7, 8, 9];

/** A campaign of Mestre Teste's with Jogador Teste in it (Pensantus), a map with a closed door and an open session. */
export async function tableForPuzzles(master: Page, player: Page, name: string): Promise<PuzzleTable> {
  const table = await tableForMaps(master, player, name);
  const map = await mapToPaint(master, table.campaignId, 'A capela', 20);
  await paintCells(master, table.campaignId, map.mapId, 'MAP_LAYER_WALL', 1, WALL_ROWS.map((row) => [10, row]));
  await paintCells(master, table.campaignId, map.mapId, 'MAP_LAYER_DOORS', DOOR_CLOSED, [[10, 6]]);
  await startSessionRPC(master, table.campaignId);
  return { ...table, map, door: { col: 10, row: 6 } };
}

export async function paintCells(master: Page, campaignId: string, mapId: string, layer: 'MAP_LAYER_WALL' | 'MAP_LAYER_DOORS', value: number, squares: number[][]): Promise<void> {
  const res = await callRPC(master, 'meurpg.maps.v1.MapService/PaintMapCells', {
    campaignId,
    mapId,
    layer,
    value,
    squares: squares.map(([col, row]) => ({ col, row })),
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** The state of the door at a square, as the master reads the layers. */
export async function doorState(page: Page, campaignId: string, mapId: string, col: number, row: number): Promise<number> {
  const res = await callRPC(page, 'meurpg.maps.v1.MapService/GetMapLayers', { campaignId, mapId });
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  const columns = body.gridColumns as number;
  const doors = Buffer.from(body.doors ?? '', 'base64');
  const n = row * columns + col;
  const byte = doors[n >> 1] ?? 0;
  return n & 1 ? byte >> 4 : byte & 15;
}

type Body = Record<string, unknown>;

async function expectOk(res: Awaited<ReturnType<typeof callRPC>>): Promise<Body> {
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()) as Body;
}

/** "Apagar as luzes" of `size` a side, made through the API; returns its ID. */
export async function createLightsRPC(master: Page, campaignId: string, name: string, size = 5, extra: Body = {}): Promise<string> {
  const body = await expectOk(await callRPC(master, `${service}/CreatePuzzle`, { campaignId, name, config: { lights: { size } }, ...extra }));
  return (body.puzzle as { id: string }).id;
}

/** The combination lock of the artboard, with an optional "Ao resolver". */
export async function createLockRPC(master: Page, campaignId: string, name: string, extra: Body = {}): Promise<string> {
  const body = await expectOk(
    await callRPC(master, `${service}/CreatePuzzle`, {
      campaignId,
      name,
      config: { lock: { wheels: 4, alphabet: 'PUZZLE_ALPHABET_RUNES' } },
      solution: { lock: { wheels: LOCK.solution } },
      start: { lock: { wheels: LOCK.start } },
      ...extra,
    }),
  );
  return (body.puzzle as { id: string }).id;
}

/** The turning symbols: four pillars of four glyphs, each turning its neighbours too. */
export async function createPillarsRPC(master: Page, campaignId: string, name: string, extra: Body = {}): Promise<string> {
  const body = await expectOk(
    await callRPC(master, `${service}/CreatePuzzle`, {
      campaignId,
      name,
      config: { pillars: { pillars: 4, symbols: 4, links: [{ alsoTurns: [1] }, { alsoTurns: [0, 2] }, { alsoTurns: [1, 3] }, { alsoTurns: [2] }] } },
      solution: { pillars: { pillars: [1, 0, 3, 2] } },
      ...extra,
    }),
  );
  return (body.puzzle as { id: string }).id;
}

export async function showPuzzleRPC(master: Page, campaignId: string, puzzleId: string): Promise<void> {
  await expectOk(await callRPC(master, `${service}/ShowPuzzle`, { campaignId, puzzleId }));
}

export async function closePuzzleRPC(master: Page, campaignId: string, puzzleId: string): Promise<void> {
  await expectOk(await callRPC(master, `${service}/ClosePuzzle`, { campaignId, puzzleId }));
}

/**
 * Solves a puzzle the way the server says is shortest: the master's own read carries the moves (`minimum.path`), and each one is made as
 * the player. Only for a puzzle whose start is generated (the lights and the pillars).
 */
export async function solveByThePathRPC(master: Page, player: Page, campaignId: string, puzzleId: string): Promise<void> {
  const run = (await masterRunJson(master, campaignId, puzzleId)).run as { minimum?: { path?: Body[] } };
  for (const move of run.minimum?.path ?? []) {
    await moveRPC(player, campaignId, puzzleId, move);
  }
}

/** The raw JSON of `GetPuzzleRun` for a player: exactly what the app reads, to look for what must never be there. */
export async function playerRunText(player: Page, campaignId: string, puzzleId: string): Promise<string> {
  const res = await callRPC(player, `${service}/GetPuzzleRun`, { campaignId, puzzleId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.text();
}

export async function playerRunJson(player: Page, campaignId: string, puzzleId: string): Promise<Body> {
  return JSON.parse(await playerRunText(player, campaignId, puzzleId)) as Body;
}

/** One move through the API, as the player; the key is new each time. */
export async function moveRPC(player: Page, campaignId: string, puzzleId: string, move: Body): Promise<Body> {
  return expectOk(await callRPC(player, `${service}/MakePuzzleMove`, { campaignId, puzzleId, move, idempotencyKey: crypto.randomUUID() }));
}

/** The master's own live read of a puzzle. */
export async function masterRunJson(master: Page, campaignId: string, puzzleId: string): Promise<Body> {
  return expectOk(await callRPC(master, `${service}/GetMasterPuzzleRun`, { campaignId, puzzleId }));
}

/** Ends the session the table opened, so the shared accounts do not pile up open sessions. */
export async function endTable(master: Page, campaignId: string): Promise<void> {
  await endOpenSessionRPC(master, campaignId);
}

/** The master's session page, ready: the "Quebra-cabeças" panel is there. */
export async function openMasterSession(master: Page, campaignId: string): Promise<void> {
  await openSessionPage(master, campaignId);
}

export function puzzleRoute(campaignId: string, ...rest: string[]): string {
  return ['', 'campanhas', campaignId, ...rest].join('/');
}
