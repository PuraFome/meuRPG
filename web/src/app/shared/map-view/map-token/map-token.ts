import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { ViewToken, bpToPercent } from '../map-geometry';

/**
 * A character's token on the map (README-B, "Map markers"): an `ink` disc
 * with the initial (Alegreya 700) and a 2px `surface` halo. Hidden (the
 * master only): hollow, with a dashed border and the eye-off badge. The
 * caller's own token (`token.mine`) gets an accent halo. A button only
 * where the master can drag it; otherwise a plain element, hidden from
 * assistive tech (the lists carry the same names).
 */
@Component({
  selector: 'app-map-token',
  imports: [MatIconModule],
  template: `
    @if (interactive()) {
      <button
        type="button"
        class="tk__hit"
        [attr.data-item]="'token:' + token().characterId"
        [attr.aria-label]="label()"
        [attr.aria-pressed]="selected()"
      >
        <span class="tk__disc">{{ initial() }}</span>
        @if (token().hidden) {
          <span class="tk__badge" aria-hidden="true"><mat-icon>visibility_off</mat-icon></span>
        }
      </button>
    } @else {
      <span class="tk__hit" aria-hidden="true">
        <span class="tk__disc">{{ initial() }}</span>
        @if (token().hidden) {
          <span class="tk__badge"><mat-icon>visibility_off</mat-icon></span>
        }
      </span>
    }
  `,
  styleUrl: './map-token.scss',
  host: {
    '[style.left.%]': 'left()',
    '[style.top.%]': 'top()',
    '[class.tk--hidden]': 'token().hidden',
    '[class.tk--mine]': 'token().mine',
    '[class.tk--selected]': 'selected()',
    '[class.tk--raised]': 'raised()',
  },
})
export class MapToken {
  readonly token = input.required<ViewToken>();
  readonly initial = input.required<string>();
  readonly at = input<{ xBp: number; yBp: number } | null>(null);
  readonly interactive = input(false);
  readonly selected = input(false);
  readonly raised = input(false);

  protected readonly left = computed(() => bpToPercent(this.at()?.xBp ?? this.token().xBp));
  protected readonly top = computed(() => bpToPercent(this.at()?.yBp ?? this.token().yBp));
  protected readonly label = computed(() => {
    const t = this.token();
    return `${t.name}${t.mine ? ' (você)' : ''}, ${t.hidden ? 'escondido' : 'visível'}`;
  });
}
