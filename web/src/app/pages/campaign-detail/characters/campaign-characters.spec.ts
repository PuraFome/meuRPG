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
