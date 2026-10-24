import { DOCUMENT } from '@angular/common';
import { DestroyRef, type Signal, effect, inject } from '@angular/core';

/** The room the page keeps under the sticky bars: the app bar and the combat bar (64 + 88 + 20); on a phone the fixed bar and the nav. */
const STICKY_TOP = '172px';
const STICKY_BOTTOM = '152px';

/**
 * The sticky bars (the app bar and the combat bar from 1024px, the fixed bar and the nav on a phone) never hide the focus: while the
 * master's combat is on screen the page keeps this much room at the edge it scrolls under (WCAG 2.4.11). Call it in a constructor.
 */
export function keepFocusClear(onScreen: Signal<boolean>, laptop: Signal<boolean>): void {
  const root = inject(DOCUMENT).documentElement;
  effect(() => {
    root.style.scrollPaddingTop = onScreen() && laptop() ? STICKY_TOP : '';
    root.style.scrollPaddingBottom = onScreen() && !laptop() ? STICKY_BOTTOM : '';
  });
  inject(DestroyRef).onDestroy(() => {
    root.style.scrollPaddingTop = '';
    root.style.scrollPaddingBottom = '';
  });
}
