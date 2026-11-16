import { signal } from '@angular/core';
import type { Observable } from 'rxjs';

import { DiceMode, DicePreference } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { AttunementEntryVm } from '../../core/characters/attunement';
import {
  type ConsumableKind,
  type ConsumableNotice,
  type ConsumableTargetVm,
  consumableKind,
  costsAction,
  targetRows,
  type TurnBlock,
  useNotice,
} from '../../core/characters/consumables';
import { effectivePreference } from '../../core/campaigns/dice-labels';
import {
  type ChargesClient,
  type ConsumableRoll,
  type TableDice,
} from '../../core/play/charges-client';
import {
  chargesDidNotGetThrough,
  consumableErrorMessage,
  consumableTargetsErrorMessage,
} from '../../core/play/charges-errors';
import type {
  TargetsOutcome,
  UseConsumableData,
  UseConsumableRequest,
} from './use-consumable-sheet';

/** The character whose consumables are used, as the page that owns the use holds them. */
export interface ConsumableOwner {
  readonly id: string;
  readonly name: string;
  /** The master acts for this character: the sheet reads "Usar por Tor". */
  readonly byMaster: boolean;
  /** The magic items the character holds, one entry for each unit. */
  readonly entries: readonly AttunementEntryVm[];
}

/** What the use of the consumables needs of the page that owns it: the character's sheet, or its combat. */
export interface ConsumableUseHost {
  readonly client: () => ChargesClient;
  readonly campaignId: () => string;
  /** The character as the page holds it. */
  readonly owner: () => ConsumableOwner | null;
  /** Whether a combat is running, when the page knows (the combat's "Itens" does); `null` on the sheet, which says both costs. */
  readonly inCombat: () => boolean | null;
  /** What the page knows keeps a potion from the Action (the turn is not the character's, or the Action is spent); `null` where it does not know, as on the sheet. */
  readonly turnBlock: () => TurnBlock | null;
  /** Reads what the page holds of the character again without the loading state; `true` when the new read is on screen. */
  readonly reload: () => Promise<boolean>;
  /** The table's dice and the player's choice (RN-18), read once from the campaign. */
  readonly dice: () => Promise<TableDice>;
  /** Puts the focus on the row of the consumable (its "Usar") once the page has drawn it again. */
  readonly focusConsumable: (key: string) => void;
  /** The catalog category of an item key ("potion", "scroll"), or '' while it is not read. */
  readonly categoryOf: (key: string) => string;
  readonly openUse: (data: UseConsumableData) => Observable<boolean | undefined>;
}

/**
 * The potions and scrolls of a character: "Usar" opens the use sheet (`UseConsumableSheet`) and every call goes to
 * `ChargesService`. The page reads the character again after a use, since the unit is gone. Works on a locked sheet, for
 * the owner and for the master, who acts for the character ("Usar por Tor"). The sheet does not know whether a combat is
 * running, so it says both costs; the combat's "Itens" says there is one.
 */
export class ConsumableUse {
  /** What the last use said, for the live region under the magic items or the combat's items. */
  readonly result = signal<ConsumableNotice | null>(null);

  constructor(private readonly host: ConsumableUseHost) {}

  /** "Usar" on the row of a consumable: `entry` is the unit the row sends (one entry of the group). */
  openUse(entry: AttunementEntryVm): void {
    const vm = this.host.owner();
    if (!vm) {
      return;
    }
    this.result.set(null);
    const kind: ConsumableKind = consumableKind(this.host.categoryOf(entry.key)) ?? 'potion';
    const left = Math.max(1, vm.entries.filter((e) => e.key === entry.key).length);
    void this.host.dice().then((dice) => {
      this.host
        .openUse({
          itemName: entry.namePt,
          itemKey: entry.key,
          kind,
          effectPt: entry.effectPt ?? '',
          left,
          actingFor: vm.byMaster ? vm.name : null,
          inCombat: this.host.inCombat(),
          blockedBy: costsAction(kind, entry.key) ? this.host.turnBlock() : null,
          canApp: dice.mode !== DiceMode.PHYSICAL,
          canType: dice.mode !== DiceMode.APP,
          preferApp: effectivePreference(dice.mode, dice.preference) === DicePreference.APP,
          loadTargets: () => this.loadTargets(vm, entry, kind),
          use: (request, key) => this.use(vm, entry, kind, left, request, key),
        })
        .subscribe(() => this.host.focusConsumable(entry.key));
    });
  }

  private async loadTargets(
    vm: ConsumableOwner,
    entry: AttunementEntryVm,
    kind: ConsumableKind,
  ): Promise<TargetsOutcome> {
    try {
      const list = await this.host
        .client()
        .listConsumableTargets(this.host.campaignId(), vm.id, entry.id);
      return { targets: targetRows(list), error: '' };
    } catch (err) {
      if (!chargesDidNotGetThrough(err)) {
        await this.host.reload();
      }
      return {
        targets: [],
        error: consumableTargetsErrorMessage(err, {
          kind,
          actingFor: vm.byMaster ? vm.name : undefined,
        }),
      };
    }
  }

  private async use(
    vm: ConsumableOwner,
    entry: AttunementEntryVm,
    kind: ConsumableKind,
    left: number,
    request: UseConsumableRequest,
    key: string,
  ): Promise<string> {
    const target: ConsumableTargetVm | null = request.target;
    const roll: ConsumableRoll = request.roll;
    try {
      const res = await this.host
        .client()
        .useConsumable(this.host.campaignId(), vm.id, entry.id, target?.id ?? '', roll, key);
      await this.host.reload();
      this.result.set(
        useNotice(res, {
          itemName: entry.namePt,
          effectPt: entry.effectPt ?? '',
          userName: vm.name,
          receiverName: target?.name ?? vm.name,
          gift: target !== null,
          woke: target?.down === true,
          left: left - 1,
        }),
      );
      return '';
    } catch (err) {
      if (!chargesDidNotGetThrough(err)) {
        await this.host.reload();
      }
      return consumableErrorMessage(err, {
        kind,
        targetName: target?.name,
        actingFor: vm.byMaster ? vm.name : undefined,
      });
    }
  }
}
