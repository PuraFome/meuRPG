import { TestBed } from '@angular/core/testing';

import type { MapLayers } from '../../core/maps/layers';
import { MapLayersLegend } from './map-layers-legend';
import { MapLayersOverlay } from './map-layers';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();

const layers: MapLayers = {
  columns: 10,
  rows: 5,
  walls: [{ col: 0, row: 0 }],
  terrain: [
    { col: 2, row: 2 },
    { col: 3, row: 2 },
  ],
  half: [{ col: 5, row: 4 }],
  threeQuarters: [],
};

describe('MapLayersOverlay', () => {
  it('draws one mark for each painted square, at its place in the grid, hidden from the accessibility tree', () => {
    const fixture = TestBed.createComponent(MapLayersOverlay);
    fixture.componentRef.setInput('layers', layers);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.getAttribute('aria-hidden')).toBe('true');
    expect(el.querySelectorAll('.sq--wall').length).toBe(1);
    expect(el.querySelectorAll('.sq--terrain').length).toBe(2);
    expect(el.querySelectorAll('.sq--half').length).toBe(1);
    expect(el.querySelectorAll('.sq--three').length).toBe(0);
    const half = el.querySelector<HTMLElement>('.sq--half')!;
    expect(half.style.left).toBe('50%');
    expect(half.style.top).toBe('80%');
  });
});

describe('MapLayersLegend', () => {
  it('names only the marks the map has, in the order of MAP-LANGUAGE.md, and projects the screen\'s own', () => {
    const fixture = TestBed.createComponent(MapLayersLegend);
    fixture.componentRef.setInput('layers', layers);
    fixture.detectChanges();
    const items = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('li'), (li) => plain(li.textContent));
    expect(items).toEqual(['Parede', 'Terreno difícil', 'Meia cobertura']);
  });

  it('is empty for a map with nothing painted', () => {
    const fixture = TestBed.createComponent(MapLayersLegend);
    fixture.componentRef.setInput('layers', { columns: 4, rows: 4, walls: [], terrain: [], half: [], threeQuarters: [] });
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).querySelectorAll('li').length).toBe(0);
  });
});

describe('MapLayersOverlay: the editor\'s marks', () => {
  const lit: MapLayers = {
    ...layers,
    light: { bright: [{ col: 1, row: 1 }], dim: [{ col: 2, row: 1 }], dark: [{ col: 3, row: 1 }] },
  };

  it('draws the painted light as a glyph in the square, a sun, a half moon and a moon, only when asked to', () => {
    const fixture = TestBed.createComponent(MapLayersOverlay);
    fixture.componentRef.setInput('layers', lit);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelectorAll('.sq--light')).toHaveLength(0);
    fixture.componentRef.setInput('lightGlyphs', true);
    fixture.detectChanges();
    expect(Array.from(el.querySelectorAll('.sq--light .lg'), (g) => g.textContent?.trim())).toEqual(['light_mode', 'contrast', 'dark_mode']);
    // A glyph, never a texture: no hatch on the square.
    expect(el.querySelector('.sq--light')?.classList.contains('sq--wall')).toBe(false);
  });

  it('draws no light for a map whose layers carry none (a player\'s)', () => {
    const fixture = TestBed.createComponent(MapLayersOverlay);
    fixture.componentRef.setInput('layers', layers);
    fixture.componentRef.setInput('lightGlyphs', true);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).querySelectorAll('.sq--light')).toHaveLength(0);
  });

  it('draws the brush cursor over the squares a stroke would paint, dashed when it erases', () => {
    const fixture = TestBed.createComponent(MapLayersOverlay);
    fixture.componentRef.setInput('layers', layers);
    fixture.componentRef.setInput('cursor', { col: 2, row: 1, w: 3, h: 3 });
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const cursor = el.querySelector<HTMLElement>('.cursor')!;
    expect(cursor.style.left).toBe('20%');
    expect(cursor.style.top).toBe('20%');
    expect(cursor.style.width).toBe('30%');
    expect(cursor.style.height).toBe('60%');
    expect(cursor.classList.contains('cursor--erase')).toBe(false);
    fixture.componentRef.setInput('cursor', { col: 2, row: 1, w: 1, h: 1, erase: true });
    fixture.detectChanges();
    expect(el.querySelector('.cursor')?.classList.contains('cursor--erase')).toBe(true);
    fixture.componentRef.setInput('cursor', null);
    fixture.detectChanges();
    expect(el.querySelector('.cursor')).toBeNull();
  });
});
