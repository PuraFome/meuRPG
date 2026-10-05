import { Component } from '@angular/core';

import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';

/**
 * The three shapes of the order's tokens (E9-12): a player's character is a round disc, an NPC a rounded square
 * and a player's creature a round token with a dashed outline. Drawn once a creature is in the order, so no
 * meaning is the shape alone: each one is named.
 */
@Component({
  selector: 'app-order-legend',
  imports: [CombatantToken],
  template: `
    <ul class="legend" aria-label="Formas das fichas">
      <li><app-combatant-token initial="P" [size]="22" />Jogador</li>
      <li><app-combatant-token initial="C" [npc]="true" [size]="22" />NPC</li>
      <li><app-combatant-token initial="N" [creature]="true" [size]="22" />Criatura de um jogador</li>
    </ul>
  `,
  styles: `
    .legend {
      display: flex;
      flex-wrap: wrap;
      gap: 6px 16px;
      margin: 12px 0 0;
      padding: 0;
      list-style: none;
      font-size: 14px;
      color: var(--mr-ink-muted);
    }

    li {
      display: inline-flex;
      align-items: center;
      gap: 6px;
    }
  `,
})
export class OrderLegend {}
