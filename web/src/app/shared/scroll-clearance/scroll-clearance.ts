import { effect } from '@angular/core';

/** The room kept between a sticky bar and what is scrolled or focused into view beside it, in px. */
const CLEARANCE_GAP = 16;

/**
 * Keeps a sticky bar from covering what is scrolled or focused into view (WCAG 2.2, 2.4.11): while the bar sticks to the `side`
 * of the screen, the page's `scroll-padding` on that side is the bar's height, its offset and a gap; when the bar does not stick
 * (a layout where it flows with the page) or is gone, the padding is cleared. Call it in a constructor (it needs an injection
 * context); the bar's size is followed with a `ResizeObserver`.
 */
export function keepClear(bar: () => HTMLElement | undefined, side: 'top' | 'bottom'): void {
  effect((onCleanup) => {
    const el = bar();
    if (!el || typeof ResizeObserver === 'undefined') {
      return;
    }
    const root = document.documentElement;
    const property = side === 'top' ? 'scrollPaddingTop' : 'scrollPaddingBottom';
    const set = () => {
      const style = getComputedStyle(el);
      const offset = Number.parseFloat(side === 'top' ? style.top : style.bottom);
      root.style[property] =
        style.position === 'sticky'
          ? `${el.offsetHeight + (Number.isNaN(offset) ? 0 : offset) + CLEARANCE_GAP}px`
          : '';
    };
    set();
    const observer = new ResizeObserver(set);
    observer.observe(el);
    onCleanup(() => {
      observer.disconnect();
      root.style[property] = '';
    });
  });
}
