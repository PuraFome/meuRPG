import { ChangeDetectionStrategy, Component, input } from '@angular/core';

import type { MapLayers } from '../../core/maps/layers';

/**
 * The names of the layer marks on a map (MAP-LANGUAGE.md): "Parede", "Terreno
 * difícil", "Meia cobertura", "Três quartos", each only when the map has one
 * (a legend lists what is drawn). The swatches are `.mr-swatch` (styles/_ui.scss),
 * which draws the overlay's own looks. The screen that shows the map projects
 * its other marks (fog states before, movement marks and tokens after) as `li`s
 * into the same list, so the order of MAP-LANGUAGE.md holds.
 */
@Component({
  selector: 'app-map-layers-legend',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ul class="mr-legend" [class.mr-legend--on-map]="onMap()" [attr.aria-label]="label()">
      <ng-content select="[before]" />
      @if (layers().walls.length) {
        <li><span class="mr-swatch mr-swatch--wall" aria-hidden="true"></span>Parede</li>
      }
      @if (layers().terrain.length) {
        <li><span class="mr-swatch mr-swatch--terrain" aria-hidden="true"></span>Terreno difícil</li>
      }
      @if (layers().half.length) {
        <li><span class="mr-swatch mr-swatch--half" aria-hidden="true"></span>Meia cobertura</li>
      }
      @if (layers().threeQuarters.length) {
        <li><span class="mr-swatch mr-swatch--three" aria-hidden="true"></span>Três quartos</li>
      }
      @if (light(); as l) {
        @if (l.bright) {
          <li><span class="lgl lgl--bright" aria-hidden="true"></span>Claro</li>
        }
        @if (l.dim) {
          <li><span class="lgl lgl--dim" aria-hidden="true"></span>Penumbra</li>
        }
        @if (l.dark) {
          <li><span class="lgl lgl--dark" aria-hidden="true"></span>Escuro</li>
        }
      }
      <ng-content />
    </ul>
  `,
  styles: `
    :host {
      display: block;
    }

    /* The light's glyph, as the map draws it (a round paper chip with the font's ligature). */
    .lgl {
      box-sizing: border-box;
      flex: none;
      width: 22px;
      height: 22px;
      border: 1px solid var(--mr-map-token-ink);
      border-radius: 50%;
      background: var(--mr-map-token-surface);
      color: var(--mr-map-token-ink);
      font-family: 'Material Symbols Outlined';
      font-size: 15px;
      font-weight: 400;
      line-height: 20px;
      text-align: center;
      font-feature-settings: 'liga';
      overflow: hidden;
    }

    .lgl--bright::before {
      content: 'light_mode';
    }

    .lgl--dim::before {
      content: 'contrast';
    }

    .lgl--dark::before {
      content: 'dark_mode';
    }
  `,
})
export class MapLayersLegend {
  readonly layers = input.required<MapLayers>();
  readonly label = input('Legenda do mapa');
  /** The swatches sit on a sample of the map's floor (a fog map's legend). */
  readonly onMap = input(false);
  /** Which painted light levels the map has (the editor): "Claro", "Penumbra", "Escuro", the names of the toolbar. */
  readonly light = input<{ bright: boolean; dim: boolean; dark: boolean } | null>(null);
}
