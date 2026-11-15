import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import { of } from 'rxjs';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  ConsumableTargetSchema,
  ConsumableTargetState,
  UseConsumableResponseSchema,
} from '../../../../gen/meurpg/play/v1/charges_pb';
import {
  ConsumableUseOutcome,
  EncounterBlockedReason,
  EncounterBlockedSchema,
} from '../../../../gen/meurpg/play/v1/combat_pb';
import type { AttunementEntryVm } from '../../../core/characters/attunement';
import type { ChargesClient } from '../../../core/play/charges-client';
import type { CharacterSheetVm } from '../character-sheet.types';
import { SheetConsumables, type SheetConsumablesHost } from './sheet-consumables';
import type { UseConsumableData } from './use-consumable-sheet';

const entry = (id: string, over: Partial<AttunementEntryVm> = {}): AttunementEntryVm => ({
  id,
  key: 'item:potion-of-healing',
  namePt: 'Poção de cura',
  kind: 'magic',
  attuned: false,
  asks: false,
  restrictionPt: '',
  restriction: 'none',
  sameItemAttuned: false,
  effectPt: 'Cura 2d4 + 2',
  ...over,
});

const gone = (reason: EncounterBlockedReason) =>
  new ConnectError('x', Code.FailedPrecondition, undefined, [
    { desc: EncounterBlockedSchema, value: create(EncounterBlockedSchema, { reason }) },
  ]);

