import { ErrorHandler } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';

import {
  CHUNK_RELOAD_MARKER,
  ChunkReloadErrorHandler,
  PAGE_RELOAD,
  isChunkLoadError,
  ChunkReload,
} from './chunk-reload';

const CHROME = 'Failed to fetch dynamically imported module: https://x.test/chunk-ABC.js';
const WEBKIT = 'Importing a module script failed.';
const FIREFOX = 'error loading dynamically imported module: https://x.test/chunk-ABC.js';
const ROUTER_STATE = { navigationId: 3 };

/** What Angular 21.2 reports for a failed `@defer` without an `@error` block: a RuntimeError whose text embeds the message. */
const DEFER_ERROR = new Error(
  `NG0750: Loading dependencies for \`@defer\` block failed.\n\nThe loading resulted in the following error:\n\n${CHROME}`,
);

describe('isChunkLoadError', () => {
  it.each([CHROME, WEBKIT, FIREFOX])('recognises %s', (message) => {
    expect(isChunkLoadError(new TypeError(message))).toBe(true);
  });

  it('recognises a ChunkLoadError by name, a string and a cause chain', () => {
    const named = new Error('x');
    named.name = 'ChunkLoadError';
    expect(isChunkLoadError(named)).toBe(true);
    expect(isChunkLoadError(WEBKIT)).toBe(true);
    expect(
      isChunkLoadError(new Error('wrapper', { cause: new Error('mid', { cause: named }) })),
    ).toBe(true);
    expect(isChunkLoadError(DEFER_ERROR)).toBe(true);
  });

  it('rejects other errors and values', () => {
    expect(isChunkLoadError(new Error('boom'))).toBe(false);
    expect(isChunkLoadError(null)).toBe(false);
    expect(isChunkLoadError(undefined)).toBe(false);
    expect(isChunkLoadError(42)).toBe(false);
  });

  it('stops on a cyclic cause chain', () => {
    const loop = new Error('loop');
    (loop as { cause?: unknown }).cause = loop;
    expect(isChunkLoadError(loop)).toBe(false);
  });
});

describe('chunk reload', () => {
  let reload: ReturnType<typeof vi.fn>;
  let history: History;

  beforeEach(() => {
    reload = vi.fn();
    history = window.history;
    history.replaceState(ROUTER_STATE, '');
    TestBed.configureTestingModule({
      providers: [
        { provide: PAGE_RELOAD, useValue: reload },
        { provide: ErrorHandler, useClass: ChunkReloadErrorHandler },
        provideRouter([
          { path: 'lazy', loadComponent: () => Promise.reject(new TypeError(CHROME)) },
        ]),
      ],
    });
  });

  afterEach(() => history.replaceState(null, ''));

  it('reloads once for a failed lazy chunk and keeps the router state keys', () => {
    const handler = TestBed.inject(ErrorHandler);
    handler.handleError(new TypeError(CHROME));
    expect(reload).toHaveBeenCalledTimes(1);
    expect(history.state).toEqual({ ...ROUTER_STATE, [CHUNK_RELOAD_MARKER]: true });
  });

  it('does not reload a second time while the marker is present', () => {
    const handler = TestBed.inject(ErrorHandler);
    const log = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    handler.handleError(new TypeError(CHROME));
    handler.handleError(new TypeError(WEBKIT));
    expect(reload).toHaveBeenCalledTimes(1);
    expect(log).toHaveBeenCalledTimes(1);
  });

  it('reloads again on a new history entry, which has no marker', () => {
    const handler = TestBed.inject(ErrorHandler);
    handler.handleError(new TypeError(CHROME));
    history.replaceState(null, '');
    handler.handleError(new TypeError(CHROME));
    expect(reload).toHaveBeenCalledTimes(2);
  });

  it('reloads for a failed @defer block error', () => {
    TestBed.inject(ErrorHandler).handleError(DEFER_ERROR);
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it('never reloads for a non-chunk error, and logs it', () => {
    const log = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    TestBed.inject(ErrorHandler).handleError(new Error('boom'));
    expect(reload).not.toHaveBeenCalled();
    expect(history.state).toEqual(ROUTER_STATE);
    expect(log).toHaveBeenCalled();
  });

  it('does not reload on a page that started on a marked entry, even after the router rewrote the state', () => {
    history.replaceState({ ...ROUTER_STATE, [CHUNK_RELOAD_MARKER]: true }, '');
    TestBed.inject(ChunkReload);
    // The router replaces history.state after each navigation, without the marker.
    history.replaceState(ROUTER_STATE, '');
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    TestBed.inject(ErrorHandler).handleError(new TypeError(CHROME));
    expect(reload).not.toHaveBeenCalled();
  });

  it('reloads for the rejection of a lazy route that fails to load', async () => {
    const rejection = await TestBed.inject(Router)
      .navigateByUrl('/lazy')
      .catch((err: unknown) => err);
    // The rejection reaches the ErrorHandler through the global listeners; the real history proof is in e2e.
    expect(isChunkLoadError(rejection)).toBe(true);
    TestBed.inject(ErrorHandler).handleError(rejection);
    expect(reload).toHaveBeenCalledTimes(1);
  });
});
