import { ComponentFixture, TestBed } from '@angular/core/testing';

import type { Square } from '../../../core/combat/combat-grid';
import { PaintSurface, type Stroke } from './paint-surface';

// A 24 x 16 grid on a 240 x 160 box: every square is 10 px.
describe('PaintSurface', () => {
  let fixture: ComponentFixture<PaintSurface>;
  let host: HTMLElement;
  let strokes: Stroke[];
  let ends: number;
  let hovers: (Square | null)[];
  let left: number;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    fixture = TestBed.createComponent(PaintSurface);
    fixture.componentRef.setInput('columns', 24);
    fixture.componentRef.setInput('rows', 16);
    fixture.componentRef.setInput('cursor', null);
    strokes = [];
    hovers = [];
    ends = 0;
    left = 0;
    fixture.componentInstance.stroke.subscribe((s) => strokes.push(s));
    fixture.componentInstance.hover.subscribe((h) => hovers.push(h));
    fixture.componentInstance.strokeEnd.subscribe(() => ends++);
    fixture.componentInstance.leave.subscribe(() => left++);
    fixture.detectChanges();
    host = fixture.nativeElement;
    host.getBoundingClientRect = () => ({ left: 100, top: 50, width: 240, height: 160, right: 340, bottom: 210, x: 100, y: 50, toJSON: () => ({}) });
  });

  function pointer(type: string, x: number, y: number, init: MouseEventInit = {}): void {
    const event = new MouseEvent(type, { clientX: 100 + x, clientY: 50 + y, bubbles: true, button: 0, ...init });
    Object.defineProperty(event, 'pointerId', { value: 1 });
    host.dispatchEvent(event);
  }

  it('is a group of the app with a name that says the keys', () => {
    expect(host.getAttribute('role')).toBe('application');
    expect(host.getAttribute('tabindex')).toBe('0');
    expect(host.getAttribute('aria-label')).toContain('Espaço para pintar');
  });

  it('a click paints the square under it', () => {
    pointer('pointerdown', 25, 35);
    pointer('pointerup', 25, 35);
    expect(strokes).toEqual([{ centers: [{ col: 2, row: 3 }], erase: false }]);
    expect(ends).toBe(1);
  });

  it('a drag paints square by square, with none skipped between two events', () => {
    pointer('pointerdown', 5, 5);
    pointer('pointermove', 45, 5);
    pointer('pointerup', 45, 5);
    expect(strokes.flatMap((s) => s.centers)).toEqual([0, 1, 2, 3, 4].map((col) => ({ col, row: 0 })));
    expect(ends).toBe(1);
  });

  it('Shift erases, and moving without pressing only moves the brush cursor', () => {
    pointer('pointermove', 15, 15);
    expect(strokes).toEqual([]);
    expect(hovers.at(-1)).toEqual({ col: 1, row: 1 });
    pointer('pointerdown', 15, 15, { shiftKey: true });
    expect(strokes[0].erase).toBe(true);
  });

  it('lets the map pan under Alt, and does not paint with another button', () => {
    pointer('pointerdown', 15, 15, { altKey: true });
    pointer('pointerdown', 15, 15, { button: 2 });
    expect(strokes).toEqual([]);
  });

  it('keeps a square off the grid inside it', () => {
    pointer('pointerdown', 9999, 9999);
    expect(strokes[0].centers).toEqual([{ col: 23, row: 15 }]);
  });

  it('moves the cursor with the arrows, paints on Space, erases on Shift+Space and leaves on Esc', () => {
    host.dispatchEvent(new Event('focus'));
    expect(hovers.at(-1)).toEqual({ col: 12, row: 8 });
    fixture.componentRef.setInput('cursor', { col: 12, row: 8 });
    fixture.detectChanges();
    host.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight' }));
    expect(hovers.at(-1)).toEqual({ col: 13, row: 8 });
    host.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', code: 'Space' }));
    expect(strokes.at(-1)).toEqual({ centers: [{ col: 12, row: 8 }], erase: false });
    host.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', code: 'Space', shiftKey: true }));
    expect(strokes.at(-1)?.erase).toBe(true);
    host.dispatchEvent(new KeyboardEvent('keyup', { key: ' ', code: 'Space' }));
    expect(ends).toBe(1);
    host.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(left).toBe(1);
    expect(hovers.at(-1)).toBeNull();
  });
});
