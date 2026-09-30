import { Component, input } from '@angular/core';

import { abilityLabel, formatModifier } from '../../../core/characters/character-labels';
import { AbilityScoreVm } from '../character-sheet.types';

/**
 * The six ability medallions (docs/design.md, AbilityMedallion): the name,
 * the modifier large, and the score in a pill across the bottom edge, all
 * exactly as the server sent them. A column on the desktop, a row of six on
 * a tablet, a 3×2 grid on a phone.
 */
@Component({
  selector: 'app-ability-medallions',
  templateUrl: './ability-medallions.html',
  styleUrl: './ability-medallions.scss',
})
export class AbilityMedallions {
  readonly abilities = input.required<readonly AbilityScoreVm[]>();

  protected readonly abilityLabel = abilityLabel;
  protected readonly formatModifier = formatModifier;
}
