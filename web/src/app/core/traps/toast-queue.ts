import { signal } from '@angular/core';

/** One toast of the session page: "Você notou uma armadilha.", "Brisa encontrou o Baú de moedas." */
export interface Toast {
  readonly id: number;
  readonly icon: string;
  /** The bold first phrase. */
  readonly title: string;
  /** What follows it. */
  readonly text: string;
}

/** A toast goes away by itself after 8 seconds (E9-08 G); the ✕ takes it away sooner. */
export const TOAST_MS = 8000;

/**
 * The toasts on a player's session page, newest last. A toast stays on screen for `TOAST_MS`, then goes by
 * itself; `dismiss` takes it away at once. Plain TypeScript with signals and timers, tested with fake timers;
 * `clear` stops every timer (the page is leaving).
 */
export class ToastQueue {
  readonly toasts = signal<readonly Toast[]>([]);
  private next = 1;
  private readonly timers = new Map<number, ReturnType<typeof setTimeout>>();

  push(icon: string, title: string, text = ''): void {
    const id = this.next++;
    this.toasts.update((list) => [...list, { id, icon, title, text }]);
    this.timers.set(id, setTimeout(() => this.dismiss(id), TOAST_MS));
  }

  dismiss(id: number): void {
    const timer = this.timers.get(id);
    if (timer !== undefined) {
      clearTimeout(timer);
      this.timers.delete(id);
    }
    this.toasts.update((list) => list.filter((t) => t.id !== id));
  }

  clear(): void {
    for (const timer of this.timers.values()) {
      clearTimeout(timer);
    }
    this.timers.clear();
    this.toasts.set([]);
  }
}
