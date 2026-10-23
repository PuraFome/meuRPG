import { computed, signal } from '@angular/core';

import {
  type SendSessionMessageRequest,
  type SessionMessage,
  SessionMessageSpeakerKind,
} from '../../../gen/meurpg/play/v1/session_messages_pb';
import { newKey } from '../connect/idempotency';
import { readErrorMessage, sendErrorMessage } from './conversation-errors';
import { type Draft, buildSendRequest } from './message-view';

/** The slice of `ConversationClient` the state uses, so a spec hands it a fake. */
export interface ConversationApi {
  send(request: SendSessionMessageRequest): Promise<SessionMessage>;
  list(campaignId: string, beforeId?: string): Promise<MessagesPage>;
  signalTyping(campaignId: string): Promise<void>;
}

export interface MessagesPage {
  /** Oldest first. */
  readonly messages: readonly SessionMessage[];
  readonly hasOlder: boolean;
  readonly voiceEnabled: boolean;
}

/** Time as the state needs it; a spec runs it on fake timers. */
export interface ConversationTimers {
  now(): number;
  setTimeout(run: () => void, ms: number): unknown;
  clearTimeout(handle: unknown): void;
}

const REAL_TIMERS: ConversationTimers = {
  now: () => Date.now(),
  setTimeout: (run, ms) => setTimeout(run, ms),
  clearTimeout: (handle) => clearTimeout(handle as ReturnType<typeof setTimeout>),
};

/** The server answers more than one `SignalMasterTyping` per 2 s with nothing; the client does not even ask. */
export const TYPING_THROTTLE_MS = 2000;

/** How many pages a reconnection reads before it gives up joining the gap to what is on screen. */
const MAX_GAP_PAGES = 5;

/** A line of this reader that left but is not confirmed yet (`sending`), or did not go (`failed`). */
export interface PendingSend {
  /** The idempotency key, kept across retries: a retry never stores the line twice. */
  readonly key: string;
  readonly request: SendSessionMessageRequest;
  readonly text: string;
  readonly status: 'sending' | 'failed';
  /** Why it failed, worded for the person; empty while sending. */
  readonly error: string;
}

export type ConversationEntry =
  | { readonly kind: 'line'; readonly message: SessionMessage }
  | { readonly kind: 'pending'; readonly pending: PendingSend };

const NANOS_PER_MS = 1e6;

function timeOf(message: SessionMessage): number {
  const at = message.createdAt;
  // A line without a time is the newest we know of.
  return at ? Number(at.seconds) * 1000 + at.nanos / NANOS_PER_MS : Number.POSITIVE_INFINITY;
}

/** Union by id, ordered by time; a line keeps its place among equals. */
function union(
  known: readonly SessionMessage[],
  more: readonly SessionMessage[],
): SessionMessage[] {
  const seen = new Set(known.map((m) => m.id));
  const added = more.filter((m) => !seen.has(m.id) && seen.add(m.id));
  return [...known, ...added].sort((a, b) => timeOf(a) - timeOf(b));
}

/**
 * The conversation of the scene as this reader sees it (MR-051, RN-36): the
 * lines oldest first, the lines being sent, the unread count and the typing
 * hint. Pure TypeScript over signals, so the rules are tested without a DOM:
 *
 * - a line is kept once, by id, wherever it comes from (a page, the stream, the
 *   answer of the reader's own send), in the order of the table;
 * - a send is shown at once, after the lines, and is replaced by the server's
 *   line; a retry reuses the key, so a line that did go through is not doubled;
 * - lines of others that arrive while the conversation is not on screen are
 *   counted, until it is shown;
 * - the typing hint carries no text and goes away by itself (RN-10).
 */
export class ConversationState {
  readonly lines = signal<readonly SessionMessage[]>([]);
  readonly pending = signal<readonly PendingSend[]>([]);
  readonly hasOlder = signal(false);
  /** The server can make the voice of a line ("Ouvir"); false hides the button. */
  readonly voiceEnabled = signal(false);
  readonly typing = signal(false);
  readonly unread = signal(0);
  /** The first page was read, so "empty" is true and not "not read yet". */
  readonly loaded = signal(false);
  /** Why the last read failed; empty when it did not. */
  readonly error = signal('');
  readonly entries = computed<readonly ConversationEntry[]>(() => [
    ...this.lines().map((message) => ({ kind: 'line' as const, message })),
    ...this.pending().map((pending) => ({ kind: 'pending' as const, pending })),
  ]);

