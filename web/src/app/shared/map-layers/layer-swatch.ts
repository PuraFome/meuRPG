import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { PaintTool } from '../../core/maps/paint-tools';
import { DoorMark } from './door-mark';

/**
 * The small drawing that stands for a painted layer in a list ("Camadas" of the editor, and the master's phone): the same looks the map and
 * its legend use (`.mr-swatch`, styles/_ui.scss), one per tool. "Superfície" is split along the diagonal, water over climbable.
 * Presentational and decorative: the row's words name the layer.
 */
@Component({
  selector: 'app-layer-swatch',
  imports: [DoorMark, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @switch (tool()) {
      @case ('terrain') {
        <span class="mr-swatch mr-swatch--terrain"></span>
      }
      @case ('surface') {
        <span class="mr-swatch mr-swatch--surface"></span>
      }
      @case ('wall') {
        <span class="mr-swatch mr-swatch--wall"></span>
      }
      @case ('cover') {
        <span class="mr-swatch mr-swatch--half"></span>
      }
      @case ('nopass') {
        <span class="mr-swatch mr-swatch--nopass"></span>
      }
      @case ('door') {
        <span class="mr-swatch mr-swatch--door"><app-door-mark [state]="2" /></span>
      }
      @default {
        <span class="mr-swatch sw__light"><mat-icon>contrast</mat-icon></span>
      }
    }
  `,
  styles: `
    :host {
      display: flex;
      flex: none;
      width: 30px;
    }

    .mr-swatch {
      width: 30px;
      height: 30px;
    }

    .sw__light {
      display: flex;
      align-items: center;
      justify-content: center;
      color: var(--mr-map-token-ink);
    }

    .sw__light .mat-icon {
      width: 18px;
      height: 18px;
      font-size: var(--mr-font-18);
    }
  `,
  host: { 'aria-hidden': 'true' },
})
export class LayerSwatch {
  readonly tool = input.required<PaintTool>();
}
