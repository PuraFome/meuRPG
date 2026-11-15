import { signal } from '@angular/core';
import type { Observable } from 'rxjs';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  ItemLoss,
  ItemRechargeKind,
  type ItemRecharge,
  type SheetMagicItem,
} from '../../../../gen/meurpg/characters/v1/characters_pb';
import { ChargesBlockedReason } from '../../../../gen/meurpg/play/v1/charges_pb';
import type { AttunementEntryVm } from '../../../core/characters/attunement';
import { describeCharacterError } from '../../../core/characters/character-errors';
import {
  type AmmunitionHint,
  AMMUNITION_KINDS,
  ammunitionOf,
  articleOf,
  recoverOf,
} from '../../../core/characters/charges';
import { effectivePreference } from '../../../core/campaigns/dice-labels';
import { ActionKey } from '../../../core/connect/idempotency';
import {
  ChargesClient,
  type LastChargeRoll,
  type TableDice,
} from '../../../core/play/charges-client';
import {
  chargesBlocked,
  chargesBlockedMessage,
  chargesDidNotGetThrough,
  chargesErrorMessage,
} from '../../../core/play/charges-errors';
import type { CharacterSheetVm } from '../character-sheet.types';
import { rechargeWithCap } from '../../../core/characters/recharge-text';
import type { AddAmmunitionData } from './add-ammunition-sheet';
import type { LastChargeData, LastChargeOutcome } from './last-charge-sheet';
import type { UseChargesData } from './use-charges-sheet';

/** What the charges of the sheet need of the page that owns them. */
export interface SheetChargesHost {
  /** The client, asked for when a charge is first used: a page that never uses one never builds it. */
  readonly client: () => ChargesClient;
  readonly campaignId: () => string;
  /** The character as the page holds it. */
  readonly vm: () => CharacterSheetVm | null;
  /** Reads the sheet again without the loading state; `true` when the new sheet is on screen. */
  readonly reload: () => Promise<boolean>;
  /** Puts the focus on the row of the item (its "Usar" or "Rolar", or the row itself when it has none) once the page has drawn
   * it again: the sheets do not restore it, since the button that opened them may be gone. */
  readonly focusRow: (itemId: string) => void;
  readonly openUse: (data: UseChargesData) => Observable<boolean | undefined>;
  readonly openRoll: (data: LastChargeData) => Observable<boolean | undefined>;
  readonly openAdd: (data: AddAmmunitionData) => Observable<boolean | undefined>;
}

/** The most pieces of one kind the server takes. */
const MOST_AMMUNITION = 9999;

const FALLBACK_DICE: TableDice = { mode: DiceMode.PLAYERS_CHOOSE, preference: DicePreference.APP };

/** "carga" / "cargas" with its count. */
const cargas = (n: number) => `${n} ${n === 1 ? 'carga' : 'cargas'}`;

/** What the roll of the last charge came to, in words, and whether it is good news for the item. */
function rollResult(
  name: string,
  res: { d20: number; regained: number; lost: ItemLoss },
  recharge?: ItemRecharge,
): { text: string; tone: 'good' | 'bad' } {
  const head = `Tirou ${res.d20}. ${articleOf(name)} ${name}`;
  switch (res.lost) {
    case ItemLoss.GONE:
      return { text: `${head} virou cinzas: fica na ficha como destruída.`, tone: 'bad' };
    case ItemLoss.PLAIN_ITEM:
      return { text: `${head} perdeu a magia: virou um item comum.`, tone: 'bad' };
    case ItemLoss.PARTLY_MAGIC:
      return {
        text: `${head} perdeu a magia: ficou só o bônus no ataque e no dano.`,
        tone: 'bad',
      };
  }
  return res.regained > 0
    ? { text: `${head} recuperou ${cargas(res.regained)}.`, tone: 'good' }
    : {
        text: `${head} fica, sem cargas${recharge?.kind === ItemRechargeKind.DAWN ? ', e volta ao amanhecer' : ''}.`,
        tone: 'good',
      };
}

/**
 * The charges and the ammunition of the sheet: "Usar" (the use sheet), the d20 of the last charge, "Adicionar munição" and
 * "Recuperar a metade". Every write goes to `ChargesService` and the page reads the sheet again for the numbers it derives.
 * The use and the d20 work on a locked sheet. The d20 follows the table's dice (RN-18), read once from the campaign.
 */