describe('SheetConsumables', () => {
  const client = { useConsumable: vi.fn(), listConsumableTargets: vi.fn() };
  const reload = vi.fn();
  const focusConsumable = vi.fn();
  const dice = vi.fn();
  let opened: UseConsumableData | undefined;
  let entries: AttunementEntryVm[];
  let isMaster: boolean;
  let consumables: SheetConsumables;

  beforeEach(() => {
    opened = undefined;
    isMaster = false;
    entries = [entry('p1'), entry('p2'), entry('r1', { key: 'item:ring', namePt: 'Anel' })];
    Object.values(client).forEach((f) => f.mockReset());
    reload.mockReset().mockResolvedValue(true);
    focusConsumable.mockReset();
    dice.mockReset().mockResolvedValue({
      mode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
    });
    const host: SheetConsumablesHost = {
      client: () => client as unknown as ChargesClient,
      campaignId: () => 'camp',
      vm: () =>
        ({
          id: 'c1',
          name: 'Tor',
          isMaster,
          sheet: { kind: 'full', attunement: { entries, attuned: 0 } },
        }) as unknown as CharacterSheetVm,
      reload,
      dice,
      focusConsumable,
      categoryOf: (key) => (key === 'item:potion-of-healing' ? 'potion' : ''),
      openUse: (data) => {
        opened = data;
        return of(true);
      },
    };
    consumables = new SheetConsumables(host);
  });

  async function open(over: AttunementEntryVm = entries[0]) {
    consumables.openUse(over);
    await vi.waitFor(() => expect(opened).toBeDefined());
    return opened!;
  }

  it('opens the sheet with the item, what it does, how many are left and how the dice come', async () => {
    const data = await open();
    expect(data).toMatchObject({
      itemName: 'Poção de cura',
      itemKey: 'item:potion-of-healing',
      kind: 'potion',
      effectPt: 'Cura 2d4 + 2',
      left: 2,
      actingFor: null,
      inCombat: null,
      canApp: true,
      canType: true,
      preferApp: true,
    });
  });

  it('opens for the master as the character it acts for', async () => {
    isMaster = true;
    const data = await open();
    expect(data.actingFor).toBe('Tor');
  });

  it('plays the physical dice of a table that forces them', async () => {
    dice.mockResolvedValue({ mode: DiceMode.PHYSICAL, preference: DicePreference.PHYSICAL });
    const data = await open();
    expect([data.canApp, data.canType, data.preferApp]).toEqual([false, true, false]);
  });

  it('lists the receivers the server sends, worded for the board', async () => {
    client.listConsumableTargets.mockResolvedValue([
      create(ConsumableTargetSchema, {
        characterId: 'c2',
        name: 'Brisa',
        distanceFt: 5,
        state: ConsumableTargetState.DOWN,
        mayReceive: true,
      }),
    ]);
    const data = await open();
    const out = await data.loadTargets();
    expect(client.listConsumableTargets).toHaveBeenCalledWith('camp', 'c1', 'p1');
    expect(out).toEqual({
      targets: [
        { id: 'c2', name: 'Brisa', distanceFt: 5, down: true, line: 'A 1,5\u00a0m · Caída' },
      ],
      error: '',
    });
  });

  it('says why the list could not be read, in words', async () => {
    client.listConsumableTargets.mockRejectedValue(gone(EncounterBlockedReason.ITEM_GONE));
    const data = await open();
    expect(await data.loadTargets()).toEqual({
      targets: [],
      error: 'Esse item não está mais na ficha: alguém já o usou ou o tirou. Nada foi gasto.',
    });
  });

  it('drinks: sends the id of one entry of the group, reads the sheet and says what happened', async () => {
    client.useConsumable.mockResolvedValue(
      create(UseConsumableResponseSchema, {
        outcome: ConsumableUseOutcome.HEALED,
        diceText: '2d4+2',
        faces: [3, 2],
        rollTotal: 7,
        healed: 7,
      }),
    );
    const data = await open();
    const why = await data.use({ target: null, roll: { inApp: true } }, 'k1');
    expect(why).toBe('');
    expect(client.useConsumable).toHaveBeenCalledWith(
      'camp',
      'c1',
      'p1',
      '',
      { inApp: true },
      'k1',
    );
    expect(reload).toHaveBeenCalled();
    expect(consumables.result()).toEqual({
      tone: 'good',
      icon: 'favorite',
      headline: 'Tor recebeu 7 PV.',
      detail: '2d4 + 2: 3 + 2 + 2 = 7. Resta 1 Poção de cura.',
      text: [],
    });
  });

  it('gives it to a character who is down, and says they wake', async () => {
    client.useConsumable.mockResolvedValue(
      create(UseConsumableResponseSchema, {
        outcome: ConsumableUseOutcome.HEALED,
        faces: [3, 2],
        rollTotal: 7,
        healed: 7,
      }),
    );
    const data = await open();
    await data.use(
      {
        target: {
          id: 'c2',
          name: 'Brisa',
          distanceFt: 5,
          down: true,
          line: 'A 1,5\u00a0m · Caída',
        },
        roll: { faces: [3, 2] },
      },
      'k2',
    );
    expect(client.useConsumable).toHaveBeenCalledWith(
      'camp',
      'c1',
      'p1',
      'c2',
      { faces: [3, 2] },
      'k2',
    );
    expect(consumables.result()?.headline).toBe('Brisa recebeu 7 PV e acorda.');
  });

  it('counts the units that are left from the group it opened with', async () => {
    client.useConsumable.mockResolvedValue(
      create(UseConsumableResponseSchema, { outcome: ConsumableUseOutcome.TEXT_ONLY }),
    );
    entries = [entry('p1', { effectPt: '' })];
    const data = await open(entries[0]);
    await data.use({ target: null, roll: { inApp: true } }, 'k');
    expect(consumables.result()?.headline).toBe('Tor bebeu a Poção de cura.');
  });

  it('refuses in words and reads the sheet again when the item is gone', async () => {
    client.useConsumable.mockRejectedValue(gone(EncounterBlockedReason.ITEM_GONE));
    const data = await open();
    const why = await data.use({ target: null, roll: { inApp: true } }, 'k1');
    expect(why).toBe(
      'Esse item não está mais na ficha: alguém já o usou ou o tirou. Nada foi gasto.',
    );
    expect(reload).toHaveBeenCalled();
    expect(consumables.result()).toBeNull();
  });

  it('says the receiver is out of reach with the sentence of the combat', async () => {
    client.useConsumable.mockRejectedValue(gone(EncounterBlockedReason.TARGET_NOT_ADJACENT));
    const data = await open();
    const why = await data.use(
      {
        target: { id: 'c2', name: 'Brisa', distanceFt: 5, down: false, line: '' },
        roll: { inApp: true },
      },
      'k1',
    );
    expect(why).toBe(
      'Quem recebe o item precisa estar a até 1,5 m de quem o dá, e os dois na luta ou os dois fora dela. Nada foi gasto.',
    );
  });

  it('names the character the master acts for when it is not their turn', async () => {
    isMaster = true;
    client.useConsumable.mockRejectedValue(gone(EncounterBlockedReason.NOT_YOUR_TURN));
    const data = await open();
    expect(await data.use({ target: null, roll: { inApp: true } }, 'k1')).toBe(
      'Só na vez de Tor: beber ou dar gasta a ação.',
    );
  });

  it('does not read the sheet again for a call that did not get through: nothing changed', async () => {
    client.useConsumable.mockRejectedValue(new ConnectError('x', Code.Unavailable));
    const data = await open();
    const why = await data.use({ target: null, roll: { inApp: true } }, 'k1');
    expect(why).toBe('Não deu para usar a poção. Nada foi gasto. Tente de novo.');
    expect(reload).not.toHaveBeenCalled();
  });

  it('puts the focus back on the row of the item once the sheet closes', async () => {
    await open();
    expect(focusConsumable).toHaveBeenCalledWith('item:potion-of-healing');
  });

  it('clears the last notice when the sheet opens again', async () => {
    client.useConsumable.mockResolvedValue(
      create(UseConsumableResponseSchema, { outcome: ConsumableUseOutcome.TEXT_ONLY }),
    );
    const data = await open();
    await data.use({ target: null, roll: { inApp: true } }, 'k');
    expect(consumables.result()).not.toBeNull();
    consumables.openUse(entries[0]);
    expect(consumables.result()).toBeNull();
  });
});
