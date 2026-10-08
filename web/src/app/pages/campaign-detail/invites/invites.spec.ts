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

function invite(
  id: string,
  state: InviteState,
  useCount = 0,
  maxUses = 1,
  requiresApproval = false,
): Invite {
  return {
    id,
    state,
    useCount,
    maxUses,
    requiresApproval,
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

  it('lists invites with their Portuguese status as tags (Ativo/Usado/Expirado/Revogado)', async () => {
    fake.listInvitesResult = Promise.resolve({
      invites: [
        invite('i1', InviteState.ACTIVE),
        invite('i2', InviteState.USED_UP, 1, 1),
        invite('i3', InviteState.EXPIRED),
        invite('i4', InviteState.REVOKED),
      ],
    });
    const { el } = await render();

    const tags = Array.from(el.querySelectorAll('.mr-tag')).map((t) => t.textContent?.trim());
    expect(tags).toEqual(['Ativo', 'Usado', 'Expirado', 'Revogado']);
    // A list of rows, not a table, with the uses spelled out.
    expect(el.querySelector('table')).toBeNull();
    expect(el.querySelector('ul[aria-label="Convites gerados"]')).toBeTruthy();
    expect(el.textContent).toContain('1 de 1 uso');
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

    expect(fake.createInvite).toHaveBeenCalledWith('camp-1', 1, 7, false);
    const el = fixture.nativeElement as HTMLElement;
    expect(el.textContent).toContain('não será mostrado de novo');
    expect(el.textContent).toContain('/invite#t=sekret-token');
  });

  it('"Exigir aprovação do mestre" creates an invite with approval (MR-024)', async () => {
    fake.createInvite.mockResolvedValue({
      invite: invite('approval-invite', InviteState.ACTIVE, 0, 1, true),
      token: 'tok',
    });
    const { fixture, el } = await render();

    const checkbox = Array.from(el.querySelectorAll('mat-checkbox')).find((c) =>
      c.textContent?.includes('Exigir aprovação do mestre'),
    );
    expect(checkbox).toBeTruthy();
    (checkbox!.querySelector('input[type="checkbox"]') as HTMLInputElement).click();
    fixture.detectChanges();

    await fixture.componentInstance['createInvite']();
    fixture.detectChanges();

    expect(fake.createInvite).toHaveBeenCalledWith('camp-1', 1, 7, true);
    const text = (fixture.nativeElement as HTMLElement).textContent;
    expect(text).toContain('só entra na campanha depois que você aprovar');
    expect(text).toContain('Exige aprovação do mestre');
    // The form goes back to the default: the next invite needs no approval.
    expect(fixture.componentInstance['form'].getRawValue().requiresApproval).toBe(false);
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

  it('tells the master the campaign has the most invites that work when the cap refuses', async () => {
    fake.createInvite.mockRejectedValue(new ConnectError('full', Code.ResourceExhausted));
    const { fixture } = await render();
    await fixture.componentInstance['createInvite']();
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('50 convites valendo');
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
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('Revogado');
    // Nothing left to revoke.
    const buttons = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('button'));
    expect(buttons.some((b) => b.textContent?.includes('Revogar'))).toBe(false);
  });

  it('"Copiar link" copies the revealed link and says so', async () => {
    fake.createInvite.mockResolvedValue({
      invite: invite('new-invite', InviteState.ACTIVE),
      token: 'tok',
    });
    const writeText = vi.fn().mockResolvedValue(undefined);
    const original = Object.getOwnPropertyDescriptor(navigator, 'clipboard');
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    try {
      const { fixture } = await render();
      await fixture.componentInstance['createInvite']();
      fixture.detectChanges();

      const el = fixture.nativeElement as HTMLElement;
      const link = el.querySelector('.invite-reveal__link')?.textContent?.trim();
      expect(link).toMatch(/\/invite#t=tok$/);
      const copy = Array.from(el.querySelectorAll('button')).find((b) =>
        b.textContent?.includes('Copiar link'),
      ) as HTMLButtonElement;
      copy.click();
      await fixture.whenStable();
      fixture.detectChanges();

      expect(writeText).toHaveBeenCalledWith(link);
      expect(el.textContent).toContain('Link copiado.');
    } finally {
      if (original) {
        Object.defineProperty(navigator, 'clipboard', original);
      } else {
        delete (navigator as { clipboard?: unknown }).clipboard;
      }
    }
  });
});
