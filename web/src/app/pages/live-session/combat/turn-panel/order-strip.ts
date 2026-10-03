import { Component, input } from '@angular/core';

import type { Combatant, Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatantInitial, isPlayer, stateWord } from '../../../../core/combat/combat-view';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';

/**
 * The order as a strip of chips (E6-05): a token, the name and one word. An
 * NPC says its state (Ileso, Ferido, Muito ferido, Derrotado), another
 * player only "Jogador" (their name is not sent to the other players), and
 * the one on turn has the 2px accent frame and the word "Vez"; the player's
 * own chip says "Você". Hidden combatants are not in the list a player gets.
 */
@Component({
  selector: 'app-order-strip',
  imports: [CombatantToken],
  template: `
  <ol class="strip" aria-label="Ordem de iniciativa">
    @for (c of encounter().combatants; track c.id) {
      <li class="chip" [class.chip--turn]="current(c)">
        <span class="chip__top">
          <app-combatant-token
            [initial]="initial(c)"
            [npc]="npc(c)"
            [defeated]="c.defeated"
            [mine]="c.mine"
            [size]="24"
          />
          @if (current(c)) {
            <span class="chip__word chip__word--turn">Vez</span>
          } @else if (c.mine) {
            <span class="chip__word">Você</span>
          }
        </span>
        <span class="chip__name" [class.chip__name--out]="c.defeated">{{ c.label }}</span>
        <span class="chip__sub">{{ word(c) }}</span>
      </li>
    }
  </ol>
  `,
  styles: `
:host {
  display: block;
}

.strip {
  display: flex;
  gap: 8px;
  margin: 0;
  padding: 0 0 2px;
  list-style: none;
  overflow-x: auto;
  scroll-snap-type: x proximity;
}

.chip {
  display: flex;
  flex: 0 0 104px;
  flex-direction: column;
  gap: 3px;
  box-sizing: border-box;
  min-height: 68px;
  padding: 8px 10px;
  border: 1px solid var(--mr-line);
  border-radius: var(--mr-radius-md);
  background: var(--mr-surface);
  scroll-snap-align: start;
}

.chip--turn {
  padding: 7px 9px;
  border: 2px solid var(--mr-accent);
  background: var(--mr-accent-soft);
}

.chip__top {
  display: flex;
  align-items: center;
  gap: 6px;
}

.chip__word {
  font-size: 13px;
  font-weight: 700;
  color: var(--mr-ink-muted);
}

.chip__word--turn {
  color: var(--mr-accent-text);
}

.chip__name {
  font-size: 15px;
  font-weight: 700;
  line-height: 18px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chip__name--out {
  color: var(--mr-ink-muted);
  text-decoration: line-through;
}

.chip__sub {
  font-size: 14px;
  line-height: 17px;
  color: var(--mr-ink-muted);
}

  `,
})
export class OrderStrip {
  readonly encounter = input.required<Encounter>();

  protected initial(c: Combatant): string {
    return combatantInitial(c.label);
  }

  protected npc(c: Combatant): boolean {
    return !isPlayer(c);
  }

  protected word(c: Combatant): string {
    return isPlayer(c) ? 'Jogador' : stateWord(c.state);
  }

  protected current(c: Combatant): boolean {
    return c.id === this.encounter().currentCombatantId;
  }
}
