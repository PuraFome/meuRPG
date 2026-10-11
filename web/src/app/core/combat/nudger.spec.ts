import { TestBed } from '@angular/core/testing';
import { Code, ConnectError } from '@connectrpc/connect';
import { afterEach, beforeEach, vi } from 'vitest';

import { CONNECT_TRANSPORT } from '../connect/transport';
import { NUDGE_COOLDOWN_MS, NUDGE_DONE, NUDGE_MESSAGE_MS, NUDGE_WAIT, Nudger } from './nudger';

// The master's "Chamar a atenção": one call per character, confirmed in words, off for the server's wait.

function setup(outcome: 'ok' | Code = 'ok') {
  TestBed.configureTestingModule({
    providers: [Nudger, { provide: CONNECT_TRANSPORT, useValue: {} }],
  });
  const nudger = TestBed.inject(Nudger);
  const sent: object[] = [];
  (nudger as unknown as { client: unknown }).client = {
    nudgePlayer: (req: object) => {
      sent.push(req);
      return outcome === 'ok'
        ? Promise.resolve({})
        : Promise.reject(new ConnectError('no', outcome));
    },
  };
  return { nudger, sent };
}

describe('Nudger', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    TestBed.resetTestingModule();
  });
  afterEach(() => vi.useRealTimers());

  it('calls the character with the campaign, confirms it and turns that character off', async () => {
    const { nudger, sent } = setup();
    await nudger.call('camp', 'pens');
    expect(sent).toEqual([{ campaignId: 'camp', characterId: 'pens' }]);
    expect(nudger.message()).toBe(NUDGE_DONE);
    expect(nudger.messageFor()).toBe('pens');
    expect(nudger.cooling().has('pens')).toBe(true);
    expect(nudger.cooling().has('toren')).toBe(false);
  });

  it('does not send a second call during the wait, and allows it after', async () => {
    const { nudger, sent } = setup();
    await nudger.call('camp', 'pens');
    await nudger.call('camp', 'pens');
    expect(sent).toHaveLength(1);
    vi.advanceTimersByTime(NUDGE_COOLDOWN_MS);
    expect(nudger.cooling().has('pens')).toBe(false);
    await nudger.call('camp', 'pens');
    expect(sent).toHaveLength(2);
  });

  it('empties the message after a few seconds', async () => {
    const { nudger } = setup();
    await nudger.call('camp', 'pens');
    vi.advanceTimersByTime(NUDGE_MESSAGE_MS);
    expect(nudger.message()).toBe('');
    expect(nudger.messageFor()).toBe('');
  });

  it('shows the server wait as a refusal and keeps the character off', async () => {
    const { nudger } = setup(Code.ResourceExhausted);
    await nudger.call('camp', 'pens');
    expect(nudger.message()).toBe(NUDGE_WAIT);
    expect(nudger.cooling().has('pens')).toBe(true);
  });

  it('gives the button back after any other failure, with a sentence for it', async () => {
    const { nudger } = setup(Code.NotFound);
    await nudger.call('camp', 'pens');
    expect(nudger.message()).toBe('Esse personagem não tem jogador na mesa.');
    expect(nudger.cooling().has('pens')).toBe(false);
  });
});
