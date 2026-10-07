import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { create } from '@bufbuild/protobuf';

import { ListXPAwardsResponseSchema, XPAwardMode, XPAwardSchema } from '../../../../gen/meurpg/progression/v1/progression_pb';
import { ProgressionClient } from '../../../core/progression/progression-client';
import type { CharacterSheetVm } from '../character-sheet.types';
import { LevelUpBanner } from './level-up-banner';
import { LevelUpDoneNotice } from './level-up-done';

@Component({
  imports: [LevelUpBanner],
  template: `<app-level-up-banner [vm]="vm" />`,
})
class Host {
  vm = {} as CharacterSheetVm;
}

const base = {
  id: 'ch-1',
  campaignId: 'camp-1',
  name: 'Pensantus',
  characterKind: 'player',
  state: 'locked',
  isMaster: false,
  canLevelUp: true,
  levelUpReason: 'xp',
  totalLevel: 3,
  nextLevelXp: 2700,
} as CharacterSheetVm;

describe('LevelUpBanner (MR-040)', () => {
  const listAwards = vi.fn();

  function setup(over: Partial<CharacterSheetVm> = {}) {
    listAwards.mockReset().mockResolvedValue(
      create(ListXPAwardsResponseSchema, {
        awards: [
          create(XPAwardSchema, { mode: XPAwardMode.XP_AWARD_MODE_MILESTONE, reason: 'Chegar ao Vale Seco', undone: true, shares: [{ characterId: 'ch-1' }] }),
          create(XPAwardSchema, { mode: XPAwardMode.XP_AWARD_MODE_MILESTONE, reason: 'Cruzar a ponte', shares: [{ characterId: 'outro' }] }),
          create(XPAwardSchema, { mode: XPAwardMode.XP_AWARD_MODE_MILESTONE, reason: 'Chegar ao Vale Seco', shares: [{ characterId: 'ch-1' }] }),
        ],
      }),
    );
    TestBed.configureTestingModule({ providers: [provideRouter([]), { provide: ProgressionClient, useValue: { listAwards } }] });
    const fixture = TestBed.createComponent(Host);
    fixture.componentInstance.vm = { ...base, ...over };
    fixture.detectChanges();
    return fixture;
  }
  const settle = async (f: ReturnType<typeof setup>) => {
    await f.whenStable();
    f.detectChanges();
    return (f.nativeElement as HTMLElement).textContent?.replace(/\s+/g, ' ') ?? '';
  };

  it('gives the owner of a locked sheet the reason and the one filled button, with the next level in it', async () => {
    const f = setup();
    const text = await settle(f);
    expect(text).toContain('Pensantus pode subir de nível');
    expect(text).toContain('Você chegou a 2.700 XP. Você pode subir para o nível 4.');
    const link = (f.nativeElement as HTMLElement).querySelector('a') as HTMLAnchorElement;
    expect(link.textContent?.trim()).toBe('Subir para o nível 4');
    expect(link.getAttribute('href')).toBe('/campaigns/camp-1/characters/ch-1/level-up');
    expect(link.classList.contains('mat-mdc-unelevated-button')).toBe(true);
    expect(listAwards).not.toHaveBeenCalled();
  });

  it('names the milestone the master wrote, the newest one that marks this character and was not undone', async () => {
    const f = setup({ levelUpReason: 'milestone' });
    await settle(f);
    await f.whenStable();
    expect(await settle(f)).toContain('O mestre marcou “Chegar ao Vale Seco”. Você pode subir para o nível 4.');
  });

  it('asks the history once per character, not on every re-read of the sheet', async () => {
    const f = setup({ levelUpReason: 'milestone' });
    await settle(f);
    expect(listAwards).toHaveBeenCalledTimes(1);
    // The sheet is read again (the XP block, the stream) with other numbers: same character, same reason.
    f.componentInstance.vm = { ...f.componentInstance.vm, revision: 9, nextLevelXp: 2800 } as CharacterSheetVm;
    f.changeDetectorRef.detectChanges();
    await settle(f);
    expect(listAwards).toHaveBeenCalledTimes(1);
  });

  it('still works when the milestone cannot be read', async () => {
    const f = setup({ levelUpReason: 'milestone' });
    listAwards.mockRejectedValue(new Error('x'));
    expect(await settle(f)).toContain('O mestre marcou');
  });

  it.each([
    ['the master', { isMaster: true }],
    ['a draft sheet', { state: 'draft' as const }],
    ['an NPC', { characterKind: 'enemy' as const }],
    ['a character that cannot level up', { canLevelUp: false }],
  ])('draws nothing for %s', async (_who, over) => {
    const f = setup(over as Partial<CharacterSheetVm>);
    expect(await settle(f)).not.toContain('pode subir de nível');
    expect((f.nativeElement as HTMLElement).querySelector('a')).toBeNull();
  });
});

describe('LevelUpDoneNotice', () => {
  it('says the level was reached and the master was told, as a status the player dismisses', () => {
    const fixture = TestBed.createComponent(LevelUpDoneNotice);
    fixture.componentRef.setInput('done', { name: 'Pensantus', level: 4 });
    const dismissed = vi.fn();
    fixture.componentInstance.dismissed.subscribe(dismissed);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('[role=status]')?.textContent?.replace(/\s+/g, ' ')).toContain('Pensantus subiu para o nível 4. O mestre foi avisado.');
    (el.querySelector('button[aria-label="Dispensar o aviso"]') as HTMLButtonElement).click();
    expect(dismissed).toHaveBeenCalled();
  });
});
