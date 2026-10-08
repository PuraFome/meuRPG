import { Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { SpellDamageChoice } from '../../../../../gen/meurpg/rules/v1/rules_pb';

/** One damage type the caster may pick: its content key and its Portuguese name. */
export interface DamageTypeChoice {
  readonly key: string;
  readonly name: string;
}

let nextId = 0;

/**
 * "Tipo de dano" (a spell that lets the caster pick): one radio card for each damage
 * type the spell offers, the first chosen. Under a spell that scales (Coluna de Chamas)
 * it says the dice of a higher circle go to the chosen type; under one that only deals
 * the chosen type (Guardiões Espirituais) it says the others are not dealt. Native
 * radios under the cards, as in the slot picker.
 */
@Component({
  selector: 'app-damage-type-picker',
  imports: [MatIconModule],
  template: `
    <span class="cap" [id]="id + '-l'">Tipo de dano</span>
    <div class="rows" role="radiogroup" [attr.aria-labelledby]="id + '-l'">
      @for (c of choices(); track c.key) {
        <label class="row">
          <input
            type="radio"
            class="mr-visually-hidden"
            [name]="id"
            [checked]="c.key === chosen()"
            (change)="pick.emit(c.key)"
          />
          <b class="row__title">{{ c.name }}</b>
          @if (c.key === chosen()) {
            <mat-icon class="row__check" aria-hidden="true">check_circle</mat-icon>
          } @else {
            <mat-icon class="row__empty" aria-hidden="true">radio_button_unchecked</mat-icon>
          }
        </label>
      }
    </div>
    <span class="hint">{{ hint() }}</span>
  `,
  styleUrl: './damage-type-picker.scss',
})
export class DamageTypePicker {
  readonly choices = input.required<readonly DamageTypeChoice[]>();
  readonly chosen = input.required<string>();
  readonly choice = input.required<SpellDamageChoice>();
  readonly pick = output<string>();

  protected readonly id = `damage-type-picker-${nextId++}`;

  protected hint(): string {
    return this.choice() === SpellDamageChoice.SCALE
      ? 'Os dados a mais de um espaço de magia maior vão para o tipo escolhido.'
      : 'Só o tipo escolhido causa dano.';
  }
}
