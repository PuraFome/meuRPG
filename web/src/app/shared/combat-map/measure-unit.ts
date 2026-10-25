import { type Signal, effect, signal } from '@angular/core';

/**
 * The width of one square of the map in px, followed with a `ResizeObserver` (0 until the map is drawn, and where there is none):
 * the labels of the target choice are laid out in px. Call it in a field initialiser (it needs an injection context).
 */
export function watchUnit(
  surface: () => HTMLElement | undefined,
  columns: () => number,
): Signal<number> {
  const unit = signal(0);
  effect((onCleanup) => {
    const el = surface();
    const cols = columns();
    if (!el || cols <= 0 || typeof ResizeObserver === 'undefined') {
      return;
    }
    const read = () => unit.set(el.getBoundingClientRect().width / cols);
    read();
    const observer = new ResizeObserver(read);
    observer.observe(el);
    onCleanup(() => observer.disconnect());
  });
  return unit.asReadonly();
}

/** The path of the grid's lines, in squares (a 1 px SVG over the map). */
export function gridPath(columns: number, rows: number): string {
  const parts: string[] = [];
  for (let c = 0; c <= columns; c++) {
    parts.push(`M${c} 0V${rows}`);
  }
  for (let r = 0; r <= rows; r++) {
    parts.push(`M0 ${r}H${columns}`);
  }
  return parts.join('');
}
