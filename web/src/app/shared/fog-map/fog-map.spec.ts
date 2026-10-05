import { TestBed } from '@angular/core/testing';
import { textOf } from '../../core/format/text-testing';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { CharacterKind } from '../../../gen/meurpg/characters/v1/characters_pb';
import { decodeVision, type Vision } from '../../core/maps/vision';
import { visionResponse } from '../../core/maps/vision-testing';
import { mapToken } from '../../core/maps/maps-testing';
import type { MapLayers } from '../../core/maps/layers';
import { FogMap } from './fog-map';

const plain = textOf;

const pensantus = mapToken('p', 'Pensantus', { mine: true, xBp: 2000, yBp: 2000 });
const toren = mapToken('t', 'Toren', { xBp: 3000, yBp: 2000 });
const goblin = mapToken('g', 'Goblin 2', { kind: CharacterKind.MINION, xBp: 8000, yBp: 2000 });
const nanquim = mapToken('p', 'Nanquim', { creatureId: 'c1', kind: CharacterKind.UNSPECIFIED, xBp: 2500, yBp: 2500 });

const layers: MapLayers = { columns: 4, rows: 4, walls: [{ col: 0, row: 0 }], terrain: [], half: [{ col: 1, row: 1 }], threeQuarters: [] };

function tiled(partial: Parameters<typeof visionResponse>[1] = {}): Vision {
  return decodeVision(
    visionResponse(['....', '.gB.', '.dr.', '....'], {
      tilesPath: '/images/maps/m1/tiles/',
      tileSquares: 2,
      tiles: [
        { $typeName: 'meurpg.maps.v1.MapTile', tx: 0, ty: 0, revision: 3 },
        { $typeName: 'meurpg.maps.v1.MapTile', tx: 1, ty: 1, revision: 1 },
      ],
      ...partial,
    }),
  );
}

