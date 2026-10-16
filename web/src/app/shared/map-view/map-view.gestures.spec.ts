import { type ComponentFixture, TestBed } from '@angular/core/testing';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { mapPoint, mapToken } from '../../core/maps/maps-testing';
import { type MapMove, MapView } from './map-view';

const IMAGE = { url: '/images/img-1', width: 2400, height: 1600 };
const VIEW = { left: 0, top: 0, width: 1000, height: 600 };

interface Internals {
  transform(): { x: number; y: number; scale: number };
  size: { set(v: { width: number; height: number }): void };
}

/** The pan, drag, pinch and wheel of the map: what it reports (`moved`, `emptyClick`) and how the view moves. */
describe('MapView, pointers and keys', () => {
  let fixture: ComponentFixture<MapView>;
  let el: HTMLElement;
  let moves: MapMove[];
  let clicks: { xBp: number; yBp: number }[];

  const taverna = mapPoint('p1', 'Taverna', { revealed: true, xBp: 5000, yBp: 5000 });
  const guarda = mapToken('t1', 'Guarda', { xBp: 1000, yBp: 1000 });

  function setup(inputs: Record<string, unknown> = {}): void {
    fixture = TestBed.createComponent(MapView);
    const ref = fixture.componentRef;
    ref.setInput('image', IMAGE);
    ref.setInput('points', [taverna]);
    ref.setInput('tokens', [guarda]);
    ref.setInput('isMaster', true);
    ref.setInput('mode', 'edit');
    for (const [k, v] of Object.entries(inputs)) {
      ref.setInput(k, v);
    }
    moves = [];
    clicks = [];
    fixture.componentInstance.moved.subscribe((m) => moves.push(m));
    fixture.componentInstance.emptyClick.subscribe((c) => clicks.push(c));
    fixture.detectChanges();
    el = fixture.nativeElement as HTMLElement;
    viewport().getBoundingClientRect = () => ({ ...VIEW, right: 1000, bottom: 600 }) as DOMRect;
    internals().size.set({ width: VIEW.width, height: VIEW.height });
  }

  const internals = () => fixture.componentInstance as unknown as Internals;
  const viewport = () => el.querySelector<HTMLElement>('.mv')!;
  const item = (key: string) => el.querySelector<HTMLElement>(`[data-item="${key}"]`)!;

  function pointer(
    target: Element,
    type: string,
    over: { x?: number; y?: number; id?: number; button?: number; kind?: string } = {},
  ): void {
    const event = new MouseEvent(type, {
      bubbles: true,
      clientX: over.x ?? 0,
      clientY: over.y ?? 0,
      button: over.button ?? 0,
    });
    Object.defineProperty(event, 'pointerId', { value: over.id ?? 1 });
    Object.defineProperty(event, 'pointerType', { value: over.kind ?? 'mouse' });
    target.dispatchEvent(event);
  }

  beforeEach(() => TestBed.configureTestingModule({ imports: [MapView] }));

  describe('dragging', () => {
    it('drags a point of the editor and reports where it was dropped', () => {
      setup();
      pointer(item('point:p1'), 'pointerdown', { x: 500, y: 300 });
      pointer(item('point:p1'), 'pointermove', { x: 250, y: 150 });
      pointer(item('point:p1'), 'pointerup', { x: 250, y: 150 });
      expect(moves).toEqual([{ kind: 'point', id: 'p1', xBp: 2500, yBp: 2500 }]);
    });

    it('drags a token where tokens move, but not a point', () => {
      setup({ mode: 'tokens' });
      pointer(item('point:p1'), 'pointerdown', { x: 500, y: 300 });
      pointer(item('point:p1'), 'pointermove', { x: 100, y: 100 });
      pointer(item('point:p1'), 'pointerup');
      expect(moves).toEqual([]);
      const token = el.querySelector<HTMLElement>('[data-item^="token:"]')!;
      pointer(token, 'pointerdown', { x: 100, y: 60 });
      pointer(token, 'pointermove', { x: 500, y: 300 });
      pointer(token, 'pointerup', { x: 500, y: 300 });
      expect(moves).toHaveLength(1);
      expect(moves[0]).toMatchObject({ kind: 'token', xBp: 5000, yBp: 5000 });
    });

    it('treats a few pixels as a click: nothing moves, an empty spot reports where it was', () => {
      setup();
      pointer(item('point:p1'), 'pointerdown', { x: 500, y: 300 });
      pointer(item('point:p1'), 'pointermove', { x: 502, y: 301 });
      pointer(item('point:p1'), 'pointerup');
      expect(moves).toEqual([]);
      expect(clicks).toEqual([]);
      pointer(viewport(), 'pointerdown', { x: 100, y: 300 });
      pointer(viewport(), 'pointerup', { x: 100, y: 300 });
      expect(clicks).toEqual([{ xBp: 1000, yBp: 5000 }]);
    });

    it('drops nothing when the gesture is cancelled, and ignores another pointer', () => {
      setup();
      pointer(item('point:p1'), 'pointerdown', { id: 1 });
      pointer(item('point:p1'), 'pointermove', { id: 2, x: 300, y: 300 });
      pointer(item('point:p1'), 'pointerup', { id: 2 });
      pointer(item('point:p1'), 'pointermove', { id: 1, x: 300, y: 300 });
      pointer(item('point:p1'), 'pointercancel', { id: 1 });
      expect(moves).toEqual([]);
      pointer(item('point:p1'), 'pointerup', { id: 1 });
      expect(moves).toEqual([]);
    });

    it('ignores the secondary button, and everything in the still preview', () => {
      setup();
      pointer(viewport(), 'pointerdown', { button: 2 });
      pointer(viewport(), 'pointerup');
      expect(clicks).toEqual([]);
      const preview = TestBed.createComponent(MapView);
      preview.componentRef.setInput('image', IMAGE);
      preview.componentRef.setInput('mode', 'preview');
      preview.detectChanges();
      const view = preview.nativeElement.querySelector('.mv') as HTMLElement;
      preview.componentInstance.emptyClick.subscribe((c) => clicks.push(c));
      pointer(view, 'pointerdown');
      pointer(view, 'pointerup');
      view.dispatchEvent(new WheelEvent('wheel', { deltaY: -100, cancelable: true }));
      expect(clicks).toEqual([]);
      expect((preview.componentInstance as unknown as Internals).transform().scale).toBe(1);
    });
  });

  describe('panning and zooming', () => {
    it('pans the zoomed map with the mouse on an empty spot, never with a finger', () => {
      setup();
      fixture.componentInstance.zoomIn();
      fixture.componentInstance.zoomIn();
      const before = internals().transform();
      expect(before.scale).toBeGreaterThan(1);
      pointer(viewport(), 'pointerdown', { x: 500, y: 300, kind: 'touch' });
      pointer(viewport(), 'pointermove', { x: 450, y: 280, kind: 'touch' });
      pointer(viewport(), 'pointerup', { x: 450, y: 280, kind: 'touch' });
      expect(internals().transform()).toEqual(before);
      pointer(viewport(), 'pointerdown', { x: 500, y: 300 });
      pointer(viewport(), 'pointermove', { x: 450, y: 280 });
      pointer(viewport(), 'pointerup', { x: 450, y: 280 });
      const after = internals().transform();
      expect(after.scale).toBe(before.scale);
      expect(after.x).toBe(before.x - 50);
      expect(after.y).toBe(before.y - 20);
      expect(clicks).toEqual([]);
    });

    it('zooms with two fingers and stops when one of them is lifted', () => {
      setup();
      pointer(viewport(), 'pointerdown', { id: 1, x: 400, y: 300, kind: 'touch' });
      pointer(viewport(), 'pointerdown', { id: 2, x: 600, y: 300, kind: 'touch' });
      pointer(viewport(), 'pointermove', { id: 2, x: 800, y: 300, kind: 'touch' });
      const zoomed = internals().transform();
      expect(zoomed.scale).toBeCloseTo(2);
      pointer(viewport(), 'pointerdown', { id: 3, x: 500, y: 100, kind: 'touch' });
      pointer(viewport(), 'pointerup', { id: 3, kind: 'touch' });
      pointer(viewport(), 'pointerup', { id: 2, kind: 'touch' });
      pointer(viewport(), 'pointermove', { id: 1, x: 100, y: 300, kind: 'touch' });
      expect(internals().transform()).toEqual(zoomed);
      pointer(viewport(), 'pointerup', { id: 1, kind: 'touch' });
      expect(clicks).toEqual([]);
    });

    it('zooms around the pointer with the wheel', () => {
      setup();
      const zoomIn = new WheelEvent('wheel', {
        deltaY: -100,
        clientX: 500,
        clientY: 300,
        cancelable: true,
      });
      viewport().dispatchEvent(zoomIn);
      expect(zoomIn.defaultPrevented).toBe(true);
      expect(internals().transform().scale).toBeCloseTo(1.12);
      viewport().dispatchEvent(
        new WheelEvent('wheel', { deltaY: 100, clientX: 500, clientY: 300 }),
      );
      expect(internals().transform().scale).toBeCloseTo(1);
    });
  });

  describe('the keyboard', () => {
    function key(k: string, shift = false): KeyboardEvent {
      const event = new KeyboardEvent('keydown', {
        key: k,
        shiftKey: shift,
        bubbles: true,
        cancelable: true,
      });
      item('point:p1').dispatchEvent(event);
      return event;
    }

    it('nudges the point and reports it on release, 5 % with Shift', () => {
      setup();
      expect(key('ArrowRight').defaultPrevented).toBe(true);
      key('ArrowRight', true);
      item('point:p1').dispatchEvent(
        new KeyboardEvent('keyup', { key: 'ArrowRight', bubbles: true }),
      );
      expect(moves).toEqual([{ kind: 'point', id: 'p1', xBp: 5550, yBp: 5000 }]);
    });

    it('ignores other keys, a key up with nothing nudged and a point it may not move', () => {
      setup();
      expect(key('a').defaultPrevented).toBe(false);
      item('point:p1').dispatchEvent(
        new KeyboardEvent('keyup', { key: 'ArrowRight', bubbles: true }),
      );
      key('ArrowUp');
      item('point:p1').dispatchEvent(new KeyboardEvent('keyup', { key: 'a', bubbles: true }));
      expect(moves).toEqual([]);
      viewport().dispatchEvent(
        new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true, cancelable: true }),
      );
      expect(moves).toEqual([]);
    });
  });

  describe('the labels', () => {
    afterEach(() => vi.restoreAllMocks());

    it('puts each label beside its marker once the image has a size', () => {
      const rect = (left: number, top: number, width: number, height: number): DOMRect =>
        ({ left, top, width, height, right: left + width, bottom: top + height }) as DOMRect;
      vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(function (
        this: Element,
      ) {
        if (this.classList.contains('mv__img')) return rect(0, 0, 1000, 600);
        if (this.classList.contains('lbl__pill')) return rect(0, 0, 80, 20);
        if (this.classList.contains('pt__shape')) return rect(490, 290, 20, 20);
        if (this.classList.contains('lbl')) return rect(500, 300, 0, 0);
        return rect(0, 0, 1000, 600);
      });
      setup();
      fixture.componentRef.setInput('faded', true);
      fixture.detectChanges();
      const label = el.querySelector<HTMLElement>('.lbl')!;
      expect(label.hasAttribute('data-placed')).toBe(true);
      const pill = label.querySelector<HTMLElement>('.lbl__pill')!;
      expect(pill.style.getPropertyValue('--lbl-x')).toMatch(/px$/);
      expect(pill.style.getPropertyValue('--lbl-y')).toMatch(/px$/);
    });
  });
});