export class SheetCharges {
  /** What the last use or roll said, for the live region under the magic items. */
  readonly result = signal<{ readonly text: string; readonly tone: 'good' | 'bad' } | null>(null);
  readonly error = signal('');
  readonly ammunitionNotice = signal('');
  readonly ammunitionError = signal('');
  /** The character's latest combat is still running: the recovery waits. */
  readonly combatRunning = signal(false);

  private dice: Promise<TableDice> | null = null;
  private readonly recoverKey = new ActionKey();
  private recovering = false;

  constructor(private readonly host: SheetChargesHost) {}

  /** The vitals changed (a combat may have ended): the recovery is tried again on the next tap. */
  combatMayHaveEnded(): void {
    this.combatRunning.set(false);
  }

  openUse(entry: AttunementEntryVm): void {
    const c = entry.charges;
    const vm = this.host.vm();
    if (!c || c.state !== 'ready' || !vm) {
      return;
    }
    this.clear();
    this.host
      .openUse({
        itemName: entry.namePt,
        max: c.max,
        left: c.left,
        rechargeNote: c.recharge ? rechargeWithCap(c.recharge, c.max) : undefined,
        spend: (amount, key) => this.spend(entry, amount, key),
      })
      .subscribe(() => {
        const asked = this.askedRoll;
        this.askedRoll = null;
        if (asked) {
          // The d20 takes the focus back to the row when it closes.
          this.openRoll(asked);
        } else {
          this.host.focusRow(entry.id);
        }
      });
  }

  /** Set when the last charge went from an item that asks for its d20: the d20 opens as the use sheet closes. */
  private askedRoll: AttunementEntryVm | null = null;

  private async spend(entry: AttunementEntryVm, amount: number, key: string): Promise<string> {
    const vm = this.host.vm();
    if (!vm) {
      return 'A ficha não está aberta.';
    }
    try {
      const res = await this.host
        .client()
        .spendCharges(this.host.campaignId(), vm.id, entry.id, amount, key);
      await this.host.reload();
      const item = res.item;
      this.result.set(item ? this.spentResult(entry, amount, item) : null);
      if (item?.lastRollPending) {
        this.askedRoll = entry;
      }
      return '';
    } catch (err) {
      const left = entry.charges?.left ?? 0;
      if (!chargesDidNotGetThrough(err)) {
        await this.host.reload();
      }
      return chargesErrorMessage(err, {
        itemName: entry.namePt,
        left,
        recharge: entry.charges?.recharge,
      });
    }
  }

  private spentResult(
    entry: AttunementEntryVm,
    amount: number,
    item: SheetMagicItem,
  ): { text: string; tone: 'good' | 'bad' } {
    const name = entry.namePt;
    if (item.lost !== ItemLoss.UNSPECIFIED) {
      return {
        text: `Gastou a última carga. ${articleOf(name)} ${name} se perdeu: fica na ficha como destruída.`,
        tone: 'bad',
      };
    }
    const of = articleOf(name) === 'A' ? 'da' : 'do';
    if (item.lastRollPending) {
      return {
        text: `Gastou a última carga ${of} ${name}. Falta rolar o d20.`,
        tone: 'good',
      };
    }
    return {
      text: `Gastou ${cargas(amount)} ${of} ${name}. Restam ${Math.max(item.chargesLeft, 0)} de ${item.chargesMax}.`,
      tone: 'good',
    };
  }

  /** "Rolar": the d20 of the item's last charge, the way the table's dice say. */
  openRoll(entry: AttunementEntryVm): void {
    const c = entry.charges;
    if (!c || !this.host.vm()) {
      return;
    }
    this.clear();
    void this.tableDice().then((dice) => {
      this.host
        .openRoll({
          itemName: entry.namePt,
          max: c.max,
          canApp: dice.mode !== DiceMode.PHYSICAL,
          canType: dice.mode !== DiceMode.APP,
          preferApp: effectivePreference(dice.mode, dice.preference) === DicePreference.APP,
          recharge: c.recharge,
          roll: (roll, key) => this.roll(entry, roll, key),
        })
        .subscribe(() => this.host.focusRow(entry.id));
    });
  }

  private tableDice(): Promise<TableDice> {
    const known =
      this.dice ??
      this.host
        .client()
        .diceOf(this.host.campaignId())
        .catch(() => {
          // Without the setting the sheet offers both ways; the server refuses the one the table does not allow, in words.
          this.dice = null;
          return FALLBACK_DICE;
        });
    this.dice = known;
    return known;
  }

