import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { type DoorKind, type MapLayers, doorCounts } from '../../core/maps/layers';
import { DOOR_NAME, DoorMark } from './door-mark';
import { StairMark } from './stair-mark';

/** The doors' entries in the order of MAP-LANGUAGE-E10.md: fechada, aberta, trancada, grade, secreta. */
const DOOR_ORDER: readonly DoorKind[] = [2, 1, 3, 4, 5];

/**
 * The names of the layer marks on a map (MAP-LANGUAGE.md): "Parede", "Terreno
 * difícil", "Meia cobertura", "Três quartos", each only when the map has one
 * (a legend lists what is drawn), and, after "Parede", the doors the map really has (`app-door-mark`: "Porta fechada", "Porta aberta",
 * "Porta trancada", "Grade", "Porta secreta"); the padlock and the secret door are the master's, and their entries say "(só você vê)" with the
 * crossed eye. The swatches are `.mr-swatch` (styles/_ui.scss),
 * which draws the overlay's own looks. The screen that shows the map projects
 * its other marks (fog states before, movement marks and tokens after) as `li`s
 * into the same list, so the order of MAP-LANGUAGE.md holds.
 */
@Component({
  selector: 'app-map-layers-legend',
  imports: [DoorMark, MatIconModule, StairMark],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ul class="mr-legend" [class.mr-legend--on-map]="onMap()" [attr.aria-label]="label()">
      <ng-content select="[before]" />
      @if (wallsShown()) {
        <li><span class="mr-swatch mr-swatch--wall" aria-hidden="true"></span>Parede</li>
      }
      @for (d of doorKinds(); track d) {
        <li>
          <span class="mr-swatch mr-swatch--door" aria-hidden="true"><app-door-mark [state]="d" /></span>
          <!-- One piece, so the flex gap of the row does not open a second gap before the note. -->
          <span class="nm">{{ names[d] }}{{ d === 3 || d === 5 ? ' (só você vê)' : '' }}@if (d === 3 || d === 5) {<mat-icon aria-hidden="true">visibility_off</mat-icon>}</span>
        </li>
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
      @if (stairs().up) {
        <li><span class="mr-swatch mr-swatch--stair" aria-hidden="true"><app-stair-mark direction="up" /></span>Escada para cima</li>
      }
      @if (stairs().down) {
        <li><span class="mr-swatch mr-swatch--stair" aria-hidden="true"><app-stair-mark direction="down" /></span>Escada para baixo</li>
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

    // A master-only door says "(só você vê)" with the crossed eye, as the other master-only marks of the legend do.
    .nm {
      display: inline-flex;
      align-items: center;
      gap: 4px;
    }

    .nm .mat-icon {
      width: 18px;
      height: 18px;
      font-size: 18px;
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
  protected readonly names = DOOR_NAME;
  /** The kinds of door the map has, in the legend's order. */
  protected readonly doorKinds = computed(() => {
    const counts = doorCounts(this.layers());
    return DOOR_ORDER.filter((k) => counts[k] > 0);
  });
  readonly label = input('Legenda do mapa');
  /** A drawing with no wall squares of its own (the dungeon preview draws its walls as one shape) still names "Parede". */
  readonly showWalls = input<boolean | null>(null);
  protected readonly wallsShown = computed(
    () => this.showWalls() ?? this.layers().walls.length > 0,
  );
  /** Which stairs the map has (a generated dungeon's): the legend names those, after the doors. */
  readonly stairs = input<{ up: boolean; down: boolean }>({ up: false, down: false });
  /** The swatches sit on a sample of the map's floor (a fog map's legend). */
  readonly onMap = input(false);
  /** Which painted light levels the map has (the editor): "Claro", "Penumbra", "Escuro", the names of the toolbar. */
  readonly light = input<{ bright: boolean; dim: boolean; dark: boolean } | null>(null);
}
