import { type DestroyRef, type Signal, signal } from '@angular/core';

/** Whether the browser tab is on screen, as a signal; the listener goes with `destroyRef`. */
export function pageVisible(document: Document, destroyRef: DestroyRef): Signal<boolean> {
  const visible = signal(document.visibilityState !== 'hidden');
  const update = () => visible.set(document.visibilityState !== 'hidden');
  document.addEventListener('visibilitychange', update);
  destroyRef.onDestroy(() => document.removeEventListener('visibilitychange', update));
  return visible;
}
