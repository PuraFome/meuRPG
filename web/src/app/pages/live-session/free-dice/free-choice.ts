import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/** One choice of a `FreeChoice` group. */
export interface FreeChoiceOption {
  readonly value: string;
  readonly label: string;
}

let nextId = 0;

/**
 * A row of radio chips for the dice form ("d4 … d100", "Normal | Vantagem | Desvantagem", "Para a mesa | Só eu"): real
 * radio buttons under the labels, so the arrow keys move the choice and a screen reader hears "d6, selecionado, 2 de 7";
 * the chosen chip has a check as well as the filled look (never colour alone). Seven chips fit a 320 px phone, so the
 * chips are tight and never wider than their share of the row.
 */
@Component({
  selector: 'app-free-choice',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <div class="fc" role="radiogroup" [attr.aria-label]="label()">
      @for (o of options(); track o.value) {
        <label class="fc__chip" [class.fc__chip--on]="o.value === value()">
          <input
            type="radio"
            class="mr-visually-hidden"
            [name]="name"
            [value]="o.value"
            [checked]="o.value === value()"
            (change)="choose.emit(o.value)"
          />
          @if (o.value === value()) {
            <mat-icon aria-hidden="true">check</mat-icon>
          }
          <span class="fc__text">{{ o.label }}</span>
        </label>
      }
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .fc {
      display: grid;
      grid-auto-flow: column;
      grid-auto-columns: minmax(0, 1fr);
      gap: 6px;
    }

    .fc__chip {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 2px;
      box-sizing: border-box;
      min-height: 44px;
      padding: 0 2px;
      border: 1.5px solid var(--mr-control-line);
      border-radius: var(--mr-radius-sm);
      background: var(--mr-surface);
      color: var(--mr-ink);
      font-weight: 700;
      text-align: center;
      cursor: pointer;

      .mat-icon {
        flex: none;
        width: 14px;
        height: 14px;
        font-size: 14px;
      }

      &:has(:focus-visible) {
        outline: 3px solid var(--mr-focus);
        outline-offset: 2px;
      }
    }

    .fc__chip--on {
      border-color: var(--mr-accent);
      background: var(--mr-accent-soft);
      color: var(--mr-accent-strong);
    }
  `,
})
export class FreeChoice {
  readonly label = input.required<string>();
  readonly options = input.required<readonly FreeChoiceOption[]>();
  readonly value = input.required<string>();
  readonly choose = output<string>();
  protected readonly name = `fc-${nextId++}`;
}
