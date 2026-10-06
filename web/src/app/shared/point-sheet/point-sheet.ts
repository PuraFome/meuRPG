import { Component, ElementRef, afterNextRender, input, output, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { type MapPoint, MapPointKind } from '../../../gen/meurpg/maps/v1/maps_pb';
import { pointKindIcon, pointKindLabel } from '../map-view/map-labels';
import { TreasureFacts } from '../treasure-facts/treasure-facts';

/**
 * What a player reads when they open a point (E5-25, E5-26): the kind's
 * glyph, the name (Alegreya 22), the kind in words and the description.
 * A Submapa point whose target the player may see also offers "Abrir
 * <mapa>": the sheet comes first, never a direct jump, so the description
 * stays readable (README-B, "Unsure" 2). The server drops the target for a
 * map the player can't see, so a point without `targetMap` only reads.
 *
 * On a phone the parent shows it as a non-modal bottom sheet over the map,
 * which stays pannable; from a tablet up it is the first block of the side
 * column. On open, focus goes to the title; "Fechar" and Esc close (the
 * parent puts focus back on the marker).
 */
@Component({
  selector: 'app-point-sheet',
  imports: [MatButtonModule, MatIconModule, TreasureFacts],
  template: `
    <header class="sheet__head">
      <span class="sheet__glyph" aria-hidden="true">
        <mat-icon>{{ icon() }}</mat-icon>
      </span>
      <div class="sheet__titles">
        <h2 #title class="sheet__title" tabindex="-1">{{ point().name }}</h2>
        <p class="sheet__kind">{{ kind() }}</p>
      </div>
      <button type="button" class="sheet__close" aria-label="Fechar" (click)="closed.emit()">
        <mat-icon aria-hidden="true">close</mat-icon>
      </button>
    </header>
    @if (found()) {
      <!-- A treasure that was found: who found it, its value and what is inside (E9-09 4). -->
      <app-treasure-facts [point]="point()" />
    } @else if (point().description) {
      <p class="sheet__text">{{ point().description }}</p>
    }
    @if (point().targetMap; as target) {
      <button matButton="outlined" type="button" class="sheet__open" (click)="open.emit(target.id)">
        <mat-icon aria-hidden="true">map</mat-icon>Abrir {{ target.name }}
      </button>
    }
  `,
  styleUrl: './point-sheet.scss',
  host: { '(keydown.escape)': 'closed.emit()' },
})
export class PointSheet {
  readonly point = input.required<MapPoint>();
  readonly closed = output<void>();
  /** "Abrir <mapa>": the target map's ID. */
  readonly open = output<string>();

  private readonly title = viewChild.required<ElementRef<HTMLElement>>('title');
  protected found = () => this.point().kind === MapPointKind.TREASURE && this.point().treasureFoundAt !== undefined;
  protected icon = () => pointKindIcon(this.point().kind, this.point().stairs);
  protected kind = () => pointKindLabel(this.point().kind, this.point().stairs);

  constructor() {
    // Opening the sheet moves focus to its title (a panel that appears on
    // an action); the page recreates it for each point it opens.
    afterNextRender(() => this.focusTitle());
  }

  /** Focus the title again (the same sheet shows another point). */
  focusTitle(): void {
    this.title().nativeElement.focus({ preventScroll: true });
  }
}
