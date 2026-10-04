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
  const dialogOpen = vi.fn();

  function respond(mode: XpMode, characters = [character('p', 'Pensantus', 2716), character('t', 'Toren', 2366), character('b', 'Brisa', 0)], awards: ReturnType<typeof award>[] = []) {
    experience.mockResolvedValue(create(GetCampaignExperienceResponseSchema, { xpMode: mode, characters }));
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
        { provide: ProgressionClient, useValue: { experience, listAwards, listMilestones } },
        { provide: RosterClient, useValue: { list: () => Promise.resolve([{ id: 'p', name: 'Pensantus', kind: 1, playerUserId: 'u', classSummary: 'Mago 3', raceName: '', playerName: 'Vinicius' }]) } },
        { provide: AuthService, useValue: { state: () => ({ status: 'signed-in', user: { id: 'u', displayName: null } }) } },
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
    await store.load('camp-1', true);
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
    dialogOpen.mockReset().mockReturnValue({ afterClosed: () => of(undefined), afterDismissed: () => of(undefined) });
    // jsdom has no matchMedia: openSheet falls back to the dialog.
  });

  it('shows each character\'s XP and what is missing, with the tag for who can level up', async () => {
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
    expect(text(el)).toContain('Ninguém recebeu XP ainda. Dê XP depois de um combate ou quando quiser.');
    expect(el.querySelector('app-award-history')).toBeNull();
  });

  it('tells a player there is no XP yet, without an invitation', async () => {
    respond(XpMode.ENEMIES, [character('p', 'Pensantus', 0)]);
    const { el } = await setup(false);
    expect(text(el)).toContain('Ninguém recebeu XP ainda.');
    expect(text(el)).not.toContain('Dê XP');
  });

  it('gives the master "Dar XP" and nobody else', async () => {
    respond(XpMode.ENEMIES);
    expect(Array.from((await setup(true)).el.querySelectorAll('button')).some((b) => b.textContent?.trim() === 'Dar XP')).toBe(true);
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
      expect(Array.from(el.querySelectorAll('button')).some((b) => b.textContent?.trim() === 'Dar XP')).toBe(false);
      expect(listAwards).toHaveBeenCalled(); // the page loads it for every mode; this panel does not read it
    });
  });

  it('says it could not load, and tries again', async () => {
    experience.mockRejectedValue(new Error('x'));
    listAwards.mockResolvedValue(create(ListXPAwardsResponseSchema, {}));
    const { fixture, el, store } = await setup(true);
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('Não foi possível carregar a experiência');

    respond(XpMode.ENEMIES);
    Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes('Tentar de novo'))!.click();
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
    inner.triggerEventHandler('given', { kind: 'xp', result: { award: award('a1', { totalXp: 350 }), xpEach: 116, lostXp: 2 } });
    await fixture.whenStable();
    fixture.detectChanges();

    expect(el.querySelector('.exp__status')?.getAttribute('role')).toBe('status');
    expect(el.querySelector('.exp__status')?.textContent).toContain('350');
    expect(el.querySelector('.exp__status')?.textContent).toContain('116 para cada');
    expect(refresh).toHaveBeenCalled();
  });
});
