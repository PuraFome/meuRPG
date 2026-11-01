import { DOCUMENT } from '@angular/common';
import { ErrorHandler, Injectable, InjectionToken, inject } from '@angular/core';

/** The `history.state` key that marks an entry whose page was already reloaded for a chunk load error. */
export const CHUNK_RELOAD_MARKER = 'chunkReloaded';

const CHUNK_ERROR_NAME = 'ChunkLoadError';
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
  const { name, message } = err as { name?: unknown; message?: unknown };
  return name === CHUNK_ERROR_NAME || (typeof message === 'string' && hasChunkMessage(message));
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
 */
@Injectable({ providedIn: 'root' })
export class ChunkReload {
  private readonly history = inject(DOCUMENT).defaultView?.history;
  private readonly reload = inject(PAGE_RELOAD);
  private readonly markedAtStart = isMarked(this.history?.state);

  /** Reloads and returns true for a chunk load error on a page that has not reloaded for it; returns false otherwise. */
  reloadOnce(err: unknown): boolean {
    const history = this.history;
    if (!history || !isChunkLoadError(err) || this.markedAtStart || isMarked(history.state)) {
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
