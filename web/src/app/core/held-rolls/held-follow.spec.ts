import { signal } from '@angular/core';

import type { HeldRollOffer } from '../../../gen/meurpg/play/v1/combat_pb';
import { HeldFollow } from './held-follow';
import { markAnsweredHere } from './held-roll-controller';
import { MASTER_ANSWERED_TEXT } from './held-vanish';
import { heldLucky } from './held-roll-testing';

function setup(shown: HeldRollOffer | null) {
  const offer = signal<HeldRollOffer | null>(shown);
  const held = { offer, ask: (o: HeldRollOffer | null | undefined) => offer.set(o ?? null) };
  return { offer, follow: new HeldFollow(held) };
}

describe('HeldFollow', () => {
  it('says the master answered when the question is no longer open', () => {
    const lucky = heldLucky([1], 5, { holdId: 'follow-1' });
    const { offer, follow } = setup(lucky);
    follow.see(lucky);
    expect(follow.vanish.notice()).toBe('');
    follow.see(null);
    expect(follow.vanish.notice()).toBe(MASTER_ANSWERED_TEXT);
    expect(offer()).toBeNull();
  });

  it('says nothing when this browser sent the answer', () => {
    const lucky = heldLucky([1], 5, { holdId: 'follow-2' });
    const { offer, follow } = setup(lucky);
    follow.see(lucky);
    markAnsweredHere(lucky);
    offer.set(null);
    follow.see(null);
    expect(follow.vanish.notice()).toBe('');
  });

  it('treats another hold on the roller as this question gone', () => {
    const mine = heldLucky([1], 5, { holdId: 'follow-3' });
    const other = heldLucky([1], 5, { holdId: 'follow-other' });
    const { offer, follow } = setup(mine);
    follow.see(mine);
    follow.see(other);
    expect(follow.vanish.notice()).toBe(MASTER_ANSWERED_TEXT);
    expect(offer()).toBeNull();
  });

  it('does nothing while the sheet shows no question', () => {
    const { follow } = setup(null);
    follow.see(heldLucky([1], 5, { holdId: 'follow-4' }));
    expect(follow.vanish.notice()).toBe('');
  });
});
