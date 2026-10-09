import type { ContestClient } from './contest-client';
import { GroupCheckState } from './group-check-state';
import { groupCheck } from './contest-testing';
import { create } from '@bufbuild/protobuf';
import { GroupCheckMemberViewSchema } from '../../../gen/meurpg/play/v1/contest_types_pb';

const api = (read: () => Promise<ReturnType<typeof groupCheck> | null>) =>
  ({ groupCheck: read }) as unknown as ContestClient;

describe('GroupCheckState', () => {
  it('reads the group check and the one member a player is sent: their own', async () => {
    const state = new GroupCheckState();
    const member = create(GroupCheckMemberViewSchema, { characterId: 'b', name: 'Brisa' });
    await state.load(api(() => Promise.resolve(groupCheck({ members: [member] }))), 'c');
    expect(state.view()?.id).toBe('gc1');
    expect(state.own()?.name).toBe('Brisa');
  });

  it('has none when the session has no group check', async () => {
    const state = new GroupCheckState();
    await state.load(api(() => Promise.resolve(null)), 'c');
    expect(state.view()).toBeNull();
    expect(state.own()).toBeNull();
  });

  it('drops a read that arrives after the answer of the own roll', async () => {
    const state = new GroupCheckState();
    let release: (v: ReturnType<typeof groupCheck>) => void = () => undefined;
    const slow = new Promise<ReturnType<typeof groupCheck>>((r) => (release = r));
    const read = state.load(api(() => slow), 'c');
    state.apply(groupCheck({ id: 'after' }));
    release(groupCheck({ id: 'before' }));
    await read;
    expect(state.view()?.id).toBe('after');
  });
});
