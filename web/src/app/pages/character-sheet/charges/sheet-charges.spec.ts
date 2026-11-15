import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import { Subject, of } from 'rxjs';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
  ItemLoss,
  ItemRechargeKind,
  ItemRechargeSchema,
  SheetAmmunitionSchema,
  SheetMagicItemSchema,
} from '../../../../gen/meurpg/characters/v1/characters_pb';
import {
  ChargesBlockedReason,
  ChargesBlockedSchema,
  RollLastChargeResponseSchema,
  SpendChargesResponseSchema,
} from '../../../../gen/meurpg/play/v1/charges_pb';
import type { AttunementEntryVm } from '../../../core/characters/attunement';
import type { ChargesClient } from '../../../core/play/charges-client';
import type { CharacterSheetVm } from '../character-sheet.types';
import type { AddAmmunitionData } from './add-ammunition-sheet';
import type { LastChargeData } from './last-charge-sheet';
import { SheetCharges, type SheetChargesHost } from './sheet-charges';
import type { UseChargesData } from './use-charges-sheet';

const entry = (over: Partial<AttunementEntryVm> = {}): AttunementEntryVm => ({
  id: 'w1',
  key: 'item:wand-of-web',
  namePt: 'Varinha de teia',
  kind: 'magic',
  attuned: false,
  asks: false,
  restrictionPt: '',
  restriction: 'none',
  sameItemAttuned: false,
  charges: { max: 7, left: 5, state: 'ready' },
  ...over,
});

const refusal = (reason: CharacterBlockedReason) =>
  new ConnectError('x', Code.FailedPrecondition, undefined, [
    { desc: CharacterBlockedSchema, value: create(CharacterBlockedSchema, { reason }) },
  ]);
const blocked = (reason: ChargesBlockedReason, needed = 0) =>
  new ConnectError('x', Code.FailedPrecondition, undefined, [
    { desc: ChargesBlockedSchema, value: create(ChargesBlockedSchema, { reason, needed }) },
  ]);

