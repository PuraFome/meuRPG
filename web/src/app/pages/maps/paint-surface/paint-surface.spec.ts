import { ComponentFixture, TestBed } from '@angular/core/testing';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

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
    host.getBoundingClientRect = () => ({
      left: 100,
      top: 50,
      width: 240,
      height: 160,
      right: 340,
      bottom: 210,
      x: 100,
      y: 50,
      toJSON: () => ({}),
    });
  });

  function pointer(
    type: string,
    x: number,
    y: number,
    init: MouseEventInit & { id?: number; kind?: string } = {},
  ): void {
    const { id = 1, kind = 'mouse', ...mouse } = init;
    const event = new MouseEvent(type, {
      clientX: 100 + x,
      clientY: 50 + y,
      bubbles: true,
      button: 0,
      ...mouse,
    });
    Object.defineProperty(event, 'pointerId', { value: id });
    Object.defineProperty(event, 'pointerType', { value: kind });
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
    expect(strokes.flatMap((s) => s.centers)).toEqual(
      [0, 1, 2, 3, 4].map((col) => ({ col, row: 0 })),
    );
    expect(ends).toBe(1);
  });

  it('Shift erases, and moving without pressing only moves the brush cursor', () => {
    pointer('pointermove', 15, 15);
    expect(strokes).toEqual([]);
    expect(hovers.at(-1)).toEqual({ col: 1, row: 1 });
    pointer('pointerdown', 15, 15, { shiftKey: true });
    expect(strokes[0].erase).toBe(true);
  });

  it('a mouse press stays with the surface: the map view does not see it', () => {
    const reached: string[] = [];
    host.parentElement?.addEventListener('pointerdown', () => reached.push('down'));
    pointer('pointerdown', 25, 35);
    expect(reached).toEqual([]);
  });

  it('lets the map pan under Alt, and does not paint with another button', () => {
    pointer('pointerdown', 15, 15, { altKey: true });
    pointer('pointerdown', 15, 15, { button: 2 });
    expect(strokes).toEqual([]);
  });

  it('paints nothing outside the map: a press beyond it, or a drag that leaves it, never paints the edge', () => {
    pointer('pointerdown', 9999, 9999);
    expect(strokes).toEqual([]);
    pointer('pointerup', 9999, 9999);
    pointer('pointerdown', 235, 5);
    pointer('pointermove', 400, 5);
    pointer('pointerup', 400, 5);
    expect(strokes.flatMap((s) => s.centers)).toEqual([{ col: 23, row: 0 }]);
  });

  it('tells the editor about the brush only when it changes square', () => {
    pointer('pointermove', 12, 12);
    pointer('pointermove', 14, 15);
    pointer('pointermove', 18, 12);
    expect(hovers).toEqual([{ col: 1, row: 1 }]);
    pointer('pointermove', 22, 12);
    expect(hovers).toHaveLength(2);
  });

  it('says the square aloud, column and row from 1, when the arrows move the brush', () => {
    fixture.componentRef.setInput('cursor', { col: 12, row: 8 });
    fixture.detectChanges();
    host.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight' }));
    fixture.detectChanges();
    expect(host.querySelector('[aria-live="polite"]')?.textContent).toBe('Coluna 14, linha 9');
  });

  describe('on a touch screen', () => {
    beforeEach(() => vi.useFakeTimers());
    afterEach(() => vi.useRealTimers());

    it('one finger paints (its first square a moment after it lands), and lets the touch reach the map view', () => {
      pointer('pointerdown', 25, 35, { kind: 'touch' });
      expect(strokes).toEqual([]);
      vi.advanceTimersByTime(100);
      expect(strokes).toEqual([{ centers: [{ col: 2, row: 3 }], erase: false }]);
      pointer('pointermove', 55, 35, { kind: 'touch' });
      expect(strokes.flatMap((s) => s.centers).at(-1)).toEqual({ col: 5, row: 3 });
      pointer('pointerup', 55, 35, { kind: 'touch' });
      expect(ends).toBe(1);
    });

    it('two fingers pan and zoom: the first one leaves no mark and nothing paints until both are up', () => {
      const reached: string[] = [];
      host.parentElement?.addEventListener('pointerdown', () => reached.push('down'));
      pointer('pointerdown', 25, 35, { kind: 'touch', id: 1 });
      pointer('pointerdown', 85, 95, { kind: 'touch', id: 2 });
      vi.advanceTimersByTime(200);
      pointer('pointermove', 45, 35, { kind: 'touch', id: 1 });
      pointer('pointermove', 105, 95, { kind: 'touch', id: 2 });
      expect(strokes).toEqual([]);
      pointer('pointerup', 45, 35, { kind: 'touch', id: 1 });
      pointer('pointerup', 105, 95, { kind: 'touch', id: 2 });
      // Both touches went on to the map view, which pinches.
      expect(reached).toEqual(['down', 'down']);
      pointer('pointerdown', 25, 35, { kind: 'touch', id: 3 });
      vi.advanceTimersByTime(100);
      expect(strokes).toHaveLength(1);
    });
  });

  it('moves the cursor with the arrows, paints on Space, erases on Shift+Space and leaves on Esc', () => {
    host.dispatchEvent(new Event('focus'));
    expect(hovers.at(-1)).toEqual({ col: 12, row: 8 });
    fixture.componentRef.setInput('cursor', { col: 12, row: 8 });
    fixture.detectChanges();
    host.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight' }));
    expect(hovers.at(-1)).toEqual({ col: 13, row: 8 });
    host.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', code: 'Space' }));
    // Space paints where the arrow just put the brush, even before the editor has drawn the move.
    expect(strokes.at(-1)).toEqual({ centers: [{ col: 13, row: 8 }], erase: false });
    host.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', code: 'Space', shiftKey: true }));
    expect(strokes.at(-1)?.erase).toBe(true);
    host.dispatchEvent(new KeyboardEvent('keyup', { key: ' ', code: 'Space' }));
    expect(ends).toBe(1);
    host.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(left).toBe(1);
    expect(hovers.at(-1)).toBeNull();
  });
});
