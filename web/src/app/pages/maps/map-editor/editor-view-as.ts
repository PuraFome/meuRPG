import { DestroyRef, effect, inject, signal, untracked } from '@angular/core';

import type { Map as MapMessage } from '../../../../gen/meurpg/maps/v1/maps_pb';
import type { MapsClient } from '../../../core/maps/maps-client';
import { ViewAsCounts } from '../../../core/maps/view-as';
import type { ViewAsPerson } from '../../../shared/fog-map/view-as-list';

/** The pause before the counts of "Ver como" are read, so a burst of changes reads once. */
const COUNTS_DELAY_MS = 250;

/** What "Ver como" needs from the editor. */
export interface EditorViewAsHost {
  readonly api: Pick<MapsClient, 'vision'>;
  campaignId(): string;
  mapId(): string;
  map(): MapMessage | null;
  /** The characters the master can look as. */
  people(): readonly ViewAsPerson[];
}

/**
 * "Ver como": the map as one player gets it (which character, the note and the word when they are gone) and the counts shown
 * beside each ("22 quadrados vistos"), read when the fog is on and the characters are known. Built in an injection context: it owns
 * two effects.
 */
export class EditorViewAs {
  readonly viewAs = signal<string | null>(null);
  readonly viewNote = signal('');
  readonly viewGone = signal('');
  readonly counts = new ViewAsCounts((mapId, characterId) =>
    this.host.api.vision(this.host.campaignId(), mapId, characterId),
  );
  private timer: ReturnType<typeof setTimeout> | undefined;

  constructor(private readonly host: EditorViewAsHost) {
    inject(DestroyRef).onDestroy(() => clearTimeout(this.timer));
    // "Ver como" counts ("22 quadrados vistos"): read when the fog is on and the characters are known.
    effect(() => {
      const map = this.host.map();
      const ids = this.host.people().map((p) => p.id);
      if (!map?.fogEnabled || ids.length === 0) {
        return;
      }
      untracked(() => {
        clearTimeout(this.timer);
        this.timer = setTimeout(() => void this.counts.read(map.id, ids), COUNTS_DELAY_MS);
      });
    });
    // The fog went off: nothing to look as.
    effect(() => {
      if (this.host.map()?.fogEnabled === false && this.viewAs() !== null) {
        untracked(() => this.set(null));
      }
    });
  }

  set(id: string | null): void {
    this.viewAs.set(id);
    this.viewNote.set('');
    this.viewGone.set('');
  }

  gone(): void {
    this.viewAs.set(null);
    this.viewNote.set('');
    this.viewGone.set('Esse personagem morreu ou saiu da campanha. Voltamos para “Todos”.');
  }

  /** The fog was forgotten: the counts are read again. */
  forgotten(): void {
    if (this.host.map()?.fogEnabled) {
      void this.counts.read(
        this.host.mapId(),
        this.host.people().map((p) => p.id),
      );
    }
  }
}
