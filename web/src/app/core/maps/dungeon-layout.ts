import {
  type DungeonDoor,
  DungeonAxis,
  DungeonDoorKind,
  type DungeonExit,
  type DungeonRoom,
  DungeonSide,
  type DungeonStair,
} from '../../../gen/meurpg/maps/v1/dungeons_pb';
import { SQUARE_FT, metersNumber } from '../units';
import { tight } from '../format/text';
import type { DoorKind, DoorSquare } from './layers';

/**
 * Drawing helpers for the generated dungeon (E10-05 1 and 5). The browser draws what the server sends: nothing here generates or
 * decides anything about the dungeon; it only turns the server's compact layout into marks and words.
 */

/** The `DoorState` a generator door kind is drawn as (`DungeonDoorKind` in dungeons.proto); a passage is floor, so it has none. */
export function layerKindOf(kind: DungeonDoorKind): DoorKind | null {
  switch (kind) {
    case DungeonDoorKind.CLOSED:
      return 2;
    case DungeonDoorKind.LOCKED:
      return 3;
    case DungeonDoorKind.BARRED:
      return 4;
    case DungeonDoorKind.SECRET:
      return 5;
    default:
      return null;
  }
}

/** The doors a layout draws: every door but the passages, with the axis of its wall (`h`: a wall that runs east to west). */
export function doorSquaresOf(doors: readonly DungeonDoor[]): DoorSquare[] {
  const out: DoorSquare[] = [];
  for (const d of doors) {
    const state = layerKindOf(d.kind);
    if (state !== null) {
      out.push({ col: d.x, row: d.y, state, axis: d.axis === DungeonAxis.VERTICAL_WALL ? 'v' : 'h' });
    }
  }
  return out;
}

/** "13 portas e 8 passagens": the passages are floor, so they are counted apart (MAP-LANGUAGE-E10.md). */
export function doorCountText(doors: readonly DungeonDoor[]): string {
  const passages = doors.filter((d) => d.kind === DungeonDoorKind.ARCHWAY).length;
  const real = doors.length - passages;
  return `${real} ${real === 1 ? 'porta' : 'portas'} e ${passages} ${passages === 1 ? 'passagem' : 'passagens'}`;
}

/** "9 salas", "1 sala". */
export function roomCountText(count: number): string {
  return `${count} ${count === 1 ? 'sala' : 'salas'}`;
}

/** "2 escadas", "1 escada", "Nenhuma escada". */
export function stairCountText(count: number): string {
  return count === 0 ? 'nenhuma escada' : `${count} ${count === 1 ? 'escada' : 'escadas'}`;
}

/**
 * The path of the wall squares of a preview: `open` is one bit a square (set where a creature can stand), the layout of the walls layer,
 * row-major with square n at bit n % 8 of byte n / 8. Runs of wall on a row are one rectangle each, so a 121 × 121 dungeon is a short
 * string, not thousands of elements. The path is in squares (a viewBox of `width` × `height`).
 */
export function wallPath(open: Uint8Array, width: number, height: number): string {
  const parts: string[] = [];
  for (let row = 0; row < height; row++) {
    let start = -1;
    for (let col = 0; col <= width; col++) {
      const wall = col < width && !isOpen(open, row * width + col);
      if (wall && start < 0) {
        start = col;
      } else if (!wall && start >= 0) {
        parts.push(`M${start} ${row}h${col - start}v1h${start - col}z`);
        start = -1;
      }
    }
  }
  return parts.join('');
}

function isOpen(open: Uint8Array, n: number): boolean {
  const byte = open[n >> 3];
  return byte !== undefined && ((byte >> (n & 7)) & 1) === 1;
}

/** A room's size in meters, "10,5 × 7,5 m" (a square is 1,5 m). */
export function roomSizeText(floor: { width: number; height: number }): string {
  return tight(`${metersNumber(floor.width * SQUARE_FT)} × ${metersNumber(floor.height * SQUARE_FT)} m`);
}

/** A dungeon's size in meters: "46,5 × 31,5 m". */
export function dungeonSizeText(width: number, height: number): string {
  return roomSizeText({ width, height });
}

export const DOOR_KIND_TEXT: Readonly<Record<number, string>> = {
  [DungeonDoorKind.ARCHWAY]: 'Passagem',
  [DungeonDoorKind.CLOSED]: 'Porta fechada',
  [DungeonDoorKind.BARRED]: 'Grade',
  [DungeonDoorKind.LOCKED]: 'Porta trancada',
  [DungeonDoorKind.SECRET]: 'Porta secreta',
};

const SIDE_TEXT: Readonly<Record<number, string>> = {
  [DungeonSide.NORTH]: 'norte',
  [DungeonSide.EAST]: 'leste',
  [DungeonSide.SOUTH]: 'sul',
  [DungeonSide.WEST]: 'oeste',
};

/** "Porta trancada ao norte, para a sala 3" (the true kind: this list is the master's). */
export function exitText(exit: DungeonExit): string {
  const kind = DOOR_KIND_TEXT[exit.kind] ?? 'Porta';
  const side = SIDE_TEXT[exit.side];
  const where = exit.otherRoomId > 0 ? `, para a sala ${exit.otherRoomId}` : ', para um corredor';
  return `${kind}${side ? ` ao ${side}` : ''}${where}`;
}

/** What only the rooms list says of a door, besides its kind: the generator put a trap on it (the map draws a plain door). */
export const TRAP_NOTE = 'Porta com armadilha';

/** A room behind a secret door only: it has doors, and every one is secret (the master finds it on the list, the players do not). */
export function behindSecretDoor(room: DungeonRoom): boolean {
  return room.exits.length > 0 && room.exits.every((e) => e.kind === DungeonDoorKind.SECRET);
}

/** The stairs that stand outside every room (at the end of a corridor): the list names them apart, and the entrance among them. */
export function stairsOutsideRooms(rooms: readonly DungeonRoom[], stairs: readonly DungeonStair[]): DungeonStair[] {
  return stairs.filter((s) => !rooms.some((r) => stairsInRoom(r, [s]).length > 0));
}

/** Which stairs stand on a room's floor ("Escada para cima", "Escada para baixo"). */
export function stairsInRoom(room: DungeonRoom, stairs: readonly DungeonStair[]): DungeonStair[] {
  const f = room.floor;
  if (!f) {
    return [];
  }
  return stairs.filter((s) => s.x >= f.x && s.x < f.x + f.width && s.y >= f.y && s.y < f.y + f.height);
}
