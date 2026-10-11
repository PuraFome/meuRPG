import { create } from '@bufbuild/protobuf';

import {
  ConcentrationSaveResultSchema,
  CounterspellResultSchema,
  CuttingWordsResultSchema,
  DeflectMissilesResultSchema,
  DiceRollSchema,
  HellishRebukeResultSchema,
  ReactionResultSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { CheckRollSchema, RollModeKind } from '../../../gen/meurpg/play/v1/contest_types_pb';
import { EffectSaveResultSchema } from '../../../gen/meurpg/play/v1/lasting_effects_pb';
import {
  showOfCheck,
  showOfConcentration,
  showOfEffectSave,
  showOfReaction,
} from './roll-animator';

const d20 = (face: number, physical = false) =>
  create(DiceRollSchema, {
    diceCount: 1,
    diceSides: 20,
    faces: [face],
    modifier: 3,
    total: face + 3,
    physical,
  });

describe('the roll shows of the other in-app rolls', () => {
  it('shows a contest, Hide or group-check d20 and not a typed one', () => {
    const roll = create(CheckRollSchema, { faces: [12], modifier: 5, total: 17 });
    const show = showOfCheck('Teste de Atletismo', roll, { withTotal: true });
    expect(show?.label).toBe('Teste de Atletismo');
    expect(show?.dice).toEqual([{ sides: 20, face: 12, counts: true }]);
    expect(show?.line).toBe('12 + 5 = 17');
    expect(showOfCheck('x', { ...roll, physical: true })).toBeNull();
    expect(showOfCheck('x', undefined)).toBeNull();
  });

  it('highlights the d20 that counts with advantage and with disadvantage', () => {
    const adv = create(CheckRollSchema, {
      faces: [4, 15],
      modifier: 0,
      total: 15,
      mode: RollModeKind.ADVANTAGE,
    });
    expect(showOfCheck('x', adv)?.dice.map((d) => d.counts)).toEqual([false, true]);
    const dis = { ...adv, mode: RollModeKind.DISADVANTAGE };
    expect(showOfCheck('x', dis)?.dice.map((d) => d.counts)).toEqual([true, false]);
  });

  it("shows the dice of a reaction, never the attacker's Repreensão Infernal save, a typed die or an ineffective Cutting Words", () => {
    const rebuke = create(ReactionResultSchema, {
      result: {
        case: 'hellishRebuke',
        value: create(HellishRebukeResultSchema, { save: d20(9), saved: false }),
      },
    });
    // The save is the attacker's roll, not the reactor's.
    expect(showOfReaction(rebuke)).toBeNull();
    const typed = create(ReactionResultSchema, {
      result: {
        case: 'hellishRebuke',
        value: create(HellishRebukeResultSchema, { save: d20(9, true) }),
      },
    });
    expect(showOfReaction(typed)).toBeNull();
    const counter = create(ReactionResultSchema, {
      result: {
        case: 'counterspell',
        value: create(CounterspellResultSchema, { countered: true, check: d20(14) }),
      },
    });
    expect(showOfReaction(counter)?.outcome?.word).toBe('Anulada');
    const noCheck = create(ReactionResultSchema, {
      result: {
        case: 'counterspell',
        value: create(CounterspellResultSchema, { countered: true }),
      },
    });
    expect(showOfReaction(noCheck)).toBeNull();
    const cutting = create(ReactionResultSchema, {
      result: {
        case: 'cuttingWords',
        value: create(CuttingWordsResultSchema, { effective: false, die: d20(2) }),
      },
    });
    expect(showOfReaction(cutting)).toBeNull();
    const deflect = create(ReactionResultSchema, {
      result: {
        case: 'deflectMissiles',
        value: create(DeflectMissilesResultSchema, { reduction: d20(7) }),
      },
    });
    expect(showOfReaction(deflect)?.label).toBe('Defletir Projéteis');
    expect(showOfReaction(undefined)).toBeNull();
  });

  it('shows a concentration save with the word the sheet tells, but not a typed one', () => {
    const kept = create(ConcentrationSaveResultSchema, { save: d20(15), kept: true });
    expect(showOfConcentration(kept)?.outcome).toEqual({ word: 'Passou', good: true });
    expect(showOfConcentration({ ...kept, save: d20(15, true) })).toBeNull();
    expect(showOfConcentration(undefined)).toBeNull();
  });

  it('shows an effect saving throw the app rolled, not a skipped or typed one', () => {
    const saved = create(EffectSaveResultSchema, {
      d20: 14,
      d20Faces: [14],
      modifier: 2,
      total: 16,
      saved: true,
    });
    const show = showOfEffectSave('Teste de resistência de Sabedoria', saved);
    expect(show?.line).toBe('14 + 2 = 16');
    expect(show?.outcome).toEqual({ word: 'Passou', good: true });
    expect(showOfEffectSave('x', { ...saved, physical: true })).toBeNull();
    expect(showOfEffectSave('x', { ...saved, skipped: true })).toBeNull();
  });
});
