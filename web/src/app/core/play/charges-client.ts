import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import {
  CampaignService,
  DiceMode,
  DicePreference,
} from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CharacterService,
  type ItemLoss,
  type SheetAmmunition,
  type SheetMagicItem,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import {
  ChargesService,
  type ConsumableTarget,
  type DawnResponse,
  type RollLastChargeResponse,
  type SetChargesResponse,
  type SpendChargesResponse,
  type UseConsumableResponse,
} from '../../../gen/meurpg/play/v1/charges_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** How the d20 of an item's last charge comes (RN-18): the app rolls it, or the player typed the face of a real die
 * (and, for a 20 on an item that regains charges, the face of the die that counts them). */
export type LastChargeRoll =
  { readonly inApp: true } | { readonly face: number; readonly regainFace?: number };

/** The answer of `listConsumableTargets`. */
export interface ConsumableTargets {
  readonly targets: ConsumableTarget[];
  /** The character is a combatant of a running combat. */
  readonly inCombat: boolean;
}

/** How the healing dice of a potion come (RN-18): the app rolls them, or the player typed one face for each die. */
export type ConsumableRoll = { readonly inApp: true } | { readonly faces: readonly number[] };

/** The table's dice setting and the player's own choice, which decide whether a die is typed or rolled in the app. */
export interface TableDice {
  readonly mode: DiceMode;
  readonly preference: DicePreference;
}

/**
 * Thin wrapper around the generated `ChargesService` client (the charges of magic items and the ammunition), in the
 * shape of `ResourceClient`. Only lazy code imports it. Every write that is not idempotent by nature takes the key its
 * caller made once for that request (`ActionKey`); callers word the errors with `charges-errors.ts`.
 */
@Injectable({ providedIn: 'root' })
export class ChargesClient {
  private readonly client = createClient(ChargesService, inject(CONNECT_TRANSPORT));
  private readonly campaigns = createClient(CampaignService, inject(CONNECT_TRANSPORT));
  private readonly characters = createClient(CharacterService, inject(CONNECT_TRANSPORT));

  /** Charges spent from one entry of the sheet's magic items: the owner (a locked sheet too) and the master. */
  spendCharges(
    campaignId: string,
    characterId: string,
    entryId: string,
    amount: number,
    idempotencyKey: string,
  ): Promise<SpendChargesResponse> {
    return this.client.spendCharges({ campaignId, characterId, entryId, amount, idempotencyKey });
  }

  /** The d20 of the last charge, which only an item that waits for it takes. */
  rollLastCharge(
    campaignId: string,
    characterId: string,
    entryId: string,
    roll: LastChargeRoll,
    idempotencyKey: string,
  ): Promise<RollLastChargeResponse> {
    return this.client.rollLastCharge({
      campaignId,
      characterId,
      entryId,
      idempotencyKey,
      roll:
        'inApp' in roll
          ? { case: 'rollInApp', value: true }
          : { case: 'd20Face', value: roll.face },
      typedRegainFace: 'inApp' in roll ? 0 : (roll.regainFace ?? 0),
    });
  }

  /** The master's correction: the charges left (also the start of an item that rolls it) and whether the item was lost. */
  setCharges(
    campaignId: string,
    characterId: string,
    entryId: string,
    chargesLeft: number,
    lost: ItemLoss,
    idempotencyKey: string,
  ): Promise<SetChargesResponse> {
    return this.client.setCharges({
      campaignId,
      characterId,
      entryId,
      chargesLeft,
      lost,
      idempotencyKey,
    });
  }

  /**
   * Uses up one unit of a consumable of the character's sheet (a potion): the entry is the one the caller names, the
   * receiver is the user when `targetCharacterId` is empty. The dice come the way the table says (`roll`); only a healing
   * potion rolls them. One key for the request: the same use again after a failure is a retry.
   */
  useConsumable(
    campaignId: string,
    characterId: string,
    entryId: string,
    targetCharacterId: string,
    roll: ConsumableRoll,
    idempotencyKey: string,
  ): Promise<UseConsumableResponse> {
    return this.client.useConsumable({
      campaignId,
      characterId,
      entryId,
      targetCharacterId,
      idempotencyKey,
      rollInApp: 'inApp' in roll,
      typedFaces: 'inApp' in roll ? [] : [...roll.faces],
    });
  }

  /**
   * Who the character may give the consumable to (the adjacent player characters in a combat, the party out of one) and
   * whether the character is in a running combat, which the server tests the same way as the list.
   */
  async listConsumableTargets(
    campaignId: string,
    characterId: string,
    entryId: string,
  ): Promise<ConsumableTargets> {
    const res = await this.client.listConsumableTargets({ campaignId, characterId, entryId });
    return { targets: res.targets, inCombat: res.inCombat };
  }

  /** The pieces of one kind of ammunition (0 to 9999): the owner while the sheet is open to editing, the master always. */
  async setAmmunition(
    campaignId: string,
    characterId: string,
    ammoKey: string,
    quantity: number,
  ): Promise<SheetAmmunition | undefined> {
    const res = await this.client.setAmmunition({ campaignId, characterId, ammoKey, quantity });
    return res.ammunition;
  }

  /** Half of what each kind fired in the character's latest combat comes back, once for each combat. Answers the list. */
  async recoverAmmunition(
    campaignId: string,
    characterId: string,
    idempotencyKey: string,
  ): Promise<SheetAmmunition[]> {
    const res = await this.client.recoverAmmunition({ campaignId, characterId, idempotencyKey });
    return res.ammunition;
  }

  /** The master's "Amanhecer": the items of every character of the campaign get their dawn. */
  dawn(campaignId: string, idempotencyKey: string): Promise<DawnResponse> {
    return this.client.dawn({ campaignId, idempotencyKey });
  }

  /** The magic items and the counted ammunition of a character's sheet, for the master's panel (the master reads every sheet). */
  async sheetOf(
    campaignId: string,
    characterId: string,
  ): Promise<{ magicItems: SheetMagicItem[]; ammunition: SheetAmmunition[] }> {
    const res = await this.characters.getCharacter({ campaignId, characterId });
    const content = res.character?.sheet?.content;
    return content?.case === 'full'
      ? { magicItems: content.value.magicItems, ammunition: content.value.ammunition }
      : { magicItems: [], ammunition: [] };
  }

  /** The campaign's dice mode and the caller's choice, as `GetCampaign` says them. */
  async diceOf(campaignId: string): Promise<TableDice> {
    const res = await this.campaigns.getCampaign({ campaignId });
    return {
      mode: res.campaign?.diceMode ?? DiceMode.PLAYERS_CHOOSE,
      preference: res.campaign?.myDicePreference ?? DicePreference.APP,
    };
  }
}
