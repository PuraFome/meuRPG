import { Injectable } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';
import { of } from 'rxjs';

import { Campaign, Member, Role, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CampaignsService } from '../../core/campaigns/campaigns.service';
import { CampaignDetail } from './campaign-detail';

/** Covers both CampaignDetail's own calls and CampaignInvites' (the master
 * section it renders as a child), so this fake needs every method both use. */
@Injectable()
class FakeCampaignsService {
  getCampaignResult: Promise<{ campaign: Campaign | undefined }> = Promise.resolve({
    campaign: undefined,
  });
  listMembersResult: Promise<{ members: Member[] }> = Promise.resolve({ members: [] });
  listInvitesResult: Promise<{ invites: [] }> = Promise.resolve({ invites: [] });

  getCampaign(): Promise<{ campaign: Campaign | undefined }> {
    return this.getCampaignResult;
  }
  listMembers(): Promise<{ members: Member[] }> {
    return this.listMembersResult;
  }
  listInvites(): Promise<{ invites: [] }> {
    return this.listInvitesResult;
  }
}

function campaign(id: string, name: string, myRole: Role): Campaign {
  return { id, name, myRole, xpMode: XpMode.ENEMIES, createdAt: undefined } as Campaign;
}

function member(userId: string, displayName: string, role: Role): Member {
  return { userId, displayName, role, joinedAt: undefined } as Member;
}

function activatedRouteFor(id: string) {
  return { paramMap: of(convertToParamMap({ id })) };
}

/** Drains pending microtasks (the paramMap subscription and the
 * Promise.all().then() chain it kicks off both hop through a few) before
 * the next detectChanges() — more robust here than relying solely on
 * `fixture.whenStable()`, since the async work is not itself signal-driven
 * until the very end of the chain. */
function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

describe('CampaignDetail', () => {
  let fake: FakeCampaignsService;

  function configure(id = 'camp-1'): void {
    TestBed.configureTestingModule({
      imports: [CampaignDetail],
      providers: [
        { provide: CampaignsService, useClass: FakeCampaignsService },
        { provide: ActivatedRoute, useValue: activatedRouteFor(id) },
      ],
    });
    fake = TestBed.inject(CampaignsService) as unknown as FakeCampaignsService;
  }

  async function render(): Promise<HTMLElement> {
    const fixture = TestBed.createComponent(CampaignDetail);
    fixture.detectChanges();
    await flush();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('shows the campaign name as the only h1, and members with display names', async () => {
    configure();
    fake.getCampaignResult = Promise.resolve({
      campaign: campaign('camp-1', 'Mirathel', Role.MASTER),
    });
    fake.listMembersResult = Promise.resolve({
      members: [member('u1', 'Vinicius', Role.MASTER), member('u2', '', Role.PLAYER)],
    });

    const el = await render();
    const headings = el.querySelectorAll('h1');
    expect(headings.length).toBe(1);
    expect(headings[0].textContent).toContain('Mirathel');
    expect(el.textContent).toContain('Vinicius');
    // Never an e-mail; the neutral fallback for someone with no name yet.
    expect(el.textContent).toContain('Sem nome');
    expect(el.textContent).not.toContain('@');
  });

  it('shows "campanha não encontrada" for a not_found response, and never reveals why', async () => {
    configure();
    fake.getCampaignResult = Promise.reject(new ConnectError('no such campaign', Code.NotFound));
    fake.listMembersResult = Promise.reject(new ConnectError('no such campaign', Code.NotFound));

    const el = await render();
    expect(el.querySelector('h1')?.textContent).toContain('não encontrada');
  });

  it('shows the Convites section only for the master', async () => {
    configure();
    fake.getCampaignResult = Promise.resolve({
      campaign: campaign('camp-1', 'Mirathel', Role.MASTER),
    });
    fake.listMembersResult = Promise.resolve({ members: [] });

    const el = await render();
    expect(el.textContent).toContain('Convites');
  });

  it('hides the Convites section for a player', async () => {
    configure();
    fake.getCampaignResult = Promise.resolve({
      campaign: campaign('camp-1', 'Mirathel', Role.PLAYER),
    });
    fake.listMembersResult = Promise.resolve({ members: [] });

    const el = await render();
    expect(el.textContent).not.toContain('Convites');
  });

  it('shows a generic error message for a non-not_found failure', async () => {
    configure();
    fake.getCampaignResult = Promise.reject(new ConnectError('down', Code.Unavailable));
    fake.listMembersResult = Promise.reject(new ConnectError('down', Code.Unavailable));

    const el = await render();
    expect(el.querySelector('h1')?.textContent).not.toContain('não encontrada');
    expect(el.textContent).toContain('Tente de novo');
  });
});
