import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';

import { PuzzleAlphabet } from '../../../../gen/meurpg/play/v1/puzzles_pb';
import { type Draft, alphabetSize, clamped, resized } from '../../../core/puzzles/puzzle-draft';
import { alphabetFaces } from '../../../core/puzzles/puzzle-symbols';
import { LockBoard } from '../../../shared/puzzle-boards/lock-board';
import { SecretPill } from '../../../shared/puzzle-boards/secret-pill';
import type { Turn } from '../../../shared/puzzle-boards/symbol-columns';
import { type PickOption, PickGroup } from '../fields/pick-group';
import { Stepper } from '../fields/stepper';

const ALPHABETS: readonly PickOption<PuzzleAlphabet>[] = [
  { value: PuzzleAlphabet.DIGITS, title: 'Dígitos' },
  { value: PuzzleAlphabet.LETTERS, title: 'Letras' },
  { value: PuzzleAlphabet.RUNES, title: 'Runas' },
];

/**
 * The form of the combination lock (E10-06 state 2): how many wheels (2 to 6), what they show (digits, letters or our runes),
 * the solution and where the wheels start. The master turns the wheels with the arrows: the solution is "Só você vê",
 * and "Recomeçar" in a session always goes back to the start chosen here. The start must differ from the solution (the form says so;
 * the server checks too).
 */
@Component({
  selector: 'app-lock-form',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [LockBoard, PickGroup, SecretPill, Stepper],
  template: `
    <app-stepper label="Rodas" noun="roda" [value]="draft().wheels" [min]="2" [max]="6" (valueChange)="setWheels($event)" />
    <app-pick-group legend="Alfabeto" layout="segments" [options]="alphabets" [value]="draft().alphabet" (valueChange)="setAlphabet($event)" />

    <section class="part" aria-labelledby="lock-solution">
      <h3 class="part__title"><span id="lock-solution">A solução</span><app-secret-pill /></h3>
      <app-lock-board mode="edit" label="Solução da fechadura" [wheels]="draft().lockSolution" [faces]="faces()" (turn)="turnSolution($event)" />
    </section>

    <section class="part" aria-labelledby="lock-start">
      <h3 class="part__title" id="lock-start">O começo</h3>
      <app-lock-board mode="edit" label="Começo da fechadura" [wheels]="draft().lockStart" [faces]="faces()" (turn)="turnStart($event)" />
      <p class="part__help">Você escolhe onde as rodas começam. “Recomeçar” volta sempre a este começo.</p>
      @if (error()) {
        <p class="part__error" role="alert">{{ error() }}</p>
      }
    </section>
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-4);
    }

    .part {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);
    }

    .part__title {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: var(--mr-space-2);
      margin: 0;
      font-size: 16px;
      font-weight: 700;
    }

    .part__help {
      margin: 0;
      font-size: 14px;
      line-height: 20px;
      color: var(--mr-ink-muted);
    }

    .part__error {
      margin: 0;
      font-size: 14px;
      color: var(--mr-danger-ink);
    }
  `,
})
export class LockForm {
  protected readonly alphabets = ALPHABETS;
  readonly draft = input.required<Draft>();
  /** What is wrong with the start ("O começo é igual à solução"), once the master tried to save. */
  readonly error = input('');
  readonly patch = output<Partial<Draft>>();

  protected readonly faces = computed(() => alphabetFaces(this.draft().alphabet));

  protected setWheels(wheels: number): void {
    const d = this.draft();
    // A new wheel starts on the first face and the start of a new wheel on the second: never solved by accident.
    this.patch.emit({
      wheels,
      lockSolution: resized(d.lockSolution, wheels),
      lockStart: resized(d.lockStart, wheels, 1),
    });
  }

  protected setAlphabet(alphabet: PuzzleAlphabet): void {
    const d = this.draft();
    const n = alphabetSize(alphabet);
    this.patch.emit({
      alphabet,
      lockSolution: clamped(d.lockSolution, n),
      lockStart: clamped(d.lockStart, n),
    });
  }

  protected turnSolution(turn: Turn): void {
    this.patch.emit({ lockSolution: this.turned(this.draft().lockSolution, turn) });
  }

  protected turnStart(turn: Turn): void {
    this.patch.emit({ lockStart: this.turned(this.draft().lockStart, turn) });
  }

  private turned(values: readonly number[], turn: Turn): number[] {
    const n = alphabetSize(this.draft().alphabet);
    return values.map((v, i) => (i === turn.index ? (((v + turn.delta) % n) + n) % n : v));
  }
}