  private visible = true;
  private generation = 0;
  private openedFor = '';
  private lastSignal = Number.NEGATIVE_INFINITY;
  private typingTimer: unknown;
  private olderRunning = false;

  constructor(
    private readonly api: ConversationApi,
    private readonly campaignId: () => string,
    /** The reader's account, to tell their own lines from others'. */
    private readonly selfUserId: () => string,
    private readonly timers: ConversationTimers = REAL_TIMERS,
  ) {}

  dispose(): void {
    this.timers.clearTimeout(this.typingTimer);
  }

  /** The page says whether the conversation is on screen (the phone's tab). Showing it reads everything. */
  setVisible(visible: boolean): void {
    this.visible = visible;
    if (visible) {
      this.markRead();
    }
  }

  markRead(): void {
    this.unread.set(0);
  }

  /** Called on every `ready` of the stream: the first one reads the newest page, a reconnection reads the gap. */
  sync(): Promise<void> {
    const id = this.campaignId();
    if (this.openedFor !== id) {
      this.reset(id);
    }
    return this.loaded() ? this.readGap() : this.open();
  }

  /** The newest page. Lines the stream brought meanwhile are kept. */
  async open(): Promise<void> {
    const generation = ++this.generation;
    try {
      const page = await this.api.list(this.campaignId());
      if (generation !== this.generation) {
        return;
      }
      this.lines.update((known) => union(known, page.messages));
      this.hasOlder.set(page.hasOlder);
      this.voiceEnabled.set(page.voiceEnabled);
      this.loaded.set(true);
      this.error.set('');
    } catch (err) {
      if (generation === this.generation) {
        this.error.set(readErrorMessage(err));
      }
    }
  }

  /** The page before the oldest line on screen. */
  async loadOlder(): Promise<void> {
    const oldest = this.lines()[0];
    if (!oldest || !this.hasOlder() || this.olderRunning) {
      return;
    }
    this.olderRunning = true;
    const generation = this.generation;
    try {
      const page = await this.api.list(this.campaignId(), oldest.id);
      if (generation === this.generation) {
        this.lines.update((known) => union(known, page.messages));
        this.hasOlder.set(page.hasOlder);
      }
    } catch (err) {
      this.error.set(readErrorMessage(err));
    } finally {
      this.olderRunning = false;
    }
  }

  /** After a reconnection: the lines that came while the stream was down, joined to the ones on screen by id. */
  async readGap(): Promise<void> {
    const generation = ++this.generation;
    try {
      const known = new Set(this.lines().map((m) => m.id));
      const first = await this.api.list(this.campaignId());
      let page = first;
      let fetched = [...first.messages];
      for (let n = 1; n < MAX_GAP_PAGES && this.opensGap(page, fetched, known); n++) {
        page = await this.api.list(this.campaignId(), fetched[0].id);
        fetched = [...page.messages, ...fetched];
      }
      if (generation !== this.generation) {
        return;
      }
      this.joinGap(fetched, known, page.hasOlder);
      this.voiceEnabled.set(first.voiceEnabled);
    } catch {
      // The next `ready` reads it again; what is on screen stays.
    }
  }

  /** Another page is needed when the lines read stop short of any line already on screen. */
  private opensGap(
    page: MessagesPage,
    fetched: readonly SessionMessage[],
    known: ReadonlySet<string>,
  ): boolean {
    return (
      page.hasOlder && fetched.length > 0 && known.size > 0 && !fetched.some((m) => known.has(m.id))
    );
  }

  private joinGap(
    fetched: readonly SessionMessage[],
    known: ReadonlySet<string>,
    hasOlder: boolean,
  ): void {
    if (known.size > 0 && !fetched.some((m) => known.has(m.id)) && hasOlder) {
      // Too long a gap to join: start again from the newest lines.
      this.lines.set(union([], fetched));
      this.hasOlder.set(hasOlder);
      return;
    }
    if (known.size === 0) {
      this.hasOlder.set(hasOlder);
    }
    const news = fetched.filter((m) => !known.has(m.id));
    this.lines.update((lines) => union(lines, fetched));
    this.countUnread(news);
    this.loaded.set(true);
  }

