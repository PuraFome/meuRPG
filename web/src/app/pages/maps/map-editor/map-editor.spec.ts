import { create } from '@bufbuild/protobuf';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';

import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { LightLevel, MapLayer, MapPointKind, MapPointSchema, TrapState } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { TrapTrigger } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { LightPresets } from '../../../core/maps/light-presets';
import { MapState } from '../../../core/maps/map-state';
import { MapsClient } from '../../../core/maps/maps-client';
import { FakeMapsClient, mapMessage, mapPoint, mapResponse, mapToken } from '../../../core/maps/maps-testing';
import { RosterClient } from '../../../core/maps/roster-client';
import { visionResponse } from '../../../core/maps/vision-testing';
import { TrapPresets } from '../../../core/traps/trap-presets';
import { MapView } from '../../../shared/map-view/map-view';
import { PaintSurface } from '../paint-surface/paint-surface';
import { MapEditor } from './map-editor';

const roster = [
  { id: 'c-pensantus', name: 'Pensantus', kind: CharacterKind.PLAYER, playerUserId: 'u1', classSummary: 'Mago 3', raceName: 'Gnomo', playerName: 'Vinicius' },
  { id: 'c-toren', name: 'Toren', kind: CharacterKind.PLAYER, playerUserId: 'u2', classSummary: 'Guerreiro 5', raceName: 'Humano', playerName: 'Caio' },
];

const pit = create(MapPointSchema, { id: 'pit', mapId: 'map-1', kind: MapPointKind.TRAP, name: 'Fosso escondido', xBp: 4800, yBp: 5000, trap: { presetKey: '', noticeDc: 15, findDc: 15, areaSize: 2, trigger: TrapTrigger.ENTER, state: TrapState.ARMED } });
const chest = create(MapPointSchema, { id: 'chest', mapId: 'map-1', kind: MapPointKind.TREASURE, name: 'Baú de moedas', xBp: 7000, yBp: 8000, treasureValuePo: 250 });
const torch = create(MapPointSchema, { id: 'torch', mapId: 'map-1', kind: MapPointKind.LIGHT, name: 'Tocha da guarita', xBp: 8000, yBp: 3000, light: { presetKey: 'light:torch', brightFt: 20, dimFt: 20 } });
const tavern = mapPoint('tavern', 'Taverna', { kind: MapPointKind.SCENE });

