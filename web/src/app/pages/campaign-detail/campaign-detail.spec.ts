import { Injectable, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';
import { of } from 'rxjs';

import { Campaign, Member, Role, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { AuthService, AuthState } from '../../core/auth/auth.service';
import { CampaignsService } from '../../core/campaigns/campaigns.service';
import { GalleryClient } from '../../core/images/gallery-client';
import { FakeGalleryClient } from '../../core/images/gallery-testing';
import { CampaignDetail } from './campaign-detail';
import {
  CampaignCharactersSource,
  CampaignCharactersVm,
} from './characters/campaign-characters.types';
import { GameSessionSource, GameSessionVm } from './game-session/game-session-card.types';

/** `CampaignCharacters` (the "Personagens" section) and `GameSessionCard`
 * (the master's "Sessão" card) are children of this page too — see
 * `campaign-detail.html`. Both need a provider or Angular DI throws. */
@Injectable()
class FakeCampaignCharactersSource {
  listCharactersResult: Promise<CampaignCharactersVm> = Promise.resolve({
    playerCharacters: [],
    npcs: [],
    hasLivingCharacter: false,
  });
  listCharacters(): Promise<CampaignCharactersVm> {
    return this.listCharactersResult;
  }
}

@Injectable()
class FakeGameSessionSource {
  getCurrentSessionResult: Promise<GameSessionVm | null> = Promise.resolve(null);
  getCurrentSession(): Promise<GameSessionVm | null> {
    return this.getCurrentSessionResult;
  }
}

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
  listPendingMembers(): Promise<{ members: [] }> {
    return Promise.resolve({ members: [] });
  }
}

/** Who is signed in: `u1` unless a test says otherwise. */
class FakeAuthService {
  readonly state = signal<AuthState>({
    status: 'signed-in',
    user: { id: 'u1', displayName: null },
    sessionExpiresAt: null,
  });
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
        { provide: CampaignCharactersSource, useClass: FakeCampaignCharactersSource },
        { provide: GameSessionSource, useClass: FakeGameSessionSource },
        { provide: GalleryClient, useClass: FakeGalleryClient },
        { provide: AuthService, useClass: FakeAuthService },
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
    expect(el.textContent).toContain('Você é mestre nesta campanha. XP por inimigos derrotados.');
    expect(el.textContent).toContain('Vinicius');
    // Never an e-mail, and never a bare "Sem nome": the fallback says the role.
    expect(el.textContent).toContain('Jogador sem nome');
    expect(el.textContent).not.toContain('Sem nome');
    expect(el.textContent).not.toContain('@');
  });

  it('marks the viewer\'s own row, and points them to "Meu perfil" while they have no name', async () => {
    configure();
    fake.getCampaignResult = Promise.resolve({
      campaign: campaign('camp-1', 'Mirathel', Role.MASTER),
    });
    fake.listMembersResult = Promise.resolve({
      members: [member('u1', '', Role.MASTER), member('u2', 'Vinicius', Role.PLAYER)],
    });

    const el = await render();
    const rows = Array.from(el.querySelectorAll('section[aria-labelledby="members-heading"] li'));
    expect(rows[0].textContent).toContain('Mestre sem nome');
    expect(rows[0].textContent).toContain('(você)');
    expect(rows[0].querySelector('a')?.getAttribute('href')).toBe('/perfil');
    // Somebody else's row: no "(você)" and no link to the viewer's profile.
    expect(rows[1].textContent).not.toContain('(você)');
    expect(rows[1].querySelector('a')).toBeNull();
  });

  it('shows "campanha não encontrada" for a not_found response, and never reveals why', async () => {
    configure();
    // ListMembers is never asked: the page loads the campaign first.
    fake.getCampaignResult = Promise.reject(new ConnectError('no such campaign', Code.NotFound));

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

  it('shows the "Personagens" section for everyone, and the "Sessão" card only for the master', async () => {
    configure();
    fake.getCampaignResult = Promise.resolve({
      campaign: campaign('camp-1', 'Mirathel', Role.MASTER),
    });
    fake.listMembersResult = Promise.resolve({ members: [] });

    const el = await render();
    expect(el.textContent).toContain('Personagens');
    expect(el.textContent).toContain('Sessão');
  });

  it('hides the "Sessão" card for a player', async () => {
    configure();
    fake.getCampaignResult = Promise.resolve({
      campaign: campaign('camp-1', 'Mirathel', Role.PLAYER),
    });
    fake.listMembersResult = Promise.resolve({ members: [] });

    const el = await render();
    expect(el.textContent).toContain('Personagens');
    expect(el.textContent).not.toContain('Iniciar sessão');
  });

  it('shows the "Galeria" panel only for the master (MR-019)', async () => {
    configure();
    fake.getCampaignResult = Promise.resolve({
      campaign: campaign('camp-1', 'Mirathel', Role.MASTER),
    });
    fake.listMembersResult = Promise.resolve({ members: [] });
    expect((await render()).textContent).toContain('Abrir galeria');

    TestBed.resetTestingModule();
    configure();
    fake.getCampaignResult = Promise.resolve({
      campaign: campaign('camp-1', 'Mirathel', Role.PLAYER),
    });
    fake.listMembersResult = Promise.resolve({ members: [] });
    const el = await render();
    expect(el.textContent).not.toContain('Galeria');
    expect((TestBed.inject(GalleryClient) as unknown as FakeGalleryClient).calls).toEqual([]);
  });

  it('a pending member sees the wait banner and their character, never the members (MR-024)', async () => {
    configure();
    fake.getCampaignResult = Promise.resolve({
      campaign: { ...campaign('camp-1', 'Mirathel', Role.PLAYER), awaitingApproval: true, diceMode: 1, dicePreference: 1 },
    });
    const listMembers = vi.spyOn(fake, 'listMembers');

    const el = await render();
    expect(el.querySelector('h1')?.textContent).toContain('Mirathel');
    expect(el.textContent).toContain('Esperando a aprovação do mestre');
    expect(el.textContent).toContain('Personagens');
    expect(el.textContent).not.toContain('Membros');
    expect(el.textContent).not.toContain('Convites');
    expect(listMembers).not.toHaveBeenCalled();
  });

  it('shows a generic error message for a non-not_found failure', async () => {
    configure();
    fake.getCampaignResult = Promise.reject(new ConnectError('down', Code.Unavailable));

    const el = await render();
    expect(el.querySelector('h1')?.textContent).not.toContain('não encontrada');
    expect(el.textContent).toContain('Tente de novo');
  });
});
