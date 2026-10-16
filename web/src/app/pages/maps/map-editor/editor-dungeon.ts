import { computed, effect, inject, signal, untracked } from '@angular/core';

import type { Map as MapMessage } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { DungeonInfo } from '../../../core/maps/dungeon-info';
import { DungeonsClient } from '../../../core/maps/dungeons-client';
import { BP_MAX } from '../../../shared/map-view/map-geometry';
import type { MapView } from '../../../shared/map-view/map-view';
import type { RoomOutline } from '../dungeon-rooms/dungeon-rooms';

/** The zoom a chosen room is shown at: the room fills about this share of the width, between the two limits. */
const ROOM_FILL = 0.55;
const ROOM_SCALE_MIN = 1.2;
const ROOM_SCALE_MAX = 3;

/** What the dungeon rooms need from the editor. */
export interface EditorDungeonHost {
  campaignId(): string;
  map(): MapMessage | null;
  view(): Pick<MapView, 'fit' | 'focusOn'> | undefined;
}

/**
 * What the server says of a generated dungeon (its rooms list, and whether the image is still the generator's), and the room
 * outlined on the map. Built in an injection context: it owns two effects.
 */
export class EditorDungeon {
  readonly info = new DungeonInfo(inject(DungeonsClient));
  /** The room outlined on the map (chosen in the rooms list). */
  readonly roomOutline = signal<RoomOutline | null>(null);

  // Computed apart, so a change of anything else in the map (its revision, its fog) reads nothing again.
  private readonly mapId = computed(() => this.host.map()?.id ?? '');
  private readonly imageId = computed(() => this.host.map()?.image?.id ?? '');
  /** The server says (to the master only) whether the map is a generated dungeon: only then are its rooms asked for. */
  private readonly isDungeon = computed(() => this.host.map()?.generatedDungeon ?? false);

  constructor(private readonly host: EditorDungeonHost) {
    // A generated dungeon's rooms: read when a map opens and when its image changes ("Gerada pelo app" is about the image).
    effect(() => {
      const id = this.mapId();
      void this.imageId();
      const dungeon = this.isDungeon();
      untracked(() => {
        if (id === '' || !dungeon) {
          this.info.clear();
          return;
        }
        void this.info.load(this.host.campaignId(), id);
      });
    });
    effect(() => {
      void this.mapId();
      untracked(() => this.roomOutline.set(null));
    });
  }

  reload(): void {
    void this.info.load(this.host.campaignId(), this.mapId());
  }

  /** A room chosen in the list is outlined, and the map goes to it (and back to the whole map when it is let go). */
  outline(room: RoomOutline | null): void {
    this.roomOutline.set(room);
    const m = this.host.map();
    const view = this.host.view();
    if (!room || !m || m.gridColumns <= 0 || m.gridRows <= 0) {
      view?.fit();
      return;
    }
    const scale = Math.min(
      ROOM_SCALE_MAX,
      Math.max(ROOM_SCALE_MIN, (ROOM_FILL * m.gridColumns) / room.width),
    );
    view?.focusOn(
      {
        xBp: ((room.x + room.width / 2) / m.gridColumns) * BP_MAX,
        yBp: ((room.y + room.height / 2) / m.gridRows) * BP_MAX,
      },
      scale,
    );
  }
}
