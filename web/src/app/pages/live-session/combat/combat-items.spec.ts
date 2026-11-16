import { create } from '@bufbuild/protobuf';
import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  ConsumableKind,
  SheetMagicItemSchema,
} from '../../../../gen/meurpg/characters/v1/characters_pb';
import { UseConsumableResponseSchema } from '../../../../gen/meurpg/play/v1/charges_pb';
import { ConsumableUseOutcome } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { TurnBlock } from '../../../core/characters/consumables';
import type { ChargesClient } from '../../../core/play/charges-client';
import type { UseConsumableData } from '../../../shared/consumable-use/use-consumable-sheet';
import { CombatItems } from './combat-items';

const potion = (id: string, key = 'item:potion-of-healing', namePt = 'Poção de cura') =>
  create(SheetMagicItemSchema, {
    id,
    key,
    namePt,
    effectPt: 'Cura 2d4 + 2',
    consumable: {
      kind: ConsumableKind.POTION,
      costsAction: true,
      healDiceCount: 2,
      healDiceSides: 4,
      healBonus: 2,
    },
  });

describe('CombatItems', () => {
  const client = { sheetOf: vi.fn(), useConsumable: vi.fn(), listConsumableTargets: vi.fn() };
  const focusItem = vi.fn();
  let opened: UseConsumableData | undefined;
  const characterId = signal('c1');
  const byMaster = signal(false);
  const signature = signal('1');
  const enabled = signal(true);
  const turnBlock = signal<TurnBlock | null>(null);

  function build() {
    return TestBed.runInInjectionContext(
      () =>
        new CombatItems({
          client: () => client as unknown as ChargesClient,
          campaignId: () => 'camp',
          characterId,
          name: () => 'Tor',
          byMaster,
          enabled,
          signature,
          turnBlock,
          dice: () =>
            Promise.resolve({ mode: DiceMode.PLAYERS_CHOOSE, preference: DicePreference.APP }),
          openUse: (data) => {
            opened = data;
            return of(true);
          },
          focusItem,
        }),
    );
  }
  const settle = async () => {
    TestBed.tick();
    await Promise.resolve();
    await Promise.resolve();
  };

  beforeEach(() => {
    opened = undefined;
    characterId.set('c1');
    byMaster.set(false);
    signature.set('1');
    enabled.set(true);
    turnBlock.set(null);
    Object.values(client).forEach((f) => f.mockReset());
    focusItem.mockReset();
    client.sheetOf.mockResolvedValue({
      magicItems: [
        potion('p1'),
        potion('p2'),
        create(SheetMagicItemSchema, { id: 'r', key: 'item:ring', namePt: 'Anel' }),
      ],
      ammunition: [],
    });
  });

  it("reads the character's items when it is the player's turn, and groups the consumables the server says are one", async () => {
    const items = build();
    await settle();
    expect(client.sheetOf).toHaveBeenCalledWith('camp', 'c1');
    expect(items.rows().map((r) => [r.namePt, r.count, r.economy])).toEqual([
      ['Poção de cura', 2, 'action'],
    ]);
  });

  it("reads nothing off the player's turn", async () => {
    enabled.set(false);
    build();
    await settle();
    expect(client.sheetOf).not.toHaveBeenCalled();
  });

  it('reads again when what the combat changes about the turn changes, and for another character', async () => {
    build();
    await settle();
    signature.set('2');
    await settle();
    characterId.set('c2');
    await settle();
    expect(client.sheetOf.mock.calls.map((c) => c[1])).toEqual(['c1', 'c1', 'c2']);
  });

  it('keeps what it shows when a read fails', async () => {
    const items = build();
    await settle();
    client.sheetOf.mockRejectedValue(new Error('x'));
    signature.set('2');
    await settle();
    expect(items.rows()).toHaveLength(1);
  });

  it('opens the "Usar" sheet of the combat: a combat is running, and the master acts for the character', async () => {
    const items = build();
    await settle();
    byMaster.set(true);
    items.open(items.rows()[0].first);
    await vi.waitFor(() => expect(opened).toBeDefined());
    expect(opened).toMatchObject({
      itemName: 'Poção de cura',
      kind: 'potion',
      left: 2,
      inCombat: true,
      actingFor: 'Tor',
    });
    expect(focusItem).toHaveBeenCalledWith('item:potion-of-healing');
  });

  it('tells the sheet the Action is spent when the combat says so, for the row folded under "Ação · Usada"', async () => {
    const items = build();
    await settle();
    turnBlock.set('action');
    items.open(items.rows()[0].first);
    await vi.waitFor(() => expect(opened).toBeDefined());
    expect(opened?.blockedBy).toBe('action');
  });

  it('tells the sheet it is not the turn of the character', async () => {
    const items = build();
    await settle();
    turnBlock.set('turn');
    items.open(items.rows()[0].first);
    await vi.waitFor(() => expect(opened).toBeDefined());
    expect(opened?.blockedBy).toBe('turn');
  });

  it('opens the sheet live while the turn and the Action are the character’s', async () => {
    const items = build();
    await settle();
    items.open(items.rows()[0].first);
    await vi.waitFor(() => expect(opened).toBeDefined());
    expect(opened?.blockedBy).toBeNull();
  });

  it('reads the items again after a use and says what it did, with one unit less', async () => {
    const items = build();
    await settle();
    client.useConsumable.mockResolvedValue(
      create(UseConsumableResponseSchema, {
        outcome: ConsumableUseOutcome.HEALED,
        diceText: '2d4+2',
        faces: [3, 2],
        rollTotal: 7,
        healed: 7,
      }),
    );
    client.sheetOf.mockResolvedValue({ magicItems: [potion('p2')], ammunition: [] });
    items.open(items.rows()[0].first);
    await vi.waitFor(() => expect(opened).toBeDefined());
    expect(await opened!.use({ target: null, roll: { inApp: true } }, 'k1')).toEqual({
      refusal: null,
    });
    expect(client.useConsumable).toHaveBeenCalledWith(
      'camp',
      'c1',
      'p1',
      '',
      { inApp: true },
      'k1',
    );
    expect(items.rows()[0].count).toBe(1);
    expect(items.notice()).toMatchObject({
      headline: 'Tor recebeu 7 PV.',
      detail: '2d4 + 2: 3 + 2 + 2 = 7. Resta 1 Poção de cura.',
    });
  });

  it('lists an item the server says is no consumable as none, whatever its key', async () => {
    client.sheetOf.mockResolvedValue({
      magicItems: [
        potion('x', 'item:potion-of-healing', 'Poção sem campo'),
        create(SheetMagicItemSchema, {
          id: 'y',
          key: 'item:potion-of-healing',
          namePt: 'Sem consumível',
        }),
      ],
      ammunition: [],
    });
    const items = build();
    await settle();
    expect(items.rows().map((r) => r.namePt)).toEqual(['Poção sem campo']);
  });

  it('forgets what the last use said', async () => {
    const items = build();
    await settle();
    items.use.result.set({ tone: 'good', icon: 'favorite', headline: 'x', detail: '', text: [] });
    items.forgetNotice();
    expect(items.notice()).toBeNull();
  });
});
