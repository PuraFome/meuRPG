import { DOCUMENT, DestroyRef, Signal, inject, signal } from '@angular/core';

/**
 * A CSS media query as a signal (the map pages choose a layout in code:
 * the editor on a computer, the lists on a phone). Call it in an injection
 * context; the listener goes away with the component.
 */
export function mediaQuery(query: string): Signal<boolean> {
  const view = inject(DOCUMENT).defaultView;
  const list = view?.matchMedia?.(query) ?? null;
  const matches = signal(list?.matches ?? false);
  if (list) {
    const listener = (event: MediaQueryListEvent) => matches.set(event.matches);
    list.addEventListener('change', listener);
    inject(DestroyRef).onDestroy(() => list.removeEventListener('change', listener));
  }
  return matches;
}

/** The phone layout: under 768px, as the rest of the app. */
export const PHONE_QUERY = '(max-width: 767.98px)';
