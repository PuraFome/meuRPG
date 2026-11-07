import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  afterNextRender,
  computed,
  effect,
  Injector,
  inject,
  input,
  output,
  signal,
} from '@angular/core';

import type { Square } from '../../../core/combat/combat-grid';
import { squareAt, stepSquare } from '../../../core/combat/combat-grid';
import { cellsPath, outlinePath, sameSquare } from '../../../core/combat/spell-area';
import {
  type PaintTool,
  type SunChange,
  cursorNote,
  lineSquares,
  strokeChange,
  withStroke,
} from '../../../core/combat/sunlight-paint';

let nextId = 0;

/** The crosses of the sunlit squares: one tile of 0.2 x 0.2 squares, drawn as a stroked path (the legend's swatch uses the same). */
export const SUN_TILE = 0.2;
export const SUN_CROSS = 'M0.05 0.05L0.15 0.15M0.15 0.05L0.05 0.15';

/**
 * The sun on the master's map (#704): the squares the master painted, drawn as crosses with a dotted edge (never the line patterns
 * of the spell zones), or, when the sun is over the whole map, a dotted frame around it. While `painting` it takes the pointer
 * over the map: a click or a drag adds (`paint`) or removes (`erase`) squares, the stroke is sent once, when the pointer is
 * released; the arrow keys move a cursor and Space applies the tool to its square, announced in a live region. It draws what the
 * server sent (`squares`) and previews only the stroke under the pointer.
 */
@Component({
  selector: 'app-sunlight-layer',
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './sunlight-layer.html',
  styleUrl: './sunlight-layer.scss',
})
export class SunlightLayer {
  readonly columns = input.required<number>();
  readonly rows = input.required<number>();
  /** The squares the server says are in the sun. */
  readonly squares = input<readonly Square[]>([]);
  /** The sun is over the whole map: no squares, a frame. */
  readonly wholeMap = input(false);
  readonly painting = input(false);
  readonly tool = input<PaintTool>('paint');

  /** What a stroke or a key changed. */
  readonly paint = output<SunChange>();

  protected readonly uid = `sl-${nextId++}`;
  protected readonly tile = SUN_TILE;
  protected readonly cross = SUN_CROSS;
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private stroke: { pointer: number; last: Square; touched: Square[] } | null = null;
  private readonly touched = signal<readonly Square[]>([]);
  protected readonly cursor = signal<Square | null>(null);
  private readonly injector = inject(Injector);
  /** What the live region says: the cursor's square, and whether it is in the sun (after the server's answer). */
  protected readonly note = computed(() => {
    const at = this.cursor();
    return this.painting() && at ? cursorNote(at, this.lit(at)) : '';
  });

  /** The squares as drawn: the server's, with the stroke under the pointer previewed. */
  protected readonly shown = computed(() =>
    this.touched().length
      ? withStroke(this.squares(), this.tool(), this.touched())
      : this.squares(),
  );
  protected readonly fill = computed(() => cellsPath(this.shown()));
  protected readonly edge = computed(() => outlinePath(this.shown()));

  constructor() {
    effect(() => {
      if (!this.painting()) {
        this.stroke = null;
        this.touched.set([]);
        this.cursor.set(null);
        return;
      }
      afterNextRender(
        () => this.host.nativeElement.querySelector<HTMLElement>('.sl__pad')?.focus(),
        { injector: this.injector },
      );
    });
  }

  private under(event: PointerEvent | MouseEvent): Square {
    const rect = this.host.nativeElement.getBoundingClientRect();
    return squareAt(
      (event.clientX - rect.left) / Math.max(1, rect.width),
      (event.clientY - rect.top) / Math.max(1, rect.height),
      this.columns(),
      this.rows(),
    );
  }

  protected down(event: PointerEvent): void {
    event.stopPropagation();
    if (event.button > 0) {
      return;
    }
    const at = this.under(event);
    this.stroke = { pointer: event.pointerId, last: at, touched: [at] };
    this.touched.set([at]);
    (event.currentTarget as HTMLElement).setPointerCapture?.(event.pointerId);
    this.cursor.set(at);
  }

  protected move(event: PointerEvent): void {
    event.stopPropagation();
    const s = this.stroke;
    if (!s || s.pointer !== event.pointerId) {
      return;
    }
    const at = this.under(event);
    if (sameSquare(at, s.last)) {
      return;
    }
    s.touched.push(...lineSquares(s.last, at).slice(1));
    s.last = at;
    this.touched.set([...s.touched]);
  }

  protected up(event: PointerEvent): void {
    event.stopPropagation();
    const s = this.stroke;
    if (!s || s.pointer !== event.pointerId) {
      return;
    }
    this.stroke = null;
    this.touched.set([]);
    if (event.type === 'pointerup') {
      this.send(s.touched);
    }
  }

  protected key(event: KeyboardEvent): void {
    event.stopPropagation();
    const cols = this.columns();
    const rows = this.rows();
    if (event.key.startsWith('Arrow')) {
      event.preventDefault();
      const next = stepSquare(this.cursor() ?? { col: 0, row: 0 }, event.key, cols, rows);
      this.cursor.set(next);
    } else if (event.key === ' ' || event.key === 'Spacebar') {
      event.preventDefault();
      const at = this.cursor() ?? { col: 0, row: 0 };
      this.cursor.set(at);
      this.send([at]);
    }
  }

  protected focused(): void {
    this.cursor.update((at) => at ?? { col: 0, row: 0 });
  }

  private lit(at: Square): boolean {
    return this.squares().some((s) => sameSquare(s, at));
  }

  private send(touched: readonly Square[]): void {
    // The server's squares may be older than a stroke still in the air (a square painted and erased within one round trip), so
    // the stroke is sent whole, each square once; the server ignores what is already so.
    const change = strokeChange(this.tool() === 'erase' ? touched : [], this.tool(), touched);
    if (change.addSquares.length + change.removeSquares.length > 0) {
      this.paint.emit(change);
    }
  }
}
