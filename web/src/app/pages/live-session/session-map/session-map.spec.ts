import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideRouter } from '@angular/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { MapState, tokenKey } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import { SceneClient } from '../../../core/play/scene-client';
import { mapMessage, mapPoint, mapResponse, mapToken } from '../../../core/maps/maps-testing';
import { create } from '@bufbuild/protobuf';

import { MapPointKind, MapRefSchema } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { MapView } from '../../../shared/map-view/map-view';
import { LiveSessionSource } from '../../../core/play/live-session.types';
import { SessionMap } from './session-map';

describe('SessionMap dragging a token', () => {
  const placeToken = vi.fn();
  const raven = mapToken('pens', 'Nanquim', { creatureId: 'raven', xBp: 2000, yBp: 2000 });
  const owner = mapToken('pens', 'Pensantus', { mine: true, xBp: 6000, yBp: 6000 });

  async function mount(tokens = [raven, owner]) {
    const state = new MapState(async () =>
      mapResponse(mapMessage('map-1', 'Torre', { revealed: true }), [], tokens),
    );
    await state.open('map-1');
    const fixture = TestBed.createComponent(SessionMap);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('mapId', 'map-1');
    fixture.componentRef.setInput('isMaster', true);
    fixture.detectChanges();
    const view = fixture.debugElement.query(By.directive(MapView)).componentInstance as MapView;
    return { state, view, fixture };
  }

  const at = (state: MapState, key: string) =>
    state
      .tokens()
      .filter((t) => tokenKey(t) === key)
      .map((t) => [t.xBp, t.yBp]);

  beforeEach(() => {
    placeToken.mockReset();
    placeToken.mockImplementation(async (_c, _m, id, xBp, yBp) => mapToken(id, 'x', { xBp, yBp }));
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: MapsClient, useValue: { placeToken } },
        { provide: LiveSessionSource, useValue: {} },
        { provide: SceneClient, useValue: {} },
      ],
    });
  });

  it('saves the move of a character token by its character id', async () => {
    const { state, view } = await mount([owner]);
    view.moved.emit({ kind: 'token', id: 'pens', xBp: 7000, yBp: 7000 });
    await Promise.resolve();
    expect(placeToken).toHaveBeenCalledWith('camp-1', 'map-1', 'pens', 7000, 7000);
    expect(at(state, 'pens')).toEqual([[7000, 7000]]);
  });

  it('saves the move of a creature token by its creature id', async () => {
    const { view } = await mount();
    view.moved.emit({ kind: 'token', id: 'raven', xBp: 3000, yBp: 3000 });
    await Promise.resolve();
    expect(placeToken).toHaveBeenCalledTimes(1);
    expect(placeToken).toHaveBeenCalledWith('camp-1', 'map-1', '', 3000, 3000, 'raven');
  });

  it("moves the owner's token, not its creature's, when the owner is dragged", async () => {
    const { state, view } = await mount();
    view.moved.emit({ kind: 'token', id: 'pens', xBp: 7000, yBp: 7000 });
    await Promise.resolve();
    expect(at(state, 'raven')).toEqual([[2000, 2000]]);
    expect(at(state, 'pens')).toEqual([[7000, 7000]]);
  });

  it('drops the banner of a refused move once a later move is saved', async () => {
    placeToken.mockRejectedValueOnce(new Error('refused'));
    const { view, fixture } = await mount([owner]);
    const banner = () => (fixture.nativeElement as HTMLElement).querySelector('[role="alert"]');
    view.moved.emit({ kind: 'token', id: 'pens', xBp: 7000, yBp: 7000 });
    await fixture.whenStable();
    fixture.detectChanges();
    expect(banner()).not.toBeNull();
    view.moved.emit({ kind: 'token', id: 'pens', xBp: 7100, yBp: 7100 });
    await fixture.whenStable();
    fixture.detectChanges();
    expect(banner()).toBeNull();
  });
});

describe('SessionMap: "Ir para <mapa>" on a Submapa point', () => {
  const setCurrentMap = vi.fn();

  async function mount() {
    const stairs = mapPoint('p-sub', 'Torre', {
      kind: MapPointKind.SUBMAP,
      revealed: true,
      targetMap: create(MapRefSchema, { id: 'map-2', name: 'Interior da torre' }),
    });
    const noTarget = mapPoint('p-none', 'Escada', { kind: MapPointKind.SUBMAP, revealed: true });
    const state = new MapState(async () =>
      mapResponse(mapMessage('map-1', 'Torre', { revealed: true }), [stairs, noTarget], []),
    );
    await state.open('map-1');
    const fixture = TestBed.createComponent(SessionMap);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('mapId', 'map-1');
    fixture.componentRef.setInput('isMaster', true);
    const changed: (string | null)[] = [];
    fixture.componentInstance.currentChanged.subscribe((m) => changed.push(m));
    fixture.detectChanges();
    return { fixture, changed, el: fixture.nativeElement as HTMLElement };
  }

  beforeEach(() => {
    setCurrentMap.mockReset();
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: MapsClient, useValue: {} },
        { provide: LiveSessionSource, useValue: { setCurrentMap } },
        { provide: SceneClient, useValue: {} },
      ],
    });
  });

  const goButtons = (el: HTMLElement) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('.row__go'));

  it('offers it only on a Submapa point that has a destination', async () => {
    const { el } = await mount();
    expect(goButtons(el)).toHaveLength(1);
    expect(goButtons(el)[0].textContent).toContain('Ir para Interior da torre');
    expect(goButtons(el)[0].getAttribute('aria-label')).toBe(
      'Levar a sessão para Interior da torre',
    );
  });

  it("changes the session's current map to the destination", async () => {
    setCurrentMap.mockResolvedValue('map-2');
    const { fixture, changed, el } = await mount();
    goButtons(el)[0].click();
    await fixture.whenStable();
    expect(setCurrentMap).toHaveBeenCalledWith('camp-1', 'map-2');
    expect(changed).toEqual(['map-2']);
  });

  it('says so when the session is over', async () => {
    const { ConnectError, Code } = await import('@connectrpc/connect');
    setCurrentMap.mockRejectedValue(new ConnectError('no', Code.FailedPrecondition));
    const { fixture, changed, el } = await mount();
    goButtons(el)[0].click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(changed).toEqual([]);
    expect(el.textContent).toContain('A sessão acabou: o mapa só muda durante a sessão.');
  });
});

