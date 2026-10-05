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
    <ul class="mr-legend" [attr.aria-label]="label()">
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
      <ng-content />
    </ul>
  `,
  styles: ':host { display: block; }',
})
export class MapLayersLegend {
  readonly layers = input.required<MapLayers>();
  readonly label = input('Legenda do mapa');
}
