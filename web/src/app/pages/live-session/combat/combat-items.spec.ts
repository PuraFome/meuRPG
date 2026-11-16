import { create } from '@bufbuild/protobuf';
import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { SheetMagicItemSchema } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { UseConsumableResponseSchema } from '../../../../gen/meurpg/play/v1/charges_pb';
import { ConsumableUseOutcome } from '../../../../gen/meurpg/play/v1/combat_pb';
import type { ChargesClient } from '../../../core/play/charges-client';
import type { UseConsumableData } from '../../../shared/consumable-use/use-consumable-sheet';
import { CombatItems } from './combat-items';

const potion = (id: string, key = 'item:potion-of-healing', namePt = 'Poção de cura') =>
  create(SheetMagicItemSchema, { id, key, namePt, effectPt: 'Cura 2d4 + 2' });

const CATEGORY: Record<string, string> = {
  'item:potion-of-healing': 'potion',
  'item:oil-of-sharpness': 'potion',
};

describe('CombatItems', () => {
  const client = { sheetOf: vi.fn(), useConsumable: vi.fn(), listConsumableTargets: vi.fn() };
  const load = vi.fn();
  const focusItem = vi.fn();
  let opened: UseConsumableData | undefined;
  const characterId = signal('c1');
  const byMaster = signal(false);
  const signature = signal('1');
  const enabled = signal(true);

  function build() {
    return TestBed.runInInjectionContext(
      () =>
        new CombatItems({
          client: () => client as unknown as ChargesClient,
          categories: { load, categoryOf: (k) => CATEGORY[k] ?? '' },
          campaignId: () => 'camp',
          characterId,
          name: () => 'Tor',
          byMaster,
          enabled,
          signature,
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
    Object.values(client).forEach((f) => f.mockReset());
    load.mockReset().mockResolvedValue(undefined);
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

  it("reads the character's items and the catalog categories when it is the player's turn, and groups the consumables", async () => {
    const items = build();
    await settle();
    expect(load).toHaveBeenCalledWith('camp');
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
      itemKey: 'item:potion-of-healing',
      left: 2,
      inCombat: true,
      actingFor: 'Tor',
    });
    expect(focusItem).toHaveBeenCalledWith('item:potion-of-healing');
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
    expect(await opened!.use({ target: null, roll: { inApp: true } }, 'k1')).toBe('');
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

  it('forgets what the last use said', async () => {
    const items = build();
    await settle();
    items.use.result.set({ tone: 'good', icon: 'favorite', headline: 'x', detail: '', text: [] });
    items.forgetNotice();
    expect(items.notice()).toBeNull();
  });
});
