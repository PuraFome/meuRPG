import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { of } from 'rxjs';

import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { TreasureToConvertSchema, XPAwardSchema } from '../../../gen/meurpg/progression/v1/progression_pb';
import type { ExperienceRow } from '../../core/progression/experience-store';
import { AwardXpSheet } from './award-xp-sheet';
import { MilestoneSheet } from './milestone-sheet';
import { TownSheet } from './town-sheet';
import { type GiveResult, XpGiveButton } from './xp-give-button';

const rows: ExperienceRow[] = [{ id: 'p', name: 'Pensantus', playerUserId: '', sub: '', level: 3, xp: 0, nextLevelXp: 300, canLevelUp: false, levelUpReason: 0 }];

@Component({
  imports: [XpGiveButton],
  template: `<app-xp-give-button
    campaignId="camp-1"
    campaignName="Mirathel"
    [xpMode]="mode"
    [rows]="rows"
    [treasures]="treasures"
    [treasuresTotal]="treasures.length"
    [showTown]="showTown"
    (given)="results.push($event)"
  />`,
})
class Host {
  mode = XpMode.ENEMIES;
  rows = rows;
  treasures = [create(TreasureToConvertSchema, { pointId: 'c', name: 'Baú de moedas', valuePo: 250 })];
  showTown = false;
  results: GiveResult[] = [];
}

describe('XpGiveButton', () => {
  const open = vi.fn();

  function setup(mode: XpMode, answer: unknown) {
    open.mockReset().mockReturnValue({ afterClosed: () => of(answer) });
    TestBed.configureTestingModule({
      providers: [{ provide: MatDialog, useValue: { open } }, { provide: MatBottomSheet, useValue: { open } }],
    });
    const fixture = TestBed.createComponent(Host);
    fixture.componentInstance.mode = mode;
    fixture.detectChanges();
    const button = fixture.nativeElement.querySelector('button') as HTMLButtonElement | null;
    return { fixture, button, host: fixture.componentInstance };
  }

  it('is an outlined "Dar XP" that opens the award sheet with the party', () => {
    const { button } = setup(XpMode.ENEMIES, undefined);
    expect(button?.textContent?.trim()).toBe('Dar XP');
    expect(button?.classList.contains('mat-mdc-outlined-button')).toBe(true);
    button!.click();

    expect(open.mock.calls[0][0]).toBe(AwardXpSheet);
    expect(open.mock.calls[0][1]).toMatchObject({ data: { campaignId: 'camp-1', xpMode: XpMode.ENEMIES, rows }, width: '560px' });
  });

  it('is "Registrar marco" in a milestones campaign, and opens the milestone sheet', () => {
    const { button } = setup(XpMode.MILESTONES, undefined);
    expect(button?.textContent?.trim()).toContain('Registrar marco');
    button!.click();
    expect(open.mock.calls[0][0]).toBe(MilestoneSheet);
    expect(open.mock.calls[0][1].data).toMatchObject({ campaignId: 'camp-1', campaignName: 'Mirathel' });
  });

  it('draws nothing until it knows how the campaign levels', () => {
    expect(setup(XpMode.UNSPECIFIED, undefined).button).toBeNull();
  });

  it('says what was given, and nothing when the master cancelled', () => {
    const cancelled = setup(XpMode.ENEMIES, undefined);
    cancelled.button!.click();
    expect(cancelled.host.results).toEqual([]);

    TestBed.resetTestingModule();
    const award = create(XPAwardSchema, { id: 'a1' });
    const given = setup(XpMode.ENEMIES, { award, xpEach: 50, lostXp: 0 });
    given.button!.click();
    expect(given.host.results).toEqual([{ kind: 'xp', result: { award, xpEach: 50, lostXp: 0 } }]);
  });

  describe('"Voltar à cidade" (E9-09)', () => {
    function gold(answer: unknown, showTown = true) {
      open.mockReset().mockReturnValue({ afterClosed: () => of(answer) });
      TestBed.resetTestingModule();
      TestBed.configureTestingModule({
        providers: [{ provide: MatDialog, useValue: { open } }, { provide: MatBottomSheet, useValue: { open } }],
      });
      const fixture = TestBed.createComponent(Host);
      fixture.componentInstance.mode = XpMode.GOLD;
      fixture.componentInstance.showTown = showTown;
      fixture.detectChanges();
      const buttons = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('button'));
      return { fixture, buttons, host: fixture.componentInstance };
    }

    it('adds an outlined "Voltar à cidade" beside "Dar XP" in a campaign by gold, and opens the conversion with the treasures', () => {
      const { buttons } = gold(undefined);
      expect(buttons.map((b) => b.textContent?.replace('currency_exchange', '').trim())).toEqual(['Dar XP', 'Voltar à cidade']);
      expect(buttons.every((b) => b.classList.contains('mat-mdc-outlined-button'))).toBe(true);
      buttons[1].click();
      expect(open.mock.calls[0][0]).toBe(TownSheet);
      expect(open.mock.calls[0][1]).toMatchObject({ data: { campaignId: 'camp-1', xpMode: XpMode.GOLD, rows, total: 1 }, width: '600px' });
      expect(open.mock.calls[0][1].data.treasures).toHaveLength(1);
    });

    it('has only "Dar XP" where the host does not ask for it (the session page), or in another mode', () => {
      expect(gold(undefined, false).buttons).toHaveLength(1);
      TestBed.resetTestingModule();
      open.mockReset().mockReturnValue({ afterClosed: () => of(undefined) });
      TestBed.configureTestingModule({
        providers: [{ provide: MatDialog, useValue: { open } }, { provide: MatBottomSheet, useValue: { open } }],
      });
      const fixture = TestBed.createComponent(Host);
      fixture.componentInstance.showTown = true;
      fixture.detectChanges();
      expect(fixture.nativeElement.querySelectorAll('button')).toHaveLength(1);
    });

    it('hands "Dar XP" the treasures in a campaign by gold only', () => {
      const { buttons } = gold(undefined);
      buttons[0].click();
      expect(open.mock.calls[0][1].data.treasures).toHaveLength(1);
      TestBed.resetTestingModule();
      open.mockReset().mockReturnValue({ afterClosed: () => of(undefined) });
      TestBed.configureTestingModule({
        providers: [{ provide: MatDialog, useValue: { open } }, { provide: MatBottomSheet, useValue: { open } }],
      });
      const fixture = TestBed.createComponent(Host);
      fixture.detectChanges();
      (fixture.nativeElement.querySelector('button') as HTMLButtonElement).click();
      expect(open.mock.calls[0][1].data.treasures).toBeUndefined();
    });

    it('opens the conversion when "Dar XP" asks for it (and gives nothing itself)', () => {
      const { buttons, host } = gold({ town: true });
      // The conversion is then cancelled.
      open.mockReturnValueOnce({ afterClosed: () => of({ town: true }) }).mockReturnValueOnce({ afterClosed: () => of(undefined) });
      buttons[0].click();
      expect(open.mock.calls.map((c) => c[0])).toEqual([AwardXpSheet, TownSheet]);
      expect(host.results).toEqual([]);
    });

    it('says what was converted, as any XP award', () => {
      const award = create(XPAwardSchema, { id: 'a1', treasureCount: 3, gold: 420 });
      const { buttons, host } = gold({ award, xpEach: 105, lostXp: 0 });
      buttons[1].click();
      expect(host.results).toEqual([{ kind: 'xp', result: { award, xpEach: 105, lostXp: 0 } }]);
    });
  });
});
