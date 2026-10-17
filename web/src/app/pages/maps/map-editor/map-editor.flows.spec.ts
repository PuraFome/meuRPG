import { type ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { MapPointKind } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { DungeonsClient } from '../../../core/maps/dungeons-client';
import { FakeDungeonsClient } from '../../../core/maps/dungeons-testing';
import { LightPresets } from '../../../core/maps/light-presets';
import { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import {
  FakeMapsClient,
  mapMessage,
  mapPoint,
  mapResponse,
  mapToken,
} from '../../../core/maps/maps-testing';
import { RosterClient } from '../../../core/maps/roster-client';
import { TrapPresets } from '../../../core/traps/trap-presets';
import { MapEditor } from './map-editor';

const people = [
  {
    id: 'c1',
    name: 'Pensantus',
    kind: CharacterKind.PLAYER,
    playerUserId: 'u1',
    classSummary: 'Mago 3',
    raceName: 'Gnomo',
    playerName: 'Vinicius',
  },
  {
    id: 'c2',
    name: 'Toren',
    kind: CharacterKind.PLAYER,
    playerUserId: 'u2',
    classSummary: 'Guerreiro 5',
    raceName: 'Humano',
    playerName: 'Caio',
  },
];

/** What the editor reports and saves, call by call: the paths the DOM specs of `MapEditor` leave (refusals, empty answers, the other kinds). */
interface Editor {
  message(): string;
  panelError(): string | null;
  dirty: { set(v: boolean): void };
  placing: { set(k: MapPointKind | null): void };
  selection: {
    (): { kind: string; id: string } | null;
    set(v: { kind: string; id: string } | null): void;
  };
  pending(): { kind: string; id: string } | null;
  pendingMode(): string | null;
  viewGone(): string;
  viewAs(): string | null;
  roomOutline(): unknown;
  justCreated(): boolean;
  view(): { focusOn: (...a: unknown[]) => void; fit: () => void } | undefined;
  activePanel: () => unknown;
  items: {
    addToken(e: { id: string; name: string }): Promise<void>;
    move(m: { kind: string; id: string; xBp: number; yBp: number }): Promise<void>;
    toggleTokenHidden(): Promise<void>;
    removeToken(): Promise<void>;
  };
  sel: {
    saveAndContinue(): Promise<void>;
    discardAndContinue(): void;
    keepEditing(): void;
  };
  pointSaves: { save(): Promise<boolean>; remove(): Promise<void> };
  dungeonCtl: { outline(room: unknown): void };
  viewAsCtl: { forgotten(): void; gone(): void; set(id: string | null): void };
  onEmptyClick(at: { xBp: number; yBp: number }): void;
  requestSelect(next: { kind: string; id: string } | null): void;
  setSceneActions(p: unknown, a: unknown[]): void;
  setShowDc(p: unknown, v: boolean): void;
  setClues(p: unknown, c: unknown[]): void;
  setImages(p: unknown, i: unknown[]): void;
  setMode(mode: string): void;
}

describe('MapEditor, flows and refusals', () => {
  let fixture: ComponentFixture<MapEditor>;
  let api: FakeMapsClient;
  let state: MapState;
  const ed = () => fixture.componentInstance as unknown as Editor;
  const settle = async () => {
    fixture.detectChanges();
    await vi.advanceTimersByTimeAsync(0);
    await fixture.whenStable();
    fixture.detectChanges();
  };

  const scene = mapPoint('scene', 'Taverna', { kind: MapPointKind.SCENE, xBp: 3000, yBp: 3000 });
  const creature = mapToken('c1', 'Goblin', { creatureId: 'cr1', xBp: 2000, yBp: 2000 });
  const hero = mapToken('c1', 'Pensantus', { xBp: 4000, yBp: 4000 });

  async function setup(
    mapPartial: Parameters<typeof mapMessage>[2] = { gridColumns: 24, gridRows: 16 },
    tokens = [hero],
  ): Promise<void> {
    api = new FakeMapsClient();
    const map = mapMessage('map-1', 'Caverna', mapPartial);
    state = new MapState(async () => mapResponse(map, [scene], tokens));
    await state.open('map-1');
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: MapsClient, useValue: api },
        { provide: DungeonsClient, useValue: new FakeDungeonsClient() },
        { provide: RosterClient, useValue: { list: () => Promise.resolve(people) } },
        {
          provide: LightPresets,
          useValue: { list: () => Promise.resolve([{ key: 'light:torch' }]) },
        },
        {
          provide: TrapPresets,
          useValue: { list: () => Promise.resolve({ presets: [{ findDc: 13 }], severities: [] }) },
        },
      ],
    });
    fixture = TestBed.createComponent(MapEditor);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('state', state);
    fixture.detectChanges();
    await settle();
    api.calls.length = 0;
  }

  beforeEach(() => vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] }));
  afterEach(() => vi.useRealTimers());

  describe('tokens', () => {
    it('puts a token on the map and selects it, or says it could not', async () => {
      await setup(undefined, []);
      await ed().items.addToken({ id: 'c2', name: 'Toren' });
      expect(api.calls.some((c) => c.startsWith('placeToken map-1 c2'))).toBe(true);
      expect(ed().message()).toBe('Toren está no mapa.');
      expect(ed().selection()).toEqual({ kind: 'token', id: 'c2' });
      api.failWith = new ConnectError('x', Code.Internal);
      await ed().items.addToken({ id: 'c1', name: 'Pensantus' });
      expect(ed().message()).toContain('falar com o servidor');
    });

    it('hides and shows the selected token, and says when it could not', async () => {
      await setup();
      await ed().items.toggleTokenHidden();
      expect(api.calls).toEqual([]);
      ed().selection.set({ kind: 'token', id: 'c1' });
      await ed().items.toggleTokenHidden();
      expect(api.calls).toEqual(['setTokenHidden map-1 c1 true']);
      api.failWith = new ConnectError('x', Code.Internal);
      await ed().items.toggleTokenHidden();
      expect(ed().message()).toContain('falar com o servidor');
    });

    it('takes the selected token off the map, or says it could not', async () => {
      await setup();
      await ed().items.removeToken();
      ed().selection.set({ kind: 'token', id: 'c1' });
      api.failWith = new ConnectError('x', Code.Internal);
      await ed().items.removeToken();
      expect(ed().message()).toContain('falar com o servidor');
      expect(ed().selection()).not.toBeNull();
      api.failWith = null;
      await ed().items.removeToken();
      expect(ed().message()).toBe('Pensantus saiu do mapa.');
      expect(ed().selection()).toBeNull();
      expect(state.tokens()).toHaveLength(0);
    });
  });

  describe('moving', () => {
    it('saves the move of a point, and puts it back with a message when the server refuses', async () => {
      await setup();
      await ed().items.move({ kind: 'point', id: 'scene', xBp: 100, yBp: 200 });
      expect(api.calls).toEqual(['updatePoint map-1 scene {"xBp":100,"yBp":200}']);
      expect(state.points()[0]).toMatchObject({ xBp: 100, yBp: 200 });
      api.failWith = new ConnectError('x', Code.Internal);
      await ed().items.move({ kind: 'point', id: 'scene', xBp: 900, yBp: 900 });
      expect(state.points()[0]).toMatchObject({ xBp: 100, yBp: 200 });
      expect(ed().message()).toContain('falar com o servidor');
    });

    it('ignores the move of what is not on the map', async () => {
      await setup();
      await ed().items.move({ kind: 'point', id: 'nope', xBp: 1, yBp: 1 });
      await ed().items.move({ kind: 'token', id: 'nope', xBp: 1, yBp: 1 });
      expect(api.calls).toEqual([]);
    });

    it('saves the move of a token, a creature by its creature id, and puts it back when refused', async () => {
      await setup(undefined, [hero]);
      await ed().items.move({ kind: 'token', id: 'c1', xBp: 10, yBp: 20 });
      expect(api.calls).toEqual(['placeToken map-1 c1 10 20']);
      api.failWith = new ConnectError('x', Code.Internal);
      await ed().items.move({ kind: 'token', id: 'c1', xBp: 50, yBp: 60 });
      expect(state.tokens()[0]).toMatchObject({ xBp: 10, yBp: 20 });
      expect(ed().message()).toContain('falar com o servidor');
      await setup(undefined, [creature]);
      await ed().items.move({ kind: 'token', id: 'cr1', xBp: 30, yBp: 40 });
      expect(api.calls).toEqual(['placeToken map-1 creature:cr1 30 40']);
    });
  });

  describe('new points', () => {
    const at = { xBp: 1234, yBp: 4321 };

    async function place(kind: MapPointKind): Promise<string> {
      ed().selection.set(null);
      ed().dirty.set(false);
      ed().placing.set(kind);
      ed().onEmptyClick(at);
      await vi.advanceTimersByTimeAsync(0);
      await settle();
      return api.calls.find((c) => c.startsWith('createPoint'))!;
    }

    it('names each kind and gives a light, a trap and a treasure what the server takes', async () => {
      await setup();
      expect(await place(MapPointKind.BATTLE)).toContain('"name":"Nova batalha"');
      api.calls.length = 0;
      expect(await place(MapPointKind.SUBMAP)).toContain('"name":"Novo submapa"');
      api.calls.length = 0;
      expect(await place(MapPointKind.SCENE)).toContain('"name":"Nova cena"');
      api.calls.length = 0;
      expect(await place(MapPointKind.LIGHT)).toContain('"light":{"presetKey":"light:torch"}');
      api.calls.length = 0;
      const trap = await place(MapPointKind.TRAP);
      expect(trap).toContain('"name":"Nova armadilha"');
      expect(trap).toContain('"findDc":13');
      api.calls.length = 0;
      const chest = await place(MapPointKind.TREASURE);
      expect(chest).toContain('"treasureValuePo":0');
      expect(chest).toContain('"name":"Novo tesouro"');
      expect(ed().justCreated()).toBe(true);
      expect(ed().message()).toContain('Ponto criado');
    });

    it('does not put a point down while the chosen one has unsaved changes', async () => {
      await setup();
      ed().selection.set({ kind: 'point', id: 'scene' });
      ed().dirty.set(true);
      ed().placing.set(MapPointKind.SCENE);
      ed().onEmptyClick(at);
      await settle();
      expect(api.calls).toEqual([]);
      expect(ed().message()).toContain('Salve ou descarte');
    });

    it('says why a point could not be made', async () => {
      await setup();
      api.failWith = new ConnectError('x', Code.Internal);
      ed().placing.set(MapPointKind.SCENE);
      ed().onEmptyClick(at);
      await settle();
      expect(ed().message()).toContain('falar com o servidor');
    });

    it('clears the selection on a click in the empty when nothing is being placed', async () => {
      await setup();
      ed().selection.set({ kind: 'point', id: 'scene' });
      ed().onEmptyClick(at);
      expect(ed().selection()).toBeNull();
    });
  });

  describe('the selection and the question', () => {
    it('keeps the selection when the same one is asked for', async () => {
      await setup();
      ed().requestSelect({ kind: 'point', id: 'scene' });
      ed().dirty.set(true);
      ed().requestSelect({ kind: 'point', id: 'scene' });
      expect(ed().pending()).toBeNull();
    });

    it('asks before leaving a dirty point, and goes on after "Descartar" or stays on "Continuar editando"', async () => {
      await setup();
      const discard = vi.fn();
      ed().activePanel = () => ({ changes: () => undefined, discard });
      ed().requestSelect({ kind: 'point', id: 'scene' });
      ed().dirty.set(true);
      ed().requestSelect(null);
      expect(ed().pending()).toEqual({ kind: 'point', id: '' });
      ed().sel.keepEditing();
      expect(ed().pending()).toBeNull();
      expect(ed().selection()).toEqual({ kind: 'point', id: 'scene' });
      ed().requestSelect({ kind: 'token', id: 'c1' });
      ed().sel.discardAndContinue();
      expect(discard).toHaveBeenCalled();
      expect(ed().selection()).toEqual({ kind: 'token', id: 'c1' });
    });

    it('asks before "Pintar" with a dirty point and goes there after the save', async () => {
      await setup();
      ed().activePanel = () => ({ changes: () => ({ name: 'Novo' }), discard: vi.fn() });
      ed().requestSelect({ kind: 'point', id: 'scene' });
      ed().dirty.set(true);
      ed().setMode('paint');
      expect(ed().pendingMode()).toBe('paint');
      await ed().sel.saveAndContinue();
      expect(ed().pendingMode()).toBeNull();
      expect(ed().selection()).toBeNull();
    });

    it('stays on the question when the save did not go through', async () => {
      await setup();
      ed().activePanel = () => ({ changes: () => ({ name: 'Novo' }), discard: vi.fn() });
      ed().requestSelect({ kind: 'point', id: 'scene' });
      ed().dirty.set(true);
      ed().requestSelect(null);
      api.failWith = new ConnectError('x', Code.Internal);
      await ed().sel.saveAndContinue();
      expect(ed().pending()).not.toBeNull();
    });
  });

  describe('saving a point', () => {
    function panel(changes: unknown): void {
      ed().activePanel = () => ({ changes: () => changes, discard: vi.fn() });
    }

    it('saves nothing without a point or a panel that answers', async () => {
      await setup();
      expect(await ed().pointSaves.save()).toBe(false);
      ed().selection.set({ kind: 'point', id: 'scene' });
      panel(undefined);
      expect(await ed().pointSaves.save()).toBe(false);
      expect(api.calls).toEqual([]);
    });

    it('is saved when nothing changed, and not when the form breaks a rule', async () => {
      await setup();
      ed().selection.set({ kind: 'point', id: 'scene' });
      panel(null);
      expect(await ed().pointSaves.save()).toBe(true);
      ed().dirty.set(true);
      expect(await ed().pointSaves.save()).toBe(false);
      expect(api.calls).toEqual([]);
    });

    it('sends the changes and says the point was saved, or why it was not', async () => {
      await setup();
      ed().selection.set({ kind: 'point', id: 'scene' });
      panel({ name: 'Novo' });
      expect(await ed().pointSaves.save()).toBe(true);
      expect(api.calls).toEqual(['updatePoint map-1 scene {"name":"Novo"}']);
      expect(ed().message()).toBe('Novo salvo.');
      api.failWith = new ConnectError('x', Code.Internal);
      expect(await ed().pointSaves.save()).toBe(false);
      expect(ed().panelError()).toContain('falar com o servidor');
    });

    it('carries what the lists save on their own into the point of the map', async () => {
      await setup();
      const point = state.points()[0];
      ed().setSceneActions(point, [{ id: 'a' }]);
      ed().setShowDc(point, true);
      ed().setClues(point, [{ id: 'k' }]);
      ed().setImages(point, [{ id: 'i' }]);
      expect(state.points()[0]).toMatchObject({
        sceneActions: [{ id: 'a' }],
        showDc: true,
        clues: [{ id: 'k' }],
        images: [{ id: 'i' }],
      });
      const gone = mapPoint('gone', 'Fora');
      ed().setClues(gone, [{ id: 'z' }]);
      expect(state.points().find((p) => p.id === 'gone')?.clues).toEqual([{ id: 'z' }]);
    });

    it('deletes the selected point, or says why it could not', async () => {
      await setup();
      await ed().pointSaves.remove();
      expect(api.calls).toEqual([]);
      ed().selection.set({ kind: 'point', id: 'scene' });
      api.failWith = new ConnectError('x', Code.Internal);
      await ed().pointSaves.remove();
      expect(ed().panelError()).toContain('falar com o servidor');
      api.failWith = null;
      await ed().pointSaves.remove();
      expect(ed().message()).toBe('Taverna foi apagado.');
      expect(state.points()).toHaveLength(0);
      expect(ed().selection()).toBeNull();
    });
  });

  describe('the dungeon rooms and "Ver como"', () => {
    it('goes to the chosen room, and back to the whole map without a room or a grid', async () => {
      await setup();
      const view = ed().view()!;
      const focusOn = vi.spyOn(view, 'focusOn');
      const fit = vi.spyOn(view, 'fit');
      ed().dungeonCtl.outline({ x: 2, y: 4, width: 4, height: 2 });
      expect(focusOn).toHaveBeenCalledWith(
        { xBp: (2 * 10000 * (4 / 24)) / 2 + 0, yBp: expect.any(Number) },
        3,
      );
      ed().dungeonCtl.outline(null);
      expect(fit).toHaveBeenCalledTimes(1);
      expect(ed().roomOutline()).toBeNull();
    });

    it('zooms no further than 3 and no less than 1.2', async () => {
      await setup();
      const focusOn = vi.spyOn(ed().view()!, 'focusOn');
      ed().dungeonCtl.outline({ x: 0, y: 0, width: 1, height: 1 });
      ed().dungeonCtl.outline({ x: 0, y: 0, width: 100, height: 1 });
      expect(focusOn.mock.calls.map((c) => c[1])).toEqual([3, 1.2]);
    });

    it('does not go to a room on a map with no grid', async () => {
      await setup({ gridColumns: 0, gridRows: 0 });
      const view = ed().view()!;
      const fit = vi.spyOn(view, 'fit');
      const focusOn = vi.spyOn(view, 'focusOn');
      ed().dungeonCtl.outline({ x: 0, y: 0, width: 2, height: 2 });
      expect(fit).toHaveBeenCalled();
      expect(focusOn).not.toHaveBeenCalled();
    });

    it('reads the counts of "Ver como" when the fog is forgotten, only with the fog on', async () => {
      await setup({ gridColumns: 24, gridRows: 16, fogEnabled: false });
      ed().viewAsCtl.forgotten();
      await settle();
      expect(api.calls.filter((c) => c.startsWith('vision'))).toEqual([]);
      state.setMap(
        mapMessage('map-1', 'Caverna', { gridColumns: 24, gridRows: 16, fogEnabled: true }),
      );
      await vi.advanceTimersByTimeAsync(400);
      await settle();
      api.calls.length = 0;
      ed().viewAsCtl.forgotten();
      await settle();
      expect(api.calls.filter((c) => c.startsWith('vision')).length).toBeGreaterThan(0);
    });

    it('goes back to "Todos" with a word when the character it looked as is gone', async () => {
      await setup();
      ed().viewAsCtl.set('c1');
      expect(ed().viewAs()).toBe('c1');
      ed().viewAsCtl.gone();
      expect(ed().viewAs()).toBeNull();
      expect(ed().viewGone()).toContain('morreu ou saiu da campanha');
      ed().viewAsCtl.set('c2');
      expect(ed().viewGone()).toBe('');
    });

    it('stops looking as someone when the fog goes off', async () => {
      await setup({ gridColumns: 24, gridRows: 16, fogEnabled: true });
      ed().viewAsCtl.set('c1');
      state.setMap(
        mapMessage('map-1', 'Caverna', { gridColumns: 24, gridRows: 16, fogEnabled: false }),
      );
      await settle();
      expect(ed().viewAs()).toBeNull();
    });
  });
});
