import { DOCUMENT } from '@angular/common';
import { ErrorHandler, Injectable, InjectionToken, inject } from '@angular/core';

/** The `history.state` key that marks an entry whose page was already reloaded for a chunk load error. */
export const CHUNK_RELOAD_MARKER = 'chunkReloaded';

const CHUNK_ERROR_NAME = 'ChunkLoadError';
// Angular's `RuntimeError` code for a `@defer` block whose dependencies failed to load. A production build keeps only
// the code in the text ("NG0750"): the failed import is not in it, so the code is the only sign.
const DEFER_LOADING_FAILED_CODE = -750;
// Chromium and WebKit reject a pending `import()` right after `beforeunload` when a navigation cancels it. That
// rejection is not a stale deploy, so the page does not reload for it. The flag lasts this long, in case the page stays.
const LEAVING_MS = 2000;
const MAX_CAUSE_DEPTH = 8;

// Chrome and Edge, Safari and WebKit, Firefox: what each engine says when `import()` cannot load a module.
const CHUNK_ERROR_MESSAGES = [
  'failed to fetch dynamically imported module',
  'importing a module script failed',
  'error loading dynamically imported module',
];

/** Whether `err`, or an error in its `cause` chain, is a failed load of a lazy module. */
export function isChunkLoadError(err: unknown): boolean {
  let current: unknown = err;
  for (let depth = 0; depth < MAX_CAUSE_DEPTH && current != null; depth++) {
    if (matchesChunkError(current)) {
      return true;
    }
    current = typeof current === 'object' ? (current as { cause?: unknown }).cause : undefined;
  }
  return false;
}

function matchesChunkError(err: unknown): boolean {
  if (typeof err === 'string') {
    return hasChunkMessage(err);
  }
  if (typeof err !== 'object') {
    return false;
  }
  const { name, message, code } = err as { name?: unknown; message?: unknown; code?: unknown };
  return (
    name === CHUNK_ERROR_NAME ||
    code === DEFER_LOADING_FAILED_CODE ||
    (typeof message === 'string' && hasChunkMessage(message))
  );
}

function hasChunkMessage(text: string): boolean {
  const lower = text.toLowerCase();
  return CHUNK_ERROR_MESSAGES.some((known) => lower.includes(known));
}

/** Reloads the page. Tests replace it, because a test page cannot reload. */
export const PAGE_RELOAD = new InjectionToken<() => void>('PAGE_RELOAD', {
  factory: () => {
    const view = inject(DOCUMENT).defaultView;
    return () => view?.location.reload();
  },
});

/**
 * Reloads the page once when a lazy part fails to load after a deploy.
 * The router rewrites `history.state` after every navigation, failed ones included, and drops the marker.
 * So the service reads the marker when the app starts, before the first navigation, and keeps it in memory.
 * A page that started on a marked entry never reloads again.
 * A load that fails because the page is already leaving (`beforeunload` fired) is no deploy: it neither reloads
 * (a reload would replace the navigation the page is making) nor reaches the default handler.
 */
@Injectable({ providedIn: 'root' })
export class ChunkReload {
  private readonly history = inject(DOCUMENT).defaultView?.history;
  private readonly reload = inject(PAGE_RELOAD);
  private readonly markedAtStart = isMarked(this.history?.state);
  private leaving = false;

  constructor() {
    inject(DOCUMENT).defaultView?.addEventListener('beforeunload', () => {
      this.leaving = true;
      setTimeout(() => (this.leaving = false), LEAVING_MS);
    });
  }

  /**
   * Returns true when `err` is a chunk load error that was handled: the page reloaded for it, or is leaving.
   * Returns false for every other error, and for a chunk load error on a page that already reloaded for it.
   */
  reloadOnce(err: unknown): boolean {
    const history = this.history;
    if (!history || !isChunkLoadError(err)) {
      return false;
    }
    if (this.leaving) {
      return true;
    }
    if (this.markedAtStart || isMarked(history.state)) {
      return false;
    }
    history.replaceState({ ...(history.state as object | null), [CHUNK_RELOAD_MARKER]: true }, '');
    this.reload();
    return true;
  }
}

function isMarked(state: unknown): boolean {
  return (
    typeof state === 'object' &&
    state !== null &&
    (state as Record<string, unknown>)[CHUNK_RELOAD_MARKER] === true
  );
}

/** Reloads on a chunk load error and sends every other error, and a repeated one, to the default handler. */
@Injectable()
export class ChunkReloadErrorHandler extends ErrorHandler {
  private readonly chunkReload = inject(ChunkReload);

  override handleError(error: unknown): void {
    if (!this.chunkReload.reloadOnce(error)) {
      super.handleError(error);
    }
  }
}
