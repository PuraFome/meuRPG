import {
  ChangeDetectionStrategy,
  Component,
  effect,
  inject,
  input,
  output,
  untracked,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';

import type { MapToken } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { carriedOptions } from '../../../core/maps/carried-light';
import type { DarkWatch } from '../../../core/maps/dark-watch';
import { type LightOption, LightPresets } from '../../../core/maps/light-presets';
import type { MapState } from '../../../core/maps/map-state';
import { DarkWarning } from '../../../shared/dark-warning/dark-warning';
import { openCarriedLight } from '../carried-light/carried-light-sheet';

/**
 * The master's light warning on the session map card (B3): `app-dark-warning` over the page's `DarkWatch`, with the
 * quick light that already exists. "Acender luz" opens the carried-light sheet for that character (the same
 * `SetCarriedLight` that "Luz dos personagens" uses; nothing of its own), and the choice updates the token and reads
 * the warning again. "Ver no mapa" is passed on to the page. The watch reads again when the map's tokens or the map
 * itself change (`map_changed` makes the page read the map again); the page also reads it on the vision hints.
 * Master only: the session map never creates it for a player (RN-10).
 */
@Component({
  selector: 'app-session-dark',
  imports: [DarkWarning],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-dark-warning
      [characters]="watch().characters()"
      [announcement]="watch().announcement()"
      [canLight]="true"
      (show)="show.emit($event)"
      (light)="openLight($event)"
    />
  `,
})
export class SessionDark {
  private readonly presets = inject(LightPresets);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);

  readonly campaignId = input.required<string>();
  readonly state = input.required<MapState>();
  readonly watch = input.required<DarkWatch>();

  /** "Ver no mapa": the page centers the map on this character. */
  readonly show = output<string>();

  constructor() {
    // The map or its tokens changed (a carried light, a Luz point, a token placed or moved): who is in the dark may have.
    effect(() => {
      this.state().map();
      this.state().tokens();
      untracked(() => void this.watch().refresh());
    });
  }

  protected async openLight(characterId: string): Promise<void> {
    const state = this.state();
    const mapId = state.map()?.id;
    const token = state.tokens().find((t) => !t.creatureId && t.characterId === characterId);
    if (!mapId || !token) {
      return;
    }
    let all: readonly LightOption[];
    try {
      all = await this.presets.list(this.campaignId());
    } catch {
      return;
    }
    openCarriedLight(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      mapId,
      characterId,
      characterName: token.name,
      title: `Acender luz: ${token.name}`,
      current: token.carriedLight,
      options: carriedOptions(all, token.carriedLight),
      changed: (next: MapToken) => {
        state.upsertToken(next);
        void this.watch().refresh();
      },
    }).subscribe();
  }
}
