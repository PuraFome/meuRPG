import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import { vi } from 'vitest';

import {
  CharacterInTheDarkSchema,
  type ListCharactersInTheDarkResponse,
  ListCharactersInTheDarkResponseSchema,
} from '../../../gen/meurpg/maps/v1/maps_pb';
import { DarkWatch } from './dark-watch';
import { mapToken } from './map-builders-testing';

const answer = (...names: string[]): ListCharactersInTheDarkResponse =>
  create(ListCharactersInTheDarkResponseSchema, {
    characters: names.map((n) =>
      create(CharacterInTheDarkSchema, { characterId: `c-${n}`, characterName: n, col: 1, row: 1 }),
    ),
  });

describe('DarkWatch', () => {
  function watch(load: (mapId: string) => Promise<ListCharactersInTheDarkResponse>, map = 'm1') {
    const tokens = { value: [mapToken('c-Brisa', 'Brisa')] };
    const target = { value: map as string | null };
    const w = new DarkWatch(
      load,
      () => target.value,
      () => tokens.value,
    );
    return { w, tokens, target };
  }

  it('reads the dark characters of the map', async () => {
    const load = vi.fn(async () => answer('Brisa'));
    const { w } = watch(load);
    await w.refresh();
    expect(load).toHaveBeenCalledWith('m1');
    expect(w.characters().map((c) => c.name)).toEqual(['Brisa']);
    expect([...w.ids()]).toEqual(['c-Brisa']);
  });

  it('never asks where it does not apply (a player, no map, fog off): the target is null', async () => {
    const load = vi.fn(async () => answer('Brisa'));
    const { w, target } = watch(load);
    await w.refresh();
    target.value = null;
    await w.refresh();
    expect(load).toHaveBeenCalledTimes(1);
    expect(w.characters()).toEqual([]);
  });

  it('shows nothing when the read fails, and again when it works', async () => {
    let fail = false;
    const { w } = watch(async () => {
      if (fail) {
        throw new ConnectError('down', Code.Unavailable);
      }
      return answer('Brisa');
    });
    await w.refresh();
    expect(w.characters()).toHaveLength(1);
    fail = true;
    await w.refresh();
    expect(w.characters()).toEqual([]);
    fail = false;
    await w.refresh();
    expect(w.characters()).toHaveLength(1);
  });

  it('shows nothing when the server says permission_denied', async () => {
    const { w } = watch(async () => {
      throw new ConnectError('no', Code.PermissionDenied);
    });
    await w.refresh();
    expect(w.characters()).toEqual([]);
  });

  it('reads once more, not once per call, for a burst of hints, and keeps the newest answer', async () => {
    const releases: (() => void)[] = [];
    const load = vi.fn(
      (_m: string) =>
        new Promise<ListCharactersInTheDarkResponse>((resolve) => {
          const n = releases.length;
          releases.push(() => resolve(n === 0 ? answer('Brisa') : answer('Jabuti')));
        }),
    );
    const { w } = watch(load);
    const first = w.refresh();
    const second = w.refresh();
    const third = w.refresh();
    releases[0]();
    await vi.waitFor(() => expect(releases).toHaveLength(2));
    releases[1]();
    await Promise.all([first, second, third]);
    expect(load).toHaveBeenCalledTimes(2);
    expect(w.characters().map((c) => c.name)).toEqual(['Jabuti']);
  });

  it('announces a torch that went out between two reads, and a light that took someone out of the dark', async () => {
    let dark = ['Brisa'];
    const { w, tokens } = watch(async () => answer(...dark));
    tokens.value = [
      mapToken('c-bilbo', 'bilbo', { carriedLight: 'light:torch' }),
      mapToken('c-Brisa', 'Brisa'),
    ];
    await w.refresh();
    expect(w.announcement()).toBe('');
    tokens.value = [mapToken('c-bilbo', 'bilbo'), mapToken('c-Brisa', 'Brisa')];
    await w.refresh();
    expect(w.announcement()).toBe('A tocha de bilbo apagou. A luz de bilbo ficou “Nenhuma”.');
    dark = [];
    tokens.value = [
      mapToken('c-bilbo', 'bilbo'),
      mapToken('c-Brisa', 'Brisa', { carriedLight: 'light:torch' }),
    ];
    await w.refresh();
    expect(w.announcement()).toBe('Brisa acendeu a tocha. Não está mais no escuro.');
  });

  it('forgets the announcement when the map changes', async () => {
    const { w, tokens, target } = watch(async () => answer());
    tokens.value = [mapToken('c-bilbo', 'bilbo', { carriedLight: 'light:torch' })];
    await w.refresh();
    tokens.value = [mapToken('c-bilbo', 'bilbo')];
    await w.refresh();
    expect(w.announcement()).not.toBe('');
    target.value = 'm2';
    await w.refresh();
    expect(w.announcement()).toBe('');
  });
});
