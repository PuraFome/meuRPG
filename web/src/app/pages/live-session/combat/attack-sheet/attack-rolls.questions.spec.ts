// The question of a held attack roll on the attack sheet: what the sheet takes from the answer of `AnswerHeldRoll` (the
// attack resolved, or still held for the die on the new d20), what tumbles, and the question a reaction window held.
import { create } from '@bufbuild/protobuf';

import {
  AttackOutcome,
  AttackRollSchema,
  DiceRollSchema,
  HeldRollKind,
  HeldRollOfferSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { HeldRollController } from '../../../../core/held-rolls/held-roll-controller';
import { heldAnswer, heldAttackAnswer } from '../../../../core/held-rolls/held-roll-testing';
import type { AttackDamage } from './attack-damage';
import { AttackRolls } from './attack-rolls';
import type { AttackSheetData } from './attack-sheet-types';
import { AttackState } from './attack-state';

const d20 = (face: number, over: Record<string, unknown> = {}) =>
  create(DiceRollSchema, {
    diceCount: 1,
    diceSides: 20,
    faces: [face],
    modifier: 6,
    total: face + 6,
    ...over,
  });
const lucky = create(HeldRollOfferSchema, {
  holdId: 'h1',
  kind: HeldRollKind.WEAPON_ATTACK,
  d20: d20(1),
  question: { case: 'lucky', value: { luckyIndex: 0 } },
});
const die = create(HeldRollOfferSchema, {
  holdId: 'h2',
  kind: HeldRollKind.WEAPON_ATTACK,
  d20: d20(9),
  question: { case: 'die', value: { sides: 8, fromName: 'Orla' } },
});

describe('the question of a held attack roll', () => {
  let played: unknown[];
  let held: string[];
  let state: CombatState;
  let rolls: AttackRolls;
  let question: HeldRollController;
  let s: AttackState;

  beforeEach(() => {
    played = [];
    held = [];
    state = new CombatState();
    state.apply(encounter({ combatants: [combatant({ id: 'a', label: 'Toren' })] }));
    const data = {
      campaignId: 'c',
      encounterId: 'enc',
      attackerId: 'a',
      state,
      onHeld: (id: string) => held.push(id),
    } as unknown as AttackSheetData;
    s = new AttackState(data);
    s.stage.set('done');
    question = new HeldRollController({
      send: () => Promise.reject(new Error('unused')),
      settle: () => undefined,
    });
    rolls = new AttackRolls(s, {} as AttackDamage, question, {
      api: {} as never,
      animator: { play: (show) => played.push(show) },
      attack: {} as never,
      name: 'Espada',
      modeChoice: () => undefined,
    });
  });

  const resolved = (face = 14) => ({
    encounter: encounter({}),
    roll: create(AttackRollSchema, {
      outcome: AttackOutcome.HIT,
      d20: d20(face, { rerolled: [{ index: 0, from: 1, to: face, sourceKey: 'trait:lucky' }] }),
      bonusDice: [{ sides: 8, face: 5, used: true }],
    }),
    pending: undefined,
  });

  describe('a roll held for a question', () => {
    it('asks Lucky’s question before the result, tells the page about the hold and stops typing', () => {
      s.typing.set(true);
      rolls.settle({
        roll: create(AttackRollSchema, { d20: d20(1) }),
        pending: undefined,
        held: lucky,
      });
      expect(question.offer()).toBe(lucky);
      expect(s.stage()).toBe('lucky');
      expect(s.typing()).toBe(false);
      expect(held).toEqual(['h1']);
    });

    it('asks the die’s question next, and no question once the roll is written', () => {
      rolls.settle({
        roll: create(AttackRollSchema, { d20: d20(9) }),
        pending: undefined,
        held: die,
      });
      expect(s.stage()).toBe('inspire');
      rolls.settle(resolved());
      expect(question.offer()).toBeNull();
      expect(s.stage()).toBe('damage' === s.stage() ? 'damage' : 'done');
    });
  });

  describe('the question a reaction window held', () => {
    it('shows the question the combat now carries and tells the page about the hold', () => {
      rolls.askAfterWindow(lucky);
      expect(s.stage()).toBe('lucky');
      expect(question.offer()).toBe(lucky);
      rolls.askAfterWindow(die);
      expect(s.stage()).toBe('inspire');
      expect(held).toEqual(['h1', 'h2']);
    });

    it('does nothing when the combat carries no question', () => {
      rolls.askAfterWindow(undefined);
      expect(s.stage()).toBe('done');
      expect(held).toEqual([]);
    });
  });

  describe('the answer', () => {
    const roll = { inApp: true } as const;

    it('settles the attack it brings and tumbles the d20 Lucky rolled again in the app', () => {
      rolls.answered(heldAttackAnswer(resolved()), {
        answer: { kind: 'lucky', reroll: true, roll },
      });
      expect(s.roll()?.outcome).toBe(AttackOutcome.HIT);
      expect(played).toHaveLength(1);
      expect(state.encounter()).not.toBeNull();
    });

    it('tumbles nothing for a typed face, or for the 1 kept', () => {
      rolls.answered(heldAttackAnswer(resolved()), {
        answer: { kind: 'lucky', reroll: true, roll: { face: 14 } },
      });
      rolls.answered(heldAttackAnswer(resolved()), {
        answer: { kind: 'lucky', reroll: false, roll: null },
      });
      expect(played).toEqual([]);
    });

    it('tumbles the die the player added in the app, and not one kept', () => {
      rolls.answered(heldAttackAnswer(resolved()), { answer: { kind: 'die', use: true, roll } });
      expect(played).toHaveLength(1);
      rolls.answered(heldAttackAnswer(resolved()), {
        answer: { kind: 'die', use: false, roll: null },
      });
      expect(played).toHaveLength(1);
    });

    it('goes on to the die on the new d20 when the roll is still held', () => {
      rolls.answered(
        heldAttackAnswer({
          ...resolved(6),
          roll: create(AttackRollSchema, { d20: d20(6) }),
          held: die,
        }),
        { answer: { kind: 'lucky', reroll: true, roll } },
      );
      expect(s.stage()).toBe('inspire');
      expect(question.offer()).toBe(die);
    });

    it('fails when the answer is not an attack', () => {
      expect(() =>
        rolls.answered(heldAnswer({ result: { case: 'sceneCheck', value: {} } }), {
          answer: { kind: 'lucky', reroll: false, roll: null },
        }),
      ).toThrow('without its attack');
    });
  });
});
