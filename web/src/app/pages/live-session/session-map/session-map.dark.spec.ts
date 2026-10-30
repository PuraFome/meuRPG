import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { vi } from 'vitest';

import {
  CharacterInTheDarkSchema,
  ListCharactersInTheDarkResponseSchema,
} from '../../../../gen/meurpg/maps/v1/maps_pb';
import { DarkWatch } from '../../../core/maps/dark-watch';
import { FogView } from '../../../core/maps/fog-view';
import { LightPresets } from '../../../core/maps/light-presets';
import { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import { mapMessage, mapResponse, mapToken } from '../../../core/maps/maps-testing';
import { visionResponse } from '../../../core/maps/vision-testing';
import { LiveSessionSource } from '../../../core/play/live-session.types';
import { SceneClient } from '../../../core/play/scene-client';
import { settle } from '../../../../test-settle';
import { SessionMap } from './session-map';

describe('SessionMap: the light warning (B3)', () => {
  const load = vi.fn(async () =>
    create(ListCharactersInTheDarkResponseSchema, {
      characters: [
        create(CharacterInTheDarkSchema, {
          characterId: 'c-brisa',
          characterName: 'Brisa',
          col: 1,
          row: 0,
        }),
      ],
    }),
  );

  async function mount(isMaster: boolean) {
    load.mockClear();
    const tokens = [
      mapToken('c-brisa', 'Brisa', { xBp: 3000, yBp: 2000 }),
      mapToken('c-bilbo', 'bilbo', { xBp: 7000, yBp: 2000 }),
    ];
    const state = new MapState(async () =>
      mapResponse(mapMessage('map-1', 'Caverna', { revealed: true, fogEnabled: true }), [], tokens),
    );
    await state.open('map-1');
    const fog = new FogView(
      async () => visionResponse(['BBBB', 'BBBB']),
      async () => ({
        gridColumns: 4,
        gridRows: 2,
        difficultTerrain: new Uint8Array(),
        wall: new Uint8Array(),
        cover: new Uint8Array(),
      }),
    );
    await fog.open('map-1');
    const dark = new DarkWatch(
      load,
      () => 'map-1',
      () => state.tokens(),
    );
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: MapsClient, useValue: {} },
        { provide: LiveSessionSource, useValue: {} },
        { provide: SceneClient, useValue: {} },
        { provide: LightPresets, useValue: { list: async () => [] } },
      ],
    });
    const fixture = TestBed.createComponent(SessionMap);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('mapId', 'map-1');
    fixture.componentRef.setInput('isMaster', isMaster);
    fixture.componentRef.setInput('fog', fog);
    fixture.componentRef.setInput('dark', dark);
    await settle(fixture, 3);
    return { fixture, el: fixture.nativeElement as HTMLElement, dark };
  }

  it('the master’s card carries the notice above the map', async () => {
    const { el } = await mount(true);
    expect(el.querySelector('app-session-dark .dark')?.textContent).toContain(
      'Brisa não enxerga onde está',
    );
    expect(el.querySelectorAll('.ring')).toHaveLength(1);
    expect(el.querySelector('.label')).toBeNull();
  });

  it('"Ver no mapa" marks only the chosen one with “<Nome> · no escuro”', async () => {
    const { fixture, el } = await mount(true);
    el.querySelector<HTMLButtonElement>('button[aria-label="Ver no mapa, Brisa"]')!.click();
    await settle(fixture, 2);
    expect(el.querySelector('.label')?.textContent?.trim()).toBe('Brisa · no escuro');
  });

  it('the player’s screen never changes: no notice, no marks, no read', async () => {
    const { el } = await mount(false);
    expect(el.querySelector('app-session-dark')).toBeNull();
    expect(el.querySelector('.dark')).toBeNull();
    expect(el.querySelector('.ring')).toBeNull();
    expect(load).not.toHaveBeenCalled();
  });
});
