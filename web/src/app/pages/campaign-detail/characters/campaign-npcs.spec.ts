import { Injectable } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { nextTurn } from '../../../../test-settle';
import { CampaignNpcs } from './campaign-npcs';
import { CampaignRoster } from './campaign-roster';
import {
  CampaignCharacterListItemVm,
  CampaignCharactersSource,
  CampaignCharactersVm,
} from './campaign-characters.types';

const npc = (id: string, name: string, extra: Partial<CampaignCharacterListItemVm> = {}) =>
  ({
    id,
    name,
    kind: 'story',
    state: 'draft',
    classSummary: '',
    playerDisplayName: null,
    ...extra,
  }) as CampaignCharacterListItemVm;

@Injectable()
class FakeSource {
  vm: CampaignCharactersVm = { playerCharacters: [], npcs: [], hasLivingCharacter: false };
  calls = 0;

  listCharacters(): Promise<CampaignCharactersVm> {
    this.calls++;
    return Promise.resolve(this.vm);
  }

  openChoices(): Promise<never[]> {
    return Promise.resolve([]);
  }
}

describe('CampaignNpcs', () => {
  let source: FakeSource;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        CampaignRoster,
        { provide: CampaignCharactersSource, useClass: FakeSource },
      ],
    });
    source = TestBed.inject(CampaignCharactersSource) as unknown as FakeSource;
  });

  async function render() {
    const fixture = TestBed.createComponent(CampaignNpcs);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.detectChanges();
    await fixture.whenStable();
    await nextTurn();
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('invites the first NPC, with the "Novo NPC" menu in the title row', async () => {
    const { el } = await render();
    expect(el.querySelector('h2')?.textContent).toContain('NPCs');
    expect(el.textContent).toContain('Nenhum NPC ainda');
    expect(el.querySelector('.npcs__new')?.textContent).toContain('Novo NPC');
  });

  it('lists the NPCs with their initials and a link to each sheet, and counts them', async () => {
    source.vm = {
      playerCharacters: [],
      npcs: [npc('n1', 'Bartolo'), npc('n2', 'Capitão bandido', { classSummary: 'Líder' })],
      hasLivingCharacter: false,
    };
    const { el } = await render();
    expect(el.textContent).toContain('2 NPCs');
    const rows = Array.from(el.querySelectorAll('li a'));
    expect(rows.map((a) => a.querySelector('.row__initials')?.textContent)).toEqual(['B', 'CB']);
    expect(rows[0].getAttribute('href')).toBe('/campaigns/camp-1/characters/n1');
    expect(rows[0].textContent).toContain('NPC de história');
    expect(rows[1].textContent).toContain('Líder');
  });

  it('shows five NPCs and "Ver todos os 9 NPCs", which shows the rest and focuses the sixth', async () => {
    source.vm = {
      playerCharacters: [],
      npcs: Array.from({ length: 9 }, (_, i) => npc(`n${i}`, `NPC ${i}`)),
      hasLivingCharacter: false,
    };
    const { fixture, el } = await render();
    expect(el.querySelectorAll('li')).toHaveLength(5);
    const more = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Ver todos os 9 NPCs'),
    )!;
    more.click();
    await fixture.whenStable();
    await nextTurn();
    fixture.detectChanges();
    expect(el.querySelectorAll('li')).toHaveLength(9);
    expect(document.activeElement).toBe(el.querySelector('.js-first-extra'));
  });

  it('reads the roster once for the players and the NPCs', async () => {
    await render();
    await TestBed.inject(CampaignRoster).ensureLoaded('camp-1');
    expect(source.calls).toBe(1);
  });
});
