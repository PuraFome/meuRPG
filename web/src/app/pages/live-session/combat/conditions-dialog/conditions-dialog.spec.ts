import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { CombatClient } from '../../../../core/combat/combat-client';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { ConditionsDialog } from './conditions-dialog';

describe('ConditionsDialog', () => {
  function open() {
    const before = combatant({ id: 'g', label: 'Goblin', conditions: [] });
    const poisoned = { ...before, conditions: ['condition:poisoned'] };
    const state = new CombatState();
    state.apply(encounter({ id: 'e1', revision: 1, combatants: [before] }));
    const setConditions = vi.fn(async () =>
      encounter({ id: 'e1', revision: 3, combatants: [poisoned] }),
    );
    TestBed.configureTestingModule({
      providers: [
        { provide: CombatClient, useValue: { setConditions } },
        { provide: MatDialogRef, useValue: { close: vi.fn() } },
        {
          provide: MAT_DIALOG_DATA,
          useValue: { campaignId: 'c1', encounterId: 'e1', combatant: before, state },
        },
      ],
    });
    const fixture = TestBed.createComponent(ConditionsDialog);
    fixture.detectChanges();
    const cmp = fixture.componentInstance as unknown as {
      toggle(k: string): void;
      save(): Promise<void>;
      changed(): boolean;
    };
    return { state, poisoned, setConditions, cmp };
  }

  const sentKeys = (call: unknown[]) => (call[3] as { keys: string[] }).keys;

  it('keeps a condition added while it is open when the master saves another one', async () => {
    const { state, poisoned, setConditions, cmp } = open();
    state.apply(encounter({ id: 'e1', revision: 2, combatants: [poisoned] }));
    cmp.toggle('condition:prone');
    await cmp.save();
    expect(setConditions).toHaveBeenCalledTimes(1);
    expect(sentKeys(setConditions.mock.calls[0] as unknown[]).sort()).toEqual([
      'condition:poisoned',
      'condition:prone',
    ]);
  });

  it('is not changed by a condition added elsewhere', () => {
    const { state, poisoned, cmp } = open();
    state.apply(encounter({ id: 'e1', revision: 2, combatants: [poisoned] }));
    expect(cmp.changed()).toBe(false);
  });

  it('does not bring back a condition the master took off when it is added elsewhere again', async () => {
    const { state, poisoned, setConditions, cmp } = open();
    state.apply(encounter({ id: 'e1', revision: 2, combatants: [poisoned] }));
    cmp.toggle('condition:poisoned');
    await cmp.save();
    expect(sentKeys(setConditions.mock.calls[0] as unknown[])).toEqual([]);
  });
});
