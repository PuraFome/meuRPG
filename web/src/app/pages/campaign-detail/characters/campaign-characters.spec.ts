import { Injectable } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { ExperienceStore, type ExperienceRow } from '../../../core/progression/experience-store';
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
        {
          id: 'n1',
          name: 'Goblin',
          kind: 'minion',
          state: 'draft',
          classSummary: '',
          playerDisplayName: null,
        },
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
        {
          id: 'c1',
          name: 'Pensantus',
          kind: 'player',
          state: 'draft',
          classSummary: 'Mago 3',
          playerDisplayName: 'Vinicius',
        },
        {
          id: 'c2',
          name: 'Novata',
          kind: 'player',
          state: 'pending',
          classSummary: 'Ladina 1',
          playerDisplayName: 'Samuel',
        },
      ],
      npcs: [],
      hasLivingCharacter: false,
    });

    const { el } = await render(true);

    const heading = Array.from(el.querySelectorAll('h3')).find((h) =>
      h.textContent?.includes('Esperando aprovação'),
    );
    expect(heading).toBeTruthy();
    const queue = el.querySelector(
      'ul[aria-labelledby="awaiting-approval-heading"]',
    ) as HTMLElement;
    expect(queue.textContent).toContain('Novata');
    expect(queue.textContent).toContain('Samuel');
    expect(queue.textContent).not.toContain('Pensantus');
    expect(queue.querySelector('a')?.getAttribute('href')).toBe('/campaigns/camp-1/characters/c2');
  });

  it('tags the waiting row "Ajustes pedidos" once the master asked for changes, and no other row', async () => {
    const pending = (id: string, reviewStatus: 'awaiting' | 'changes_requested') => ({
      id,
      name: id,
      kind: 'player' as const,
      state: 'pending' as const,
      classSummary: 'Mago 1',
      playerDisplayName: 'Lia',
      reviewStatus,
    });
    fake.listCharactersResult = Promise.resolve({
      playerCharacters: [pending('Lyra', 'changes_requested'), pending('Novata', 'awaiting')],
      npcs: [],
      hasLivingCharacter: false,
    });

    const { el } = await render(true);

    const rows = Array.from(
      el.querySelectorAll('ul[aria-labelledby="awaiting-approval-heading"] li'),
    );
    expect(rows[0].textContent).toContain('Ajustes pedidos');
    expect(rows[1].textContent).not.toContain('Ajustes pedidos');
  });

  it('shows no "Esperando aprovação" when nobody waits', async () => {
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
      npcs: [],
      hasLivingCharacter: false,
    });
    const { el } = await render(true);
    expect(el.textContent).not.toContain('Esperando aprovação');
  });

  it('shows a pending player their character as "Pendente de aprovação", without the create button', async () => {
    fake.listCharactersResult = Promise.resolve({
      playerCharacters: [
        {
          id: 'c2',
          name: 'Novata',
          kind: 'player',
          state: 'pending',
          classSummary: 'Ladina 1',
          playerDisplayName: 'Samuel',
        },
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

  describe('"Pode subir de nível" (RN-12, D4)', () => {
    const row = (id: string, canLevelUp: boolean): ExperienceRow => ({
      id,
      name: id,
      playerUserId: '',
      sub: '',
      level: 3,
      xp: 2716,
      nextLevelXp: 2700,
      canLevelUp,
      levelUpReason: 1,
    });
    const list = (state = 'locked' as const) =>
      Promise.resolve({
        playerCharacters: [
          {
            id: 'c1',
            name: 'Pensantus',
            kind: 'player' as const,
            state,
            classSummary: 'Mago 3',
            playerDisplayName: 'Vinicius',
          },
          {
            id: 'c2',
            name: 'Toren',
            kind: 'player' as const,
            state,
            classSummary: 'Guerreiro 3',
            playerDisplayName: 'Caio',
          },
        ],
        npcs: [],
        hasLivingCharacter: true,
      });

    function withStore(rows: ExperienceRow[]) {
      TestBed.resetTestingModule();
      TestBed.configureTestingModule({
        imports: [CampaignCharacters],
        providers: [
          provideRouter([]),
          ExperienceStore,
          { provide: CampaignCharactersSource, useClass: FakeCampaignCharactersSource },
        ],
      });
      TestBed.inject(ExperienceStore).rows.set(rows);
      fake = TestBed.inject(CampaignCharactersSource) as unknown as FakeCampaignCharactersSource;
    }

    it('tags who can level up, beside the state, and nobody else', async () => {
      withStore([row('c1', true), row('c2', false)]);
      fake.listCharactersResult = list();
      const { el } = await render(true);

      const rows = Array.from(el.querySelectorAll('li'));
      expect(rows[0].querySelector('app-level-up-tag')?.textContent).toContain(
        'Pode subir de nível',
      );
      expect(rows[0].textContent).toContain('Travada');
      expect(rows[1].querySelector('app-level-up-tag')).toBeNull();
    });

    it('shows a player their own tag', async () => {
      withStore([row('c1', true)]);
      fake.listCharactersResult = list();
      const { el } = await render(false);
      expect(el.querySelectorAll('app-level-up-tag')).toHaveLength(1);
    });

    it('works without the XP store, with no tag', async () => {
      fake.listCharactersResult = list();
      const { el } = await render(true);
      expect(el.querySelector('app-level-up-tag')).toBeNull();
      expect(el.textContent).toContain('Pensantus');
    });
  });

  describe('the reserved characters (MR-049)', () => {
    const reservedKai = {
      id: 'kai',
      name: 'Kai',
      kind: 'player' as const,
      state: 'draft' as const,
      classSummary: 'Monge 5',
      playerDisplayName: null,
      raceName: 'Humano',
      reserved: true,
      claim: { state: 'none' as const, expiresAt: null, claimedBy: null },
    };
    const claimedBrisa = {
      id: 'brisa',
      name: 'Brisa',
      kind: 'player' as const,
      state: 'draft' as const,
      classSummary: 'Ladino 5',
      playerDisplayName: 'Lia',
      raceName: 'Halfling',
      reserved: false,
      claim: { state: 'used' as const, expiresAt: null, claimedBy: 'Lia' },
    };
    const pensantus = {
      id: 'c1',
      name: 'Pensantus',
      kind: 'player' as const,
      state: 'draft' as const,
      classSummary: 'Mago 3',
      playerDisplayName: 'Vinicius',
    };

    it("shows the master a panel of their own, apart from the players' characters, with the way to make one", async () => {
      fake.listCharactersResult = Promise.resolve({
        playerCharacters: [pensantus, reservedKai, claimedBrisa],
        npcs: [],
        hasLivingCharacter: true,
      });

      const { el } = await render(true);

      const players = el.querySelector('#players-heading')!.closest('section')!;
      expect(players.textContent).toContain('Pensantus');
      expect(players.textContent).not.toContain('Kai');
      // A character a player took stays among the players': it is theirs now, with everything that goes with it.
      expect(players.textContent).toContain('Brisa');
      const reserved = el.querySelector('app-reserved-characters')!;
      expect(reserved.querySelector('h3')?.textContent).toContain('Personagens reservados');
      expect(reserved.textContent).toContain('Kai');
      expect(reserved.textContent).toContain('Sem link');
      expect(reserved.textContent).toContain('Assumido por Lia');
      expect(reserved.textContent).not.toContain('Pensantus');
      expect(reserved.querySelector('a.reserved__create')?.getAttribute('href')).toBe(
        '/campaigns/camp-1/reserved/new',
      );
    });

    it('shows the panel with only its call to action while there is nothing reserved', async () => {
      fake.listCharactersResult = Promise.resolve({
        playerCharacters: [pensantus],
        npcs: [],
        hasLivingCharacter: true,
      });

      const { el } = await render(true);

      expect(el.querySelector('app-reserved-characters .reserved__list')).toBeNull();
      expect(el.querySelector('app-reserved-characters')?.textContent).toContain(
        'Criar personagem para um jogador',
      );
    });

    it('never shows a player the panel, and a reserved character does not count as theirs', async () => {
      fake.listCharactersResult = Promise.resolve({
        playerCharacters: [],
        npcs: [],
        hasLivingCharacter: false,
      });

      const { el } = await render(false);

      expect(el.querySelector('app-reserved-characters')).toBeNull();
      expect(el.textContent).not.toContain('Personagens reservados');
      expect(el.textContent).toContain('Criar meu personagem');
    });
  });
});
