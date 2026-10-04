import { Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { STEP_LABELS, type StepKey } from '../../../core/levelup/levelup-flow';

/**
 * The steps of the guided level-up as a numbered list (MR-040, E8-15): from a tablet up, the
 * numbers with their words and a line between them, the done ones with a tick and the
 * current one bold with a filled number; on a phone, one segment per step (the words are
 * read by "Passo 3 de 4 · Magias" under it, and by a screen reader from the list). The
 * steps are not buttons: "Voltar" and "Próximo" move between them, so a step cannot
 * be skipped over a missing choice.
 */
@Component({
  selector: 'app-steps-bar',
  imports: [MatIconModule],
  templateUrl: './steps-bar.html',
  styleUrl: './steps-bar.scss',
})
export class StepsBar {
  readonly steps = input.required<readonly StepKey[]>();
  readonly index = input.required<number>();
  protected readonly labels = STEP_LABELS;
}
