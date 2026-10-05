import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { ChestIcon } from '../../chest-icon/chest-icon';
import { ViewPoint, bpToPercent } from '../map-geometry';
import { pointAriaLabel, pointHidden, pointKindIcon } from '../map-labels';

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
  imports: [ChestIcon, MatIconModule, NgTemplateOutlet],
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
        @if (point().kind === 5) {
          <app-chest-icon class="pt__icon pt__chest" />
        } @else {
          <mat-icon class="pt__icon" aria-hidden="true">{{ icon() }}</mat-icon>
        }
      </span>
      @if (hidden() && !pin()) {
        <span class="pt__badge" aria-hidden="true"><mat-icon>visibility_off</mat-icon></span>
      }
    </ng-template>
  `,
  styleUrl: './map-marker.scss',
  host: {
    '[style.left.%]': 'left()',
    '[style.top.%]': 'top()',
    '[class.pt--hidden]': 'hidden()',
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
  /** The screen draws traps, chests and lights itself (`app-map-pins`, the editor and the master's phone): this is then only their hit area. Without it the marker draws its own icon. */
  readonly pinsDrawn = input(false);

  protected readonly left = computed(() => bpToPercent(this.at()?.xBp ?? this.point().xBp));
  protected readonly top = computed(() => bpToPercent(this.at()?.yBp ?? this.point().yBp));
  protected readonly hidden = computed(() => pointHidden(this.point()));
  protected readonly icon = computed(() => pointKindIcon(this.point().kind));
  protected readonly label = computed(() => pointAriaLabel(this.point()));
  /** A trap, a treasure or a light (kinds 4 to 6): the map's own marks (`app-map-pins`) draw it, and this is only the hit area and the selection ring. */
  protected readonly pin = computed(() => this.pinsDrawn() && this.point().kind >= 4);
  protected readonly kindClass = computed(() => {
    switch (this.point().kind) {
      case 1:
        return 'battle';
      case 2:
        return 'submap';
      case 4:
      case 5:
      case 6:
        return this.pinsDrawn() ? 'pin' : 'scene';
      default:
        return 'scene';
    }
  });
}
