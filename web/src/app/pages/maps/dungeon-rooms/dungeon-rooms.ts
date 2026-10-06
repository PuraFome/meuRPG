import { ChangeDetectionStrategy, Component, computed, inject, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { DungeonRoom, GetDungeonRoomsResponse } from '../../../../gen/meurpg/maps/v1/dungeons_pb';
import { placeSceneFailure } from '../../../core/maps/dungeon-errors';
import { TRAP_NOTE, behindSecretDoor, exitText, roomSizeText, stairsInRoom, stairsOutsideRooms } from '../../../core/maps/dungeon-layout';
import { DungeonsClient } from '../../../core/maps/dungeons-client';
import type { MapState } from '../../../core/maps/map-state';
import { StairMark } from '../../../shared/map-layers/stair-mark';

/** A room on the map: where its floor is, in squares, for the outline the editor draws. */
export interface RoomOutline {
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
}

/**
 * The master's list of a generated dungeon's rooms, beside the map (E10-05 5): the room's number and size, the stairs on its floor, its
 * exits with their true door kinds, the doors that have a trap ("Porta com armadilha: …", the map draws them as plain doors), a room
 * behind a secret door, and "Pôr uma cena nesta sala". Only the master gets this list (the server answers a player with `not_found`;
 * the page that hosts it never asks as a player), and it says so.
 *
 * Choosing a room outlines it on the map (`outline`, a solid 3 px accent frame in the editor); choosing it again lets go. "Pôr uma cena nesta
 * sala" makes the hidden scene point "Sala N" in the middle of the room (`PlaceDungeonScene`) and shows it on the map at once; it can be
 * repeated, each call makes a point.
 */
@Component({
  selector: 'app-dungeon-rooms',
  imports: [MatButtonModule, MatIconModule, StairMark],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './dungeon-rooms.html',
  styleUrl: './dungeon-rooms.scss',
})
export class DungeonRooms {
  private readonly api = inject(DungeonsClient);

  readonly campaignId = input.required<string>();
  readonly mapId = input.required<string>();
  readonly info = input.required<GetDungeonRoomsResponse>();
  /** The page's map state: the new scene point goes in it, so the map shows it at once. */
  readonly state = input.required<MapState>();
  /** The rooms list changed on the server (a scene was put): read it again. */
  readonly changed = output<void>();
  /** The room that is outlined on the map, or `null`. */
  readonly outline = output<RoomOutline | null>();

  protected readonly selected = signal<number | null>(null);
  protected readonly busy = signal<number | null>(null);
  protected readonly message = signal('');
  protected readonly problem = signal<{ room: number; text: string } | null>(null);

  protected readonly rooms = computed(() => this.info().rooms);
  protected readonly entranceRoom = computed(() => {
    const e = this.info().entrance;
    return e?.onStairs ? (this.info().rooms.find((r) => stairsInRoom(r, this.info().stairs).some((s) => s.x === e.x && s.y === e.y))?.id ?? null) : null;
  });

  protected size(room: DungeonRoom): string {
    return room.floor ? roomSizeText(room.floor) : '';
  }

  protected stairs(room: DungeonRoom) {
    return stairsInRoom(room, this.info().stairs);
  }

  /** Each exit with its true kind and, once, the trap note when the door has a trap (the map draws a plain door). */
  protected exits(room: DungeonRoom): { text: string; trapped: boolean }[] {
    return room.exits.map((e) => ({ text: exitText(e), trapped: e.trapped }));
  }

  protected readonly trapNote = TRAP_NOTE;

  /** The stairs outside every room, at the end of a corridor; the entrance is named. */
  protected readonly corridorStairs = computed(() =>
    stairsOutsideRooms(this.info().rooms, this.info().stairs).map((s) => {
      const e = this.info().entrance;
      return { up: s.up, entrance: !!e?.onStairs && e.x === s.x && e.y === s.y, col: s.x + 1, row: s.y + 1 };
    }),
  );

  protected secret(room: DungeonRoom): boolean {
    return behindSecretDoor(room);
  }

  protected scenes(room: DungeonRoom): string {
    const n = room.scenePointIds.length;
    return n === 0 ? '' : `${n} ${n === 1 ? 'cena' : 'cenas'} nesta sala`;
  }

  protected toggle(room: DungeonRoom): void {
    const next = this.selected() === room.id ? null : room.id;
    this.selected.set(next);
    this.outline.emit(next === null || !room.floor ? null : { x: room.floor.x, y: room.floor.y, width: room.floor.width, height: room.floor.height });
  }

  protected async place(room: DungeonRoom): Promise<void> {
    if (this.busy() !== null) {
      return;
    }
    this.busy.set(room.id);
    this.problem.set(null);
    this.message.set('');
    try {
      const point = await this.api.placeScene(this.campaignId(), this.mapId(), room.id);
      this.state().upsertPoint(point);
      this.message.set(`Cena “Sala ${room.id}” posta no mapa, escondida dos jogadores.`);
      this.changed.emit();
    } catch (err) {
      this.problem.set({ room: room.id, text: placeSceneFailure(err) });
    } finally {
      this.busy.set(null);
    }
  }
}
