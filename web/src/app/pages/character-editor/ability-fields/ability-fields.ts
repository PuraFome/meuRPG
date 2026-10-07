import { Component, input } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import { abilityLabel } from '../../../core/characters/character-labels';
import { ABILITY_KEYS, AbilityKey } from '../../../core/characters/characters.types';

export type AbilityFormGroup = FormGroup<Record<AbilityKey, FormControl<number>>>;

/**
 * The six ability numbers as narrow fields, laid out like the sheet's
 * ability column: three by two on a tablet or desktop, two by three on a
 * phone, in sheet order (Força to Carisma). Used twice on the "Habilidades"
 * step: for the scores (1 to 30) and for the manual bonuses (-10 to +10).
 *
 * Only what the person typed: no modifier is computed here (every D&D rule
 * runs on the server, and the sheet shows the result).
 *
 * `nameSuffix` goes into each label for screen readers only, so the bonus
 * fields are "Força (bônus manual)" and never share the score field's exact
 * name "Força" (the e2e helpers select on it).
 */
@Component({
  selector: 'app-ability-fields',
  imports: [MatFormFieldModule, MatInputModule, ReactiveFormsModule],
  templateUrl: './ability-fields.html',
  styleUrl: './ability-fields.scss',
})
export class AbilityFields {
  readonly group = input.required<AbilityFormGroup>();
  readonly min = input.required<number>();
  readonly max = input.required<number>();
  /** How to fix an out-of-range value, e.g. "De 1 a 30." */
  readonly rangeError = input.required<string>();
  readonly nameSuffix = input('');

  protected readonly keys = ABILITY_KEYS;
  protected readonly abilityLabel = abilityLabel;
}