describe('SheetCharges', () => {
  const client = {
    spendCharges: vi.fn(),
    rollLastCharge: vi.fn(),
    setAmmunition: vi.fn(),
    recoverAmmunition: vi.fn(),
    diceOf: vi.fn(),
  };
  let held = 15;
  let opened: { use?: UseChargesData; roll?: LastChargeData; add?: AddAmmunitionData };
  let rollOpened: number;
  const reload = vi.fn();
  const focusRow = vi.fn();
  let charges: SheetCharges;

  function make(over: Partial<SheetChargesHost> = {}) {
    opened = {};
    rollOpened = 0;
    charges = new SheetCharges({
      client: () => client as unknown as ChargesClient,
      campaignId: () => 'camp',
      vm: () =>
        ({
          id: 'c1',
          name: 'Pensantus',
          fullSheet: {
            ammunition: [create(SheetAmmunitionSchema, { key: 'equipment:arrow', quantity: held })],
          },
        }) as unknown as CharacterSheetVm,
      reload,
      focusRow,
      openUse: (data) => {
        opened.use = data;
        return of(true);
      },
      openRoll: (data) => {
        opened.roll = data;
        rollOpened += 1;
        return of(true);
      },
      openAdd: (data) => {
        opened.add = data;
        return of(true);
      },
      ...over,
    });
  }

  beforeEach(() => {
    held = 15;
    focusRow.mockReset();
    Object.values(client).forEach((f) => f.mockReset());
    reload.mockReset().mockResolvedValue(true);
    client.diceOf.mockResolvedValue({
      mode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
    });
    make();
  });

  describe('the focus after a sheet closes', () => {
    it('goes back to the row of the item when the use sheet closes', () => {
      charges.openUse(entry());
      expect(focusRow).toHaveBeenCalledWith('w1');
    });

    it('goes back to the row of the item when the d20 sheet closes, and not before', async () => {
      charges.openRoll(entry({ charges: { max: 7, left: 0, state: 'pending' } }));
      await vi.waitFor(() => expect(focusRow).toHaveBeenCalledWith('w1'));
      expect(focusRow).toHaveBeenCalledTimes(1);
    });

    it('waits for the d20 that the last charge opens, and returns once, after it', async () => {
      client.spendCharges.mockResolvedValue(
        create(SpendChargesResponseSchema, {
          item: create(SheetMagicItemSchema, {
            chargesMax: 7,
            chargesLeft: 0,
            lastRollPending: true,
          }),
        }),
      );
      const order: string[] = [];
      const closed = new Subject<boolean>();
      make({
        openUse: (data) => {
          opened.use = data;
          return closed;
        },
        openRoll: (data) => {
          opened.roll = data;
          order.push('roll');
          return of(true);
        },
      });
      focusRow.mockImplementation(() => order.push('focus'));
      charges.openUse(entry());
      await opened.use!.spend(1, 'k1');
      expect(order).toEqual([]);
      closed.next(true);
      closed.complete();
      await vi.waitFor(() => expect(order).toEqual(['roll', 'focus']));
    });
  });

  describe('the recharge rules', () => {
    const dawn = create(ItemRechargeSchema, { kind: ItemRechargeKind.DAWN, regainDice: '1d6+1' });

    it('hand the use sheet the line that says when the charges come back', () => {
      charges.openUse(entry({ charges: { max: 7, left: 5, state: 'ready', recharge: dawn } }));
      expect(opened.use?.rechargeNote).toBe('Volta 1d6 + 1 ao amanhecer, até 7.');
      charges.openUse(entry());
      expect(opened.use?.rechargeNote).toBeUndefined();
    });

    it('hand the d20 sheet the rules of the item', async () => {
      charges.openRoll(entry({ charges: { max: 7, left: 0, state: 'pending', recharge: dawn } }));
      await vi.waitFor(() => expect(opened.roll).toBeDefined());
      expect(opened.roll?.recharge).toBe(dawn);
    });

    it('say in a refusal that the item comes back at dawn', async () => {
      client.spendCharges.mockRejectedValue(refusal(CharacterBlockedReason.ITEM_NO_CHARGES));
      charges.openUse(
        entry({
          namePt: 'Varinha de segredos',
          charges: { max: 3, left: 5, state: 'ready', recharge: dawn },
        }),
      );
      expect(await opened.use!.spend(1, 'k')).toBe(
        'Sem cargas. A Varinha de segredos volta ao amanhecer.',
      );
    });
  });

  describe('using charges', () => {
    it('opens the use sheet with what the item holds', () => {
      charges.openUse(entry());
      expect(opened.use).toMatchObject({ itemName: 'Varinha de teia', max: 7, left: 5 });
    });

    it('spends the amount on the entry under the key the sheet gives, reads the sheet again and says what is left', async () => {
      client.spendCharges.mockResolvedValue(
        create(SpendChargesResponseSchema, {
          item: create(SheetMagicItemSchema, { chargesMax: 7, chargesLeft: 3 }),
        }),
      );
      charges.openUse(entry());
      const why = await opened.use!.spend(2, 'k1');
      expect(why).toBe('');
      expect(client.spendCharges).toHaveBeenCalledWith('camp', 'c1', 'w1', 2, 'k1');
      expect(reload).toHaveBeenCalled();
      expect(charges.result()).toEqual({
        text: 'Gastou 2 cargas da Varinha de teia. Restam 3 de 7.',
        tone: 'good',
      });
    });

    it('opens the d20 right away when that was the last charge of an item that asks for it', async () => {
      client.spendCharges.mockResolvedValue(
        create(SpendChargesResponseSchema, {
          item: create(SheetMagicItemSchema, {
            chargesMax: 7,
            chargesLeft: 0,
            lastRollPending: true,
          }),
        }),
      );
      const closed = new Subject<boolean>();
      make({
        openUse: (data) => {
          opened.use = data;
          return closed;
        },
      });
      charges.openUse(entry({ charges: { max: 7, left: 1, state: 'ready' } }));
      await opened.use!.spend(1, 'k1');
      expect(rollOpened).toBe(0);
      closed.next(true);
      await vi.waitFor(() => expect(rollOpened).toBe(1));
    });

    it('says an item lost with its last charge', async () => {
      client.spendCharges.mockResolvedValue(
        create(SpendChargesResponseSchema, {
          item: create(SheetMagicItemSchema, {
            chargesMax: 7,
            chargesLeft: 0,
            lost: ItemLoss.GONE,
          }),
        }),
      );
      charges.openUse(entry({ charges: { max: 7, left: 1, state: 'ready' } }));
      await opened.use!.spend(1, 'k1');
      expect(charges.result()).toEqual({
        text: 'Gastou a última carga. A Varinha de teia se perdeu: fica na ficha como destruída.',
        tone: 'bad',
      });
      expect(rollOpened).toBe(0);
    });

    it('words the refusals with the item and reads the sheet again', async () => {
      client.spendCharges.mockRejectedValue(refusal(CharacterBlockedReason.CHARGES_NOT_ENOUGH));
      charges.openUse(entry());
      const why = await opened.use!.spend(5, 'k1');
      expect(why).toBe('Cargas a menos. A Varinha de teia só tem 5 cargas.');
      expect(reload).toHaveBeenCalled();
    });

    it('says nothing changed when the call did not get through, and does not read again', async () => {
      client.spendCharges.mockRejectedValue(new ConnectError('x', Code.Unavailable));
      charges.openUse(entry());
      expect(await opened.use!.spend(1, 'k1')).toBe(
        'Não foi possível gastar a carga. Nada mudou. Tente de novo.',
      );
      expect(reload).not.toHaveBeenCalled();
    });

    it('does nothing for an entry with no charges', () => {
      charges.openUse(entry({ charges: undefined }));
      expect(opened.use).toBeUndefined();
    });
  });

  describe('the d20 of the last charge', () => {
    const pending = entry({ charges: { max: 7, left: 0, state: 'pending' } });

    it('follows the table dice: the app and the player’s choice (RN-18)', async () => {
      client.diceOf.mockResolvedValue({ mode: DiceMode.PHYSICAL, preference: DicePreference.APP });
      charges.openRoll(pending);
      await vi.waitFor(() => expect(opened.roll).toBeDefined());
      expect(opened.roll).toMatchObject({ canApp: false, canType: true, preferApp: false });
    });

    it('lets a player who rolls their own dice lead with the face at a table that lets them choose', async () => {
      client.diceOf.mockResolvedValue({
        mode: DiceMode.PLAYERS_CHOOSE,
        preference: DicePreference.PHYSICAL,
      });
      charges.openRoll(pending);
      await vi.waitFor(() => expect(opened.roll).toBeDefined());
      expect(opened.roll).toMatchObject({ canApp: true, canType: true, preferApp: false });
    });

    it('rolls in the app and tells "Tirou 14" with the item staying', async () => {
      client.rollLastCharge.mockResolvedValue(
        create(RollLastChargeResponseSchema, {
          d20: 14,
          item: create(SheetMagicItemSchema, { chargesMax: 7, chargesLeft: 0 }),
        }),
      );
      charges.openRoll(pending);
      await vi.waitFor(() => expect(opened.roll).toBeDefined());
      const out = await opened.roll!.roll({ inApp: true }, 'k1');
      expect(out).toEqual({ kind: 'done', d20: 14 });
      expect(client.rollLastCharge).toHaveBeenCalledWith('camp', 'c1', 'w1', { inApp: true }, 'k1');
      expect(reload).toHaveBeenCalled();
      expect(charges.result()).toEqual({
        text: 'Tirou 14. A Varinha de teia fica, sem cargas.',
        tone: 'good',
      });
    });

    it('tells a stay that the item comes back at dawn, when its rules say so', async () => {
      client.rollLastCharge.mockResolvedValue(create(RollLastChargeResponseSchema, { d20: 14 }));
      charges.openRoll(
        entry({
          charges: {
            max: 7,
            left: 0,
            state: 'pending',
            recharge: create(ItemRechargeSchema, {
              kind: ItemRechargeKind.DAWN,
              regainDice: '1d6+1',
            }),
          },
        }),
      );
      await vi.waitFor(() => expect(opened.roll).toBeDefined());
      await opened.roll!.roll({ inApp: true }, 'k1');
      expect(charges.result()?.text).toBe(
        'Tirou 14. A Varinha de teia fica, sem cargas, e volta ao amanhecer.',
      );
    });

    it('tells a 1 that lost the item', async () => {
      client.rollLastCharge.mockResolvedValue(
        create(RollLastChargeResponseSchema, { d20: 1, lost: ItemLoss.GONE }),
      );
      charges.openRoll(pending);
      await vi.waitFor(() => expect(opened.roll).toBeDefined());
      await opened.roll!.roll({ face: 1 }, 'k1');
      expect(charges.result()).toEqual({
        text: 'Tirou 1. A Varinha de teia virou cinzas: fica na ficha como destruída.',
        tone: 'bad',
      });
    });

    it('tells an item that lost only its magic', async () => {
      client.rollLastCharge.mockResolvedValue(
        create(RollLastChargeResponseSchema, { d20: 1, lost: ItemLoss.PLAIN_ITEM }),
      );
      charges.openRoll(pending);
      await vi.waitFor(() => expect(opened.roll).toBeDefined());
      await opened.roll!.roll({ face: 1 }, 'k1');
      expect(charges.result()?.text).toBe(
        'Tirou 1. A Varinha de teia perdeu a magia: virou um item comum.',
      );
      client.rollLastCharge.mockResolvedValue(
        create(RollLastChargeResponseSchema, { d20: 1, lost: ItemLoss.PARTLY_MAGIC }),
      );
      await opened.roll!.roll({ face: 1 }, 'k2');
      expect(charges.result()?.text).toBe(
        'Tirou 1. A Varinha de teia perdeu a magia: ficou só o bônus no ataque e no dano.',
      );
    });

    it('tells a 20 that gave charges back', async () => {
      client.rollLastCharge.mockResolvedValue(
        create(RollLastChargeResponseSchema, { d20: 20, regained: 3 }),
      );
      charges.openRoll(pending);
      await vi.waitFor(() => expect(opened.roll).toBeDefined());
      await opened.roll!.roll({ face: 20, regainFace: 3 }, 'k1');
      expect(client.rollLastCharge).toHaveBeenCalledWith(
        'camp',
        'c1',
        'w1',
        { face: 20, regainFace: 3 },
        'k1',
      );
      expect(charges.result()).toEqual({
        text: 'Tirou 20. A Varinha de teia recuperou 3 cargas.',
        tone: 'good',
      });
    });

    it('asks the sheet for the face of the regain die when the server needs it', async () => {
      client.rollLastCharge.mockRejectedValue(blocked(ChargesBlockedReason.REGAIN_FACE_NEEDED, 6));
      charges.openRoll(pending);
      await vi.waitFor(() => expect(opened.roll).toBeDefined());
      expect(await opened.roll!.roll({ face: 20 }, 'k1')).toEqual({ kind: 'regain', faces: 6 });
    });

    it('words any other refusal', async () => {
      client.rollLastCharge.mockRejectedValue(new ConnectError('x', Code.Unavailable));
      charges.openRoll(pending);
      await vi.waitFor(() => expect(opened.roll).toBeDefined());
      expect(await opened.roll!.roll({ inApp: true }, 'k1')).toEqual({
        kind: 'error',
        text: 'Não foi possível gastar a carga. Nada mudou. Tente de novo.',
      });
    });
  });

  describe('ammunition', () => {
    it('opens "Adicionar munição" with what the sheet holds, and with the hint’s kind and count', () => {
      charges.openAdd({ name: 'Flechas (20)', key: 'equipment:arrow', quantity: 20 });
      expect(opened.add).toMatchObject({
        kind: 'equipment:arrow',
        quantity: 20,
        current: { 'equipment:arrow': 15 },
      });
      charges.openAdd(null);
      expect(opened.add?.kind).toBeUndefined();
    });

    it('adds the pieces to what the sheet holds, reads the sheet again and says it', async () => {
      client.setAmmunition.mockResolvedValue(
        create(SheetAmmunitionSchema, { key: 'equipment:arrow', namePt: 'Flechas', quantity: 35 }),
      );
      charges.openAdd(null);
      const why = await opened.add!.add('equipment:arrow', 20);
      expect(why).toBe('');
      expect(client.setAmmunition).toHaveBeenCalledWith('camp', 'c1', 'equipment:arrow', 35);
      expect(reload).toHaveBeenCalled();
      await vi.waitFor(() => expect(charges.ammunitionNotice()).toBe('Flechas: 35.'));
    });

    it('reads the sheet again before it adds, so a shot fired while the sheet was open is not given back', async () => {
      client.setAmmunition.mockResolvedValue(
        create(SheetAmmunitionSchema, { key: 'equipment:arrow', namePt: 'Flechas', quantity: 32 }),
      );
      charges.openAdd(null);
      // The count the sheet opened on was 15; two shots later the character holds 13.
      reload.mockImplementation(() => {
        held = 13;
        return Promise.resolve(true);
      });
      await opened.add!.add('equipment:arrow', 20);
      expect(client.setAmmunition).toHaveBeenCalledWith('camp', 'c1', 'equipment:arrow', 33);
    });

    it('never asks for more than the 9999 the server takes', async () => {
      client.setAmmunition.mockResolvedValue(
        create(SheetAmmunitionSchema, {
          key: 'equipment:arrow',
          namePt: 'Flechas',
          quantity: 9999,
        }),
      );
      held = 9990;
      charges.openAdd(null);
      await opened.add!.add('equipment:arrow', 50);
      expect(client.setAmmunition).toHaveBeenCalledWith('camp', 'c1', 'equipment:arrow', 9999);
    });

    it('words the refusal of a locked sheet', async () => {
      client.setAmmunition.mockRejectedValue(refusal(CharacterBlockedReason.SHEET_LOCKED));
      charges.openAdd(null);
      expect(await opened.add!.add('equipment:arrow', 20)).toContain('travada');
    });

    it('recovers half under one key and says what came back', async () => {
      client.recoverAmmunition.mockResolvedValue([
        create(SheetAmmunitionSchema, {
          key: 'equipment:arrow',
          namePt: 'Flechas',
          quantity: 18,
          spentLastCombat: 6,
          recovered: true,
        }),
      ]);
      await charges.recover();
      expect(client.recoverAmmunition).toHaveBeenCalledWith('camp', 'c1', expect.any(String));
      expect(reload).toHaveBeenCalled();
      expect(charges.ammunitionNotice()).toBe('Recuperou 3 flechas deste combate.');
      expect(charges.combatRunning()).toBe(false);
    });

    it('turns the recovery off while the combat runs', async () => {
      client.recoverAmmunition.mockRejectedValue(blocked(ChargesBlockedReason.COMBAT_RUNNING));
      await charges.recover();
      expect(charges.combatRunning()).toBe(true);
      expect(charges.ammunitionError()).toBe('');
    });

    it('says it was already recovered and reads the sheet, which then shows it done', async () => {
      client.recoverAmmunition.mockRejectedValue(blocked(ChargesBlockedReason.ALREADY_RECOVERED));
      await charges.recover();
      expect(charges.ammunitionError()).toBe('A munição desse combate já foi recolhida.');
      expect(reload).toHaveBeenCalled();
    });

    it('sends the same key again after a failure, and a new one after it worked', async () => {
      client.recoverAmmunition.mockRejectedValueOnce(new ConnectError('x', Code.Unavailable));
      client.recoverAmmunition.mockResolvedValue([]);
      await charges.recover();
      await charges.recover();
      await charges.recover();
      const keys = client.recoverAmmunition.mock.calls.map((c) => c[2]);
      expect(keys[1]).toBe(keys[0]);
      expect(keys[2]).not.toBe(keys[1]);
    });

    it('does not send two recoveries at once', async () => {
      let done: (v: never[]) => void = () => undefined;
      client.recoverAmmunition.mockReturnValue(new Promise<never[]>((r) => (done = r)));
      const first = charges.recover();
      void charges.recover();
      expect(client.recoverAmmunition).toHaveBeenCalledTimes(1);
      done([]);
      await first;
    });
  });
});
