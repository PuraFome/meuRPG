import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { AskTone, NpcCardVm } from '../../../core/npc-check/npc-check-card';

/** The icon of each row's tag: never colour alone. */
const TAG_ICONS: Readonly<Record<AskTone, string>> = {
  waiting: 'hourglass_empty',
  queue: '',
  cut: 'content_cut',
  pass: '',
  closed: '',
};

/**
 * The master's card of an NPC's check (591-02, 591-06): the creature and the check, the number in a box, how it came, and one row
 * for each bard asked with what each answered. While a bard still has the question it offers "Continuar sem esperar"; once the
 * check is settled it has the "X" that hides it on this device only. It is for the master alone and says so.
 */
@Component({
  selector: 'app-npc-check-card',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './npc-check-card.html',
  styleUrl: './npc-check-card.scss',
})
export class NpcCheckCard {
  readonly card = input.required<NpcCardVm>();
  readonly busy = input(false);
  readonly error = input('');
  readonly skip = output<void>();
  readonly dismiss = output<void>();
  protected readonly icons = TAG_ICONS;
}
