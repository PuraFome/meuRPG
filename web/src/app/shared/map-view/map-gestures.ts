import { type WritableSignal } from '@angular/core';

import {
  type ViewTransform,
  clampTransform,
  distance,
  screenToBp,
  zoomAround,
} from './map-geometry';

/** What is selected on the map, and where it moved: see `MapMove` in `map-view`. */
export interface GestureMove {
  readonly kind: 'point' | 'token';
  readonly id: string;
  readonly xBp: number;
  readonly yBp: number;
}

/** What the gestures need from the map. */
export interface GestureHost {
  interactive(): boolean;
  viewport(): HTMLElement;
  readonly transform: WritableSignal<ViewTransform>;
  size(): { width: number; height: number };
  readonly override: WritableSignal<GestureMove | null>;
  /** The item as something the mode lets the person move, with its position; `null` otherwise. */
  movable(item: string | null): GestureMove | null;
  itemOf(target: EventTarget | null): string | null;
  report(move: GestureMove): void;
  emptyClick(at: { xBp: number; yBp: number }): void;
}

/** A pointer movement under this many pixels is a click, not a drag. */
const DRAG_THRESHOLD = 4;
/** One wheel notch zooms by this factor. */
const WHEEL_STEP = 1.12;

function midpoint(a: { x: number; y: number }, b: { x: number; y: number }) {
  return { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
}

/**
 * The pan, drag, pinch and wheel of the map: the mouse drags an empty spot to pan or an item to move it, two fingers pan and
 * pinch (one finger still scrolls the page), the wheel zooms around the pointer.
 */
export class MapGestures {
  private readonly pointers = new Map<number, { x: number; y: number }>();
  private gesture: {
    id: number;
    item: string | null;
    startX: number;
    startY: number;
    origin: ViewTransform;
    moved: boolean;
    touch: boolean;
  } | null = null;
  private pinch: { dist: number; mid: { x: number; y: number }; origin: ViewTransform } | null =
    null;

  constructor(private readonly host: GestureHost) {}

  down(event: PointerEvent): void {
    if (!this.host.interactive() || event.button > 0) {
      return;
    }
    const item = this.host.itemOf(event.target);
    this.pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
    if (this.pointers.size === 2) {
      const [a, b] = [...this.pointers.values()];
      this.pinch = { dist: distance(a, b), mid: midpoint(a, b), origin: this.host.transform() };
      this.gesture = null;
      this.host.override.set(null);
      return;
    }
    if (this.pointers.size > 2) {
      return;
    }
    this.gesture = {
      id: event.pointerId,
      item,
      startX: event.clientX,
      startY: event.clientY,
      origin: this.host.transform(),
      moved: false,
      touch: event.pointerType === 'touch',
    };
  }

  move(event: PointerEvent): void {
    if (this.pointers.has(event.pointerId)) {
      this.pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
    }
    if (this.pinch && this.pointers.size === 2) {
      this.pinchMove();
      return;
    }
    const g = this.gesture;
    if (!g || g.id !== event.pointerId) {
      return;
    }
    const dx = event.clientX - g.startX;
    const dy = event.clientY - g.startY;
    if (!g.moved && Math.hypot(dx, dy) < DRAG_THRESHOLD) {
      return;
    }
    const target = this.host.movable(g.item);
    if (!g.moved) {
      g.moved = true;
      if (target || !g.touch) {
        this.host.viewport().setPointerCapture?.(event.pointerId);
      }
    }
    if (target) {
      const at = screenToBp(
        event.clientX,
        event.clientY,
        this.host.viewport().getBoundingClientRect(),
        this.host.transform(),
      );
      this.host.override.set({ ...target, ...at });
    } else if (!g.touch) {
      const { width, height } = this.host.size();
      this.host.transform.set(
        clampTransform({ ...g.origin, x: g.origin.x + dx, y: g.origin.y + dy }, width, height),
      );
    }
  }

  up(event: PointerEvent): void {
    this.pointers.delete(event.pointerId);
    if (this.pinch) {
      if (this.pointers.size < 2) {
        this.pinch = null;
        this.gesture = null;
      }
      return;
    }
    const g = this.gesture;
    if (!g || g.id !== event.pointerId) {
      return;
    }
    this.gesture = null;
    const dropped = this.host.override();
    if (event.type === 'pointercancel') {
      this.host.override.set(null);
      return;
    }
    if (g.moved) {
      if (dropped) {
        this.host.override.set(null);
        this.host.report(dropped);
      }
      return;
    }
    if (!g.item) {
      this.host.emptyClick(
        screenToBp(
          event.clientX,
          event.clientY,
          this.host.viewport().getBoundingClientRect(),
          this.host.transform(),
        ),
      );
    }
  }

  private pinchMove(): void {
    const pinch = this.pinch;
    if (!pinch) {
      return;
    }
    const [a, b] = [...this.pointers.values()];
    const rect = this.host.viewport().getBoundingClientRect();
    const mid = midpoint(a, b);
    const { width, height } = this.host.size();
    const zoomed = zoomAround(
      pinch.origin,
      pinch.origin.scale * (distance(a, b) / Math.max(1, pinch.dist)),
      pinch.mid.x - rect.left,
      pinch.mid.y - rect.top,
      width,
      height,
    );
    this.host.transform.set(
      clampTransform(
        { ...zoomed, x: zoomed.x + (mid.x - pinch.mid.x), y: zoomed.y + (mid.y - pinch.mid.y) },
        width,
        height,
      ),
    );
  }

  wheel(event: WheelEvent): void {
    if (!this.host.interactive()) {
      return;
    }
    event.preventDefault();
    const rect = this.host.viewport().getBoundingClientRect();
    const t = this.host.transform();
    const next = t.scale * (event.deltaY < 0 ? WHEEL_STEP : 1 / WHEEL_STEP);
    this.host.transform.set(
      zoomAround(
        t,
        next,
        event.clientX - rect.left,
        event.clientY - rect.top,
        rect.width,
        rect.height,
      ),
    );
  }
}