describe('MapEditor', () => {
  let fixture: ComponentFixture<MapEditor>;
  let el: HTMLElement;
  let api: FakeMapsClient;
  let state: MapState;

  async function setup(mapPartial: Parameters<typeof mapMessage>[2] = { gridColumns: 24, gridRows: 16, fogEnabled: false }, inputs: { combatRunning?: boolean; sessionNumber?: number | null } = {}) {
    api = new FakeMapsClient();
    const map = mapMessage('map-1', 'A caverna do Vale Seco', mapPartial);
    api.layersResponse = { $typeName: 'meurpg.maps.v1.GetMapLayersResponse', gridColumns: mapPartial.gridColumns ?? 0, gridRows: mapPartial.gridRows ?? 0, layersRevision: 1, difficultTerrain: new Uint8Array(), wall: new Uint8Array(), cover: new Uint8Array(), light: new Uint8Array(), fogWithheld: false };
    state = new MapState(async () => mapResponse(map, [tavern, pit, chest, torch], [mapToken('c-pensantus', 'Pensantus')]));
    await state.open('map-1');
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [
        { provide: MapsClient, useValue: api },
        { provide: RosterClient, useValue: { list: () => Promise.resolve(roster) } },
        { provide: LightPresets, useValue: { list: () => Promise.resolve([{ key: 'light:torch', name: 'Tocha', radii: '6 m claro + 6 m de penumbra', brightFt: 20, dimFt: 20 }]) } },
        { provide: TrapPresets, useValue: { list: () => Promise.resolve({ presets: [], severities: [] }) } },
      ],
    });
    fixture = TestBed.createComponent(MapEditor);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('combatRunning', inputs.combatRunning ?? false);
    fixture.componentRef.setInput('sessionNumber', inputs.sessionNumber ?? null);
    fixture.detectChanges();
    el = fixture.nativeElement;
    await settle();
  }
  const settle = async () => {
    fixture.detectChanges();
    await new Promise((r) => setTimeout(r));
    await fixture.whenStable();
    fixture.detectChanges();
  };
  const text = () => (el.textContent ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ');
  const button = (t: string) => Array.from(el.querySelectorAll<HTMLElement>('button')).find((b) => b.textContent?.trim().endsWith(t))!;
  const radio = (t: string) => Array.from(el.querySelectorAll<HTMLElement>('[role="radio"]')).find((b) => b.textContent?.trim().endsWith(t))!;
  const surface = () => fixture.debugElement.query(By.directive(PaintSurface))?.componentInstance as PaintSurface | undefined;
  const flush = async () => {
    await new Promise((r) => setTimeout(r, 400));
    await settle();
  };

  describe('modes', () => {
    it('opens on "Pontos": the list of points, with the three new kinds on the bar', async () => {
      await setup();
      expect(radio('Pontos').getAttribute('aria-checked')).toBe('true');
      expect(text()).toContain('Pontos do mapa');
      expect(text()).toContain('Fosso escondido');
      expect(text()).toContain('Baú de moedas');
      expect(text()).toContain('Tocha da guarita');
      expect(surface()).toBeUndefined();
    });

    it('"Pintar" shows the tools, "Camadas", "Grade" and "Névoa de guerra", and the surface that catches the brush', async () => {
      await setup();
      radio('Pintar').click();
      await settle();
      expect(button('Terreno difícil')).toBeTruthy();
      expect(text()).toContain('Camadas');
      expect(text()).toContain('Tudo salvo');
      expect(text()).toContain('Grade');
      expect(text()).toContain('Névoa de guerra');
      expect(surface()).toBeDefined();
      expect(text()).not.toContain('Pontos do mapa');
    });
  });

  describe('unsaved changes', () => {
    async function dirtyTrap(): Promise<void> {
      await setup();
      Array.from(el.querySelectorAll<HTMLElement>('button.pl__row')).find((r) => r.textContent?.includes('Fosso escondido'))!.click();
      await settle();
      const field = Array.from(el.querySelectorAll('mat-form-field')).find((f) => f.querySelector('mat-label')?.textContent?.trim() === 'CD para achar (Investigação)')!.querySelector('input')!;
      field.value = '12';
      field.dispatchEvent(new Event('input'));
      await settle();
    }

    it('asks in place before going to "Pintar", and "Continuar editando" stays', async () => {
      await dirtyTrap();
      radio('Pintar').click();
      await settle();
      expect(text()).toContain('Salvar as mudanças em Fosso escondido?');
      expect(radio('Pontos').getAttribute('aria-checked')).toBe('true');
      button('Continuar editando').click();
      await settle();
      expect(text()).not.toContain('Salvar as mudanças em');
      expect(radio('Pontos').getAttribute('aria-checked')).toBe('true');
    });

    it('"Descartar mudanças" goes on to "Pintar"; "Salvar e continuar" saves first', async () => {
      await dirtyTrap();
      radio('Pintar').click();
      await settle();
      button('Descartar mudanças').click();
      await settle();
      expect(radio('Pintar').getAttribute('aria-checked')).toBe('true');
      expect(api.calls.some((c) => c.startsWith('updatePoint'))).toBe(false);
      radio('Pontos').click();
      await settle();
      Array.from(el.querySelectorAll<HTMLElement>('button.pl__row')).find((r) => r.textContent?.includes('Fosso escondido'))!.click();
      await settle();
      const field = Array.from(el.querySelectorAll('mat-form-field')).find((f) => f.querySelector('mat-label')?.textContent?.trim() === 'CD para achar (Investigação)')!.querySelector('input')!;
      field.value = '12';
      field.dispatchEvent(new Event('input'));
      await settle();
      radio('Pintar').click();
      await settle();
      button('Salvar e continuar').click();
      await settle();
      await settle();
      expect(api.calls.some((c) => c.startsWith('updatePoint'))).toBe(true);
      expect(radio('Pintar').getAttribute('aria-checked')).toBe('true');
    });
  });

  describe('painting', () => {
    it('reads the painted layers once, and paints a stroke at once with "Salvando" until it reaches the server', async () => {
      await setup();
      expect(api.calls.filter((c) => c.startsWith('layers'))).toHaveLength(1);
      radio('Pintar').click();
      await settle();
      button('Parede').click();
      await settle();
      surface()!.stroke.emit({ centers: [{ col: 3, row: 4 }], erase: false });
      await settle();
      // On the screen before the server answers.
      expect(text()).toContain('1 quadrado · bloqueia movimento, visão e luz');
      expect(text()).toContain('Salvando');
      await flush();
      expect(api.paints).toEqual([{ layer: MapLayer.WALL, value: 1, squares: [{ col: 3, row: 4 }] }]);
      expect(text()).toContain('Tudo salvo');
    });

    it('a drag is one call with every square; the 3 × 3 brush paints nine', async () => {
      await setup();
      radio('Pintar').click();
      await settle();
      button('Terreno difícil').click();
      radio('3×3').click();
      await settle();
      surface()!.stroke.emit({ centers: [{ col: 5, row: 5 }], erase: false });
      surface()!.stroke.emit({ centers: [{ col: 6, row: 5 }], erase: false });
      surface()!.strokeEnd.emit();
      await flush();
      expect(api.paints).toHaveLength(1);
      expect(api.paints[0].layer).toBe(MapLayer.DIFFICULT_TERRAIN);
      expect(api.paints[0].squares).toHaveLength(12);
    });

    it('"Apagar" and Shift erase the chosen tool\'s layer with value 0', async () => {
      await setup();
      radio('Pintar').click();
      await settle();
      button('Parede').click();
      surface()!.stroke.emit({ centers: [{ col: 1, row: 1 }], erase: false });
      surface()!.stroke.emit({ centers: [{ col: 1, row: 1 }], erase: true });
      await flush();
      expect(api.paints.map((p) => `${p.layer}:${p.value}`)).toEqual([`${MapLayer.WALL}:1`, `${MapLayer.WALL}:0`]);
      expect(text()).toContain('nada pintado');
    });

    it('cover paints the chosen degree; the light, the chosen level', async () => {
      await setup();
      radio('Pintar').click();
      await settle();
      button('Cobertura').click();
      await settle();
      expect(text()).toContain('Graus de cobertura');
      expect(text()).toContain('+2 na CA e nas salvaguardas de Destreza. Dá para passar por cima.');
      radio('Três quartos').click();
      surface()!.stroke.emit({ centers: [{ col: 2, row: 2 }], erase: false });
      button('Luz').click();
      await settle();
      radio('Claro').click();
      surface()!.stroke.emit({ centers: [{ col: 3, row: 2 }], erase: false });
      await flush();
      expect(api.paints.map((p) => `${p.layer}:${p.value}`)).toEqual([`${MapLayer.COVER}:2`, `${MapLayer.LIGHT}:3`]);
    });

    it('says "Não salvou" when the server refuses, keeps what was painted and tries again', async () => {
      await setup();
      radio('Pintar').click();
      await settle();
      button('Parede').click();
      api.failWith = new Error('offline');
      surface()!.stroke.emit({ centers: [{ col: 0, row: 0 }], erase: false });
      await flush();
      expect(text()).toContain('Não salvou');
      expect(text()).toContain('1 quadrado · bloqueia');
      api.failWith = null;
      button('Tentar de novo').click();
      await flush();
      expect(text()).toContain('Tudo salvo');
      expect(api.paints).toHaveLength(1);
    });

    it('hides a layer on his own map with its switch, and sends nothing', async () => {
      await setup();
      radio('Pintar').click();
      await settle();
      button('Parede').click();
      surface()!.stroke.emit({ centers: [{ col: 0, row: 0 }], erase: false });
      await settle();
      expect(el.querySelectorAll('app-editor-overlay .sq--wall')).toHaveLength(1);
      const wall = Array.from(el.querySelectorAll<HTMLElement>('app-layers-panel [role="switch"]'))[1];
      wall.click();
      await settle();
      expect(el.querySelectorAll('app-editor-overlay .sq--wall')).toHaveLength(0);
    });

    it('leaving "Pintar" sends what waits', async () => {
      await setup();
      radio('Pintar').click();
      await settle();
      button('Parede').click();
      surface()!.stroke.emit({ centers: [{ col: 0, row: 0 }], erase: false });
      radio('Pontos').click();
      await settle();
      await Promise.resolve();
      expect(api.paints).toHaveLength(1);
    });

    it('without a grid there is nothing to paint: no surface, the tools cannot act, and the reason is said', async () => {
      await setup({ gridColumns: 0, gridRows: 0 });
      radio('Pintar').click();
      await settle();
      expect(surface()).toBeUndefined();
      expect(text()).toContain('Defina a grade para pintar e ligar a névoa.');
      expect(el.querySelector('#bar-why')).not.toBeNull();
      expect(button('Parede').getAttribute('aria-disabled')).toBe('true');
      expect(text()).toContain('Este mapa ainda não tem grade.');
      expect(text()).toContain('Precisa da grade definida.');
    });

    it('while a combat runs says so, still paints, and turns the grid change off', async () => {
      await setup(undefined, { combatRunning: true });
      expect(text()).toContain('Um combate está em andamento neste mapa. Dá para pintar e apagar; a grade e a imagem só mudam depois dele.');
      radio('Pintar').click();
      await settle();
      expect(surface()).toBeDefined();
      expect(button('Mudar a grade').getAttribute('aria-disabled')).toBe('true');
      expect(text()).toContain('Desligado enquanto o combate dura.');
    });

    it('tells the page when a new grid or image would erase something', async () => {
      await setup({ gridColumns: 24, gridRows: 16, fogEnabled: true });
      const erases: boolean[] = [];
      fixture.componentInstance.erasesChange.subscribe((e) => erases.push(e));
      fixture.detectChanges();
      await settle();
      // The fog is on: the players may have seen something.
      expect(fixture.componentInstance['erases']()).toBe(true);
    });
  });

  describe('the new points', () => {
    function pick(name: string): void {
      const row = Array.from(el.querySelectorAll<HTMLElement>('button.pl__row')).find((r) => r.textContent?.includes(name))!;
      row.click();
    }

    it('opens the panel of each kind', async () => {
      await setup();
      pick('Fosso escondido');
      await settle();
      expect(el.querySelector('app-trap-point-panel')).not.toBeNull();
      expect(text()).toContain('Predefinições do SRD');
      fixture.componentInstance['requestSelect'](null);
      await settle();
      pick('Baú de moedas');
      await settle();
      expect(el.querySelector('app-treasure-point-panel')).not.toBeNull();
      fixture.componentInstance['requestSelect'](null);
      await settle();
      pick('Tocha da guarita');
      await settle();
      expect(el.querySelector('app-light-point-panel')).not.toBeNull();
      expect(text()).toContain('Tipo de luz');
      fixture.componentInstance['requestSelect'](null);
      await settle();
      pick('Taverna');
      await settle();
      expect(el.querySelector('app-point-panel')).not.toBeNull();
    });

    it('draws the radii of the selected light as two rings and names them in the legend', async () => {
      await setup();
      pick('Tocha da guarita');
      await settle();
      expect(el.querySelector('app-editor-overlay .reach__bright')).not.toBeNull();
      expect(el.querySelector('app-editor-overlay .reach__dim')).not.toBeNull();
      expect(text()).toContain('Alcance da luz clara');
      expect(text()).toContain('Alcance da penumbra');
    });

    it('creates a trap with a spec the server takes, a light from the torch preset and a treasure with no value', async () => {
      await setup();
      const view = fixture.debugElement.query(By.directive(MapView)).componentInstance as MapView;
      for (const [label, kind] of [['Armadilha', MapPointKind.TRAP], ['Luz', MapPointKind.LIGHT], ['Tesouro', MapPointKind.TREASURE]] as const) {
        button(label).click();
        view.emptyClick.emit({ xBp: 1000, yBp: 2000 });
        await settle();
        const call = api.calls.filter((c) => c.startsWith('createPoint')).at(-1)!;
        expect(call).toContain(`"kind":${kind}`);
        fixture.componentInstance['requestSelect'](null);
        fixture.componentInstance['dirty'].set(false);
        await settle();
      }
      expect(api.calls.filter((c) => c.startsWith('createPoint'))).toHaveLength(3);
    });

    it('a trap and a chest on one square stay two rows of the list', async () => {
      await setup();
      const names = Array.from(el.querySelectorAll('.pl__name'), (n) => n.textContent);
      expect(names).toContain('Fosso escondido');
      expect(names).toContain('Baú de moedas');
    });
  });

  describe('"Ver como"', () => {
    it('is offered with the fog on, and shows that player\'s map with "Voltar à sua vista"', async () => {
      await setup({ gridColumns: 24, gridRows: 16, fogEnabled: true, baseLight: LightLevel.DARK });
      api.visions.set('c-toren', visionResponse(['B'.repeat(24), ...Array.from({ length: 15 }, () => '.'.repeat(24))]));
      api.visions.set('c-pensantus', visionResponse(['B'.repeat(24), ...Array.from({ length: 15 }, () => '.'.repeat(24))]));
      api.responses.set('map-1@c-toren', mapResponse(mapMessage('map-1', 'A caverna do Vale Seco', { gridColumns: 24, gridRows: 16, fogEnabled: true }), [], [mapToken('c-toren', 'Toren')]));
      await settle();
      expect(text()).toContain('Ver como');
      const row = Array.from(el.querySelectorAll<HTMLElement>('app-view-as-list [role="radio"]')).find((r) => r.textContent?.includes('Toren'))!;
      row.click();
      await settle();
      expect(text()).toContain('Você está vendo o mapa como Toren');
      expect(text()).toContain('Para voltar à sua vista, escolha “Todos”.');
      expect(button('Voltar à sua vista')).toBeTruthy();
      expect(el.querySelector('app-editor-bar')).toBeNull();
      button('Voltar à sua vista').click();
      await settle();
      expect(el.querySelector('app-editor-bar')).not.toBeNull();
      expect(text()).not.toContain('Você está vendo o mapa como');
    });

    it('is not offered without the fog', async () => {
      await setup();
      expect(text()).not.toContain('Ver como');
    });
  });
});
