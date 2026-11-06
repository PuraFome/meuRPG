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
/** What WebKit throws when the server answers a lazy file with index.html: a TypeError with no code and no cause. */
const WEBKIT_HTML =
  "'text/html' is not a valid JavaScript MIME type for module script 'https://x.test/chunk-ABC.js'.";
const FIREFOX = 'error loading dynamically imported module: https://x.test/chunk-ABC.js';
const ROUTER_STATE = { navigationId: 3 };

/** What Angular 21.2 reports for a failed `@defer` without an `@error` block: a RuntimeError whose text embeds the message. */
const DEFER_ERROR = new Error(
  `NG0750: Loading dependencies for \`@defer\` block failed.\n\nThe loading resulted in the following error:\n\n${CHROME}`,
);

/** What a production build reports for a failed `@defer`: only the code. */
const DEFER_PROD_ERROR = Object.assign(new Error('NG0750'), { code: -750 });

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

  it('recognises a failed @defer block by its code when a production build keeps no message', () => {
    expect(isChunkLoadError(DEFER_PROD_ERROR)).toBe(true);
    expect(isChunkLoadError(Object.assign(new Error('NG0200'), { code: -200 }))).toBe(false);
  });

  it('recognises the WebKit error for a lazy file that the server answered with HTML', () => {
    expect(isChunkLoadError(new TypeError(WEBKIT_HTML))).toBe(true);
    expect(isChunkLoadError(new TypeError(WEBKIT_HTML.replace('text/html', 'text/plain')))).toBe(
      true,
    );
  });

  it('rejects the MIME and module errors of a real bug in a module', () => {
    expect(isChunkLoadError(new SyntaxError("Unexpected token '<'"))).toBe(false);
    expect(isChunkLoadError(new TypeError('Cannot read properties of undefined'))).toBe(false);
    expect(isChunkLoadError(new TypeError("'text/html' is not a valid value"))).toBe(false);
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

  afterEach(() => {
    vi.restoreAllMocks();
    history.replaceState(null, '');
  });

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

  it('reloads once for the WebKit error of a lazy file answered with HTML', () => {
    const handler = TestBed.inject(ErrorHandler);
    handler.handleError(new TypeError(WEBKIT_HTML));
    expect(reload).toHaveBeenCalledTimes(1);
    handler.handleError(new TypeError(WEBKIT_HTML));
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it('reloads for a failed @defer block error', () => {
    TestBed.inject(ErrorHandler).handleError(DEFER_ERROR);
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it('reloads for a production @defer error that carries only its code', () => {
    TestBed.inject(ErrorHandler).handleError(DEFER_PROD_ERROR);
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it('neither reloads nor logs a chunk error that a navigation cancelled', () => {
    TestBed.inject(ChunkReload);
    const log = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    window.dispatchEvent(new Event('beforeunload'));
    TestBed.inject(ErrorHandler).handleError(new TypeError(WEBKIT));
    TestBed.inject(ErrorHandler).handleError(DEFER_PROD_ERROR);
    expect(reload).not.toHaveBeenCalled();
    expect(history.state).toEqual(ROUTER_STATE);
    expect(log).not.toHaveBeenCalled();
  });

  it('still logs another error while the page is leaving', () => {
    TestBed.inject(ChunkReload);
    const log = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    window.dispatchEvent(new Event('beforeunload'));
    TestBed.inject(ErrorHandler).handleError(new Error('boom'));
    expect(log).toHaveBeenCalled();
  });

  it('reloads again once the leaving window is over, as when the user stayed on the page', () => {
    vi.useFakeTimers();
    try {
      TestBed.inject(ChunkReload);
      window.dispatchEvent(new Event('beforeunload'));
      vi.advanceTimersByTime(2001);
      TestBed.inject(ErrorHandler).handleError(new TypeError(CHROME));
      expect(reload).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
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
