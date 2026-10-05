import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { TrapActivity as TrapActivityMessage } from '../../../../../gen/meurpg/play/v1/traps_pb';
import { activityLines } from '../../../../core/traps/trap-log';

/**
 * "Registro" (E9-08 5, F, MR-035): what traps did in the session outside a combat, newest first: the firings
 * (with who fell and what happened), the searches (the dice, and what was found) and the traps noticed by
 * passing. The master reads every line with the dice; a player only the lines of their own characters,
 * with their own dice and "passou" or "falhou", never a DC and never a trap their characters do not know
 * (the server decides, RN-10). In a combat the same lines are in the combat log.
 */
@Component({
  selector: 'app-trap-activity',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (lines().length > 0) {
      <section class="ta mr-panel" aria-labelledby="ta-h">
        <h3 class="ta__h" id="ta-h">Registro</h3>
        <ul class="ta__list" aria-live="polite" aria-relevant="additions">
          @for (l of lines(); track l.id) {
            <li class="ta__row">
              <mat-icon class="ta__icon" aria-hidden="true">{{ l.icon }}</mat-icon>
              <span class="ta__text">
                @if (l.actor) {
                  <b>{{ l.actor }}</b>
                }{{ l.text }}
              </span>
            </li>
          }
        </ul>
      </section>
    }
  `,
  styles: `
    :host {
      display: contents;
    }

    .ta {
      margin-top: var(--mr-space-4);
    }

    .ta__h {
      margin: 0 0 var(--mr-space-2);
      font-family: var(--mr-font-display, inherit);
      font-size: 20px;
      line-height: 26px;
    }

    .ta__list {
      margin: 0;
      padding: 0;
      list-style: none;
    }

    .ta__row {
      display: flex;
      align-items: flex-start;
      gap: var(--mr-space-3);
      padding: 10px 0;
      font-size: 15px;
      line-height: 21px;
    }

    .ta__row + .ta__row {
      border-top: 1px solid var(--mr-rule);
    }

    .ta__icon {
      flex: none;
      width: 20px;
      height: 20px;
      margin-top: 1px;
      font-size: 20px;
    }

    .ta__text {
      min-width: 0;
      overflow-wrap: anywhere;
    }
  `,
})
export class TrapActivityList {
  readonly activity = input.required<readonly TrapActivityMessage[]>();
  readonly master = input(false);

  protected readonly lines = computed(() =>
    [...this.activity()].reverse().flatMap((a) => activityLines(a, this.master())),
  );
}
