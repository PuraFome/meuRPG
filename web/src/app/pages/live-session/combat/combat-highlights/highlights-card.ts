import { Component, computed, effect, inject, input, output, signal, untracked } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { CharacterHighlights } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import {
  type HighlightTile,
  type OwnNumber,
  highlightTiles,
  ownNumbers,
} from '../../../../core/combat/combat-highlights';

/**
 * The players' "O combate acabou" card (MR-032, E8-11 state 3): at the top of
 * the session page when the server says the combat ended, until the player
 * closes it with "Fechar" or the ✕ (44px, so the way out stays in sight even
 * on a 320x568 screen where the card scrolls with the page).
 *
 * It shows what the master's panel shows, without the table: every category
 * that has a winner, with the number and the names (a tie names everyone), and
 * "Você" on a tile the reader's own character won. "Seu resultado, Pensantus"
 * gives the reader's own four numbers, zeros included: the server sends a
 * player exactly one row of the table, their own character's, never anyone
 * else's (RN-20).
 *
 * The page does not move focus to it (the player is already reading it): the
 * title sits in a polite live region, so a screen reader says "O combate
 * acabou" and the rest is a tab away. Motion (a 200ms fade) only with
 * `prefers-reduced-motion: no-preference`. A failed read shows nothing: it is
 * a bonus, not a screen the player needs.
 */
@Component({
  selector: 'app-highlights-card',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './highlights-card.html',
  styleUrl: './highlights-card.scss',
})
export class HighlightsCard {
  private readonly api = inject(CombatClient);

  readonly campaignId = input.required<string>();
  readonly encounterId = input.required<string>();
  /** "Emboscada na estrada · 4 rodadas". */
  readonly subtitle = input('');
  /** The reader's own character: marks "Você" and gives "Seu resultado". */
  readonly characterId = input('');
  readonly characterName = input('');

  readonly closed = output<void>();

  protected readonly tiles = signal<readonly HighlightTile[]>([]);
  protected readonly loaded = signal(false);
  private readonly rows = signal<readonly CharacterHighlights[]>([]);
  protected readonly own = computed<OwnNumber[]>(() => ownNumbers(this.rows(), this.characterId()));

  constructor() {
    effect(() => {
      const campaignId = this.campaignId();
      const encounterId = this.encounterId();
      untracked(() => void this.load(campaignId, encounterId));
    });
  }

  private async load(campaignId: string, encounterId: string): Promise<void> {
    this.loaded.set(false);
    try {
      const res = await this.api.highlights(campaignId, encounterId);
      this.tiles.set(highlightTiles(res));
      this.rows.set(res.characters);
      this.loaded.set(true);
    } catch {
      // Best effort: with no highlights there is no card.
      this.tiles.set([]);
      this.loaded.set(false);
    }
  }

  protected mine(tile: HighlightTile): boolean {
    return !!this.characterId() && tile.characterIds.includes(this.characterId());
  }
}
