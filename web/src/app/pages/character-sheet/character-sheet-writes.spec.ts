import { Code, ConnectError } from '@connectrpc/connect';

import { type CharacterSheetSource, type CharacterSheetVm } from './character-sheet.types';
import { SheetWrites, type SheetWritesHost, playerNameOf } from './character-sheet-writes';

const vmOf = (over: Partial<CharacterSheetVm> = {}) =>
  ({ id: 'c1', playerDisplayName: 'Lia', ...over }) as CharacterSheetVm;

function setup(source: Partial<CharacterSheetSource> = {}, apply = true) {
  const host = {
    source: source as CharacterSheetSource,
    navigate: vi.fn(() => Promise.resolve(true)),
    stillOn: vi.fn(() => apply),
    apply: vi.fn(() => apply),
    reload: vi.fn(),
    focus: vi.fn(),
  } satisfies SheetWritesHost;
  return { host, writes: new SheetWrites(host) };
}

describe('SheetWrites', () => {
  it('names the player, or "o jogador" while there is none', () => {
    expect(playerNameOf(vmOf())).toBe('Lia');
    expect(playerNameOf(vmOf({ playerDisplayName: null }))).toBe('o jogador');
  });

  it('asks twice before a death or a rejection, and puts the focus on the replacement', () => {
    const { host, writes } = setup();
    writes.askToConfirmDeath();
    expect(writes.confirmingDeath()).toBe(true);
    expect(host.focus).toHaveBeenLastCalledWith('.js-confirm-death');
    writes.cancelDeath();
    expect(writes.confirmingDeath()).toBe(false);
    expect(host.focus).toHaveBeenLastCalledWith('.js-mark-dead');
    writes.askToConfirmReject();
    expect(host.focus).toHaveBeenLastCalledWith('.js-cancel-reject');
    writes.cancelReject();
    expect(host.focus).toHaveBeenLastCalledWith('.js-reject');
  });

  it('opens the request for changes in place of the approval, and closes it', () => {
    const { host, writes } = setup();
    writes.confirmingReject.set(true);
    writes.announcement.set('x');
    writes.askForChanges();
    expect(writes.requestingChanges()).toBe(true);
    expect(writes.confirmingReject()).toBe(false);
    expect(writes.announcement()).toBe('');
    writes.cancelRequestChanges();
    expect(writes.requestingChanges()).toBe(false);
    expect(host.focus).toHaveBeenLastCalledWith('.js-ask-changes');
  });

  it('brings a character back and says so', async () => {
    const reviveCharacter = vi.fn(() => Promise.resolve(vmOf()));
    const { writes } = setup({ reviveCharacter });
    writes.askToRevive();
    await writes.revive('camp', 'c1', 'Toren');
    expect(reviveCharacter).toHaveBeenCalledWith('camp', 'c1', expect.any(String));
    expect(writes.confirmingRevive()).toBe(false);
    expect(writes.reviveAnnouncement()).toBe('Toren voltou à vida');
  });

  it('reads the character again when a revival fails for another reason', async () => {
    const reviveCharacter = vi.fn(() => Promise.reject(new ConnectError('gone', Code.NotFound)));
    const { host, writes } = setup({ reviveCharacter });
    await writes.revive('camp', 'c1', 'Toren');
    expect(writes.reviveState().status).toBe('error');
    expect(host.reload).toHaveBeenCalled();
  });

  it('leaves the page alone when the answer of a write is for a character it left', async () => {
    const approveCharacter = vi.fn(() => Promise.resolve(vmOf()));
    const { writes } = setup({ approveCharacter }, false);
    await writes.approve('camp', 'c1');
    expect(writes.approvalState().status).toBe('saving');
  });

  it('goes back to the campaign after a rejection, unless the page left the character', async () => {
    const rejectCharacter = vi.fn(() => Promise.resolve());
    const first = setup({ rejectCharacter });
    await first.writes.reject('camp', 'c1');
    expect(first.host.navigate).toHaveBeenCalledWith(['/campaigns', 'camp']);
    const gone = setup({ rejectCharacter }, false);
    await gone.writes.reject('camp', 'c1');
    expect(gone.host.navigate).not.toHaveBeenCalled();
  });

  it('forgets everything open when another character comes', () => {
    const { writes } = setup();
    writes.confirmingDeath.set(true);
    writes.resubmitted.set(true);
    writes.reviveAnnouncement.set('x');
    writes.approvalState.set({ status: 'saving' });
    writes.reset();
    expect(writes.confirmingDeath()).toBe(false);
    expect(writes.resubmitted()).toBe(false);
    expect(writes.reviveAnnouncement()).toBe('');
    expect(writes.approvalState().status).toBe('idle');
  });
});
