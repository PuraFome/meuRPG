import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Step } from '../../../../core/combat/attack-flow';

/** The stepper of the attack sheet (E6-07): Alvo, Rolar, Dano. A done step
 * has a check, the current one `aria-current="step"`; the number is never
 * the only sign, the name is always beside it. */
@Component({
  selector: 'app-attack-steps',
  imports: [MatIconModule],
  template: `
    <ol class="steps" aria-label="Passos do ataque">
      @for (s of steps(); track s.name; let i = $index) {
        <li
          class="steps__item"
          [class.steps__item--todo]="s.state === 'todo'"
          [attr.aria-current]="s.state === 'current' ? 'step' : null"
        >
          <span class="steps__dot" [class.steps__dot--done]="s.state === 'done'" [class.steps__dot--now]="s.state === 'current'">
            @if (s.state === 'done') {
              <mat-icon aria-hidden="true">check</mat-icon>
            } @else {
              {{ i + 1 }}
            }
          </span>
          <span class="steps__name">{{ s.name }}</span>
        </li>
      }
    </ol>
  `,
  styleUrl: './attack-steps.scss',
})
export class AttackSteps {
  readonly steps = input.required<readonly Step[]>();
}
