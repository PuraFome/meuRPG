import { TestBed } from '@angular/core/testing';

import type { DoorSquare, MapLayers } from '../../core/maps/layers';
import { DoorPicks } from './door-picks';

const layers: MapLayers = {
  columns: 10,
  rows: 5,
  walls: [],
  terrain: [],
  half: [],
  threeQuarters: [],
  doors: [
    { col: 2, row: 1, state: 2, axis: 'h' },
    { col: 6, row: 3, state: 5, axis: 'v' },
  ],
};

describe('DoorPicks', () => {
  function setup(l: MapLayers = layers) {
    const fixture = TestBed.createComponent(DoorPicks);
    fixture.componentRef.setInput('layers', l);
    fixture.componentRef.setInput('columns', 10);
    fixture.componentRef.setInput('rows', 5);
    const picked: DoorSquare[] = [];
    fixture.componentInstance.pick.subscribe((d) => picked.push(d));
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, picked, fixture };
  }

  it('is one button over each door, named by its kind and its place, with a way in for the keyboard', () => {
    const { el } = setup();
    const buttons = Array.from(el.querySelectorAll<HTMLButtonElement>('button'));
    expect(buttons.map((b) => b.getAttribute('aria-label'))).toEqual([
      'Porta fechada, coluna 3, linha 2. Abrir as opções da porta',
      'Porta secreta, coluna 7, linha 4. Abrir as opções da porta',
    ]);
    // Centred on the door's square.
    expect(buttons[0].style.left).toBe('25%');
    expect(buttons[0].style.top).toBe('30%');
  });

  it('tells the page which door was tapped', () => {
    const { el, picked } = setup();
    el.querySelectorAll<HTMLButtonElement>('button')[1].click();
    expect(picked).toEqual([{ col: 6, row: 3, state: 5, axis: 'v' }]);
  });

  it('has nothing for a map with no doors', () => {
    const { el } = setup({ ...layers, doors: undefined });
    expect(el.querySelectorAll('button')).toHaveLength(0);
  });

  it('is one tab stop: only one door is tabbable, and the arrow keys walk the doors in reading order', () => {
    const { el, fixture } = setup();
    const buttons = Array.from(el.querySelectorAll<HTMLButtonElement>('button'));
    expect(buttons.map((b) => b.tabIndex)).toEqual([0, -1]);
    buttons[0].focus();
    buttons[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    expect(document.activeElement).toBe(buttons[1]);
    fixture.detectChanges();
    expect(buttons.map((b) => b.tabIndex)).toEqual([-1, 0]);
    buttons[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
    expect(document.activeElement).toBe(buttons[0]);
  });
});
