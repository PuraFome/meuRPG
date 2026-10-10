import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { ABILITY_CHOICES } from '../../../../core/effects/ability-choice';

let nextId = 0;

/**
 * "Habilidade" (Aprimorar Habilidade): one radio card for each of the six abilities, with the animal of the spell
 * under it (Touro, Gato, Urso, Raposa, Coruja, Águia). None starts chosen; the caller holds the cast back until one is.
 * Native radios under the cards, as in the damage type picker.
 */
@Component({
  selector: 'app-ability-picker',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <span class="cap" [id]="id + '-l'">Habilidade</span>
    <div class="rows" role="radiogroup" [attr.aria-labelledby]="id + '-l'">
      @for (a of abilities; track a.key) {
        <label class="row">
          <input
            type="radio"
            class="mr-visually-hidden"
            [name]="id"
            [checked]="a.key === chosen()"
            (change)="pick.emit(a.key)"
          />
          <span class="row__text">
            <b class="row__title">{{ a.name }}</b>
            <span class="row__hint">{{ a.hint }}</span>
          </span>
          @if (a.key === chosen()) {
            <mat-icon class="row__check" aria-hidden="true">check_circle</mat-icon>
          } @else {
            <mat-icon class="row__empty" aria-hidden="true">radio_button_unchecked</mat-icon>
          }
        </label>
      }
    </div>
    <span class="hint">A magia vale para a habilidade escolhida.</span>
  `,
  styleUrl: './ability-picker.scss',
})
export class AbilityPicker {
  /** The key chosen; empty while none is. */
  readonly chosen = input('');
  readonly pick = output<string>();

  protected readonly abilities = ABILITY_CHOICES;
  protected readonly id = `ability-picker-${nextId++}`;
}
