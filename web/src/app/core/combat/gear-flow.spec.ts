import { TestBed } from '@angular/core/testing';
import { Code, ConnectError } from '@connectrpc/connect';

import type { GearVm } from '../characters/gear';
import { GearFlow, gearBlocked, gearCost } from './gear-flow';

const sword: GearVm = {
  key: 'equipment:longsword',
  namePt: 'Espada longa',
  kind: 'weapon',
  equipped: true,
  twoHanded: false,
  armorClassIfToggled: 0,
};

function flow(
  read: () => Promise<{ gear: GearVm[]; armorClass: number }>,
  set = () => Promise.resolve(),
) {
  const calls = { read: 0, set: [] as [string, boolean][] };
  const f = TestBed.runInInjectionContext(
    () =>
      new GearFlow({
        client: {
          read: () => {
            calls.read++;
            return read();
          },
          set: (_c, _ch, key, equipped) => {
            calls.set.push([key, equipped]);
            return set();
          },
        },
        campaignId: () => 'c1',
        characterId: () => 'ch1',
      }),
  );
  return { f, calls };
}

describe('GearFlow', () => {
  it('reads the gear once per character', async () => {
    const { f, calls } = flow(() => Promise.resolve({ gear: [sword], armorClass: 14 }));
    await f.load();
    await f.load();
    expect(calls.read).toBe(1);
    expect(f.gear()).toEqual([sword]);
    expect(f.armorClass()).toBe(14);
  });

  it('asks the server, reads again and says the change with the server armor class', async () => {
    let ac = 14;
    const { f, calls } = flow(() => Promise.resolve({ gear: [sword], armorClass: ac }));
    await f.load();
    ac = 12;
    expect(await f.toggle(sword.key, false)).toBe(true);
    expect(calls.set).toEqual([[sword.key, false]]);
    expect(f.notice()).toBe('Guardado: Espada longa. Classe de armadura 12.');
    expect(f.busyKey()).toBeNull();
  });

  it('says the refusal and keeps the list when the server refuses', async () => {
    const { f } = flow(
      () => Promise.resolve({ gear: [sword], armorClass: 14 }),
      () => Promise.reject(new ConnectError('nope', Code.FailedPrecondition)),
    );
    await f.load();
    expect(await f.toggle(sword.key, false)).toBe(false);
    expect(f.error()).not.toBe('');
    expect(f.notice()).toBe('');
    expect(f.gear()).toEqual([sword]);
    expect(f.busyKey()).toBeNull();
  });
});

describe('GearFlow, when a read fails', () => {
  it('tries the first read again on the next look', async () => {
    let fail = true;
    const { f, calls } = flow(() =>
      fail ? Promise.reject(new Error('x')) : Promise.resolve({ gear: [sword], armorClass: 14 }),
    );
    await f.load();
    expect(f.gear()).toEqual([]);
    fail = false;
    await f.load();
    expect(calls.read).toBe(2);
    expect(f.gear()).toEqual([sword]);
  });

  it('does not announce a change whose list could not be read again', async () => {
    let n = 0;
    const { f } = flow(() =>
      n++ === 0
        ? Promise.resolve({ gear: [sword], armorClass: 14 })
        : Promise.reject(new Error('x')),
    );
    await f.load();
    expect(await f.toggle(sword.key, false)).toBe(true);
    expect(f.notice()).toBe('');
    expect(f.error()).toContain('não deu para ler');
  });
});

describe('the cost line', () => {
  const turn = { objectInteractionUsed: false, master: false };
  it('is the free interaction for the first weapon and the action for the next, the shield and nothing for armor', () => {
    expect(gearCost(sword, turn)).toBe('Usa a interação gratuita do turno.');
    expect(gearCost(sword, { ...turn, objectInteractionUsed: true })).toBe('Usa a sua ação.');
    expect(gearCost({ ...sword, kind: 'shield' }, turn)).toBe('Usa a sua ação.');
    expect(gearCost({ ...sword, kind: 'armor' }, turn)).toBe('');
    expect(gearBlocked({ ...sword, kind: 'armor' }, turn)).not.toBe('');
    expect(gearBlocked({ ...sword, kind: 'armor' }, { ...turn, master: true })).toBe('');
  });
});
