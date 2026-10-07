import { Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/** "Força − 0 +": a small whole number with a 44 px minus and plus, the value shown with its sign ("+2", "0"). */
@Component({
  selector: 'app-number-stepper',
  imports: [MatIconModule],
  template: `
    <div class="step" role="group" [attr.aria-label]="label()" [attr.data-field]="path() || null">
      <span class="step__label" aria-hidden="true">{{ label() }}</span>
      <button type="button" class="step__btn" [attr.aria-label]="'Menos ' + label()" [disabled]="!softDisable() && value() <= min()" [attr.aria-disabled]="softDisable() && value() <= min() ? 'true' : null" [class.step__btn--soft]="softDisable() && value() <= min()" (click)="bump(-1)">
        <mat-icon aria-hidden="true">remove</mat-icon>
      </button>
      <output class="step__value" [attr.aria-label]="label() + ' ' + text()">{{ text() }}</output>
      <button type="button" class="step__btn" [attr.aria-label]="'Mais ' + label()" [disabled]="!softDisable() && value() >= max()" [attr.aria-disabled]="softDisable() && value() >= max() ? 'true' : null" [class.step__btn--soft]="softDisable() && value() >= max()" (click)="bump(1)">
        <mat-icon aria-hidden="true">add</mat-icon>
      </button>
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .step {
      display: flex;
      align-items: center;
      gap: var(--mr-space-2);
    }

    .step__label {
      flex: 1 1 auto;
      min-width: 0;
      font-size: 17px;
      color: var(--mr-ink-muted);
    }

    .step__btn {
      flex: none;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 44px;
      height: 44px;
      padding: 0;
      border: 1px solid var(--mr-control-line);
      border-radius: var(--mr-radius-sm);
      background: var(--mr-surface);
      color: var(--mr-ink);
      cursor: pointer;

      &:disabled,
      &--soft {
        opacity: 0.4;
        cursor: default;
      }

      &:focus-visible {
        outline: 2px solid var(--mr-focus);
        outline-offset: 2px;
      }
    }

    .step__value {
      flex: none;
      min-width: 36px;
      text-align: center;
      font-family: var(--mr-font-display);
      font-size: 22px;
      font-weight: 800;
    }
  `,
})
export class NumberStepper {
  readonly label = input.required<string>();
  readonly value = input.required<number>();
  readonly min = input(-9);
  readonly max = input(9);
  readonly path = input('');
  readonly valueChange = output<number>();
  /** Plain numbers ("4") instead of the signed ones of a modifier ("+4"): a level, a count. */
  readonly signed = input(true);
  /** At a limit the button says `aria-disabled` and stays focusable, so the focus never drops when the last step is taken. */
  readonly softDisable = input(false);

  protected bump(by: number): void {
    const next = this.value() + by;
    if (next >= this.min() && next <= this.max()) {
      this.valueChange.emit(next);
    }
  }

  protected readonly text = computed(() => (!this.signed() ? String(this.value()) : this.value() > 0 ? `+${this.value()}` : this.value() < 0 ? `−${Math.abs(this.value())}` : '0'));
}
