import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import type { CheckRoll } from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import {
  checkNaturalMark,
  countedFace,
  modeLabel,
  noteLines,
  pairOf,
  rollFormula,
} from '../../../../core/combat/contest-view';

/**
 * The player's own roll of a contest, of Hide or of a group check (W7-X): the d20 that counts in a box, the total beside it
 * and the formula under it ("1d20 (15) + 5 · Atletismo"). A roll with advantage or disadvantage says the mode, shows the two
 * dice with the one that counts marked "vale" and the other "descartado" (never colour alone), and the circumstances behind it,
 * one sentence each ("Vantagem: Ajuda de Orla"). Only the player's own roll is ever given to it: a total of an NPC never reaches a
 * player (RN-20).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-contest-roll',
  template: `
    <div class="roll" role="status" aria-live="polite">
      <span class="roll__box" aria-hidden="true">{{ face() }}</span>
      <span class="roll__text">
        <span class="roll__total">{{ roll().total }}</span>
        <span class="roll__formula">{{ formula() }}</span>
        @if (natural()) {
          <span class="mr-tag roll__natural" data-testid="natural-mark">{{ natural() }}</span>
        }
      </span>
      <span class="mr-visually-hidden"
        >Seu total: {{ roll().total }}. {{ formula() }}.{{ natural() ? ' ' + natural() + '.' : '' }}</span
      >
    </div>
    @if (mode() || pair().length || notes().length) {
      <ul class="why" aria-label="Como foi rolado">
        @if (mode()) {
          <li class="why__mode">{{ mode() }}</li>
        }
        @if (pair().length) {
          <li class="why__dice">
            @for (d of pair(); track $index) {
              <span class="die" [class.die--counts]="d.counts"
                ><b>{{ d.value }}</b> {{ d.counts ? 'vale' : 'descartado' }}</span
              >
            }
          </li>
        }
        @for (n of notes(); track $index) {
          <li>{{ n }}</li>
        }
      </ul>
    }
    @if (roll().maneuverDieSides > 0) {
      <p class="by" data-testid="maneuver-die">
        {{ roll().maneuverNamePt }}: d{{ roll().maneuverDieSides }} ({{ roll().maneuverDieFace }}) já está no total.
      </p>
    }
    @if (roll().rolledByMaster) {
      <p class="by">O mestre rolou por você.</p>
    }
  `,
  styleUrl: './contest-roll.scss',
})
export class ContestRoll {
  readonly roll = input.required<CheckRoll>();
  /** "Atletismo": said after the formula when the roller chose the skill. */
  readonly skill = input('');

  protected readonly face = computed(() => countedFace(this.roll()));
  protected readonly formula = computed(() => {
    const base = rollFormula(this.roll());
    const physical = this.roll().physical ? ' · dado físico' : '';
    return `${base}${this.skill() ? ` · ${this.skill()}` : ''}${physical}`;
  });
  /** "20 natural" or "1 natural": information only, a check has no automatic success or failure (SRD 5.1). */
  protected readonly natural = computed(() => checkNaturalMark(this.roll()));
  protected readonly mode = computed(() => modeLabel(this.roll().mode));
  protected readonly pair = computed(() => pairOf(this.roll()));
  protected readonly notes = computed(() => noteLines(this.roll()));
}
