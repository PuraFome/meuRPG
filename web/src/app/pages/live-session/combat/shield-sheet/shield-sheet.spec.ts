import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { MatSnackBar } from '@angular/material/snack-bar';
import { ConnectError, Code } from '@connectrpc/connect';

import { CombatantKind, CombatantSide } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { ShieldSheet } from './shield-sheet';

const toren = combatant({
  id: 't',
  label: 'Toren',
  kind: CombatantKind.PLAYER,
  side: CombatantSide.PARTY,
  mine: true,
});
const cap = combatant({ id: 'cap', label: 'Capitão Goblin' });

describe('ShieldSheet: the idempotency key follows the answer', () => {
  it('"Conjurar" after a lost "Não usar" is a new request with a new key', async () => {
    const keys: Record<string, string> = {};
    const api = {
      declineReaction: (_c: string, _e: string, _p: string, key: string) => {
        keys['decline'] = key;
        return Promise.reject(new ConnectError('timeout', Code.Unavailable));
      },
      useReaction: (_c: string, _e: string, _p: string, _s: unknown, key: string) => {
        keys['use'] = key;
        return Promise.reject(new ConnectError('stop', Code.Aborted));
      },
    };
    const state = new CombatState();
    state.apply(encounter({ currentCombatantId: 't', combatants: [toren, cap] }));
    const data = {
      campaignId: 'c',
      encounterId: 'enc',
      round: 1,
      state,
      armorClass: 15,
      pact: null,
      prompt: {
        pendingDamageId: 'p1',
        spellNamePt: 'Escudo Arcano',
        slots: [{ level: 1, pact: false, free: 2 }],
      },
      usage: [{ level: 1, total: 2, used: 0 }],
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: () => undefined } },
        { provide: CombatClient, useValue: api },
      ],
    });
    const sheet = TestBed.createComponent(ShieldSheet).componentInstance as unknown as {
      use(): Promise<void>;
      decline(): Promise<void>;
    };
    await sheet.decline();
    await sheet.use();
    expect(keys['use']).toBeDefined();
    expect(keys['use']).not.toBe(keys['decline']);
  });
});

describe('ShieldSheet: a prompt nobody awaits closes by itself', () => {
  const prompt = (id: string) => ({
    pendingDamageId: id,
    targetId: 't',
    spellNamePt: 'Escudo Arcano',
    slots: [{ level: 1, pact: false, free: 2 }],
  });

  it('closes with a notice when the master answers for the player', async () => {
    const closed: unknown[] = [];
    const notices: string[] = [];
    const state = new CombatState();
    state.apply(
      encounter({
        currentCombatantId: 'cap',
        combatants: [toren, cap],
        reactionPrompts: [prompt('p1')] as never,
      }),
    );
    const data = {
      campaignId: 'c',
      encounterId: 'enc',
      round: 1,
      state,
      armorClass: 15,
      pact: null,
      prompt: prompt('p1'),
      usage: [{ level: 1, total: 2, used: 0 }],
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: (r: unknown) => closed.push(r) } },
        { provide: CombatClient, useValue: {} },
        { provide: MatSnackBar, useValue: { open: (text: string) => notices.push(text) } },
      ],
    });
    const fixture = TestBed.createComponent(ShieldSheet);
    fixture.detectChanges();
    await fixture.whenStable();
    expect(closed).toEqual([]);

    state.apply(encounter({ currentCombatantId: 'cap', combatants: [toren, cap] }));
    fixture.detectChanges();
    await fixture.whenStable();
    expect(closed).toEqual([false]);
    expect(notices).toEqual(['O mestre respondeu por você: o ataque já não espera a sua reação.']);
  });
});
