import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { InspirationDie } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { dieCard } from '../../../../core/resources/bardic-inspiration';

/**
 * The die card of Bardic Inspiration (PM-07c 12, middle): "Inspiração de Bardo: d8" over "de Orla · até 8 min", on the
 * sheet and in the player's turn. Only the die's holder and the master are sent the die; the time left comes from the
 * round the combat is in (the die lasts 100 rounds, which the server counts).
 */
@Component({
  selector: 'app-inspiration-card',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <mat-icon class="card__icon" aria-hidden="true">music_note</mat-icon>
    <span class="card__text">
      <b class="card__title">{{ card().title }}</b>
      <span class="card__sub">{{ card().sub }}</span>
    </span>
  `,
  styles: `
    :host {
      display: flex;
      align-items: center;
      gap: var(--mr-space-3);
      box-sizing: border-box;
      padding: 10px 14px;
      border: 1px solid var(--mr-accent);
      border-radius: var(--mr-radius-md);
      background: var(--mr-accent-soft);
    }

    .card__icon {
      flex: none;
      color: var(--mr-accent);
    }

    .card__text {
      display: flex;
      flex-direction: column;
      min-width: 0;
    }

    .card__title {
      font-size: 16px;
      line-height: 21px;
    }

    .card__sub {
      font-size: 14px;
      line-height: 19px;
      color: var(--mr-ink-muted);
    }
  `,
})
export class InspirationCard {
  readonly die = input.required<InspirationDie>();
  /** The combat's round now. */
  readonly round = input.required<number>();
  protected readonly card = computed(() => dieCard(this.die(), this.round()));
}
