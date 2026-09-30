import { Component, input } from '@angular/core';

import { formatModifier } from '../../../core/characters/character-labels';
import { formatMeters } from '../sheet-format';

/**
 * The combat numbers of the paper sheet, as one `<dl>`: the Armour Class in
 * the garnet shield, Iniciativa and Deslocamento boxes, then the hit point
 * maximum (garnet outline) and the hit dice. A basic NPC sheet has no
 * initiative or hit dice, so those inputs are `null` there and their boxes
 * are left out.
 */
@Component({
  selector: 'app-combat-stats',
  templateUrl: './combat-stats.html',
  styleUrl: './combat-stats.scss',
})
export class CombatStats {
  readonly armorClass = input.required<number>();
  readonly initiative = input<number | null>(null);
  readonly speedFt = input.required<number>();
  readonly hitPointsMax = input.required<number>();
  readonly hitDice = input<string | null>(null);

  protected readonly formatModifier = formatModifier;
  protected readonly formatMeters = formatMeters;
}
