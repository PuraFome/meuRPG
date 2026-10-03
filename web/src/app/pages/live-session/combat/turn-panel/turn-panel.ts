import { Component, ElementRef, computed, effect, input, output, viewChild } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant, Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { formatMeters, feetToMeters } from '../../../../core/combat/combat-grid';
import {
  combatantInitial,
  isPlayer,
  ownCombatant,
  roundLabel,
  stateWord,
  turnBanner,
} from '../../../../core/combat/combat-view';
import { mediaQuery } from '../../../../shared/map-view/media-query';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';
import { EndTurn } from './end-turn';
import { OrderStrip } from './order-strip';

/**
 * What a player sees at the top of a running combat (E6-05, E6-06): whose
 * turn it is, who is next, and the order as a strip of chips. The banner is
 * a live region, so a turn change is heard once. When the turn is a hidden
 * combatant's it says "Vez do mestre", with no name and no highlighted chip.
 * On the player's own turn ("Sua vez") the banner is the hero, with the
 * movement left and, from 1024px, "Encerrar turno" (`EndTurn`). The groups
 * of actions (`ActionGroups`) come right after it, with "Mover" in the
 * Movimento group. Focus goes to the hero when the turn arrives, so the next
 * Tab reaches what the player can do.
 */
@Component({
  selector: 'app-turn-panel',
  imports: [CombatantToken, EndTurn, MatIconModule, OrderStrip],
  templateUrl: './turn-panel.html',
  styleUrl: './turn-panel.scss',
})
export class TurnPanel {
  readonly encounter = input.required<Encounter>();
  readonly busy = input(false);

  readonly endTurn = output<void>();

  private readonly hero = viewChild<ElementRef<HTMLElement>>('hero');

  /** From 1024px the player's own turn is a compact banner (E6-14); from
   * 1280px the order is a column on the left, so the strip goes. */
  private readonly desktop = mediaQuery('(min-width: 1024px)');
  protected readonly wide = mediaQuery('(min-width: 1280px)');
  protected readonly compact = computed(() => this.desktop() && this.banner().mine);
  protected readonly banner = computed(() => turnBanner(this.encounter()));
  protected readonly round = computed(() => roundLabel(this.encounter().round));
  protected readonly own = computed(() => ownCombatant(this.encounter()));
  protected readonly strip = computed(() => this.encounter().combatants);
  protected readonly movement = computed(() => {
    const own = this.own();
    return own
      ? {
          left: formatMeters(feetToMeters(own.movementLeftFt)),
          total: formatMeters(feetToMeters(own.speedFt * (own.dashed ? 2 : 1))),
          used: own.movementUsedFt > 0 ? formatMeters(feetToMeters(own.movementUsedFt)) : '',
          none: own.movementLeftFt <= 0,
          percent: Math.max(
            0,
            Math.min(100, (own.movementLeftFt / Math.max(1, own.speedFt * (own.dashed ? 2 : 1))) * 100),
          ),
        }
      : null;
  });
  protected readonly nextLine = computed(() => {
    const banner = this.banner();
    if (banner.mine || banner.masterTurn || !banner.next) {
      return null;
    }
    return banner.nextIsMine ? { prefix: 'Você é o próximo: depois dele, ', name: banner.next.label } : null;
  });

  constructor() {
    // Focus moves to the hero when the turn arrives, once, so the next Tab
    // reaches "Mover".
    let wasMine = false;
    effect(() => {
      const mine = this.banner().mine;
      if (mine && !wasMine) {
        queueMicrotask(() => this.hero()?.nativeElement.focus());
      }
      wasMine = mine;
    });
  }

  protected initial(c: Combatant): string {
    return combatantInitial(c.label);
  }

  protected npc(c: Combatant): boolean {
    return !isPlayer(c);
  }

  /** The chip's second line: an NPC's state word, "Jogador" for another
   * player (their name is not sent to the other players). */
  protected word(c: Combatant): string {
    return isPlayer(c) ? 'Jogador' : stateWord(c.state);
  }

  protected current(c: Combatant): boolean {
    return c.id === this.encounter().currentCombatantId;
  }
}
