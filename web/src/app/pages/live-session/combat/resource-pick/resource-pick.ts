import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/** A radio card of a resource dialog's list. */
export interface PickRow {
  readonly id: string;
  readonly title: string;
  readonly sub: string;
  /** Why it cannot be chosen, written in the card; empty when it can. */
  readonly blocked: string;
}

let nextId = 0;

/**
 * The list of a resource dialog (PM-07c 9, 10, 12): one radio card for each creature or each slot level, with a line of
 * detail ("Ferida · a 1,5 m", "custa 3 pontos") and, for a card that cannot be chosen, the reason written in it with a
 * dashed frame (never colour alone). Native radios under the cards, so the arrows move the choice and a screen reader
 * reads "1 de 3"; the chosen card has the 2px accent frame and a check. A disabled card is not in the tab order but
 * is still read.
 */
@Component({
  selector: 'app-resource-pick',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <fieldset class="pick">
      <legend class="pick__legend" [class.mr-visually-hidden]="legendHidden()">{{ label() }}</legend>
      @for (r of rows(); track r.id) {
        <label class="opt" [class.opt--on]="r.id === chosen()" [class.opt--off]="!!r.blocked">
          <input
            type="radio"
            class="mr-visually-hidden"
            [name]="name"
            [checked]="r.id === chosen()"
            [disabled]="!!r.blocked"
            (change)="pick.emit(r.id)"
          />
          <span class="opt__radio" aria-hidden="true"></span>
          <span class="opt__text">
            <b>{{ r.title }}</b>
            @if (r.sub) {
              <span class="opt__sub">{{ r.sub }}</span>
            }
            @if (r.blocked) {
              <span class="opt__why"><mat-icon aria-hidden="true">block</mat-icon>{{ r.blocked }}</span>
            }
          </span>
        </label>
      } @empty {
        <p class="pick__empty">{{ empty() }}</p>
      }
    </fieldset>
  `,
  styleUrl: './resource-pick.scss',
})
export class ResourcePick {
  readonly rows = input.required<readonly PickRow[]>();
  readonly chosen = input<string | null>(null);
  /** The group's name ("Quem tocar"). */
  readonly label = input.required<string>();
  readonly legendHidden = input(false);
  /** What the list says when it has no rows. */
  readonly empty = input('Não há ninguém ao alcance.');
  readonly pick = output<string>();
  protected readonly name = `resource-pick-${nextId++}`;
}
