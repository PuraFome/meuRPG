import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { JumpLimits } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { type JumpMode, limitFor, runSeal } from '../../../../core/combat/jump-plan';
import { metersFixed } from '../../../../core/units';

/**
 * What a jump can do this turn (E9-06): the two limits from the sheet's Força
 * ("Distância 4,8 m com corrida · 2,4 m parado", "Altura 1,8 m ..."), the seal
 * that says which applies ("Com corrida: você andou 3,0 m a pé antes de
 * saltar", "Parado: ..."), and, for a high jump, the stepper. Every number is
 * the server's (`JumpLimits`); nothing is worked out here but the pick of the
 * running or the standing one, as the server's `running_start` says.
 */
@Component({
  selector: 'app-jump-panel',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <dl class="lim">
      <div class="lim__row">
        <dt>Distância</dt>
        <dd><b>{{ long().running }}</b> com corrida · <b>{{ long().standing }}</b> parado</dd>
      </div>
      <div class="lim__row">
        <dt>Altura</dt>
        <dd><b>{{ high().running }}</b> com corrida · <b>{{ high().standing }}</b> parado</dd>
      </div>
    </dl>
    <p class="seal" role="status">
      <span class="seal__word"><mat-icon aria-hidden="true">{{ limits().runningStart ? 'directions_run' : 'accessibility_new' }}</mat-icon>{{ seal().word }}</span>
      {{ seal().reason }}
    </p>
    @if (kind() === 'high') {
      <div class="high">
        <span class="high__cap" id="high-cap">Altura do salto</span>
        <div class="high__row">
          <button
            type="button"
            class="high__btn"
            aria-label="Diminuir a altura em 0,3 m"
            [attr.aria-disabled]="atMin() ? 'true' : null"
            (click)="step.emit(-1)"
          >
            <mat-icon aria-hidden="true">remove</mat-icon>
          </button>
          <span class="high__val" role="status" aria-live="polite" aria-labelledby="high-cap">{{ heightText() }}</span>
          <button
            type="button"
            class="high__btn"
            aria-label="Aumentar a altura em 0,3 m"
            [attr.aria-disabled]="atMax() ? 'true' : null"
            (click)="step.emit(1)"
          >
            <mat-icon aria-hidden="true">add</mat-icon>
          </button>
        </div>
      </div>
    }
  `,
  styleUrl: './jump-panel.scss',
})
export class JumpPanel {
  readonly limits = input.required<JumpLimits>();
  readonly kind = input.required<JumpMode>();
  /** The high jump's height, in tenths of a foot. */
  readonly height = input(0);
  /** The bounds of the stepper, from `maxHeight`. */
  readonly atMin = input(false);
  readonly atMax = input(false);
  /** The stepper moved one step: `+1` or `-1`. */
  readonly step = output<1 | -1>();

  protected readonly long = computed(() => ({ running: metersFixed(this.limits().longRunningDft / 10), standing: metersFixed(this.limits().longStandingDft / 10) }));
  protected readonly high = computed(() => ({ running: metersFixed(this.limits().highRunningDft / 10), standing: metersFixed(this.limits().highStandingDft / 10) }));
  protected readonly seal = computed(() => runSeal(this.limits()));
  protected readonly heightText = computed(() => metersFixed(this.height() / 10));
  /** For the page: the limit that applies to the picked kind. */
  readonly applies = computed(() => limitFor(this.limits(), this.kind()));
}
