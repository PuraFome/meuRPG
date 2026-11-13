import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import type { MapLayers } from '../../core/maps/layers';
import { layerLines } from '../../core/maps/paint-tools';
import { LayerSwatch } from './layer-swatch';

/**
 * "Camadas", read-only (the master's phone, E9-01 7, #589): each layer that has something painted, with its drawing and how much it holds
 * ("Superfície · 32 de água e 14 escaláveis · custam o dobro"). A layer with nothing painted is not listed, and a map with none has no
 * panel. The editor's own "Camadas" (`app-layers-panel`) is the one with switches and the save tag; painting is for the computer.
 */
@Component({
  selector: 'app-map-layers-list',
  imports: [LayerSwatch],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (lines().length > 0) {
      <section class="mr-panel" aria-labelledby="mll-title">
        <h2 class="mr-panel__title" id="mll-title">Camadas</h2>
        <ul class="mll">
          @for (line of lines(); track line.tool) {
            <li class="mll__row">
              <app-layer-swatch [tool]="line.tool" />
              <span class="mll__text">
                <span class="mll__name">{{ line.name }}</span>&ngsp;<span class="mll__detail">{{ line.detail }}</span>
              </span>
            </li>
          }
        </ul>
      </section>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .mll {
      display: flex;
      flex-direction: column;
      margin: 0;
      padding: 0;
      list-style: none;
    }

    .mll__row {
      display: flex;
      align-items: center;
      gap: var(--mr-space-3);
      padding: var(--mr-space-3) 0;

      & + & {
        border-top: 1px solid var(--mr-rule);
      }

      &:first-child {
        padding-top: 0;
      }

      &:last-child {
        padding-bottom: 0;
      }
    }

    .mll__text {
      display: flex;
      flex-direction: column;
      min-width: 0;
    }

    .mll__name {
      font-weight: 700;
    }

    .mll__detail {
      font-size: var(--mr-font-14);
      line-height: 19px;
      color: var(--mr-ink-muted);
    }
  `,
})
export class MapLayersList {
  readonly layers = input.required<MapLayers>();
  protected readonly lines = computed(() =>
    layerLines(this.layers(), true).filter((l) => l.count > 0),
  );
}
