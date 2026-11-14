import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { bonusChipName } from '../../core/characters/gear';

/** The magic bonus of a piece as a small pill, "+N" with a spark: solid when the bonus is added to the numbers now,
 * dashed and muted when it is not. With `words` the pill is followed by the words saying the same. */
@Component({
  selector: 'app-bonus-chip',
  imports: [MatIconModule],
  templateUrl: './bonus-chip.html',
  styleUrl: './bonus-chip.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class BonusChip {
  readonly bonus = input.required<number>();
  readonly counts = input(true);
  readonly words = input(false);

  protected readonly name = computed(() => bonusChipName(this.bonus(), this.counts()));
  protected readonly label = computed(() => bonusChipName(this.bonus(), this.counts(), true));
}
