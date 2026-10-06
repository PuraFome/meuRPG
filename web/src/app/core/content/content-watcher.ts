import { DOCUMENT, DestroyRef, Injectable, effect, inject, untracked } from '@angular/core';

import { LiveSessionSourceLive } from '../../pages/live-session/live-session-source.live';
import { LiveStream } from '../../pages/live-session/live-stream';
import { OpenSessions } from '../../shell/live-notice/open-sessions';

/** The wait that gathers a burst of hints into one read: the server already lets one through every 250 ms. */
export const CONTENT_DEBOUNCE_MS = 300;

/**
 * Listens to a campaign's live session for `content_changed` (RN-23, RN-10) and says so, so a page that shows the table's
 * content (the content list and its options, "Magias", the character editor, the level-up, an open sheet) reads it again with
 * its own role. The hint carries nothing: the page asks the server, which answers for who is asking. It is the session page's
 * own stream client (`LiveStream`, ADR-0005's rules: backoff, closed when the tab is hidden, no reconnect without access), like
 * the sheet's `XpWatcher`, one stream per page (`providers: [ContentWatcher, LiveSessionSourceLive]`). Hints that come close
 * together are one read (`CONTENT_DEBOUNCE_MS`), and a later `ready` (a reconnection, the tab back) reads once, because a hint
 * may have been missed. Without an open session nobody sends a hint, so there is no stream: the page reads on load.
 */
@Injectable()
export class ContentWatcher {
  private readonly source = inject(LiveSessionSourceLive);
  private readonly document = inject(DOCUMENT);
  private readonly openSessions = inject(OpenSessions, { optional: true });
  private readonly destroyRef = inject(DestroyRef);

  private stream: LiveStream | null = null;
  private campaignId: string | null = null;
  private timer: ReturnType<typeof setTimeout> | null = null;

  constructor() {
    this.destroyRef.onDestroy(() => this.follow(null, () => undefined));
  }

  /** Follows the campaign while it has an open session (call it in a constructor: it makes an effect). */
  whileLive(campaignId: () => string, onChange: () => void): void {
    effect(() => {
      const id = campaignId();
      const live = id !== '' && (this.openSessions?.sessions().some((o) => o.campaignId === id) ?? false);
      untracked(() => this.follow(live ? id : null, onChange));
    });
  }

  /** Follows `campaignId`'s session, or stops with `null`. */
  follow(campaignId: string | null, onChange: () => void): void {
    if (campaignId === this.campaignId) {
      return;
    }
    this.stream?.stop();
    this.stream = null;
    this.clearTimer();
    this.campaignId = campaignId;
    if (campaignId === null) {
      return;
    }
    let first = true;
    const hint = () => {
      this.clearTimer();
      this.timer = setTimeout(() => {
        this.timer = null;
        onChange();
      }, CONTENT_DEBOUNCE_MS);
    };
    const stream = new LiveStream({
      open: (signal) => this.source.watch(campaignId, signal),
      classify: (err) => this.source.classifyError(err),
      document: this.document,
      handlers: {
        // The page reads on load itself: only a later `ready` reads again.
        onReady: () => {
          if (!first) {
            hint();
          }
          first = false;
        },
        onVitals: () => undefined,
        onContentChanged: hint,
        onEnded: () => this.stop(stream),
        onFatal: () => this.stop(stream),
      },
    });
    this.stream = stream;
    stream.start();
  }

  private stop(stream: LiveStream): void {
    stream.stop();
    if (this.stream === stream) {
      this.stream = null;
      this.campaignId = null;
    }
  }

  private clearTimer(): void {
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }
}
