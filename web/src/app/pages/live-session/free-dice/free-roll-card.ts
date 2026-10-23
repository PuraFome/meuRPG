import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import {
  type FreeRollVm,
  audienceText,
  naturalWord,
  resultLine,
} from '../../../core/free-dice/free-roll';

/** More dice than this wrap into lines under the sum, instead of sitting beside it. */
const MANY_DICE = 6;

/**
 * The result of a free roll (B5-06 states 2 to 6): the big total, the formula with the sum, a box for each die (the d20
 * that advantage or disadvantage left out is dashed and says "descartado"), and "Rolagem livre · para a mesa" or the
 * master's "Só você" badge. A natural 20 or 1 is in the words of the formula line, never in colour alone. The
 * sentence a screen reader hears is a polite status, so the result is read when it lands.
 */
@Component({
  selector: 'app-free-roll-card',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <div
      class="card"
      [class.card--many]="many()"
      [class.card--secret]="roll().secret"
      [class.card--natural]="natural() !== null"
    >
      <span class="card__total" aria-hidden="true">{{ roll().total }}</span>
      <div class="card__body">
        <p class="card__line">{{ line() }}</p>
        <div class="dice" aria-hidden="true">
          @for (d of roll().dice; track $index) {
            <span class="die" [class.die--discarded]="d.discarded">
              <span class="die__box">{{ d.face }}</span>
              @if (d.discarded) {
                <span class="die__note">descartado</span>
              }
            </span>
          }
        </div>
        <p class="card__who">
          Rolagem livre ·
          @if (roll().secret) {
            <span class="badge"><mat-icon aria-hidden="true">visibility_off</mat-icon>Só você</span>
          } @else {
            {{ audience() }}
          }
        </p>
      </div>
    </div>
    <p class="mr-visually-hidden" role="status">{{ spoken() }}</p>
  `,
  styleUrl: './free-roll-card.scss',
})
export class FreeRollCard {
  readonly roll = input.required<FreeRollVm>();

  protected readonly line = computed(() => resultLine(this.roll()));
  protected readonly natural = computed(() => naturalWord(this.roll()));
  protected readonly audience = computed(() => audienceText(this.roll()));
  protected readonly many = computed(() => this.roll().dice.length > MANY_DICE);
  /** "Resultado 11. 2d6 + 3 = 11. Rolagem livre, para a mesa." */
  protected readonly spoken = computed(
    () => `Resultado ${this.roll().total}. ${this.line()}. Rolagem livre, ${this.audience()}.`,
  );
}
