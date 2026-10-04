import { Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { HighlightTile, OwnNumber } from '../../core/combat/combat-highlights';

/**
 * The players' card of an ending (MR-032, E8-11 states 3 and 5): "O combate
 * acabou" for a combat and "A sessão acabou" for the session. At the top of
 * the page, until the player closes it with "Fechar" or the ✕ (44px, so the way
 * out stays in sight even on a 320x568 screen where the card scrolls with the
 * page).
 *
 * It shows what the master's panel shows, without the table: every category that
 * has a winner, with the number and the names (a tie names everyone), and "Você"
 * on a tile the reader's own character won. "Seu resultado, Pensantus" gives the
 * reader's own numbers, zeros included: the server sends a player their own row,
 * never anyone else's (RN-20).
 *
 * The page does not move focus to it (the player is already reading it): the
 * title sits in a polite live region, so a screen reader says "O combate acabou"
 * and the rest is a tab away. Motion (a 200ms fade) only with
 * `prefers-reduced-motion: no-preference`.
 */
@Component({
  selector: 'app-highlights-frame',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './highlights-frame.html',
  styleUrl: './highlights-frame.scss',
})
export class HighlightsFrame {
  readonly ariaLabel = input.required<string>();
  /** The green word with the check: "Combate encerrado". */
  readonly tag = input.required<string>();
  readonly title = input.required<string>();
  /** "Emboscada na estrada · 4 rodadas". */
  readonly subtitle = input('');
  readonly tiles = input.required<readonly HighlightTile[]>();
  /** The title over the tiles: "Destaques" for a combat, "Resumo da sessão" for the session. */
  readonly heading = input('Destaques');
  /** Said when no category has a winner. */
  readonly none = input('');
  /** The reader's own character: marks "Você". */
  readonly characterId = input('');
  readonly ownTitle = input('');
  readonly own = input<readonly OwnNumber[]>([]);

  readonly closed = output<void>();

  protected mine(tile: HighlightTile): boolean {
    return !!this.characterId() && tile.characterIds.includes(this.characterId());
  }
}
