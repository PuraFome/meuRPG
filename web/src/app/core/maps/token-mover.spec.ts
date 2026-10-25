import { Code, ConnectError } from '@connectrpc/connect';

import type { GetMapResponse } from '../../../gen/meurpg/maps/v1/maps_pb';
import { MapMoveStop } from '../../../gen/meurpg/maps/v1/maps_pb';
import { FakeTokenMoveClient } from './token-move-testing';
import { MapState } from './map-state';
import { mapMessage, mapToken } from './maps-testing';
import { TokenMover } from './token-mover';
import { centerOf } from './token-move';

const GRID = { columns: 24, rows: 16 };

async function setup() {
  const api = new FakeTokenMoveClient();
  const at = (col: number, row: number) => centerOf(col, row, GRID);
  const state = new MapState(
    async () =>
      ({
        map: mapMessage('m1', 'Cripta', { gridColumns: 24, gridRows: 16 }),
        points: [],
        tokens: [
          mapToken('tor', 'Tor', { mapId: 'm1', ...at(5, 5) }),
          mapToken('pensantus', 'Pensantus', { mapId: 'm1', ...at(2, 2) }),
          mapToken('brisa', 'Brisa', { mapId: 'm1', ...at(3, 2) }),
          mapToken('tor', 'Goblin 1', { mapId: 'm1', ...at(9, 9), creatureId: 'cr1' }),
        ],
      }) as unknown as GetMapResponse,
  );
  await state.open('m1');
  const mover = new TokenMover(api, state, () => 'c1');
  return { api, state, mover, at };
}

