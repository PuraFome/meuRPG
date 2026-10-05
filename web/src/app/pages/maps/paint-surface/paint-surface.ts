import { ChangeDetectionStrategy, Component, ElementRef, inject, input, output, signal } from '@angular/core';

import { type Square, stepSquare } from '../../../core/combat/combat-grid';
import { lineSquares, squareUnder } from '../../../core/maps/paint-tools';

/** One piece of a stroke: the squares the pointer (or the keyboard) passed over, and whether it erases. */
export interface Stroke {
  readonly centers: readonly Square[];
  readonly erase: boolean;
}

/**
 * The surface that catches the master's painting (E9-01 1, MR-034): as big as the map's picture (it sits in the
 * map view's stage, so it pans and zooms with it), transparent, above the markers. A drag paints square by
 * square, a click one square, `Shift` erases; no square is skipped between two pointer events (the stroke is
 * the line between them). Keyboard: the arrows move the brush cursor, `Espaço` paints, `Esc` leaves. `Alt` and
 * a drag pass through to the map view, which pans it. It only reports where; the editor paints, saves and says
 * "Tudo salvo".
 */
@Component({
  selector: 'app-paint-surface',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: '',
  styles: `
    :host {
      position: absolute;
      inset: 0;
      z-index: 10;
      display: block;
      cursor: crosshair;
      touch-action: none;
    }

    :host(:focus-visible) {
      outline: 3px solid var(--mr-focus);
      outline-offset: -3px;
    }
  `,
  host: {
    role: 'application',
    tabindex: '0',
    '[attr.aria-label]': 'label()',
    '(pointerdown)': 'onDown($event)',
    '(pointermove)': 'onMove($event)',
    '(pointerup)': 'onUp($event)',
    '(pointercancel)': 'onUp($event)',
    '(pointerleave)': 'onLeave()',
    '(click)': '$event.stopPropagation()',
    '(keydown)': 'onKey($event)',
    '(keyup)': 'onKeyUp($event)',
    '(focus)': 'onFocus()',
  },
})
export class PaintSurface {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly columns = input.required<number>();
  readonly rows = input.required<number>();
  /** The square the brush cursor is on (the editor draws it), `null` while there is none. */
  readonly cursor = input<Square | null>(null);
  /** The accessible name: the tool and the keys. */
  readonly label = input('Área de pintura do mapa. Use as setas para mover o pincel, Espaço para pintar e Esc para sair.');

  readonly stroke = output<Stroke>();
  readonly strokeEnd = output<void>();
  readonly hover = output<Square | null>();
  /** `Esc`: the editor takes the focus back to the tool bar. */
  readonly leave = output<void>();

  private readonly active = signal<number | null>(null);
  private last: Square | null = null;

  private squareAt(event: PointerEvent): Square {
    const rect = this.host.nativeElement.getBoundingClientRect();
    return squareUnder(
      (event.clientX - rect.left) / Math.max(1, rect.width),
      (event.clientY - rect.top) / Math.max(1, rect.height),
      this.columns(),
      this.rows(),
    );
  }

  protected onDown(event: PointerEvent): void {
    // Alt, or another button: the map view pans.
    if (event.button !== 0 || event.altKey) {
      return;
    }
    event.stopPropagation();
    this.host.nativeElement.focus({ preventScroll: true });
    this.host.nativeElement.setPointerCapture?.(event.pointerId);
    this.active.set(event.pointerId);
    const at = this.squareAt(event);
    this.last = at;
    this.hover.emit(at);
    this.stroke.emit({ centers: [at], erase: event.shiftKey });
  }

  protected onMove(event: PointerEvent): void {
    const at = this.squareAt(event);
    this.hover.emit(at);
    if (this.active() !== event.pointerId || !this.last) {
      return;
    }
    if (at.col === this.last.col && at.row === this.last.row) {
      return;
    }
    const line = lineSquares(this.last, at).slice(1);
    this.last = at;
    this.stroke.emit({ centers: line, erase: event.shiftKey });
  }

  protected onUp(event: PointerEvent): void {
    if (this.active() !== event.pointerId) {
      return;
    }
    event.stopPropagation();
    this.active.set(null);
    this.last = null;
    this.strokeEnd.emit();
  }

  protected onLeave(): void {
    if (this.active() === null) {
      this.hover.emit(null);
    }
  }

  /** The first focus puts the cursor in the middle of the map, where the arrows start from. */
  protected onFocus(): void {
    if (!this.cursor()) {
      this.hover.emit({ col: Math.floor(this.columns() / 2), row: Math.floor(this.rows() / 2) });
    }
  }

  protected onKeyUp(event: KeyboardEvent): void {
    if (event.key === ' ' || event.code === 'Space') {
      this.strokeEnd.emit();
    }
  }

  protected onKey(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      event.preventDefault();
      this.hover.emit(null);
      this.leave.emit();
      return;
    }
    const from = this.cursor() ?? { col: Math.floor(this.columns() / 2), row: Math.floor(this.rows() / 2) };
    if (event.key.startsWith('Arrow')) {
      event.preventDefault();
      const to = stepSquare(from, event.key, this.columns(), this.rows());
      this.hover.emit(to);
      return;
    }
    if (event.key === ' ' || event.code === 'Space') {
      event.preventDefault();
      this.stroke.emit({ centers: [from], erase: event.shiftKey });
    }
  }
}
