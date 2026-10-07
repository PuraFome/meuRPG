import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import {
  type Draft,
  BELLS_MAX,
  BELLS_MIN,
  STEPS_MAX,
  STEPS_MIN,
} from '../../../core/puzzles/puzzle-draft';
import { bellFaces } from '../../../core/puzzles/puzzle-symbols';
import { BellsBoard } from '../../../shared/puzzle-boards/bells-board';
import { SecretPill } from '../../../shared/puzzle-boards/secret-pill';
import { SequenceStrip } from '../../../shared/puzzle-boards/sequence-strip';
import { Stepper } from '../fields/stepper';

/** How long each step lights in "Tocar para testar": the server's own pace for the players (1,2 s). */
export const TEST_STEP_MS = 1200;

/**
 * The form of the sequence (MR-038, RN-27, E10-12 state 2): how many bells (3 to 8) and the steps, put by tapping a bell for each step
 * (3 to 12, at least two different bells), in the order the players must repeat them. The sequence is "Só você vê". "Tocar para testar"
 * lights the steps one after another on this screen, for the master alone; it needs no sound, and it is not what a player sees (a
 * player's phone shows each step only as the server reveals it, in a play the master starts in the session). "Apagar o último passo"
 * takes the last one out. Fewer bells than a step used takes that step out.
 */
@Component({
  selector: 'app-sequence-form',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [BellsBoard, MatButtonModule, MatIconModule, SecretPill, SequenceStrip, Stepper],
  template: `
    <app-stepper label="Quantos sinos" noun="sino" [value]="draft().bells" [min]="bellsMin" [max]="bellsMax" (valueChange)="setBells($event)" />

    <section class="part" aria-labelledby="bells-title">
      <h3 class="part__title" id="bells-title">Os sinos</h3>
      <app-bells-board mode="pick" label="Os sinos: toque num para pôr o próximo passo" [faces]="faces()" [lit]="litBell()" [disabled]="draft().steps.length >= stepsMax" (strike)="add($event)" />
      <p class="part__help">Toque num sino para pôr o próximo passo. Os desenhos são nossos.</p>
    </section>

    <section class="part" aria-labelledby="steps-title">
      <h3 class="part__title"><span id="steps-title">A sequência</span><app-secret-pill /></h3>
      @if (draft().steps.length === 0) {
        <p class="part__help">Ainda sem passos: toque num sino acima.</p>
      } @else {
        <app-sequence-strip [faces]="faces()" [steps]="draft().steps" [total]="draft().steps.length" [current]="testing()" label="Os passos da sequência" />
      }
      <p class="part__help" role="status">{{ countText() }}</p>
      <div class="acts">
        <button matButton="outlined" type="button" class="acts__test" [disabled]="draft().steps.length < 2 || testing() >= 0" disabledInteractive [class.mr-button--off]="draft().steps.length < 2" (click)="test()">
          <mat-icon aria-hidden="true">notifications</mat-icon>{{ testing() >= 0 ? 'Tocando...' : 'Tocar para testar' }}
        </button>
        <button matButton type="button" class="acts__undo" [disabled]="draft().steps.length === 0" disabledInteractive [class.mr-button--off]="draft().steps.length === 0" (click)="undo()">Apagar o último passo</button>
      </div>
      @if (error()) {
        <p class="part__bad field-error" role="alert">{{ error() }}</p>
      }
      <p class="mr-visually-hidden" role="status" aria-live="polite">{{ announce() }}</p>
    </section>
  `,
  styleUrl: './sequence-form.scss',
})
export class SequenceForm {
  private readonly destroyRef = inject(DestroyRef);
  protected readonly bellsMin = BELLS_MIN;
  protected readonly bellsMax = BELLS_MAX;
  protected readonly stepsMax = STEPS_MAX;

  readonly draft = input.required<Draft>();
  /** What is wrong with the steps, once the master tried to save (then the server's). */
  readonly error = input('');
  readonly patch = output<Partial<Draft>>();

  protected readonly faces = computed(() => bellFaces(this.draft().bells));
  /** The step lighting in the test, or -1. */
  protected readonly testing = signal(-1);
  protected readonly announce = signal('');
  protected readonly litBell = computed(() => {
    const at = this.testing();
    return at >= 0 ? (this.draft().steps[at] ?? -1) : -1;
  });
  protected readonly countText = computed(() => {
    const n = this.draft().steps.length;
    return `${n === 1 ? '1 passo' : `${n} passos`} (de ${STEPS_MIN} a ${STEPS_MAX}).`;
  });

  private timer: ReturnType<typeof setTimeout> | undefined;

  constructor() {
    this.destroyRef.onDestroy(() => clearTimeout(this.timer));
  }

  protected setBells(bells: number): void {
    // A step that named a bell that is gone goes too.
    this.patch.emit({ bells, steps: this.draft().steps.filter((s) => s < bells) });
  }

  protected add(bell: number): void {
    if (this.draft().steps.length < STEPS_MAX) {
      this.patch.emit({ steps: [...this.draft().steps, bell] });
      this.announce.set(
        `Passo ${this.draft().steps.length + 1}: ${this.faces()[bell]?.namePt ?? ''}.`,
      );
    }
  }

  protected undo(): void {
    this.patch.emit({ steps: this.draft().steps.slice(0, -1) });
  }

  /** "Tocar para testar": lights the steps one by one, here, for the master. */
  protected test(): void {
    clearTimeout(this.timer);
    const run = (at: number): void => {
      const steps = this.draft().steps;
      if (at >= steps.length) {
        this.testing.set(-1);
        return;
      }
      this.testing.set(at);
      this.announce.set(`Passo ${at + 1}: ${this.faces()[steps[at]]?.namePt ?? ''}.`);
      this.timer = setTimeout(() => run(at + 1), TEST_STEP_MS);
    };
    run(0);
  }
}
