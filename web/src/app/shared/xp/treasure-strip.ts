import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { TreasureToConvert } from '../../../gen/meurpg/progression/v1/progression_pb';
import type { LoadState } from '../../core/progression/experience-store';
import { moreTreasures, stripHeadline, stripLine } from '../../core/progression/treasure';

/** How many treasures the strip names one by one; the rest is "e mais N". */
const LINES = 5;

/**
 * The treasure strip of the master's XP screens (E9-09, MR-041): what the party
 * found and no award converted yet. In a campaign by gold it says "Encontrado,
 * ainda não convertido" with the count, the PO and each find, and (when `button`
 * is set, in "Dar XP") the outlined "Voltar à cidade" that opens the
 * conversion; empty, it invites the next find. In a campaign by enemies the
 * treasure never becomes XP: the strip exists only when there is one, and says
 * so ("Tesouro encontrado") with the line why. A campaign by milestones has no
 * strip at all (the host draws none). The list is the server's: at most 100
 * finds, and "Há mais N" when `total` says there are more.
 */
@Component({
  selector: 'app-treasure-strip',
  imports: [MatButtonModule, MatIconModule],
  template: `
    @if (visible()) {
      <section class="strip" [class.strip--compact]="compact()" [attr.aria-label]="label()">
        <mat-icon class="strip__icon" aria-hidden="true">currency_exchange</mat-icon>
        <div class="strip__body">
          <p class="strip__label">{{ label() }}</p>
          @if (state() === 'loading' && treasures().length === 0) {
            <p class="strip__line" role="status">Lendo os tesouros encontrados...</p>
          } @else if (state() === 'error' && treasures().length === 0) {
            <p class="strip__line">
              Não foi possível ler os tesouros.
              <button mat-button type="button" class="strip__retry" (click)="retry.emit()">Tentar de novo</button>
            </p>
          } @else if (treasures().length === 0) {
            <p class="strip__line">Nenhum tesouro esperando. Os que o grupo encontrar aparecem aqui.</p>
          } @else {
            <p class="strip__big">{{ headline() }}</p>
            <ul class="strip__list">
              @for (t of shown(); track t.pointId) {
                <li>{{ line(t) }}</li>
              }
            </ul>
            @if (hidden() > 0) {
              <p class="strip__more">e mais {{ hidden() }}</p>
            }
            @if (extra() > 0) {
              <p class="strip__more">{{ moreText() }}</p>
            }
            @if (!gold()) {
              <p class="strip__why">
                Esta campanha dá XP por inimigos, então o tesouro não vira XP. Ele aparece no resumo de cada sessão.
              </p>
            }
          }
        </div>
        @if (button() && gold()) {
          <button mat-stroked-button type="button" class="strip__go" (click)="town.emit()">
            <mat-icon aria-hidden="true">currency_exchange</mat-icon>Voltar à cidade
          </button>
        }
      </section>
    }
  `,
  styleUrl: './treasure-strip.scss',
})
export class TreasureStrip {
  readonly treasures = input.required<readonly TreasureToConvert[]>();
  /** How many treasures are waiting in all (the list stops at 100). */
  readonly total = input(0);
  readonly mode = input.required<XpMode>();
  readonly state = input<LoadState>('ready');
  /** The outlined "Voltar à cidade" beside the text ("Dar XP"'s strip). */
  readonly button = input(false);
  /** A phone's sheet: the headline and the button only, so the form under it keeps its room. */
  readonly compact = input(false);

  readonly town = output<void>();
  readonly retry = output<void>();

  protected readonly gold = computed(() => this.mode() === XpMode.GOLD);
  protected readonly visible = computed(
    () => this.gold() || (this.mode() === XpMode.ENEMIES && this.treasures().length > 0),
  );
  protected readonly label = computed(() => (this.gold() ? 'Encontrado, ainda não convertido' : 'Tesouro encontrado'));
  protected readonly headline = computed(() => stripHeadline(this.treasures()));
  protected readonly shown = computed(() => this.treasures().slice(0, LINES));
  protected readonly hidden = computed(() => Math.max(0, this.treasures().length - LINES));
  protected readonly moreText = computed(() => moreTreasures(this.extra(), 'para depois'));
  protected readonly extra = computed(() => Math.max(0, this.total() - this.treasures().length));
  protected readonly line = stripLine;
}
