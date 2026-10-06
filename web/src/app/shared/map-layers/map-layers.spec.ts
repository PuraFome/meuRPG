import { TestBed } from '@angular/core/testing';

import { type MapLayers, decodeLayers } from '../../core/maps/layers';
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
    // One plain element a square (the font's ligature is drawn by its stylesheet): a sun, a half moon, a moon.
    expect(Array.from(el.querySelectorAll('.sq--light .lg'), (g) => g.className)).toEqual(['lg lg--bright', 'lg lg--dim', 'lg lg--dark']);
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

// The doors (RN-26, MAP-LANGUAGE-E10.md): one mark per kind, and the legend names only the doors the map really has.
describe('the doors on a map', () => {
  const doors: MapLayers = {
    ...layers,
    doors: [
      { col: 1, row: 1, state: 2, axis: 'h' },
      { col: 3, row: 1, state: 1, axis: 'h' },
      { col: 5, row: 1, state: 3, axis: 'v' },
      { col: 7, row: 1, state: 4, axis: 'h' },
      { col: 9, row: 1, state: 5, axis: 'h' },
    ],
  };

  it('draws one door mark a door, at its square, each kind in its own drawing', () => {
    const fixture = TestBed.createComponent(MapLayersOverlay);
    fixture.componentRef.setInput('layers', doors);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const marks = Array.from(el.querySelectorAll('.sq--door'));
    expect(marks).toHaveLength(5);
    expect((marks[0] as HTMLElement).style.left).toBe('10%');
    expect((marks[0] as HTMLElement).style.top).toBe('20%');
    // Closed and locked: the bar, and only the locked one has the padlock. Open: a leaf and a dashed arc. Grade: the comb. Secret: the hatch, a frame and a keyhole.
    expect(marks[0].querySelectorAll('.dm__ink')).toHaveLength(1);
    expect(marks[0].querySelector('.dm__lock')).toBeNull();
    expect(marks[1].querySelector('.dm__arc')).not.toBeNull();
    expect(marks[1].querySelector('.dm__leaf')).not.toBeNull();
    expect(marks[2].querySelector('svg.dm__lock')).not.toBeNull();
    expect(marks[2].querySelector('.dm__svg--v')).not.toBeNull();
    expect(marks[3].querySelector('path.dm__ink')?.getAttribute('d')).toContain('V86');
    expect(marks[4].querySelector('.dm__frame')).not.toBeNull();
    expect(marks[4].querySelector('.dm__key')).not.toBeNull();
  });

  it('a player\'s view draws no padlock and no keyhole even if the layer carried a locked and a secret door (RN-10)', () => {
    // Squares 0 (locked), 1 (secret) and 2 (grade) of a 3 x 1 grid, decoded as a player.
    const decoded = decodeLayers(
      { gridColumns: 3, gridRows: 1, difficultTerrain: new Uint8Array(), wall: new Uint8Array(), cover: new Uint8Array(), doors: Uint8Array.of(0x53, 0x04) },
      true,
    );
    const fixture = TestBed.createComponent(MapLayersOverlay);
    fixture.componentRef.setInput('layers', decoded);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.dm__lock')).toBeNull();
    expect(el.querySelector('.dm__key')).toBeNull();
    expect(el.querySelectorAll('.sq--door')).toHaveLength(2);
    const legend = TestBed.createComponent(MapLayersLegend);
    legend.componentRef.setInput('layers', decoded);
    legend.detectChanges();
    const items = Array.from((legend.nativeElement as HTMLElement).querySelectorAll('li'), (li) => plain(li.textContent));
    expect(items).toEqual(['Porta fechada', 'Grade']);
  });

  it('names the doors the map has, after "Parede", in the order of the map language; the padlock and the secret door say "só você vê" with the crossed eye', () => {
    const fixture = TestBed.createComponent(MapLayersLegend);
    fixture.componentRef.setInput('layers', doors);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const items = Array.from(el.querySelectorAll('li'), (li) => plain(li.textContent));
    expect(items).toEqual([
      'Parede',
      'Porta fechada',
      'Porta aberta',
      'Porta trancada (só você vê)visibility_off',
      'Grade',
      'Porta secreta (só você vê)visibility_off',
      'Terreno difícil',
      'Meia cobertura',
    ]);
    expect(el.querySelectorAll('.nm mat-icon')).toHaveLength(2);
  });

  it('lists a kind only when the map has one: a player\'s map with a closed door says "Porta fechada" and nothing about a lock', () => {
    const fixture = TestBed.createComponent(MapLayersLegend);
    fixture.componentRef.setInput('layers', { ...layers, doors: [{ col: 1, row: 1, state: 2, axis: 'h' }] });
    fixture.detectChanges();
    const items = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('li'), (li) => plain(li.textContent));
    expect(items).toContain('Porta fechada');
    expect(items.join('|')).not.toContain('trancada');
    expect(items.join('|')).not.toContain('secreta');
  });

  it('the door tool\'s cursor holds the kind it would paint, and says it in words', () => {
    const fixture = TestBed.createComponent(MapLayersOverlay);
    fixture.componentRef.setInput('layers', layers);
    fixture.componentRef.setInput('cursor', { col: 2, row: 2, w: 1, h: 1, door: { state: 3, label: 'Fechada → Trancada' } });
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.cursor .cursor__door')).not.toBeNull();
    expect(plain(el.querySelector('.cursor__tag')?.textContent)).toBe('Fechada → Trancada');
  });
});
