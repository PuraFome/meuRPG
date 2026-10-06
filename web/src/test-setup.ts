import { afterEach, beforeEach, vi } from 'vitest';

/**
 * Runs before every spec file (the `setupFiles` of the `test` target in angular.json).
 *
 * The Angular unit-test builder runs Vitest without isolation between spec files (`isolate: false`), so a global that one
 * file stubs (`vi.stubGlobal('matchMedia', …)`) stays in place for the next file the same worker runs: a test that passes
 * alone fails after another one, depending on which files share a worker. Every test ends with the stubbed globals
 * restored, so no spec depends on the order the files run in.
 *
 * jsdom has no `Element.prototype.scrollIntoView`, which components call (a question brought into view, the field a
 * refusal points at). Every test starts with a fresh no-op mock of it: a spec never depends on another file having
 * defined it first, and a mock one test changes never reaches the next.
 */
beforeEach(() => {
  Element.prototype.scrollIntoView = vi.fn();
});

afterEach(() => {
  vi.unstubAllGlobals();
});
