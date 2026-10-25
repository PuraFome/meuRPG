import { computed, Directive } from '@angular/core';

import { isDead, isDown, isPlayer, soloTitle } from '../../../core/combat/combat-view';
import { moveLine } from '../../../core/combat/move-line';
import { economyShapes } from '../../../shared/economy-shapes/economy-shapes';
import { CombatViewDerivedDirective } from './combat-view-derived';

/**
 * What the master's "Turno" reads to act for a player character (C8-01, C8-02): the card with "Agir por …" while he does not, the
 * character's shapes and hit points while he does, and whether the NPC's card is the one to draw. The character the master acts
 * for is `acting`; `own` is that one (see `CombatViewStateDirective`).
 */
@Directive()
export abstract class CombatViewActingDirective extends CombatViewDerivedDirective {
  /** The player character on turn, on its feet, that the master has not taken the turn of: its card has "Agir por". */
  protected readonly actCard = computed(() => {
    const c = this.masterCard();
    return c && isPlayer(c) && !this.acting() && !isDown(c) && !isDead(c) && !this.cardVisible()
      ? c
      : null;
  });
  /** "Vez do Pensantus": the same title the bar and the player's own screens give a turn. */
  protected readonly actCardTitle = computed(() => {
    const e = this.encounter();
    const c = this.actCard();
    return e && c ? soloTitle(e, c) : '';
  });
  /** The NPC card: for an NPC on turn, and for a player's turn that left damage to apply; never over the player's own card. */
  protected readonly npcCardOn = computed(() => {
    const c = this.masterCard();
    if (!c || this.acting() || this.actCard()) {
      return null;
    }
    const damage = this.cardVisible();
    if (isPlayer(c)) {
      return damage ? c : null;
    }
    return !this.laptop() || damage || this.theatre() ? c : null;
  });
  /** The hit points of the character on the card (from its vitals: the master reads the numbers). */
  protected readonly actCardHitPoints = computed(() => {
    const c = this.actCard();
    const v = c ? this.vitals().find((x) => x.characterId === c.characterId) : undefined;
    return v ? { current: v.hitPointsCurrent, max: v.hitPointsMax } : null;
  });
  /** The shapes of the character the master acts for, with the walker while there is a move to make. */
  protected readonly actingShapes = computed(() => {
    const c = this.acting();
    if (!c) {
      return [];
    }
    const line = moveLine(c, this.contestFacts()?.grappled === true);
    return economyShapes(c, {
      attacksLeft: this.attacksLeft(),
      attacksPerAction: this.attacksPerAction(),
      moveText: line.canMove ? line.left : null,
    });
  });
}
