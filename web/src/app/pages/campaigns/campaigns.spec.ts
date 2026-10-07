import { Injectable, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import { Campaign, Role, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CampaignsService } from '../../core/campaigns/campaigns.service';
import { OpenSessions } from '../../shell/live-notice/open-sessions';
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
  const liveCampaignIds = signal<ReadonlySet<string>>(new Set());

  beforeEach(() => {
    liveCampaignIds.set(new Set());
    TestBed.configureTestingModule({
      imports: [Campaigns],
      providers: [
        provideRouter([]),
        { provide: CampaignsService, useClass: FakeCampaignsService },
        { provide: OpenSessions, useValue: { liveCampaignIds: liveCampaignIds.asReadonly() } },
      ],
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
    // It invites both ways in: creating one, or an invite link.
    expect(el.textContent).toContain('Criar campanha');
    expect(el.textContent).toContain('link de convite');
  });

  it('lists campaigns with the role shown as a Mestre/Jogador tag', async () => {
    fake.listMyCampaignsResult = Promise.resolve({
      campaigns: [
        campaign('c1', 'Mirathel', Role.MASTER),
        campaign('c2', 'Segunda Mesa', Role.PLAYER),
      ],
    });
    const el = await render();

    expect(el.textContent).toContain('Mirathel');
    expect(el.textContent).toContain('Mestre');
    expect(el.textContent).toContain('Segunda Mesa');
    expect(el.textContent).toContain('Jogador');

    // Each row is one link, with the name and the role tag inside it.
    const links = Array.from(el.querySelectorAll('a[href]'));
    const first = links.find((a) => a.getAttribute('href') === '/campaigns/c1');
    expect(first?.textContent).toContain('Mirathel');
    expect(first?.querySelector('.mr-tag')?.textContent?.trim()).toBe('Mestre');
    expect(first?.textContent).toContain('XP por inimigos derrotados');
  });

  it('tags a campaign with an open session "Sessão ao vivo", next to the role (RN-06, E5-01)', async () => {
    fake.listMyCampaignsResult = Promise.resolve({
      campaigns: [
        campaign('c1', 'Mirathel', Role.PLAYER),
        campaign('c2', 'Estrada de Ossos', Role.MASTER),
      ],
    });
    liveCampaignIds.set(new Set(['c1']));
    const el = await render();

    const links = Array.from(el.querySelectorAll('a[href]'));
    const live = links.find((a) => a.getAttribute('href') === '/campaigns/c1');
    const quiet = links.find((a) => a.getAttribute('href') === '/campaigns/c2');
    expect(live?.textContent).toContain('Sessão ao vivo');
    expect(live?.textContent).toContain('Jogador');
    expect(quiet?.textContent).not.toContain('Sessão ao vivo');
  });

  it("shows a campaign that awaits the master's approval as such, not as jogador (MR-024)", async () => {
    fake.listMyCampaignsResult = Promise.resolve({
      campaigns: [{ ...campaign('c1', 'Mirathel', Role.PLAYER), awaitingApproval: true, diceMode: 1, dicePreference: 1 }],
    });
    const el = await render();
    expect(el.textContent).toContain('Esperando a aprovação do mestre');
    expect(el.querySelector('.mr-tag')?.textContent?.trim()).toBe('Pendente');
    expect(el.textContent).not.toContain('jogador');
    expect(el.textContent).not.toContain('Jogador');
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
    expect(navigateSpy).toHaveBeenCalledWith(['/campaigns', 'new-id']);
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
