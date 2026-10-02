import { Code, ConnectError } from '@connectrpc/connect';

import { MapReveals } from './map-reveals';
import { MapState } from './map-state';
import { FakeMapsClient, mapMessage, mapPoint, mapResponse, mapToken } from './maps-testing';

async function setup() {
  const api = new FakeMapsClient();
  const state = new MapState(async () =>
    mapResponse(mapMessage('map-1', 'Mirathel'), [mapPoint('p1', 'Torre')], [mapToken('t1', 'Goblin', { hidden: true })]),
  );
  await state.open('map-1');
  const reveals = new MapReveals(api as never, () => state, () => 'camp-1');
  return { api, state, reveals };
}

describe('MapReveals', () => {
  it('reveals a point and puts the answer into the map', async () => {
    const { api, state, reveals } = await setup();
    await reveals.togglePoint(state.points()[0], true);
    expect(api.calls).toContain('setPointRevealed map-1 p1 true');
    expect(state.points()[0].revealed).toBe(true);
    expect(reveals.announcement()).toBe('Torre foi revelado aos jogadores.');
    expect(reveals.pendingId()).toBeNull();
  });

  it('shows a hidden token to the players', async () => {
    const { api, state, reveals } = await setup();
    await reveals.toggleToken(state.tokens()[0], false);
    expect(api.calls).toContain('setTokenHidden map-1 t1 false');
    expect(state.tokens()[0].hidden).toBe(false);
  });

  it('keeps the state and says what happened when the call fails', async () => {
    const { api, state, reveals } = await setup();
    api.failWith = new ConnectError('no', Code.PermissionDenied);
    await reveals.togglePoint(state.points()[0], true);
    expect(state.points()[0].revealed).toBe(false);
    expect(reveals.error()).toContain('Só o mestre');
    expect(reveals.pendingId()).toBeNull();
  });
});
