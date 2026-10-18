// The initiator's roll kept for a question: the contest opens when the question is answered.
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CombatantKind,
  CombatantSide,
  HeldRollKind,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  CheckOptionSchema,
  ContestAttackOptionKind,
  ContestAttackOptionSchema,
  ContestPurpose,
  ContestTargetSchema,
  RollModeKind,
} from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { StartContestResponseSchema } from '../../../../../gen/meurpg/play/v1/contests_pb';
import { CreatureSize } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { CombatState } from '../../../../core/combat/combat-state';
import { ContestClient } from '../../../../core/combat/contest-client';
import { ContestState } from '../../../../core/combat/contest-state';
import { FakeContestClient, checkRoll, contestView } from '../../../../core/combat/contest-testing';
import { heldAnswer, heldDie } from '../../../../core/held-rolls/held-roll-testing';
import { ResourceClient } from '../../../../core/resources/resources-client';
import { ContestSheet, type ContestSheetData } from './contest-sheet';

describe('ContestSheet: the roll kept for a question', () => {
  it('opens the contest with the answer, and not before', async () => {
    Element.prototype.scrollTo = vi.fn() as never;
    const enc = encounter({
      combatants: [
        combatant({
          id: 't',
          label: 'Toren',
          kind: CombatantKind.PLAYER,
          side: CombatantSide.PARTY,
        }),
        combatant({ id: 'h', label: 'Hobgoblin' }),
      ],
    });
    const api = new FakeContestClient();
    api.encounterAnswer = enc;
    api.held = heldDie(15, 5, 8, { kind: HeldRollKind.CONTEST });
    const opened = contestView({ id: 'ct1', initiatorRoll: checkRoll() });
    const resources = {
      answerHeldRoll: vi.fn(async () =>
        heldAnswer({
          result: {
            case: 'contest',
            value: create(StartContestResponseSchema, { encounter: enc, contest: opened }),
          },
        }),
      ),
    };
    const state = new CombatState();
    state.encounter.set(enc);
    const contests = new ContestState();
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
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state,
      contests,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: ContestClient, useValue: api.as() },
        { provide: ResourceClient, useValue: resources },
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
    button('Rolar no app')!.click();
    await settle();
    expect(contests.contest('ct1')).toBeUndefined();
    expect(el.querySelector('app-held-roll-prompt')).not.toBeNull();

    button('Guardar o dado')!.click();
    await settle();
    expect(resources.answerHeldRoll).toHaveBeenCalledTimes(1);
    expect(contests.contest('ct1')).toBeDefined();
    expect(el.querySelector('app-held-roll-prompt')).toBeNull();
  });
});
