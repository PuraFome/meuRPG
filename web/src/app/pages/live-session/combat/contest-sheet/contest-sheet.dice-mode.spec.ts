import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatantKind, CombatantSide } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  CheckOptionSchema,
  ContestAttackOptionKind,
  ContestAttackOptionSchema,
  ContestPurpose,
  ContestTargetSchema,
  RollModeKind,
} from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { CreatureSize } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { CombatState } from '../../../../core/combat/combat-state';
import { ContestClient } from '../../../../core/combat/contest-client';
import { ContestState } from '../../../../core/combat/contest-state';
import { FakeContestClient, textOf } from '../../../../core/combat/contest-testing';
import { ContestSheet, type ContestSheetData } from './contest-sheet';

function table() {
  return encounter({
    combatants: [
      combatant({
        id: 't',
        label: 'Toren',
        kind: CombatantKind.PLAYER,
        side: CombatantSide.PARTY,
        size: CreatureSize.MEDIUM,
      } as never),
      combatant({ id: 'h', label: 'Hobgoblin' }),
    ],
  });
}

describe('ContestSheet: the campaign’s dice mode (RN-18)', () => {
  let api: FakeContestClient;

  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
  });

  async function setupAtRoll(diceMode: DiceMode) {
    api = new FakeContestClient();
    api.encounterAnswer = table();
    const state = new CombatState();
    state.encounter.set(table());
    const data: ContestSheetData = {
      campaignId: 'c',
      encounterId: 'enc',
      initiatorId: 't',
      purpose: ContestPurpose.GRAPPLE,
      attack: create(ContestAttackOptionSchema, {
        kind: ContestAttackOptionKind.GRAPPLE,
        replacesAttack: true,
        enabled: true,
        targets: [
          create(ContestTargetSchema, {
            combatantId: 'h',
            size: CreatureSize.MEDIUM,
            distanceFt: 5,
            eligible: true,
          }),
        ],
        rollOption: create(CheckOptionSchema, {
          modifier: 5,
          known: true,
          mode: RollModeKind.NORMAL,
        }),
      }),
      diceMode,
      preference: DicePreference.APP,
      state,
      contests: new ContestState(),
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: ContestClient, useValue: api.as() },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn(), disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(ContestSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    const button = (name: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        b.textContent?.includes(name),
      );
    el.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click();
    await settle();
    button('Rolar a disputa')!.click();
    await settle();
    expect(textOf(el)).toContain('Seu teste de Força (Atletismo): +5.');
    return { el, button, settle };
  }

  it('follows the campaign’s dice mode: forced app offers only "Rolar no app" and sends no typed value', async () => {
    const { el, button, settle } = await setupAtRoll(DiceMode.APP);
    expect(button('Digitar o resultado')).toBeUndefined();
    expect(el.querySelector('input[type="text"]')).toBeNull();
    button('Rolar no app')!.click();
    await settle();
    expect(api.started[0].input.die).toEqual({ inApp: true });
  });

  it('follows the campaign’s dice mode: forced physical offers only the typed entry and sends the face, never the total', async () => {
    const { el, button, settle } = await setupAtRoll(DiceMode.PHYSICAL);
    expect(button('Rolar no app')).toBeUndefined();
    const field = el.querySelector<HTMLInputElement>('input[type="text"]')!;
    field.value = '14';
    field.dispatchEvent(new Event('input'));
    await settle();
    button('Confirmar')!.click();
    await settle();
    // The +5 of Atletismo is not added: 19 is never sent.
    expect(api.started[0].input.die).toEqual({ faces: [14] });
  });
});
