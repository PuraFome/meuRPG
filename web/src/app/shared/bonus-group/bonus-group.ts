import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

let nextId = 0;

/** "Nenhum", "+1", "+2", "+3": the value each word stands for. */
const OPTIONS: readonly { readonly value: number; readonly word: string }[] = [
  { value: 0, word: 'Nenhum' },
  { value: 1, word: '+1' },
  { value: 2, word: '+2' },
  { value: 3, word: '+3' },
];

/**
 * The magic bonus of a piece as a radio group, "Bônus mágico": four equal segments in one bordered row.
 * The chosen one has a check and the soft accent; `disabled` mutes the row and makes it inert while a save runs.
 */
@Component({
  selector: 'app-bonus-group',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './bonus-group.html',
  styleUrl: './bonus-group.scss',
})
export class BonusGroup {
  /** The bonus now, 0 to 3; 0 is no bonus. */
  readonly value = input(0);
  readonly disabled = input(false);
  readonly valueChange = output<number>();

  protected readonly options = OPTIONS;
  protected readonly name = `bonus-group-${nextId++}`;
}
