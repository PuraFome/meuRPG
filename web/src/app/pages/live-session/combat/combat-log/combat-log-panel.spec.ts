import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';

import { CombatClient } from '../../../../core/combat/combat-client';
import { CombatLogState } from '../../../../core/combat/combat-log-state';
import { CombatState } from '../../../../core/combat/combat-state';
import { encounter } from '../../../../core/combat/combat-testing';
import { CombatLogPanel } from './combat-log-panel';

describe('CombatLogPanel: the error of the undo', () => {
  it('is said by a status region that was there first, the notice without a role', () => {
    TestBed.configureTestingModule({
      providers: [
        { provide: CombatClient, useValue: {} },
        { provide: MatDialog, useValue: { open: vi.fn() } },
        { provide: MatBottomSheet, useValue: { open: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(CombatLogPanel);
    fixture.componentRef.setInput('log', new CombatLogState());
    fixture.componentRef.setInput('encounter', encounter({ round: 1 }));
    fixture.componentRef.setInput('campaignId', 'c');
    fixture.componentRef.setInput('state', new CombatState());
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const live = el.querySelector('.mr-live[role="status"]')!;
    expect(live.textContent?.trim()).toBe('');
    (fixture.componentInstance as unknown as { error: { set(v: string): void } }).error.set(
      'Outra pessoa agiu primeiro.',
    );
    fixture.detectChanges();
    const notice = live.querySelector('.mr-notice--warning')!;
    expect(notice.textContent).toContain('Outra pessoa agiu primeiro.');
    expect(notice.hasAttribute('role')).toBe(false);
  });
});
