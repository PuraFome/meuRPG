import { create } from '@bufbuild/protobuf';
import { type ComponentFixture, TestBed } from '@angular/core/testing';
import { Code, ConnectError } from '@connectrpc/connect';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  CharacterInTheDarkSchema,
  ListCharactersInTheDarkResponseSchema,
} from '../../../../gen/meurpg/maps/v1/maps_pb';
import { DungeonsClient } from '../../../core/maps/dungeons-client';
import { FakeDungeonsClient } from '../../../core/maps/dungeons-testing';
import { LightPresets } from '../../../core/maps/light-presets';
import { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import { FakeMapsClient, mapMessage, mapResponse, mapToken } from '../../../core/maps/maps-testing';
import { RosterClient } from '../../../core/maps/roster-client';
import { TrapPresets } from '../../../core/traps/trap-presets';
import { MapEditor } from './map-editor';

describe('MapEditor: the light warning (B3)', () => {
  let fixture: ComponentFixture<MapEditor>;
  let el: HTMLElement;
  let api: FakeMapsClient;

  const dark = (...names: string[]) =>
    create(ListCharactersInTheDarkResponseSchema, {
      characters: names.map((n) =>
        create(CharacterInTheDarkSchema, {
          characterId: `c-${n}`,
          characterName: n,
          col: 2,
          row: 2,
        }),
      ),
    });

  const settle = async () => {
    for (let i = 0; i < 3; i++) {
      fixture.detectChanges();
      await vi.advanceTimersByTimeAsync(0);
      await fixture.whenStable();
    }
    fixture.detectChanges();
  };

  async function setup(fogEnabled: boolean, answer: ReturnType<typeof dark> | Error) {
    api = new FakeMapsClient();
    const read = api.charactersInTheDark.bind(api);
    api.charactersInTheDark = async (c, m) => {
      await read(c, m);
      if (answer instanceof Error) {
        throw answer;
      }
      return answer;
    };
    api.layersResponse = {
      $typeName: 'meurpg.maps.v1.GetMapLayersResponse',
      gridColumns: 24,
      gridRows: 16,
      layersRevision: 1,
      difficultTerrain: new Uint8Array(),
      wall: new Uint8Array(),
      cover: new Uint8Array(),
      light: new Uint8Array(),
      doors: new Uint8Array(),
      fogWithheld: false,
      noPass: new Uint8Array(),
    };
    const state = new MapState(async () =>
      mapResponse(
        mapMessage('map-1', 'A caverna do Vale Seco', {
          gridColumns: 24,
          gridRows: 16,
          fogEnabled,
        }),
        [],
        [mapToken('c-Brisa', 'Brisa'), mapToken('c-Jabuti', 'Jabuti', { xBp: 7000 })],
      ),
    );
    await state.open('map-1');
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [
        { provide: MapsClient, useValue: api },
        { provide: DungeonsClient, useValue: new FakeDungeonsClient() },
        { provide: RosterClient, useValue: { list: () => Promise.resolve([]) } },
        { provide: LightPresets, useValue: { list: () => Promise.resolve([]) } },
        {
          provide: TrapPresets,
          useValue: { list: () => Promise.resolve({ presets: [], severities: [] }) },
        },
      ],
    });
    fixture = TestBed.createComponent(MapEditor);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('combatRunning', false);
    fixture.componentRef.setInput('sessionNumber', null);
    el = fixture.nativeElement;
    await settle();
  }

  beforeEach(() => vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] }));
  afterEach(() => vi.useRealTimers());

  const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim();
  const mode = (label: string) =>
    Array.from(el.querySelectorAll<HTMLElement>('[role="radio"]')).find((b) =>
      b.textContent?.trim().endsWith(label),
    )!;

  it('shows the full notice over the map in "Pontos", with "Ver no mapa" and no "Acender luz"', async () => {
    await setup(true, dark('Brisa', 'Jabuti'));
    expect(flat(el.querySelector('.dark__title'))).toBe(
      'No escuro: 2 personagens não enxergam onde estão.',
    );
    expect(el.querySelectorAll('.ring')).toHaveLength(2);
    expect(el.textContent).not.toContain('Acender luz');
  });

  it('shrinks to one line in "Pintar"', async () => {
    await setup(true, dark('Brisa', 'Jabuti'));
    mode('Pintar').click();
    await settle();
    expect(flat(el.querySelector('.dark--line'))).toContain('No escuro: Brisa, Jabuti.');
    expect(el.querySelector('.dark__title')).toBeNull();
  });

  it('"Ver no mapa" labels only the chosen one', async () => {
    await setup(true, dark('Brisa', 'Jabuti'));
    el.querySelector<HTMLButtonElement>('button[aria-label="Ver Jabuti no mapa"]')!.click();
    await settle();
    expect(Array.from(el.querySelectorAll('.label')).map((l) => flat(l))).toEqual([
      'Jabuti · no escuro',
    ]);
  });

  it('shows nothing, and asks nothing, with the fog off', async () => {
    await setup(false, dark('Brisa'));
    expect(el.querySelector('.dark')).toBeNull();
    expect(api.calls.some((c) => c.startsWith('inTheDark'))).toBe(false);
  });

  it('shows nothing when the read fails: not even an error in this card', async () => {
    await setup(true, new ConnectError('down', Code.Unavailable));
    expect(el.querySelector('.dark')).toBeNull();
    expect(el.querySelector('app-editor-dark [role="alert"]')).toBeNull();
  });
});
