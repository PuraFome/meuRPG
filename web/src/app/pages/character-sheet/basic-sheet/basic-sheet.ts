import { Component, input } from '@angular/core';

import { formatModifier } from '../../../core/characters/character-labels';
import { BasicSheetVm } from '../character-sheet.types';
import { CombatStats } from '../combat-stats/combat-stats';

/**
 * A minion's or story NPC's basic sheet (`BasicSheet`, MR-005), compact, with
 * the full sheet's own pieces: the combat numbers (no initiative or hit
 * dice), the one attack written on rules like the spell trio, and the
 * short description for the table.
 */
@Component({
  selector: 'app-basic-sheet',
  imports: [CombatStats],
  templateUrl: './basic-sheet.html',
  styleUrl: './basic-sheet.scss',
})
export class BasicSheet {
  readonly sheet = input.required<BasicSheetVm>();

  protected readonly formatModifier = formatModifier;
}
