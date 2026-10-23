import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import type { MapToken } from '../../../gen/meurpg/maps/v1/maps_pb';
import { bpToPercent } from '../map-view/map-geometry';

/**
 * The master's light warning on the map (B3): a dashed ring around every character in the dark, and, for the one the
 * master chose with "Ver no mapa", the label "<Nome> · no escuro". It is drawn over the picture inside the map's stage
 * (`mapOverlay`), so it pans and zooms with it, and kept the size of a token whatever the zoom. `aria-hidden`: the
 * notice beside the map says the same in words. The master's map only: a player's map never gets it (RN-10).
 */
@Component({
  selector: 'app-dark-marks',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @for (t of marked(); track t.characterId) {
      <span class="ring" [attr.data-of]="t.characterId" [style.left.%]="left(t)" [style.top.%]="top(t)"></span>
    }
    @if (chosenToken(); as t) {
      <span class="label" [style.left.%]="left(t)" [style.top.%]="top(t)">{{ t.name }} · no escuro</span>
    }
  `,
  styleUrl: './dark-marks.scss',
  host: { 'aria-hidden': 'true' },
})
export class DarkMarks {
  readonly tokens = input.required<readonly MapToken[]>();
  /** The ids of the characters in the dark. */
  readonly dark = input.required<ReadonlySet<string>>();
  /** The character "Ver no mapa" chose: only this one gets the label. */
  readonly chosen = input<string | null>(null);

  protected readonly marked = computed(() =>
    this.tokens().filter((t) => !t.creatureId && this.dark().has(t.characterId)),
  );
  protected readonly chosenToken = computed(
    () => this.marked().find((t) => t.characterId === this.chosen()) ?? null,
  );

  protected left(t: MapToken): number {
    return bpToPercent(t.xBp);
  }

  protected top(t: MapToken): number {
    return bpToPercent(t.yBp);
  }
}
