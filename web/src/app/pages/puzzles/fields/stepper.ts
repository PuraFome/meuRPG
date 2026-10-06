import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * "− 4 +" with "De 2 a 6" beside it (the lock's wheels, the pillars' count): two 44 px buttons around the number. At a limit the
 * button is disabled and the words beside the number say why. `noun` finishes the buttons' names ("Menos uma roda").
 */
@Component({
  selector: 'app-stepper',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <span class="st__label" [id]="labelId()">{{ label() }}</span>
    <div class="st" role="group" [attr.aria-labelledby]="labelId()">
      <button type="button" class="st__btn" [attr.aria-disabled]="value() <= min() ? 'true' : null" [attr.aria-label]="'Menos ' + noun()" (click)="value() > min() && valueChange.emit(value() - 1)">
        <mat-icon aria-hidden="true">remove</mat-icon>
      </button>
      <output class="st__value" aria-live="polite">{{ value() }}</output>
      <button type="button" class="st__btn" [attr.aria-disabled]="value() >= max() ? 'true' : null" [attr.aria-label]="'Mais ' + noun()" (click)="value() < max() && valueChange.emit(value() + 1)">
        <mat-icon aria-hidden="true">add</mat-icon>
      </button>
      <span class="st__range">De {{ min() }} a {{ max() }}</span>
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .st__label {
      display: block;
      margin-bottom: var(--mr-space-2);
      font-size: 16px;
      font-weight: 700;
    }

    .st {
      display: flex;
      align-items: center;
      gap: var(--mr-space-3);
    }

    .st__btn {
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

    .st__value {
      min-width: 28px;
      text-align: center;
      font-family: var(--mr-font-display);
      font-weight: 800;
      font-size: 24px;
    }

    .st__range {
      font-size: 15px;
      color: var(--mr-ink-muted);
    }
  `,
})
export class Stepper {
  private static next = 0;
  private readonly uid = Stepper.next++;
  protected readonly labelId = computed(() => `stepper-${this.uid}`);
  readonly label = input.required<string>();
  /** "roda", "pilar", "símbolo": finishes "Menos …" and "Mais …". */
  readonly noun = input.required<string>();
  readonly value = input.required<number>();
  readonly min = input.required<number>();
  readonly max = input.required<number>();
  readonly valueChange = output<number>();
}
