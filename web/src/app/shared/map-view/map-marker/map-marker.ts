import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { ViewPoint, bpToPercent } from '../map-geometry';
import { pointAriaLabel, pointKindIcon } from '../map-labels';

/**
 * A point of interest on the map (README-B, "Map markers"): a shape by kind
 * (diamond: Batalha, rounded square: Submapa, circle: Cena de RP) with its
 * glyph, drawn at the point's position. The shape is 34px and the hit area
 * 44px. Hidden: a dashed border, a muted glyph and an eye-off badge, so
 * the state is never colour alone. Selected: a 3px accent border.
 *
 * Presentational: `MapView` places the markers and handles pointers and
 * keys. With `interactive` the marker is a button; without it (the
 * previews), a plain element that is hidden from assistive tech (the lists
 * next to the map say the same things).
 */
@Component({
  selector: 'app-map-marker',
  imports: [MatIconModule, NgTemplateOutlet],
  template: `
    @if (interactive()) {
      <button
        type="button"
        class="pt__hit"
        [attr.data-item]="'point:' + point().id"
        [attr.aria-label]="label()"
        [attr.aria-pressed]="selected()"
      >
        <ng-container *ngTemplateOutlet="shape" />
      </button>
    } @else {
      <span class="pt__hit" aria-hidden="true"><ng-container *ngTemplateOutlet="shape" /></span>
    }
    <ng-template #shape>
      <span class="pt__shape" [class]="'pt__shape pt__shape--' + kindClass()">
        <mat-icon class="pt__icon" aria-hidden="true">{{ icon() }}</mat-icon>
      </span>
      @if (!point().revealed) {
        <span class="pt__badge" aria-hidden="true"><mat-icon>visibility_off</mat-icon></span>
      }
    </ng-template>
  `,
  styleUrl: './map-marker.scss',
  host: {
    '[style.left.%]': 'left()',
    '[style.top.%]': 'top()',
    '[class.pt--hidden]': '!point().revealed',
    '[class.pt--remembered]': '!!point().remembered',
    '[class.pt--selected]': 'selected()',
    '[class.pt--raised]': 'raised()',
  },
})
export class MapMarker {
  readonly point = input.required<ViewPoint>();
  /** Where to draw it, when a drag or a key is moving it (basis points). */
  readonly at = input<{ xBp: number; yBp: number } | null>(null);
  readonly interactive = input(true);
  readonly selected = input(false);
  readonly raised = input(false);

  protected readonly left = computed(() => bpToPercent(this.at()?.xBp ?? this.point().xBp));
  protected readonly top = computed(() => bpToPercent(this.at()?.yBp ?? this.point().yBp));
  protected readonly icon = computed(() => pointKindIcon(this.point().kind));
  protected readonly label = computed(() => pointAriaLabel(this.point()));
  protected readonly kindClass = computed(() => {
    switch (this.point().kind) {
      case 1:
        return 'battle';
      case 2:
        return 'submap';
      default:
        return 'scene';
    }
  });
}
