import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { HighlightTile } from '../../core/combat/combat-highlights';
import { joinNames } from '../../core/play/stage-view';

/**
 * The highlight tiles of the master's panels (MR-032, E8-11): one tile per
 * category that has a winner, each with the number as the largest text, the
 * winner's name and the player behind it; a tie names everyone ("cada um"). A
 * fixed grid: one column on a phone, two from a tablet (720px), four on a desktop
 * (1100px). The combat's "Destaques do combate" and the session's "Resumo da
 * sessão" draw the same tiles.
 */
@Component({
  selector: 'app-highlight-tiles',
  imports: [MatIconModule],
  template: `
    <ul class="hl__tiles">
      @for (t of tiles(); track t.kind) {
        <li class="tile">
          <span class="tile__label"><mat-icon aria-hidden="true">{{ t.icon }}</mat-icon>{{ t.label }}</span>
          <span class="tile__sub">{{ t.sub }}</span>
          <span class="tile__value">{{ t.value }}</span>
          <span class="tile__names">{{ t.names }}</span>
          @if (playersLine(t); as line) {
            <span class="tile__who">{{ line }}</span>
          }
        </li>
      }
    </ul>
  `,
  styleUrl: './highlight-tiles.scss',
})
export class HighlightTiles {
  readonly tiles = input.required<readonly HighlightTile[]>();
  /** The players' names by character, for "de Caio". */
  readonly players = input<ReadonlyMap<string, string>>(new Map());

  /** "de Caio", "cada um · de Lia e Caio": the players behind the winners. */
  protected playersLine(tile: HighlightTile): string {
    const names = tile.characterIds
      .map((id) => this.players().get(id))
      .filter((n): n is string => !!n);
    const who = names.length > 0 ? `de ${joinNames(names)}` : '';
    return tile.tie ? (who ? `cada um · ${who}` : 'cada um') : who;
  }
}
