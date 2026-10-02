import { Component, input } from '@angular/core';

import {
  damageTypeLabel,
  formatDamageDice,
  formatModifier,
} from '../../../core/characters/character-labels';
import { BasicAttackVm, BasicSheetVm } from '../character-sheet.types';
import { CombatStats } from '../combat-stats/combat-stats';

/**
 * A minion's or story NPC's basic sheet (`BasicSheet`, MR-005), compact, with
 * the full sheet's own pieces: the combat numbers (initiative, no hit
 * dice), the attacks (name and bonus, then the damage), and the
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

  /** "1d6 + 2 cortante". */
  protected damageOf(a: BasicAttackVm): string {
    return `${formatDamageDice(a.damageDiceCount, a.damageDiceSides, a.damageBonus)} ${damageTypeLabel(a.damageType)}`;
  }
}
