import { create } from '@bufbuild/protobuf';

import { DungeonAxis, DungeonDoorKind, DungeonDoorSchema, DungeonSide } from '../../../gen/meurpg/maps/v1/dungeons_pb';
import {
  behindSecretDoor,
  doorCountText,
  doorSquaresOf,
  exitText,
  layerKindOf,
  roomSizeText,
  stairsInRoom,
  stairsOutsideRooms,
  wallPath,
} from './dungeon-layout';
import { SAMPLE_HEIGHT, SAMPLE_WIDTH, openBits, previewResponse, room, roomsResponse } from './dungeons-testing';

const door = (kind: DungeonDoorKind, axis = DungeonAxis.HORIZONTAL_WALL) => create(DungeonDoorSchema, { x: 1, y: 2, kind, axis });

describe('the dungeon layout helpers', () => {
  it('draws a generator door as the layer kind it is: a passage is floor, a trap is no kind of its own', () => {
    expect(layerKindOf(DungeonDoorKind.ARCHWAY)).toBeNull();
    expect(layerKindOf(DungeonDoorKind.CLOSED)).toBe(2);
    expect(layerKindOf(DungeonDoorKind.LOCKED)).toBe(3);
    expect(layerKindOf(DungeonDoorKind.BARRED)).toBe(4);
    expect(layerKindOf(DungeonDoorKind.SECRET)).toBe(5);
    expect(layerKindOf(DungeonDoorKind.UNSPECIFIED)).toBeNull();
  });

  it('keeps every door but the passages, with the axis of its wall', () => {
    const squares = doorSquaresOf([door(DungeonDoorKind.ARCHWAY), door(DungeonDoorKind.LOCKED, DungeonAxis.VERTICAL_WALL), door(DungeonDoorKind.CLOSED)]);
    expect(squares).toEqual([
      { col: 1, row: 2, state: 3, axis: 'v' },
      { col: 1, row: 2, state: 2, axis: 'h' },
    ]);
  });

  it('counts the doors and the passages apart ("13 portas e 8 passagens"), with the singular', () => {
    const p = previewResponse();
    expect(doorCountText(p.doors)).toBe('4 portas e 1 passagem');
    expect(doorCountText([door(DungeonDoorKind.CLOSED), door(DungeonDoorKind.ARCHWAY), door(DungeonDoorKind.ARCHWAY)])).toBe('1 porta e 2 passagens');
  });

  it('turns the open bits into a path of the wall squares, one rectangle for each run', () => {
    // A 3 × 2 grid: row 0 all wall, row 1 wall, open, wall.
    const open = openBits(3, 2, [[1, 1, 1, 1]]);
    expect(wallPath(open, 3, 2)).toBe('M0 0h3v1h-3zM0 1h1v1h-1zM2 1h1v1h-1z');
  });

  it('draws the sample dungeon without a wall on a floor square', () => {
    const p = previewResponse();
    const path = wallPath(p.open, p.width, p.height);
    expect(path.startsWith('M0 0h11v1h-11z')).toBe(true);
    // Row 1 is wall at x 0, 4 and 10 only (the two rooms' rows).
    expect(path).toContain('M0 1h1v1h-1zM4 1h1v1h-1zM10 1h1v1h-1z');
    expect(SAMPLE_WIDTH * SAMPLE_HEIGHT).toBe(99);
  });

  it('says a room size in meters (a square is 1,5 m)', () => {
    // The number and its unit stay on one line (no-break spaces).
    const plain = (t: string) => t.replace(/\u00a0/g, ' ');
    expect(plain(roomSizeText({ width: 7, height: 5 }))).toBe('10,5 × 7,5 m');
    expect(plain(roomSizeText({ width: 3, height: 3 }))).toBe('4,5 × 4,5 m');
    expect(roomSizeText({ width: 7, height: 5 })).toContain('7,5\u00a0m');
  });

  it('says an exit by its true kind, its side and what is beyond', () => {
    const [locked, passage] = roomsResponse().rooms[0]!.exits;
    expect(exitText(locked!)).toBe('Porta trancada ao sul, para um corredor');
    expect(exitText(passage!)).toBe('Passagem ao leste, para a sala 2');
    expect(locked!.trapped).toBe(true);
  });

  it('finds a room behind a secret door only', () => {
    const rooms = roomsResponse().rooms;
    expect(behindSecretDoor(rooms[0]!)).toBe(false);
    expect(behindSecretDoor(rooms[1]!)).toBe(true);
    expect(behindSecretDoor(room(9))).toBe(false);
  });

  it('finds the stairs that stand on a room', () => {
    const info = roomsResponse();
    expect(stairsInRoom(info.rooms[0]!, info.stairs).map((s) => s.up)).toEqual([true]);
    expect(stairsInRoom(info.rooms[1]!, info.stairs).map((s) => s.up)).toEqual([false]);
    expect(stairsInRoom(info.rooms[2]!, info.stairs)).toEqual([]);
    expect(DungeonSide.NORTH).toBe(1);
  });

  it('finds the stairs outside every room (at the end of a corridor)', () => {
    const info = roomsResponse();
    expect(stairsOutsideRooms(info.rooms, info.stairs)).toEqual([]);
    expect(stairsOutsideRooms([info.rooms[1]!], info.stairs).map((s) => s.up)).toEqual([true]);
  });
});
