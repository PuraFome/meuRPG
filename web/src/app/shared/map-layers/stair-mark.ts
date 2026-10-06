import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * A generated dungeon's stair, drawn in the one way MAP-LANGUAGE-E10.md gives it, on the map, in the preview and in the legend: a square
 * badge with an ink outline and a paper fill, and an arrow, up for "Escada para cima" and down for "Escada para baixo". It fills the box
 * its host gives it. Decorative: the lists, the legend and the points' labels say it in words.
 */
@Component({
  selector: 'app-stair-mark',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `<mat-icon aria-hidden="true">{{ direction() === 'up' ? 'arrow_upward' : 'arrow_downward' }}</mat-icon>`,
  styles: `
    :host {
      box-sizing: border-box;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 100%;
      height: 100%;
      border: 2px solid var(--mr-map-token-ink);
      border-radius: 18%;
      background: var(--mr-map-token-surface);
      color: var(--mr-map-token-ink);
      container-type: size;
    }

    .mat-icon {
      width: 70cqmin;
      height: 70cqmin;
      font-size: 70cqmin;
    }
  `,
})
export class StairMark {
  readonly direction = input.required<'up' | 'down'>();
}
