import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import type { DiceRoll } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { type AdvantageSource, RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { d20Faces, extraDieLines, modeWord, sourceLine } from '../../../../core/combat/roll-mode';

/**
 * The two d20 of a roll with advantage or disadvantage: the one that counts is
 * large and marked "vale"; the other is struck through and marked "descartado".
 * The mode in words ("Vantagem") comes first, then the reason the roller gave
 * and the circumstances behind it. Nothing is drawn for a normal single d20.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-d20-faces',
  template: `
    @if (pair()) {
      <span class="d20">
        <span class="d20__mode">{{ caption() ? caption() + ': ' : '' }}{{ word() }}</span>
        <span class="d20__faces" role="group" [attr.aria-label]="label()">
          @for (f of faces(); track $index) {
            <span class="face" [class.face--counted]="f.counted" [class.face--dropped]="!f.counted">
              <span class="face__n">{{ f.face }}</span>
              <span class="face__cap">{{ f.counted ? 'vale' : 'descartado' }}</span>
            </span>
          }
        </span>
      </span>
    }
    @if (differs()) {
      <span class="d20__reason">O app sugeria {{ suggestedWord() }}.</span>
    }
    @if (reason()) {
      <span class="d20__reason">Motivo: “{{ reason() }}”</span>
    }
    @for (s of sources(); track $index) {
      <span class="d20__src">{{ sourceLine(s) }}</span>
    }
    @for (line of extraLines(); track $index) {
      <span class="d20__src">{{ line }}</span>
    }
  `,
  styleUrl: './d20-faces.scss',
})
export class D20Faces {
  readonly roll = input<DiceRoll | null | undefined>(null);
  readonly mode = input<RollMode>(RollMode.NORMAL);
  readonly reason = input('');
  /** "Ataque" or "Resistência", when a result has more than one d20. */
  readonly caption = input('');
  /** What the app suggested: said when the roll used another mode. */
  readonly suggested = input<RollMode>(RollMode.UNSPECIFIED);
  readonly sources = input<readonly AdvantageSource[]>([]);

  protected readonly faces = computed(() => {
    const r = this.roll();
    return r ? d20Faces(r) : [];
  });
  protected readonly pair = computed(() => this.faces().length > 1);
  protected readonly word = computed(() => modeWord(this.mode()));
  protected readonly suggestedWord = computed(() => modeWord(this.suggested()));
  protected readonly differs = computed(
    () => this.suggested() !== RollMode.UNSPECIFIED && this.suggestedWord() !== this.word(),
  );
  protected readonly label = computed(() =>
    this.faces()
      .map((f) => `${f.face} ${f.counted ? 'vale' : 'descartado'}`)
      .join(', '),
  );
  protected readonly extraLines = computed(() => extraDieLines(this.roll(), this.sources()));
  protected sourceLine = sourceLine;
}
