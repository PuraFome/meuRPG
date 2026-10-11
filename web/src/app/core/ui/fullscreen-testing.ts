import { vi } from 'vitest';

/**
 * Stubs the document's Fullscreen API for a spec: `enabled` is `fullscreenEnabled`, `enter(el)` makes `el` the
 * full-screen element and fires `fullscreenchange`, `leave()` clears it. `request` is the spy every host's
 * `requestFullscreen` is replaced with (call `track(el)` on a host), `exit` the one for `exitFullscreen`.
 * `restore()` undoes the stubs.
 */
export function stubFullscreen(enabled: boolean) {
  let current: Element | null = null;
  Object.defineProperty(document, 'fullscreenEnabled', { configurable: true, value: enabled });
  Object.defineProperty(document, 'fullscreenElement', { configurable: true, get: () => current });
  const exit = vi.fn(() => Promise.resolve());
  Object.defineProperty(document, 'exitFullscreen', { configurable: true, value: exit });
  const request = vi.fn(() => Promise.resolve());
  const change = () => document.dispatchEvent(new Event('fullscreenchange'));
  return {
    request,
    exit,
    track: (el: HTMLElement) => {
      el.requestFullscreen = request;
    },
    enter: (el: Element) => {
      current = el;
      change();
    },
    leave: () => {
      current = null;
      change();
    },
    restore: () => {
      for (const key of ['fullscreenEnabled', 'fullscreenElement', 'exitFullscreen']) {
        Reflect.deleteProperty(document, key);
      }
    },
  };
}
