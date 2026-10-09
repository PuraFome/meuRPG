import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { type AdvantageSource, RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import type { CheckFace } from '../../../../core/play/check-roll';
import { modeWord, orNormal } from '../../../../core/combat/roll-mode';

/**
 * How a check's d20 was rolled: the mode in words ("Vantagem"), the two dice with the one that counts marked
 * "conta" and the other "não conta" (never colour alone), and the circumstances behind the mode, one sentence
 * each. Nothing is drawn for a Normal roll with one die and no source.
 */
@Component({
  selector: 'app-check-mode',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (shown()) {
      <div class="mode">
        @if (pair()) {
          <p class="mode__word">{{ word() }}</p>
          <ul class="mode__dice" aria-label="Os dois dados">
            @for (f of faces(); track $index) {
              <li class="die" [class.die--counts]="f.counts">
                <b class="die__n">{{ f.value }}</b>{{ ' ' }}
                <span class="die__cap">{{ f.counts ? 'conta' : 'não conta' }}</span>
              </li>
            }
          </ul>
        } @else if (word() !== 'Normal') {
          <p class="mode__word">{{ word() }}</p>
        }
        @if (sources().length) {
          <ul class="mode__sources" aria-label="Por que">
            @for (s of sources(); track $index) {
              <li>{{ s.textPt }}</li>
            }
          </ul>
        }
      </div>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .mode {
      display: flex;
      flex-direction: column;
      gap: 6px;
      margin-top: 8px;
    }

    .mode__word {
      margin: 0;
      font-weight: 700;
    }

    .mode__dice,
    .mode__sources {
      display: flex;
      flex-wrap: wrap;
      gap: 6px 10px;
      margin: 0;
      padding: 0;
      list-style: none;
    }

    .mode__sources {
      flex-direction: column;
      gap: 2px;
      font-size: 14px;
      line-height: 20px;
      color: var(--mr-ink-muted);
    }

    .die {
      display: inline-flex;
      align-items: baseline;
      gap: 6px;
      padding: 2px 10px;
      border: 1px dashed var(--mr-control-line);
      border-radius: 10px;
      color: var(--mr-ink-muted);
    }

    .die--counts {
      border-style: solid;
      border-color: var(--mr-ink);
      color: var(--mr-ink);
    }

    .die__n {
      font-size: 20px;
    }

    .die__cap {
      font-size: 14px;
    }
  `,
})
export class CheckMode {
  readonly mode = input<RollMode>(RollMode.NORMAL);
  readonly sources = input<readonly AdvantageSource[]>([]);
  readonly faces = input<readonly CheckFace[]>([]);

  protected readonly word = computed(() => modeWord(orNormal(this.mode())));
  protected readonly pair = computed(() => this.faces().length > 1);
  protected readonly shown = computed(
    () => this.pair() || this.word() !== 'Normal' || this.sources().length > 0,
  );
}
