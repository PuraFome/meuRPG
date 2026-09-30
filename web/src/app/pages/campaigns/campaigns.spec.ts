import { Injectable } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import { Campaign, Role, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CampaignsService } from '../../core/campaigns/campaigns.service';
import { Campaigns } from './campaigns';

/** A stand-in for CampaignsService whose calls this spec fully controls. */
@Injectable()
class FakeCampaignsService {
  listMyCampaignsResult: Promise<{ campaigns: Campaign[] }> = Promise.resolve({ campaigns: [] });
  readonly createCampaign = vi.fn();

  listMyCampaigns(): Promise<{ campaigns: Campaign[] }> {
    return this.listMyCampaignsResult;
  }
}

function campaign(id: string, name: string, myRole: Role): Campaign {
  return { id, name, myRole, xpMode: XpMode.ENEMIES, createdAt: undefined } as Campaign;
}

describe('Campaigns', () => {
  let fake: FakeCampaignsService;
  let router: Router;

  beforeEach(() => {
    TestBed.configureTestingModule({
      imports: [Campaigns],
      providers: [provideRouter([]), { provide: CampaignsService, useClass: FakeCampaignsService }],
    });
    fake = TestBed.inject(CampaignsService) as unknown as FakeCampaignsService;
    router = TestBed.inject(Router);
  });

  async function render(): Promise<HTMLElement> {
    const fixture = TestBed.createComponent(Campaigns);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('has exactly one h1, "Minhas campanhas"', async () => {
    const el = await render();
    const headings = el.querySelectorAll('h1');
    expect(headings.length).toBe(1);
    expect(headings[0].textContent).toContain('Minhas campanhas');
  });

  it('shows an empty state when there are no campaigns', async () => {
    fake.listMyCampaignsResult = Promise.resolve({ campaigns: [] });
    const el = await render();
    expect(el.textContent).toContain('Você ainda não tem nenhuma campanha');
  });

  it('lists campaigns with the role shown as mestre/jogador', async () => {
    fake.listMyCampaignsResult = Promise.resolve({
      campaigns: [
        campaign('c1', 'Mirathel', Role.MASTER),
        campaign('c2', 'Segunda Mesa', Role.PLAYER),
      ],
    });
    const el = await render();

    expect(el.textContent).toContain('Mirathel');
    expect(el.textContent).toContain('mestre');
    expect(el.textContent).toContain('Segunda Mesa');
    expect(el.textContent).toContain('jogador');

    const links = Array.from(el.querySelectorAll('a[href]'));
    expect(links.some((a) => a.getAttribute('href') === '/campanhas/c1')).toBe(true);
  });

  it('shows a message when listing campaigns fails', async () => {
    fake.listMyCampaignsResult = Promise.reject(new ConnectError('down', Code.Unavailable));
    const el = await render();
    expect(el.textContent).toContain('Não foi possível');
  });

  it('requires a name and an XP mode before submitting', async () => {
    const el = await render();
    const button = el.querySelector('button[type="submit"]') as HTMLButtonElement;
    button.click();
    expect(fake.createCampaign).not.toHaveBeenCalled();
  });

  it('creates a campaign with the chosen name and XP mode, then navigates to it', async () => {
    fake.createCampaign.mockResolvedValue({
      campaign: campaign('new-id', 'Mirathel', Role.MASTER),
    });
    const navigateSpy = vi.spyOn(router, 'navigate').mockResolvedValue(true);

    const fixture = TestBed.createComponent(Campaigns);
    fixture.detectChanges();
    await fixture.whenStable();

    const instance = fixture.componentInstance;
    instance['form'].setValue({ name: 'Mirathel', xpMode: XpMode.ENEMIES });
    await instance['submit']();

    expect(fake.createCampaign).toHaveBeenCalledWith('Mirathel', XpMode.ENEMIES);
    expect(navigateSpy).toHaveBeenCalledWith(['/campanhas', 'new-id']);
  });

  it('shows a clear message when creating a campaign fails with invalid_argument', async () => {
    fake.createCampaign.mockRejectedValue(new ConnectError('bad name', Code.InvalidArgument));

    const fixture = TestBed.createComponent(Campaigns);
    fixture.detectChanges();
    await fixture.whenStable();

    const instance = fixture.componentInstance;
    instance['form'].setValue({ name: 'x'.repeat(200), xpMode: XpMode.GOLD });
    await instance['submit']();
    fixture.detectChanges();

    expect((fixture.nativeElement as HTMLElement).textContent).toContain('1 a 80 caracteres');
  });
});
