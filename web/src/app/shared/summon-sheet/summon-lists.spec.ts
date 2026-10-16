import { create } from '@bufbuild/protobuf';

import { SummonOptionSchema } from '../../../gen/meurpg/characters/v1/characters_pb';
import type { Creature } from '../../../gen/meurpg/rules/v1/rules_pb';
import type { CreaturesClient } from '../../core/creatures/creatures-client';
import { nextTurn } from '../../../test-settle';
import { SummonLists } from './summon-lists';

const block = (key: string, armorClass = 12) =>
  ({ summary: { key, namePt: key }, armorClass }) as unknown as Creature;

function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

describe('SummonLists', () => {
  it('keeps the forms that can be read and drops the ones that cannot', async () => {
    const client = {
      statBlock: (_c: string, key: string) =>
        key === 'monster:bad' ? Promise.reject(new Error('no')) : Promise.resolve(block(key)),
    } as unknown as CreaturesClient;
    const lists = new SummonLists(client, 'camp', () => undefined);
    lists.load(
      create(SummonOptionSchema, {
        forms: [{ monsterKey: 'monster:cat' }, { monsterKey: 'monster:bad' }],
      }),
    );
    await nextTurn();
    expect(lists.forms()?.map((c) => c.summary?.key)).toEqual(['monster:cat']);
  });

  it('says a failed search in the sheet and leaves an empty list', async () => {
    const fail = vi.fn();
    const client = {
      search: () => Promise.reject(new Error('down')),
    } as unknown as CreaturesClient;
    const lists = new SummonLists(client, 'camp', fail);
    lists.load(create(SummonOptionSchema, { type: 'beast', maxCr: '1' }));
    await nextTurn();
    expect(lists.beasts()).toEqual([]);
    expect(fail).toHaveBeenCalledTimes(1);
  });

  it('drops the answer of a search another option replaced', async () => {
    const first = deferred<{ creatures: { key: string }[] }>();
    const second = deferred<{ creatures: { key: string }[] }>();
    const answers = [first, second];
    const client = {
      search: () => answers.shift()!.promise,
      statBlock: (_c: string, key: string) => Promise.resolve(block(key)),
    } as unknown as CreaturesClient;
    const fail = vi.fn();
    const lists = new SummonLists(client, 'camp', fail);
    lists.load(create(SummonOptionSchema, { type: 'beast', maxCr: '1' }));
    lists.load(create(SummonOptionSchema, { type: 'beast', maxCr: '2' }));
    second.resolve({ creatures: [{ key: 'monster:wolf' }] });
    await nextTurn();
    first.resolve({ creatures: [{ key: 'monster:bat' }] });
    await nextTurn();
    expect(lists.beasts()?.map((b) => b.key)).toEqual(['monster:wolf']);
    expect([...lists.beastBlocks().keys()]).toEqual(['monster:wolf']);
    expect(fail).not.toHaveBeenCalled();
  });

  it('reads the armor class of each kind once, and ignores a block that cannot be read', async () => {
    const statBlock = vi.fn((_c: string, key: string) =>
      key === 'monster:bad' ? Promise.reject(new Error('no')) : Promise.resolve(block(key, 14)),
    );
    const lists = new SummonLists(
      { statBlock } as unknown as CreaturesClient,
      'camp',
      () => undefined,
    );
    lists.readArmorClasses(['monster:wolf', 'monster:wolf', 'monster:bad']);
    await nextTurn();
    expect(statBlock).toHaveBeenCalledTimes(2);
    expect([...lists.armorClasses()]).toEqual([['monster:wolf', 14]]);
  });
});
