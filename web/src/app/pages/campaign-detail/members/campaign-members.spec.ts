import { Injectable } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { settle } from '../../../../test-settle';
import { CampaignsService } from '../../../core/campaigns/campaigns.service';
import type { MemberRowVm } from '../campaign-detail.copy';
import { CampaignRoster } from '../characters/campaign-roster';
import { CampaignCharactersSource } from '../characters/campaign-characters.types';
import { endPhone, startPhone } from '../phone-testing';
import { CampaignMembers } from './campaign-members';

const row = (userId: string, name: string, role = 'jogador'): MemberRowVm => ({
  userId,
  name,
  role,
  isViewer: userId === 'u1',
  hasName: name !== 'Sem nome no perfil',
  dicePreference: 0,
});

@Injectable()
class FakeCampaigns {
  listPendingMembers() {
    return Promise.resolve({ members: [] });
  }
}

describe('CampaignMembers (board C2-01, C6-04)', () => {
  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        CampaignRoster,
        { provide: CampaignsService, useClass: FakeCampaigns },
        {
          provide: CampaignCharactersSource,
          useValue: {
            listCharacters: () =>
              Promise.resolve({
                playerCharacters: [
                  {
                    id: 'b',
                    name: 'Brisa',
                    kind: 'player',
                    state: 'pending',
                    classSummary: 'Barda 1',
                    playerDisplayName: 'Bia',
                  },
                ],
                npcs: [],
                hasLivingCharacter: false,
              }),
            openChoices: () => Promise.resolve([]),
          },
        },
      ],
    });
  });
  afterEach(() => endPhone());

  async function render(isMaster = true) {
    const roster = TestBed.inject(CampaignRoster);
    await roster.ensureLoaded('camp-1');
    const fixture = TestBed.createComponent(CampaignMembers);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('campaignName', 'Mirathel');
    fixture.componentRef.setInput('isMaster', isMaster);
    fixture.componentRef.setInput('members', [
      row('u1', 'Mestre Teste', 'mestre'),
      row('u2', 'Ana'),
      row('u3', 'Bia'),
    ]);
    await settle(fixture, 2);
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('lists the people with their first letter in a circle, name and role, under "3 na mesa"', async () => {
    const { el } = await render();
    expect(el.querySelector('h2')?.textContent?.trim()).toBe('Membros');
    expect(el.querySelector('.head__count')?.textContent).toBe('3 na mesa');
    const rows = Array.from(el.querySelectorAll('li.member'));
    expect(rows.map((r) => r.querySelector('.member__avatar')?.textContent)).toEqual([
      'M',
      'A',
      'B',
    ]);
    expect(rows.map((r) => r.querySelector('.member__role')?.textContent)).toEqual([
      'mestre',
      'jogador',
      'jogador',
    ]);
  });

  it('marks "Pendente" the player whose character waits for the master (the master sees it)', async () => {
    const { el } = await render();
    const rows = Array.from(el.querySelectorAll('li.member'));
    expect(rows[2].querySelector('.mr-tag')?.textContent).toContain('Pendente');
    expect(rows[1].querySelector('.mr-tag')).toBeNull();
  });

  it('marks nobody to a player', async () => {
    const { el } = await render(false);
    expect(el.querySelector('.mr-tag')).toBeNull();
  });

  describe('on a phone', () => {
    it('folds to "Membros · 3 na mesa" with "1 pendente", and opens in place', async () => {
      startPhone();
      const { fixture, el } = await render();
      const button = el.querySelector<HTMLButtonElement>('h2 button')!;
      expect(button.textContent?.replace(/\s+/g, ' ').trim()).toBe('Membros · 3 na mesa');
      expect(button.getAttribute('aria-expanded')).toBe('false');
      expect(el.querySelector('app-panel-head .mr-tag')?.textContent).toContain('1 pendente');
      expect(el.querySelectorAll('li.member')).toHaveLength(0);

      button.click();
      await settle(fixture);
      expect(el.querySelectorAll('li.member')).toHaveLength(3);
      expect(el.querySelector('app-panel-head .mr-tag')).toBeNull();
    });
  });
});
