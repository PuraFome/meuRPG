import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { of } from 'rxjs';

import { XpMode } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { LevelUpReason } from '../../../../gen/meurpg/characters/v1/characters_pb';
import {
  CharacterExperienceSchema,
  GetCampaignExperienceResponseSchema,
  ListMilestonesResponseSchema,
  ListTreasuresToConvertResponseSchema,
  TreasureToConvertSchema,
  ListXPAwardsResponseSchema,
  XPAwardMode,
  XPAwardSchema,
} from '../../../../gen/meurpg/progression/v1/progression_pb';
import { AuthService } from '../../../core/auth/auth.service';
import { RosterClient } from '../../../core/maps/roster-client';
import { ExperienceStore } from '../../../core/progression/experience-store';
import { ProgressionClient } from '../../../core/progression/progression-client';
import { ExperiencePanel } from './experience-panel';

const nbsp = ' ';

function character(id: string, name: string, xp: number, level = 3) {
  return create(CharacterExperienceSchema, {
    characterId: id,
    name,
    level,
    experiencePoints: xp,
    nextLevelXp: 2700,
    canLevelUp: xp >= 2700,
    levelUpReason: xp >= 2700 ? LevelUpReason.XP : LevelUpReason.UNSPECIFIED,
  });
}

describe('ExperiencePanel (E7-09, E7-08)', () => {
  const experience = vi.fn();
  const listAwards = vi.fn();
  const listMilestones = vi.fn();
  const listTreasures = vi.fn();
  const dialogOpen = vi.fn();

  function respond(
    mode: XpMode,
    characters = [
      character('p', 'Pensantus', 2716),
      character('t', 'Toren', 2366),
      character('b', 'Brisa', 0),
    ],
    awards: ReturnType<typeof award>[] = [],
  ) {
    experience.mockResolvedValue(
      create(GetCampaignExperienceResponseSchema, { xpMode: mode, characters }),
    );
    listAwards.mockResolvedValue(create(ListXPAwardsResponseSchema, { awards }));
  }

  function award(id: string, over: object = {}) {
    return create(XPAwardSchema, {
      id,
      mode: XPAwardMode.XP_AWARD_MODE_ENEMIES,
      reason: 'Combate: Emboscada na estrada',
      givenByDisplayName: 'Samuel',
      totalXp: 350,
      shares: [{ characterId: 'p', characterName: 'Pensantus', xp: 116 }],
      ...over,
    });
  }

  async function setup(isMaster: boolean) {
    TestBed.configureTestingModule({
      providers: [
        ExperienceStore,
        {
          provide: ProgressionClient,
          useValue: { experience, listAwards, listMilestones, listTreasures },
        },
        {
          provide: RosterClient,
          useValue: {
            list: () =>
              Promise.resolve([
                {
                  id: 'p',
                  name: 'Pensantus',
                  kind: 1,
                  playerUserId: 'u',
                  classSummary: 'Mago 3',
                  raceName: '',
                  playerName: 'Vinicius',
                },
              ]),
          },
        },
        {
          provide: AuthService,
          useValue: {
            state: () => ({ status: 'signed-in', user: { id: 'u', displayName: null } }),
          },
        },
        { provide: MatDialog, useValue: { open: dialogOpen } },
        { provide: MatBottomSheet, useValue: { open: dialogOpen } },
      ],
    });
    const store = TestBed.inject(ExperienceStore);
    const fixture = TestBed.createComponent(ExperiencePanel);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('campaignName', 'Mirathel');
    fixture.componentRef.setInput('isMaster', isMaster);
    fixture.detectChanges();
    await store.load('camp-1', true, isMaster);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, store };
  }

  const text = (el: HTMLElement) => el.textContent!.replace(/[ \t\r\n]+/g, ' ');
  const rows = (el: HTMLElement) => Array.from(el.querySelectorAll('app-xp-rows li'));

  beforeEach(() => {
    experience.mockReset();
    listAwards.mockReset();
    listMilestones.mockReset();
    listTreasures
      .mockReset()
      .mockResolvedValue(create(ListTreasuresToConvertResponseSchema, { treasures: [], total: 0 }));
    dialogOpen
      .mockReset()
      .mockReturnValue({ afterClosed: () => of(undefined), afterDismissed: () => of(undefined) });
    // jsdom has no matchMedia: openSheet falls back to the dialog.
  });

  it("shows each character's XP and what is missing, with the tag for who can level up", async () => {
    respond(XpMode.ENEMIES);
    const { el } = await setup(true);

    expect(text(el)).toContain(`XP por inimigos derrotados. O nível 4 pede 2.700${nbsp}XP.`);
    expect(rows(el)).toHaveLength(3);
    expect(rows(el)[0].textContent).toContain(`2.716 de${nbsp}2.700${nbsp}XP`);
    expect(rows(el)[0].textContent).toContain('Pode subir de nível');
    expect(rows(el)[1].textContent).toContain(`Faltam${nbsp}334${nbsp}XP`);
    expect(rows(el)[1].querySelector('app-level-up-tag')).toBeNull();
    // Nothing is only a bar: the numbers are written.
    expect(rows(el)[2].textContent).toContain(`0 de${nbsp}2.700${nbsp}XP`);
    expect(rows(el)[0].querySelector('.bar')?.getAttribute('aria-hidden')).toBe('true');
  });

  it('never goes away: with no award it invites the first one', async () => {
    respond(XpMode.ENEMIES, [character('p', 'Pensantus', 0)]);
    const { el } = await setup(true);
    expect(el.querySelector('h2')?.textContent).toBe('Experiência');
    expect(text(el)).toContain(
      'Ninguém recebeu XP ainda. Dê XP depois de um combate ou quando quiser.',
    );
    expect(el.querySelector('app-award-history')).toBeNull();
  });

  it('tells a player there is no XP yet, without an invitation', async () => {
    respond(XpMode.ENEMIES, [character('p', 'Pensantus', 0)]);
    const { el } = await setup(false);
    expect(text(el)).toContain('Ninguém recebeu XP ainda.');
    expect(text(el)).not.toContain('Dê XP');
  });

  it('is not drawn for a player while there is no living character and no award', async () => {
    respond(XpMode.ENEMIES, []);
    const { el } = await setup(false);
    expect(el.querySelector<HTMLElement>('section')?.hidden).toBe(true);
  });

  it('is still there for the master with no living character: it invites the first award', async () => {
    respond(XpMode.ENEMIES, []);
    const { el } = await setup(true);
    expect(el.querySelector<HTMLElement>('section')?.hidden).toBe(false);
  });

  it('gives the master "Dar XP" and nobody else', async () => {
    respond(XpMode.ENEMIES);
    expect(
      Array.from((await setup(true)).el.querySelectorAll('button')).some(
        (b) => b.textContent?.trim() === 'Dar XP',
      ),
    ).toBe(true);
    TestBed.resetTestingModule();
    respond(XpMode.ENEMIES);
    expect((await setup(false)).el.querySelector('app-xp-give-button')).toBeNull();
  });

  it('shows the history under the rows, with the footer for who reads it', async () => {
    respond(XpMode.ENEMIES, undefined, [award('a1', { canUndo: true })]);
    const master = await setup(true);
    expect(master.el.querySelector('h3')?.textContent).toBe('Histórico');
    expect(master.el.querySelectorAll('app-award-history li')).toHaveLength(1);
    expect(text(master.el)).toContain('Só o último prêmio pode ser desfeito.');
    TestBed.resetTestingModule();
    respond(XpMode.ENEMIES, undefined, [award('a1')]);
    const player = await setup(false);
    expect(text(player.el)).toContain('Todos da campanha veem este histórico.');
    expect(player.el.querySelectorAll('app-award-history button')).toHaveLength(0);
  });

  describe('in a campaign that levels by milestones', () => {
    it('hands the panel over to the milestones, with no XP, no "Dar XP" and no history', async () => {
      respond(XpMode.MILESTONES, [character('p', 'Pensantus', 0)], []);
      listMilestones.mockResolvedValue(create(ListMilestonesResponseSchema, {}));
      const { el } = await setup(true);

      expect(el.querySelector('app-milestones-panel')).not.toBeNull();
      expect(text(el)).toContain('Campanha por marcos');
      expect(text(el)).not.toMatch(/\d+\s*XP/);
      expect(text(el)).not.toContain('Histórico');
      expect(
        Array.from(el.querySelectorAll('button')).some((b) => b.textContent?.trim() === 'Dar XP'),
      ).toBe(false);
      expect(listAwards).toHaveBeenCalled(); // the page loads it for every mode; this panel does not read it
    });
  });

  it('says it could not load, and tries again', async () => {
    experience.mockRejectedValue(new Error('x'));
    listAwards.mockResolvedValue(create(ListXPAwardsResponseSchema, {}));
    const { fixture, el, store } = await setup(true);
    expect(el.querySelector('[role="alert"]')?.textContent).toContain(
      'Não foi possível carregar a experiência',
    );

    respond(XpMode.ENEMIES);
    Array.from(el.querySelectorAll('button'))
      .find((b) => b.textContent?.includes('Tentar de novo'))!
      .click();
    await fixture.whenStable();
    await store.refresh();
    fixture.detectChanges();
    expect(rows(el)).toHaveLength(3);
  });

  it('says what the master just gave, in a polite status, and reads again', async () => {
    respond(XpMode.ENEMIES);
    const { fixture, el, store } = await setup(true);
    const refresh = vi.spyOn(store, 'refresh').mockResolvedValue();
    const inner = fixture.debugElement.query((d) => d.name === 'app-xp-give-button');
    inner.triggerEventHandler('given', {
      kind: 'xp',
      result: { award: award('a1', { totalXp: 350 }), xpEach: 116, lostXp: 2 },
    });
    await fixture.whenStable();
    fixture.detectChanges();

    expect(el.querySelector('.exp__status')?.getAttribute('role')).toBe('status');
    expect(el.querySelector('.exp__status')?.textContent).toContain('350');
    expect(el.querySelector('.exp__status')?.textContent).toContain('116 para cada');
    expect(refresh).toHaveBeenCalled();
  });

  describe('"Voltar à cidade" (E9-09, MR-041)', () => {
    const treasure = (id: string, name: string, valuePo: number, by: string) =>
      create(TreasureToConvertSchema, {
        pointId: id,
        name,
        valuePo,
        foundBy: [{ characterId: id, characterName: by }],
      });
    const THREE = [
      treasure('c', 'Baú de moedas', 250, 'Brisa'),
      treasure('b', 'Bolsa do capitão', 120, 'Toren'),
      treasure('i', 'Ídolo de prata', 50, 'Pensantus'),
    ];
    const buttons = (el: HTMLElement) =>
      Array.from(el.querySelectorAll('.exp__head button'), (b) =>
        b.textContent?.replace('currency_exchange', '').trim(),
      );

    it('puts "Voltar à cidade" beside "Dar XP" in a campaign by gold, and the strip with what waits', async () => {
      respond(XpMode.GOLD);
      listTreasures.mockResolvedValue(
        create(ListTreasuresToConvertResponseSchema, { treasures: THREE, total: 3 }),
      );
      const { el } = await setup(true);
      expect(buttons(el)).toEqual(['Dar XP', 'Voltar à cidade']);
      const strip = el.querySelector('app-treasure-strip')!;
      expect(strip.textContent).toContain('Encontrado, ainda não convertido');
      expect(strip.textContent).toContain(`3\u00a0tesouros · 420${nbsp}PO`);
      expect(strip.textContent).toContain(`Baú de moedas, 250${nbsp}PO, de Brisa`);
    });

    it('opens the conversion with the treasures and who is alive', async () => {
      respond(XpMode.GOLD);
      listTreasures.mockResolvedValue(
        create(ListTreasuresToConvertResponseSchema, { treasures: THREE, total: 3 }),
      );
      const { el } = await setup(true);
      Array.from(el.querySelectorAll<HTMLButtonElement>('.exp__head button'))
        .find((b) => b.textContent?.includes('Voltar à cidade'))!
        .click();
      expect(dialogOpen).toHaveBeenCalledTimes(1);
      const config = dialogOpen.mock.calls[0][1];
      expect(config.data.treasures).toHaveLength(3);
      expect(config.data.total).toBe(3);
      expect(config.data.rows).toHaveLength(3);
    });

    it('invites the next find when nothing waits, and keeps the button', async () => {
      respond(XpMode.GOLD);
      const { el } = await setup(true);
      expect(el.querySelector('app-treasure-strip')?.textContent).toContain(
        'Nenhum tesouro esperando.',
      );
      expect(buttons(el)).toContain('Voltar à cidade');
    });

    it('in a campaign by enemies there is no button, and the strip says why (when something was found)', async () => {
      respond(XpMode.ENEMIES);
      listTreasures.mockResolvedValue(
        create(ListTreasuresToConvertResponseSchema, { treasures: [THREE[0]], total: 1 }),
      );
      const { el } = await setup(true);
      expect(buttons(el)).toEqual(['Dar XP']);
      expect(el.querySelector('app-treasure-strip')?.textContent).toContain(
        'Esta campanha dá XP por inimigos, então o tesouro não vira XP.',
      );
    });

    it('in a campaign by enemies with nothing found there is no strip', async () => {
      respond(XpMode.ENEMIES);
      const { el } = await setup(true);
      expect(el.querySelector('app-treasure-strip .strip')).toBeNull();
    });

    it('shows a player no strip and no button, and asks the server for no treasure', async () => {
      respond(XpMode.GOLD);
      const { el } = await setup(false);
      expect(el.querySelector('app-treasure-strip')).toBeNull();
      expect(el.querySelector('.exp__head button')).toBeNull();
      expect(listTreasures).not.toHaveBeenCalled();
    });

    it('says what it did right after converting, and reads the XP and the treasures again', async () => {
      respond(XpMode.GOLD);
      const { fixture, el, store } = await setup(true);
      const refresh = vi.spyOn(store, 'refresh').mockResolvedValue();
      const award = create(XPAwardSchema, {
        id: 'a1',
        gold: 420,
        treasureCount: 3,
        totalXp: 420,
        shares: [
          { characterId: 'p', characterName: 'Pensantus', xp: 105 },
          { characterId: 't', characterName: 'Toren', xp: 105 },
        ],
      });
      fixture.debugElement
        .query((d) => d.name === 'app-xp-give-button')
        .triggerEventHandler('given', { kind: 'xp', result: { award, xpEach: 105, lostXp: 0 } });
      await fixture.whenStable();
      fixture.detectChanges();
      expect(el.querySelector('.exp__status')?.textContent).toContain(
        `Voltar à cidade: Pensantus e Toren receberam 105${nbsp}XP cada. Os 3\u00a0tesouros foram convertidos.`,
      );
      expect(refresh).toHaveBeenCalled();
    });
  });

  describe('right after an award (E9-09 state 7)', () => {
    const shares = [
      { characterId: 'p', characterName: 'Pensantus', xp: 105 },
      { characterId: 't', characterName: 'Toren', xp: 105 },
    ];
    const town = () =>
      create(XPAwardSchema, { id: 'a1', gold: 420, treasureCount: 3, totalXp: 420, shares });

    it('writes "+105 XP" in place of "Faltam…" beside each one who got it, and drops it on an undo', async () => {
      respond(XpMode.GOLD, undefined, [award('a1', { canUndo: true })]);
      const { fixture, el, store } = await setup(true);
      vi.spyOn(store, 'refresh').mockResolvedValue();
      fixture.debugElement
        .query((d) => d.name === 'app-xp-give-button')
        .triggerEventHandler('given', {
          kind: 'xp',
          result: { award: town(), xpEach: 105, lostXp: 0 },
        });
      await fixture.whenStable();
      fixture.detectChanges();
      const gains = Array.from(el.querySelectorAll('.num__gain'), (g) => g.textContent);
      // Pensantus can level up (the tag stays); Toren shows what he got; Brisa, who got nothing, keeps "Faltam…".
      expect(gains).toEqual([`+105${nbsp}XP`]);
      expect(rows(el)[2].textContent).toContain('Faltam');

      fixture.debugElement
        .query((d) => d.name === 'app-award-history')
        ?.triggerEventHandler('undone', undefined);
      fixture.detectChanges();
      expect(el.querySelectorAll('.num__gain')).toHaveLength(0);
      expect(el.querySelector('.exp__status')?.textContent?.trim()).toBe('');
    });

    it('has one notice at a time: the history keeps its own while the confirmation is up', async () => {
      respond(XpMode.GOLD, undefined, [award('a1', { canUndo: true })]);
      const { fixture, el, store } = await setup(true);
      vi.spyOn(store, 'refresh').mockResolvedValue();
      fixture.debugElement
        .query((d) => d.name === 'app-xp-give-button')
        .triggerEventHandler('given', {
          kind: 'xp',
          result: { award: town(), xpEach: 105, lostXp: 0 },
        });
      fixture.detectChanges();
      const history = fixture.debugElement.query((d) => d.name === 'app-award-history');
      expect(history.componentInstance.hideNotice()).toBe(true);
      expect(el.querySelector('.exp__status')?.textContent).toContain('Voltar à cidade');
    });

    it('asks for the full-width buttons on a phone, for every mode', async () => {
      respond(XpMode.ENEMIES);
      const { el } = await setup(true);
      expect(el.querySelector('app-xp-give-button')?.classList.contains('block')).toBe(true);
    });
  });
});