  private async roll(
    entry: AttunementEntryVm,
    roll: LastChargeRoll,
    key: string,
  ): Promise<LastChargeOutcome> {
    const vm = this.host.vm();
    if (!vm) {
      return { kind: 'error', text: 'A ficha não está aberta.' };
    }
    try {
      const res = await this.host
        .client()
        .rollLastCharge(this.host.campaignId(), vm.id, entry.id, roll, key);
      await this.host.reload();
      this.result.set(rollResult(entry.namePt, res, entry.charges?.recharge));
      return { kind: 'done', d20: res.d20 };
    } catch (err) {
      const blocked = chargesBlocked(err);
      if (blocked?.reason === ChargesBlockedReason.REGAIN_FACE_NEEDED) {
        return { kind: 'regain', faces: blocked.needed };
      }
      if (!chargesDidNotGetThrough(err)) {
        await this.host.reload();
      }
      return { kind: 'error', text: chargesErrorMessage(err, { itemName: entry.namePt, left: 0 }) };
    }
  }

  /** "Adicionar munição" or the hint's "Adicionar": the owner while the sheet is open to editing, the master always. */
  openAdd(hint: AmmunitionHint | null): void {
    const vm = this.host.vm();
    if (!vm) {
      return;
    }
    this.clear();
    const current: Record<string, number> = {};
    for (const a of vm.fullSheet?.ammunition ?? []) {
      current[a.key] = a.quantity;
    }
    this.host
      .openAdd({
        kind: hint?.key,
        quantity: hint?.quantity,
        current,
        add: (key, amount) => this.addAmmunition(key, amount),
      })
      .subscribe();
  }

  /**
   * Adds `amount` pieces to what the character holds. `SetAmmunition` sets a total and has no delta, so the sheet is read again
   * first: a shot fired while the sheet was open is not given back. (Two writers within the same instant can still cross.)
   */
  private async addAmmunition(key: string, amount: number): Promise<string> {
    if (!this.host.vm()) {
      return 'A ficha não está aberta.';
    }
    try {
      await this.host.reload();
      const vm = this.host.vm();
      if (!vm) {
        return 'A ficha não está aberta.';
      }
      const held = vm.fullSheet?.ammunition.find((a) => a.key === key)?.quantity ?? 0;
      const total = Math.min(held + amount, MOST_AMMUNITION);
      const ammo = await this.host
        .client()
        .setAmmunition(this.host.campaignId(), vm.id, key, total);
      await this.host.reload();
      const name = ammo?.namePt || AMMUNITION_KINDS.find((k) => k.key === key)?.namePt || '';
      this.ammunitionNotice.set(`${name}: ${ammo?.quantity ?? total}.`);
      return '';
    } catch (err) {
      return describeCharacterError(err);
    }
  }

  /** "Recuperar a metade": half of what the latest combat fired, once. */
  async recover(): Promise<void> {
    const vm = this.host.vm();
    if (!vm || this.recovering) {
      return;
    }
    this.recovering = true;
    this.clear();
    try {
      const list = await this.host
        .client()
        .recoverAmmunition(
          this.host.campaignId(),
          vm.id,
          this.recoverKey.keyFor({ character: vm.id }),
        );
      this.recoverKey.renew();
      this.combatRunning.set(false);
      await this.host.reload();
      const done = recoverOf(ammunitionOf(list));
      this.ammunitionNotice.set(
        done.count > 0 ? `Recuperou ${done.pieces} deste combate.` : 'Nada a recuperar.',
      );
    } catch (err) {
      const blocked = chargesBlocked(err);
      if (blocked?.reason === ChargesBlockedReason.COMBAT_RUNNING) {
        this.combatRunning.set(true);
        return;
      }
      if (blocked?.reason === ChargesBlockedReason.ALREADY_RECOVERED) {
        await this.host.reload();
      }
      this.ammunitionError.set(
        blocked ? chargesBlockedMessage(blocked) : describeCharacterError(err),
      );
    } finally {
      this.recovering = false;
    }
  }

  private clear(): void {
    this.result.set(null);
    this.error.set('');
    this.ammunitionNotice.set('');
    this.ammunitionError.set('');
  }
}
