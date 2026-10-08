import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { ViewToken, bpToPercent, tokenKey } from '../map-geometry';

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
        [attr.data-item]="'token:' + key()"
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
    '[class.tk--mine]': 'token().mine && !token().creatureId',
    '[class.tk--npc]': 'npc()',
    '[class.tk--creature]': 'creature()',
    '[class.tk--selected]': 'selected()',
    '[class.tk--raised]': 'raised()',
    '[class.tk--legend]': 'legend()',
  },
})
export class MapToken {
  readonly token = input.required<ViewToken>();
  readonly initial = input.required<string>();
  readonly at = input<{ xBp: number; yBp: number } | null>(null);
  readonly interactive = input(false);
  readonly selected = input(false);
  readonly raised = input(false);
  /** Draw an NPC as the white rounded square and a character's creature with a dashed ring (docs/design.md; the fog map). */
  readonly kindShapes = input(false);
  /** Drawn in a legend: in the flow of the line, at the legend's size, never positioned on a map. The drawing is the same one. */
  readonly legend = input(false);

  protected readonly left = computed(() => bpToPercent(this.at()?.xBp ?? this.token().xBp));
  protected readonly top = computed(() => bpToPercent(this.at()?.yBp ?? this.token().yBp));
  protected readonly key = computed(() => tokenKey(this.token()));
  protected readonly creature = computed(() => this.kindShapes() && !!this.token().creatureId);
  protected readonly npc = computed(() => {
    const kind = this.token().kind;
    return (
      this.kindShapes() &&
      !this.token().creatureId &&
      kind !== undefined &&
      kind !== CharacterKind.PLAYER &&
      kind !== CharacterKind.UNSPECIFIED
    );
  });
  protected readonly label = computed(() => {
    const t = this.token();
    return `${t.name}${t.mine ? ' (você)' : ''}, ${t.hidden ? 'escondido' : 'visível'}`;
  });
}
