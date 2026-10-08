// The adjust sheet works from the combatant as the combat has it now, not as it was at open.
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { CombatClient } from '../../../../core/combat/combat-client';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { AdjustNpc } from './adjust-npc';

describe('AdjustNpc reads the live hit points', () => {
  it('compares an exact value with the current hit points', async () => {
    const at5 = combatant({ id: 'g', label: 'Goblin', hitPointsCurrent: 5, hitPointsMax: 12 });
    const at10 = { ...at5, hitPointsCurrent: 10 };
    const state = new CombatState();
    state.apply(encounter({ id: 'e1', revision: 1, combatants: [at5] }));
    const adjustHitPoints = vi.fn(async () =>
      encounter({ id: 'e1', revision: 3, combatants: [at10] }),
    );
    const close = vi.fn();
    TestBed.configureTestingModule({
      providers: [
        { provide: CombatClient, useValue: { adjustHitPoints } },
        { provide: MatDialogRef, useValue: { close } },
        {
          provide: MAT_DIALOG_DATA,
          useValue: { campaignId: 'c1', encounterId: 'e1', combatant: at5, state },
        },
      ],
    });
    const fixture = TestBed.createComponent(AdjustNpc);
    fixture.detectChanges();
    const cmp = fixture.componentInstance as unknown as {
      mode: { set(v: string): void };
      amount: { set(v: number): void };
      save(): Promise<void>;
    };
    // The stream brings the goblin to 10 HP while the sheet is open.
    state.apply(encounter({ id: 'e1', revision: 2, combatants: [at10] }));
    cmp.mode.set('exact');
    cmp.amount.set(5);
    await cmp.save();
    expect(adjustHitPoints).toHaveBeenCalledTimes(1);
    expect(adjustHitPoints).toHaveBeenCalledWith(
      'c1',
      'e1',
      'g',
      { change: { kind: 'exact', value: 5 }, temporary: undefined },
      expect.any(String),
    );
  });
});
