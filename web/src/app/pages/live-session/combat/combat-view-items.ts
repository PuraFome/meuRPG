import { afterNextRender, computed, Directive, effect, untracked } from '@angular/core';

import { turnBlockOf } from '../../../core/characters/consumables';
import { focusWithRing } from '../../../core/creatures/focus-ring';
import { ChargesClient } from '../../../core/play/charges-client';
import { openUseConsumable } from '../../../shared/consumable-use/use-consumable-sheet';
import { tabId } from '../../../shared/tab-list/tab-list';
import { CombatItems } from './combat-items';
import { CombatViewPartsDirective } from './combat-view-parts';

/** The combat's "Itens" tab: the potions and scrolls of the character on turn and the "Usar" sheet. */
@Directive()
export abstract class CombatViewItemsDirective extends CombatViewPartsDirective {
  /** The potions and scrolls of the character on turn: the "Itens" tab and the "Usar" sheet of the combat (`CombatItems`). */
  protected readonly items = new CombatItems({
    client: () => this.injector.get(ChargesClient),
    campaignId: () => this.campaignId(),
    // The own combatant: the player's character, or the one the master acts for.
    characterId: () => this.own()?.characterId ?? '',
    name: () => this.own()?.label ?? '',
    byMaster: () => this.isMaster(),
    enabled: () => this.myTurn() && !this.ownDown(),
    signature: () => {
      const o = this.own();
      return o ? `${this.encounter()?.currentCombatantId}|${o.actionUsed}` : '';
    },
    // The Action of the turn is spent (the row sits folded under "Ação · Usada"), or the turn is not the character's: the sheet says so.
    turnBlock: () =>
      turnBlockOf({ myTurn: this.myTurn(), actionUsed: this.own()?.actionUsed ?? false }),
    dice: () => Promise.resolve({ mode: this.diceMode(), preference: this.dicePreference() }),
    openUse: (data) => openUseConsumable(this.dialog, this.bottomSheet, data),
    // The row of the potion is folded or gone once the Action is spent: the tab takes the focus then.
    focusItem: (key) =>
      afterNextRender(
        () => {
          const root = this.host.nativeElement;
          focusWithRing(
            root.querySelector<HTMLElement>(`[data-item-row="${key}"] .row__btn`) ??
              root.querySelector<HTMLElement>(`#${tabId('acts', 'items')}`),
          );
        },
        { injector: this.injector },
      ),
  });

  private readonly turnOf = computed(() => this.encounter()?.currentCombatantId ?? '');

  constructor() {
    super();
    // What a use said is not carried into the next turn.
    effect(() => {
      this.turnOf();
      untracked(() => this.items.forgetNotice());
    });
  }
}