describe("SessionMap: the master's tool tabs (C6-05)", () => {
  async function mount(tools: boolean) {
    const state = new MapState(async () =>
      mapResponse(mapMessage('map-1', 'Torre', { revealed: true }), [], []),
    );
    await state.open('map-1');
    const fixture = TestBed.createComponent(SessionMap);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('mapId', 'map-1');
    fixture.componentRef.setInput('isMaster', true);
    fixture.componentRef.setInput('tools', tools);
    fixture.componentRef.setInput('counts', { tokens: 3, traps: 0, treasure: 2, encounters: 1 });
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: MapsClient, useValue: {} },
        { provide: LiveSessionSource, useValue: {} },
        { provide: SceneClient, useValue: {} },
      ],
    });
  });

  it('draws no tabs unless asked', async () => {
    expect((await mount(false)).querySelector('[role="tablist"]')).toBeNull();
  });

  it('names the tools, with their counts, and keeps every panel mounted', async () => {
    const el = await mount(true);
    const list = el.querySelector('[role="tablist"]')!;
    expect(list.getAttribute('aria-label')).toBe('Ferramentas do mapa');
    const names = Array.from(list.querySelectorAll('[role="tab"]'), (t) =>
      (t.querySelector('span')?.textContent ?? '').trim(),
    );
    expect(names).toEqual(
      expect.arrayContaining(['Pontos', 'Tokens', 'Armadilhas', 'Tesouro', 'Encontros']),
    );
    expect(el.querySelector('#map-tool-panel-pontos')?.hasAttribute('hidden')).toBe(false);
    expect(el.querySelector('#map-tool-panel-tokens')?.hasAttribute('hidden')).toBe(true);
  });
});

describe('SessionMap, the player on a phone (C5-01)', () => {
  const asPhone = (matches: boolean) => {
    window.matchMedia = (() => ({
      matches,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    })) as unknown as typeof window.matchMedia;
  };

  async function mount(searchable: boolean) {
    const state = new MapState(async () =>
      mapResponse(mapMessage('map-1', 'Cripta do Corvo', { revealed: true }), [], []),
    );
    await state.open('map-1');
    const fixture = TestBed.createComponent(SessionMap);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('mapId', 'map-1');
    fixture.componentRef.setInput('searchable', searchable);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  beforeEach(() =>
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: MapsClient, useValue: {} },
        { provide: LiveSessionSource, useValue: {} },
        { provide: SceneClient, useValue: {} },
      ],
    }),
  );
  afterEach(() => {
    delete (window as unknown as Record<string, unknown>)['matchMedia'];
  });

  it('is one row, "Mapa · Cripta do Corvo" with "Ver mapa" to the whole map, and no preview', async () => {
    asPhone(true);
    const el = await mount(false);
    expect(el.querySelector('.map-row__title')?.textContent?.trim()).toBe('Mapa · Cripta do Corvo');
    const open = el.querySelector('a.map-row__open') as HTMLAnchorElement;
    expect(open.textContent).toContain('Ver mapa');
    expect(open.getAttribute('href')).toBe('/campaigns/camp-1/maps/map-1?from=session');
    expect(el.querySelector('app-map-view')).toBeNull();
    expect(el.querySelector('.map__search')).toBeNull();
  });

  it('keeps "Procurar armadilhas" under the row', async () => {
    asPhone(true);
    const el = await mount(true);
    expect(el.querySelector('.map-row .map__search')?.textContent).toContain('Procurar armadilhas');
  });

  it('is the preview with "Ver mapa" on a computer', async () => {
    asPhone(false);
    const el = await mount(false);
    expect(el.querySelector('.map-row')).toBeNull();
    // "Mapa" and, muted, the map's name and that it is the current one; "Ver mapa" is an icon button at the right (C6-08).
    expect(el.querySelector('#session-map-heading')?.textContent?.replace(/\s+/g, ' ').trim()).toBe(
      'mapMapa · Cripta do Corvo · mapa atual',
    );
    const full = el.querySelector('a.map__full') as HTMLAnchorElement;
    expect(full.getAttribute('aria-label')).toBe('Ver mapa');
    expect(full.getAttribute('href')).toBe('/campaigns/camp-1/maps/map-1?from=session');
    expect(el.querySelector('a.map__open')).toBeNull();
  });

  it('captions the map "Mapa · <nome>" under it in the scene\'s "Mapa" tab, as the board does', async () => {
    asPhone(false);
    const state = new MapState(async () =>
      mapResponse(mapMessage('map-1', 'Cripta do Corvo', { revealed: true }), [], []),
    );
    await state.open('map-1');
    const fixture = TestBed.createComponent(SessionMap);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('mapId', 'map-1');
    fixture.componentRef.setInput('inTab', true);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.map__caption')?.textContent?.trim()).toBe('Mapa · Cripta do Corvo');
  });
});
