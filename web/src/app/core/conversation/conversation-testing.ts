import { type MessageInitShape, create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import {
  type SendSessionMessageRequest,
  type SessionMessage,
  SessionMessageSchema,
  SessionMessageSpeakerKind,
} from '../../../gen/meurpg/play/v1/session_messages_pb';
import type { ConversationApi, MessagesPage } from './conversation-state';

/**
 * Builders and a stand-in for the conversation specs (never imported by the
 * app itself, so never bundled): a line as the server sends it, and a
 * `ConversationApi` whose answers the spec settles by hand, so it decides
 * which of two calls answers first.
 */
export function line(
  id: string,
  partial: Omit<MessageInitShape<typeof SessionMessageSchema>, 'id' | 'createdAt'> & {
    at?: number;
  } = {},
): SessionMessage {
  const { at = 0, ...rest } = partial;
  return create(SessionMessageSchema, {
    id,
    speakerKind: SessionMessageSpeakerKind.NARRATOR,
    text: `texto ${id}`,
    forEveryone: true,
    ...rest,
    createdAt: timestampFromDate(new Date(Date.UTC(2026, 9, 3, 21, 0, at))),
  });
}

interface Waiting<T> {
  resolve(value: T): void;
  reject(reason: unknown): void;
}

function deferred<T>(): { promise: Promise<T> } & Waiting<T> {
  let handlers!: Waiting<T>;
  const promise = new Promise<T>((resolve, reject) => (handlers = { resolve, reject }));
  return { promise, ...handlers };
}

export class FakeConversationApi implements ConversationApi {
  readonly sent: SendSessionMessageRequest[] = [];
  readonly listed: { campaignId: string; beforeId: string }[] = [];
  typingCalls = 0;
  failTyping = false;
  private readonly sends: Waiting<SessionMessage>[] = [];
  private readonly lists: Waiting<MessagesPage>[] = [];

  send(request: SendSessionMessageRequest): Promise<SessionMessage> {
    this.sent.push(request);
    const d = deferred<SessionMessage>();
    this.sends.push(d);
    return d.promise;
  }

  list(campaignId: string, beforeId = ''): Promise<MessagesPage> {
    this.listed.push({ campaignId, beforeId });
    const d = deferred<MessagesPage>();
    this.lists.push(d);
    return d.promise;
  }

  signalTyping(): Promise<void> {
    this.typingCalls++;
    return this.failTyping ? Promise.reject(new Error('down')) : Promise.resolve();
  }

  /** Settles the oldest send that still waits. */
  answerSend(message: SessionMessage): void {
    this.sends.shift()?.resolve(message);
  }

  failSend(error: unknown): void {
    this.sends.shift()?.reject(error);
  }

  /** Settles the oldest list call that still waits. */
  answerList(messages: SessionMessage[], hasOlder = false, voiceEnabled = true): void {
    this.lists.shift()?.resolve({ messages, hasOlder, voiceEnabled });
  }

  failList(error: unknown): void {
    this.lists.shift()?.reject(error);
  }
}
