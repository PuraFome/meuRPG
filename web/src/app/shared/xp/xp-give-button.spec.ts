import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { of } from 'rxjs';

import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { XPAwardSchema } from '../../../gen/meurpg/progression/v1/progression_pb';
import type { ExperienceRow } from '../../core/progression/experience-store';
import { AwardXpSheet } from './award-xp-sheet';
import { MilestoneSheet } from './milestone-sheet';
import { type GiveResult, XpGiveButton } from './xp-give-button';

const rows: ExperienceRow[] = [{ id: 'p', name: 'Pensantus', sub: '', level: 3, xp: 0, nextLevelXp: 300, canLevelUp: false, levelUpReason: 0 }];

@Component({
  imports: [XpGiveButton],
  template: `<app-xp-give-button campaignId="camp-1" campaignName="Mirathel" [xpMode]="mode" [rows]="rows" (given)="results.push($event)" />`,
})
class Host {
  mode = XpMode.ENEMIES;
  rows = rows;
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
});
