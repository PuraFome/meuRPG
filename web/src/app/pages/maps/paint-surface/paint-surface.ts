import { ChangeDetectionStrategy, Component, DestroyRef, ElementRef, inject, input, output, signal } from '@angular/core';

import { type Square, squareAt, stepSquare } from '../../../core/combat/combat-grid';
import { lineSquares } from '../../../core/maps/paint-tools';

/** One piece of a stroke: the squares the pointer (or the keyboard) passed over, and whether it erases. */
export interface Stroke {
  readonly centers: readonly Square[];
  readonly erase: boolean;
}

/**
 * The surface that catches the master's painting (E9-01 1, MR-034): as big as the map's picture (it sits in the
 * map view's stage, so it pans and zooms with it), transparent, above the markers. A drag paints square by
 * square, a click one square, `Shift` erases; no square is skipped between two pointer events (the stroke is
 * the line between them). On a touch screen one finger paints and two fingers pan and zoom the map (the surface lets the touches through to
 * the map view, and stops painting while a second finger is down; a finger waits 90 ms before its first square, so the first of two
 * fingers does not leave a mark). Keyboard: the arrows move the brush cursor, `Espaço` paints, `Esc` leaves. `Alt` and
 * a drag pass through to the map view, which pans it. It only reports where; the editor paints, saves and says
 * "Tudo salvo".
 */
@Component({
  selector: 'app-paint-surface',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: '<span class="mr-visually-hidden" aria-live="polite">{{ spoken() }}</span>',
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
  /** What a screen reader hears when the arrows move the brush: the column and the row, counted from 1. */
  protected readonly spoken = signal('');
  private last: Square | null = null;
  private lastHover: Square | null = null;
  /** The fingers on the glass; with two, the map pans and zooms and nothing is painted. */
  private readonly touches = new Set<number>();
  private multi = false;
  /** The first square of a finger, held back a moment in case a second finger comes. */
  private held: { at: Square; erase: boolean } | null = null;
  private holdTimer: ReturnType<typeof setTimeout> | undefined;

  constructor() {
    inject(DestroyRef).onDestroy(() => clearTimeout(this.holdTimer));
  }

  private releaseHeld(): void {
    clearTimeout(this.holdTimer);
    const held = this.held;
    this.held = null;
    if (held && !this.multi) {
      this.stroke.emit({ centers: [held.at], erase: held.erase });
    }
  }

  /** The square under the pointer, or `null` outside the map: a drag that leaves it must not paint its edge. */
  private squareAt(event: PointerEvent): Square | null {
    const rect = this.host.nativeElement.getBoundingClientRect();
    const x = (event.clientX - rect.left) / Math.max(1, rect.width);
    const y = (event.clientY - rect.top) / Math.max(1, rect.height);
    return x < 0 || y < 0 || x >= 1 || y >= 1 ? null : squareAt(x, y, this.columns(), this.rows());
  }

  protected onDown(event: PointerEvent): void {
    // Alt, or another button: the map view pans.
    if (event.button !== 0 || event.altKey) {
      return;
    }
    if (event.pointerType === 'touch') {
      this.touches.add(event.pointerId);
      if (this.touches.size > 1) {
        // A second finger: the map view pans and zooms (it sees both touches); what the first began is let go.
        this.multi = true;
        clearTimeout(this.holdTimer);
        this.held = null;
        this.active.set(null);
        this.last = null;
        return;
      }
    } else {
      event.stopPropagation();
    }
    this.host.nativeElement.focus({ preventScroll: true });
    this.host.nativeElement.setPointerCapture?.(event.pointerId);
    this.active.set(event.pointerId);
    const at = this.squareAt(event);
    if (!at) {
      return;
    }
    this.last = at;
    this.emitHover(at);
    if (event.pointerType === 'touch') {
      this.held = { at, erase: event.shiftKey };
      this.holdTimer = setTimeout(() => this.releaseHeld(), 90);
      return;
    }
    this.stroke.emit({ centers: [at], erase: event.shiftKey });
  }

  protected onMove(event: PointerEvent): void {
    const at = this.squareAt(event);
    this.emitHover(at);
    if (this.multi || !at || this.active() !== event.pointerId || !this.last) {
      return;
    }
    if (at.col === this.last.col && at.row === this.last.row) {
      return;
    }
    this.releaseHeld();
    const line = lineSquares(this.last, at).slice(1);
    this.last = at;
    this.stroke.emit({ centers: line, erase: event.shiftKey });
  }

  /** Tells the editor where the brush is only when it changed square: a `pointermove` inside one square is not news. */
  private emitHover(at: Square | null): void {
    const was = this.lastHover;
    if (at === null ? was === null : was !== null && was.col === at.col && was.row === at.row) {
      return;
    }
    this.lastHover = at;
    this.hover.emit(at);
  }

  protected onUp(event: PointerEvent): void {
    if (event.pointerType === 'touch') {
      this.touches.delete(event.pointerId);
      if (this.touches.size === 0) {
        this.multi = false;
      }
    }
    if (this.active() !== event.pointerId) {
      return;
    }
    if (event.pointerType !== 'touch') {
      event.stopPropagation();
    }
    this.releaseHeld();
    this.active.set(null);
    this.last = null;
    this.strokeEnd.emit();
  }

  protected onLeave(): void {
    if (this.active() === null) {
      this.emitHover(null);
    }
  }

  /** The first focus puts the cursor in the middle of the map, where the arrows start from. */
  protected onFocus(): void {
    if (!this.cursor()) {
      this.emitHover({ col: Math.floor(this.columns() / 2), row: Math.floor(this.rows() / 2) });
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
      this.emitHover(null);
      this.leave.emit();
      return;
    }
    // The square the brush is on now: its own record first (the input follows a render later, and two quick key presses must not both read the old one).
    const from = this.lastHover ?? this.cursor() ?? { col: Math.floor(this.columns() / 2), row: Math.floor(this.rows() / 2) };
    if (event.key.startsWith('Arrow')) {
      event.preventDefault();
      const to = stepSquare(from, event.key, this.columns(), this.rows());
      this.emitHover(to);
      this.spoken.set(`Coluna ${to.col + 1}, linha ${to.row + 1}`);
      return;
    }
    if (event.key === ' ' || event.code === 'Space') {
      event.preventDefault();
      this.stroke.emit({ centers: [from], erase: event.shiftKey });
    }
  }
}
