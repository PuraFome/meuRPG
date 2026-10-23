import { Code, ConnectError } from '@connectrpc/connect';
import { signal } from '@angular/core';

import { CombatState } from '../../../core/combat/combat-state';
import { encounter, spellZone } from '../../../core/combat/combat-testing';
import { fakeCombatClient } from '../../../core/combat/combat-client-testing';
import { ZoneFlow } from './zone-flow';

function setup(endSpellZone: () => Promise<unknown>, master = true) {
  const state = new CombatState();
  state.apply(encounter({ id: 'e1', zones: [spellZone({ id: 'z1' })] }));
  const busy = signal(false);
  const refreshAfter = vi.fn(async () => undefined);
  const flow = new ZoneFlow({
    api: fakeCombatClient({ endSpellZone: vi.fn(endSpellZone) }),
    calls: { busy, refreshAfter },
    campaignId: () => 'c1',
    encounter: () => state.shown(),
    state: () => state,
    isMaster: () => master,
  });
  return { flow, state, busy, refreshAfter };
}

describe('ZoneFlow', () => {
  it('reads the zones of the combat for the master or a player', () => {
    expect(setup(async () => encounter({})).flow.views().map((v) => v.id)).toEqual(['z1']);
    expect(setup(async () => encounter({}), false).flow.views()[0].canEnd).toBe(false);
  });

  it('ends a zone, applies the combat the server answers and is busy only meanwhile', async () => {
    let during = false;
    const next = encounter({ id: 'e1', revision: 9, zones: [] });
    const { flow, state, busy } = setup(async () => {
      during = busy();
      return next;
    });
    expect(await flow.end('z1')).toBe(true);
    expect(during).toBe(true);
    expect(busy()).toBe(false);
    expect(flow.views()).toEqual([]);
    expect(state.shown()?.revision).toBe(9);
  });

  it('says a refusal in the words of the end, and reads the combat again when it was stale', async () => {
    const { flow, refreshAfter } = setup(async () => {
      throw new ConnectError('gone', Code.NotFound);
    });
    expect(await flow.end('z1')).toBe(false);
    expect(flow.error()).toContain('não existe mais');
    expect(refreshAfter).toHaveBeenCalled();
    // the next try starts clean
    expect(flow.error()).not.toBe('');
  });

  it('makes no call while another is in the air', async () => {
    const call = vi.fn(async () => encounter({}));
    const { flow, busy } = setup(call);
    busy.set(true);
    expect(await flow.end('z1')).toBe(false);
    expect(call).not.toHaveBeenCalled();
  });

  it('keeps the zone the master asked to move, for the picker that reads it', () => {
    const { flow } = setup(async () => encounter({}));
    expect(flow.moving()).toBeNull();
    flow.moving.set('z1');
    expect(flow.moving()).toBe('z1');
  });
});
