import { Component, computed, input } from '@angular/core';

import { formatModifier } from '../../../core/characters/character-labels';
import { metersText, squaresWithFeet } from '../../../core/units';

/**
 * The combat numbers of the paper sheet, as one `<dl>`: the Armour Class in
 * the garnet shield, the Iniciativa box, the Deslocamento box (full width, on
 * one line: "7,5 m · 5 quadrados (25 pés)", E8-01), then the hit point
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
  /** The meters in the display type, then the squares and the feet in the small text after it. */
  protected readonly speed = computed(() => ({
    meters: metersText(this.speedFt()),
    rest: squaresWithFeet(this.speedFt()),
  }));
}
