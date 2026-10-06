import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { ViewPoint, ViewToken, tokenInitial, tokenKey } from '../map-geometry';
import { pointHidden } from '../map-labels';
import { StairMark } from '../../map-layers/stair-mark';
import { MapToken } from '../map-token/map-token';

/**
 * The legend under a map (README-B): the three marker shapes and, for the
 * master, "Revelado" (solid) and "Escondido" (dashed, with the badge); and
 * the tokens with their names, "(você)" on the caller's own and
 * "escondido" on a hidden one. Every state has a word as well as a line
 * style.
 */
@Component({
  selector: 'app-map-legend',
  imports: [MapToken, MatIconModule, StairMark],
  template: `
    @if (shapes() && (showBattle() || showSubmap() || showStairUp() || showStairDown() || showScene() || showRevealed() || showHidden())) {
      <ul class="lg" aria-label="Legenda do mapa">
        @if (showBattle()) {
          <li>
            <span class="lg__shape lg__shape--battle"><mat-icon>swords</mat-icon></span
            >Batalha
          </li>
        }
        @if (showSubmap()) {
          <li>
            <span class="lg__shape lg__shape--submap"><mat-icon>stairs</mat-icon></span
            >Submapa
          </li>
        }
        @if (showStairUp()) {
          <li><span class="lg__stair"><app-stair-mark direction="up" /></span>Escada para cima</li>
        }
        @if (showStairDown()) {
          <li><span class="lg__stair"><app-stair-mark direction="down" /></span>Escada para baixo</li>
        }
        @if (showScene()) {
          <li>
            <span class="lg__shape lg__shape--scene"><mat-icon>chat_bubble</mat-icon></span
            >Cena de RP
          </li>
        }
        @if (showRevealed()) {
          <li><span class="lg__shape lg__shape--box"></span>Revelado</li>
        }
        @if (showHidden()) {
          <li><span class="lg__shape lg__shape--box lg__shape--dashed"></span>Escondido</li>
        }
      </ul>
    }
    @if (tokens().length > 0) {
      <ul class="lg" aria-label="Tokens no mapa">
        @if (states() || tokensTitle()) {
          <li class="lg__title">Tokens</li>
        }
        @for (t of tokens(); track key(t)) {
          <li>
            <app-map-token [token]="t" [initial]="initial(t)" [kindShapes]="kindShapes()" [legend]="true" />
            <span>{{ t.name }}{{ t.hidden ? ', escondido' : '' }}@if (t.mine) {<span class="lg__you">&nbsp;(você)</span>}</span>
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
  /** The points of the map: the legend then lists only the kinds and the states the map has (a legend names what is drawn). Without it, all of them. */
  readonly points = input<readonly ViewPoint[] | null>(null);
  /** Draws an NPC as the white rounded square, as the map does (`kindShapes` of the map). */
  readonly kindShapes = input(false);
  /** "Tokens" above the names, even where the states are not listed (the editor, painting). */
  readonly tokensTitle = input(false);
  /** The letter of a token, as the map writes it (`mapTokenInitial`: "G2", "C"). */
  readonly initialOf = input<((token: ViewToken, all: readonly ViewToken[]) => string) | null>(null);

  private has(kind: number): boolean {
    const points = this.points();
    return points === null || points.some((p) => p.kind === kind);
  }
  protected readonly showBattle = computed(() => this.has(1));
  /** A generated dungeon's stairs (the point says so: `stairs`) are named for themselves, never as "Submapa". */
  protected readonly showSubmap = computed(() => this.points() === null || this.points()!.some((p) => p.kind === 2 && !p.stairs));
  protected readonly showStairUp = computed(() => this.points() !== null && this.points()!.some((p) => p.stairs === 1));
  protected readonly showStairDown = computed(() => this.points() !== null && this.points()!.some((p) => p.stairs === 2));
  protected readonly showScene = computed(() => this.points() === null || this.points()!.some((p) => p.kind === 3 || p.kind === 0));
  /** The two states belong to the three plain kinds: a trap, a treasure and a light have their own marks in the pins' legend. */
  private readonly plain = computed(() => (this.points() ?? []).filter((p) => p.kind <= 3 && !p.stairs));
  protected readonly showRevealed = computed(() => this.states() && (this.points() === null || this.plain().some((p) => !pointHidden(p))));
  protected readonly showHidden = computed(() => this.states() && (this.points() === null || this.plain().some((p) => pointHidden(p))));

  protected readonly key = tokenKey;
  private readonly all = computed(() => this.tokens());
  protected initial(token: ViewToken): string {
    return (this.initialOf() ?? tokenInitial)(token, this.all());
  }
}
