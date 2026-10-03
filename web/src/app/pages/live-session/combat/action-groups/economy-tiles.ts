import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { feetToMeters, formatMeters, tight } from '../../../../core/combat/combat-grid';

/**
 * What the player has this turn, at the top of "O que você pode fazer" from
 * 1024px (E6-14): Ação, Ação bônus and Reação ("Disponível" is an open circle
 * and the word, "Usada" a dashed tile with a filled ✕) and the movement
 * with its bar. On a phone the same four are the hero's bigger tiles.
 */
@Component({
  selector: 'app-economy-tiles',
  imports: [MatIconModule],
  template: `
    <ul class="eco" aria-label="O que você tem neste turno">
      @for (t of tiles(); track t.name) {
        <li class="tile" [class.tile--used]="t.used">
          <span class="tile__name">{{ t.name }}</span>
          <span class="tile__word">
            <span class="dot" aria-hidden="true">@if (t.used) {<mat-icon>close</mat-icon>}</span>{{ t.used ? 'Usada' : 'Disponível' }}
          </span>
        </li>
      }
      <li class="tile tile--move">
        <span class="tile__name">Movimento</span>
        <span class="tile__word tile__word--num">{{ move().text }}</span>
        <span class="bar" aria-hidden="true"><span class="bar__fill" [style.width.%]="move().percent"></span></span>
      </li>
    </ul>
  `,
  styleUrl: './economy-tiles.scss',
})
export class EconomyTiles {
  readonly own = input.required<Combatant>();

  protected readonly tiles = computed(() => [
    { name: 'Ação', used: this.own().actionUsed },
    { name: 'Ação bônus', used: this.own().bonusActionUsed },
    { name: 'Reação', used: this.own().reactionUsed },
  ]);
  protected readonly move = computed(() => {
    const c = this.own();
    const total = Math.max(1, c.speedFt * (c.dashed ? 2 : 1));
    return {
      text: tight(`${formatMeters(feetToMeters(c.movementLeftFt))} de ${formatMeters(feetToMeters(total))}`),
      percent: Math.max(0, Math.min(100, (c.movementLeftFt / total) * 100)),
    };
  });
}
