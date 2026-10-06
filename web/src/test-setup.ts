import { afterEach, vi } from 'vitest';

/**
 * Runs before every spec file (the `setupFiles` of the `test` target in angular.json).
 *
 * The Angular unit-test builder runs Vitest without isolation between spec files (`isolate: false`), so a global that one
 * file stubs (`vi.stubGlobal('matchMedia', …)`) stays in place for the next file the same worker runs: a test that passes
 * alone fails after another one, depending on which files share a worker. Every test ends with the stubbed globals
 * restored, so no spec depends on the order the files run in.
 */
afterEach(() => {
  vi.unstubAllGlobals();
});
