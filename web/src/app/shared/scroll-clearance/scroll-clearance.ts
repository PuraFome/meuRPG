import { effect } from '@angular/core';

/** The room kept between a sticky bar and what is scrolled or focused into view beside it, in px. */
const CLEARANCE_GAP = 16;

/** What each live bar asks, by side: the page's padding is the largest, so a bar that goes away never clears the room of the one that stays. */
const asked = { top: new Map<symbol, number>(), bottom: new Map<symbol, number>() };

function apply(side: 'top' | 'bottom'): void {
  const values = [...asked[side].values()];
  document.documentElement.style[side === 'top' ? 'scrollPaddingTop' : 'scrollPaddingBottom'] =
    values.length ? `${Math.max(...values)}px` : '';
}

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
    const id = Symbol('keepClear');
    const set = () => {
      const style = getComputedStyle(el);
      const offset = Number.parseFloat(side === 'top' ? style.top : style.bottom);
      if (style.position === 'sticky') {
        asked[side].set(id, el.offsetHeight + (Number.isNaN(offset) ? 0 : offset) + CLEARANCE_GAP);
      } else {
        asked[side].delete(id);
      }
      apply(side);
    };
    set();
    const observer = new ResizeObserver(set);
    observer.observe(el);
    onCleanup(() => {
      observer.disconnect();
      asked[side].delete(id);
      apply(side);
    });
  });
}
