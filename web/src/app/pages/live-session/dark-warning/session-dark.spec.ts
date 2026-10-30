import { create } from '@bufbuild/protobuf';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { TestBed } from '@angular/core/testing';
import { vi } from 'vitest';

import {
  CharacterInTheDarkSchema,
  ListCharactersInTheDarkResponseSchema,
} from '../../../../gen/meurpg/maps/v1/maps_pb';
import { DarkWatch } from '../../../core/maps/dark-watch';
import { lightRadii, LightPresets } from '../../../core/maps/light-presets';
import { MapState } from '../../../core/maps/map-state';
import { mapMessage, mapResponse, mapToken } from '../../../core/maps/maps-testing';
import { settle } from '../../../../test-settle';
import type { CarriedLightData } from '../carried-light/carried-light-sheet';
import { SessionDark } from './session-dark';

const options = [
  { key: 'light:torch', name: 'Tocha', radii: lightRadii(20, 20) },
  { key: 'light:hooded-lantern', name: 'Lanterna coberta', radii: lightRadii(30, 30) },
];

const answer = (...names: string[]) =>
  create(ListCharactersInTheDarkResponseSchema, {
    characters: names.map((n) =>
      create(CharacterInTheDarkSchema, { characterId: `c-${n}`, characterName: n, col: 2, row: 2 }),
    ),
  });

describe('SessionDark: the warning on the master’s session map card', () => {
  const open = vi.fn(() => ({ afterClosed: () => ({ subscribe: () => undefined }) }));

  async function mount(dark: string[] = ['Brisa']) {
    const brisa = mapToken('c-Brisa', 'Brisa');
    const state = new MapState(async () =>
      mapResponse(mapMessage('map-1', 'Caverna', { fogEnabled: true }), [], [brisa]),
    );
    await state.open('map-1');
    const load = vi.fn(async () => answer(...dark));
    const watch = new DarkWatch(
      load,
      () => 'map-1',
      () => state.tokens(),
    );
    TestBed.configureTestingModule({
      providers: [
        { provide: LightPresets, useValue: { list: async () => options } },
        { provide: MatDialog, useValue: { open } },
        { provide: MatBottomSheet, useValue: { open } },
      ],
    });
    const fixture = TestBed.createComponent(SessionDark);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('watch', watch);
    const shown = vi.fn();
    fixture.componentInstance.show.subscribe(shown);
    await settle(fixture, 3);
    return { fixture, state, watch, load, shown, el: fixture.nativeElement as HTMLElement };
  }

  const button = (el: HTMLElement, label: string) =>
    el.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`)!;

  beforeEach(() => open.mockClear());

  it('reads the dark characters when it appears and shows the notice', async () => {
    const { el, load } = await mount();
    expect(load).toHaveBeenCalledWith('map-1');
    expect(el.textContent).toContain('No escuro: Brisa não enxerga onde está');
  });

  it('shows nothing at all when nobody is in the dark', async () => {
    const { el } = await mount([]);
    expect(el.querySelector('.dark')).toBeNull();
  });

  it('reads again when the map’s tokens change (a carried light changed, a token moved)', async () => {
    const { fixture, state, load } = await mount();
    state.upsertToken(mapToken('c-Brisa', 'Brisa', { carriedLight: 'light:torch' }));
    await settle(fixture, 2);
    expect(load.mock.calls.length).toBeGreaterThan(1);
  });

  it('passes "Ver no mapa" on to the page', async () => {
    const { el, shown } = await mount();
    button(el, 'Ver no mapa, Brisa').click();
    expect(shown).toHaveBeenCalledWith('c-Brisa');
  });

  it('"Acender luz" opens the existing carried-light sheet for that character, with the lights it knows', async () => {
    const { fixture, el } = await mount();
    button(el, 'Acender luz de Brisa').click();
    await settle(fixture, 2);
    expect(open).toHaveBeenCalledTimes(1);
    const config = (open.mock.calls[0] as unknown as [unknown, { data: CarriedLightData }])[1];
    expect(config.data).toMatchObject({
      campaignId: 'camp-1',
      mapId: 'map-1',
      characterId: 'c-Brisa',
      characterName: 'Brisa',
      current: '',
    });
    expect(config.data.options.map((o) => o.key)).toEqual(['light:torch', 'light:hooded-lantern']);
  });

  it('after a light is chosen, the token is updated and the warning is read again', async () => {
    const { fixture, el, state, load } = await mount();
    button(el, 'Acender luz de Brisa').click();
    await settle(fixture, 2);
    const config = (open.mock.calls[0] as unknown as [unknown, { data: CarriedLightData }])[1];
    const before = load.mock.calls.length;
    config.data.changed(mapToken('c-Brisa', 'Brisa', { carriedLight: 'light:torch' }), options[0]);
    await settle(fixture, 2);
    expect(state.tokens()[0].carriedLight).toBe('light:torch');
    expect(load.mock.calls.length).toBeGreaterThan(before);
  });

  it('shows nothing when the read fails: no error in this card', async () => {
    const fixture = await mount();
    const failing = new DarkWatch(
      async () => {
        throw new Error('down');
      },
      () => 'map-1',
      () => [],
    );
    fixture.fixture.componentRef.setInput('watch', failing);
    await settle(fixture.fixture, 3);
    expect(fixture.el.querySelector('.dark')).toBeNull();
    expect(fixture.el.querySelector('[role="alert"]')).toBeNull();
    expect(fixture.el.textContent?.trim()).toBe('');
  });
});
