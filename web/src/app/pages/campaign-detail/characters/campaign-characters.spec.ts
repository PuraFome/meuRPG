import { Injectable } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { CampaignCharacters } from './campaign-characters';
import { CampaignCharactersSource, CampaignCharactersVm } from './campaign-characters.types';

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

describe('CampaignCharacters', () => {
  let fake: FakeCampaignCharactersSource;

  beforeEach(() => {
    TestBed.configureTestingModule({
      imports: [CampaignCharacters],
      providers: [
        provideRouter([]),
        { provide: CampaignCharactersSource, useClass: FakeCampaignCharactersSource },
      ],
    });
    fake = TestBed.inject(CampaignCharactersSource) as unknown as FakeCampaignCharactersSource;
  });

  async function render(isMaster: boolean): Promise<{
    el: HTMLElement;
    fixture: ComponentFixture<CampaignCharacters>;
  }> {
    const fixture = TestBed.createComponent(CampaignCharacters);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('isMaster', isMaster);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, fixture };
  }

  it('shows the master\'s two lists and the "Novo NPC" menu trigger', async () => {
    fake.listCharactersResult = Promise.resolve({
      playerCharacters: [
        {
          id: 'c1',
          name: 'Pensantus',
          kind: 'player',
          state: 'draft',
          classSummary: 'Mago 3',
          playerDisplayName: 'Vinicius',
        },
      ],
      npcs: [
        { id: 'n1', name: 'Goblin', kind: 'minion', state: 'draft', classSummary: '', playerDisplayName: null },
      ],
      hasLivingCharacter: false,
    });

    const { el } = await render(true);

    expect(el.textContent).toContain('Personagens dos jogadores');
    expect(el.textContent).toContain('Pensantus');
    expect(el.textContent).toContain('Vinicius');
    expect(el.textContent).toContain('NPCs');
    expect(el.textContent).toContain('Goblin');
    expect(el.textContent).toContain('Novo NPC');
    expect(el.textContent).not.toContain('Criar meu personagem');
  });

  it('shows the master the characters waiting for approval, apart, each linking to its sheet (MR-024)', async () => {
    fake.listCharactersResult = Promise.resolve({
      playerCharacters: [
        { id: 'c1', name: 'Pensantus', kind: 'player', state: 'draft', classSummary: 'Mago 3', playerDisplayName: 'Vinicius' },
        { id: 'c2', name: 'Novata', kind: 'player', state: 'pending', classSummary: 'Ladina 1', playerDisplayName: 'Samuel' },
      ],
      npcs: [],
      hasLivingCharacter: false,
    });

    const { el } = await render(true);

    const heading = Array.from(el.querySelectorAll('h3')).find((h) => h.textContent?.includes('Esperando aprovação'));
    expect(heading).toBeTruthy();
    const queue = el.querySelector('ul[aria-labelledby="awaiting-approval-heading"]') as HTMLElement;
    expect(queue.textContent).toContain('Novata');
    expect(queue.textContent).toContain('Samuel');
    expect(queue.textContent).not.toContain('Pensantus');
    expect(queue.querySelector('a')?.getAttribute('href')).toBe('/campanhas/camp-1/personagens/c2');
  });

  it('shows no "Esperando aprovação" when nobody waits', async () => {
    fake.listCharactersResult = Promise.resolve({
      playerCharacters: [
        { id: 'c1', name: 'Pensantus', kind: 'player', state: 'draft', classSummary: 'Mago 3', playerDisplayName: 'Vinicius' },
      ],
      npcs: [],
      hasLivingCharacter: false,
    });
    const { el } = await render(true);
    expect(el.textContent).not.toContain('Esperando aprovação');
  });

  it('shows a pending player their character as "Pendente de aprovação", without the create button', async () => {
    fake.listCharactersResult = Promise.resolve({
      playerCharacters: [
        { id: 'c2', name: 'Novata', kind: 'player', state: 'pending', classSummary: 'Ladina 1', playerDisplayName: 'Samuel' },
      ],
      npcs: [],
      hasLivingCharacter: true,
    });
    const { el } = await render(false);
    expect(el.textContent).toContain('Meus personagens');
    expect(el.textContent).toContain('Novata');
    expect(el.textContent).toContain('Pendente de aprovação');
    expect(el.textContent).not.toContain('Esperando aprovação');
    expect(el.textContent).not.toContain('Criar meu personagem');
  });

  it('shows "Criar meu personagem" for a player with no living character', async () => {
    fake.listCharactersResult = Promise.resolve({
      playerCharacters: [],
      npcs: [],
      hasLivingCharacter: false,
    });

    const { el } = await render(false);

    expect(el.textContent).toContain('Meus personagens');
    expect(el.textContent).toContain('Criar meu personagem');
    expect(el.textContent).not.toContain('NPCs');
  });

  it('hides "Criar meu personagem" once the player has a living character', async () => {
    fake.listCharactersResult = Promise.resolve({
      playerCharacters: [
        {
          id: 'c1',
          name: 'Pensantus',
          kind: 'player',
          state: 'draft',
          classSummary: 'Mago 3',
          playerDisplayName: null,
        },
      ],
      npcs: [],
      hasLivingCharacter: true,
    });

    const { el } = await render(false);

    expect(el.textContent).not.toContain('Criar meu personagem');
  });
});