  /** A line from the stream, or from the answer of the reader's own send. Returns whether it was new. */
  receive(message: SessionMessage): boolean {
    if (this.lines().some((m) => m.id === message.id)) {
      return false;
    }
    this.lines.update((lines) => union(lines, [message]));
    this.dropSendingEcho(message);
    // The master's line is the end of "está escrevendo…".
    if (
      message.speakerKind === SessionMessageSpeakerKind.NARRATOR ||
      message.speakerKind === SessionMessageSpeakerKind.NPC
    ) {
      this.hideTyping();
    }
    this.countUnread([message]);
    return true;
  }

  /**
   * The stream can tell the reader's own line before the answer of the send arrives. The row that says "sending"
   * stands for that same line, so it goes now, or the line would be drawn twice until the answer. Only a row still
   * sending is taken: a failed one may be a line the server never got.
   */
  private dropSendingEcho(message: SessionMessage): void {
    const self = this.selfUserId();
    if (self === '' || message.authorUserId !== self) {
      return;
    }
    const echo = this.pending().find((p) => p.status === 'sending' && p.text === message.text);
    if (echo) {
      this.discard(echo.key);
    }
  }

  private countUnread(news: readonly SessionMessage[]): void {
    if (this.visible) {
      return;
    }
    const self = this.selfUserId();
    const others = news.filter((m) => m.authorUserId !== self || self === '').length;
    this.unread.update((n) => n + others);
  }

  /** Shows the line at once and sends it. Returns why the draft is not valid, or `null` when it went. */
  send(draft: Draft): string | null {
    const built = buildSendRequest(draft, this.campaignId(), newKey());
    if (!built.ok) {
      return built.error;
    }
    const key = built.request.idempotencyKey;
    this.pending.update((list) => [
      ...list,
      { key, request: built.request, text: built.request.text, status: 'sending', error: '' },
    ]);
    void this.deliver(key);
    return null;
  }

  /** "Tentar de novo": the same request with the same key. */
  retry(key: string): void {
    if (this.pending().find((p) => p.key === key)?.status === 'failed') {
      this.setPending(key, { status: 'sending', error: '' });
      void this.deliver(key);
    }
  }

  /** Gives up on a failed line. */
  discard(key: string): void {
    this.pending.update((list) => list.filter((p) => p.key !== key));
  }

  private async deliver(key: string): Promise<void> {
    const request = this.pending().find((p) => p.key === key)?.request;
    if (!request) {
      return;
    }
    try {
      const message = await this.api.send(request);
      this.discard(key);
      this.receive(message);
    } catch (err) {
      this.setPending(key, { status: 'failed', error: sendErrorMessage(err) });
    }
  }

  private setPending(key: string, change: Partial<PendingSend>): void {
    this.pending.update((list) => list.map((p) => (p.key === key ? { ...p, ...change } : p)));
  }

  /** `master_typing`: shows the hint for the seconds the server says, counted again by each hint. */
  masterTyping(expiresInSeconds: number): void {
    this.timers.clearTimeout(this.typingTimer);
    if (expiresInSeconds <= 0) {
      this.typing.set(false);
      return;
    }
    this.typing.set(true);
    this.typingTimer = this.timers.setTimeout(
      () => this.typing.set(false),
      expiresInSeconds * 1000,
    );
  }

  private hideTyping(): void {
    this.timers.clearTimeout(this.typingTimer);
    this.typing.set(false);
  }

  /** The master is typing: at most one call per 2 s, and a failure is not worth telling. */
  signalTyping(): void {
    const now = this.timers.now();
    if (now - this.lastSignal < TYPING_THROTTLE_MS) {
      return;
    }
    this.lastSignal = now;
    this.api.signalTyping(this.campaignId()).catch(() => undefined);
  }

  private reset(campaignId: string): void {
    this.generation++;
    this.openedFor = campaignId;
    this.lines.set([]);
    this.pending.set([]);
    this.hasOlder.set(false);
    this.unread.set(0);
    this.loaded.set(false);
    this.error.set('');
    this.hideTyping();
  }
}
