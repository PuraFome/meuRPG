import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { HitCardView } from '../../../../core/effects/hit-effect-text';

let nextId = 0;

/**
 * The effect waiting for the caster's next weapon hit (#577, boards 577-05 and 577-06): at the top of "Ataques", right under
 * the economy row and the tabs, one card for each spell whose damage rides on the hit. The name (with the marked creature),
 * a chip "Concentração" when the spell needs it, and one line: when it applies, the dice, the saving throw and the time left.
 * Only the caster's player is given these numbers; the others read the mark's name on the creature and nothing else. Each card
 * is a `role="group"` named by its title, and one that was spent leaves the list.
 */
@Component({
  selector: 'app-hit-effect-cards',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    @for (c of cards(); track c.key; let i = $index) {
      <section class="hit" data-testid="hit-effect-card" role="group" [attr.aria-labelledby]="id + '-' + i">
        <div class="hit__head">
          <mat-icon class="hit__icon" aria-hidden="true">target</mat-icon>
          <h3 class="hit__title" [id]="id + '-' + i">{{ c.title }}</h3>
          @if (c.concentration) {
            <span class="hit__chip"><mat-icon aria-hidden="true">hourglass_empty</mat-icon>Concentração</span>
          }
        </div>
        <p class="hit__line">{{ c.line }}</p>
      </section>
    }
  `,
  styleUrl: './hit-effect-cards.scss',
})
export class HitEffectCards {
  readonly cards = input<readonly HitCardView[]>([]);

  protected readonly id = `hit-${nextId++}`;
}
