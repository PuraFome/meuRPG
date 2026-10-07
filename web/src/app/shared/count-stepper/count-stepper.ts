import {
  ChangeDetectionStrategy,
  Component,
  computed,
  input,
  linkedSignal,
  output,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * "[−] 3 [+]" (E10-08 state 4, E10-09 state 1): two 44 px buttons around the number in a box. At a limit the button is
 * `aria-disabled` and does nothing (it stays in the tab order, like the other steppers). `noun` finishes the buttons'
 * names ("Menos um Bandido"); `minusLabel`, `plusLabel` and `valueLabel` (`{n}` stands for the count) replace the whole wording
 * for a noun that is not masculine or a count with another reading ("Menos uma Maça", "{n} peças"); `minLabel` names the minus button at the lowest value when it means something else ("Tirar
 * o Ogro": the builder takes a creature out at 1). The number is a live `output`, so a tap is announced.
 */
@Component({
  selector: 'app-count-stepper',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <button
      type="button"
      class="cs__btn"
      [attr.aria-disabled]="shown() <= min() ? 'true' : null"
      [attr.aria-label]="shown() <= minAt() && minLabel() ? minLabel() : minusText()"
      (click)="step(-1)"
    >
      <mat-icon aria-hidden="true">remove</mat-icon>
    </button>
    <output class="cs__value" aria-live="polite" [attr.aria-label]="valueText()">{{ shown() }}</output>
    <button
      type="button"
      class="cs__btn"
      [attr.aria-disabled]="shown() >= max() ? 'true' : null"
      [attr.aria-label]="plusText()"
      (click)="step(1)"
    >
      <mat-icon aria-hidden="true">add</mat-icon>
    </button>
  `,
  styles: `
    :host {
      display: inline-flex;
      align-items: center;
      gap: 8px;
    }

    .cs__btn {
      flex: none;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      box-sizing: border-box;
      width: 44px;
      height: 44px;
      padding: 0;
      border: 1.5px solid var(--mr-control-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
      color: var(--mr-ink);
      cursor: pointer;

      &:focus-visible {
        outline: 3px solid var(--mr-focus);
        outline-offset: 2px;
      }

      &[aria-disabled='true'] {
        opacity: 0.4;
        cursor: default;
      }
    }

    .cs__value {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      box-sizing: border-box;
      flex: none;
      min-width: 56px;
      height: 44px;
      padding: 0 8px;
      border: 1.5px solid var(--mr-control-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
      font-family: var(--mr-font-display);
      font-size: 22px;
      font-weight: 800;
    }
  `,
})
export class CountStepper {
  readonly value = input.required<number>();
  readonly min = input(1);
  readonly max = input(10);
  /** "Bandido", "Ogro": finishes "Menos um …" and "Mais um …". */
  readonly noun = input.required<string>();
  /** The name of the minus button at `minAt`, when it takes the thing out instead of lowering it. */
  readonly minLabel = input('');
  readonly minAt = input(1);
  /** The wording of the buttons and of the number for a noun that "Menos um … / Mais um … / 3 …" does not suit. */
  readonly minusLabel = input('');
  readonly plusLabel = input('');
  readonly valueLabel = input('');
  readonly valueChange = output<number>();

  /** What the stepper shows: the value it was given, moved at once by each tap, so two quick taps add two even before the parent draws again. */
  protected readonly shown = linkedSignal(() => this.value());

  protected readonly minusText = computed(() =>
    this.shown() <= this.minAt() && this.minLabel()
      ? this.minLabel()
      : this.minusLabel() || `Menos um ${this.noun()}`,
  );
  protected readonly plusText = computed(() => this.plusLabel() || `Mais um ${this.noun()}`);
  protected readonly valueText = computed(() =>
    this.valueLabel()
      ? this.valueLabel().replace('{n}', String(this.shown()))
      : `${this.shown()} ${this.noun()}`,
  );

  protected step(delta: number): void {
    const next = this.shown() + delta;
    if (next >= this.min() && next <= this.max()) {
      this.shown.set(next);
      this.valueChange.emit(next);
    }
  }
}
