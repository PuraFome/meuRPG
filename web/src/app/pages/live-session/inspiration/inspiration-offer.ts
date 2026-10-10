import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { OutsideInspirationOffer } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { InspirationRoll } from '../../../core/resources/resources-client';

/**
 * The question a roll outside a combat asks when its character holds a Bardic Inspiration die (SRD 5.1: the creature
 * "can wait until after it rolls the d20 before deciding", but must decide before the DM says whether the roll
 * succeeds): the d20 already rolled, and "Usar a Inspiração de Bardo (d8)?". Using it rolls the die in the app, or takes
 * the face of a real die; keeping it leaves the die for the next roll. The result is not shown until it is answered.
 */
@Component({
  selector: 'app-inspiration-offer',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule],
  template: `
    <section class="offer" aria-labelledby="offer-q">
      <p class="d20">
        <span class="d20__face">{{ d20() }}</span>
        <span class="d20__text">Seu d20{{ bonusText() }}. O resultado só aparece depois da sua escolha.</span>
      </p>
      <h3 id="offer-q" class="q">Usar a Inspiração de Bardo (d{{ offer().sides }})?</h3>
      @if (offer().fromName) {
        <p class="small">Dada por {{ offer().fromName }}. Some o dado a esta rolagem; depois dele, o dado se perde.</p>
      }
      @if (typing()) {
        <label class="typed">
          <span>Número que saiu no d{{ offer().sides }} (1 a {{ offer().sides }})</span>
          <input
            type="number"
            inputmode="numeric"
            [min]="1"
            [max]="offer().sides"
            [value]="face()"
            (input)="face.set($any($event.target).valueAsNumber)"
          />
        </label>
      }
      <div class="btns">
        @if (canApp()) {
          <button matButton="filled" type="button" [disabled]="busy()" disabledInteractive (click)="use.emit({ inApp: true })">
            <mat-icon aria-hidden="true">music_note</mat-icon>Usar e rolar o d{{ offer().sides }}
          </button>
        }
        @if (canType() && !typing()) {
          <button matButton="outlined" type="button" [disabled]="busy()" disabledInteractive (click)="typing.set(true)">
            Digitar o dado
          </button>
        }
        @if (typing()) {
          <button
            matButton="filled"
            type="button"
            [disabled]="busy() || !faceOk()"
            disabledInteractive
            (click)="use.emit({ face: face() })"
          >
            Usar o dado digitado
          </button>
        }
        <button matButton="outlined" type="button" [disabled]="busy()" disabledInteractive (click)="keep.emit()">
          Guardar o dado
        </button>
      </div>
    </section>
  `,
  styles: `
    .offer { display: grid; gap: 12px; }
    .d20 { display: flex; align-items: center; gap: 12px; margin: 0; }
    .d20__face { font: var(--mr-type-title, 600 28px/1 system-ui); min-width: 2.2ch; text-align: center; }
    .q { margin: 0; font-size: 1.05rem; }
    .small { margin: 0; color: var(--mr-text-muted, inherit); font-size: 0.875rem; }
    .typed { display: grid; gap: 4px; }
    .typed input { min-height: 44px; font-size: 1rem; padding: 0 12px; }
    .btns { display: grid; gap: 8px; }
    @media (min-width: 600px) { .btns { grid-auto-flow: column; justify-content: start; } }
  `,
})
export class InspirationOffer {
  readonly offer = input.required<OutsideInspirationOffer>();
  readonly canApp = input(true);
  readonly canType = input(true);
  readonly busy = input(false);
  readonly use = output<InspirationRoll>();
  readonly keep = output<void>();

  protected readonly typing = signal(false);
  protected readonly face = signal(Number.NaN);
  protected readonly faceOk = computed(() => {
    const f = this.face();
    return Number.isInteger(f) && f >= 1 && f <= this.offer().sides;
  });
  /** The d20 that counts. */
  protected readonly d20 = computed(() => {
    const roll = this.offer().d20;
    return roll?.faces[roll.countedIndex] ?? roll?.faces[0] ?? 0;
  });
  protected readonly bonusText = computed(() => {
    const m = this.offer().d20?.modifier ?? 0;
    return m === 0 ? '' : ` (${m > 0 ? '+' : '−'}${Math.abs(m)} de bônus)`;
  });
}
