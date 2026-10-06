import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { metersFixed, squaresFree } from '../../../../core/units';
import { tight } from '../../../../core/format/text';

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
            <span class="dot" aria-hidden="true">@if (t.used) {<mat-icon>close</mat-icon>}</span>{{ t.word }}
          </span>
        </li>
      }
      <li class="tile tile--move">
        <span class="tile__name">Movimento</span>
        <span class="tile__word tile__word--num">{{ move().text }}</span>
        <span class="bar" aria-hidden="true"><span class="bar__fill" [style.width.%]="move().percent"></span></span>
        @if (!theatre()) {
          <span class="tile__sub">{{ move().free }}</span>
        }
      </li>
    </ul>
  `,
  styleUrl: './economy-tiles.scss',
})
export class EconomyTiles {
  readonly own = input.required<Combatant>();
  /** Extra Attack: the attacks that remain once the first spent the action. */
  readonly attacksLeft = input(0);
  readonly attacksPerAction = input(1);
  /** No map (RN-25): the movement is a number of meters, with no squares to count. */
  readonly theatre = input(false);

  protected readonly tiles = computed(() => {
    const c = this.own();
    const left = this.attacksLeft();
    const partial = c.actionUsed && left > 0 && this.attacksPerAction() > 1;
    return [
      {
        name: 'Ação',
        used: c.actionUsed && !partial,
        word: partial ? `${left} ${left === 1 ? 'ataque restante' : 'ataques restantes'}` : c.actionUsed ? 'Usada' : 'Disponível',
      },
      { name: 'Ação bônus', used: c.bonusActionUsed, word: c.bonusActionUsed ? 'Usada' : 'Disponível' },
      { name: 'Reação', used: c.reactionUsed, word: c.reactionUsed ? 'Usada' : 'Disponível' },
    ];
  });
  protected readonly move = computed(() => {
    const c = this.own();
    // In tenths of a foot, as the server keeps it, and metres with one decimal ("6,9 m de 9,0 m").
    const total = Math.max(1, c.speedDft);
    return {
      text: tight(`${metersFixed(c.movementLeftDft / 10)} de ${metersFixed(total / 10)}`),
      free: squaresFree(c.movementLeftFt),
      percent: Math.max(0, Math.min(100, (c.movementLeftDft / total) * 100)),
    };
  });
}
