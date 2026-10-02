import { ComponentFixture, TestBed } from '@angular/core/testing';

import { mapPoint, mapToken } from '../../core/maps/maps-testing';
import { MapMove, MapView } from './map-view';

const IMAGE = { url: '/images/img-1', width: 2400, height: 1600 };

describe('MapView', () => {
  let fixture: ComponentFixture<MapView>;
  let el: HTMLElement;
  let moves: MapMove[];

  const taverna = mapPoint('p1', 'Taverna do Javali', { revealed: true, xBp: 5000, yBp: 5000 });
  const covil = mapPoint('p2', 'Covil dos goblins', { revealed: false, kind: 2 });
  const pensantus = mapToken('t1', 'Pensantus', { mine: true });
  const capitao = mapToken('t2', 'Capitão Goblin', { hidden: true });

  function setup(inputs: Record<string, unknown>): void {
    fixture = TestBed.createComponent(MapView);
    fixture.componentRef.setInput('image', IMAGE);
    fixture.componentRef.setInput('mapName', 'Mirathel e arredores');
    fixture.componentRef.setInput('points', [taverna, covil]);
    fixture.componentRef.setInput('tokens', [pensantus, capitao]);
    for (const [key, value] of Object.entries(inputs)) {
      fixture.componentRef.setInput(key, value);
    }
    moves = [];
    fixture.componentInstance.moved.subscribe((m) => moves.push(m));
    fixture.detectChanges();
    el = fixture.nativeElement as HTMLElement;
  }

  function item(key: string): HTMLButtonElement {
    return el.querySelector(`[data-item="${key}"]`) as HTMLButtonElement;
  }

  beforeEach(() => TestBed.configureTestingModule({ imports: [MapView] }));

  it('reserves the image size before it loads', () => {
    setup({});
    expect((el.querySelector('.mv') as HTMLElement).style.aspectRatio).toBe('2400 / 1600');
    const img = el.querySelector('img') as HTMLImageElement;
    expect(img.getAttribute('width')).toBe('2400');
    expect(img.getAttribute('height')).toBe('1600');
  });

  it('draws only the revealed points and the visible tokens for a player', () => {
    setup({ isMaster: false });
    expect(item('point:p1')).toBeTruthy();
    expect(item('point:p2')).toBeNull();
    expect(el.textContent).not.toContain('Covil dos goblins');
    expect(el.textContent).not.toContain('Escondido');
  });

  it('draws hidden things for the master, with the word "Escondido"', () => {
    setup({ isMaster: true, mode: 'tokens' });
    expect(item('point:p2').getAttribute('aria-label')).toBe(
      'Covil dos goblins, Submapa, escondido',
    );
    expect(el.querySelectorAll('.lbl--hidden')).toHaveLength(1);
    expect(el.querySelector('.lbl--hidden')?.textContent).toContain('Escondido');
    expect(item('token:t2').getAttribute('aria-label')).toBe('Capitão Goblin, escondido');
    expect(item('token:t1').getAttribute('aria-label')).toBe('Pensantus (você), visível');
  });

  it('labels the markers with the name and the kind', () => {
    setup({});
    expect(item('point:p1').getAttribute('aria-label')).toBe('Taverna do Javali, Cena de RP');
  });

  it('opens a point on click, and a token only where tokens move', () => {
    setup({ isMaster: true, mode: 'view' });
    const selected: string[] = [];
    fixture.componentInstance.pointSelect.subscribe((id) => selected.push(id));
    item('point:p1').click();
    expect(selected).toEqual(['p1']);
    expect(item('token:t1')).toBeNull(); // not a button in the view mode
  });

  it('keeps the markers inert when points are not selectable', () => {
    setup({ isMaster: true, selectablePoints: false });
    expect(item('point:p1')).toBeNull();
    expect(el.querySelectorAll('app-map-marker').length).toBe(2);
  });

  it('is a still picture in preview mode: no buttons, no zoom controls', () => {
    setup({ mode: 'preview', controls: 'overlay' });
    expect(el.querySelectorAll('button')).toHaveLength(0);
    expect(el.querySelector('.mv__controls')).toBeNull();
    expect(el.querySelector('.mv')?.getAttribute('role')).toBe('img');
  });

  it('moves the selected item with the arrow keys and reports it on key release', () => {
    setup({ isMaster: true, mode: 'edit', selected: { kind: 'point', id: 'p1' } });
    const marker = item('point:p1');
    const press = (type: string, init: KeyboardEventInit) =>
      marker.dispatchEvent(new KeyboardEvent(type, { bubbles: true, cancelable: true, ...init }));

    press('keydown', { key: 'ArrowRight' });
    press('keydown', { key: 'ArrowRight' });
    press('keydown', { key: 'ArrowDown', shiftKey: true });
    expect(moves).toEqual([]); // nothing is saved while the key is down
    fixture.detectChanges();
    expect((marker.closest('app-map-marker') as HTMLElement).style.left).toBe('51%');
    press('keyup', { key: 'ArrowDown' });
    expect(moves).toEqual([{ kind: 'point', id: 'p1', xBp: 5100, yBp: 5500 }]);
  });

  it('does not move a point in a mode that does not edit points', () => {
    setup({ isMaster: true, mode: 'tokens' });
    const marker = item('point:p1');
    expect(marker).toBeTruthy();
    marker.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    marker.dispatchEvent(new KeyboardEvent('keyup', { key: 'ArrowRight', bubbles: true }));
    expect(moves).toEqual([]);
  });

  it('moves a token with the keyboard where tokens move', () => {
    setup({ isMaster: true, mode: 'tokens' });
    const token = item('token:t1');
    token.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true }));
    token.dispatchEvent(new KeyboardEvent('keyup', { key: 'ArrowUp', bubbles: true }));
    expect(moves).toEqual([{ kind: 'token', id: 't1', xBp: 4000, yBp: 3950 }]);
  });

  it('zooms with the buttons from 100 % to 400 % and fits again', () => {
    setup({ mode: 'view' });
    const view = fixture.componentInstance;
    const buttons = (label: string) =>
      el.querySelector(`button[aria-label="${label}"]`) as HTMLButtonElement;
    expect(buttons('Reduzir').disabled).toBe(true);
    buttons('Ampliar').click();
    expect(view.scale()).toBe(1.25);
    for (let i = 0; i < 20; i++) {
      view.zoomIn();
    }
    expect(view.scale()).toBe(4);
    fixture.detectChanges();
    expect(buttons('Ampliar').disabled).toBe(true);
    buttons('Ajustar à tela').click();
    expect(view.scale()).toBe(1);
  });

  it('raises an overlapped item while it is hovered or focused', () => {
    setup({ isMaster: true, mode: 'tokens' });
    item('token:t1').dispatchEvent(new Event('focusin', { bubbles: true }));
    fixture.detectChanges();
    expect(el.querySelector('app-map-token.tk--raised')).toBeTruthy();
    item('token:t1').dispatchEvent(new Event('focusout', { bubbles: true }));
    fixture.detectChanges();
    expect(el.querySelector('app-map-token.tk--raised')).toBeNull();
  });

  it('gives two tokens with the same letter two letters', () => {
    setup({
      isMaster: true,
      mode: 'tokens',
      tokens: [mapToken('a', 'Brisa'), mapToken('b', 'Boris'), mapToken('c', 'Toren')],
    });
    const discs = Array.from(el.querySelectorAll('.tk__disc')).map((d) => d.textContent?.trim());
    expect(discs).toEqual(['Br', 'Bo', 'T']);
  });
});
