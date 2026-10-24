import {
  ChangeDetectionStrategy,
  Component,
  effect,
  input,
  output,
  untracked,
} from '@angular/core';

import type { DarkWatch } from '../../../core/maps/dark-watch';
import type { MapState } from '../../../core/maps/map-state';
import { DarkWarning } from '../../../shared/dark-warning/dark-warning';

/**
 * The master's light warning on the map page (B3): `app-dark-warning` over the editor's `DarkWatch`. In "Pintar" it
 * is one line. This page has no quick light control, so there is only "Ver no mapa". The watch reads again when the
 * map or its tokens change (the page's live stream reads the map again on `map_changed` and `token_moved`). With the
 * fog off, or when the read fails, nothing is drawn.
 */
@Component({
  selector: 'app-editor-dark',
  imports: [DarkWarning],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-dark-warning
      [characters]="watch().characters()"
      [announcement]="watch().announcement()"
      [compact]="compact()"
      [canLight]="false"
      (show)="show.emit($event)"
    />
  `,
})
export class EditorDark {
  readonly state = input.required<MapState>();
  readonly watch = input.required<DarkWatch>();
  /** The painting mode: one line. */
  readonly compact = input(false);

  /** "Ver no mapa": the page centers the map on this character. */
  readonly show = output<string>();

  constructor() {
    effect(() => {
      this.state().map();
      this.state().tokens();
      untracked(() => void this.watch().refresh());
    });
  }
}
