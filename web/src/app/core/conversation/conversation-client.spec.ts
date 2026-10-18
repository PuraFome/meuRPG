import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  GameSessionBlockedReason,
  GameSessionBlockedSchema,
} from '../../../gen/meurpg/play/v1/play_pb';
import { SendSessionMessageRequestSchema } from '../../../gen/meurpg/play/v1/session_messages_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { ConversationClient } from './conversation-client';
import { readErrorMessage, sendErrorMessage } from './conversation-errors';
import { line } from './conversation-testing';

function client(calls: Record<string, unknown>) {
  TestBed.configureTestingModule({
    providers: [ConversationClient, { provide: CONNECT_TRANSPORT, useValue: {} }],
  });
  const c = TestBed.inject(ConversationClient);
  (c as unknown as { client: unknown }).client = calls;
  return c;
}

describe('ConversationClient', () => {
  it('sends the request as it is, key included, and returns the line', async () => {
    const sent: unknown[] = [];
    const c = client({
      sendSessionMessage: (req: unknown) => {
        sent.push(req);
        return Promise.resolve({ message: line('m1') });
      },
    });
    const request = create(SendSessionMessageRequestSchema, {
      campaignId: 'c',
      idempotencyKey: 'k',
    });
    expect((await c.send(request)).id).toBe('m1');
    expect(sent).toEqual([request]);
  });

  it('refuses an answer with no line', async () => {
    const c = client({ sendSessionMessage: () => Promise.resolve({}) });
    await expect(c.send(create(SendSessionMessageRequestSchema))).rejects.toThrow();
  });

  it('lists a page, before a line when asked', async () => {
    const asked: unknown[] = [];
    const c = client({
      listSessionMessages: (req: unknown) => {
        asked.push(req);
        return Promise.resolve({ messages: [line('a')], hasOlder: true, voiceEnabled: true });
      },
    });
    const page = await c.list('c', 'x');
    expect(asked).toEqual([{ campaignId: 'c', beforeId: 'x' }]);
    expect(page).toMatchObject({ hasOlder: true, voiceEnabled: true });
    expect(page.messages).toHaveLength(1);
  });

  it('signals the typing of the campaign', async () => {
    const asked: unknown[] = [];
    const c = client({
      signalMasterTyping: (req: unknown) => {
        asked.push(req);
        return Promise.resolve({});
      },
    });
    await c.signalTyping('c');
    expect(asked).toEqual([{ campaignId: 'c' }]);
  });
});

describe('the conversation errors', () => {
  const noSession = new ConnectError('x', Code.FailedPrecondition, undefined, [
    {
      desc: GameSessionBlockedSchema,
      value: { reason: GameSessionBlockedReason.NO_OPEN_SESSION },
    },
  ]);

  it('a send that fails says "Não foi enviada." and why', () => {
    expect(sendErrorMessage(new ConnectError('x', Code.Unavailable))).toBe(
      'Não foi enviada. A conexão caiu.',
    );
    expect(sendErrorMessage(noSession)).toContain('sessão acabou');
    expect(sendErrorMessage(new ConnectError('x', Code.InvalidArgument))).toContain('2.000');
    expect(sendErrorMessage(new ConnectError('x', Code.ResourceExhausted))).toContain('Espere');
  });

  it('a read that fails says what the panel can do about it', () => {
    expect(readErrorMessage(noSession)).toContain('Não há sessão aberta');
    expect(readErrorMessage(new ConnectError('x', Code.Unavailable))).toContain('Não deu para ler');
  });
});
