import { DOCUMENT } from '@angular/common';
import { type ElementRef, DestroyRef, type Signal, effect, inject } from '@angular/core';

/** The air between a sticky bar and what sticks under it, and the room the focus keeps beyond it. */
const GAP = 12;
const FOCUS_ROOM = 8;
/** Where the columns stick before the bar is measured (the app bar and a bar of 88 px). */
const FALLBACK_TOP = 164;
const FALLBACK_BOTTOM = 152;

/**
 * The master's combat sticks its columns under the combat bar, and a phone keeps its fixed bar over the nav. Both are measured, not
 * assumed: `--combat-top` (the bar's bottom edge once it sticks, plus a gap) is set on the screen for the order, the map and the rail to
 * stick at, and the page keeps that much room (plus a little) at the edge it scrolls under, so the focus never hides behind a bar
 * (WCAG 2.4.11). `tick` is read to measure again when the bar changes (the initiative becoming a running combat). Call it in a constructor.
 */
export function keepFocusClear(
  host: ElementRef<HTMLElement>,
  onScreen: Signal<boolean>,
  laptop: Signal<boolean>,
  tick: Signal<unknown>,
): void {
  const root = inject(DOCUMENT).documentElement;
  const clear = () => {
    root.style.scrollPaddingTop = '';
    root.style.scrollPaddingBottom = '';
    host.nativeElement.style.removeProperty('--combat-top');
  };
  const measure = () => {
    const el = host.nativeElement;
    if (!onScreen()) {
      clear();
      return;
    }
    if (laptop()) {
      const bar = el.querySelector<HTMLElement>('app-combat-bar');
      const top = bar
        ? parseFloat(getComputedStyle(bar).top) + bar.offsetHeight
        : FALLBACK_TOP - GAP;
      el.style.setProperty('--combat-top', `${Math.round(top + GAP)}px`);
      root.style.scrollPaddingTop = `${Math.round(top + GAP + FOCUS_ROOM)}px`;
      root.style.scrollPaddingBottom = '';
    } else {
      const bar = el.querySelector<HTMLElement>('.phone__bar');
      const room = bar ? window.innerHeight - bar.getBoundingClientRect().top : FALLBACK_BOTTOM;
      root.style.scrollPaddingTop = '';
      root.style.scrollPaddingBottom = `${Math.round(room + FOCUS_ROOM)}px`;
    }
  };
  const observer =
    typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(() => measure());
  effect(() => {
    onScreen();
    laptop();
    tick();
    observer?.disconnect();
    queueMicrotask(() => {
      measure();
      const el = host.nativeElement;
      for (const part of el.querySelectorAll('app-combat-bar, .phone__bar')) {
        observer?.observe(part);
      }
    });
  });
  inject(DestroyRef).onDestroy(() => {
    observer?.disconnect();
    clear();
  });
}
