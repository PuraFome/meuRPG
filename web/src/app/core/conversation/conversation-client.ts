import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { PlayService } from '../../../gen/meurpg/play/v1/play_pb';
import type {
  SendSessionMessageRequest,
  SessionMessage,
} from '../../../gen/meurpg/play/v1/session_messages_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import type { ConversationApi, MessagesPage } from './conversation-state';

/**
 * Thin wrapper around the conversation calls of `PlayService` (MR-051,
 * RN-36). The idempotency key is made by the caller, once per line, and sent
 * again on a retry. Callers map errors with `sendErrorMessage` /
 * `readErrorMessage`. `providedIn: 'root'`, imported only by lazy code.
 */
@Injectable({ providedIn: 'root' })
export class ConversationClient implements ConversationApi {
  private readonly client = createClient(PlayService, inject(CONNECT_TRANSPORT));

  /** `SendSessionMessage`: the line as its sender reads it. */
  async send(request: SendSessionMessageRequest): Promise<SessionMessage> {
    const res = await this.client.sendSessionMessage(request);
    if (!res.message) {
      throw new Error('SendSessionMessage answered without its line');
    }
    return res.message;
  }

  /** `ListSessionMessages`: a page of the open session's lines, oldest first;
   * `beforeId` asks for the page before that line. */
  async list(campaignId: string, beforeId = ''): Promise<MessagesPage> {
    const res = await this.client.listSessionMessages({ campaignId, beforeId });
    return { messages: res.messages, hasOlder: res.hasOlder, voiceEnabled: res.voiceEnabled };
  }

  /** `SignalMasterTyping`: tells the players the master is writing. */
  async signalTyping(campaignId: string): Promise<void> {
    await this.client.signalMasterTyping({ campaignId });
  }
}
