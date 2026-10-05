import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { ViewToken, tokenInitial, tokenKey } from '../map-geometry';

/**
 * The legend under a map (README-B): the three marker shapes and, for the
 * master, "Revelado" (solid) and "Escondido" (dashed, with the badge); and
 * the tokens with their names, "(você)" on the caller's own and
 * "escondido" on a hidden one. Every state has a word as well as a line
 * style.
 */
@Component({
  selector: 'app-map-legend',
  imports: [MatIconModule],
  template: `
    @if (shapes()) {
      <ul class="lg" aria-label="Legenda do mapa">
        <li>
          <span class="lg__shape lg__shape--battle"><mat-icon>swords</mat-icon></span
          >Batalha
        </li>
        <li>
          <span class="lg__shape lg__shape--submap"><mat-icon>stairs</mat-icon></span
          >Submapa
        </li>
        <li>
          <span class="lg__shape lg__shape--scene"><mat-icon>chat_bubble</mat-icon></span
          >Cena de RP
        </li>
        @if (states()) {
          <li><span class="lg__shape lg__shape--box"></span>Revelado</li>
          <li><span class="lg__shape lg__shape--box lg__shape--dashed"></span>Escondido</li>
        }
      </ul>
    }
    @if (tokens().length > 0) {
      <ul class="lg" aria-label="Tokens no mapa">
        @if (states()) {
          <li class="lg__title">Tokens</li>
        }
        @for (t of tokens(); track key(t)) {
          <li>
            <span
              class="lg__disc"
              [class.lg__disc--hidden]="t.hidden"
              [class.lg__disc--mine]="t.mine"
              >{{ initial(t) }}</span
            >
            {{ t.name }}{{ t.hidden ? ', escondido' : '' }}
            @if (t.mine) {
              <span class="lg__you"> (você)</span>
            }
          </li>
        }
      </ul>
    }
  `,
  styleUrl: './map-legend.scss',
})
export class MapLegend {
  /** The tokens to name under the map. */
  readonly tokens = input<readonly ViewToken[]>([]);
  /** The three marker shapes (the master's maps and the editor). */
  readonly shapes = input(true);
  /** "Revelado" and "Escondido": only the master sees both states. */
  readonly states = input(false);

  protected readonly key = tokenKey;
  private readonly all = computed(() => this.tokens());
  protected initial(token: ViewToken): string {
    return tokenInitial(token, this.all());
  }
}
