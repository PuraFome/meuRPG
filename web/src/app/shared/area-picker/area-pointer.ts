import { type WritableSignal, signal } from '@angular/core';

import type { AreaPick } from '../../core/combat/area-flow';
import { samePick } from '../../core/combat/area-flow';
import { fingerAt } from './area-moves';

/** How far above the finger the template follows a touch, so the finger does not cover what is chosen (PM-02a). */
export const FINGER_LIFT_PX = 44;

/** What the pointer of the area map needs from the map. */
export interface AreaPointerHost {
  readOnly(): boolean;
  /** The pick the square under the pointer makes, `lift` pixels above it. */
  pickUnder(event: PointerEvent, lift: number): AreaPick | null;
  readonly hover: WritableSignal<AreaPick | null>;
  setHover(pick: AreaPick | null, byKey: boolean): void;
  tap(pick: AreaPick | null): void;
}

function touchLike(event: PointerEvent): boolean {
  return event.pointerType === 'touch' || event.pointerType === 'pen';
}

/**
 * The mouse and the finger on the area map: the mouse moves the template and a release places it; a finger drags it 44 px above
 * itself, with a dotted line to it, and lifting places it.
 */
export class AreaPointer {
  /** Where the finger is, in percent of the map, while it drags. */
  readonly finger = signal<{ x: number; y: number } | null>(null);
  private touching: number | null = null;

  constructor(private readonly host: AreaPointerHost) {}

  down(event: PointerEvent): void {
    if (this.host.readOnly() || !touchLike(event)) {
      return;
    }
    this.touching = event.pointerId;
    (event.currentTarget as HTMLElement).setPointerCapture?.(event.pointerId);
    this.follow(event);
  }

  move(event: PointerEvent): void {
    if (this.host.readOnly()) {
      return;
    }
    if (touchLike(event)) {
      if (this.touching === event.pointerId) {
        this.follow(event);
      }
      return;
    }
    const pick = this.host.pickUnder(event, 0);
    if (!samePick(pick, this.host.hover())) {
      this.host.setHover(pick, false);
    }
  }

  up(event: PointerEvent): void {
    if (this.host.readOnly()) {
      return;
    }
    if (touchLike(event)) {
      if (this.touching !== event.pointerId) {
        return;
      }
      this.touching = null;
      this.finger.set(null);
      this.host.tap(this.host.pickUnder(event, FINGER_LIFT_PX));
      return;
    }
    if (event.button <= 0) {
      this.host.tap(this.host.pickUnder(event, 0));
    }
  }

  leave(event: PointerEvent): void {
    if (!touchLike(event)) {
      this.host.hover.set(null);
    }
  }

  cancel(): void {
    this.touching = null;
    this.finger.set(null);
    this.host.hover.set(null);
  }

  private follow(event: PointerEvent): void {
    this.host.setHover(this.host.pickUnder(event, FINGER_LIFT_PX), false);
    this.finger.set(fingerAt(event));
  }
}