describe('FogMap', () => {
  beforeEach(() => TestBed.configureTestingModule({ imports: [FogMap] }));
  afterEach(() => vi.unstubAllGlobals());

  function create(inputs: Record<string, unknown> = {}) {
    const fixture = TestBed.createComponent(FogMap);
    fixture.componentRef.setInput('imageWidth', 960);
    fixture.componentRef.setInput('imageHeight', 640);
    fixture.componentRef.setInput('mapName', 'A caverna do Vale Seco');
    fixture.componentRef.setInput('vision', tiled());
    fixture.componentRef.setInput('layers', layers);
    fixture.componentRef.setInput('tokens', [pensantus, toren, goblin]);
    fixture.componentRef.setInput('viewer', { name: 'Pensantus', own: true });
    for (const [k, v] of Object.entries(inputs)) {
      fixture.componentRef.setInput(k, v);
    }
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  const settle = (fixture: ReturnType<typeof create>['fixture']) => {
    (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLImageElement>('.fb__tile').forEach((t) => t.dispatchEvent(new Event('load')));
    fixture.detectChanges();
  };

  it('says "Carregando o mapa" once, as a status, with "parte N de M", until the tiles are in; then the notice goes', () => {
    const { fixture, el } = create();
    const notice = el.querySelector('[data-testid="fog-loading"]')!;
    expect(notice.getAttribute('role')).toBe('status');
    expect(plain(notice)).toContain('Carregando o mapa');
    expect(plain(notice)).toContain('Chegando a parte 1 de 2');
    el.querySelector<HTMLImageElement>('.fb__tile')!.dispatchEvent(new Event('load'));
    fixture.detectChanges();
    expect(plain(el.querySelector('[data-testid="fog-loading"]'))).toContain('Chegando a parte 2 de 2');
    settle(fixture);
    expect(el.querySelector('[data-testid="fog-loading"]')).toBeNull();
  });

  it('draws a token only over a place whose tile has arrived, and a token on a square with no tile at once', () => {
    const { fixture, el } = create();
    const names = () => Array.from(el.querySelectorAll('ul[aria-label="No mapa"] li'), (li) => plain(li));
    // Nothing has arrived: Pensantus (tile 0:0) and Toren wait; the goblin is on a square whose tile does not exist (it is black).
    expect(names()).toEqual(['Goblin 2']);
    settle(fixture);
    expect(names()).toEqual(['Pensantus (você)', 'Toren', 'Goblin 2']);
  });

  it('while the vision is read, the place is a still striped frame, with no map and no token yet', () => {
    const { el } = create({ vision: null, status: 'loading' });
    expect(el.querySelector('.fm__wait')).not.toBeNull();
    expect(el.querySelector('app-map-view')).toBeNull();
    expect(plain(el.querySelector('[data-testid="fog-loading"]'))).toContain('Carregando o mapa');
  });

  it('keeps the loading notice off when the screen has its own banner', () => {
    const { el } = create({ notices: false });
    expect(el.querySelector('[data-testid="fog-loading"]')).toBeNull();
  });

  it('says in words that the character is not on the map, and what is still seen', () => {
    const { el } = create({ vision: tiled({ characterOnMap: false }), viewer: { name: 'Brisa', own: true } });
    const notice = el.querySelector('[data-testid="fog-off-map"]')!;
    expect(notice.getAttribute('role')).toBe('status');
    expect(plain(notice)).toBe('location_off Brisa fora do mapa Seu personagem não está neste mapa. Você vê só o que já tinha visto.');
  });

  it('says it of another character when the master reads as them', () => {
    const { el } = create({ vision: tiled({ characterOnMap: false }), viewer: { name: 'Brisa', own: false } });
    expect(plain(el.querySelector('[data-testid="fog-off-map"]'))).toContain('O personagem não está neste mapa: o jogador vê só o que já tinha visto.');
  });

  it('names every state the map draws, in the order of MAP-LANGUAGE.md, then the layers and the tokens', () => {
    const { fixture, el } = create({ tokens: [pensantus, toren, goblin, nanquim] });
    settle(fixture);
    const items = Array.from(el.querySelectorAll('.mr-legend li'), (li) => plain(li));
    expect(items).toEqual([
      'Visto',
      'Penumbra',
      'No escuro, em cinza',
      'Já visto',
      'Não visto',
      'Parede',
      'Meia cobertura',
      'P Você',
      'B Companheiro',
      'G Inimigo à vista',
      'N Criatura',
    ]);
  });

  it('names only what is on the map: a map with nothing remembered has no "Já visto"', () => {
    const { el } = create({ vision: decodeVision(visionResponse(['BB', 'dd'])), layers: null, tokens: [pensantus] });
    expect(Array.from(el.querySelectorAll('.mr-legend li'), (li) => plain(li))).toEqual(['Visto', 'Penumbra', 'P Você']);
  });

  it('writes what the viewer sees: the squares and the enemies in sight', () => {
    const { fixture, el } = create();
    settle(fixture);
    expect(plain(el.querySelector('[data-testid="fog-caption"]'))).toContain('Você vê 3 de 16 quadrados à vista. Inimigos à vista: Goblin 2.');
  });

  it('draws the party with its own marks: the viewer with the garnet ring, an NPC as a square, a creature with a dashed ring', () => {
    const { fixture, el } = create({ tokens: [pensantus, toren, goblin, nanquim] });
    settle(fixture);
    expect(el.querySelectorAll('app-map-token.tk--mine').length).toBe(1);
    expect(el.querySelectorAll('app-map-token.tk--npc').length).toBe(1);
    expect(el.querySelectorAll('app-map-token.tk--creature').length).toBe(1);
    expect(plain(el.querySelector('app-map-token.tk--npc'))).toBe('G2');
  });

  it('has the zoom buttons, 44 px, named, that start disabled at the whole map', () => {
    const { el } = create();
    const buttons = Array.from(el.querySelectorAll<HTMLButtonElement>('.fm__zoom button'));
    expect(buttons.map((b) => b.getAttribute('aria-label'))).toEqual(['Reduzir', 'Ampliar', 'Ajustar à tela']);
    expect(buttons.map((b) => b.disabled)).toEqual([true, false, true]);
  });

  it('on a phone, puts the party in chips above the map, and a tap takes the view to that character', () => {
    vi.stubGlobal('matchMedia', (query: string) => ({ matches: true, media: query, addEventListener: () => undefined, removeEventListener: () => undefined }));
    const { fixture, el } = create({ tokens: [pensantus, toren, goblin, nanquim] });
    settle(fixture);
    const chips = Array.from(el.querySelectorAll<HTMLButtonElement>('.fm__chip'));
    expect(chips.map((c) => plain(c))).toEqual(['P Pensantus (você)', 'T Toren']);
    expect(plain(el.querySelector('.fm__hint'))).toBe('Dois dedos para ampliar');
    const focus = vi.spyOn(fixture.componentInstance['view']()!, 'focusOn');
    chips[1].click();
    expect(focus).toHaveBeenCalledWith({ xBp: 3000, yBp: 2000 });
  });

  it('shows the badge of the master\'s "Ver como" over the map', () => {
    const { el } = create({ badge: 'Vendo como Toren (Caio)' });
    expect(plain(el.querySelector('.fm__badge'))).toBe('visibility Vendo como Toren (Caio)');
  });
});
