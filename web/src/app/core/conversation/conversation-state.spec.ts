import { Code, ConnectError } from '@connectrpc/connect';

import { SessionMessageSpeakerKind as Kind } from '../../../gen/meurpg/play/v1/session_messages_pb';
import { ConversationState, TYPING_THROTTLE_MS, type PendingSend } from './conversation-state';
import { FakeConversationApi, line } from './conversation-testing';
import type { Draft } from './message-view';

const oocDraft: Draft = {
  role: 'player',
  text: 'Eu vou até a porta.',
  as: 'out-of-character',
  characterId: '',
  onlyMaster: false,
};

describe('ConversationState', () => {
  let api: FakeConversationApi;
  let state: ConversationState;
  let self: string;
  const flush = () => vi.advanceTimersByTimeAsync(0);
  const ids = () => state.lines().map((l) => l.id);
  const only = (): PendingSend => state.pending()[0];

  beforeEach(() => {
    vi.useFakeTimers();
    api = new FakeConversationApi();
    self = 'u-me';
    state = new ConversationState(
      api,
      () => 'c1',
      () => self,
    );
  });

  afterEach(() => {
    state.dispose();
    vi.useRealTimers();
  });

  async function opened(...messages: ReturnType<typeof line>[]): Promise<void> {
    const open = state.open();
    api.answerList(messages, true, true);
    await open;
  }

  describe('open and loadOlder', () => {
    it('reads the newest page, oldest first, with hasOlder and voiceEnabled', async () => {
      const open = state.open();
      expect(api.listed).toEqual([{ campaignId: 'c1', beforeId: '' }]);
      api.answerList([line('a', { at: 1 }), line('b', { at: 2 })], true, false);
      await open;
      expect(ids()).toEqual(['a', 'b']);
      expect(state.hasOlder()).toBe(true);
      expect(state.voiceEnabled()).toBe(false);
      expect(state.loaded()).toBe(true);
    });

    it('does not count the history as unread', async () => {
      state.setVisible(false);
      await opened(line('a'), line('b'));
      expect(state.unread()).toBe(0);
    });

    it('says why it could not read, and keeps what it had', async () => {
      await opened(line('a'));
      const again = state.open();
      api.failList(new ConnectError('x', Code.Unavailable));
      await again;
      expect(state.error()).not.toBe('');
      expect(ids()).toEqual(['a']);
    });

    it('keeps a line the stream brought while the page was on its way', async () => {
      const open = state.open();
      state.receive(line('c', { at: 3 }));
      api.answerList([line('a', { at: 1 }), line('b', { at: 2 })]);
      await open;
      expect(ids()).toEqual(['a', 'b', 'c']);
    });

    it('loadOlder asks before the oldest line and puts the page in front', async () => {
      await opened(line('c', { at: 3 }), line('d', { at: 4 }));
      const older = state.loadOlder();
      expect(api.listed.at(-1)).toEqual({ campaignId: 'c1', beforeId: 'c' });
      api.answerList([line('a', { at: 1 }), line('b', { at: 2 })], false);
      await older;
      expect(ids()).toEqual(['a', 'b', 'c', 'd']);
      expect(state.hasOlder()).toBe(false);
    });

    it('loadOlder does nothing when there is nothing older or a read is on its way', async () => {
      const open = state.open();
      api.answerList([line('a')], false);
      await open;
      await state.loadOlder();
      expect(api.listed).toHaveLength(1);
    });
  });

  describe('lines that arrive', () => {
    it('keeps one copy of a line and the order of the table', async () => {
      await opened(line('a', { at: 1 }));
      state.receive(line('c', { at: 3 }));
      state.receive(line('b', { at: 2 }));
      state.receive(line('b', { at: 2 }));
      expect(ids()).toEqual(['a', 'b', 'c']);
    });

    it('counts the lines of others that arrive while it is not visible, and markRead clears', async () => {
      await opened();
      state.setVisible(false);
      state.receive(line('a', { at: 1, authorUserId: 'u-mestre' }));
      state.receive(line('a', { at: 1, authorUserId: 'u-mestre' }));
      state.receive(line('b', { at: 2, authorUserId: 'u-me' }));
      state.receive(line('c', { at: 3 }));
      expect(state.unread()).toBe(2);
      state.setVisible(true);
      expect(state.unread()).toBe(0);
      state.receive(line('d', { at: 4 }));
      expect(state.unread()).toBe(0);
    });

    it('markRead clears by itself', async () => {
      state.setVisible(false);
      state.receive(line('a'));
      state.markRead();
      expect(state.unread()).toBe(0);
    });
  });

  describe('sending', () => {
    it('refuses a draft that is not valid, sending nothing', () => {
      expect(state.send({ ...oocDraft, text: '  ' })).toBe('Escreva uma mensagem.');
      expect(api.sent).toEqual([]);
      expect(state.pending()).toEqual([]);
    });

    it('shows the line as sending, with a key, at the end of the list', async () => {
      await opened(line('a'));
      expect(state.send(oocDraft)).toBeNull();
      expect(only()).toMatchObject({ status: 'sending', text: 'Eu vou até a porta.' });
      expect(api.sent[0]).toMatchObject({
        campaignId: 'c1',
        speakerKind: Kind.OUT_OF_CHARACTER,
        idempotencyKey: only().key,
      });
      expect(state.entries().map((e) => e.kind)).toEqual(['line', 'pending']);
    });

    it("replaces the pending line with the server's, once", async () => {
      state.send(oocDraft);
      api.answerSend(line('s1', { at: 5 }));
      await flush();
      expect(state.pending()).toEqual([]);
      expect(ids()).toEqual(['s1']);
    });

    it('the stream event for the same line before the response leaves one copy', async () => {
      state.send(oocDraft);
      state.receive(line('s1', { at: 5, authorUserId: 'u-me' }));
      api.answerSend(line('s1', { at: 5, authorUserId: 'u-me' }));
      await flush();
      expect(ids()).toEqual(['s1']);
      expect(state.pending()).toEqual([]);
    });

    it('shows one row, not two, while only the stream has told the line (the answer is still on its way)', async () => {
      state.send(oocDraft);
      state.receive(line('s1', { at: 5, authorUserId: 'u-me', text: oocDraft.text }));
      expect(state.entries()).toHaveLength(1);
      expect(state.entries()[0].kind).toBe('line');
      api.answerSend(line('s1', { at: 5, authorUserId: 'u-me', text: oocDraft.text }));
      await flush();
      expect(state.entries()).toHaveLength(1);
    });

    it('keeps a failed line when another line with the same words arrives from the stream', async () => {
      state.send(oocDraft);
      api.failSend(new ConnectError('x', Code.Unavailable));
      await flush();
      state.receive(line('s9', { at: 9, authorUserId: 'u-me', text: oocDraft.text }));
      expect(state.pending()).toHaveLength(1);
      expect(state.entries()).toHaveLength(2);
    });

    it('the stream event after the response leaves one copy too', async () => {
      state.send(oocDraft);
      api.answerSend(line('s1', { at: 5, authorUserId: 'u-me' }));
      await flush();
      state.receive(line('s1', { at: 5, authorUserId: 'u-me' }));
      expect(ids()).toEqual(['s1']);
    });

    it('a failure keeps the line, marked failed with a reason', async () => {
      state.send(oocDraft);
      api.failSend(new ConnectError('x', Code.Unavailable));
      await flush();
      expect(only()).toMatchObject({
        status: 'failed',
        error: 'Não foi enviada. A conexão caiu.',
      });
    });

    it('a refusal says its own reason', async () => {
      state.send(oocDraft);
      api.failSend(new ConnectError('x', Code.PermissionDenied));
      await flush();
      expect(only().error).toMatch(/^Não foi enviada\. /);
      expect(only().error).not.toContain('conexão');
    });

    it('retry sends the same request with the same key and only once the line is failed', async () => {
      state.send(oocDraft);
      const key = only().key;
      state.retry(key);
      expect(api.sent).toHaveLength(1);
      api.failSend(new ConnectError('x', Code.Unavailable));
      await flush();
      state.retry(key);
      expect(only().status).toBe('sending');
      expect(api.sent).toHaveLength(2);
      expect(api.sent[1].idempotencyKey).toBe(api.sent[0].idempotencyKey);
      expect(api.sent[1].text).toBe(api.sent[0].text);
      api.answerSend(line('s1', { at: 5 }));
      await flush();
      expect(state.pending()).toEqual([]);
      expect(ids()).toEqual(['s1']);
    });

    it('a retry that finds the line already stored does not duplicate it', async () => {
      state.send(oocDraft);
      api.failSend(new ConnectError('x', Code.Unavailable));
      await flush();
      state.receive(line('s1', { at: 5, authorUserId: 'u-me' }));
      state.retry(only().key);
      api.answerSend(line('s1', { at: 5, authorUserId: 'u-me' }));
      await flush();
      expect(ids()).toEqual(['s1']);
      expect(state.pending()).toEqual([]);
    });

    it('discard drops a failed line', async () => {
      state.send(oocDraft);
      api.failSend(new ConnectError('x', Code.Unavailable));
      await flush();
      state.discard(only().key);
      expect(state.pending()).toEqual([]);
    });

    it('two sends have two keys, each retried on its own', async () => {
      state.send(oocDraft);
      state.send({ ...oocDraft, text: 'Segunda.' });
      expect(new Set(state.pending().map((p) => p.key)).size).toBe(2);
    });
  });

  describe('the typing hint', () => {
    it('turns on and off by itself after the seconds the server says', () => {
      state.masterTyping(6);
      expect(state.typing()).toBe(true);
      vi.advanceTimersByTime(5_999);
      expect(state.typing()).toBe(true);
      vi.advanceTimersByTime(1);
      expect(state.typing()).toBe(false);
    });

    it('another hint starts the count again', () => {
      state.masterTyping(6);
      vi.advanceTimersByTime(4_000);
      state.masterTyping(6);
      vi.advanceTimersByTime(4_000);
      expect(state.typing()).toBe(true);
      vi.advanceTimersByTime(2_000);
      expect(state.typing()).toBe(false);
    });

    it("goes away when the master's line arrives", () => {
      state.masterTyping(6);
      state.receive(line('a', { speakerKind: Kind.NARRATOR }));
      expect(state.typing()).toBe(false);
    });

    it('stays when a player line arrives', () => {
      state.masterTyping(6);
      state.receive(line('a', { speakerKind: Kind.OUT_OF_CHARACTER }));
      expect(state.typing()).toBe(true);
    });

    it('a hint of no seconds shows nothing', () => {
      state.masterTyping(0);
      expect(state.typing()).toBe(false);
    });
  });

  describe('signalTyping', () => {
    it('calls at most once every two seconds', () => {
      state.signalTyping();
      state.signalTyping();
      vi.advanceTimersByTime(TYPING_THROTTLE_MS - 1);
      state.signalTyping();
      expect(api.typingCalls).toBe(1);
      vi.advanceTimersByTime(1);
      state.signalTyping();
      expect(api.typingCalls).toBe(2);
    });

    it('ignores a failure', async () => {
      api.failTyping = true;
      state.signalTyping();
      await flush();
      expect(state.error()).toBe('');
    });
  });

  describe('sync and readGap', () => {
    it('the first sync opens; the next ones read the gap', async () => {
      const first = state.sync();
      api.answerList([line('a', { at: 1 })]);
      await first;
      const second = state.sync();
      api.answerList([line('a', { at: 1 }), line('b', { at: 2 })]);
      await second;
      expect(ids()).toEqual(['a', 'b']);
      expect(api.listed.map((l) => l.beforeId)).toEqual(['', '']);
    });

    it('sync opens again when the first read failed', async () => {
      const first = state.sync();
      api.failList(new ConnectError('x', Code.Unavailable));
      await first;
      const second = state.sync();
      api.answerList([line('a')]);
      await second;
      expect(ids()).toEqual(['a']);
    });

    it('merges the gap by id, in order, without duplicating, and counts the new lines', async () => {
      await opened(line('a', { at: 1 }), line('b', { at: 2 }));
      state.setVisible(false);
      const gap = state.readGap();
      api.answerList([line('b', { at: 2 }), line('c', { at: 3 }), line('d', { at: 4 })], true);
      await gap;
      expect(ids()).toEqual(['a', 'b', 'c', 'd']);
      expect(state.unread()).toBe(2);
    });

    it('does not count the gap as unread while visible', async () => {
      await opened(line('a', { at: 1 }));
      const gap = state.readGap();
      api.answerList([line('a', { at: 1 }), line('b', { at: 2 })]);
      await gap;
      expect(state.unread()).toBe(0);
    });

    it('reads older pages until it meets a line it knows', async () => {
      await opened(line('a', { at: 1 }), line('b', { at: 2 }));
      const gap = state.readGap();
      api.answerList([line('e', { at: 5 }), line('f', { at: 6 })], true);
      await flush();
      expect(api.listed.at(-1)).toEqual({ campaignId: 'c1', beforeId: 'e' });
      api.answerList([line('c', { at: 3 }), line('d', { at: 4 }), line('b', { at: 2 })], true);
      await gap;
      expect(ids()).toEqual(['a', 'b', 'c', 'd', 'e', 'f']);
      expect(state.hasOlder()).toBe(true);
    });

    it('a gap too long to join starts again from the newest page', async () => {
      await opened(line('a', { at: 1 }));
      const gap = state.readGap();
      for (let n = 0; n < 10; n++) {
        api.answerList([line(`x${n}`, { at: 100 - n })], true);
        await flush();
      }
      await gap;
      expect(ids()).not.toContain('a');
      expect(state.hasOlder()).toBe(true);
    });

    it('a failed gap read keeps what is on screen', async () => {
      await opened(line('a'));
      const gap = state.readGap();
      api.failList(new ConnectError('x', Code.Unavailable));
      await gap;
      expect(ids()).toEqual(['a']);
    });

    it('a new campaign starts the conversation again', async () => {
      let campaign = 'c1';
      state = new ConversationState(
        api,
        () => campaign,
        () => self,
      );
      const first = state.sync();
      api.answerList([line('a')]);
      await first;
      campaign = 'c2';
      const second = state.sync();
      expect(state.lines()).toEqual([]);
      api.answerList([line('z')]);
      await second;
      expect(ids()).toEqual(['z']);
      expect(api.listed.at(-1)).toEqual({ campaignId: 'c2', beforeId: '' });
    });
  });
});