describe('TokenMover', () => {
  it("chooses one token, or a group when it adds; a creature's token is never a character to choose", async () => {
    const { mover } = await setup();
    mover.pick('tor');
    expect(mover.selected()).toEqual(['tor']);
    mover.pick('tor');
    expect(mover.selected()).toEqual([]);
    mover.pick('tor');
    mover.pick('brisa', true);
    expect(mover.selected()).toEqual(['tor', 'brisa']);
    mover.setSelecting(true);
    mover.pick('pensantus');
    expect(mover.selected()).toEqual(['tor', 'brisa', 'pensantus']);
    mover.pick('brisa');
    expect(mover.selected()).toEqual(['tor', 'pensantus']);
    mover.pick('ninguem');
    expect(mover.selected()).toEqual(['tor', 'pensantus']);
    mover.clear();
    expect(mover.selected()).toEqual([]);
    expect(mover.characters().map((t) => t.name)).toEqual(['Tor', 'Pensantus', 'Brisa']);
  });

  it('sets the destination by a direction and the squares a click, and by a tap on the map', async () => {
    const { mover, at } = await setup();
    mover.pick('tor');
    mover.choose('e');
    mover.setSteps(3);
    expect(mover.target()).toEqual(at(8, 5));
    expect(mover.summary()?.title.replace(/ /g, ' ')).toBe('Tor vai 3 quadrados para a direita');
    expect(mover.summary()?.detail.replace(/\u00a0/g, ' ')).toBe('Distância: 4,5 m');
    mover.setSteps(99);
    expect(mover.steps()).toBe(20);
    mover.tap({ xBp: at(10, 5).xBp + 10, yBp: at(10, 5).yBp });
    expect(mover.target()).toEqual(at(10, 5));
    expect(mover.direction()).toBeNull();
  });

  it('moves the chosen token with the answer, and the token stays put until the server answers', async () => {
    const { api, state, mover, at } = await setup();
    mover.pick('tor');
    mover.choose('e');
    mover.setSteps(5);
    const before = state.tokens().find((t) => t.name === 'Tor' && !t.creatureId)!;
    api.moveOutcome = {
      moved: true,
      blocked: '',
      results: [
        {
          characterId: 'tor',
          name: 'Tor',
          ...at(10, 5),
          asked: 5,
          moved: 5,
          stopped: MapMoveStop.UNSPECIFIED,
        },
      ],
    };
    const moving = mover.move();
    expect(mover.busy()).toBe(true);
    expect(state.tokens().find((t) => t.name === 'Tor' && !t.creatureId)).toBe(before);
    expect(await moving).toBe(true);
    expect(api.calls).toContain(`moveTokens m1 tor ${at(10, 5).xBp} ${at(10, 5).yBp}`);
    expect(state.tokens().find((t) => t.name === 'Tor' && !t.creatureId)).toMatchObject(at(10, 5));
    expect(mover.last()?.notice.text).toBe('Tor chegou ao destino.');
    expect(mover.last()?.lines[0].replace(/ /g, ' ')).toBe(
      'Pedido: 5 quadrados (7,5 m) · andou 5 (7,5 m)',
    );
    expect(mover.target()).toBeNull();
    expect(mover.busy()).toBe(false);
  });

  it('keeps the line and the stop label when the token stopped before the wall', async () => {
    const { api, mover, at } = await setup();
    mover.pick('tor');
    mover.choose('e');
    mover.setSteps(10);
    api.moveOutcome = {
      moved: true,
      blocked: '',
      results: [
        {
          characterId: 'tor',
          name: 'Tor',
          ...at(10, 5),
          asked: 10,
          moved: 5,
          stopped: MapMoveStop.NO_PASS,
        },
      ],
    };
    await mover.move();
    expect(mover.last()?.notice.tone).toBe('stopped');
    expect(mover.trail()?.to).toEqual(at(10, 5));
    expect(mover.trail()?.stop?.label.replace(/ /g, ' ')).toBe('7,5 m · parou antes');
    expect(mover.trail()?.stop?.at).toEqual(at(15, 5));
  });

  it('moves a group around the first, and a drag of one chosen token takes the others along', async () => {
    const { api, mover, at } = await setup();
    mover.pick('tor');
    mover.pick('pensantus', true);
    mover.pick('brisa', true);
    mover.tap(at(8, 8));
    await mover.move();
    expect(api.calls.at(-1)).toBe(
      `moveTokens m1 tor,pensantus,brisa ${at(8, 8).xBp} ${at(8, 8).yBp}`,
    );
    await mover.drag('brisa', at(12, 12));
    expect(api.calls.at(-1)).toBe(
      `moveTokens m1 brisa,tor,pensantus ${at(12, 12).xBp} ${at(12, 12).yBp}`,
    );
    mover.clear();
    await mover.drag('tor', at(1, 1));
    expect(api.calls.at(-1)).toBe(`moveTokens m1 tor ${at(1, 1).xBp} ${at(1, 1).yBp}`);
  });

  it('says nobody moved when the group does not fit, and leaves every token where it was', async () => {
    const { api, state, mover, at } = await setup();
    mover.pick('tor');
    mover.pick('brisa', true);
    mover.tap(at(8, 8));
    const before = state.tokens().map((t) => [t.xBp, t.yBp]);
    api.moveOutcome = {
      moved: false,
      blocked: 'brisa',
      results: [
        {
          characterId: 'tor',
          name: 'Tor',
          ...at(5, 5),
          asked: 3,
          moved: 0,
          stopped: MapMoveStop.NO_PASS,
        },
        {
          characterId: 'brisa',
          name: 'Brisa',
          ...at(3, 2),
          asked: 3,
          moved: 0,
          stopped: MapMoveStop.NO_PASS,
        },
      ],
    };
    expect(await mover.move()).toBe(false);
    expect(mover.last()?.notice.lead).toBe('Ninguém se moveu.');
    expect(mover.trail()).toBeNull();
    expect(state.tokens().map((t) => [t.xBp, t.yBp])).toEqual(before);
  });

  it('writes a refusal and the token stays: one move at a time, and no destination, no move', async () => {
    const { api, mover, at } = await setup();
    expect(await mover.move()).toBe(false);
    mover.pick('tor');
    mover.tap(at(8, 8));
    api.failWith = new ConnectError('x', Code.NotFound);
    expect(await mover.move()).toBe(false);
    expect(mover.error()).toBe(
      'Não deu para mover Tor. Ele não está mais neste mapa. O token voltou para onde estava.',
    );
    expect(mover.busy()).toBe(false);
  });
});
