import { Code, ConnectError } from '@connectrpc/connect';

import { FakeFreeDiceClient, freeRoll } from './free-dice-testing';
import { FreeDiceState } from './free-dice-state';
import type { FreeRollVm } from './free-roll';

describe('FreeDiceState', () => {
  let api: FakeFreeDiceClient;
  let shown: FreeRollVm[];
  let master: boolean;
  let keys: number;
  let state: FreeDiceState;

  beforeEach(() => {
    api = new FakeFreeDiceClient();
    shown = [];
    master = false;
    keys = 0;
    state = new FreeDiceState(api as never, {
      campaignId: () => 'c1',
      isMaster: () => master,
      show: (r) => shown.push(r),
      now: () => new Date(2026, 9, 18, 20, 30),
      makeKey: () => `k${++keys}`,
    });
  });

  describe('the lists', () => {
    it('starts loading and reads the table and the caller\'s own rolls', async () => {
      expect(state.listState()).toBe('loading');
      api.lists = { table: [freeRoll('a', {}, 5)], mine: [freeRoll('b', {}, 4)] };
      await state.refresh();
      expect(state.listState()).toBe('ready');
      expect(state.table().map((r) => r.id)).toEqual(['a']);
      expect(state.mine().map((r) => r.id)).toEqual(['b']);
    });

    it('says the list failed and keeps what it had, and the form still rolls', async () => {
      api.lists = { table: [freeRoll('a')], mine: [] };
      await state.refresh();
      api.listError = new ConnectError('down', Code.Unavailable);
      await state.refresh();
      expect(state.listState()).toBe('failed');
      expect(state.table().map((r) => r.id)).toEqual(['a']);
      await state.roll();
      expect(api.rolls).toHaveLength(1);
    });

    it('does not lose a roll made while the read was on its way', async () => {
      let release!: () => void;
      const slow = new Promise<void>((r) => (release = r));
      const list = api.list;
      api.list = async () => {
        await slow;
        return list();
      };
      const reading = state.refresh();
      await state.roll();
      release();
      await reading;
      expect(state.mine().map((r) => r.id)).toEqual(['answer']);
      expect(state.table().map((r) => r.id)).toEqual(['answer']);
    });

    it('keeps the table to 50 and the own list to 10, newest first', async () => {
      api.lists = {
        table: Array.from({ length: 60 }, (_, i) => freeRoll(`t${i}`, {}, 59 - i)),
        mine: Array.from({ length: 12 }, (_, i) => freeRoll(`m${i}`, {}, 11 - i)),
      };
      await state.refresh();
      expect(state.table()).toHaveLength(50);
      expect(state.mine()).toHaveLength(10);
      expect(state.table()[0].id).toBe('t0');
    });

    it('starts again empty for another session', async () => {
      api.lists = { table: [freeRoll('a')], mine: [freeRoll('a')] };
      await state.refresh();
      state.clear();
      expect(state.table()).toEqual([]);
      expect(state.mine()).toEqual([]);
      expect(state.listState()).toBe('loading');
      expect(state.result()).toBeNull();
    });
  });

  describe('rolling', () => {
    it('sends the form with a key and shows the answer', async () => {
      state.change({ sides: 6, count: 2, modifier: 3 });
      await state.roll();
      expect(api.rolls).toEqual([
        {
          campaignId: 'c1',
          sides: 6,
          count: 2,
          modifier: 3,
          mode: 'normal',
          secret: false,
          idempotencyKey: 'k1',
        },
      ]);
      expect(state.result()?.id).toBe('answer');
      expect(state.rolling()).toBe(false);
      expect(state.error()).toBe('');
    });

    it('rolls once for a double tap', async () => {
      let release!: () => void;
      api.hold = new Promise<void>((r) => (release = r));
      const first = state.roll();
      const second = state.roll();
      expect(state.rolling()).toBe(true);
      release();
      await Promise.all([first, second]);
      expect(api.rolls).toHaveLength(1);
    });

    it('renews the key after a roll that worked, so the next tap is a new roll', async () => {
      await state.roll();
      api.answer = freeRoll('second');
      await state.roll();
      expect(api.rolls.map((r) => r.idempotencyKey)).toEqual(['k1', 'k2']);
    });

    it('keeps the key when the answer was lost, so the retry cannot roll twice', async () => {
      api.rollError = new ConnectError('down', Code.Unavailable);
      await state.roll();
      expect(state.error()).not.toBe('');
      api.rollError = null;
      await state.roll();
      expect(api.rolls.map((r) => r.idempotencyKey)).toEqual(['k1', 'k1']);
      expect(state.error()).toBe('');
    });

    it('uses a new key when the dice changed between the tries', async () => {
      api.rollError = new ConnectError('down', Code.Unavailable);
      await state.roll();
      api.rollError = null;
      state.change({ count: 3 });
      await state.roll();
      expect(api.rolls.map((r) => r.idempotencyKey)).toEqual(['k1', 'k2']);
    });

    it('says the wait when the server refuses for asking too often, and the form stays', async () => {
      api.rollError = new ConnectError(
        'slow',
        Code.ResourceExhausted,
        new Headers({ 'Retry-After': '3' }),
      );
      await state.roll();
      expect(state.error()).toBe('Rolagens demais em pouco tempo. Tente de novo em 3 segundos.');
      expect(state.result()).toBeNull();
      expect(state.rolling()).toBe(false);
    });

    it('puts the roll first in the own list and the table, once', async () => {
      api.lists = { table: [freeRoll('old', {}, 1)], mine: [freeRoll('old', {}, 1)] };
      await state.refresh();
      await state.roll();
      expect(state.mine().map((r) => r.id)).toEqual(['answer', 'old']);
      expect(state.table().map((r) => r.id)).toEqual(['answer', 'old']);
    });

    it('never asks for a secret roll for a player, whatever the form says', async () => {
      state.change({ secret: true });
      await state.roll();
      expect(api.rolls[0].secret).toBe(false);
    });

    it('lets the master ask for a secret roll, and keeps it in his own lists', async () => {
      master = true;
      state.change({ secret: true });
      api.answer = freeRoll('s1', { secret: true, byMaster: true, rollerName: 'Mestre' });
      await state.roll();
      expect(api.rolls[0].secret).toBe(true);
      expect(state.mine()[0].secret).toBe(true);
      expect(state.table()[0].secret).toBe(true);
    });

    it('does not show the roller\'s own roll on the table\'s dice', async () => {
      await state.roll();
      expect(shown).toEqual([]);
    });
  });

  describe('the form', () => {
    it('keeps every value in range and advantage only for one d20', () => {
      state.change({ count: 50, modifier: -99, sides: 6, mode: 'advantage' });
      expect(state.form()).toEqual({ sides: 6, count: 20, modifier: -20, mode: 'normal', secret: false });
      state.change({ sides: 20, count: 1, mode: 'advantage' });
      expect(state.form().mode).toBe('advantage');
      state.change({ count: 2 });
      expect(state.form().mode).toBe('normal');
    });

    it('fills the form from a shortcut and keeps the modifier', () => {
      state.change({ modifier: 4 });
      state.shortcut({ label: '2d6', sides: 6, count: 2 });
      expect(state.form()).toMatchObject({ sides: 6, count: 2, modifier: 4, mode: 'normal' });
    });

    it('goes back to the form with "Mudar dados", keeping the dice', async () => {
      state.change({ sides: 8 });
      await state.roll();
      state.backToForm();
      expect(state.result()).toBeNull();
      expect(state.form().sides).toBe(8);
    });

    it('"de novo" rolls the same dice with a new key', async () => {
      await state.roll();
      await state.again();
      expect(api.rolls).toHaveLength(2);
      expect(api.rolls[1].idempotencyKey).toBe('k2');
    });

    it('"Repetir" fills the form with that roll\'s dice and rolls at once', async () => {
      master = true;
      await state.repeat(freeRoll('x', { sides: 20, count: 1, modifier: 5, mode: 'advantage', secret: true }));
      expect(api.rolls[0]).toMatchObject({ sides: 20, count: 1, modifier: 5, mode: 'advantage', secret: true });
      expect(state.form()).toMatchObject({ sides: 20, modifier: 5, mode: 'advantage' });
    });
  });

  describe('what the stream says', () => {
    it('shows another person\'s table roll on the dice and adds it to the table', () => {
      state.heard(freeRoll('o1', { rollerName: 'Tor' }, 3));
      expect(shown.map((r) => r.id)).toEqual(['o1']);
      expect(state.table().map((r) => r.id)).toEqual(['o1']);
      expect(state.mine()).toEqual([]);
    });

    it('ignores a roll it already has (the answer came first)', async () => {
      await state.roll();
      state.heard(freeRoll('answer'));
      expect(shown).toEqual([]);
      expect(state.table()).toHaveLength(1);
    });

    it('holds a roll heard while its own is on the way, and does not show the roller\'s own twice', async () => {
      let release!: () => void;
      api.hold = new Promise<void>((r) => (release = r));
      const rolling = state.roll();
      state.heard(freeRoll('answer'));
      state.heard(freeRoll('other', { rollerName: 'Tor' }, 2));
      expect(shown).toEqual([]);
      release();
      await rolling;
      expect(shown.map((r) => r.id)).toEqual(['other']);
      expect(state.table().map((r) => r.id).sort()).toEqual(['answer', 'other']);
    });

    it('still reads the table when the answer failed, after the roll that was held', async () => {
      let release!: () => void;
      api.hold = new Promise<void>((r) => (release = r));
      api.rollError = new ConnectError('down', Code.Unavailable);
      const rolling = state.roll();
      state.heard(freeRoll('other', { rollerName: 'Tor' }));
      release();
      await rolling;
      expect(shown.map((r) => r.id)).toEqual(['other']);
    });

    it('keeps the table to 50', () => {
      for (let i = 0; i < 55; i++) {
        state.heard(freeRoll(`o${i}`, {}, i % 59));
      }
      expect(state.table()).toHaveLength(50);
    });
  });
});
