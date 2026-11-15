import { TestBed } from '@angular/core/testing';
import { afterEach, beforeEach, vi } from 'vitest';

import { FavorBlockedReason } from '../../../gen/meurpg/play/v1/favor_pb';
import { FavorClient } from './favor-client';
import { FAVOR_GRANT_FAILED, FAVOR_NO_SESSION, FAVOR_NOTE_MS, FavorState } from './favor-state';
import { FakeFavorClient, failedRead, favorRefused, noOpenSession } from './favor-testing';

function setup() {
  const client = new FakeFavorClient();
  TestBed.configureTestingModule({
    providers: [FavorState, { provide: FavorClient, useValue: client.asClient() }],
  });
  return { client, favor: TestBed.inject(FavorState) };
}

const pens = { characterId: 'pens', name: 'Pensantus' };
const tor = { characterId: 'tor', name: 'Tor' };

describe('FavorState', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    TestBed.resetTestingModule();
  });
  afterEach(() => vi.useRealTimers());

  it('reads nothing until a session is followed, and offers nothing', async () => {
    const { client, favor } = setup();
    await favor.refresh();
    expect(client.reads).toEqual([]);
    expect(favor.campaignId()).toBe('');
    expect(favor.count()).toBe(0);
  });

  it('reads who has it when it follows a session, and counts them', async () => {
    const { client, favor } = setup();
    client.holders = [pens];
    favor.follow('camp');
    await vi.waitFor(() => expect(favor.count()).toBe(1));
    expect(client.reads).toEqual(['camp']);
    expect(favor.has('pens')).toBe(true);
    expect(favor.has('tor')).toBe(false);
    expect(favor.holders().get('pens')).toBe('Pensantus');
  });

  it('shows nothing when the read fails, and the next read brings it back', async () => {
    const { client, favor } = setup();
    client.holders = [pens];
    favor.follow('camp');
    await vi.waitFor(() => expect(favor.has('pens')).toBe(true));
    client.holders = failedRead;
    await favor.refresh();
    expect(favor.has('pens')).toBe(false);
    expect(favor.count()).toBe(0);
    client.holders = [pens];
    await favor.refresh();
    expect(favor.has('pens')).toBe(true);
    // A read that failed and came back is not a new grant.
    expect(favor.gained()).toBe('');
  });

  it('turns a burst of hints into one read behind the last one', async () => {
    const { client, favor } = setup();
    favor.follow('camp');
    await vi.waitFor(() => expect(client.reads).toHaveLength(1));
    client.reads.length = 0;
    client.holders = [pens];
    void favor.refresh();
    void favor.refresh();
    void favor.refresh();
    await vi.waitFor(() => expect(favor.has('pens')).toBe(true));
    expect(client.reads.length).toBeLessThanOrEqual(2);
  });

  it('forgets everything when the session stops, and a read that was on its way does not bring it back', async () => {
    const { client, favor } = setup();
    client.holders = [pens];
    favor.follow('camp');
    favor.stop();
    await Promise.resolve();
    await Promise.resolve();
    expect(favor.campaignId()).toBe('');
    expect(favor.count()).toBe(0);
  });

  it('says a character was given it when a later read has one more, never for the first read', async () => {
    const { client, favor } = setup();
    client.holders = [tor];
    favor.follow('camp');
    await vi.waitFor(() => expect(favor.has('tor')).toBe(true));
    expect(favor.gained()).toBe('');
    client.holders = [tor, pens];
    await favor.refresh();
    expect(favor.gained()).toBe('pens');
    favor.dismissGained();
    expect(favor.gained()).toBe('');
  });

  it('drops the notice when the character no longer has it', async () => {
    const { client, favor } = setup();
    favor.follow('camp');
    await vi.waitFor(() => expect(client.reads).toHaveLength(1));
    client.holders = [pens];
    await favor.refresh();
    expect(favor.gained()).toBe('pens');
    client.holders = [];
    await favor.refresh();
    expect(favor.gained()).toBe('');
  });

  describe('the grant', () => {
    it('sends the campaign, the character and a key, says who got it and reads again', async () => {
      const { client, favor } = setup();
      favor.follow('camp');
      await vi.waitFor(() => expect(client.reads).toHaveLength(1));
      client.holders = [pens];
      await favor.grant('pens', 'Pensantus');
      expect(client.grants).toHaveLength(1);
      expect(client.grants[0]).toMatchObject({ campaignId: 'camp', characterId: 'pens' });
      expect(client.grants[0].key).not.toBe('');
      expect(favor.note()).toEqual({
        characterId: 'pens',
        text: 'Pensantus ganhou Inspiração.',
        refused: false,
      });
      expect(favor.has('pens')).toBe(true);
    });

    it('does not send a second grant for the same character while one is on its way', async () => {
      const { client, favor } = setup();
      favor.follow('camp');
      const first = favor.grant('pens', 'Pensantus');
      const second = favor.grant('pens', 'Pensantus');
      await Promise.all([first, second]);
      expect(client.grants).toHaveLength(1);
    });

    it('says why it was refused, in the words of favor-errors, and reads again', async () => {
      const { client, favor } = setup();
      favor.follow('camp');
      client.grantError = favorRefused(FavorBlockedReason.ALREADY_HAS);
      client.holders = [pens];
      await favor.grant('pens', 'Pensantus');
      expect(favor.note()).toEqual({
        characterId: 'pens',
        text: 'Essa personagem já tem Inspiração.',
        refused: true,
      });
      expect(favor.has('pens')).toBe(true);
    });

    it('says "Só numa sessão aberta." when the session ended with the menu open', async () => {
      const { client, favor } = setup();
      favor.follow('camp');
      client.grantError = noOpenSession();
      await favor.grant('pens', 'Pensantus');
      expect(favor.note()?.text).toBe(FAVOR_NO_SESSION);
      expect(favor.note()?.refused).toBe(true);
    });

    it('says so when it fails for another reason', async () => {
      const { client, favor } = setup();
      favor.follow('camp');
      client.grantError = new Error('down');
      await favor.grant('pens', 'Pensantus');
      expect(favor.note()?.text).toBe(FAVOR_GRANT_FAILED);
    });

    it('says "Só numa sessão aberta." and sends nothing when no session is followed', async () => {
      const { client, favor } = setup();
      await favor.grant('pens', 'Pensantus');
      expect(client.grants).toEqual([]);
      expect(favor.note()?.text).toBe(FAVOR_NO_SESSION);
    });

    it('empties the line after a few seconds', async () => {
      const { favor } = setup();
      favor.follow('camp');
      await favor.grant('pens', 'Pensantus');
      vi.advanceTimersByTime(FAVOR_NOTE_MS);
      expect(favor.note()).toBeNull();
    });
  });
});
