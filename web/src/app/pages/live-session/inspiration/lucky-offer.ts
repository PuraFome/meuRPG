import { ChangeDetectionStrategy, Component, computed, input, model, output } from '@angular/core';

import type { LuckyOffer } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { LuckyRoll } from '../../../core/resources/resources-client';
import { LuckyPrompt } from '../combat/lucky-prompt/lucky-prompt';

/**
 * The Lucky question of a roll that is not a weapon attack (a scene check, a group check, a trap search, a puzzle hint try),
 * in the place `InspirationOffer` takes for the Bardic Inspiration die: the d20 already rolled ("Seu d20", both faces with
 * advantage or disadvantage, so the 1 is there to see), and under it the question itself (`LuckyPrompt`: "Tirou 1: rolar de
 * novo?", "Rolar de novo" or "Ficar com o 1"). The attack sheet shows its d20 in its own result card, so it uses the prompt
 * alone. Nothing about the result is told until the player answers.
 */
@Component({
  selector: 'app-lucky-offer',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [LuckyPrompt],
  template: `
    <div class="offer">
      <p class="d20">
        <span class="d20__face">{{ faces() }}</span>
        <span class="d20__text">Seu d20{{ bonusText() }}. O resultado só aparece depois da sua escolha.</span>
      </p>
      <app-lucky-prompt
        [offer]="offer()"
        [canApp]="canApp()"
        [canType]="canType()"
        [preferApp]="preferApp()"
        [busy]="busy()"
        [(typing)]="typing"
        (reroll)="reroll.emit($event)"
        (keep)="keep.emit()"
      />
    </div>
  `,
  styles: `
    .offer { display: grid; gap: 12px; }
    .d20 { display: flex; align-items: center; gap: 12px; margin: 0; }
    .d20__face { font: var(--mr-type-title, 600 28px/1 system-ui); min-width: 2.2ch; text-align: center; }
  `,
})
export class LuckyOfferCard {
  /** The question: the d20 with the die that shows the 1. */
  readonly offer = input.required<LuckyOffer>();
  readonly canApp = input(true);
  readonly canType = input(true);
  readonly preferApp = input(true);
  readonly busy = input(false);
  /** Typing mode, two-way, for a sheet that changes its title while the player types. */
  readonly typing = model(false);
  /** The player rolls the d20 again: rolled by the app, or the face of the real die. */
  readonly reroll = output<LuckyRoll>();
  /** The player keeps the 1. */
  readonly keep = output<void>();

  /** The d20 as it came up: "1", or "1 e 13" for a roll with advantage or disadvantage. */
  protected readonly faces = computed(() => (this.offer().d20?.faces ?? []).join(' e '));
  protected readonly bonusText = computed(() => {
    const m = this.offer().d20?.modifier ?? 0;
    return m === 0 ? '' : ` (${m > 0 ? '+' : '−'}${Math.abs(m)} de bônus)`;
  });
}
