import { TestBed } from '@angular/core/testing';

import { DiceMode, DicePreference } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { ItemLoss } from '../../../gen/meurpg/characters/v1/characters_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { ChargesClient } from './charges-client';

function clientWith(calls: Record<string, unknown[]>): ChargesClient {
  TestBed.configureTestingModule({
    providers: [ChargesClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
  });
  const client = TestBed.inject(ChargesClient);
  const record = (name: string, answer: unknown) => (req: unknown) => {
    (calls[name] ??= []).push(req);
    return Promise.resolve(answer);
  };
  (client as unknown as { client: unknown }).client = {
    spendCharges: record('spend', { item: { id: 'e1', chargesLeft: 3 } }),
    rollLastCharge: record('roll', { d20: 14, regained: 0 }),
    setCharges: record('set', { item: { id: 'e1', chargesLeft: 2 } }),
    setAmmunition: record('ammo', { ammunition: { key: 'equipment:arrow', quantity: 20 } }),
    recoverAmmunition: record('recover', {
      ammunition: [{ key: 'equipment:arrow', quantity: 18 }],
    }),
    dawn: record('dawn', { restored: [] }),
    useConsumable: record('use', { entryId: 'e1', outcome: 1, healed: 7 }),
    listConsumableTargets: record('targets', {
      targets: [{ characterId: 'c2', name: 'Brisa', distanceFt: 5 }],
    }),
  };
  (client as unknown as { characters: unknown }).characters = {
    getCharacter: record('character', {
      character: {
        sheet: {
          content: {
            case: 'full',
            value: {
              magicItems: [{ id: 'e1', namePt: 'Varinha', chargesMax: 7 }],
              ammunition: [{ key: 'equipment:arrow', quantity: 15 }],
            },
          },
        },
      },
    }),
  };
  (client as unknown as { campaigns: unknown }).campaigns = {
    getCampaign: record('campaign', {
      campaign: { diceMode: DiceMode.PHYSICAL, myDicePreference: DicePreference.PHYSICAL },
    }),
  };
  return client;
}

describe('ChargesClient', () => {
  let calls: Record<string, unknown[]>;
  beforeEach(() => (calls = {}));

  it('spends charges of one entry under the given key', async () => {
    const res = await clientWith(calls).spendCharges('camp', 'c1', 'e1', 2, 'k1');
    expect(calls['spend']).toEqual([
      { campaignId: 'camp', characterId: 'c1', entryId: 'e1', amount: 2, idempotencyKey: 'k1' },
    ]);
    expect(res.item?.chargesLeft).toBe(3);
  });

  it('rolls the last charge in the app or with the face of a real die (RN-18)', async () => {
    const client = clientWith(calls);
    await client.rollLastCharge('camp', 'c1', 'e1', { inApp: true }, 'k1');
    await client.rollLastCharge('camp', 'c1', 'e1', { face: 20, regainFace: 3 }, 'k2');
    await client.rollLastCharge('camp', 'c1', 'e1', { face: 7 }, 'k3');
    expect(calls['roll']).toEqual([
      {
        campaignId: 'camp',
        characterId: 'c1',
        entryId: 'e1',
        idempotencyKey: 'k1',
        roll: { case: 'rollInApp', value: true },
        typedRegainFace: 0,
      },
      {
        campaignId: 'camp',
        characterId: 'c1',
        entryId: 'e1',
        idempotencyKey: 'k2',
        roll: { case: 'd20Face', value: 20 },
        typedRegainFace: 3,
      },
      {
        campaignId: 'camp',
        characterId: 'c1',
        entryId: 'e1',
        idempotencyKey: 'k3',
        roll: { case: 'd20Face', value: 7 },
        typedRegainFace: 0,
      },
    ]);
  });

  it("sets the master's correction: charges and what the item lost", async () => {
    await clientWith(calls).setCharges('camp', 'c1', 'e1', 0, ItemLoss.GONE, 'k1');
    expect(calls['set']).toEqual([
      {
        campaignId: 'camp',
        characterId: 'c1',
        entryId: 'e1',
        chargesLeft: 0,
        lost: ItemLoss.GONE,
        idempotencyKey: 'k1',
      },
    ]);
  });

  it('sets the pieces of one kind of ammunition, with no key (the write is idempotent)', async () => {
    const res = await clientWith(calls).setAmmunition('camp', 'c1', 'equipment:arrow', 20);
    expect(calls['ammo']).toEqual([
      { campaignId: 'camp', characterId: 'c1', ammoKey: 'equipment:arrow', quantity: 20 },
    ]);
    expect(res?.quantity).toBe(20);
  });

  it('takes the ammunition back once, under the given key', async () => {
    const list = await clientWith(calls).recoverAmmunition('camp', 'c1', 'k1');
    expect(calls['recover']).toEqual([
      { campaignId: 'camp', characterId: 'c1', idempotencyKey: 'k1' },
    ]);
    expect(list.map((a) => a.quantity)).toEqual([18]);
  });

  it("gives the table's dawn to the items of the campaign", async () => {
    await clientWith(calls).dawn('camp', 'k1');
    expect(calls['dawn']).toEqual([{ campaignId: 'camp', idempotencyKey: 'k1' }]);
  });

  it("reads a character's magic items and counted ammunition, whoever owns the sheet", async () => {
    const sheet = await clientWith(calls).sheetOf('camp', 'c1');
    expect(calls['character']).toEqual([{ campaignId: 'camp', characterId: 'c1' }]);
    expect(sheet.magicItems.map((m) => m.id)).toEqual(['e1']);
    expect(sheet.ammunition.map((a) => a.quantity)).toEqual([15]);
  });

  it('reads no items and no ammunition from a character with no full sheet', async () => {
    const client = clientWith(calls);
    (client as unknown as { characters: unknown }).characters = {
      getCharacter: () => Promise.resolve({ character: { sheet: { content: { case: 'basic' } } } }),
    };
    expect(await client.sheetOf('camp', 'c1')).toEqual({ magicItems: [], ammunition: [] });
  });

  it("reads the table's dice setting and the player's choice", async () => {
    const dice = await clientWith(calls).diceOf('camp');
    expect(calls['campaign']).toEqual([{ campaignId: 'camp' }]);
    expect(dice).toEqual({ mode: DiceMode.PHYSICAL, preference: DicePreference.PHYSICAL });
  });

  it('uses one unit of a consumable on the user, the app rolling the dice, under the given key', async () => {
    const res = await clientWith(calls).useConsumable(
      'camp',
      'c1',
      'e1',
      '',
      { inApp: true },
      'k1',
    );
    expect(calls['use']).toEqual([
      {
        campaignId: 'camp',
        characterId: 'c1',
        entryId: 'e1',
        targetCharacterId: '',
        idempotencyKey: 'k1',
        rollInApp: true,
        typedFaces: [],
      },
    ]);
    expect(res.healed).toBe(7);
  });

  it('gives the consumable to another character with the faces of real dice (RN-18)', async () => {
    await clientWith(calls).useConsumable(
      'camp',
      'c1',
      'e1',
      'c2',
      { faces: [3, 2] },
      'k2',
    );
    expect(calls['use']).toEqual([
      {
        campaignId: 'camp',
        characterId: 'c1',
        entryId: 'e1',
        targetCharacterId: 'c2',
        idempotencyKey: 'k2',
        rollInApp: false,
        typedFaces: [3, 2],
      },
    ]);
  });

  it('lists who a consumable may be given to', async () => {
    const targets = await clientWith(calls).listConsumableTargets('camp', 'c1', 'e1');
    expect(calls['targets']).toEqual([{ campaignId: 'camp', characterId: 'c1', entryId: 'e1' }]);
    expect(targets.map((t) => t.name)).toEqual(['Brisa']);
  });
});
