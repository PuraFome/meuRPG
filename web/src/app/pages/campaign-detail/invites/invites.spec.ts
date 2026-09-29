import { Injectable } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Code, ConnectError } from '@connectrpc/connect';

import { Invite, InviteState } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CampaignsService } from '../../../core/campaigns/campaigns.service';
import { CampaignInvites } from './invites';

@Injectable()
class FakeCampaignsService {
  listInvitesResult: Promise<{ invites: Invite[] }> = Promise.resolve({ invites: [] });
  readonly createInvite = vi.fn();
  readonly revokeInvite = vi.fn();

  listInvites(): Promise<{ invites: Invite[] }> {
    return this.listInvitesResult;
  }
}

function invite(id: string, state: InviteState, useCount = 0, maxUses = 1): Invite {
  return {
    id,
    state,
    useCount,
    maxUses,
    createdAt: undefined,
    expiresAt: undefined,
    revokedAt: undefined,
  } as Invite;
}

describe('CampaignInvites', () => {
  let fake: FakeCampaignsService;

  beforeEach(() => {
    TestBed.configureTestingModule({
      imports: [CampaignInvites],
      providers: [{ provide: CampaignsService, useClass: FakeCampaignsService }],
    });
    fake = TestBed.inject(CampaignsService) as unknown as FakeCampaignsService;
  });

  async function render(): Promise<{
    el: HTMLElement;
    fixture: ComponentFixture<CampaignInvites>;
  }> {
    const fixture = TestBed.createComponent(CampaignInvites);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, fixture };
  }

  it('lists invites with their Portuguese status (ativo/usado/expirado/revogado)', async () => {
    fake.listInvitesResult = Promise.resolve({
      invites: [
        invite('i1', InviteState.ACTIVE),
        invite('i2', InviteState.USED_UP, 1, 1),
        invite('i3', InviteState.EXPIRED),
        invite('i4', InviteState.REVOKED),
      ],
    });
    const { el } = await render();

    expect(el.textContent).toContain('ativo');
    expect(el.textContent).toContain('usado');
    expect(el.textContent).toContain('expirado');
    expect(el.textContent).toContain('revogado');
  });

  it('shows a message when listing invites fails (e.g. permission_denied)', async () => {
    fake.listInvitesResult = Promise.reject(new ConnectError('nope', Code.PermissionDenied));
    const { el } = await render();
    expect(el.textContent).toContain('Só o mestre');
  });

  it('creates an invite, reveals the link once, and prepends it to the list', async () => {
    fake.createInvite.mockResolvedValue({
      invite: invite('new-invite', InviteState.ACTIVE),
      token: 'sekret-token',
    });
    const { fixture } = await render();
    const instance = fixture.componentInstance;

    await instance['createInvite']();
    fixture.detectChanges();

    expect(fake.createInvite).toHaveBeenCalledWith('camp-1', 1, 7);
    const el = fixture.nativeElement as HTMLElement;
    expect(el.textContent).toContain('não será mostrado de novo');
    expect(el.textContent).toContain('/convite#t=sekret-token');
  });

  it('dismissing the revealed link clears it', async () => {
    fake.createInvite.mockResolvedValue({
      invite: invite('new-invite', InviteState.ACTIVE),
      token: 'tok',
    });
    const { fixture } = await render();
    const instance = fixture.componentInstance;
    await instance['createInvite']();
    fixture.detectChanges();

    instance['dismissReveal']();
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).textContent).not.toContain(
      'não será mostrado de novo',
    );
  });

  it('shows a clear message when creating an invite fails', async () => {
    fake.createInvite.mockRejectedValue(new ConnectError('bad', Code.InvalidArgument));
    const { fixture } = await render();
    await fixture.componentInstance['createInvite']();
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('1 a 20');
  });

  it('revokes an invite and updates its status in place', async () => {
    fake.listInvitesResult = Promise.resolve({ invites: [invite('i1', InviteState.ACTIVE)] });
    fake.revokeInvite.mockResolvedValue({ invite: invite('i1', InviteState.REVOKED) });
    const { fixture, el } = await render();

    const revokeButton = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Revogar'),
    ) as HTMLButtonElement;
    expect(revokeButton).toBeTruthy();
    revokeButton.click();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(fake.revokeInvite).toHaveBeenCalledWith('camp-1', 'i1');
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('revogado');
  });
});
