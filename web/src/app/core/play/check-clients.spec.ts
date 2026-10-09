import { TestBed } from '@angular/core/testing';

import { CONNECT_TRANSPORT } from '../connect/transport';
import { PuzzlesClient } from '../puzzles/puzzles-client';
import { TrapsClient } from '../traps/traps-client';
import { SceneClient } from './scene-client';

function stub<T extends object>(token: new (...args: never[]) => T, method: string) {
  TestBed.configureTestingModule({
    providers: [token, { provide: CONNECT_TRANSPORT, useValue: {} }],
  });
  const sent: Record<string, unknown>[] = [];
  const client = TestBed.inject(token);
  (client as unknown as { client: unknown }).client = {
    [method]: (req: Record<string, unknown>) => {
      sent.push(req);
      return Promise.resolve({ roll: {}, run: {} });
    },
  };
  return { client, sent };
}

describe('the rolls of a check send a pair of faces', () => {
  it('scene: one face, or both in d20Faces with no single roll', async () => {
    const { client, sent } = stub(SceneClient, 'rollSceneCheck');
    await client.roll('c', 'a', { face: 7 }, 'k1');
    await client.roll('c', 'a', { faces: [7, 15] }, 'k2');
    expect(sent[0]).toMatchObject({ roll: { case: 'd20Face', value: 7 }, d20Faces: [] });
    expect(sent[1]).toMatchObject({ roll: { case: undefined }, d20Faces: [7, 15] });
  });

  it('puzzle hint: the same', async () => {
    const { client, sent } = stub(PuzzlesClient, 'tryPuzzleHint');
    await client.tryHint('c', 'p', { faces: [3, 9] }, 'k');
    expect(sent[0]).toMatchObject({ d20Faces: [3, 9] });
  });

  it('trap search: a second face goes as the pair', async () => {
    const { client, sent } = stub(TrapsClient, 'searchForTraps');
    await client.search('c', 'perception', { face: 4 }, 'k1');
    await client.search('c', 'perception', { face: 4, face2: 12 }, 'k2');
    expect(sent[0]).toMatchObject({ roll: { case: 'd20Face', value: 4 }, d20Faces: [] });
    expect(sent[1]).toMatchObject({ roll: { case: undefined }, d20Faces: [4, 12] });
  });
});
