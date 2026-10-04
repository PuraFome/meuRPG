import { Component, computed, effect, inject, input, signal, untracked } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { CombatClient } from '../../../../core/combat/combat-client';
import { combatErrorMessage } from '../../../../core/combat/combat-errors';
import {
  type HighlightRow,
  type HighlightTile,
  highlightRows,
  highlightTiles,
} from '../../../../core/combat/combat-highlights';
import { joinNames } from '../../../../core/play/stage-view';
import type { CombatantInfo } from '../combat-info';

type LoadState = { status: 'loading' } | { status: 'error'; message: string } | { status: 'ready' };

/**
 * "Destaques do combate" in the master's end-of-combat summary (MR-032, E8-11):
 * a panel across the page between the summary tiles and the XP block. One tile
 * per category that has a winner (Mais dano causado, Mais cura, Tanque, Golpe
 * final, Acertos críticos), each with the number as the largest text, the
 * winner's name and the player's; a tie names everyone tied ("cada um"). A
 * category where everybody has 0 is not there (the server leaves it out). Under
 * the tiles, "Números de cada jogador" shows every number, zeros included, so
 * the master can check: only the master gets that table (RN-20). The players
 * get the same tiles on their card (`HighlightsCard`).
 *
 * The numbers are the damage and healing really applied, past 0 hit points not
 * counted, and nothing the master undid.
 */
@Component({
  selector: 'app-combat-highlights',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './combat-highlights.html',
  styleUrl: './combat-highlights.scss',
})
export class CombatHighlights {
  private readonly api = inject(CombatClient);

  readonly campaignId = input.required<string>();
  readonly encounterId = input.required<string>();
  /** The players' names by character, for "de Caio". */
  readonly info = input<ReadonlyMap<string, CombatantInfo>>(new Map());

  protected readonly state = signal<LoadState>({ status: 'loading' });
  protected readonly tiles = signal<readonly HighlightTile[]>([]);
  protected readonly rows = signal<readonly HighlightRow[]>([]);
  protected readonly error = computed(() => {
    const s = this.state();
    return s.status === 'error' ? s.message : '';
  });
  protected readonly empty = computed(() => this.state().status === 'ready' && this.tiles().length === 0);

  constructor() {
    effect(() => {
      const campaignId = this.campaignId();
      const encounterId = this.encounterId();
      untracked(() => void this.load(campaignId, encounterId));
    });
  }

  protected async load(campaignId = this.campaignId(), encounterId = this.encounterId()): Promise<void> {
    this.state.set({ status: 'loading' });
    try {
      const res = await this.api.highlights(campaignId, encounterId);
      this.tiles.set(highlightTiles(res));
      this.rows.set(highlightRows(res.characters));
      this.state.set({ status: 'ready' });
    } catch (err) {
      this.state.set({ status: 'error', message: combatErrorMessage(err, 'carregar os destaques') });
    }
  }

  /** "de Caio", "cada um · de Lia e Caio": the players behind the winners. */
  protected playersLine(tile: HighlightTile): string {
    const names = tile.characterIds
      .map((id) => this.info().get(id)?.playerName)
      .filter((n): n is string => !!n);
    const who = names.length > 0 ? `de ${joinNames(names)}` : '';
    return tile.tie ? (who ? `cada um · ${who}` : 'cada um') : who;
  }
}
