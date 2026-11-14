import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { type MarksVm } from '../../core/characters/gear';
import { BonusChip } from '../bonus-chip/bonus-chip';

/** The chips of one mark as the legend draws them, in the grid's order. */
const MARKS = [
  { key: 'silvered', word: 'Prateada', icon: 'dark_mode' },
  { key: 'adamantine', word: 'Adamantina', icon: 'diamond' },
  { key: 'magical', word: 'Mágica', icon: 'auto_awesome' },
] as const;

/**
 * The line that says what the chips on the rows beside it mean: the marks that appear (a round icon and its word), the two
 * attunement chips, and the magic bonus chip with its words. One piece for the sheet's "Equipamento" and the combat's "Ataques"
 * tab; the host passes only what its rows draw, and nothing is drawn when that is nothing. With `decorative` the line is hidden
 * from a screen reader: the chips on the rows already carry their words.
 */
@Component({
  selector: 'app-chip-legend',
  imports: [BonusChip, MatIconModule],
  templateUrl: './chip-legend.html',
  styleUrl: './chip-legend.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ChipLegend {
  /** The marks some row shows (one entry per row; the union is drawn). */
  readonly marks = input<readonly (MarksVm | null | undefined)[]>([]);
  /** Some row carries an attunement chip. */
  readonly attunement = input(false);
  /** The bonus chip on the rows: solid when some bonus counts now, dashed when none does, absent for none. */
  readonly bonus = input<'' | 'counts' | 'idle'>('');
  readonly decorative = input(false);

  protected readonly shown = computed(() =>
    MARKS.filter((m) => this.marks().some((marks) => marks?.[m.key])),
  );
  protected readonly any = computed(
    () => this.shown().length > 0 || this.attunement() || this.bonus() !== '',
  );
}
