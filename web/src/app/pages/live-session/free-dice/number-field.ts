import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  computed,
  input,
  linkedSignal,
  output,
  viewChild,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

const MINUS = '−';

/** The number as the box shows it: a signed one has its "+" and a typographic minus. */
export function numberText(n: number, signed: boolean): string {
  if (n < 0) {
    return `${MINUS}${Math.abs(n)}`;
  }
  return signed && n > 0 ? `+${n}` : String(n);
}

/** What was typed as a whole number, or `null`: accepts "+3", "-3" and the typographic minus. */
export function typedNumber(text: string): number | null {
  const clean = text.replace(MINUS, '-').replace(/\s+/g, '');
  return /^[+-]?\d+$/.test(clean) ? Number(clean) : null;
}

/**
 * "[−] 2 [+]" with a box the person can type in ("Digite o número ou use os botões."): two 44 px buttons around a text box.
 * A typed number is kept in range when the box is left (or Enter); anything that is not a number goes back to the last
 * good one. A button at a limit is `aria-disabled` and stays in the tab order, like the other steppers.
 */
@Component({
  selector: 'app-number-field',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <button
      type="button"
      class="nf__btn"
      [attr.aria-disabled]="shown() <= min() ? 'true' : null"
      [attr.aria-label]="minusLabel()"
      (click)="step(-1)"
    >
      <mat-icon aria-hidden="true">remove</mat-icon>
    </button>
    <input
      #box
      class="nf__box"
      type="text"
      inputmode="numeric"
      autocomplete="off"
      [id]="inputId()"
      [value]="text()"
      [attr.aria-describedby]="describedBy() || null"
      (change)="commit(box.value)"
      (keydown.enter)="commit(box.value)"
    />
    <button
      type="button"
      class="nf__btn"
      [attr.aria-disabled]="shown() >= max() ? 'true' : null"
      [attr.aria-label]="plusLabel()"
      (click)="step(1)"
    >
      <mat-icon aria-hidden="true">add</mat-icon>
    </button>
  `,
  styles: `
    :host {
      display: flex;
      align-items: center;
      gap: var(--mr-space-2);
    }

    .nf__btn {
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
        border-style: dashed;
        opacity: 0.5;
        cursor: default;
      }
    }

    .nf__box {
      flex: 1;
      min-width: 0;
      box-sizing: border-box;
      height: 44px;
      padding: 0 var(--mr-space-2);
      border: 1.5px solid var(--mr-control-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
      color: var(--mr-ink);
      font-family: var(--mr-font-display);
      font-size: var(--mr-font-22);
      font-weight: 800;
      text-align: center;

      &:focus-visible {
        outline: 3px solid var(--mr-focus);
        outline-offset: 2px;
      }
    }
  `,
})
export class NumberField {
  readonly inputId = input.required<string>();
  readonly value = input.required<number>();
  readonly min = input.required<number>();
  readonly max = input.required<number>();
  /** A modifier shows its "+". */
  readonly signed = input(false);
  readonly minusLabel = input.required<string>();
  readonly plusLabel = input.required<string>();
  readonly describedBy = input('');
  readonly valueChange = output<number>();

  /** What the field holds: the value it was given, moved at once by each tap, so two quick taps add two before the parent draws. */
  protected readonly shown = linkedSignal(() => this.value());
  protected readonly text = computed(() => numberText(this.shown(), this.signed()));
  private readonly box = viewChild.required<ElementRef<HTMLInputElement>>('box');

  protected step(delta: number): void {
    const next = this.shown() + delta;
    if (next >= this.min() && next <= this.max()) {
      this.shown.set(next);
      this.valueChange.emit(next);
    }
  }

  /** The box was left: the number in range, or what was there before when it is not a number. */
  protected commit(typed: string): void {
    const n = typedNumber(typed);
    const next = n === null ? this.shown() : Math.min(this.max(), Math.max(this.min(), n));
    if (next !== this.shown()) {
      this.shown.set(next);
      this.valueChange.emit(next);
    }
    // The parent may not move (the same number again): the box still says what is kept.
    this.box().nativeElement.value = numberText(next, this.signed());
  }
}
